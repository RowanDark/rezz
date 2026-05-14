package patterns

import "testing"

func TestDedupeExact(t *testing.T) {
	s := NewFindingStore(DedupeExact)

	f1 := Finding{Pattern: "AWS Key", Match: "AKIAIOSFODNN7EXAMPLE", URL: "https://example.com/a"}
	f2 := Finding{Pattern: "AWS Key", Match: "AKIAIOSFODNN7EXAMPLE", URL: "https://example.com/a"} // exact dup
	f3 := Finding{Pattern: "AWS Key", Match: "AKIAIOSFODNN7EXAMPLE", URL: "https://example.com/b"} // diff URL

	if !s.Add(f1) {
		t.Error("first add should return true")
	}
	if s.Add(f2) {
		t.Error("exact duplicate should return false")
	}
	if !s.Add(f3) {
		t.Error("same match different URL should be added in exact mode")
	}

	if s.Count() != 2 {
		t.Errorf("expected 2 findings, got %d", s.Count())
	}
}

func TestDedupeGlobal(t *testing.T) {
	s := NewFindingStore(DedupeGlobal)

	f1 := Finding{Pattern: "AWS Key", Match: "AKIAIOSFODNN7EXAMPLE", URL: "https://example.com/a"}
	f2 := Finding{Pattern: "AWS Key", Match: "AKIAIOSFODNN7EXAMPLE", URL: "https://example.com/b"} // same match diff URL
	f3 := Finding{Pattern: "AWS Key", Match: "AKIAIOSFODNN7EXAMPLE", URL: "https://example.com/a"} // exact dup

	s.Add(f1)
	s.Add(f2)
	s.Add(f3)

	if s.Count() != 1 {
		t.Errorf("expected 1 finding in global mode, got %d", s.Count())
	}

	findings := s.Findings()
	if len(findings[0].AlsoFoundOn) != 1 {
		t.Errorf("expected 1 AlsoFoundOn entry, got %d", len(findings[0].AlsoFoundOn))
	}
	if findings[0].AlsoFoundOn[0] != "https://example.com/b" {
		t.Errorf("unexpected AlsoFoundOn: %v", findings[0].AlsoFoundOn)
	}
}

func TestDedupeNone(t *testing.T) {
	s := NewFindingStore(DedupeNone)

	f := Finding{Pattern: "AWS Key", Match: "AKIAIOSFODNN7EXAMPLE", URL: "https://example.com/a"}
	s.Add(f)
	s.Add(f)
	s.Add(f)

	if s.Count() != 3 {
		t.Errorf("expected 3 findings with no dedup, got %d", s.Count())
	}
}

func TestDiffPatternSameMatch(t *testing.T) {
	s := NewFindingStore(DedupeGlobal)

	f1 := Finding{Pattern: "AWS Key", Match: "AKIAIOSFODNN7EXAMPLE", URL: "https://example.com"}
	f2 := Finding{Pattern: "Generic Key", Match: "AKIAIOSFODNN7EXAMPLE", URL: "https://example.com"}

	s.Add(f1)
	s.Add(f2)

	// Different patterns — both should be stored even in global mode
	if s.Count() != 2 {
		t.Errorf("expected 2 findings for different patterns, got %d", s.Count())
	}
}
