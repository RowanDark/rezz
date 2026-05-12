package extractor

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"unicode"

	"github.com/PuerkitoBio/goquery"
	"github.com/RowanDark/v0x/internal/config"
)

var emailRegex = regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`)
var wordSplitter = regexp.MustCompile(`\W+`)

// Result holds extracted data from one or more pages.
type Result struct {
	Words      []string
	Emails     []string
	Meta       map[string]string
	WordSource map[string]string // "body", "meta", or "email"
}

// Aggregator merges Results from concurrent page extractions.
type Aggregator struct {
	mu         sync.Mutex
	words      map[string]struct{}
	emails     map[string]struct{}
	meta       map[string]string
	wordSource map[string]string
}

func NewAggregator() *Aggregator {
	return &Aggregator{
		words:      make(map[string]struct{}),
		emails:     make(map[string]struct{}),
		meta:       make(map[string]string),
		wordSource: make(map[string]string),
	}
}

func (a *Aggregator) Add(r Result) {
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, w := range r.Words {
		if _, exists := a.words[w]; !exists {
			a.words[w] = struct{}{}
			src := "body"
			if r.WordSource != nil {
				if s, ok := r.WordSource[w]; ok {
					src = s
				}
			}
			a.wordSource[w] = src
		}
	}
	for _, e := range r.Emails {
		a.emails[e] = struct{}{}
	}
	for k, v := range r.Meta {
		if _, exists := a.meta[k]; !exists {
			a.meta[k] = v
		}
	}
}

func (a *Aggregator) Finalize() Result {
	a.mu.Lock()
	defer a.mu.Unlock()
	words := make([]string, 0, len(a.words))
	for w := range a.words {
		words = append(words, w)
	}
	emails := make([]string, 0, len(a.emails))
	for e := range a.emails {
		emails = append(emails, e)
	}
	meta := make(map[string]string, len(a.meta))
	for k, v := range a.meta {
		meta[k] = v
	}
	wordSource := make(map[string]string, len(a.wordSource))
	for k, v := range a.wordSource {
		wordSource[k] = v
	}
	return Result{Words: words, Emails: emails, Meta: meta, WordSource: wordSource}
}

// Extract processes raw HTML and returns words, emails, and metadata.
func Extract(html string, cfg config.Config) Result {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return Result{Meta: make(map[string]string)}
	}

	// extractMeta MUST run before extractWords: extractWords removes <script>
	// nodes, which would destroy JSON-LD data before extractMeta can parse it.
	meta := extractMeta(doc)
	words := extractWords(doc, cfg.MinWordLength)
	emails := extractEmails(html)

	wordSource := make(map[string]string, len(words))
	for _, w := range words {
		wordSource[w] = "body"
	}

	return Result{Words: words, Emails: emails, Meta: meta, WordSource: wordSource}
}

func extractWords(doc *goquery.Document, minLen int) []string {
	doc.Find("script, style, noscript").Remove()

	seen := make(map[string]struct{})
	var words []string

	doc.Find("body").Each(func(_ int, s *goquery.Selection) {
		text := s.Text()
		for _, tok := range wordSplitter.Split(text, -1) {
			w := strings.ToLower(tok)
			if len(w) < minLen {
				continue
			}
			if isAllDigits(w) {
				continue
			}
			if _, exists := seen[w]; !exists {
				seen[w] = struct{}{}
				words = append(words, w)
			}
		}
	})

	return words
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

func extractEmails(html string) []string {
	matches := emailRegex.FindAllString(html, -1)
	seen := make(map[string]struct{}, len(matches))
	var emails []string
	for _, m := range matches {
		lower := strings.ToLower(m)
		if _, exists := seen[lower]; !exists {
			seen[lower] = struct{}{}
			emails = append(emails, lower)
		}
	}
	return emails
}

func extractMeta(doc *goquery.Document) map[string]string {
	meta := make(map[string]string)

	doc.Find("meta").Each(func(_ int, s *goquery.Selection) {
		if name, ok := s.Attr("name"); ok {
			if content, ok := s.Attr("content"); ok && content != "" {
				meta[strings.ToLower(name)] = content
			}
		}
		if prop, ok := s.Attr("property"); ok {
			if content, ok := s.Attr("content"); ok && content != "" {
				meta[strings.ToLower(prop)] = content
			}
		}
	})

	doc.Find(`script[type="application/ld+json"]`).Each(func(_ int, s *goquery.Selection) {
		raw := strings.TrimSpace(s.Text())
		if raw == "" {
			return
		}
		var obj map[string]interface{}
		if err := json.Unmarshal([]byte(raw), &obj); err != nil {
			return
		}
		flattenJSONLD("", obj, meta)
	})

	return meta
}

func flattenJSONLD(prefix string, obj map[string]interface{}, out map[string]string) {
	typeName := ""
	if t, ok := obj["@type"]; ok {
		switch v := t.(type) {
		case string:
			typeName = v
		case []interface{}:
			if len(v) > 0 {
				if s, ok := v[0].(string); ok {
					typeName = s
				}
			}
		}
	}

	keyBase := prefix
	if typeName != "" {
		if keyBase == "" {
			keyBase = typeName
		} else {
			keyBase = keyBase + "." + typeName
		}
	}

	for k, v := range obj {
		if k == "@type" || k == "@context" {
			continue
		}
		fullKey := k
		if keyBase != "" {
			fullKey = keyBase + "." + k
		}
		switch val := v.(type) {
		case string:
			if _, exists := out[fullKey]; !exists {
				out[fullKey] = val
			}
		case float64:
			if _, exists := out[fullKey]; !exists {
				out[fullKey] = fmt.Sprintf("%g", val)
			}
		case bool:
			if _, exists := out[fullKey]; !exists {
				if val {
					out[fullKey] = "true"
				} else {
					out[fullKey] = "false"
				}
			}
		case map[string]interface{}:
			flattenJSONLD(fullKey, val, out)
		case []interface{}:
			for i, item := range val {
				if nested, ok := item.(map[string]interface{}); ok {
					flattenJSONLD(fmt.Sprintf("%s[%d]", fullKey, i), nested, out)
				} else if s, ok := item.(string); ok {
					itemKey := fmt.Sprintf("%s[%d]", fullKey, i)
					if _, exists := out[itemKey]; !exists {
						out[itemKey] = s
					}
				}
			}
		}
	}
}
