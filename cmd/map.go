package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"time"

	"github.com/fatih/color"
	"github.com/RowanDark/rezz/internal/config"
	"github.com/RowanDark/rezz/internal/crawler"
	"github.com/RowanDark/rezz/internal/extractor"
	"github.com/RowanDark/rezz/internal/fingerprint"
	"github.com/RowanDark/rezz/internal/patterns"
	"github.com/RowanDark/rezz/internal/scope"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
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
	f.Float64Var(&mapCfg.Jitter, "jitter", 0.5,
		"Random jitter factor applied to --delay (0 = no jitter, 1.0 = up to 2x delay)")
	f.BoolVar(&mapCfg.NoRobots, "no-robots", false, "Skip robots.txt fetching and checking")
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
	f.BoolVar(&mapCfg.NoProbe, "no-probe", false, "Disable HEAD probing of discovered endpoints")

	// Auth flags — same as root command
	f.StringVar(&mapCfg.AuthFormURL, "auth-form-url", "", "Login form URL")
	f.StringVar(&mapCfg.AuthFormUser, "auth-form-user", "", "Login username")
	f.StringVar(&mapCfg.AuthFormPass, "auth-form-pass", "", "Login password")
	f.StringVar(&mapCfg.AuthCookie, "auth-cookie", "", "Cookie string to inject")
	f.StringVar(&mapCfg.AuthBearer, "auth-bearer", "", "Bearer token")
	f.StringVar(&mapCfg.AuthHeader, "auth-header", "", "Custom auth header")
	f.StringVar(&mapCfg.Patterns, "patterns", "endpoints,javascript",
		"Pattern kits for endpoint classification (api-keys,credentials,endpoints,financial,javascript,headers,cloud,all)")
	f.StringVar(&mapCfg.CustomFile, "custom", "",
		"Path to a custom patterns YAML file")

	mapCmd.MarkFlagRequired("url") //nolint:errcheck
	rootCmd.AddCommand(mapCmd)
}

