package crawler

import (
	"testing"

	"github.com/RowanDark/rezz/internal/scope"
)

func TestExtractLinksStripsPathParameters(t *testing.T) {
	eng, _ := scope.New("", "https://example.com", false)

	html := `<html><body>
		<a href="/landing.xhtml">clean</a>
		<a href="/landing.xhtml;jsessionid=d177c31796c72fbe8c3a7bff18aa">session</a>
	</body></html>`

	links := extractLinks(html, "https://example.com/", eng, false)

	if len(links) != 1 {
		t.Fatalf("expected 1 deduplicated link, got %d: %v", len(links), links)
	}
	if links[0] != "https://example.com/landing.xhtml" {
		t.Errorf("expected https://example.com/landing.xhtml, got %s", links[0])
	}
}
