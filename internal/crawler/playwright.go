package crawler

import (
	"context"
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/RowanDark/rezz/internal/auth"
	"github.com/RowanDark/rezz/internal/config"
	"github.com/playwright-community/playwright-go"
)

// PlaywrightCrawler uses a headless Chromium browser to render pages before
// extracting links, enabling discovery of dynamically loaded content.
type PlaywrightCrawler struct {
	auth auth.Strategy
}

type queueItem struct {
	url   string
	depth int
}

func (c *PlaywrightCrawler) Crawl(ctx context.Context, cfg config.Config) (<-chan Page, error) {
	base, err := url.Parse(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	pw, err := playwright.Run()
	if err != nil {
		return nil, fmt.Errorf("playwright start: %w", err)
	}

	browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(true),
	})
	if err != nil {
		pw.Stop() //nolint:errcheck
		return nil, fmt.Errorf("browser launch: %w", err)
	}

	// A single shared context preserves session cookies across all crawled pages,
	// which is necessary for auth strategies to remain effective after login.
	bctx, err := browser.NewContext(playwright.BrowserNewContextOptions{
		UserAgent: playwright.String(cfg.UserAgent),
	})
	if err != nil {
		browser.Close()
		pw.Stop() //nolint:errcheck
		return nil, fmt.Errorf("browser context: %w", err)
	}

	if c.auth != nil {
		if err := c.auth.ApplyToPage(ctx, bctx, cfg); err != nil {
			bctx.Close()
			browser.Close()
			pw.Stop() //nolint:errcheck
			return nil, fmt.Errorf("auth: %w", err)
		}
	}

	pages := make(chan Page)
	delay := time.Duration(cfg.Delay) * time.Millisecond

	go func() {
		defer close(pages)
		defer bctx.Close()
		defer browser.Close()
		defer pw.Stop() //nolint:errcheck

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

			pg, err := bctx.NewPage()
			if err != nil {
				continue
			}

			_, err = pg.Goto(item.url, playwright.PageGotoOptions{
				WaitUntil: playwright.WaitUntilStateNetworkidle,
				Timeout:   playwright.Float(30000),
			})
			if err != nil {
				pg.Close()
				continue
			}

			html, err := pg.Content()
			pg.Close()
			if err != nil {
				continue
			}

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
