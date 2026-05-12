package output_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/RowanDark/v0x/internal/extractor"
	"github.com/RowanDark/v0x/internal/output"
)

var sampleResult = extractor.Result{
	Words:  []string{"login", "admin", "config"},
	Emails: []string{"user@example.com"},
	Meta: map[string]string{
		"og:title": "Example Site",
	},
	WordSource: map[string]string{
		"login":  "body",
		"admin":  "body",
		"config": "body",
	},
}

var sampleMeta = output.OutputMeta{
	TargetURL:    "https://example.com",
	PagesCrawled: 14,
}

func TestNew_UnknownFormat(t *testing.T) {
	_, err := output.New("xml")
	if err == nil {
		t.Fatal("expected error for unknown format")
	}
}

func TestNew_KnownFormats(t *testing.T) {
	for _, f := range []string{"txt", "json", "csv", "md", "markdown", ""} {
		if _, err := output.New(f); err != nil {
			t.Errorf("New(%q) returned unexpected error: %v", f, err)
		}
	}
}

func TestTextFormatter_Sorted(t *testing.T) {
	f, _ := output.New("txt")
	var buf bytes.Buffer
	if err := f.Write(&buf, sampleResult, sampleMeta); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}
	if lines[0] != "admin" || lines[1] != "config" || lines[2] != "login" {
		t.Errorf("unexpected order: %v", lines)
	}
}

func TestJSONFormatter_ContainsFields(t *testing.T) {
	f, _ := output.New("json")
	var buf bytes.Buffer
	if err := f.Write(&buf, sampleResult, sampleMeta); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{`"target"`, `"crawled_pages"`, `"words"`, `"emails"`, `"meta"`} {
		if !strings.Contains(out, want) {
			t.Errorf("JSON output missing %q", want)
		}
	}
}

func TestCSVFormatter_Header(t *testing.T) {
	f, _ := output.New("csv")
	var buf bytes.Buffer
	if err := f.Write(&buf, sampleResult, sampleMeta); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if lines[0] != "word,length,source" {
		t.Errorf("unexpected CSV header: %q", lines[0])
	}
	if len(lines) != 4 { // header + 3 words
		t.Errorf("expected 4 lines, got %d", len(lines))
	}
}

func TestMarkdownFormatter_Sections(t *testing.T) {
	f, _ := output.New("md")
	var buf bytes.Buffer
	if err := f.Write(&buf, sampleResult, sampleMeta); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"# v0x report", "## Wordlist", "## Emails", "## Metadata", "og:title"} {
		if !strings.Contains(out, want) {
			t.Errorf("markdown output missing %q", want)
		}
	}
}

func TestTextFormatter_EmptyResult(t *testing.T) {
	f, _ := output.New("txt")
	var buf bytes.Buffer
	empty := extractor.Result{Words: []string{}, Emails: []string{}, Meta: map[string]string{}}
	if err := f.Write(&buf, empty, sampleMeta); err != nil {
		t.Fatal(err)
	}
	if buf.Len() != 0 {
		t.Errorf("expected empty output for empty result, got %q", buf.String())
	}
}
