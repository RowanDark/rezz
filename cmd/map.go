package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/RowanDark/rezz/internal/config"
	"github.com/RowanDark/rezz/internal/crawler"
	"github.com/RowanDark/rezz/internal/extractor"
	"github.com/RowanDark/rezz/internal/scope"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

var mapCmd = &cobra.Command{
	Use:          "map",
	Short:        "Map all endpoints and URLs discovered on a target",
	SilenceUsage: true,
	RunE:         runMap,
}

var mapCfg config.Config
var mapNoHeadless bool
var mapFormat string

func init() {
	f := mapCmd.Flags()
	f.StringVar(&mapCfg.URL, "url", "", "Target URL (required)")
	f.IntVar(&mapCfg.Depth, "depth", 2, "Max crawl depth")
	f.IntVar(&mapCfg.Delay, "delay", 500, "Delay in ms between requests")
	f.StringVar(&mapCfg.UserAgent, "user-agent", "rezz/1.0", "User-Agent string")
	f.BoolVar(&mapCfg.Headless, "headless", true, "Use headless browser")
	f.BoolVar(&mapNoHeadless, "no-headless", false, "Disable headless, use net/http")
	f.StringVar(&mapCfg.Scope, "scope", "", "In-scope hosts, comma-separated")
	f.BoolVar(&mapCfg.StrictScope, "strict-scope", false, "Exact host matching only")
	f.StringVar(&mapFormat, "format", "json", "Output format: stream|json")
	f.StringVar(&mapCfg.Output, "output", "", "Output file (default: stdout)")
	f.BoolVar(&mapCfg.Verbose, "verbose", false, "Verbose logging")
	f.BoolVar(&mapCfg.Quiet, "quiet", false, "Suppress banner")
	f.DurationVar(&mapCfg.Timeout, "timeout", 5*time.Minute, "Max crawl duration")
	f.BoolVar(&mapCfg.NoColor, "no-color", false, "Disable ANSI color output")
	f.BoolVar(&mapCfg.ForceColor, "color", false, "Force ANSI color output even when stdout is not a terminal")

	// Auth flags — same as root command
	f.StringVar(&mapCfg.AuthFormURL, "auth-form-url", "", "Login form URL")
	f.StringVar(&mapCfg.AuthFormUser, "auth-form-user", "", "Login username")
	f.StringVar(&mapCfg.AuthFormPass, "auth-form-pass", "", "Login password")
	f.StringVar(&mapCfg.AuthCookie, "auth-cookie", "", "Cookie string to inject")
	f.StringVar(&mapCfg.AuthBearer, "auth-bearer", "", "Bearer token")
	f.StringVar(&mapCfg.AuthHeader, "auth-header", "", "Custom auth header")

	mapCmd.MarkFlagRequired("url") //nolint:errcheck
	rootCmd.AddCommand(mapCmd)
}