func runMap(cmd *cobra.Command, args []string) error {
	if mapCfg.Jitter < 0 {
		return fmt.Errorf("--jitter must be >= 0 (got %.2f)", mapCfg.Jitter)
	}
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

	mapLog := zap.NewNop()
	if mapCfg.Verbose {
		mapLog, _ = zap.NewProduction()
	}
	kitNames := strings.Split(mapCfg.Patterns, ",")
	for i, k := range kitNames {
		kitNames[i] = strings.TrimSpace(k)
	}
	reg := patterns.New(mapLog)
	if err := reg.Load(kitNames); err != nil {
		return fmt.Errorf("patterns: %w", err)
	}
	if mapCfg.CustomFile != "" {
		if err := reg.LoadCustom(mapCfg.CustomFile); err != nil {
			return fmt.Errorf("custom patterns: %w", err)
		}
	}
	_ = reg // patterns registry available for future endpoint classification

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
	fp := fingerprint.NewProfile()
	pm := extractor.NewParamMap()
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
				for _, t := range fingerprint.FromHTML(page.HTML) {
					fp.Add(t)
				}
				pm.AddFromURL(page.URL)
				for _, e := range discovered {
					pm.AddFromURL(e.URL)
					if e.Type == extractor.TypeForm {
						pm.AddFromFields(e.Fields)
					}
				}
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

			for _, t := range fingerprint.FromHTML(page.HTML) {
				fp.Add(t)
			}
			pm.AddFromURL(page.URL)
			for _, e := range discovered {
				pm.AddFromURL(e.URL)
				if e.Type == extractor.TypeForm {
					pm.AddFromFields(e.Fields)
				}
			}

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

	if !mapCfg.NoProbe {
		if mapCfg.Verbose {
			fmt.Fprintf(os.Stderr, "rezz: probing %d endpoints...\n", len(em.Endpoints))
		}
		probeEndpoints(em.Endpoints, eng, mapCfg.UserAgent, mapCfg.Workers, mapCfg.Verbose)
	}

	if len(fp.Technologies) > 0 {
		fmt.Fprintln(os.Stderr, "── Technologies detected ──────────────")
		for _, t := range fp.Technologies {
			fmt.Fprintf(os.Stderr, "  %-22s [%s]\n", t.Name, t.Category)
		}
	}

	params := pm.Params()
	if len(params) > 0 {
		fmt.Fprintln(os.Stderr, "── Parameters found ───────────────────")
		fmt.Fprintf(os.Stderr, "  %s\n", strings.Join(params, ", "))
	}

	techs := fp.Technologies
	if techs == nil {
		techs = []fingerprint.Tech{}
	}
	if params == nil {
		params = []string{}
	}

	switch mapFormat {
	case "json":
		payload := struct {
			Target       string               `json:"target"`
			PagesCrawled int                  `json:"pages_crawled"`
			TotalFound   int                  `json:"total_found"`
			Technologies []fingerprint.Tech   `json:"technologies"`
			Parameters   []string             `json:"parameters"`
			Endpoints    []extractor.Endpoint `json:"endpoints"`
		}{
			Target:       mapCfg.URL,
			PagesCrawled: pagesCrawled,
			TotalFound:   len(em.Endpoints),
			Technologies: techs,
			Parameters:   params,
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

func statusColor(code int) string {
	c := color.New(color.FgGreen)
	switch {
	case code == 0:
		return "   "
	case code >= 500:
		c = color.New(color.FgRed, color.Bold)
	case code >= 400:
		c = color.New(color.FgYellow)
	case code >= 300:
		c = color.New(color.FgCyan)
	}
	return c.Sprintf("%3d", code)
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

	statusStr := statusColor(e.StatusCode)

	switch e.Type {
	case extractor.TypeCrawled:
		fmt.Fprintf(w, "[%s] %s  %s\n",
			ld.style.Sprint(ld.text), statusStr, e.URL)
	case extractor.TypeForm:
		fmt.Fprintf(w, "[%s] %s %s %s", ld.style.Sprint(ld.text), statusStr, e.Method, e.URL)
		if len(e.Fields) > 0 {
			fmt.Fprintf(w, "  %s", dim.Sprintf("fields: %s",
				strings.Join(e.Fields, ", ")))
		}
		fmt.Fprintln(w)
	default:
		fmt.Fprintf(w, "[%s] %s %s  %s\n",
			ld.style.Sprint(ld.text), statusStr,
			e.URL,
			dim.Sprintf("found in: %s", e.FoundIn))
	}
}

func probeEndpoints(endpoints []extractor.Endpoint, eng *scope.Engine, userAgent string, workers int, verbose bool) {
	type job struct {
		idx int
		url string
	}
	jobs := make(chan job, len(endpoints))
	for i, e := range endpoints {
		if e.StatusCode != 0 {
			continue
		}
		if e.Type == extractor.TypeOutOfScope {
			continue
		}
		if e.Type == extractor.TypeAsset {
			continue
		}
		if !eng.InScope(e.URL) {
			continue
		}
		jobs <- job{idx: i, url: e.URL}
	}
	close(jobs)

	client := &http.Client{
		Timeout: 5 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	if workers <= 0 {
		workers = 10
	}

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				req, err := http.NewRequest(http.MethodHead, j.url, nil)
				if err != nil {
					continue
				}
				req.Header.Set("User-Agent", userAgent)
				resp, err := client.Do(req)
				if err != nil {
					continue
				}
				resp.Body.Close()
				mu.Lock()
				endpoints[j.idx].StatusCode = resp.StatusCode
				mu.Unlock()
				if verbose {
					fmt.Fprintf(os.Stderr, "rezz: probe %d %s\n", resp.StatusCode, j.url)
				}
			}
		}()
	}
	wg.Wait()
}
