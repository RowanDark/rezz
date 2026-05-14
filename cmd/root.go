package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/fatih/color"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"golang.org/x/term"

	"github.com/RowanDark/rezz/internal/config"
	"github.com/RowanDark/rezz/internal/crawler"
	"github.com/RowanDark/rezz/internal/output"
	"github.com/RowanDark/rezz/internal/patterns"
	"github.com/spf13/cobra"
)

const banner = `
┌─────────────────────────────────────┐
│  ██████╗ ███████╗███████╗███████╗   │
│  ██╔══██╗██╔════╝╚══███╔╝╚══███╔╝   │
│  ██████╔╝█████╗    ███╔╝   ███╔╝    │
│  ██╔══██╗██╔══╝   ███╔╝   ███╔╝     │
│  ██║  ██║███████╗███████╗███████╗   │
│  ╚═╝  ╚═╝╚══════╝╚══════╝╚══════╝   │
│  secret scanner  v1.0.0              │
│  github.com/RowanDark/rezz           │
└─────────────────────────────────────┘
`

var cfg config.Config
var noHeadless bool

var rootCmd = &cobra.Command{
	Use:          "rezz",
	Short:        "rezz is a web crawler and secret scanner",
	SilenceUsage: true,
	Long: `rezz crawls web pages and scans for secrets, credentials, and sensitive data
using embedded regex pattern kits. It supports headless browser crawling via
playwright-go and output formats including stream, json, and csv.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if cfg.NoColor {
			color.NoColor = true
		} else if cfg.ForceColor {
			color.NoColor = false
		}

		if !cfg.Quiet && term.IsTerminal(int(os.Stdout.Fd())) {
			fmt.Fprint(os.Stderr, banner)
		}

		if cfg.URL == "" {
			return fmt.Errorf("--url is required")
		}
		if noHeadless {
			cfg.Headless = false
		}

		if cfg.AuthVerifySelector != "" && cfg.AuthFormURL == "" {
			fmt.Fprintln(os.Stderr, "rezz: warning: --auth-verify-selector has no effect without --auth-form-url")
		}

		authStrategyName := "none"
		switch {
		case cfg.AuthFormURL != "":
			authStrategyName = "form"
		case cfg.AuthBasicUser != "":
			authStrategyName = "basic"
		case cfg.AuthCookie != "":
			authStrategyName = "cookie"
		case cfg.AuthBearer != "" || cfg.AuthHeader != "":
			authStrategyName = "bearer"
		}

		if cfg.Verbose && !cfg.Quiet {
			fmt.Fprintf(os.Stderr, "rezz: crawling %s (depth=%d, headless=%v, auth=%s)\n",
				cfg.URL, cfg.Depth, cfg.Headless, authStrategyName)
		}

		// Parse pattern kit names
		kitNames := strings.Split(cfg.Patterns, ",")
		for i, k := range kitNames {
			kitNames[i] = strings.TrimSpace(k)
		}

		// Build and load pattern registry
		log := zap.NewNop()
		if cfg.Verbose {
			log, _ = zap.NewProduction()
		}
		reg := patterns.New(log)
		if err := reg.Load(kitNames); err != nil {
			return fmt.Errorf("patterns: %w", err)
		}
		if cfg.Verbose {
			fmt.Fprintf(os.Stderr, "rezz: loaded %d patterns\n", reg.Count())
		}

		// Open output writer before crawling — fail fast if we can't write.
		w := os.Stdout
		if cfg.Output != "" {
			f, err := os.Create(cfg.Output)
			if err != nil {
				return fmt.Errorf("opening output file: %w", err)
			}
			defer f.Close()
			w = f
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		if cfg.Timeout > 0 {
			var timeoutCancel context.CancelFunc
			ctx, timeoutCancel = context.WithTimeout(ctx, cfg.Timeout)
			defer timeoutCancel()
		}

		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt)
		go func() {
			select {
			case <-sigCh:
				cancel()
			case <-ctx.Done():
			}
		}()

		// Resolve dedup mode
		dedupMode := patterns.DedupeExact // default
		if cfg.NoDedup {
			dedupMode = patterns.DedupeNone
		} else if cfg.DedupGlobal {
			dedupMode = patterns.DedupeGlobal
		}
		store := patterns.NewFindingStore(dedupMode)

		c := crawler.New(cfg)
		pages, err := c.Crawl(ctx, cfg)
		if err != nil {
			return fmt.Errorf("starting crawl: %w", err)
		}

		var pagesCrawled int

		g, _ := errgroup.WithContext(ctx)
		g.Go(func() error {
			for page := range pages {
				headerStr := ""
				findings := reg.Match(page.HTML, headerStr, page.URL, 200, false)
				for _, f := range findings {
					if store.Add(f) {
						if cfg.Format == "stream" && !cfg.Quiet {
							all := store.Findings()
							printFinding(w, all[store.Count()-1])
						}
					}
				}
				pagesCrawled++
				if cfg.Verbose && !cfg.Quiet {
					fmt.Fprintf(os.Stderr, "rezz: page %s — %d findings\n", page.URL, len(findings))
				}
			}
			return nil
		})

		if err := g.Wait(); err != nil {
			return fmt.Errorf("pipeline: %w", err)
		}

		if ctx.Err() == context.DeadlineExceeded {
			fmt.Fprintf(os.Stderr, "rezz: timeout reached after %s — writing partial results\n", cfg.Timeout)
		}

		// Write collected output for non-stream formats
		switch cfg.Format {
		case "json":
			jsonFindings := make([]output.JSONFinding, 0, store.Count())
			for _, sf := range store.Findings() {
				jsonFindings = append(jsonFindings, output.JSONFinding{
					Finding:     sf.Finding,
					AlsoFoundOn: sf.AlsoFoundOn,
				})
			}
			if err := output.JSONWriter(w, jsonFindings, cfg.URL, pagesCrawled); err != nil {
				return fmt.Errorf("writing output: %w", err)
			}
		case "csv":
			if err := output.CSVWriter(w, store.Findings()); err != nil {
				return fmt.Errorf("writing output: %w", err)
			}
		}

		if !cfg.Quiet {
			if dedupMode != patterns.DedupeNone {
				fmt.Fprintf(os.Stderr, "rezz: done — %d pages crawled, %d findings (deduped)\n",
					pagesCrawled, store.Count())
			} else {
				fmt.Fprintf(os.Stderr, "rezz: done — %d pages crawled, %d findings\n",
					pagesCrawled, store.Count())
			}
		}

		if cfg.Summary {
			findings := store.Findings()
			high, medium, low := 0, 0, 0
			for _, f := range findings {
				switch f.Severity {
				case "high":
					high++
				case "medium":
					medium++
				default:
					low++
				}
			}
			highColor := color.New(color.FgRed, color.Bold)
			medColor := color.New(color.FgYellow, color.Bold)
			lowColor := color.New(color.FgCyan, color.Bold)
			boldColor := color.New(color.Bold)

			fmt.Fprintln(os.Stderr)
			fmt.Fprintf(os.Stderr, "%s\n", boldColor.Sprint("── Summary ──────────────────"))
			fmt.Fprintf(os.Stderr, "  %s  %d\n", highColor.Sprint("HIGH  "), high)
			fmt.Fprintf(os.Stderr, "  %s  %d\n", medColor.Sprint("MEDIUM"), medium)
			fmt.Fprintf(os.Stderr, "  %s  %d\n", lowColor.Sprint("LOW   "), low)
			fmt.Fprintf(os.Stderr, "  %s  %d\n", boldColor.Sprint("TOTAL "), high+medium+low)
			fmt.Fprintln(os.Stderr)
		}

		return nil
	},
}

func printFinding(w io.Writer, f patterns.SeenFinding) {
	var labelColor *color.Color
	switch f.Severity {
	case "high":
		labelColor = color.New(color.FgRed, color.Bold)
	case "medium":
		labelColor = color.New(color.FgYellow, color.Bold)
	default:
		labelColor = color.New(color.FgCyan, color.Bold)
	}

	label := severityLabel(f.Severity)
	patternBold := color.New(color.Bold)
	categoryColor := color.New(color.FgWhite)
	urlColor := color.New(color.FgHiBlack)
	matchColor := color.New(color.FgWhite, color.Bold)
	dimColor := color.New(color.FgHiBlack)

	fmt.Fprintf(w, "[%s] %s — %s\n",
		labelColor.Sprint(label),
		patternBold.Sprint(f.Pattern),
		categoryColor.Sprint(f.Category),
	)
	fmt.Fprintf(w, "    URL:   %s\n", urlColor.Sprint(f.URL))
	fmt.Fprintf(w, "    Match: %s\n", matchColor.Sprint(f.Match))
	if len(f.AlsoFoundOn) > 0 {
		fmt.Fprintf(w, "    %s\n",
			dimColor.Sprintf("Also found on %d other page(s)", len(f.AlsoFoundOn)))
	}
	fmt.Fprintln(w)
}

func severityLabel(s string) string {
	switch s {
	case "high":
		return "HIGH  "
	case "medium":
		return "MEDIUM"
	default:
		return "LOW   "
	}
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "rezz:", err)
		os.Exit(1)
	}
}

func init() {
	flags := rootCmd.PersistentFlags()

	flags.StringVar(&cfg.URL, "url", "", "Target URL (required)")
	flags.IntVar(&cfg.Depth, "depth", 2, "Max crawl depth")
	flags.StringVar(&cfg.UserAgent, "user-agent", "rezz/1.0", "Custom User-Agent string")
	flags.StringVar(&cfg.Output, "output", "", "Output file path (default: stdout)")
	flags.StringVar(&cfg.Format, "format", "stream", "Output format: stream|json|csv (default: stream)")
	flags.BoolVar(&cfg.Headless, "headless", true, "Use headless browser (playwright-go)")
	flags.BoolVar(&noHeadless, "no-headless", false, "Disable headless, use net/http instead")
	flags.IntVar(&cfg.Delay, "delay", 500, "Delay in ms between requests")
	flags.BoolVar(&cfg.Verbose, "verbose", false, "Verbose logging")
	flags.BoolVar(&cfg.Quiet, "quiet", false, "Suppress banner and non-essential output")
	flags.DurationVar(&cfg.Timeout, "timeout", 5*time.Minute, "Max crawl duration (0 = unlimited)")

	flags.StringVar(&cfg.Patterns, "patterns", "api-keys,credentials,headers",
		"Pattern kits to load, comma-separated (api-keys,credentials,endpoints,financial,javascript,headers,cloud,all)")
	flags.StringVar(&cfg.CustomFile, "custom", "",
		"Path to a custom patterns YAML file")

	// Deduplication
	flags.BoolVar(&cfg.DedupGlobal, "dedup-global", false,
		"Deduplicate findings globally — show each unique match once regardless of how many pages it appears on")
	flags.BoolVar(&cfg.NoDedup, "no-dedup", false,
		"Disable all deduplication — emit every match raw")

	// Form-based login (playwright-only)
	flags.StringVar(&cfg.AuthFormURL, "auth-form-url", "", "URL of the login form page")
	flags.StringVar(&cfg.AuthFormUser, "auth-form-user", "", "Username to submit in the login form")
	flags.StringVar(&cfg.AuthFormPass, "auth-form-pass", "", "Password to submit in the login form")
	flags.StringVar(&cfg.AuthFormUserField, "auth-form-user-field", "username", "Name attribute of the username input")
	flags.StringVar(&cfg.AuthFormPassField, "auth-form-pass-field", "password", "Name attribute of the password input")
	flags.StringVar(&cfg.AuthFormSubmit, "auth-form-submit", "[type=submit]", "CSS selector for the submit button")
	flags.StringVar(&cfg.AuthVerifySelector, "auth-verify-selector", "", "CSS selector that must be present after login to confirm authentication succeeded")

	// HTTP Basic auth
	flags.StringVar(&cfg.AuthBasicUser, "auth-basic-user", "", "HTTP Basic auth username")
	flags.StringVar(&cfg.AuthBasicPass, "auth-basic-pass", "", "HTTP Basic auth password")

	// Cookie injection
	flags.StringVar(&cfg.AuthCookie, "auth-cookie", "", `Cookie string to inject, e.g. "session=abc; token=xyz"`)

	// Bearer token / custom header
	flags.StringVar(&cfg.AuthBearer, "auth-bearer", "", "Bearer token for Authorization header")
	flags.StringVar(&cfg.AuthHeader, "auth-header", "", `Custom auth header in "Name: Value" format`)

	// Color control
	flags.BoolVar(&cfg.NoColor, "no-color", false, "Disable ANSI color output")
	flags.BoolVar(&cfg.ForceColor, "color", false, "Force ANSI color output even when stdout is not a terminal")

	// Summary
	flags.BoolVar(&cfg.Summary, "summary", false, "Print severity breakdown summary after scan completes")

	// Scope control
	flags.StringVar(&cfg.Scope, "scope", "",
		`Comma-separated list of in-scope hosts. Base domains include subdomains (e.g. "paylution.com,hyperwallet.com"). Defaults to exact host of --url.`)
	flags.BoolVar(&cfg.StrictScope, "strict-scope", false,
		"Disable subdomain expansion — all scope entries match exact host only")
}
