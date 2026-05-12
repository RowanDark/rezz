package cmd

import (
	"fmt"
	"os"

	"github.com/RowanDark/v0x/internal/config"
	"github.com/spf13/cobra"
)

var cfg config.Config
var noHeadless bool

var rootCmd = &cobra.Command{
	Use:   "v0x",
	Short: "v0x is a modern web wordlist generator",
	Long: `v0x crawls web pages and extracts words to build targeted wordlists.
It supports headless browser crawling via playwright-go and structured
output formats including txt, json, csv, and markdown.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if cfg.URL == "" {
			return fmt.Errorf("--url is required")
		}
		if noHeadless {
			cfg.Headless = false
		}
		fmt.Fprintf(os.Stderr, "v0x: crawling %s (depth=%d, headless=%v)\n", cfg.URL, cfg.Depth, cfg.Headless)
		return nil
	},
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	flags := rootCmd.PersistentFlags()

	flags.StringVar(&cfg.URL, "url", "", "Target URL (required)")
	flags.IntVar(&cfg.Depth, "depth", 2, "Max crawl depth")
	flags.IntVar(&cfg.MinWordLength, "min-word-length", 3, "Minimum word length to collect")
	flags.StringVar(&cfg.UserAgent, "user-agent", "v0x/1.0", "Custom User-Agent string")
	flags.StringVar(&cfg.Output, "output", "", "Output file path (default: stdout)")
	flags.StringVar(&cfg.Format, "format", "txt", "Output format: txt, json, csv, md")
	flags.BoolVar(&cfg.Headless, "headless", true, "Use headless browser (playwright-go)")
	flags.BoolVar(&noHeadless, "no-headless", false, "Disable headless, use net/http instead")
	flags.IntVar(&cfg.Delay, "delay", 500, "Delay in ms between requests")
	flags.BoolVar(&cfg.Verbose, "verbose", false, "Verbose logging")

	// Form-based login (playwright-only)
	flags.StringVar(&cfg.AuthFormURL, "auth-form-url", "", "URL of the login form page")
	flags.StringVar(&cfg.AuthFormUser, "auth-form-user", "", "Username to submit in the login form")
	flags.StringVar(&cfg.AuthFormPass, "auth-form-pass", "", "Password to submit in the login form")
	flags.StringVar(&cfg.AuthFormUserField, "auth-form-user-field", "username", "Name attribute of the username input")
	flags.StringVar(&cfg.AuthFormPassField, "auth-form-pass-field", "password", "Name attribute of the password input")
	flags.StringVar(&cfg.AuthFormSubmit, "auth-form-submit", "[type=submit]", "CSS selector for the submit button")

	// HTTP Basic auth
	flags.StringVar(&cfg.AuthBasicUser, "auth-basic-user", "", "HTTP Basic auth username")
	flags.StringVar(&cfg.AuthBasicPass, "auth-basic-pass", "", "HTTP Basic auth password")

	// Cookie injection
	flags.StringVar(&cfg.AuthCookie, "auth-cookie", "", `Cookie string to inject, e.g. "session=abc; token=xyz"`)

	// Bearer token / custom header
	flags.StringVar(&cfg.AuthBearer, "auth-bearer", "", "Bearer token for Authorization header")
	flags.StringVar(&cfg.AuthHeader, "auth-header", "", `Custom auth header in "Name: Value" format`)
}
