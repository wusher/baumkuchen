package blog

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Every route the site answers on, and the ones it refuses.
func TestRoutesAnswer(t *testing.T) {
	// arrange
	site, _ := siteWithPosts(t)
	srv := httptest.NewServer(site.routes())
	defer srv.Close()

	cases := []struct {
		path  string
		want  int
		holds string
	}{
		{"/", http.StatusOK, "Open"},
		{"/post/open", http.StatusOK, "Open"},
		{"/post/hidden", http.StatusOK, "draft"}, // a draft still answers locally
		{"/post/about", http.StatusOK, "About"},
		{"/post/notes", http.StatusNotFound, ""}, // no depth, so no page
		{"/post/nothing-here", http.StatusNotFound, ""},
		{"/not-a-page", http.StatusNotFound, ""},
		{"/static/style.css", http.StatusOK, ".level"},
		{"/media/pic.jpg", http.StatusOK, ""},
	}

	for _, c := range cases {
		t.Run(c.path, func(t *testing.T) {
			// act
			res, err := http.Get(srv.URL + c.path)
			if err != nil {
				t.Fatal(err)
			}
			body, _ := io.ReadAll(res.Body)
			res.Body.Close()

			// assert
			if res.StatusCode != c.want {
				t.Errorf("%s gave %d, want %d", c.path, res.StatusCode, c.want)
			}
			if c.holds != "" && !strings.Contains(string(body), c.holds) {
				t.Errorf("%s does not hold %q", c.path, c.holds)
			}
		})
	}
}

