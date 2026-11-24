package blog

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"regexp"
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
	return s.ExportWith(dir, false)
}

// ExportWith writes the whole site into dir as plain files, with the drafts
// in it when drafts is true.
//
// The build leaves a manifest behind: the list of the files it wrote, so that
// a clean can remove those files and nothing else.
func (s *Site) ExportWith(dir string, drafts bool) (int, error) {
	posts := s.store.Published()
	if drafts {
		posts = s.store.Index()
	}
	n := 0
	// written holds the files this build wrote, relative to dir.
	var written []string

	// note gives back the name of a file the build wrote.
	note := func(path string) {
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return
		}
		written = append(written, filepath.ToSlash(rel))
	}

	write := func(path, name string, data pageData) error {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		var page bytes.Buffer
		if err := s.renderTo(&page, name, data); err != nil {
			return err
		}
		if err := os.WriteFile(path, withBase(page.Bytes(), s.base), 0o644); err != nil {
			return err
		}
		note(path)
		n++
		return nil
	}

	pinned, rest := splitPinned(posts)
	if err := write(filepath.Join(dir, "index.html"), "index.html",
		pageData{Title: s.title, Pinned: pinned, Posts: rest}); err != nil {
		return n, err
	}
	if err := write(filepath.Join(dir, "404.html"), "404.html",
		pageData{Title: "Not here"}); err != nil {
		return n, err
	}
	if s.favicon.name != "" {
		if err := os.WriteFile(filepath.Join(dir, s.favicon.name), s.favicon.data, 0o644); err != nil {
			return n, err
		}
		note(filepath.Join(dir, s.favicon.name))
		n++
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
		note(out)
		n++
		return nil
	})
	if err != nil {
		return n, err
	}

	// the pictures, which live beside the posts and are served at /media/.
	// The folder inside the images folder is walked too, so a picture in a
	// folder of its own is copied with the folder it lives in.
	var pictures int
	var before, after int64
	err = fs.WalkDir(s.media, ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || strings.HasPrefix(d.Name(), ".") {
			return nil
		}
		data, err := fs.ReadFile(s.media, name)
		if err != nil {
			return err
		}
		src, dst, err := optimizeImageBytes(data, filepath.Join(dir, "media", filepath.FromSlash(name)))
		if err != nil {
			return err
		}
		before += src
		after += dst
		pictures++
		note(filepath.Join(dir, "media", filepath.FromSlash(name)))
		n++
		return nil
	})
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return n, err
	}
	// a site with no picture is not an error: there is nothing to copy, and
	// the build is still a build, so it still leaves its list behind
	if before > 0 {
		log.Printf("%d picture(s), %d KB written of %d KB, %d%% saved",
			pictures, after/1024, before/1024, 100-after*100/before)
	}
	if err := writeManifest(dir, written); err != nil {
		return n, err
	}
	return n, nil
}

// CleanBuild removes the files of an earlier build, and leaves everything
// else where it is.
//
// The build leaves a manifest behind: the list of the files it wrote. The
// clean removes those files, and the folders they leave empty. A file that
// is not in the manifest is the writer's own, and stays.
//
// A folder that holds files but no manifest is not a build, and is refused
// rather than emptied: a writer who points the build at the wrong folder
// keeps everything in it.
//
// The manifest stays behind, and is empty when the clean is done. The folder
// is still a build, it holds nothing of the build's now. A second clean
// finds nothing to remove and does not fail, and a build that fails between
// the clean and the write can be cleaned again.
func CleanBuild(dir string) error {
	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a folder", dir)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		return nil
	}
	names, err := readManifest(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%s holds files but no %s: it is not a build, "+
				"so nothing was removed", dir, manifestName)
		}
		return err
	}
	// the whole list is read and checked before anything is removed, so a
	// list that cannot be trusted takes nothing with it
	targets, err := manifestTargets(dir, names)
	if err != nil {
		return err
	}
	root := filepath.Clean(dir)
	var folders []string
	for _, target := range targets {
		if err := os.Remove(target); err != nil {
			if os.IsNotExist(err) {
				continue // it was removed already
			}
			return err
		}
		folders = append(folders, filepath.Dir(target))
	}
	if err := removeEmptyFolders(root, folders); err != nil {
		return err
	}
	// the manifest stays, and is empty: the folder is still a build, and it
	// holds nothing of the build's now
	return writeManifest(dir, nil)
}

// Every link the site writes starts at the root, because that is where the
// server answers. A site published in a folder — a GitHub project page lives
// at /the-repo/ — needs that folder in front of each one. These two patterns
// catch href="/…" and src="/…", including the bare href="/" of the wordmark,
// and leave anything else alone: an address with a scheme starts href="h, and
// one that starts // belongs to another host.
var (
	absRoot = regexp.MustCompile(`\b(href|src)="/"`)
	absPath = regexp.MustCompile(`\b(href|src)="/([^/"])`)
)

// withBase puts the folder in front of every root-absolute link on a page.
func withBase(page []byte, base string) []byte {
	if base == "" {
		return page
	}
	// the path pass runs first: it cannot match href="/", and doing it the
	// other way round would find the folder it had just written and add it
	// again
	page = absPath.ReplaceAll(page, []byte(`${1}="`+base+`/${2}`))
	return absRoot.ReplaceAll(page, []byte(`${1}="`+base+`/"`))
}

// cleanBase gives the folder a leading slash and no trailing one, so the
// rewriting above never doubles a slash. An empty one means the site lives at
// the root, and nothing is rewritten.
func cleanBase(base string) string {
	base = strings.Trim(strings.TrimSpace(base), "/")
	if base == "" {
		return ""
	}
	return "/" + base
}
