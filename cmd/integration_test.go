//go:build integration

package cmd_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/RowanDark/rezz/internal/config"
	"github.com/RowanDark/rezz/internal/crawler"
	"github.com/RowanDark/rezz/internal/patterns"
	"go.uber.org/zap"
)

func TestSmokeScanPipeline(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, `<!DOCTYPE html>
<html>
<head><title>Test</title></head>
<body>
  <pre>aws_access_key_id = AKIAIOSFODNN7EXAMPLE1</pre>
  <p>contact: test@example.com</p>
  <a href="/about">About</a>
</body>
</html>`)
	})
	mux.HandleFunc("/about", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, `<!DOCTYPE html>
<html>
<body><h1>About page</h1></body>
</html>`)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	cfg := config.Config{
		URL:       srv.URL,
		Depth:     1,
		UserAgent: "rezz-integration-test/1.0",
		Headless:  false,
		Delay:     0,
	}

	ctx := context.Background()
	c := crawler.New(cfg)
	pages, err := c.Crawl(ctx, cfg)
	if err != nil {
		t.Fatalf("Crawl() error: %v", err)
	}

	reg := patterns.New(zap.NewNop())
	if err := reg.Load([]string{"api-keys"}); err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	var allFindings []patterns.Finding
	pagesCrawled := 0
	for page := range pages {
		findings := reg.Match(page.HTML, "", page.URL, 200, false)
		allFindings = append(allFindings, findings...)
		pagesCrawled++
	}

	if pagesCrawled < 2 {
		t.Errorf("expected at least 2 pages crawled, got %d", pagesCrawled)
	}
	if len(allFindings) == 0 {
		t.Error("expected at least one finding (AWS key in test page), got 0")
	}
	t.Logf("crawled %d pages, found %d findings", pagesCrawled, len(allFindings))
}
