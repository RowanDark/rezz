package config

import "time"

// Config holds all runtime configuration derived from CLI flags.
//
// SECURITY NOTE: Credentials passed via CLI flags (--auth-*) may appear in
// shell history. Prefer environment variables or a config file for sensitive values.
type Config struct {
	URL           string
	Depth         int
	Delay         int
	MinWordLength int
	UserAgent     string
	Output        string
	Format        string
	Headless      bool
	Verbose       bool
	Quiet         bool
	Timeout       time.Duration

	// Form-based login
	AuthFormURL       string
	AuthFormUser      string
	AuthFormPass      string
	AuthFormUserField string // default: "username"
	AuthFormPassField string // default: "password"
	AuthFormSubmit    string // default: "[type=submit]"
	AuthVerifySelector string // CSS selector that must be present after login

	// HTTP Basic auth
	AuthBasicUser string
	AuthBasicPass string

	// Cookie injection ("name=value; name2=value2")
	AuthCookie string

	// Bearer token / custom header
	AuthBearer string
	AuthHeader string // "Name: Value" format

	// Scope control
	Scope       string // comma-separated scope entries
	StrictScope bool   // if true, no subdomain expansion

	// Pattern scanning
	Patterns   string // comma-separated kit names
	CustomFile string // path to custom YAML

	// Robots.txt
	NoRobots bool // --no-robots: skip robots.txt fetching and checking

	// Rate limiting
	Jitter float64 // --jitter: fraction of delay to add randomly (default 0.5)

	// Deduplication
	DedupGlobal bool // --dedup-global: deduplicate by match+pattern across all URLs
	NoDedup     bool // --no-dedup: disable all deduplication

	// Output color control
	NoColor    bool // --no-color: disable ANSI color output
	ForceColor bool // --color: force color even when not a terminal

	// Summary
	Summary bool // --summary: print severity breakdown after scan completes

	// Endpoint probing
	NoProbe bool // --no-probe: disable HEAD probing
	Workers int  // concurrent probe workers (0 = default 10)
}
