package fingerprint

import "testing"

func TestFromHTMLDetectsReact(t *testing.T) {
	html := `<div data-reactroot></div>`
	techs := FromHTML(html)
	for _, tech := range techs {
		if tech.Name == "React" {
			return
		}
	}
	t.Error("expected React to be detected")
}

func TestFromHTMLDetectsWordPress(t *testing.T) {
	html := `<link rel="stylesheet" href="/wp-content/themes/mytheme/style.css">`
	techs := FromHTML(html)
	for _, tech := range techs {
		if tech.Name == "WordPress" {
			return
		}
	}
	t.Error("expected WordPress to be detected")
}

func TestFromHeadersDetectsNginx(t *testing.T) {
	headers := map[string]string{"Server": "nginx/1.21.0"}
	techs := FromHeaders(headers)
	for _, tech := range techs {
		if tech.Name == "nginx" {
			return
		}
	}
	t.Error("expected nginx to be detected")
}

func TestFromHeadersDetectsPHP(t *testing.T) {
	headers := map[string]string{"X-Powered-By": "PHP/8.1.0"}
	techs := FromHeaders(headers)
	for _, tech := range techs {
		if tech.Name == "PHP" {
			return
		}
	}
	t.Error("expected PHP to be detected")
}
