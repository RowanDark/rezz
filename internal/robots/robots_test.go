package robots

import "testing"

func TestParseDisallow(t *testing.T) {
	content := `
User-agent: *
Disallow: /admin
Disallow: /private/
Disallow: /.git

User-agent: Googlebot
Disallow: /nogoogle
`
	c := parse(content, "https://example.com")

	if c.Allowed("https://example.com/admin/users") {
		t.Error("/admin/users should be disallowed")
	}
	if c.Allowed("https://example.com/private/data") {
		t.Error("/private/data should be disallowed")
	}
	if c.Allowed("https://example.com/.git/config") {
		t.Error("/.git/config should be disallowed")
	}
	if !c.Allowed("https://example.com/public/page") {
		t.Error("/public/page should be allowed")
	}
	// Googlebot-only rule should not affect wildcard checker
	if !c.Allowed("https://example.com/nogoogle") {
		t.Error("/nogoogle should be allowed for wildcard agent")
	}
}

func TestPermissiveOnEmpty(t *testing.T) {
	c := parse("", "https://example.com")
	if !c.Allowed("https://example.com/anything") {
		t.Error("empty robots.txt should allow everything")
	}
}

func TestAllowAll(t *testing.T) {
	content := `User-agent: *
Allow: /`
	c := parse(content, "https://example.com")
	if !c.Allowed("https://example.com/secret") {
		t.Error("allow-all robots.txt should allow everything")
	}
}
