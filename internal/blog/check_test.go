package blog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The check says what is wrong with the posts: what is missing from them, and
// which pictures they point at that are not there.
func TestCheckFindsAMissingPicture(t *testing.T) {
	// arrange
	dir := t.TempDir()
	write(t, dir, "one.md",
		"---\ndate: 2026-01-02\n---\n\n# One\n\n# Sentence\n\n![alt](/media/nope.jpg)\n")
	site := testSite(t, dir)

	// act
	report := site.Check()

	// assert
	f := finding(report, "missing-picture")
	if f == nil {
		t.Fatalf("the check did not find the missing picture: %+v", report.Findings)
	}
	if !strings.Contains(f.Message, "nope.jpg") {
		t.Errorf("the finding does not name the picture: %q", f.Message)
	}
	if f.Level != "error" {
		t.Errorf("the finding is a %q, want an error", f.Level)
	}
}

// The placement at the end of a picture's path — #left, #right or #center —
// says where the picture sits on the page. It is not part of the name of the
// file, so a picture that is there is not reported missing, and a picture that
// is not there is named without it.
func TestCheckKnowsThePlacementIsNotTheName(t *testing.T) {
	// arrange
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "images"), 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(dir, "images"), "here.jpg", "not really a picture")
	write(t, dir, "one.md",
		"---\ndate: 2026-01-02\n---\n\n# One\n\n# Sentence\n\n"+
			"![alt](/media/here.jpg#center)\n\n![alt](/media/gone.jpg#center)\n")
	site := testSite(t, dir)

	// act
	report := site.Check()

	// assert
	for _, f := range report.Findings {
		if f.Kind == "missing-picture" && strings.Contains(f.Message, "here.jpg") {
			t.Errorf("a picture that is there is reported missing: %q", f.Message)
		}
	}
	f := finding(report, "missing-picture")
	if f == nil {
		t.Fatalf("the check did not find the missing picture: %+v", report.Findings)
	}
	if !strings.Contains(f.Message, "gone.jpg") {
		t.Errorf("the finding does not name the picture: %q", f.Message)
	}
	if strings.Contains(f.Message, "#") {
		t.Errorf("the finding carries the placement, which is not part of the name: %q", f.Message)
	}
}

// A picture in a folder of its own is looked for in that folder, with or
// without the placement.
func TestCheckFindsAPictureInAFolder(t *testing.T) {
	// arrange
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "images", "deeper"), 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(dir, "images", "deeper"), "pic.jpg", "not really a picture")
	write(t, dir, "one.md",
		"---\ndate: 2026-01-02\n---\n\n# One\n\n# Sentence\n\n![alt](/media/deeper/pic.jpg#right)\n")
	site := testSite(t, dir)

	// act
	report := site.Check()

	// assert
	if f := finding(report, "missing-picture"); f != nil {
		t.Errorf("a picture that is there is reported missing: %q", f.Message)
	}
}

// The name of a picture is the path without the placement at the end of it.
func TestPictureNameDropsThePlacement(t *testing.T) {
	cases := []struct {
		path string
		want string
	}{
		{"pic.jpg", "pic.jpg"},
		{"pic.jpg#center", "pic.jpg"},
		{"pic.jpg#left", "pic.jpg"},
		{"pic.jpg#right", "pic.jpg"},
		{"deeper/pic.jpg#right", "deeper/pic.jpg"},
	}

	for _, c := range cases {
		// act + assert
		if got := pictureName(c.path); got != c.want {
			t.Errorf("pictureName(%q) = %q, want %q", c.path, got, c.want)
		}
	}
}

// finding gives the first finding of that kind on the report, if there is one.
func finding(report Report, kind string) *Finding {
	for i, f := range report.Findings {
		if f.Kind == kind {
			return &report.Findings[i]
		}
	}
	return nil
}
