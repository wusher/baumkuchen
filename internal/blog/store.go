// Reading the posts, and holding what was read: the index, the drafts,
// and the files that carry no depth at all.
//
// The posts come from a content.Source, so the same store serves a folder on
// disk and a set of files carried inside the program.
package blog

import (
	"fmt"
	"log"
	"path"
	"sort"
	"strings"
	"sync"

	"baumkuchen/internal/content"
)

type Store struct {
	src content.Source

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

// NewStore reads the posts in the folder at dir.
func NewStore(dir string) *Store {
	return NewStoreFrom(content.NewDir(dir))
}

// NewStoreFrom reads the posts a source holds.
func NewStoreFrom(src content.Source) *Store {
	return &Store{src: src, bySlug: map[string]*Post{}}
}

// markdown gives every .md file the source holds, in order, with the path
// each one was found at.
func (s *Store) markdown() ([]string, error) {
	return s.src.List()
}

// heldBack says whether a file sits in a folder whose name starts with an
// underscore. Everything under such a folder is a draft, whatever its front
// matter says: the folder is the writer saying "not yet".
func (s *Store) heldBack(file string) bool {
	dir := path.Dir(file)
	if dir == "." || dir == "/" {
		return false
	}
	for _, part := range strings.Split(dir, "/") {
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
	for _, file := range paths {
		fi, err := s.src.Stat(file)
		if err != nil {
			continue
		}
		fmt.Fprintf(&b, "%s:%d:%d;", file, fi.Size(), fi.ModTime().UnixNano())
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
	for _, file := range paths {
		p, err := parsePost(s.src, file)
		if err != nil {
			return err
		}
		if s.heldBack(file) {
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
