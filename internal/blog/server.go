// The running server: the pages it builds, the routes it answers, and the one
// renderer that the build shares with it.
package blog

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"baumkuchen/internal/content"
)

// Config is what a site needs to run. The two file systems are handed in by
// the command, because //go:embed reads paths from the folder of the package
// that asks for them, and templates/ and static/ live at the top of the tree.
type Config struct {
	// Content is where the markdown and the pictures come from: a folder on
	// disk, or files carried inside the program. When it is not given, the
	// folder at Posts is read.
	Content content.Source
	// Posts is the folder with the markdown, when Content is not given.
	Posts     string
	Templates fs.FS // holds templates/*.html
	Static    fs.FS // holds static/*
	// Base is the folder the built site is published in, if it is not the
	// root: "/baumkuchen" for a GitHub project page. The running server
	// always answers at the root and ignores this.
	Base string
	// Title is the name of the site.
	Title string
	// LiveReload rereads templates and tells an open browser when templates or
	// stylesheets change. It is intended for the disk-backed development server.
	LiveReload bool
}

// Site is one blog: the posts it has read, and the pages it can write.
type Site struct {
	store      *Store
	pages      map[string]*template.Template
	src        content.Source
	media      fs.FS
	templates  fs.FS
	static     fs.FS
	base       string
	title      string
	liveReload bool

	templatesMu     sync.RWMutex
	templateVersion string
	templateSeen    string
}

// New reads the posts and builds the pages.
func New(cfg Config) (*Site, error) {
	src := cfg.Content
	if src == nil {
		src = content.NewDir(cfg.Posts)
	}
	media, err := src.Media()
	if err != nil {
		return nil, fmt.Errorf("read the pictures: %w", err)
	}
	pages, err := buildPages(cfg.Templates)
	if err != nil {
		return nil, fmt.Errorf("read templates: %w", err)
	}
	templateVersion, err := filesVersion(cfg.Templates, "templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("read templates: %w", err)
	}
	s := &Site{
		store:           NewStoreFrom(src),
		pages:           pages,
		src:             src,
		media:           media,
		templates:       cfg.Templates,
		static:          cfg.Static,
		base:            cleanBase(cfg.Base),
		title:           cfg.Title,
		liveReload:      cfg.LiveReload,
		templateVersion: templateVersion,
		templateSeen:    templateVersion,
	}
	if err = s.store.Refresh(); err != nil {
		return nil, fmt.Errorf("read posts: %w", err)
	}
	return s, nil
}

// Counts gives what the command prints when it starts.
func (s *Site) Counts() (published, drafts, incomplete int) {
	return len(s.store.Published()), len(s.store.Drafts()), len(s.store.Incomplete())
}

// Incomplete gives the files that carry no depth, so the command can name them.
func (s *Site) Incomplete() []*Post { return s.store.Incomplete() }

type pageData struct {
	Title string
	Post  *Post
	// Pinned holds the pages that sit above the posts; Posts holds the rest.
	// The index puts a line between the two.
	Pinned []*Post
	Posts  []*Post
	// Color is the accent of the page being shown, if it asked for one.
	Color string
	// Site is the name of the whole site, for the tab and the wordmark.
	Site string
	Year int
}

// buildPages joins each page to the layout in its own template set,
// because every page file gives its content the same name.
func buildPages(files fs.FS) (map[string]*template.Template, error) {
	funcs := template.FuncMap{
		"join": strings.Join,
		"dec":  func(n int) int { return n - 1 },
	}
	pages := map[string]*template.Template{}
	for _, name := range []string{"index.html", "post.html", "404.html"} {
		t, err := template.New(name).Funcs(funcs).
			ParseFS(files, "templates/layout.html", "templates/"+name)
		if err != nil {
			return nil, err
		}
		pages[name] = t
	}
	return pages, nil
}

// routes gives the handler for the whole site. The pictures are served from
// the content, so a writer can add one without a build.
func (s *Site) routes() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/static/", http.FileServer(http.FS(s.static)))
	mux.Handle("/media/", http.StripPrefix("/media/",
		http.FileServer(http.FS(s.media))))
	if s.liveReload {
		mux.HandleFunc("/__baumkuchen_reload", s.handleLiveReload)
	}
	mux.HandleFunc("/post/", s.handlePost)
	mux.HandleFunc("/", s.handleIndex)
	return mux
}

