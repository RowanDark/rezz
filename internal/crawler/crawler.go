package crawler

import (
	"context"

	"github.com/RowanDark/v0x/internal/config"
)

// Page represents a crawled page.
type Page struct {
	URL   string
	HTML  string
	Depth int
}

// Crawler defines the interface for web crawlers.
type Crawler interface {
	Crawl(ctx context.Context, cfg config.Config) (<-chan Page, error)
}

// New returns a PlaywrightCrawler when headless mode is enabled, otherwise an HTTPCrawler.
func New(cfg config.Config) Crawler {
	if cfg.Headless {
		return &PlaywrightCrawler{}
	}
	return &HTTPCrawler{}
}
