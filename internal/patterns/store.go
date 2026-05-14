package patterns

import "sync"

// DedupeMode controls how findings are deduplicated.
type DedupeMode int

const (
	DedupeExact  DedupeMode = iota // Level 1: same pattern+match+url (default)
	DedupeGlobal                   // Level 2: same pattern+match across all URLs
	DedupeNone                     // No deduplication — raw output
)

// SeenFinding tracks where a globally-deduped finding was first seen
// and how many additional occurrences were suppressed.
type SeenFinding struct {
	Finding
	AlsoFoundOn []string // additional URLs where the same match appeared
}

// FindingStore accumulates findings with configurable deduplication.
// Safe for concurrent use.
type FindingStore struct {
	mu       sync.Mutex
	mode     DedupeMode
	seen     map[string]int // key → index in findings slice
	findings []SeenFinding
}

// NewFindingStore creates a store with the given dedup mode.
func NewFindingStore(mode DedupeMode) *FindingStore {
	return &FindingStore{
		mode: mode,
		seen: make(map[string]int),
	}
}

// Add attempts to add a finding to the store.
// Returns true if the finding was added (new), false if it was deduplicated.
// For DedupeGlobal, a deduplicated finding updates AlsoFoundOn on the original.
func (s *FindingStore) Add(f Finding) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch s.mode {
	case DedupeNone:
		s.findings = append(s.findings, SeenFinding{Finding: f})
		return true

	case DedupeExact:
		key := f.Pattern + "|" + f.Match + "|" + f.URL
		if _, exists := s.seen[key]; exists {
			return false
		}
		s.seen[key] = len(s.findings)
		s.findings = append(s.findings, SeenFinding{Finding: f})
		return true

	case DedupeGlobal:
		// First check exact key — never emit exact duplicates
		exactKey := f.Pattern + "|" + f.Match + "|" + f.URL
		if _, exists := s.seen[exactKey]; exists {
			return false
		}
		s.seen[exactKey] = -1 // mark exact as seen without storing

		// Then check global key
		globalKey := f.Pattern + "|" + f.Match
		if idx, exists := s.seen[globalKey]; exists {
			// Already seen this match on a different URL — record it
			if f.URL != s.findings[idx].URL {
				s.findings[idx].AlsoFoundOn = append(
					s.findings[idx].AlsoFoundOn, f.URL)
			}
			return false
		}
		s.seen[globalKey] = len(s.findings)
		s.findings = append(s.findings, SeenFinding{Finding: f})
		return true
	}

	return false
}

// Findings returns all stored findings in insertion order.
func (s *FindingStore) Findings() []SeenFinding {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]SeenFinding, len(s.findings))
	copy(out, s.findings)
	return out
}

// Count returns the total number of stored findings.
func (s *FindingStore) Count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.findings)
}
