package blog

import (
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// export writes the whole site into dir as plain files, and gives the number
// of files it wrote. A draft never reaches it, and neither does a file that
// carries no depth.
//
// The URLs are the ones the server serves: a post lands at
// dir/post/<slug>/index.html, so /post/<slug> works on a static host with no
// rewrite rule. Nothing is deleted: the folder is made if it is not there, and
// a file that is already there is written over. Removing an old build stays a
// separate, deliberate step.
// Export writes the whole site into dir as plain files.
func (s *Site) Export(dir string) (int, error) {
	posts := s.store.Published()
	n := 0

	write := func(path, name string, data pageData) error {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		f, err := os.Create(path)
		if err != nil {
			return err
		}
		defer f.Close()
		if err := s.renderTo(f, name, data); err != nil {
			return err
		}
		n++
		return nil
	}

	pinned, rest := splitPinned(posts)
	if err := write(filepath.Join(dir, "index.html"), "index.html",
		pageData{Title: "Four depths", Pinned: pinned, Posts: rest}); err != nil {
		return n, err
	}
	if err := write(filepath.Join(dir, "404.html"), "404.html",
		pageData{Title: "Not here"}); err != nil {
		return n, err
	}
	for _, p := range posts {
		path := filepath.Join(dir, "post", p.Slug, "index.html")
		if err := write(path, "post.html", pageData{Title: p.Title, Post: p, Color: p.Color}); err != nil {
			return n, fmt.Errorf("%s: %w", p.Slug, err)
		}
	}

	// the stylesheet and the script, out of the files built into the binary
	assets, err := fs.Sub(s.static, "static")
	if err != nil {
		return n, err
	}
	err = fs.WalkDir(assets, ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := fs.ReadFile(assets, name)
		if err != nil {
			return err
		}
		out := filepath.Join(dir, "static", filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(out, b, 0o644); err != nil {
			return err
		}
		n++
		return nil
	})
	if err != nil {
		return n, err
	}

	// the pictures, which live beside the posts and are served at /media/
	images := filepath.Join(s.posts, "images")
	entries, err := os.ReadDir(images)
	if err != nil {
		if os.IsNotExist(err) {
			return n, nil // a site with no picture is not an error
		}
		return n, err
	}
	var pictures int
	var before, after int64
	for _, e := range entries {
		if e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		src, dst, err := optimizeImage(filepath.Join(images, e.Name()),
			filepath.Join(dir, "media", e.Name()))
		if err != nil {
			return n, err
		}
		before += src
		after += dst
		pictures++
		n++
	}
	if before > 0 {
		log.Printf("%d picture(s), %d KB written of %d KB, %d%% saved",
			pictures, after/1024, before/1024, 100-after*100/before)
	}
	return n, nil
}

func copyFile(from, to string) error {
	if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
		return err
	}
	in, err := os.Open(from)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(to)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
