package fingerprint

import "strings"

type Tech struct {
	Name     string `json:"name"`
	Category string `json:"category"` // framework, server, cms, language, cdn, analytics, security
	Evidence string `json:"evidence"`
}

type Profile struct {
	Technologies []Tech
	seen         map[string]struct{}
}

func NewProfile() *Profile { return &Profile{seen: make(map[string]struct{})} }

func (p *Profile) Add(t Tech) {
	if _, exists := p.seen[t.Name]; exists {
		return
	}
	p.seen[t.Name] = struct{}{}
	p.Technologies = append(p.Technologies, t)
}

func FromHeaders(headers map[string]string) []Tech {
	var techs []Tech
	checks := []struct{ header, keyword, name, cat string }{
		{"Server", "nginx", "nginx", "server"},
		{"Server", "Apache", "Apache", "server"},
		{"Server", "IIS", "IIS", "server"},
		{"Server", "Jetty", "Jetty", "server"},
		{"Server", "Tomcat", "Tomcat", "server"},
		{"Server", "cloudflare", "Cloudflare", "cdn"},
		{"X-Powered-By", "PHP", "PHP", "language"},
		{"X-Powered-By", "ASP.NET", "ASP.NET", "framework"},
		{"X-Powered-By", "Express", "Express.js", "framework"},
		{"X-Powered-By", "Next.js", "Next.js", "framework"},
		{"CF-Ray", "", "Cloudflare", "cdn"},
		{"X-Shopify-Stage", "", "Shopify", "cms"},
	}
	for _, c := range checks {
		for k, v := range headers {
			if !strings.EqualFold(k, c.header) {
				continue
			}
			if c.keyword == "" || strings.Contains(strings.ToLower(v), strings.ToLower(c.keyword)) {
				techs = append(techs, Tech{Name: c.name, Category: c.cat, Evidence: k + ": " + v})
			}
		}
	}
	return techs
}

func FromHTML(html string) []Tech {
	var techs []Tech
	checks := []struct{ marker, name, cat string }{
		{"__NEXT_DATA__", "Next.js", "framework"},
		{"__nuxt__", "Nuxt.js", "framework"},
		{"ng-version=", "Angular", "framework"},
		{"data-reactroot", "React", "framework"},
		{"data-v-app", "Vue.js", "framework"},
		{"wp-content", "WordPress", "cms"},
		{"Drupal.settings", "Drupal", "cms"},
		{"google-analytics.com", "Google Analytics", "analytics"},
		{"googletagmanager.com", "Google Tag Manager", "analytics"},
		{"hotjar.com", "Hotjar", "analytics"},
		{"iovation", "iovation", "security"},
		{"recaptcha", "reCAPTCHA", "security"},
		{"hcaptcha", "hCaptcha", "security"},
		{"cdn.jsdelivr.net", "jsDelivr CDN", "cdn"},
	}
	lower := strings.ToLower(html)
	for _, c := range checks {
		if strings.Contains(lower, strings.ToLower(c.marker)) {
			techs = append(techs, Tech{Name: c.name, Category: c.cat, Evidence: "marker: " + c.marker})
		}
	}
	return techs
}
