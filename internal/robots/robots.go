package robots

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Checker fetches and parses robots.txt for a given host.
// If robots.txt cannot be fetched, all URLs are allowed (fail open).
type Checker struct {
	disallowed []string // path prefixes that are disallowed for *
	host       string
}

// New fetches robots.txt from the seed URL's host and parses it.
// Returns a Checker that can be queried with Allowed().
// If robots.txt returns non-200 or times out, returns a permissive checker
// that allows all paths.
func New(seedURL, userAgent string) *Checker {
	u, err := url.Parse(seedURL)
	if err != nil {
		return &Checker{}
	}
	host := u.Scheme + "://" + u.Host
	robotsURL := host + "/robots.txt"

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest(http.MethodGet, robotsURL, nil)
	if err != nil {
		return &Checker{host: host}
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		return &Checker{host: host}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return &Checker{host: host}
	}

	return parse(string(body), host)
}

// NewPermissive returns a Checker that allows all URLs.
// Used when --no-robots is set.
func NewPermissive() *Checker {
	return &Checker{}
}

// parse extracts Disallow directives for User-agent: * from robots.txt content.
func parse(content, host string) *Checker {
	c := &Checker{host: host}
	inWildcard := false

	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			inWildcard = false
			continue
		}
		// Strip inline comments
		if idx := strings.Index(line, "#"); idx >= 0 {
			line = strings.TrimSpace(line[:idx])
		}
		if line == "" {
			continue
		}

		lower := strings.ToLower(line)
		if strings.HasPrefix(lower, "user-agent:") {
			agent := strings.TrimSpace(line[11:])
			inWildcard = agent == "*"
			continue
		}
		if inWildcard && strings.HasPrefix(lower, "disallow:") {
			path := strings.TrimSpace(line[9:])
			if path != "" {
				c.disallowed = append(c.disallowed, path)
			}
		}
	}
	return c
}

// Allowed returns true if the given URL is allowed to be crawled.
// If robots.txt could not be fetched, always returns true.
func (c *Checker) Allowed(rawURL string) bool {
	if len(c.disallowed) == 0 {
		return true
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return true
	}
	path := u.Path
	if path == "" {
		path = "/"
	}
	for _, disallowed := range c.disallowed {
		if strings.HasPrefix(path, disallowed) {
			return false
		}
	}
	return true
}
