// Package auth provides authentication strategies for both headless (playwright)
// and HTTP crawl modes.
//
// SECURITY NOTE: Credentials passed via CLI flags may appear in shell history.
// Consider using environment variables or a config file for sensitive values.
package auth

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/RowanDark/v0x/internal/config"
	"github.com/playwright-community/playwright-go"
)

// Strategy defines how authentication is applied in headless and HTTP modes.
type Strategy interface {
	// ApplyToPage configures a playwright BrowserContext before crawling begins.
	ApplyToPage(ctx context.Context, browserCtx playwright.BrowserContext, cfg config.Config) error
	// ApplyToRequest adds auth headers/cookies to an outgoing HTTP request.
	ApplyToRequest(req *http.Request, cfg config.Config)
}

// New returns the appropriate Strategy based on cfg flags, or nil if no auth is configured.
// Priority: form > basic > cookie > bearer/header.
func New(cfg config.Config) Strategy {
	switch {
	case cfg.AuthFormURL != "":
		return &FormAuth{}
	case cfg.AuthBasicUser != "":
		return &BasicAuth{}
	case cfg.AuthCookie != "":
		return &CookieAuth{}
	case cfg.AuthBearer != "" || cfg.AuthHeader != "":
		return &BearerAuth{}
	default:
		return nil
	}
}

// FormAuth navigates to a login page, fills credentials, and submits the form.
// This strategy is playwright-only; it is a no-op for HTTP mode.
type FormAuth struct{}

func (f *FormAuth) ApplyToPage(ctx context.Context, browserCtx playwright.BrowserContext, cfg config.Config) error {
	pg, err := browserCtx.NewPage()
	if err != nil {
		return fmt.Errorf("form auth: new page: %w", err)
	}
	defer pg.Close()

	if _, err := pg.Goto(cfg.AuthFormURL, playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateNetworkidle,
	}); err != nil {
		return fmt.Errorf("form auth: navigate to login page: %w", err)
	}

	userField := cfg.AuthFormUserField
	if userField == "" {
		userField = "username"
	}
	passField := cfg.AuthFormPassField
	if passField == "" {
		passField = "password"
	}
	submitSel := cfg.AuthFormSubmit
	if submitSel == "" {
		submitSel = "[type=submit]"
	}

	if err := pg.Fill(fmt.Sprintf("[name=%s]", userField), cfg.AuthFormUser); err != nil {
		return fmt.Errorf("form auth: fill username field: %w", err)
	}
	if err := pg.Fill(fmt.Sprintf("[name=%s]", passField), cfg.AuthFormPass); err != nil {
		return fmt.Errorf("form auth: fill password field: %w", err)
	}
	if err := pg.Click(submitSel); err != nil {
		return fmt.Errorf("form auth: click submit: %w", err)
	}

	if err := pg.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
		State: playwright.LoadStateNetworkidle,
	}); err != nil {
		return fmt.Errorf("form auth: wait for post-login navigation: %w", err)
	}

	// NOTE: This check is heuristic. It will false-positive if the app uses MFA
	// at a /login/* path, and false-negative if the app serves a success page at
	// the same path or redirects to a different domain (SSO/OAuth). For reliable
	// auth verification, use a --auth-verify-selector flag (a CSS selector that
	// should only be present on an authenticated page).
	loginPath := ""
	if u, err := url.Parse(cfg.AuthFormURL); err == nil {
		loginPath = u.Path
	}
	if loginPath != "" && strings.Contains(pg.URL(), loginPath) {
		if cfg.AuthVerifySelector == "" {
			return fmt.Errorf("form auth: login appears to have failed — current URL still contains login path %q", loginPath)
		}
	}

	if cfg.AuthVerifySelector != "" {
		count, err := pg.Locator(cfg.AuthVerifySelector).Count()
		if err != nil {
			return fmt.Errorf("form auth: verify selector check failed: %w", err)
		}
		if count == 0 {
			return fmt.Errorf("form auth: post-login verification failed — selector %q not found on %s",
				cfg.AuthVerifySelector, pg.URL())
		}
	}

	return nil
}

// ApplyToRequest is a no-op for form auth; HTTP mode cannot replay a browser form login.
// The caller in http.go emits a warning when form auth is configured in HTTP mode.
func (f *FormAuth) ApplyToRequest(_ *http.Request, _ config.Config) {}

// BasicAuth applies HTTP Basic Authentication credentials.
type BasicAuth struct{}

func (b *BasicAuth) ApplyToPage(_ context.Context, browserCtx playwright.BrowserContext, cfg config.Config) error {
	encoded := base64.StdEncoding.EncodeToString([]byte(cfg.AuthBasicUser + ":" + cfg.AuthBasicPass))
	return browserCtx.SetExtraHTTPHeaders(map[string]string{
		"Authorization": "Basic " + encoded,
	})
}

func (b *BasicAuth) ApplyToRequest(req *http.Request, cfg config.Config) {
	encoded := base64.StdEncoding.EncodeToString([]byte(cfg.AuthBasicUser + ":" + cfg.AuthBasicPass))
	req.Header.Set("Authorization", "Basic "+encoded)
}

// CookieAuth injects cookies into the browser context or HTTP request.
type CookieAuth struct{}

func (c *CookieAuth) ApplyToPage(_ context.Context, browserCtx playwright.BrowserContext, cfg config.Config) error {
	targetURL := cfg.URL
	u, err := url.Parse(targetURL)
	if err != nil {
		return fmt.Errorf("cookie auth: parse target URL: %w", err)
	}
	origin := u.Scheme + "://" + u.Host

	var cookies []playwright.OptionalCookie
	for _, part := range strings.Split(cfg.AuthCookie, "; ") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		idx := strings.IndexByte(part, '=')
		if idx < 0 {
			continue
		}
		name := strings.TrimSpace(part[:idx])
		value := part[idx+1:]
		cookies = append(cookies, playwright.OptionalCookie{
			Name:  name,
			Value: value,
			URL:   playwright.String(origin),
		})
	}

	if len(cookies) == 0 {
		return nil
	}
	return browserCtx.AddCookies(cookies)
}

func (c *CookieAuth) ApplyToRequest(req *http.Request, cfg config.Config) {
	for _, part := range strings.Split(cfg.AuthCookie, "; ") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		idx := strings.IndexByte(part, '=')
		if idx < 0 {
			continue
		}
		req.AddCookie(&http.Cookie{
			Name:  strings.TrimSpace(part[:idx]),
			Value: part[idx+1:],
		})
	}
}

// BearerAuth applies a Bearer token or custom header.
type BearerAuth struct{}

func (b *BearerAuth) ApplyToPage(_ context.Context, browserCtx playwright.BrowserContext, cfg config.Config) error {
	headers := make(map[string]string)
	if cfg.AuthBearer != "" {
		headers["Authorization"] = "Bearer " + cfg.AuthBearer
	}
	if cfg.AuthHeader != "" {
		idx := strings.Index(cfg.AuthHeader, ": ")
		if idx >= 0 {
			headers[cfg.AuthHeader[:idx]] = cfg.AuthHeader[idx+2:]
		}
	}
	if len(headers) == 0 {
		return nil
	}
	return browserCtx.SetExtraHTTPHeaders(headers)
}

func (b *BearerAuth) ApplyToRequest(req *http.Request, cfg config.Config) {
	if cfg.AuthBearer != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.AuthBearer)
	}
	if cfg.AuthHeader != "" {
		idx := strings.Index(cfg.AuthHeader, ": ")
		if idx >= 0 {
			req.Header.Set(cfg.AuthHeader[:idx], cfg.AuthHeader[idx+2:])
		}
	}
}
