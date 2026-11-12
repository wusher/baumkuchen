package blog

import (
	"path/filepath"
	"strings"
	"testing"
)

// The index carries no label over either group: the pages stand in a row at
// the top, and the posts follow, and nothing says which is which.
func TestIndexHasNoLabels(t *testing.T) {
	// arrange
	dir := t.TempDir()
	postFile(t, dir, "about.md", "---\ndate: 2026-01-01\npinned: true\n---\n", "About")
	postFile(t, dir, "one.md", "---\ndate: 2026-02-01\n---\n", "One")
	site := testSite(t, dir)

	// act
	index := servedIndex(site)

	// assert
	if strings.Contains(index, "eyebrow") {
		t.Error("the index still carries a label over a group")
	}
	for _, gone := range []string{">pages<", ">posts<"} {
		if strings.Contains(index, gone) {
			t.Errorf("the index still shows the label %s", gone)
		}
	}
	// the groups keep their names for anyone who cannot see the page
	for _, want := range []string{`aria-label="pages"`, `aria-label="posts"`} {
		if !strings.Contains(index, want) {
			t.Errorf("the index does not name the group %s", want)
		}
	}
}

// The pages stand together in one list at the top of the index, and the posts
// follow in another. How many of each there are changes nothing.
func TestIndexGroupsThePagesAndThePosts(t *testing.T) {
	type file struct {
		front string
		title string
	}
	cases := []struct {
		name      string
		files     []file
		wantPages []string
		wantPosts []string
		wantRule  bool
	}{
		{"nothing at all", nil, nil, nil, false},
		{"one page", []file{
			{"---\ndate: 2026-01-01\npinned: true\n---\n", "About"},
		}, []string{"About"}, nil, false},
		{"two pages", []file{
			{"---\ndate: 2026-01-01\npinned: true\n---\n", "About"},
			{"---\ndate: 2026-01-02\npinned: true\n---\n", "Colophon"},
		}, []string{"About", "Colophon"}, nil, false},
		{"one post", []file{
			{"---\ndate: 2026-02-01\n---\n", "One"},
		}, nil, []string{"One"}, false},
		{"a page and a post", []file{
			{"---\ndate: 2026-01-01\npinned: true\n---\n", "About"},
			{"---\ndate: 2026-02-01\n---\n", "One"},
		}, []string{"About"}, []string{"One"}, true},
		{"two pages and two posts", []file{
			{"---\ndate: 2026-01-01\npinned: true\n---\n", "About"},
			{"---\ndate: 2026-01-02\npinned: true\n---\n", "Colophon"},
			{"---\ndate: 2026-02-01\n---\n", "One"},
			{"---\ndate: 2026-03-01\n---\n", "Two"},
		}, []string{"About", "Colophon"}, []string{"One", "Two"}, true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// arrange
			dir := t.TempDir()
			for i, f := range c.files {
				postFile(t, dir, fileName(i)+".md", f.front, f.title)
			}
			site := testSite(t, dir)

			// act
			index := servedIndex(site)

			// assert
			pages := group(index, "pages")
			posts := group(index, "posts")
			if len(c.wantPages) == 0 && pages != "" {
				t.Errorf("the index holds a row of pages, want none:\n%s", pages)
			}
			if len(c.wantPages) > 0 && pages == "" {
				t.Error("the index holds no row of pages")
			}
			if got := strings.Count(pages, `class="card"`); got != len(c.wantPages) {
				t.Errorf("the row of pages holds %d entries, want %d", got, len(c.wantPages))
			}
			for _, title := range c.wantPages {
				if !strings.Contains(pages, ">"+title+"<") {
					t.Errorf("the row of pages does not hold %q", title)
				}
			}
			for _, title := range c.wantPosts {
				if !strings.Contains(posts, ">"+title+"<") {
					t.Errorf("the posts do not hold %q", title)
				}
			}
			if got := strings.Contains(index, `<hr class="split">`); got != c.wantRule {
				t.Errorf("rule drawn = %v, want %v", got, c.wantRule)
			}
			if pages != "" && posts != "" &&
				strings.Index(index, pages) > strings.Index(index, posts) {
				t.Error("the row of pages stands below the posts")
			}
		})
	}
}

// Every page keeps its own link, and the row of pages is one list.
func TestThePagesKeepTheirLinks(t *testing.T) {
	// arrange
	dir := t.TempDir()
	postFile(t, dir, "about.md", "---\ndate: 2026-01-01\npinned: true\n---\n", "About")
	postFile(t, dir, "colophon.md", "---\ndate: 2026-01-02\npinned: true\n---\n", "Colophon")
	postFile(t, dir, "one.md", "---\ndate: 2026-02-01\n---\n", "One")
	site := testSite(t, dir)

	// act
	index := servedIndex(site)

	// assert
	pages := group(index, "pages")
	if strings.Count(pages, "<li") != 2 {
		t.Errorf("the row of pages holds %d entries, want 2", strings.Count(pages, "<li"))
	}
	for _, want := range []string{`href="/post/about"`, `href="/post/colophon"`} {
		if !strings.Contains(pages, want) {
			t.Errorf("the row of pages does not hold the link %s", want)
		}
	}
	if strings.Contains(pages, `href="/post/one"`) {
		t.Error("a post stands in the row of pages")
	}
}

// The pages stand in a row that wraps, so a narrow window keeps them in the
// same order instead of running them down the page.
func TestThePagesStandInARow(t *testing.T) {
	// arrange
	css := readFile(t, filepath.Join("..", "..", "static", "style.css"))

	// act
	i := strings.Index(css, ".feed.pinned {")
	if i < 0 {
		t.Fatal("the stylesheet holds no rule for the row of pages")
	}
	rule := css[i:]
	if end := strings.Index(rule, "}"); end > 0 {
		rule = rule[:end]
	}

	// assert
	for _, want := range []string{"display: flex", "flex-wrap: wrap"} {
		if !strings.Contains(rule, want) {
			t.Errorf("the row of pages does not carry %q", want)
		}
	}
}

// fileName gives a file its own name, so two files in one folder cannot hold
// the same slug.
func fileName(i int) string {
	return []string{"first", "second", "third", "fourth"}[i]
}

// group gives the part of the index that belongs to the list with the name,
// so a test can look at the pages or the posts on their own.
func group(index, name string) string {
	at := strings.Index(index, `aria-label="`+name+`"`)
	if at < 0 {
		return ""
	}
	start := strings.LastIndex(index[:at], "<ol")
	if start < 0 {
		return ""
	}
	end := strings.Index(index[start:], "</ol>")
	if end < 0 {
		return ""
	}
	return index[start : start+end]
}
