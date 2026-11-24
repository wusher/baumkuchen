package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wusher/baumkuchen/internal/blog"
)

// The JSON report is a contract: the golden file holds the whole of it, so a
// change to the report is a change on purpose and not by accident.
func TestTheJSONReportMatchesTheGolden(t *testing.T) {
	// arrange
	golden, err := os.ReadFile(filepath.Join("testdata", "check.golden"))
	if err != nil {
		t.Fatalf("the golden file cannot be read: %v", err)
	}

	// act
	var stdout, stderr bytes.Buffer
	code := run([]string{"check", "--json", filepath.Join("testdata", "check-fixture")},
		&stdout, &stderr)

	// assert
	if stdout.String() != string(golden) {
		t.Errorf("the report is not the one in the golden file:\n%s", stdout.String())
	}
	if code != 1 {
		t.Errorf("the check of the fixture gave the code %d, want 1: the fixture holds errors",
			code)
	}
}

// The report says which shape it is before anything else, so that a script can
// read the schema first and know what it is looking at.
func TestTheSchemaComesFirstInTheReport(t *testing.T) {
	// arrange
	var stdout, stderr bytes.Buffer

	// act
	run([]string{"check", "--json", filepath.Join("testdata", "check-fixture")},
		&stdout, &stderr)
	body := stdout.String()

	// assert
	if !strings.HasPrefix(body, "{\n  \"schema\": 1,") {
		t.Errorf("the report does not begin with the schema:\n%s", body)
	}
	var report struct {
		Schema int `json:"schema"`
	}
	if err := json.Unmarshal([]byte(body), &report); err != nil {
		t.Fatalf("the report is not JSON: %v", err)
	}
	if report.Schema != blog.SchemaVersion {
		t.Errorf("the schema is %d, want %d", report.Schema, blog.SchemaVersion)
	}
}

// Every finding names its level and its kind, and the kinds are the ones the
// program says they are.
func TestTheFindingsNameTheirLevelAndKind(t *testing.T) {
	// arrange
	dir := t.TempDir()
	write(t, filepath.Join(dir, "notes.md"), "# No Depth Here\n\nnothing.\n")
	write(t, filepath.Join(dir, "bad.md"),
		"---\ndate: 2026-13-45\n---\n\n# Bad\n\n# Sentence\n\nA line.\n\n"+
			"# Paragraph\n\nA paragraph.\n\n# Short\n\nThree paragraphs.\n\n"+
			"# Medium\n\nA page.\n\n# Long\n\nThree pages.\n")

	// act
	var stdout, stderr bytes.Buffer
	run([]string{"check", "--json", dir}, &stdout, &stderr)
	var report struct {
		Findings []struct {
			Level string `json:"level"`
			Kind  string `json:"kind"`
		} `json:"findings"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("the report is not JSON: %v", err)
	}

	// assert
	if len(report.Findings) != 2 {
		t.Fatalf("the report holds %d findings, want 2: %s",
			len(report.Findings), stdout.String())
	}
	want := map[string]string{
		blog.KindMissingDepths: blog.LevelError,
		blog.KindBadDate:       blog.LevelError,
	}
	for _, f := range report.Findings {
		level, known := want[f.Kind]
		if !known {
			t.Errorf("the finding holds the kind %q, which is not one of the four", f.Kind)
			continue
		}
		if f.Level != level {
			t.Errorf("the finding of the kind %q is a %q, want a %q",
				f.Kind, f.Level, level)
		}
	}
}
