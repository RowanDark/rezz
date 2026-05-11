package extractor

import "github.com/RowanDark/v0x/internal/config"

// Result holds extracted words and email addresses from a page.
type Result struct {
	Words  []string
	Emails []string
}

// Extractor defines the interface for content extractors.
type Extractor interface {
	Extract(html string, cfg *config.Config) (*Result, error)
}
