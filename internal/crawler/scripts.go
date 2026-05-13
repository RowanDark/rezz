package crawler

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/RowanDark/rezz/internal/scope"
)

// scriptExts is the set of file extensions we fetch as script/style resources.
var scriptExts = []string{".js", ".mjs", ".jsx", ".ts", ".css"}

// isScriptURL returns true if the URL path ends with a script or stylesheet extension.
func isScriptURL(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	path := strings.ToLower(strings.Split(u.Path, "?")[0])
	for _, ext := range scriptExts {
		if strings.HasSuffix(path, ext) {
			return true
		}
	}
	return false
}

// extractScriptSrcs parses script src and link href attributes from HTML
// and returns in-scope, unvisited script URLs.
// visited is checked via Load() only — callers must store with LoadOrStore.
func extractScriptSrcs(html, pageURL string, base *url.URL, eng *scope.Engine, visited *sync.Map) []string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil
	}

	seen := map[string]struct{}{}
	var results []string

	collect := func(rawVal string) {
		if rawVal == "" {
			return
		}
		ref, err := url.Parse(rawVal)
		if err != nil {
			return
		}
		resolved := base.ResolveReference(ref)
		resolved.Fragment = ""
		if resolved.Scheme != "http" && resolved.Scheme != "https" {
			return
		}
		s := resolved.String()
		if !isScriptURL(s) {
			return
		}
		if !eng.InScope(s) {
			return
		}
		if _, loaded := visited.Load(s); loaded {
			return
		}
		if _, exists := seen[s]; exists {
			return
		}
		seen[s] = struct{}{}
		results = append(results, s)
	}

	doc.Find("script[src]").Each(func(_ int, s *goquery.Selection) {
		val, _ := s.Attr("src")
		collect(val)
	})

	doc.Find("link[rel~=stylesheet][href]").Each(func(_ int, s *goquery.Selection) {
		val, _ := s.Attr("href")
		collect(val)
	})

	return results
}

// fetchScript fetches a single script URL using a plain HTTP client.
// Returns a *Page with IsScript=true, or nil on error.
func fetchScript(rawURL, userAgent string) (*Page, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 5<<20)) // 5MB cap
	if err != nil {
		return nil, err
	}

	return &Page{
		URL:      rawURL,
		HTML:     string(body),
		Depth:    0,
		IsScript: true,
	}, nil
}
