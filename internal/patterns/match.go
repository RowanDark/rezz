package patterns

import (
	"fmt"
	"strings"
	"time"
)

// Match runs all loaded patterns against html content and headers string.
// statusCode and fromJS are passed through to each Finding.
func (r *Registry) Match(html, headersStr, pageURL string, statusCode int, fromJS bool) []Finding {
	var findings []Finding
	for _, p := range r.patterns {
		findings = append(findings, r.matchContent(p, html, pageURL, statusCode, fromJS)...)
		findings = append(findings, r.matchContent(p, headersStr, pageURL+" (headers)", statusCode, fromJS)...)
	}
	return findings
}

func (r *Registry) matchContent(p Pattern, content, sourceURL string, statusCode int, fromJS bool) []Finding {
	var findings []Finding
	locs := p.compiled.FindAllStringIndex(content, -1)
	for _, loc := range locs {
		start, end := loc[0], loc[1]
		matchText := content[start:end]
		if len(matchText) > 200 {
			matchText = matchText[:200]
		}
		ctxStart := start - 100
		if ctxStart < 0 {
			ctxStart = 0
		}
		ctxEnd := end + 100
		if ctxEnd > len(content) {
			ctxEnd = len(content)
		}
		context := strings.Join(strings.Fields(content[ctxStart:ctxEnd]), " ")
		findings = append(findings, Finding{
			URL:        sourceURL,
			Pattern:    p.Name,
			Category:   p.Category,
			Severity:   p.Severity,
			Match:      matchText,
			Context:    context,
			StatusCode: statusCode,
			Timestamp:  time.Now(),
			FromJS:     fromJS,
		})
	}
	return findings
}

// headersToString flattens a map[string][]string into "Key: Value\n" lines.
func headersToString(headers map[string][]string) string {
	var sb strings.Builder
	for k, vs := range headers {
		for _, v := range vs {
			fmt.Fprintf(&sb, "%s: %s\n", k, v)
		}
	}
	return sb.String()
}
