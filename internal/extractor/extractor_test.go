package extractor

import (
	"strings"
	"testing"

	"github.com/RowanDark/v0x/internal/config"
)

var testCfg = config.Config{MinWordLength: 3}

func TestExtractWords(t *testing.T) {
	html := `<html><body><p>Hello World foo bar</p><script>var x=1;</script></body></html>`
	r := Extract(html, testCfg)
	wordSet := make(map[string]struct{})
	for _, w := range r.Words {
		wordSet[w] = struct{}{}
	}
	if _, ok := wordSet["hello"]; !ok {
		t.Error("expected 'hello'")
	}
	if _, ok := wordSet["world"]; !ok {
		t.Error("expected 'world'")
	}
	// script content should not appear
	if _, ok := wordSet["var"]; ok {
		t.Error("'var' from script should be excluded")
	}
}

func TestWordFilters(t *testing.T) {
	html := `<html><body><p>ab 12345 hello</p></body></html>`
	r := Extract(html, testCfg)
	wordSet := make(map[string]struct{})
	for _, w := range r.Words {
		wordSet[w] = struct{}{}
	}
	if _, ok := wordSet["ab"]; ok {
		t.Error("'ab' is below MinWordLength=3 and should be excluded")
	}
	if _, ok := wordSet["12345"]; ok {
		t.Error("all-digit token should be excluded")
	}
	if _, ok := wordSet["hello"]; !ok {
		t.Error("expected 'hello'")
	}
}

func TestExtractEmails(t *testing.T) {
	html := `<html><body>contact us at test@example.com or info@foo.org</body></html>`
	r := Extract(html, testCfg)
	emails := make(map[string]struct{})
	for _, e := range r.Emails {
		emails[e] = struct{}{}
	}
	if _, ok := emails["test@example.com"]; !ok {
		t.Error("expected test@example.com")
	}
	if _, ok := emails["info@foo.org"]; !ok {
		t.Error("expected info@foo.org")
	}
}

func TestExtractMeta(t *testing.T) {
	html := `<html><head>
		<meta name="description" content="A test page">
		<meta property="og:title" content="OG Title">
		<meta name="twitter:card" content="summary">
	</head><body></body></html>`
	r := Extract(html, testCfg)
	if r.Meta["description"] != "A test page" {
		t.Errorf("description: got %q", r.Meta["description"])
	}
	if r.Meta["og:title"] != "OG Title" {
		t.Errorf("og:title: got %q", r.Meta["og:title"])
	}
	if r.Meta["twitter:card"] != "summary" {
		t.Errorf("twitter:card: got %q", r.Meta["twitter:card"])
	}
}

func TestExtractJSONLD(t *testing.T) {
	html := `<html><head>
		<script type="application/ld+json">{"@type":"Person","name":"Alice","age":30}</script>
	</head><body></body></html>`
	r := Extract(html, testCfg)
	if r.Meta["Person.name"] != "Alice" {
		t.Errorf("Person.name: got %q", r.Meta["Person.name"])
	}
	if r.Meta["Person.age"] != "30" {
		t.Errorf("Person.age: got %q", r.Meta["Person.age"])
	}
}

func TestAdjacentBlockElements(t *testing.T) {
	html := `<html><body><h1>Foo Bar</h1><p>Baz Qux</p></body></html>`
	r := Extract(html, testCfg)
	wordSet := make(map[string]struct{})
	for _, w := range r.Words {
		wordSet[w] = struct{}{}
	}
	for _, expected := range []string{"foo", "bar", "baz", "qux"} {
		if _, ok := wordSet[expected]; !ok {
			t.Errorf("expected word %q in output", expected)
		}
	}
	for _, fused := range []string{"barfoo", "barbaz", "foobaz"} {
		if _, ok := wordSet[fused]; ok {
			t.Errorf("fused token %q must not appear in output", fused)
		}
	}
}

func TestAggregator(t *testing.T) {
	agg := NewAggregator()
	agg.Add(Result{
		Words:  []string{"hello", "world"},
		Emails: []string{"a@b.com"},
		Meta:   map[string]string{"title": "Page 1"},
	})
	agg.Add(Result{
		Words:  []string{"world", "foo"},
		Emails: []string{"a@b.com", "c@d.com"},
		Meta:   map[string]string{"title": "Page 2", "og:title": "OG"},
	})
	final := agg.Finalize()

	wordSet := make(map[string]struct{})
	for _, w := range final.Words {
		wordSet[w] = struct{}{}
	}
	for _, expected := range []string{"hello", "world", "foo"} {
		if _, ok := wordSet[expected]; !ok {
			t.Errorf("expected word %q in aggregated result", expected)
		}
	}
	if len(final.Words) != 3 {
		t.Errorf("expected 3 unique words, got %d", len(final.Words))
	}

	emailSet := make(map[string]struct{})
	for _, e := range final.Emails {
		emailSet[e] = struct{}{}
	}
	if len(emailSet) != 2 {
		t.Errorf("expected 2 unique emails, got %d", len(emailSet))
	}

	// first-writer wins for duplicate keys
	if final.Meta["title"] != "Page 1" {
		t.Errorf("title: got %q, want 'Page 1'", final.Meta["title"])
	}
	if final.Meta["og:title"] != "OG" {
		t.Errorf("og:title: got %q", final.Meta["og:title"])
	}
}

func TestAggregatorConcurrent(t *testing.T) {
	agg := NewAggregator()
	done := make(chan struct{})
	for i := 0; i < 50; i++ {
		go func(n int) {
			agg.Add(Result{
				Words:  []string{strings.Repeat("a", n+3)},
				Emails: []string{},
				Meta:   map[string]string{},
			})
			done <- struct{}{}
		}(i)
	}
	for i := 0; i < 50; i++ {
		<-done
	}
	final := agg.Finalize()
	if len(final.Words) != 50 {
		t.Errorf("expected 50 words, got %d", len(final.Words))
	}
}
