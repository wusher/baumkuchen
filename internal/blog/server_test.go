package blog

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
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
