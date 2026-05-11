package config

// Config holds all runtime configuration derived from CLI flags.
type Config struct {
	URL           string
	Depth         int
	MinWordLength int
	UserAgent     string
	Output        string
	Format        string
	Headless      bool
	Verbose       bool
}
