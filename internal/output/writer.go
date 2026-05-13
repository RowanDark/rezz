package output

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/RowanDark/rezz/internal/patterns"
)

// StreamWriter writes findings to stdout as they arrive — one line per finding.
// This is the default --format stream behaviour.
func StreamWriter(f patterns.Finding) {
	fmt.Fprintf(os.Stdout, "[%s] %s — %s\n    URL: %s\n    Match: %s\n\n",
		severityLabel(f.Severity),
		f.Pattern,
		f.Category,
		f.URL,
		f.Match,
	)
}

func severityLabel(s string) string {
	switch s {
	case "high":
		return "HIGH  "
	case "medium":
		return "MEDIUM"
	default:
		return "LOW   "
	}
}

// JSONWriter writes all findings as a JSON array to w.
func JSONWriter(w io.Writer, findings []patterns.Finding, target string, pagesCrawled int) error {
	payload := struct {
		Target       string             `json:"target"`
		PagesCrawled int                `json:"pages_crawled"`
		FindingCount int                `json:"finding_count"`
		Findings     []patterns.Finding `json:"findings"`
	}{
		Target:       target,
		PagesCrawled: pagesCrawled,
		FindingCount: len(findings),
		Findings:     findings,
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(payload)
}

// CSVWriter writes all findings as CSV to w.
func CSVWriter(w io.Writer, findings []patterns.Finding) error {
	cw := csv.NewWriter(w)
	if err := cw.Write([]string{"url", "pattern", "category", "severity", "match", "context", "status_code", "from_js"}); err != nil {
		return err
	}
	for _, f := range findings {
		if err := cw.Write([]string{
			f.URL, f.Pattern, f.Category, f.Severity,
			f.Match, f.Context,
			fmt.Sprintf("%d", f.StatusCode),
			fmt.Sprintf("%v", f.FromJS),
		}); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}
