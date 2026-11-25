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

// A query asks for the same picture again, and says nothing about the file,
// so it is not part of the name of the picture.
func TestPictureNameDropsTheQuery(t *testing.T) {
	cases := []struct {
		path string
		want string
	}{
		{"pic.jpg?v=2", "pic.jpg"},
		{"pic.jpg?v=2#center", "pic.jpg"},
		{"deeper/pic.jpg?w=100", "deeper/pic.jpg"},
	}

	for _, c := range cases {
		// act + assert
		if got := pictureName(c.path); got != c.want {
			t.Errorf("pictureName(%q) = %q, want %q", c.path, got, c.want)
		}
	}
}

// The name of a picture in a URL is written with the escapes of a URL, and is
// read back before the file is looked for.
func TestPictureNameReadsBackTheEscapes(t *testing.T) {
	cases := []struct {
		path string
		want string
	}{
		{"my%20pic.png", "my pic.png"},
		{"my%20pic.png#center", "my pic.png"},
		{"a%2Bb.jpg", "a+b.jpg"},
		{"100%.jpg", "100%.jpg"},
	}

	for _, c := range cases {
		// act + assert
		if got := pictureName(c.path); got != c.want {
			t.Errorf("pictureName(%q) = %q, want %q", c.path, got, c.want)
		}
	}
}

// A picture whose name holds a space is written with the escape of a space in
// the address, and it is the file with the space in its name that is looked
// for. A picture that is there is not reported missing.
func TestCheckFindsAPictureWithASpaceInItsName(t *testing.T) {
	// arrange
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "images"), 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(dir, "images"), "my pic.png", "not really a picture")
	write(t, dir, "one.md",
		"---\ndate: 2026-01-02\n---\n\n# One\n\n# Sentence\n\n![alt](/media/my%20pic.png)\n")
	site := testSite(t, dir)

	// act
	report := site.Check()

	// assert
	if f := finding(report, "missing-picture"); f != nil {
		t.Errorf("a picture whose name holds a space is reported missing: %q", f.Message)
	}
}

// A picture on another host is not ours: it is not in the images folder, and
// it is not the writing that is wrong. It is not looked for, and it is not
// reported.
func TestCheckIgnoresAPictureOnAnotherHost(t *testing.T) {
	// arrange
	dir := t.TempDir()
	write(t, dir, "one.md",
		"---\ndate: 2026-01-02\n---\n\n# One\n\n# Sentence\n\n"+
			"![alt](https://example.com/media/x.png)\n")
	site := testSite(t, dir)

	// act
	report := site.Check()

	// assert
	if f := finding(report, "missing-picture"); f != nil {
		t.Errorf("a picture on another host is reported missing: %q", f.Message)
	}
}

// A query asks for the same picture again, so the picture that is there is
// found, and the query is not part of the name of a picture that is not there.
func TestCheckIgnoresTheQueryOfAPicture(t *testing.T) {
	// arrange
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "images"), 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(dir, "images"), "pic.png", "not really a picture")
	write(t, dir, "one.md",
		"---\ndate: 2026-01-02\n---\n\n# One\n\n# Sentence\n\n"+
			"![alt](/media/pic.png?v=2)\n\n![alt](/media/gone.png?v=2)\n")
	site := testSite(t, dir)

	// act
	report := site.Check()

	// assert
	for _, f := range report.Findings {
		if f.Kind == "missing-picture" && strings.Contains(f.Message, "pic.png") {
			t.Errorf("a picture asked for with a query is reported missing: %q", f.Message)
		}
	}
	f := finding(report, "missing-picture")
	if f == nil {
		t.Fatalf("the check did not find the missing picture: %+v", report.Findings)
	}
	if !strings.Contains(f.Message, "gone.png") {
		t.Errorf("the finding does not name the picture: %q", f.Message)
	}
	if strings.Contains(f.Message, "?") {
		t.Errorf("the finding carries the query, which is not part of the name: %q", f.Message)
	}
}

// One missing picture in two depths is one thing wrong with the writing, so
// it is reported once, and the count of errors says so.
func TestCheckReportsAMissingPictureOnce(t *testing.T) {
	// arrange
	dir := t.TempDir()
	write(t, dir, "one.md",
		"---\ndate: 2026-01-02\n---\n\n# One\n\n# Sentence\n\n![alt](/media/gone.jpg)\n\n"+
			"# Paragraph\n\n![alt](/media/gone.jpg)\n")
	site := testSite(t, dir)

	// act
	report := site.Check()

	// assert
	n := 0
	for _, f := range report.Findings {
		if f.Kind == "missing-picture" {
			n++
		}
	}
	if n != 1 {
		t.Errorf("the check reports the missing picture %d time(s), want 1", n)
	}
	if report.Errors != 1 {
		t.Errorf("the report counts %d error(s), want 1", report.Errors)
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
