package scope

import "testing"

func TestDefaultScope(t *testing.T) {
	e, _ := New("", "https://www.paylution.com", false)
	if !e.InScope("https://www.paylution.com/login") {
		t.Error("same host should be in scope")
	}
	if e.InScope("https://api.paylution.com") {
		t.Error("subdomain should be out of scope when no --scope flag")
	}
	if e.InScope("https://facebook.com") {
		t.Error("external domain must be out of scope")
	}
}

func TestBaseDomainScope(t *testing.T) {
	e, _ := New("paylution.com", "https://www.paylution.com", false)
	if !e.InScope("https://www.paylution.com") {
		t.Error("www subdomain should match base domain")
	}
	if !e.InScope("https://api.paylution.com") {
		t.Error("api subdomain should match base domain")
	}
	if !e.InScope("https://paylution.com") {
		t.Error("apex domain should match base domain")
	}
	if e.InScope("https://notpaylution.com") {
		t.Error("different domain must not match")
	}
	if e.InScope("https://evildpaylution.com") {
		t.Error("domain ending in paylution.com must not match without dot prefix")
	}
}

func TestMultiDomainScope(t *testing.T) {
	e, _ := New("paylution.com,hyperwallet.com", "https://www.paylution.com", false)
	if !e.InScope("https://api.paylution.com") {
		t.Error("paylution subdomain should be in scope")
	}
	if !e.InScope("https://www.hyperwallet.com") {
		t.Error("hyperwallet subdomain should be in scope")
	}
	if e.InScope("https://facebook.com") {
		t.Error("external domain must be out of scope")
	}
}

func TestStrictScope(t *testing.T) {
	e, _ := New("paylution.com", "https://www.paylution.com", true)
	if e.InScope("https://api.paylution.com") {
		t.Error("subdomain must be out of scope in strict mode")
	}
	if !e.InScope("https://paylution.com/path") {
		t.Error("exact host must still be in scope in strict mode")
	}
}

func TestPortIgnored(t *testing.T) {
	e, _ := New("paylution.com", "https://www.paylution.com", false)
	if !e.InScope("https://api.paylution.com:8443/endpoint") {
		t.Error("port should be ignored in scope matching")
	}
}
