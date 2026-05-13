package extractor

import (
	"net/url"
	"regexp"
	"strings"
	"sync"

	"github.com/PuerkitoBio/goquery"
)

// EndpointType classifies a discovered URL reference.
type EndpointType string

const (
	TypeCrawled    EndpointType = "CRAWLED"
	TypeForm       EndpointType = "FORM"
	TypeJSEndpoint EndpointType = "JS-ENDPOINT"
	TypeAsset      EndpointType = "ASSET"
	TypeOutOfScope EndpointType = "OUT-OF-SCOPE"
	TypeSensitive  EndpointType = "SENSITIVE"
	TypeScript     EndpointType = "SCRIPT" // fetched JS/CSS resource
)

// Endpoint is a single discovered URL reference.
type Endpoint struct {
	Type       EndpointType `json:"type"`
	URL        string       `json:"url"`
	FoundIn    string       `json:"found_in"`
	Method     string       `json:"method,omitempty"`
	Fields     []string     `json:"fields,omitempty"`
	StatusCode int          `json:"status_code,omitempty"`
}

var sensitivePaths = []string{
	"/.git", "/.env", "/.svn", "/.htaccess", "/.htpasswd",
	"/wp-admin", "/phpinfo", "/server-status", "/server-info",
	"/admin", "/administrator", "/backend", "/console",
	"/api/swagger", "/api/docs", "/swagger", "/openapi",
	"/graphql", "/graphiql", "/__debug", "/debug",
	"/actuator", "/metrics", "/health", "/info",
	"/.well-known/security.txt",
}

var (
	reFetch      *regexp.Regexp
	reAxios      *regexp.Regexp
	reXHR        *regexp.Regexp
	reAPIPath    *regexp.Regexp
	reCSS        *regexp.Regexp
	reJSFAjax    *regexp.Regexp
	reJQueryAjax *regexp.Regexp
	reJQueryURL  *regexp.Regexp
	rePathLit    *regexp.Regexp
)

func init() {
	reFetch = regexp.MustCompile(`fetch\(\s*["` + "`" + `']([^"` + "`" + `']+)["` + "`" + `']`)
	reAxios = regexp.MustCompile(`axios\.[a-z]+\(\s*["` + "`" + `']([^"` + "`" + `']+)["` + "`" + `']`)
	reXHR = regexp.MustCompile(`\.open\(\s*["'][A-Z]+["']\s*,\s*["` + "`" + `']([^"` + "`" + `']+)["` + "`" + `']`)
	reAPIPath = regexp.MustCompile(`["` + "`" + `'](/(?:api|v[0-9]|graphql|rest|ws|webhook|oauth|auth|admin|internal)[^\s"` + "`" + `'#?]*)["` + "`" + `']`)
	reCSS = regexp.MustCompile(`url\(\s*["']?([^"')]+)["']?\s*\)`)
	reJSFAjax = regexp.MustCompile(`jsf\.ajax\.request\([^,]+,\s*[^,]+,\s*\{[^}]*action:\s*["']([^"']+)["']`)
	reJQueryAjax = regexp.MustCompile(`\$\.(?:ajax|get|post)\(\s*["` + "`" + `']([^"` + "`" + `']+)["` + "`" + `']`)
	reJQueryURL = regexp.MustCompile(`url:\s*["` + "`" + `']([^"` + "`" + `']+)["` + "`" + `']`)
	rePathLit = regexp.MustCompile(`["` + "`" + `']((?:/[a-zA-Z0-9_\-]+){2,})["` + "`" + `']`)
}

func isSensitivePath(path string) bool {
	lower := strings.ToLower(path)
	for _, sp := range sensitivePaths {
		if strings.HasPrefix(lower, strings.ToLower(sp)) {
			return true
		}
	}
	return false
}

func shouldSkip(rawURL string) bool {
	lower := strings.ToLower(strings.TrimSpace(rawURL))
	return strings.HasPrefix(lower, "data:") ||
		strings.HasPrefix(lower, "javascript:") ||
		strings.HasPrefix(lower, "mailto:") ||
		strings.HasPrefix(lower, "tel:")
}

func resolveURL(raw string, base *url.URL) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "#" {
		return "", false
	}
	if shouldSkip(raw) {
		return "", false
	}
	ref, err := url.Parse(raw)
	if err != nil {
		return "", false
	}
	resolved := base.ResolveReference(ref)
	// Strip fragment
	resolved.Fragment = ""
	return resolved.String(), true
}

func classifyURL(resolvedURL string, defaultType EndpointType) EndpointType {
	u, err := url.Parse(resolvedURL)
	if err != nil {
		return defaultType
	}
	if isSensitivePath(u.Path) {
		return TypeSensitive
	}
	return defaultType
}

