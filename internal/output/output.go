package output

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/RowanDark/v0x/internal/extractor"
)

// OutputMeta carries per-run context that formatters need but Result doesn't hold.
type OutputMeta struct {
	TargetURL    string
	PagesCrawled int
}

// Formatter serialises an extractor.Result to an io.Writer.
type Formatter interface {
	Write(w io.Writer, r extractor.Result, meta OutputMeta) error
}

// New returns the Formatter for the given format name or an error for unknown formats.
func New(format string) (Formatter, error) {
	switch strings.ToLower(format) {
	case "txt", "":
		return TextFormatter{}, nil
	case "json":
		return JSONFormatter{}, nil
	case "csv":
		return CSVFormatter{}, nil
	case "md", "markdown":
		return MarkdownFormatter{}, nil
	default:
		return nil, fmt.Errorf("unknown format %q: must be txt, json, csv, or md", format)
	}
}

// --- Text formatter -------------------------------------------------------

// TextFormatter writes one word per line, CeWL-compatible, no header.
type TextFormatter struct{}

func (TextFormatter) Write(w io.Writer, r extractor.Result, _ OutputMeta) error {
	words := sorted(r.Words)
	for _, word := range words {
		if _, err := fmt.Fprintln(w, word); err != nil {
			return err
		}
	}
	return nil
}

// --- JSON formatter -------------------------------------------------------

// JSONFormatter writes structured JSON output.
type JSONFormatter struct{}

func (JSONFormatter) Write(w io.Writer, r extractor.Result, meta OutputMeta) error {
	metaOut := r.Meta
	if metaOut == nil {
		metaOut = map[string]string{}
	}

	payload := struct {
		Target       string            `json:"target"`
		CrawledPages int               `json:"crawled_pages"`
		Words        []string          `json:"words"`
		Emails       []string          `json:"emails"`
		Meta         map[string]string `json:"meta"`
	}{
		Target:       meta.TargetURL,
		CrawledPages: meta.PagesCrawled,
		Words:        sorted(r.Words),
		Emails:       sorted(r.Emails),
		Meta:         metaOut,
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(payload)
}

// --- CSV formatter --------------------------------------------------------

// CSVFormatter writes words with metadata columns: word, length, source.
type CSVFormatter struct{}

func (CSVFormatter) Write(w io.Writer, r extractor.Result, _ OutputMeta) error {
	cw := csv.NewWriter(w)
	if err := cw.Write([]string{"word", "length", "source"}); err != nil {
		return err
	}

	words := sorted(r.Words)
	for _, word := range words {
		src := "body"
		if r.WordSource != nil {
			if s, ok := r.WordSource[word]; ok {
				src = s
			}
		}
		if err := cw.Write([]string{word, fmt.Sprintf("%d", len(word)), src}); err != nil {
			return err
		}
	}

	cw.Flush()
	return cw.Error()
}

// --- Markdown formatter ---------------------------------------------------

// MarkdownFormatter writes a human-readable markdown report.
type MarkdownFormatter struct{}

func (MarkdownFormatter) Write(w io.Writer, r extractor.Result, meta OutputMeta) error {
	words := sorted(r.Words)
	emails := sorted(r.Emails)

	fmt.Fprintf(w, "# v0x report — %s\n\n", meta.TargetURL)
	fmt.Fprintf(w, "**Pages crawled:** %d\n", meta.PagesCrawled)
	fmt.Fprintf(w, "**Words collected:** %d\n", len(words))
	fmt.Fprintf(w, "**Emails found:** %d\n\n", len(emails))

	// Wordlist section
	fmt.Fprintln(w, "## Wordlist")
	if len(words) == 0 {
		fmt.Fprintln(w, "_none_")
	} else {
		quoted := make([]string, len(words))
		for i, word := range words {
			quoted[i] = "`" + word + "`"
		}
		fmt.Fprintln(w, strings.Join(quoted, ", "))
	}
	fmt.Fprintln(w)

	// Emails section
	fmt.Fprintln(w, "## Emails")
	if len(emails) == 0 {
		fmt.Fprintln(w, "_none_")
	} else {
		for _, e := range emails {
			fmt.Fprintf(w, "- %s\n", e)
		}
	}
	fmt.Fprintln(w)

	// Metadata section
	fmt.Fprintln(w, "## Metadata")
	if len(r.Meta) == 0 {
		fmt.Fprintln(w, "_none_")
	} else {
		fmt.Fprintln(w, "| Key | Value |")
		fmt.Fprintln(w, "|-----|-------|")
		keys := make([]string, 0, len(r.Meta))
		for k := range r.Meta {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Fprintf(w, "| %s | %s |\n", mdEscape(k), mdEscape(r.Meta[k]))
		}
	}

	return nil
}

// --- helpers --------------------------------------------------------------

func mdEscape(s string) string {
	return strings.ReplaceAll(s, "|", "\\|")
}

func sorted(in []string) []string {
	out := make([]string, len(in))
	copy(out, in)
	sort.Strings(out)
	return out
}
