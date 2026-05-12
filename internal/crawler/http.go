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
	"github.com/RowanDark/v0x/internal/auth"
	"github.com/RowanDark/v0x/internal/config"
)

// HTTPCrawler uses net/http and goquery for static pages that do not require JS.
type HTTPCrawler struct {
	auth auth.Strategy
}

func (c *HTTPCrawler) Crawl(ctx context.Context, cfg config.Config) (<-chan Page, error) {
	base, err := url.Parse(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	// Form auth requires a browser; warn and skip gracefully in HTTP mode.
	if cfg.AuthFormURL != "" {
		fmt.Fprintln(os.Stderr, "v0x: warning: --auth-form-* flags are not supported in HTTP mode (--no-headless); form auth will be skipped")
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
					fmt.Fprintf(os.Stderr, "v0x: skip %s (HTTP %d)\n", item.url, resp.StatusCode)
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

			select {
			case <-ctx.Done():
				return
			case pages <- Page{URL: item.url, HTML: html, Depth: item.depth}:
			}

			if item.depth < cfg.Depth {
				links := extractLinks(html, item.url, base)
				for _, link := range links {
					if _, seen := visited.Load(link); !seen {
						queue = append(queue, queueItem{url: link, depth: item.depth + 1})
					}
				}
			}

			select {
			case <-ctx.Done():
				return
			case <-time.After(delay):
			}
		}
	}()

	return pages, nil
}

// extractLinks parses <a href> links from html, resolves them against pageURL,
// and returns only same-domain absolute URLs.
func extractLinks(html, pageURL string, base *url.URL) []string {
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

		if resolved.Host != base.Host {
			return
		}
		if resolved.Scheme != "http" && resolved.Scheme != "https" {
			return
		}
		key := resolved.String()
		if _, exists := seen[key]; exists {
			return
		}
		seen[key] = struct{}{}
		links = append(links, key)
	})

	return links
}
