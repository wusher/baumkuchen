// Reading the posts folder, and holding what was read: the index, the drafts,
// and the files that carry no depth at all.
package blog

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

type Store struct {
	dir string

	mu sync.RWMutex
	// posts holds every file that carries a depth, drafts among them, in the
	// order the index shows: pinned first, then the newest date.
	posts []*Post
	// incomplete holds the files that carry no depth at all. They cannot
	// render, so they reach neither the index nor the built site.
	incomplete []*Post
	bySlug     map[string]*Post
	stamp      string
}

func NewStore(dir string) *Store {
	return &Store{dir: dir, bySlug: map[string]*Post{}}
}

// root gives the folder to read, with any symlink resolved. The posts often
// live outside the repository, with ./posts pointing at them, and a walk does
// not follow a link at its starting point.
func (s *Store) root() string {
	if resolved, err := filepath.EvalSymlinks(s.dir); err == nil {
		return resolved
	}
	return s.dir
}

// markdown gives every .md file under the root, in order, with the path each
// one was found at. The pictures folder is stepped over, and so is anything
// hidden: a folder a person cannot see is not one they meant to publish from.
func (s *Store) markdown() ([]string, error) {
	var out []string
	root := s.root()
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() {
			if path == root {
				return nil
			}
			if name == "images" || strings.HasPrefix(name, ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.EqualFold(filepath.Ext(name), ".md") {
			out = append(out, path)
		}
		return nil
	})
	sort.Strings(out)
	return out, err
}

// heldBack says whether a file sits in a folder whose name starts with an
// underscore. Everything under such a folder is a draft, whatever its front
// matter says: the folder is the writer saying "not yet".
func (s *Store) heldBack(path string) bool {
	rel, err := filepath.Rel(s.root(), path)
	if err != nil {
		return false
	}
	for _, part := range strings.Split(filepath.Dir(rel), string(filepath.Separator)) {
		if strings.HasPrefix(part, "_") {
			return true
		}
	}
	return false
}

func (s *Store) fingerprint() string {
	paths, err := s.markdown()
	if err != nil {
		return "missing"
	}
	var b strings.Builder
	for _, path := range paths {
		fi, err := os.Stat(path)
		if err != nil {
			continue
		}
		fmt.Fprintf(&b, "%s:%d:%d;", path, fi.Size(), fi.ModTime().UnixNano())
	}
	return b.String()
}

// Refresh rereads the folder only when a file changed.

func (s *Store) Refresh() error {
	fp := s.fingerprint()
	s.mu.RLock()
	same := fp == s.stamp
	s.mu.RUnlock()
	if same {
		return nil
	}

	paths, err := s.markdown()
	if err != nil {
		return err
	}
	var posts, incomplete []*Post
	bySlug := map[string]*Post{}
	for _, path := range paths {
		p, err := ParsePost(path)
		if err != nil {
			return err
		}
		if s.heldBack(path) {
			p.Draft = true
		}
		if first, taken := bySlug[p.Slug]; taken {
			// two files of the same name in different folders would answer at
			// one URL; the first one keeps it, and the other is named here
			log.Printf("two files want /post/%s: %s and %s", p.Slug, first.File, p.File)
			continue
		}
		if p.Published {
			posts = append(posts, p)
			bySlug[p.Slug] = p
		} else {
			incomplete = append(incomplete, p)
		}
	}
	// two stable sorts: the newest first, and then the pinned pages lifted to
	// the top with that order kept inside each group
	sort.SliceStable(posts, func(i, j int) bool { return posts[i].Date.After(posts[j].Date) })
	sort.SliceStable(posts, func(i, j int) bool { return posts[i].Pinned && !posts[j].Pinned })
	sort.Slice(incomplete, func(i, j int) bool { return incomplete[i].Slug < incomplete[j].Slug })

	s.mu.Lock()
	s.posts, s.incomplete, s.bySlug, s.stamp = posts, incomplete, bySlug, fp
	s.mu.Unlock()
	return nil
}

// Index gives what the running server shows: every post that carries a depth,
// drafts among them.

func (s *Store) Index() []*Post {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.posts
}

// Published gives what the built site holds: the same list without the drafts.

func (s *Store) Published() []*Post {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*Post, 0, len(s.posts))
	for _, p := range s.posts {
		if !p.Draft {
			out = append(out, p)
		}
	}
	return out
}

// Drafts gives the posts held back from the built site.

func (s *Store) Drafts() []*Post {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*Post
	for _, p := range s.posts {
		if p.Draft {
			out = append(out, p)
		}
	}
	return out
}

// splitPinned cuts a list in two at the point where the pinned pages end. The
// list is already in that order, so this is one walk, and the index can put a
// line between the two groups.

func splitPinned(ps []*Post) (pinned, rest []*Post) {
	for i, p := range ps {
		if !p.Pinned {
			return ps[:i], ps[i:]
		}
	}
	return ps, nil
}

// Incomplete gives the files that carry no depth, for the startup log.

func (s *Store) Incomplete() []*Post {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.incomplete
}

func (s *Store) Get(slug string) (*Post, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.bySlug[slug]
	return p, ok
}
