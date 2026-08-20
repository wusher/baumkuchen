package blog

import (
	"path/filepath"
	"strings"
	"testing"
)

// A flag is on only for the words that mean yes.
func TestTruthy(t *testing.T) {
	// arrange
	yes := []string{"true", "TRUE", " yes ", "on", "1"}
	no := []string{"", "false", "no", "0", "maybe", "tru"}

	for _, v := range yes {
		// act + assert
		if !truthy(v) {
			t.Errorf("truthy(%q) = false, want true", v)
		}
	}
	for _, v := range no {
		// act + assert
		if truthy(v) {
			t.Errorf("truthy(%q) = true, want false", v)
		}
	}
}

// The store keeps three groups: what the index shows, what the build takes,
// and what cannot render at all.
func TestGroupsHoldDraftsAndIncompleteApart(t *testing.T) {
	// arrange
	dir := t.TempDir()
	postFile(t, dir, "open.md", "---\ndate: 2026-01-02\n---\n", "Open")
	postFile(t, dir, "hidden.md", "---\ndate: 2026-01-01\ndraft: true\n---\n", "Hidden")
	write(t, dir, "notes.md", "# Just A Title\n\nNo depth here.\n")
	s := NewStore(dir)

	// act
	err := s.Refresh()

	// assert
	if err != nil {
		t.Fatal(err)
	}
	if got := slugs(s.Index()); len(got) != 2 {
		t.Fatalf("index = %v, want the post and the draft", got)
	}
	if got := slugs(s.Published()); len(got) != 1 || got[0] != "open" {
		t.Errorf("published = %v, want [open]", got)
	}
	if got := slugs(s.Drafts()); len(got) != 1 || got[0] != "hidden" {
		t.Errorf("drafts = %v, want [hidden]", got)
	}
	if got := slugs(s.Incomplete()); len(got) != 1 || got[0] != "notes" {
		t.Errorf("incomplete = %v, want [notes]", got)
	}
	if _, ok := s.Get("hidden"); !ok {
		t.Error("a draft must still answer at its own URL while the server runs")
	}
	if _, ok := s.Get("notes"); ok {
		t.Error("a file with no depth must not answer at a URL")
	}
}

// A pinned page sits above every post, whatever its date, and the posts under
// it stay newest first.
func TestPinnedComesFirst(t *testing.T) {
	// arrange
	dir := t.TempDir()
	postFile(t, dir, "new.md", "---\ndate: 2026-06-01\n---\n", "Newest")
	postFile(t, dir, "old.md", "---\ndate: 2020-01-01\n---\n", "Oldest")
	postFile(t, dir, "about.md", "---\ndate: 2019-01-01\npinned: true\n---\n", "About")
	s := NewStore(dir)

	// act
	if err := s.Refresh(); err != nil {
		t.Fatal(err)
	}
	got := slugs(s.Index())

	// assert
	want := []string{"about", "new", "old"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("order = %v, want %v", got, want)
	}
	if !s.Index()[0].Pinned {
		t.Error("the first one must be the pinned page")
	}
}

// splitPinned cuts the list where the pinned pages stop.
func TestSplitPinned(t *testing.T) {
	// arrange
	cases := []struct {
		name              string
		files             map[string]string
		wantPin, wantRest int
	}{
		{"both groups", map[string]string{
			"a.md": "---\ndate: 2026-01-02\npinned: true\n---\n",
			"b.md": "---\ndate: 2026-01-01\n---\n"}, 1, 1},
		{"all pinned", map[string]string{
			"a.md": "---\ndate: 2026-01-02\npinned: true\n---\n",
			"b.md": "---\ndate: 2026-01-01\npinned: true\n---\n"}, 2, 0},
		{"no pinned", map[string]string{
			"a.md": "---\ndate: 2026-01-02\n---\n",
			"b.md": "---\ndate: 2026-01-01\n---\n"}, 0, 2},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			for name, front := range c.files {
				postFile(t, dir, name, front, name)
			}
			s := NewStore(dir)
			if err := s.Refresh(); err != nil {
				t.Fatal(err)
			}

			// act
			pinned, rest := splitPinned(s.Index())

			// assert
			if len(pinned) != c.wantPin || len(rest) != c.wantRest {
				t.Errorf("split gave %d pinned and %d posts, want %d and %d",
					len(pinned), len(rest), c.wantPin, c.wantRest)
			}
		})
	}
}

// A folder that is not there is reported, not guessed at.
func TestStoreOnAMissingFolder(t *testing.T) {
	// arrange
	s := NewStore(filepath.Join(t.TempDir(), "no-such-folder"))

	// act
	print := s.fingerprint()
	err := s.Refresh()

	// assert
	if print != "missing" {
		t.Errorf("fingerprint = %q, want %q", print, "missing")
	}
	if err == nil {
		t.Error("reading a folder that is not there must give an error")
	}
}

// The folder is read again only when something in it changed.
func TestRefreshOnlyRereadsWhenAFileChanged(t *testing.T) {
	// arrange
	dir := t.TempDir()
	postFile(t, dir, "one.md", "---\ndate: 2026-01-01\n---\n", "One")
	s := NewStore(dir)
	if err := s.Refresh(); err != nil {
		t.Fatal(err)
	}
	first := s.Index()[0]

	// act
	if err := s.Refresh(); err != nil { // nothing changed
		t.Fatal(err)
	}
	same := s.Index()[0]
	postFile(t, dir, "two.md", "---\ndate: 2026-01-02\n---\n", "Two")
	if err := s.Refresh(); err != nil { // now something did
		t.Fatal(err)
	}

	// assert
	if same != first {
		t.Error("an unchanged folder must not be read again")
	}
	if got := len(s.Index()); got != 2 {
		t.Errorf("index holds %d, want 2 after a file was added", got)
	}
}
