package crawler

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/RowanDark/rezz/internal/auth"
	"github.com/RowanDark/rezz/internal/config"
	"github.com/RowanDark/rezz/internal/ratelimit"
	"github.com/RowanDark/rezz/internal/robots"
	"github.com/RowanDark/rezz/internal/scope"
)

// HTTPCrawler uses net/http and goquery for static pages that do not require JS.
type HTTPCrawler struct {
	auth auth.Strategy
}

func (c *HTTPCrawler) Crawl(ctx context.Context, cfg config.Config) (<-chan Page, error) {
	eng, err := scope.New(cfg.Scope, cfg.URL, cfg.StrictScope)
	if err != nil {
		return nil, fmt.Errorf("scope: %w", err)
	}

	// Form auth requires a browser; warn and skip gracefully in HTTP mode.
	if cfg.AuthFormURL != "" {
		fmt.Fprintln(os.Stderr, "rezz: warning: --auth-form-* flags are not supported in HTTP mode (--no-headless); form auth will be skipped")
	}

	client := &http.Client{
		Timeout: 30 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}
	delay := time.Duration(cfg.Delay) * time.Millisecond

	pages := make(chan Page)

	go func() {
		defer close(pages)

		var rb *robots.Checker
		if !cfg.NoRobots {
			rb = robots.New(cfg.URL, cfg.UserAgent)
			if cfg.Verbose {
				u, _ := url.Parse(cfg.URL)
				fmt.Fprintf(os.Stderr, "rezz: fetched robots.txt for %s\n", u.Host)
			}
		} else {
			rb = robots.NewPermissive()
		}

		var visited sync.Map
		queue := []queueItem{{url: cfg.URL, depth: 0}}

		for len(queue) > 0 {
			select {
			case <-ctx.Done():
				return
			default:
			}

			item := queue[0]
			queue = queue[1:]

			if _, seen := visited.LoadOrStore(item.url, struct{}{}); seen {
				continue
			}

			if !rb.Allowed(item.url) {
				if cfg.Verbose {
					fmt.Fprintf(os.Stderr, "rezz: skip %s (robots.txt)\n", item.url)
				}
				continue
			}

			req, err := http.NewRequestWithContext(ctx, http.MethodGet, item.url, nil)
			if err != nil {
				continue
			}
			req.Header.Set("User-Agent", cfg.UserAgent)

			if c.auth != nil {
				c.auth.ApplyToRequest(req, cfg)
			}

			resp, err := client.Do(req)
			if err != nil {
				continue
			}
			if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				resp.Body.Close()
				if cfg.Verbose {
					fmt.Fprintf(os.Stderr, "rezz: skip %s (HTTP %d)\n", item.url, resp.StatusCode)
				}
				continue
			}

			doc, err := goquery.NewDocumentFromReader(resp.Body)
			resp.Body.Close()
			if err != nil {
				continue
			}

			var sb strings.Builder
			doc.Find("html").Each(func(_ int, s *goquery.Selection) {
				html, _ := goquery.OuterHtml(s)
				sb.WriteString(html)
			})
			html := sb.String()

			base, err := url.Parse(item.url)
			if err != nil {
				continue
			}

			select {
			case <-ctx.Done():
				return
			case pages <- Page{URL: item.url, HTML: html, Depth: item.depth}:
			}

			// Fetch in-scope script files discovered on this page
			scriptURLs := extractScriptSrcs(html, item.url, base, eng, &visited)
			for _, scriptURL := range scriptURLs {
				if _, seen := visited.LoadOrStore(scriptURL, struct{}{}); seen {
					continue
				}
				scriptPage, err := fetchScript(scriptURL, cfg.UserAgent)
				if err != nil {
					if cfg.Verbose {
						fmt.Fprintf(os.Stderr, "rezz: script fetch failed %s: %v\n", scriptURL, err)
					}
					continue
				}
				if cfg.Verbose {
					fmt.Fprintf(os.Stderr, "rezz: fetched script %s\n", scriptURL)
				}
				select {
				case pages <- *scriptPage:
				case <-ctx.Done():
					return
				}
			}

			if item.depth < cfg.Depth {
				links := extractLinks(html, item.url, eng, cfg.Verbose)
				for _, link := range links {
					if _, seen := visited.Load(link); !seen {
						queue = append(queue, queueItem{url: link, depth: item.depth + 1})
					}
				}
			}

			ratelimit.Wait(ctx, delay, cfg.Jitter)
		}
	}()

	return pages, nil
}

// extractLinks parses <a href> links and <meta http-equiv="refresh"> redirects
// from html, resolves them against pageURL, and returns only in-scope absolute URLs.
func extractLinks(html, pageURL string, eng *scope.Engine, verbose bool) []string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil
	}

	page, err := url.Parse(pageURL)
	if err != nil {
		return nil
	}

	seen := map[string]struct{}{}
	var links []string

	doc.Find("a[href]").Each(func(_ int, s *goquery.Selection) {
		href, _ := s.Attr("href")
		if href == "" {
			return
		}
		ref, err := url.Parse(href)
		if err != nil {
			return
		}
		resolved := page.ResolveReference(ref)
		resolved.Fragment = ""

		// Strip path parameters (e.g. ;jsessionid=abc123) from URL path.
		// These are appended by Java EE servers when cookie-based sessions
		// are unavailable and must be normalized for correct deduplication.
		if idx := strings.Index(resolved.Path, ";"); idx >= 0 {
			resolved.Path = resolved.Path[:idx]
			resolved.RawPath = ""
		}

		if resolved.Scheme != "http" && resolved.Scheme != "https" {
			return
		}
		if !eng.InScope(resolved.String()) {
			if verbose {
				fmt.Fprintf(os.Stderr, "rezz: skip %s (out of scope)\n", resolved.String())
			}
			return
		}
		key := resolved.String()
		if _, exists := seen[key]; exists {
			return
		}
		seen[key] = struct{}{}
		links = append(links, key)
	})

	// Extract meta-refresh redirect URLs
	doc.Find("meta[http-equiv]").Each(func(_ int, s *goquery.Selection) {
		equiv, _ := s.Attr("http-equiv")
		if !strings.EqualFold(equiv, "refresh") {
			return
		}
		content, exists := s.Attr("content")
		if !exists {
			return
		}
		// content format: "0;url=/path" or "0; URL=/path"
		parts := strings.SplitN(content, ";", 2)
		if len(parts) != 2 {
			return
		}
		urlPart := strings.TrimSpace(parts[1])
		// Strip "url=" or "URL=" prefix
		if idx := strings.Index(strings.ToLower(urlPart), "url="); idx >= 0 {
			urlPart = urlPart[idx+4:]
		}
		urlPart = strings.Trim(urlPart, `"' `)
		if urlPart == "" {
			return
		}
		ref, err := url.Parse(urlPart)
		if err != nil {
			return
		}
		resolved := page.ResolveReference(ref)
		resolved.Fragment = ""
		if resolved.Scheme != "http" && resolved.Scheme != "https" {
			return
		}
		if !eng.InScope(resolved.String()) {
			return
		}
		key := resolved.String()
		if _, exists := seen[key]; !exists {
			seen[key] = struct{}{}
			links = append(links, key)
		}
	})

	return links
}
