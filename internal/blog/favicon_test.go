package blog

import (
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wusher/baumkuchen/internal/content"
)

func TestFaviconServedAndExported(t *testing.T) {
	const svg = `<svg xmlns="http://www.w3.org/2000/svg"/>`
	const png = "\x89PNG\r\n\x1a\nexample"
	posts := t.TempDir()
	write(t, posts, "favicon.svg", svg)
	custom := t.TempDir()
	write(t, custom, "custom.PNG", png)
	root := os.DirFS("../..")
	for _, c := range []struct {
		name, file, route, mediaType, data string
		source                             content.Source
	}{
		{"discovered", "", "/favicon.svg", "image/svg+xml", svg, nil},
		{"override", "custom.PNG", "/favicon.png", "image/png", png, content.NewDir(custom)},
	} {
		t.Run(c.name, func(t *testing.T) {
			site, err := New(Config{Posts: posts, Templates: root, Static: root,
				Base: "/blog", Heading: "Read deeper",
				Favicon: c.file, FaviconSource: c.source})
			if err != nil {
				t.Fatal(err)
			}
			request := func(path string) *httptest.ResponseRecorder {
				w := httptest.NewRecorder()
				site.routes().ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
				return w
			}
			index := request("/").Body.String()
			if !strings.Contains(index, `<link rel="icon" href="`+c.route+`">`) ||
				!strings.Contains(index, `<h1>Read deeper</h1>`) {
				t.Error("the served homepage is missing its favicon link or heading")
			}
			w := request(c.route)
			if w.Code != http.StatusOK || w.Header().Get("Content-Type") != c.mediaType || w.Body.String() != c.data {
				t.Errorf("favicon: status %d, type %q, body %q", w.Code, w.Header().Get("Content-Type"), w.Body.String())
			}

			out := t.TempDir()
			if _, err := site.Export(out); err != nil {
				t.Fatal(err)
			}
			index = readFile(t, filepath.Join(out, "index.html"))
			if !strings.Contains(index, `<link rel="icon" href="/blog`+c.route+`">`) {
				t.Error("the exported favicon link must include the base")
			}
			if got := readFile(t, filepath.Join(out, strings.TrimPrefix(c.route, "/"))); got != c.data {
				t.Errorf("exported favicon is %q, want %q", got, c.data)
			}
		})
	}
}

func TestFaviconFallbackAndBadOverride(t *testing.T) {
	posts := t.TempDir()
	site := testSite(t, posts)
	w := httptest.NewRecorder()
	site.routes().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	if !strings.Contains(w.Body.String(), `<link rel="icon" href="data:image/svg+xml,`) {
		t.Error("the built-in icon must remain when the posts folder has no favicon")
	}
	if strings.Contains(w.Body.String(), "<h1") {
		t.Error("the homepage heading should be hidden when not configured")
	}

	root := os.DirFS("../..")
	_, err := New(Config{Posts: posts, Templates: root, Static: root,
		Favicon: "missing.png", FaviconSource: content.NewDir(posts)})
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("missing explicit favicon: got %v, want a missing-file error", err)
	}
}

// A file whose bytes are not the picture its name promises is refused: the
// favicon is published as it is, so a settings line must not be able to put
// some other file's contents on the site under a picture's name.
func TestFaviconRefusesBytesThatAreNotThePicture(t *testing.T) {
	// arrange
	posts := t.TempDir()
	notAPng := t.TempDir()
	write(t, notAPng, "secret.png", "-----BEGIN OPENSSH PRIVATE KEY-----")
	root := os.DirFS("../..")

	// act
	_, err := New(Config{Posts: posts, Templates: root, Static: root,
		Favicon: "secret.png", FaviconSource: content.NewDir(notAPng)})

	// assert
	if err == nil || !strings.Contains(err.Error(), "does not read as a png") {
		t.Errorf("got %v, want a refusal that names the format", err)
	}
}