// The index shows the drafts while the server runs, and never the files that
// carry no depth.
func TestIndexShowsDraftsWhileServing(t *testing.T) {
	// arrange
	site, _ := siteWithPosts(t)
	rec := httptest.NewRecorder()

	// act
	site.handleIndex(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	// assert
	body := rec.Body.String()
	for _, want := range []string{"Open", "Hidden", "About", `<hr class="split">`, "draft"} {
		if !strings.Contains(body, want) {
			t.Errorf("the index does not hold %q", want)
		}
	}
	if strings.Contains(body, "No Depth Here") {
		t.Error("a file with no depth reached the index")
	}
}

// A post written after the site started shows up on the next request.
func TestReloadFindsANewPost(t *testing.T) {
	// arrange
	site, dir := siteWithPosts(t)
	postFile(t, dir, "later.md", "---\ndate: 2026-03-01\n---\n", "Later")
	rec := httptest.NewRecorder()

	// act
	site.handleIndex(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	// assert
	if !strings.Contains(rec.Body.String(), "Later") {
		t.Error("a post written after the start did not appear")
	}
}

// A post that was already there is read again when it changes: the page shows
// the writing as it is now, not the writing from when the server started.
func TestReloadRereadsAChangedPost(t *testing.T) {
	// arrange
	site, dir := siteWithPosts(t)
	rec := httptest.NewRecorder()

	// act
	write(t, dir, "open.md",
		"---\ndate: 2026-01-02\ntag: earth\n---\n\n# Open\n\n# Sentence\n\n"+
			"A line written after the server started.\n")
	site.handlePost(rec, httptest.NewRequest(http.MethodGet, "/post/open", nil))

	// assert
	if !strings.Contains(rec.Body.String(), "written after the server started") {
		t.Errorf("the page does not hold the new writing: %s", rec.Body.String())
	}
}

// The development server reads templates from disk after it starts, injects
// the browser watcher, and changes the versions that watcher polls.
func TestLiveReloadsTemplatesAndCSS(t *testing.T) {
	// arrange
	root := t.TempDir()
	for _, dir := range []string{"templates", "static"} {
		if err := os.Mkdir(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"layout.html", "index.html", "post.html", "404.html"} {
		body, err := os.ReadFile(filepath.Join("..", "..", "templates", name))
		if err != nil {
			t.Fatal(err)
		}
		write(t, filepath.Join(root, "templates"), name, string(body))
	}
	write(t, filepath.Join(root, "static"), "style.css", "body { color: red; }")
	posts := t.TempDir()
	postFile(t, posts, "one.md", "---\ndate: 2026-01-01\n---\n", "One")
	files := os.DirFS(root)
	site, err := New(Config{
		Posts: posts, Templates: files, Static: files,
		LiveReload: true, DevFiles: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	// act
	beforePage := servedIndex(site)
	beforeVersion := reloadVersions(t, site)
	indexPath := filepath.Join(root, "templates", "index.html")
	index, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Dir(indexPath), filepath.Base(indexPath),
		strings.Replace(string(index), `<ol class="feed" role="list" aria-label="posts">`,
			`<ol class="feed fresh" role="list" aria-label="posts">`, 1))
	write(t, filepath.Join(root, "static"), "style.css", "body { color: blue; }")
	afterPage := servedIndex(site)
	afterVersion := reloadVersions(t, site)
	css := httptest.NewRecorder()
	site.routes().ServeHTTP(css,
		httptest.NewRequest(http.MethodGet, "/static/style.css?_reload=1", nil))
	var exported strings.Builder
	if err := site.renderTo(&exported, "index.html", pageData{}); err != nil {
		t.Fatal(err)
	}

	// assert
	if !strings.Contains(beforePage, "/__baumkuchen_reload") {
		t.Error("the served page does not carry the live reload watcher")
	}
	if !strings.Contains(afterPage, `class="feed fresh"`) {
		t.Error("a template edit did not reach the next request")
	}
	if strings.Contains(exported.String(), "/__baumkuchen_reload") {
		t.Error("the live reload watcher reached the shared export renderer")
	}
	if beforeVersion["templates"] == afterVersion["templates"] {
		t.Error("the template version did not change")
	}
	if beforeVersion["css"] == afterVersion["css"] {
		t.Error("the CSS version did not change")
	}
	if !strings.Contains(css.Body.String(), "color: blue") {
		t.Error("the changed stylesheet was not served from disk")
	}
}

// An edited post is reported through the reload protocol, so that a browser
// that is open refreshes itself: the writing has a version of its own, and it
// changes when a file changes.
func TestTheBrowserIsToldWhenAPostChanges(t *testing.T) {
	// arrange
	site, dir := siteWithPosts(t)
	before := reloadVersions(t, site)

	// act
	write(t, dir, "open.md",
		"---\ndate: 2026-01-02\ntag: earth\n---\n\n# Open\n\n# Sentence\n\n"+
			"A line written after the server started.\n")
	after := reloadVersions(t, site)

	// assert
	if before["posts"] == "" || after["posts"] == "" {
		t.Fatalf("the reload endpoint does not say what the writing is: %v", after)
	}
	if before["posts"] == after["posts"] {
		t.Error("the writing did not change its version after an edit")
	}
	if !strings.Contains(liveReloadScript, "next.posts !== previous.posts") {
		t.Error("the watcher does not refresh the browser when the writing changes")
	}
}

// The files on disk belong to the development mode: without it, a change to a
// template on disk does not reach the page, and with it, it does.
func TestTheFilesOnDiskBelongToTheDevelopmentMode(t *testing.T) {
	// arrange
	root := templatesOnDisk(t)
	posts := t.TempDir()
	postFile(t, posts, "one.md", "---\ndate: 2026-01-01\n---\n", "One")
	files := os.DirFS(root)
	plain, err := New(Config{
		Posts: posts, Templates: files, Static: files, LiveReload: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	dev, err := New(Config{
		Posts: posts, Templates: files, Static: files,
		LiveReload: true, DevFiles: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	before := servedIndex(plain)

	// act
	changeTheTemplate(t, root)
	afterPlain := servedIndex(plain)
	afterDev := servedIndex(dev)

	// assert
	if !strings.Contains(before, `class="feed"`) {
		t.Fatal("the page does not carry the template it was made from")
	}
	if strings.Contains(afterPlain, `class="feed fresh"`) {
		t.Error("a template on disk reached the page without the development mode")
	}
	if !strings.Contains(afterDev, `class="feed fresh"`) {
		t.Error("the development mode did not read the template from disk")
	}
}

// The error in full is for the development mode: an ordinary server says that
// the page failed, and keeps the detail to itself.
func TestTheErrorInFullIsForTheDevelopmentMode(t *testing.T) {
	// arrange
	site, _ := siteWithPosts(t)
	site.liveReload = true // the server watches the writing

	// act
	plain := httptest.NewRecorder()
	site.render(plain, http.StatusInternalServerError, "nowhere.html", pageData{})
	full := httptest.NewRecorder()
	site.devErrors = true
	site.render(full, http.StatusInternalServerError, "nowhere.html", pageData{})

	// assert
	if strings.Contains(plain.Body.String(), "no page named") {
		t.Error("an ordinary server showed the error in full")
	}
	if !strings.Contains(full.Body.String(), "no page named") {
		t.Error("the development mode did not show the error in full")
	}
}

// templatesOnDisk makes a folder with the real templates and a stylesheet in
// it, and gives the folder back.
func templatesOnDisk(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, dir := range []string{"templates", "static"} {
		if err := os.Mkdir(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"layout.html", "index.html", "post.html", "404.html"} {
		body, err := os.ReadFile(filepath.Join("..", "..", "templates", name))
		if err != nil {
			t.Fatal(err)
		}
		write(t, filepath.Join(root, "templates"), name, string(body))
	}
	write(t, filepath.Join(root, "static"), "style.css", "body { color: red; }")
	return root
}

// changeTheTemplate marks the template on disk, so that a server watching it
// can see that it changed.
func changeTheTemplate(t *testing.T, root string) {
	t.Helper()
	path := filepath.Join(root, "templates", "index.html")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(root, "templates"), "index.html",
		strings.Replace(string(body),
			`<ol class="feed" role="list" aria-label="posts">`,
			`<ol class="feed fresh" role="list" aria-label="posts">`, 1))
}

func servedIndex(site *Site) string {
	rec := httptest.NewRecorder()
	site.handleIndex(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	return rec.Body.String()
}

func reloadVersions(t *testing.T, site *Site) map[string]string {
	t.Helper()
	rec := httptest.NewRecorder()
	site.handleLiveReload(rec, httptest.NewRequest(http.MethodGet, "/__baumkuchen_reload", nil))
	var versions map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&versions); err != nil {
		t.Fatal(err)
	}
	return versions
}

// What the command prints when it starts.
func TestCountsAndIncomplete(t *testing.T) {
	// arrange
	site, _ := siteWithPosts(t)

	// act
	published, drafts, incomplete := site.Counts()
	inc := site.Incomplete()

	// assert
	if published != 2 || drafts != 1 || incomplete != 1 {
		t.Errorf("counts = %d published, %d drafts, %d incomplete; want 2, 1, 1",
			published, drafts, incomplete)
	}
	if len(inc) != 1 || inc[0].Slug != "notes" {
		t.Fatalf("incomplete = %v, want [notes]", slugs(inc))
	}
	if len(inc[0].Missing) != 5 {
		t.Errorf("missing lists %d depths, want 5", len(inc[0].Missing))
	}
}

// A page that is not there, and an address that cannot be listened on, are
// both reported rather than swallowed.
func TestTheSiteReportsWhatItCannotDo(t *testing.T) {
	// arrange
	site, _ := siteWithPosts(t)
	root := os.DirFS("../..")

	// act
	renderErr := site.renderTo(io.Discard, "nowhere.html", pageData{})
	serveErr := site.Serve("this is not an address")
	_, newErr := New(Config{Posts: "no/such/folder", Templates: root, Static: root})

	// assert
	if renderErr == nil {
		t.Error("an unknown page must give an error")
	}
	if serveErr == nil {
		t.Error("a bad address must give an error, not silence")
	}
	if newErr == nil {
		t.Error("a missing posts folder must give an error")
	}
}

// A post that names a colour carries it on the html element; one that does not
// leaves the attribute off, and the index never takes a post's colour.
func TestPageCarriesTheColour(t *testing.T) {
	// arrange
	dir := t.TempDir()
	write(t, dir, "teal.md", "---\ndate: 2026-01-02\ncolor: teal\n---\n\n# Teal\n\n# Sentence\n\nfine.\n")
	postFile(t, dir, "plain.md", "---\ndate: 2026-01-01\n---\n", "Plain")
	site := testSite(t, dir)
	srv := httptest.NewServer(site.routes())
	defer srv.Close()

	// act
	coloured := get(t, srv.URL+"/post/teal")
	plain := get(t, srv.URL+"/post/plain")
	index := get(t, srv.URL+"/")

	// assert
	if !strings.Contains(coloured, `data-color="teal"`) {
		t.Error("the page does not carry the colour it asked for")
	}
	if strings.Contains(plain, "data-color") {
		t.Error("a page that asked for nothing must carry no colour")
	}
	if strings.Contains(index, "data-color") {
		t.Error("the index must keep the site's own accent")
	}
}

// The pictures and the stylesheet are not a place to browse: a folder gives a
// 404, not a listing of what is in it.
func TestFoldersAreNotListed(t *testing.T) {
	// arrange
	site, _ := siteWithPosts(t)
	srv := httptest.NewServer(site.routes())
	defer srv.Close()

	// act
	cases := []string{"/media/", "/static/"}
	for _, path := range cases {
		t.Run(path, func(t *testing.T) {
			res, err := http.Get(srv.URL + path)
			if err != nil {
				t.Fatal(err)
			}
			body, _ := io.ReadAll(res.Body)
			res.Body.Close()

			// assert
			if res.StatusCode != http.StatusNotFound {
				t.Errorf("%s gave %d, want %d", path, res.StatusCode, http.StatusNotFound)
			}
			if strings.Contains(string(body), "pic.jpg") ||
				strings.Contains(string(body), "style.css") {
				t.Errorf("%s lists what it holds: %s", path, body)
			}
		})
	}
}

// A draft that is held back is not there, and says so.
func TestHeldBackDraftsAreNotThere(t *testing.T) {
	// arrange
	site, dir := siteWithPosts(t)
	site.hideDrafts = true
	srv := httptest.NewServer(site.routes())
	defer srv.Close()

	// act
	draft := get(t, srv.URL+"/post/hidden")
	index := get(t, srv.URL+"/")

	// assert
	if strings.Contains(index, "Hidden") {
		t.Error("a held back draft is on the index")
	}
	if strings.Contains(draft, "Hidden") {
		t.Error("a held back draft has a page of its own")
	}
	_ = dir
}

func get(t *testing.T, url string) string {
	t.Helper()
	res, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// A post that names an opening depth carries it on the levels box, so the
// script can obey it. One that names nothing carries no attribute.
func TestPageCarriesTheOpeningDepth(t *testing.T) {
	// arrange
	dir := t.TempDir()
	write(t, dir, "opens.md",
		"---\ndate: 2026-01-02\ndepth: short\n---\n\n# Opens\n\n# Sentence\n\none.\n\n# Short\n\nthree.\n")
	postFile(t, dir, "plain.md", "---\ndate: 2026-01-01\n---\n", "Plain")
	site := testSite(t, dir)
	srv := httptest.NewServer(site.routes())
	defer srv.Close()

	// act
	named := get(t, srv.URL+"/post/opens")
	plain := get(t, srv.URL+"/post/plain")

	// assert
	if !strings.Contains(named, `data-depth="short"`) {
		t.Error("the page does not carry the depth it asked to open at")
	}
	if strings.Contains(plain, "data-depth") {
		t.Error("a page that asked for nothing must carry no depth")
	}
}
