package scope

import (
	"net/url"
	"strings"
)

// Engine determines whether a given URL is within the defined scope.
type Engine struct {
	entries []entry
	strict  bool
}

type entry struct {
	host   string // lowercase, no port
	isBase bool   // true if no subdomain prefix (e.g. "paylution.com" not "www.paylution.com")
}

// New builds a scope engine from a comma-separated scope string and strict flag.
// If scopeStr is empty, seedURL's host is used as the sole exact-match entry.
func New(scopeStr string, seedURL string, strict bool) (*Engine, error) {
	e := &Engine{strict: strict}

	if scopeStr == "" {
		u, err := url.Parse(seedURL)
		if err != nil {
			return nil, err
		}
		host := strings.ToLower(u.Hostname())
		e.entries = []entry{{host: host, isBase: false}}
		return e, nil
	}

	for _, part := range strings.Split(scopeStr, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		// Heuristic: a base domain has at most one dot (e.g. "paylution.com").
		// A subdomain entry has more than one dot (e.g. "www.paylution.com").
		isBase := strings.Count(part, ".") == 1
		e.entries = append(e.entries, entry{
			host:   strings.ToLower(part),
			isBase: isBase,
		})
	}

	return e, nil
}

// InScope returns true if the given rawURL is within scope.
func (e *Engine) InScope(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname()) // strips port

	for _, entry := range e.entries {
		if e.strict || !entry.isBase {
			if host == entry.host {
				return true
			}
		} else {
			// Base domain: match exact or any subdomain.
			if host == entry.host || strings.HasSuffix(host, "."+entry.host) {
				return true
			}
		}
	}
	return false
}
