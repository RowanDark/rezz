package output

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"

	"github.com/RowanDark/rezz/internal/patterns"
)

type JSONFinding struct {
	patterns.Finding
	AlsoFoundOn []string `json:"also_found_on,omitempty"`
}

// JSONWriter writes all findings as a JSON array to w.
func JSONWriter(w io.Writer, findings []JSONFinding, target string, pagesCrawled int) error {
	payload := struct {
		Target       string        `json:"target"`
		PagesCrawled int           `json:"pages_crawled"`
		FindingCount int           `json:"finding_count"`
		Findings     []JSONFinding `json:"findings"`
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
func CSVWriter(w io.Writer, findings []patterns.SeenFinding) error {
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
