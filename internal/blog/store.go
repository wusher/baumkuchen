// Reading the posts, and holding what was read: the index, the drafts,
// and the files that carry no depth at all.
//
// The posts come from a content.Source, so the same store serves a folder on
// disk and a set of files carried inside the program.
package blog

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"path"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/wusher/baumkuchen/internal/content"
)

// VersionWindow is how long the store keeps the mark of the writing without
// reading the disk again.
//
// A browser asks for the mark every half second, and a page that is opened
// asks for it as well. A burst of questions inside this window is answered
// from the mark the store already holds, so the writing is looked at once and
// not once for every question.
const VersionWindow = 300 * time.Millisecond

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
	// markMu guards the mark of the writing and the time it was read.
	markMu sync.Mutex
	// mark is the mark of the writing as it was last read, and markSeen is
	// when it was read. A question that comes inside VersionWindow is
	// answered from these, and the disk is left alone.
	mark     string
	markSeen time.Time
	// warn says what is wrong with the writing. A store that was made
	// without a logger says nothing.
	warn *log.Logger
	// duplicates holds the slugs two files both want. The first one keeps
	// the slug, and the other is named here rather than lost.
	duplicates []Duplicate
}

// Duplicate is a slug that two files both want. They would answer at one
// address, so the first one keeps it and the second is reported.
type Duplicate struct {
	// Slug is the address they both want.
	Slug string
	// First is the file that keeps it.
	First string
	// Second is the file that does not.
	Second string
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

// Version gives a mark for the writing as it is now.
//
// Two calls give the same mark while nothing has changed, and different marks
// when a file has changed. A browser asks for it, so that an edited post
// refreshes the page instead of waiting for the writer to press the button.
//
// The mark is about the files, not about what was read from them: a file
// that was changed and cannot be read still changes the mark, because the
// writer has to see what is wrong with it.
//
// The store keeps the mark and the time it was read. A question that comes
// within VersionWindow of that time is answered from the mark it holds, so
// that a burst of questions reads the disk once and not once for each of
// them.
func (s *Store) Version() string {
	s.markMu.Lock()
	defer s.markMu.Unlock()
	if s.mark != "" && time.Since(s.markSeen) < VersionWindow {
		return s.mark
	}
	s.mark = markOf(s.fingerprint())
	s.markSeen = time.Now()
	return s.mark
}

// remember keeps the mark of the writing, and the time it was read.
//
// The store has just read the disk, so it knows the mark of the writing as it
// is now. A question that comes soon after is answered from this mark, and
// the disk is not read again for nothing.
func (s *Store) remember(print string) {
	s.markMu.Lock()
	s.mark = markOf(print)
	s.markSeen = time.Now()
	s.markMu.Unlock()
}

// markOf gives the mark of a fingerprint.
func markOf(print string) string {
	sum := sha256.Sum256([]byte(print))
	return hex.EncodeToString(sum[:])
}

// Refresh rereads the folder only when a file changed.

func (s *Store) Refresh() error {
	fp := s.fingerprint()
	// the disk was read, so the mark of the writing is known as it is now
	s.remember(fp)
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
	var duplicates []Duplicate
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
			s.warnf("two files want /post/%s: %s and %s", p.Slug, first.File, p.File)
			duplicates = append(duplicates, Duplicate{Slug: p.Slug, First: first.File, Second: p.File})
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
	s.duplicates = duplicates
	s.mu.Unlock()
	return nil
}

// warnf says what is wrong with the writing.
//
// A store that was made without a logger says nothing at all.
func (s *Store) warnf(format string, args ...any) {
	if s.warn == nil {
		return
	}
	s.warn.Printf(format, args...)
}

// Duplicates gives the slugs two files both want, so the check can say so.
func (s *Store) Duplicates() []Duplicate {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.duplicates
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