// Extract extracts all endpoint references from html content.
// html is the rendered page content, pageURL is the source page, base is used
// for resolving relative URLs.
func Extract(html, pageURL string, base *url.URL) []Endpoint {
	type dedupKey struct {
		t      EndpointType
		u      string
		source string
	}
	seen := make(map[dedupKey]struct{})
	var results []Endpoint

	add := func(e Endpoint) {
		k := dedupKey{e.Type, e.URL, e.FoundIn}
		if _, exists := seen[k]; exists {
			return
		}
		seen[k] = struct{}{}
		results = append(results, e)
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return results
	}

	// 1a. a[href]
	doc.Find("a[href]").Each(func(_ int, s *goquery.Selection) {
		href, _ := s.Attr("href")
		resolved, ok := resolveURL(href, base)
		if !ok {
			return
		}
		t := classifyURL(resolved, TypeCrawled)
		add(Endpoint{Type: t, URL: resolved, FoundIn: pageURL})
	})

	// 1b. form
	doc.Find("form").Each(func(_ int, s *goquery.Selection) {
		action, _ := s.Attr("action")
		method := strings.ToUpper(s.AttrOr("method", "GET"))

		var targetURL string
		if action == "" || action == "#" {
			targetURL = pageURL
		} else {
			resolved, ok := resolveURL(action, base)
			if !ok {
				return
			}
			targetURL = resolved
		}

		var fields []string
		s.Find("input, select, textarea").Each(func(_ int, input *goquery.Selection) {
			inputType := strings.ToLower(input.AttrOr("type", "text"))
			switch inputType {
			case "submit", "button", "reset", "hidden":
				return
			}
			name, exists := input.Attr("name")
			if exists && name != "" {
				fields = append(fields, name)
			}
		})

		t := classifyURL(targetURL, TypeForm)
		add(Endpoint{
			Type:    t,
			URL:     targetURL,
			FoundIn: pageURL,
			Method:  method,
			Fields:  fields,
		})
	})

	// 1c. assets: script[src], link[href], img[src], iframe[src]
	assetSelectors := []struct {
		sel  string
		attr string
	}{
		{"script[src]", "src"},
		{"link[href]", "href"},
		{"img[src]", "src"},
		{"iframe[src]", "src"},
	}
	for _, as := range assetSelectors {
		as := as
		doc.Find(as.sel).Each(func(_ int, s *goquery.Selection) {
			val, _ := s.Attr(as.attr)
			resolved, ok := resolveURL(val, base)
			if !ok {
				return
			}
			t := classifyURL(resolved, TypeAsset)
			add(Endpoint{Type: t, URL: resolved, FoundIn: pageURL})
		})
	}

	// 1d. meta[http-equiv=refresh]
	doc.Find("meta").Each(func(_ int, s *goquery.Selection) {
		equiv := strings.ToLower(s.AttrOr("http-equiv", ""))
		if equiv != "refresh" {
			return
		}
		content, _ := s.Attr("content")
		lower := strings.ToLower(content)
		if idx := strings.Index(lower, "url="); idx >= 0 {
			rawURL := strings.TrimSpace(content[idx+4:])
			rawURL = strings.Trim(rawURL, "'\"")
			resolved, ok := resolveURL(rawURL, base)
			if !ok {
				return
			}
			t := classifyURL(resolved, TypeCrawled)
			add(Endpoint{Type: t, URL: resolved, FoundIn: pageURL})
		}
	})

	// 2. JS regex patterns on raw HTML string
	for _, re := range []*regexp.Regexp{reFetch, reAxios, reXHR, reAPIPath, reJSFAjax, reJQueryAjax, reJQueryURL, rePathLit} {
		matches := re.FindAllStringSubmatch(html, -1)
		for _, m := range matches {
			if len(m) < 2 {
				continue
			}
			resolved, ok := resolveURL(m[1], base)
			if !ok {
				continue
			}
			t := classifyURL(resolved, TypeJSEndpoint)
			add(Endpoint{Type: t, URL: resolved, FoundIn: pageURL})
		}
	}

	// 3. CSS url() references
	for _, m := range reCSS.FindAllStringSubmatch(html, -1) {
		if len(m) < 2 {
			continue
		}
		resolved, ok := resolveURL(m[1], base)
		if !ok {
			continue
		}
		t := classifyURL(resolved, TypeAsset)
		add(Endpoint{Type: t, URL: resolved, FoundIn: pageURL})
	}

	return results
}

// EndpointMap accumulates all discovered endpoints across a crawl.
// Safe for concurrent use.
type EndpointMap struct {
	mu        sync.Mutex
	seen      map[string]struct{} // dedup key: type+url+foundIn
	Endpoints []Endpoint
}

func NewEndpointMap() *EndpointMap {
	return &EndpointMap{seen: make(map[string]struct{})}
}

func (m *EndpointMap) Add(endpoints []Endpoint) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, e := range endpoints {
		key := string(e.Type) + "|" + e.URL + "|" + e.FoundIn
		if _, exists := m.seen[key]; exists {
			continue
		}
		m.seen[key] = struct{}{}
		m.Endpoints = append(m.Endpoints, e)
	}
}

func (m *EndpointMap) AddCrawled(pageURL string, statusCode int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := string(TypeCrawled) + "|" + pageURL + "|"
	if _, exists := m.seen[key]; exists {
		return
	}
	m.seen[key] = struct{}{}
	m.Endpoints = append(m.Endpoints, Endpoint{
		Type:       TypeCrawled,
		URL:        pageURL,
		StatusCode: statusCode,
	})
}
