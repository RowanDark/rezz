package extractor

import (
	"net/url"
	"testing"
)

func TestExtractFormFields(t *testing.T) {
	base, _ := url.Parse("https://example.com")
	html := `
		<html><body>
		<form action="/login" method="POST">
			<input type="text" name="username" />
			<input type="password" name="password" />
			<input type="submit" value="Login" />
		</form>
		</body></html>
	`
	endpoints := Extract(html, "https://example.com/", base)
	var form *Endpoint
	for i := range endpoints {
		if endpoints[i].Type == TypeForm {
			form = &endpoints[i]
			break
		}
	}
	if form == nil {
		t.Fatal("expected a form endpoint")
	}
	if form.Method != "POST" {
		t.Errorf("expected POST, got %s", form.Method)
	}
	if len(form.Fields) != 2 {
		t.Errorf("expected 2 fields (submit excluded), got %d", len(form.Fields))
	}
}

func TestExtractJSEndpoints(t *testing.T) {
	base, _ := url.Parse("https://example.com")
	html := `<html><body>
	<script>
	fetch('/api/users');
	axios.get('/api/data');
	</script>
	</body></html>`

	endpoints := Extract(html, "https://example.com/", base)
	jsEndpoints := 0
	for _, e := range endpoints {
		if e.Type == TypeJSEndpoint {
			jsEndpoints++
		}
	}
	if jsEndpoints < 2 {
		t.Errorf("expected at least 2 JS endpoints, got %d", jsEndpoints)
	}
}

func TestSensitivePathClassification(t *testing.T) {
	base, _ := url.Parse("https://example.com")
	html := `
		<a href="/.git">git</a>
		<a href="/admin">admin</a>
	`
	endpoints := Extract(html, "https://example.com/", base)
	for _, e := range endpoints {
		if e.Type != TypeSensitive {
			t.Errorf("expected TypeSensitive for %s, got %s", e.URL, e.Type)
		}
	}
}

func TestSkipsDataAndJavascriptSchemes(t *testing.T) {
	base, _ := url.Parse("https://example.com")
	html := `
		<a href="javascript:void(0)">js</a>
		<a href="mailto:test@example.com">mail</a>
	`
	endpoints := Extract(html, "https://example.com/", base)
	if len(endpoints) != 0 {
		t.Errorf("expected 0 endpoints, got %d", len(endpoints))
	}
}
