package blog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The build writes the URLs the server answers on, and leaves the drafts out.
func TestExportWritesTheSite(t *testing.T) {
	// arrange
	site, _ := siteWithPosts(t)
	out := t.TempDir()

	// act
	n, err := site.Export(out)

	// assert
	if err != nil {
		t.Fatal(err)
	}
	if n < 6 {
		t.Errorf("wrote %d files, want at least 6", n)
	}
	for _, want := range []string{
		"index.html",
		"404.html",
		filepath.Join("post", "open", "index.html"),
		filepath.Join("post", "about", "index.html"),
		filepath.Join("static", "style.css"),
		filepath.Join("static", "app.js"),
		filepath.Join("media", "pic.jpg"),
	} {
		if _, err := os.Stat(filepath.Join(out, want)); err != nil {
			t.Errorf("%s is missing from the build", want)
		}
	}
	if _, err := os.Stat(filepath.Join(out, "post", "hidden", "index.html")); err == nil {
		t.Error("a draft reached the build")
	}
	if _, err := os.Stat(filepath.Join(out, "post", "notes", "index.html")); err == nil {
		t.Error("a file with no depth reached the build")
	}

	index := readFile(t, filepath.Join(out, "index.html"))
	if strings.Contains(index, "Hidden") {
		t.Error("a draft is named on the built index")
	}
	if !strings.Contains(index, "Open") {
		t.Error("the post is not on the built index")
	}
	if !strings.Contains(index, `href="/post/open"`) {
		t.Error("the built index must link the way the server does")
	}
}

// A site with no pictures builds without complaint, and makes no empty folder.
func TestExportWithNoImagesFolder(t *testing.T) {
	// arrange
	dir := t.TempDir()
	postFile(t, dir, "one.md", "---\ndate: 2026-01-01\n---\n", "One")
	site := testSite(t, dir)
	out := t.TempDir()

	// act
	n, err := site.Export(out)

	// assert
	if err != nil {
		t.Fatalf("a site with no images folder must still build: %v", err)
	}
	if n < 4 {
		t.Errorf("wrote %d files, want at least 4", n)
	}
	if _, err := os.Stat(filepath.Join(out, "media")); err == nil {
		t.Error("a media folder was made with nothing to put in it")
	}
}

// The build says so when it cannot write, instead of half finishing.
func TestExportRefusesAPathItCannotMake(t *testing.T) {
	// arrange
	site, _ := siteWithPosts(t)
	blocked := filepath.Join(t.TempDir(), "in-the-way")
	if err := os.WriteFile(blocked, []byte("not a folder"), 0o644); err != nil {
		t.Fatal(err)
	}

	// act
	_, err := site.Export(blocked)

	// assert
	if err == nil {
		t.Error("writing into a file must give an error")
	}
}

// A picture that is not there stops the copy, with a word about which one.
func TestCopyFileRefusesAMissingSource(t *testing.T) {
	// arrange
	from := filepath.Join(t.TempDir(), "no-such-file.jpg")
	to := filepath.Join(t.TempDir(), "copy.jpg")

	// act
	err := copyFile(from, to)

	// assert
	if err == nil {
		t.Fatal("copying a file that is not there must give an error")
	}
	if !strings.Contains(err.Error(), "no-such-file") {
		t.Errorf("the error does not name the file: %v", err)
	}
}

// The index draws its one line only when there are two groups to separate.
func TestIndexRuleNeedsBothGroups(t *testing.T) {
	// arrange
	cases := []struct {
		name     string
		files    map[string]string
		wantRule bool
	}{
		{"a page and a post", map[string]string{
			"about.md": "---\ndate: 2026-01-01\npinned: true\n---\n",
			"one.md":   "---\ndate: 2026-02-01\n---\n"}, true},
		{"pages only", map[string]string{
			"about.md": "---\ndate: 2026-01-01\npinned: true\n---\n"}, false},
		{"posts only", map[string]string{
			"one.md": "---\ndate: 2026-02-01\n---\n"}, false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			for name, front := range c.files {
				postFile(t, dir, name, front, strings.TrimSuffix(name, ".md"))
			}
			site := testSite(t, dir)
			out := t.TempDir()

			// act
			if _, err := site.Export(out); err != nil {
				t.Fatal(err)
			}
			index := readFile(t, filepath.Join(out, "index.html"))

			// assert
			got := strings.Contains(index, `<hr class="split">`)
			if got != c.wantRule {
				t.Errorf("rule drawn = %v, want %v", got, c.wantRule)
			}
			if c.wantRule {
				i := strings.Index(index, `<hr class="split">`)
				if strings.Index(index, ">about<") > i {
					t.Error("the pinned page is under the line")
				}
				if strings.Index(index, ">one<") < i {
					t.Error("the post is above the line")
				}
			}
		})
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