func runMap(cmd *cobra.Command, args []string) error {
	if mapNoHeadless {
		mapCfg.Headless = false
	}

	if mapCfg.NoColor {
		color.NoColor = true
	} else if mapCfg.ForceColor {
		color.NoColor = false
	}

	if !mapCfg.Quiet {
		fmt.Fprint(os.Stderr, banner)
	}

	eng, err := scope.New(mapCfg.Scope, mapCfg.URL, mapCfg.StrictScope)
	if err != nil {
		return fmt.Errorf("scope: %w", err)
	}

	base, err := url.Parse(mapCfg.URL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	_ = base

	var w io.Writer = os.Stdout
	if mapCfg.Output != "" {
		f, err := os.Create(mapCfg.Output)
		if err != nil {
			return fmt.Errorf("opening output file: %w", err)
		}
		defer f.Close()
		w = f
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if mapCfg.Timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, mapCfg.Timeout)
		defer cancel()
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

	c := crawler.New(mapCfg)
	pages, err := c.Crawl(ctx, mapCfg)
	if err != nil {
		return fmt.Errorf("starting crawl: %w", err)
	}

	em := extractor.NewEndpointMap()
	var pagesCrawled int

	g, _ := errgroup.WithContext(ctx)
	g.Go(func() error {
		for page := range pages {
			if page.IsScript {
				em.Add([]extractor.Endpoint{{
					Type:    extractor.TypeScript,
					URL:     page.URL,
					FoundIn: "crawler",
				}})
				pageURL, _ := url.Parse(page.URL)
				discovered := extractor.Extract(page.HTML, page.URL, pageURL)
				for i := range discovered {
					if !eng.InScope(discovered[i].URL) {
						discovered[i].Type = extractor.TypeOutOfScope
					}
				}
				em.Add(discovered)
				if mapFormat == "stream" && !mapCfg.Quiet {
					scriptColor := color.New(color.FgYellow)
					fmt.Fprintf(w, "[%s]       %s\n", scriptColor.Sprint("SCRIPT"), page.URL)
					for _, e := range discovered {
						printEndpoint(w, e)
					}
				}
				pagesCrawled++
				continue
			}

			em.AddCrawled(page.URL, 200)
			pageURL, _ := url.Parse(page.URL)
			discovered := extractor.Extract(page.HTML, page.URL, pageURL)

			// Classify out-of-scope endpoints
			for i := range discovered {
				if discovered[i].Type != extractor.TypeSensitive &&
					!eng.InScope(discovered[i].URL) {
					discovered[i].Type = extractor.TypeOutOfScope
				}
			}

			em.Add(discovered)

			if mapFormat == "stream" && !mapCfg.Quiet {
				for _, e := range discovered {
					printEndpoint(w, e)
				}
			}

			pagesCrawled++
			if mapCfg.Verbose {
				fmt.Fprintf(os.Stderr, "rezz: mapped %s — %d endpoints\n",
					page.URL, len(discovered))
			}
		}
		return nil
	})

	if err := g.Wait(); err != nil {
		return fmt.Errorf("pipeline: %w", err)
	}

	switch mapFormat {
	case "json":
		payload := struct {
			Target       string               `json:"target"`
			PagesCrawled int                  `json:"pages_crawled"`
			TotalFound   int                  `json:"total_found"`
			Endpoints    []extractor.Endpoint `json:"endpoints"`
		}{
			Target:       mapCfg.URL,
			PagesCrawled: pagesCrawled,
			TotalFound:   len(em.Endpoints),
			Endpoints:    em.Endpoints,
		}
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		if err := enc.Encode(payload); err != nil {
			return fmt.Errorf("writing output: %w", err)
		}
	case "stream":
		// already printed above
	default:
		return fmt.Errorf("unknown format %q: must be stream or json", mapFormat)
	}

	fmt.Fprintf(os.Stderr, "rezz: map complete — %d pages crawled, %d endpoints found\n",
		pagesCrawled, len(em.Endpoints))
	return nil
}

func printEndpoint(w io.Writer, e extractor.Endpoint) {
	type labelDef struct {
		text  string
		style *color.Color
	}
	labels := map[extractor.EndpointType]labelDef{
		extractor.TypeCrawled:    {"CRAWLED    ", color.New(color.FgGreen)},
		extractor.TypeForm:       {"FORM       ", color.New(color.FgCyan, color.Bold)},
		extractor.TypeJSEndpoint: {"JS-ENDPOINT", color.New(color.FgYellow, color.Bold)},
		extractor.TypeScript:     {"SCRIPT     ", color.New(color.FgYellow)},
		extractor.TypeAsset:      {"ASSET      ", color.New(color.FgHiBlack)},
		extractor.TypeOutOfScope: {"OUT-OF-SCOPE", color.New(color.FgHiBlack)},
		extractor.TypeSensitive:  {"SENSITIVE  ", color.New(color.FgRed, color.Bold)},
	}

	dim := color.New(color.FgHiBlack)
	ld, ok := labels[e.Type]
	if !ok {
		ld = labelDef{string(e.Type), color.New(color.Reset)}
	}

	switch e.Type {
	case extractor.TypeCrawled:
		fmt.Fprintf(w, "[%s] %3d  %s\n",
			ld.style.Sprint(ld.text), e.StatusCode, e.URL)
	case extractor.TypeForm:
		fmt.Fprintf(w, "[%s] %s %s",
			ld.style.Sprint(ld.text), e.Method, e.URL)
		if len(e.Fields) > 0 {
			fmt.Fprintf(w, "  %s", dim.Sprintf("fields: %s",
				strings.Join(e.Fields, ", ")))
		}
		fmt.Fprintln(w)
	default:
		fmt.Fprintf(w, "[%s] %s  %s\n",
			ld.style.Sprint(ld.text),
			e.URL,
			dim.Sprintf("found in: %s", e.FoundIn))
	}
}
