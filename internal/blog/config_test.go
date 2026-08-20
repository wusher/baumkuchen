package blog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The settings file is read on top of the defaults, so a short file is enough.
func TestLoadFile(t *testing.T) {
	// arrange
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.yml")
	body := "# a comment, and only two settings\ntitle: My Blog\nbase: /somewhere\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	// act
	cfg, found, err := LoadFile(path)

	// assert
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Error("the file is there, so it must be reported as found")
	}
	if cfg.Title != "My Blog" || cfg.Base != "/somewhere" {
		t.Errorf("read %+v, want the title and the base from the file", cfg)
	}
	if cfg.Posts != Defaults().Posts || cfg.Dist != Defaults().Dist || cfg.Addr != Defaults().Addr {
		t.Errorf("read %+v, want the defaults for what the file leaves out", cfg)
	}
}

// A file that is not there is not a fault: the defaults stand, and the caller
// is told so it can say something.
func TestLoadFileWhenThereIsNone(t *testing.T) {
	// arrange
	path := filepath.Join(t.TempDir(), "no-such.yml")

	// act
	cfg, found, err := LoadFile(path)

	// assert
	if err != nil {
		t.Fatalf("a missing settings file must not be an error: %v", err)
	}
	if found {
		t.Error("nothing was there, so found must be false")
	}
	if cfg != Defaults() {
		t.Errorf("read %+v, want the defaults", cfg)
	}
}

// A key that is not a setting is a typo, and a typo that is swallowed is a
// setting that quietly does nothing.
func TestLoadFileRefusesAnUnknownKey(t *testing.T) {
	// arrange
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.yml")
	if err := os.WriteFile(path, []byte("title: X\nbse: /oops\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// act
	cfg, found, err := LoadFile(path)

	// assert
	if err == nil {
		t.Fatal("an unknown key must give an error")
	}
	if !strings.Contains(err.Error(), "bse") {
		t.Errorf("the error does not name the key: %v", err)
	}
	if !found {
		t.Error("the file was there, whatever was wrong with it")
	}
	if cfg != Defaults() {
		t.Error("a bad file leaves the defaults, not half of itself")
	}
}

// The site takes its name from the settings, and every page shows it.
func TestTitleReachesThePages(t *testing.T) {
	// arrange
	dir := t.TempDir()
	postFile(t, dir, "one.md", "---\ndate: 2026-01-01\n---\n", "One")
	root := os.DirFS("../..")
	site, err := New(Config{Posts: dir, Templates: root, Static: root, Title: "My Blog"})
	if err != nil {
		t.Fatal(err)
	}
	out := t.TempDir()

	// act
	if _, err := site.Export(out); err != nil {
		t.Fatal(err)
	}
	index := readFile(t, filepath.Join(out, "index.html"))
	post := readFile(t, filepath.Join(out, "post", "one", "index.html"))

	// assert
	if !strings.Contains(index, "<title>My Blog</title>") {
		t.Error("the index tab must hold the site name once, not twice")
	}
	if !strings.Contains(post, "<title>One · My Blog</title>") {
		t.Error("a post tab must hold its own name and the site's")
	}
	if !strings.Contains(index, ">My Blog</a>") {
		t.Error("the wordmark must hold the site name")
	}
	if strings.Contains(index, "Four Depths") {
		t.Error("the old name is still written into the page")
	}
}
