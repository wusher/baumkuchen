// The running server: the pages it builds, the routes it answers, and the one
// renderer that the build shares with it.
package blog

import (
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"time"
)

// Config is what a site needs to run. The two file systems are handed in by
// the command, because //go:embed reads paths from the folder of the package
// that asks for them, and templates/ and static/ live at the top of the tree.
type Config struct {
	Posts     string // the folder with the markdown
	Templates fs.FS  // holds templates/*.html
	Static    fs.FS  // holds static/*
}

// Site is one blog: the posts it has read, and the pages it can write.
type Site struct {
	store  *Store
	pages  map[string]*template.Template
	posts  string
	static fs.FS
}

// New reads the posts folder and builds the pages.
func New(cfg Config) (*Site, error) {
	s := &Site{
		store:  NewStore(cfg.Posts),
		pages:  buildPages(cfg.Templates),
		posts:  cfg.Posts,
		static: cfg.Static,
	}
	if err := s.store.Refresh(); err != nil {
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
	Year  int
}

// buildPages joins each page to the layout in its own template set,
// because every page file gives its content the same name.
func buildPages(files fs.FS) map[string]*template.Template {
	funcs := template.FuncMap{
		"join": strings.Join,
		"dec":  func(n int) int { return n - 1 },
	}
	pages := map[string]*template.Template{}
	for _, name := range []string{"index.html", "post.html", "404.html"} {
		t := template.Must(template.New(name).Funcs(funcs).
			ParseFS(files, "templates/layout.html", "templates/"+name))
		pages[name] = t
	}
	return pages
}

// routes gives the handler for the whole site. The pictures are served from
// disk, beside the posts, so a writer can add one without a build.
func (s *Site) routes() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/static/", http.FileServer(http.FS(s.static)))
	mux.Handle("/media/", http.StripPrefix("/media/",
		http.FileServer(http.Dir(filepath.Join(s.posts, "images")))))
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

// renderTo writes one page anywhere. The server and the build both use it, so
// a page cannot come out one way on screen and another way in the folder.
func (s *Site) renderTo(w io.Writer, name string, data pageData) error {
	data.Year = time.Now().Year()
	t, ok := s.pages[name]
	if !ok {
		return fmt.Errorf("no page named %s", name)
	}
	return t.ExecuteTemplate(w, "layout", data)
}

func (s *Site) render(w http.ResponseWriter, name string, data pageData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.renderTo(w, name, data); err != nil {
		log.Printf("render %s: %v", name, err)
	}
}

func (s *Site) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		s.notFound(w)
		return
	}
	s.reload()
	pinned, rest := splitPinned(s.store.Index())
	s.render(w, "index.html", pageData{
		Title:  "Four depths",
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
