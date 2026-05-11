package crawler

import "github.com/RowanDark/v0x/internal/config"

// Crawler defines the interface for web crawlers.
type Crawler interface {
	Crawl(cfg *config.Config) ([]string, error)
}
