package crawler

import (
	"context"

	"github.com/RowanDark/rezz/internal/auth"
	"github.com/RowanDark/rezz/internal/config"
)

// Page represents a crawled page.
type Page struct {
	URL      string
	HTML     string
	Depth    int
	IsScript bool // true if this is a fetched JS or CSS resource, not an HTML page
}

// Crawler defines the interface for web crawlers.
type Crawler interface {
	Crawl(ctx context.Context, cfg config.Config) (<-chan Page, error)
}

// New returns a PlaywrightCrawler when headless mode is enabled, otherwise an HTTPCrawler.
// The appropriate auth strategy is derived from cfg and wired in automatically.
func New(cfg config.Config) Crawler {
	strategy := auth.New(cfg)
	if cfg.Headless {
		return &PlaywrightCrawler{auth: strategy}
	}
	return &HTTPCrawler{auth: strategy}
}
