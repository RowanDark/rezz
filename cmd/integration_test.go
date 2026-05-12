//go:build integration

package cmd_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/RowanDark/v0x/internal/config"
	"github.com/RowanDark/v0x/internal/crawler"
	"github.com/RowanDark/v0x/internal/extractor"
	"github.com/RowanDark/v0x/internal/output"
)

// TestSmokeCrawlPipeline spins up a minimal two-page HTTP server and asserts
// that the full crawler → extractor → formatter pipeline produces valid output.
func TestSmokeCrawlPipeline(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, `<!DOCTYPE html>
<html>
<head><meta name="description" content="smoke test index"></head>
<body>
  <h1>Welcome to the smoke test</h1>
  <p>This page contains words for the wordlist generator.</p>
  <a href="/about">About page</a>
</body>
</html>`)
	})
	mux.HandleFunc("/about", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, `<!DOCTYPE html>
<html>
<body>
  <h1>About page</h1>
  <p>Additional unique words appear here for extraction testing.</p>
</body>
</html>`)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	cfg := config.Config{
		URL:           srv.URL,
		Depth:         2,
		MinWordLength: 3,
		UserAgent:     "v0x-integration-test/1.0",
		Headless:      false,
		Delay:         0,
	}

	ctx := context.Background()
	c := crawler.New(cfg)
	pages, err := c.Crawl(ctx, cfg)
	if err != nil {
		t.Fatalf("Crawl() error: %v", err)
	}

	agg := extractor.NewAggregator()
	pagesCrawled := 0
	for page := range pages {
		r := extractor.Extract(page.HTML, cfg)
		agg.Add(r)
		pagesCrawled++
	}

	if pagesCrawled < 2 {
		t.Errorf("expected at least 2 pages crawled, got %d", pagesCrawled)
	}

	result := agg.Finalize()

	if len(result.Words) == 0 {
		t.Fatal("expected word count > 0, got 0")
	}
	t.Logf("crawled %d pages, extracted %d words", pagesCrawled, len(result.Words))

	meta := output.OutputMeta{TargetURL: srv.URL, PagesCrawled: pagesCrawled}

	t.Run("txt format", func(t *testing.T) {
		f, err := output.New("txt")
		if err != nil {
			t.Fatalf("output.New(txt): %v", err)
		}
		var buf strings.Builder
		if err := f.Write(&buf, result, meta); err != nil {
			t.Fatalf("Write: %v", err)
		}
		if buf.Len() == 0 {
			t.Error("txt output is empty")
		}
	})

	t.Run("json format", func(t *testing.T) {
		f, err := output.New("json")
		if err != nil {
			t.Fatalf("output.New(json): %v", err)
		}
		var buf strings.Builder
		if err := f.Write(&buf, result, meta); err != nil {
			t.Fatalf("Write: %v", err)
		}
		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(buf.String()), &parsed); err != nil {
			t.Fatalf("json output is invalid: %v", err)
		}
		words, ok := parsed["words"]
		if !ok {
			t.Fatal("json output missing 'words' field")
		}
		if ws, ok := words.([]interface{}); !ok || len(ws) == 0 {
			t.Error("json 'words' field is empty or wrong type")
		}
	})

	t.Run("csv format", func(t *testing.T) {
		f, err := output.New("csv")
		if err != nil {
			t.Fatalf("output.New(csv): %v", err)
		}
		var buf strings.Builder
		if err := f.Write(&buf, result, meta); err != nil {
			t.Fatalf("Write: %v", err)
		}
		lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
		// Must have at least a header row + one data row.
		if len(lines) < 2 {
			t.Errorf("csv output has too few lines: %d", len(lines))
		}
	})

	t.Run("md format", func(t *testing.T) {
		f, err := output.New("md")
		if err != nil {
			t.Fatalf("output.New(md): %v", err)
		}
		var buf strings.Builder
		if err := f.Write(&buf, result, meta); err != nil {
			t.Fatalf("Write: %v", err)
		}
		if !strings.Contains(buf.String(), "# ") {
			t.Error("markdown output missing heading")
		}
	})
}
