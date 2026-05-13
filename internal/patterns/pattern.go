package patterns

import (
	"regexp"
	"time"
)

type Pattern struct {
	Name     string `yaml:"name"`
	Regex    string `yaml:"regex"`
	Category string `yaml:"category"`
	Severity string `yaml:"severity"` // high | medium | low
	compiled *regexp.Regexp
}

type kitFile struct {
	Kit      string    `yaml:"kit"`
	Patterns []Pattern `yaml:"patterns"`
}

type Finding struct {
	URL        string    // page URL where match was found
	Pattern    string    // pattern name
	Category   string    // pattern category
	Severity   string    // high | medium | low
	Match      string    // matched text, capped at 200 chars
	Context    string    // up to 100 chars before and after match
	StatusCode int       // HTTP status of source page
	Timestamp  time.Time
	FromJS     bool // true if page came from headless render
}
