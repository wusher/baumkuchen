// Reading the posts folder, and holding what was read: the index, the drafts,
// and the files that carry no depth at all.
package blog

import (
	"fmt"
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

func (s *Store) fingerprint() string {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return "missing"
	}
	var b strings.Builder
	for _, e := range entries {
		if e.IsDir() || !strings.EqualFold(filepath.Ext(e.Name()), ".md") {
			continue
		}
		fi, err := e.Info()
		if err != nil {
			continue
		}
		fmt.Fprintf(&b, "%s:%d:%d;", e.Name(), fi.Size(), fi.ModTime().UnixNano())
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

	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return err
	}
	var posts, incomplete []*Post
	bySlug := map[string]*Post{}
	for _, e := range entries {
		if e.IsDir() || !strings.EqualFold(filepath.Ext(e.Name()), ".md") {
			continue
		}
		p, err := ParsePost(filepath.Join(s.dir, e.Name()))
		if err != nil {
			return err
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
