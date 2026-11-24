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

// A picture in a folder of its own is copied with the folder it lives in.
func TestExportCopiesPicturesInFolders(t *testing.T) {
	// arrange
	dir := t.TempDir()
	postFile(t, dir, "one.md", "---\ndate: 2026-01-01\n---\n", "One")
	if err := os.MkdirAll(filepath.Join(dir, "images", "deeper"), 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(dir, "images", "deeper"), "pic.jpg", "not really a picture")
	site := testSite(t, dir)
	out := t.TempDir()

	// act
	if _, err := site.Export(out); err != nil {
		t.Fatal(err)
	}

	// assert
	if _, err := os.Stat(filepath.Join(out, "media", "deeper", "pic.jpg")); err != nil {
		t.Errorf("the picture in the folder was not copied: %v", err)
	}
}

// A build with the drafts in it keeps them.
func TestExportWithDrafts(t *testing.T) {
	// arrange
	site, _ := siteWithPosts(t)
	out := t.TempDir()

	// act
	if _, err := site.ExportWith(out, true); err != nil {
		t.Fatal(err)
	}

	// assert
	if _, err := os.Stat(filepath.Join(out, "post", "hidden", "index.html")); err != nil {
		t.Errorf("the draft did not reach the build: %v", err)
	}
}

// The clean removes the files of an earlier build, and leaves the rest of the
// folder alone.
func TestCleanBuild(t *testing.T) {
	// arrange
	dir := t.TempDir()
	site, _ := siteWithPosts(t)
	if _, err := site.Export(dir); err != nil {
		t.Fatal(err)
	}
	write(t, dir, "notes.txt", "a file of the writer's own")

	// act
	err := CleanBuild(dir)

	// assert
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "index.html")); err == nil {
		t.Error("the index of the earlier build is still there")
	}
	if _, err := os.Stat(filepath.Join(dir, "notes.txt")); err != nil {
		t.Error("the clean took a file that was not the build's")
	}
}

// A folder that holds files but no manifest is not a build, and is not
// emptied.
func TestCleanBuildRefusesAFolderThatIsNotABuild(t *testing.T) {
	// arrange
	dir := t.TempDir()
	write(t, dir, "something.txt", "not a build")

	// act
	err := CleanBuild(dir)

	// assert
	if err == nil {
		t.Fatal("the clean must refuse a folder that is not a build")
	}
	if _, statErr := os.Stat(filepath.Join(dir, "something.txt")); statErr != nil {
		t.Error("the file was removed even though the clean refused")
	}
}

// A folder that is not there has nothing to clean, and an empty one is
// already clean.
func TestCleanBuildOnAFolderThatIsNotThere(t *testing.T) {
	// arrange
	dir := filepath.Join(t.TempDir(), "no-such-folder")

	// act + assert
	if err := CleanBuild(dir); err != nil {
		t.Errorf("a folder that is not there must not be a fault: %v", err)
	}
	if err := CleanBuild(t.TempDir()); err != nil {
		t.Errorf("an empty folder must not be a fault: %v", err)
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

// A site published in a folder gets that folder in front of every link it
// wrote, and in front of nothing else.
func TestBasePath(t *testing.T) {
	// arrange
	page := []byte(`<a href="/">home</a> <link href="/static/style.css">` +
		`<img src="/media/x.jpg"> <a href="https://example.com/x">out</a>` +
		` <a href="//cdn.example.com/y">other host</a> <a href="/post/one">a post</a>`)

	// act
	got := string(withBase(page, "/baumkuchen"))
	untouched := string(withBase(page, ""))

	// assert
	for _, want := range []string{
		`href="/baumkuchen/"`,
		`href="/baumkuchen/static/style.css"`,
		`src="/baumkuchen/media/x.jpg"`,
		`href="/baumkuchen/post/one"`,
		`href="https://example.com/x"`, // a scheme is left alone
		`href="//cdn.example.com/y"`,   // another host is left alone
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %s from\n%s", want, got)
		}
	}
	if strings.Contains(got, "/baumkuchen/baumkuchen") || strings.Contains(got, `="//baumkuchen`) {
		t.Errorf("the folder was written twice or oddly:\n%s", got)
	}
	if untouched != string(page) {
		t.Error("with no folder given, the page must come back as it was")
	}
}

// The folder is tidied, whatever shape it arrives in.
func TestCleanBase(t *testing.T) {
	// arrange
	cases := map[string]string{
		"/baumkuchen":  "/baumkuchen",
		"baumkuchen":   "/baumkuchen",
		"/baumkuchen/": "/baumkuchen",
		" /blog/ ":     "/blog",
		"/":            "",
		"":             "",
	}

	for in, want := range cases {
		t.Run(in, func(t *testing.T) {
			// act
			got := cleanBase(in)

			// assert
			if got != want {
				t.Errorf("cleanBase(%q) = %q, want %q", in, got, want)
			}
		})
	}
}

// The whole build carries the folder, and the pages still find each other.
func TestExportUnderAFolder(t *testing.T) {
	// arrange
	dir := t.TempDir()
	postFile(t, dir, "one.md", "---\ndate: 2026-01-01\n---\n", "One")
	root := os.DirFS("../..")
	site, err := New(Config{Posts: dir, Templates: root, Static: root, Base: "baumkuchen/"})
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
	for _, want := range []string{`href="/baumkuchen/static/style.css"`, `href="/baumkuchen/post/one"`} {
		if !strings.Contains(index, want) {
			t.Errorf("the index is missing %s", want)
		}
	}
	if !strings.Contains(post, `href="/baumkuchen/"`) {
		t.Error("the post cannot find its way home")
	}
	if strings.Contains(index, `href="/static`) || strings.Contains(index, `href="/post`) {
		t.Error("a link was left at the root")
	}
}