// Serve answers on addr until it cannot.
func (s *Site) Serve(addr string) error {
	srv := &http.Server{
		Addr:         addr,
		Handler:      s.routes(),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}
	return srv.ListenAndServe()
}

func (s *Site) reload() {
	if err := s.store.Refresh(); err != nil {
		log.Printf("reload: %v", err)
	}
}

// reloadTemplates replaces the cached templates only after every page parses.
// A half-written template therefore leaves the last good page in place and is
// tried again after the file changes once more.
func (s *Site) reloadTemplates() {
	if !s.liveReload {
		return
	}
	version, err := filesVersion(s.templates, "templates/*.html")
	if err != nil {
		log.Printf("reload templates: %v", err)
		return
	}

	s.templatesMu.Lock()
	defer s.templatesMu.Unlock()
	if version == s.templateSeen {
		return
	}
	s.templateSeen = version
	pages, err := buildPages(s.templates)
	if err != nil {
		log.Printf("reload templates: %v", err)
		return
	}
	s.pages = pages
	s.templateVersion = version
}

func filesVersion(files fs.FS, pattern string) (string, error) {
	names, err := fs.Glob(files, pattern)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	for _, name := range names {
		body, err := fs.ReadFile(files, name)
		if err != nil {
			return "", err
		}
		_, _ = io.WriteString(h, name)
		_, _ = h.Write(body)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// renderTo writes one page anywhere. The server and the build both use it, so
// a page cannot come out one way on screen and another way in the folder.
func (s *Site) renderTo(w io.Writer, name string, data pageData) error {
	data.Year = time.Now().Year()
	data.Site = s.title
	s.reloadTemplates()
	s.templatesMu.RLock()
	t, ok := s.pages[name]
	s.templatesMu.RUnlock()
	if !ok {
		return fmt.Errorf("no page named %s", name)
	}
	return t.ExecuteTemplate(w, "layout", data)
}

func (s *Site) render(w http.ResponseWriter, name string, data pageData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	var page bytes.Buffer
	if err := s.renderTo(&page, name, data); err != nil {
		log.Printf("render %s: %v", name, err)
		return
	}
	body := page.Bytes()
	if s.liveReload {
		body = bytes.Replace(body, []byte("</body>"), []byte(liveReloadScript+"</body>"), 1)
	}
	_, _ = w.Write(body)
}

func (s *Site) handleLiveReload(w http.ResponseWriter, _ *http.Request) {
	s.reloadTemplates()
	cssVersion, err := filesVersion(s.static, "static/*.css")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.templatesMu.RLock()
	templateVersion := s.templateVersion
	s.templatesMu.RUnlock()
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"css": cssVersion, "templates": templateVersion,
	})
}

const liveReloadScript = `<script>
(() => {
  let previous;
  async function check() {
    try {
      const response = await fetch('/__baumkuchen_reload', {cache: 'no-store'});
      const next = await response.json();
      if (previous && next.templates !== previous.templates) {
        location.reload();
        return;
      }
      if (previous && next.css !== previous.css) {
        document.querySelectorAll('link[rel="stylesheet"][href^="/static/"]').forEach(link => {
          const url = new URL(link.href);
          url.searchParams.set('_reload', Date.now());
          link.href = url;
        });
      }
      previous = next;
    } catch (_) {}
    setTimeout(check, 500);
  }
  check();
})();
</script>`

func (s *Site) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		s.notFound(w)
		return
	}
	s.reload()
	pinned, rest := splitPinned(s.store.Index())
	s.render(w, "index.html", pageData{
		Title:  s.title,
		Pinned: pinned,
		Posts:  rest,
	})
}

func (s *Site) handlePost(w http.ResponseWriter, r *http.Request) {
	s.reload()
	slug := strings.Trim(strings.TrimPrefix(r.URL.Path, "/post/"), "/")
	p, ok := s.store.Get(slug)
	if !ok {
		s.notFound(w)
		return
	}
	s.render(w, "post.html", pageData{Title: p.Title, Post: p, Color: p.Color})
}

func (s *Site) notFound(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNotFound)
	s.render(w, "404.html", pageData{Title: "Not here"})
}
