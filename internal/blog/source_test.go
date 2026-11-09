package blog

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"baumkuchen/internal/content"
)

// The site reads the posts from a set of files, not from a folder: the same
// pages, the same pictures, the same build.
func TestSiteFromAFileSet(t *testing.T) {
	// arrange
	files := fstest.MapFS{
		"content/posts/open.md":   {Data: []byte("---\ndate: 2026-01-02\ntag: earth\n---\n\n# Open\n\n# Sentence\n\nA line.\n")},
		"content/posts/hidden.md": {Data: []byte("---\ndate: 2026-01-01\ndraft: true\n---\n\n# Hidden\n\n# Sentence\n\nA line.\n")},
		"content/images/pic.jpg":  {Data: []byte("not really a picture")},
	}
	src := content.NewFS(files, "content")
	root := os.DirFS("../..")
	site, err := New(Config{Content: src, Templates: root, Static: root, Title: "Set"})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(site.routes())
	defer srv.Close()

	// act
	index := get(t, srv.URL+"/")
	post := get(t, srv.URL+"/post/open")

	// assert
	if !strings.Contains(index, "Open") {
		t.Error("the post is not on the index")
	}
	if !strings.Contains(index, "Hidden") {
		t.Error("a draft must still be on the index while the server runs")
	}
	if !strings.Contains(post, "Open") {
		t.Error("the post page does not hold its title")
	}
	res, err := http.Get(srv.URL + "/media/pic.jpg")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		t.Errorf("the picture gave %d, want 200", res.StatusCode)
	}
}

// The build reads the pictures from the file set as well.
func TestExportFromAFileSet(t *testing.T) {
	// arrange
	files := fstest.MapFS{
		"content/posts/one.md":   {Data: []byte("---\ndate: 2026-01-01\n---\n\n# One\n\n# Sentence\n\nA line.\n")},
		"content/images/pic.jpg": {Data: []byte("not really a picture")},
	}
	src := content.NewFS(files, "content")
	root := os.DirFS("../..")
	site, err := New(Config{Content: src, Templates: root, Static: root})
	if err != nil {
		t.Fatal(err)
	}
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
		filepath.Join("post", "one", "index.html"),
		filepath.Join("static", "style.css"),
		filepath.Join("media", "pic.jpg"),
	} {
		if _, err := os.Stat(filepath.Join(out, want)); err != nil {
			t.Errorf("%s is missing from the build", want)
		}
	}
}

// A folder of posts and a set of files hold the same thing, so the same site
// comes out of both.
func TestFolderAndFileSetGiveTheSameSite(t *testing.T) {
	// arrange
	dir := t.TempDir()
	postFile(t, dir, "one.md", "---\ndate: 2026-01-01\n---\n", "One")
	root := os.DirFS("../..")
	onDisk, err := New(Config{Posts: dir, Templates: root, Static: root})
	if err != nil {
		t.Fatal(err)
	}
	files := fstest.MapFS{
		"content/one.md": {Data: []byte("---\ndate: 2026-01-01\n---\n\n# One\n\n# Sentence\n\nA short line about it.\n")},
	}
	inFiles, err := New(Config{Content: content.NewFS(files, "content"), Templates: root, Static: root})
	if err != nil {
		t.Fatal(err)
	}

	// act
	diskOut, fileOut := t.TempDir(), t.TempDir()
	if _, err := onDisk.Export(diskOut); err != nil {
		t.Fatal(err)
	}
	if _, err := inFiles.Export(fileOut); err != nil {
		t.Fatal(err)
	}

	// assert
	for _, want := range []string{"index.html", filepath.Join("post", "one", "index.html")} {
		a, err := os.ReadFile(filepath.Join(diskOut, want))
		if err != nil {
			t.Fatal(err)
		}
		b, err := os.ReadFile(filepath.Join(fileOut, want))
		if err != nil {
			t.Fatal(err)
		}
		if string(a) != string(b) {
			t.Errorf("%s is not the same from both sources", want)
		}
	}
}
