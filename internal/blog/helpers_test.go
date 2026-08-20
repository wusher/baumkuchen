package blog

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// write puts one file in a folder and gives back its path.
func write(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// postFile writes a small post that publishes, with whatever front matter the
// test needs.
func postFile(t *testing.T, dir, name, front, title string) {
	t.Helper()
	write(t, dir, name, front+"\n# "+title+"\n\n# Sentence\n\nA short line about it.\n")
}

// depthFile writes a post carrying the depths named, each of n words, and
// gives back the parsed post.
func depthFile(t *testing.T, dir, name string, depths map[string]int) *Post {
	t.Helper()
	var b strings.Builder
	b.WriteString("---\ndate: 2026-05-04\n---\n\n# A Title\n")
	for _, d := range []string{"Sentence", "Paragraph", "Short", "Medium", "Long"} {
		n, ok := depths[d]
		if !ok {
			continue
		}
		b.WriteString("\n# " + d + "\n\n" + strings.TrimSpace(strings.Repeat("word ", n)) + "\n")
	}
	p, err := ParsePost(write(t, dir, name, b.String()))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// testSite builds a site on a temporary posts folder, with the real templates
// and stylesheet from the top of the tree. //go:embed cannot reach up out of a
// package, so a test reads them from disk instead.
func testSite(t *testing.T, dir string) *Site {
	t.Helper()
	root := os.DirFS("../..")
	s, err := New(Config{Posts: dir, Templates: root, Static: root})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// siteWithPosts builds a site holding one post, one draft, one pinned page,
// one file with no depth, and one picture.
func siteWithPosts(t *testing.T) (*Site, string) {
	t.Helper()
	dir := t.TempDir()
	postFile(t, dir, "open.md", "---\ndate: 2026-01-02\ntag: earth\n---\n", "Open")
	postFile(t, dir, "hidden.md", "---\ndate: 2026-01-01\ndraft: true\n---\n", "Hidden")
	postFile(t, dir, "about.md", "---\ndate: 2025-01-01\npinned: true\n---\n", "About")
	write(t, dir, "notes.md", "# No Depth Here\n\nnothing.\n")
	if err := os.MkdirAll(filepath.Join(dir, "images"), 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(dir, "images"), "pic.jpg", "not really a picture")
	return testSite(t, dir), dir
}

func keys(p *Post) []string {
	var k []string
	for _, l := range p.Levels {
		k = append(k, l.Key)
	}
	return k
}

func slugs(ps []*Post) []string {
	var out []string
	for _, p := range ps {
		out = append(out, p.Slug)
	}
	return out
}

var paraTag = regexp.MustCompile(`(?s)<p>(.*?)</p>`)
var htmlTag = regexp.MustCompile(`<[^>]*>`)

// prose gives the word count of each paragraph of a rendered depth. A picture
// and its caption share one paragraph and are not prose, so they are skipped.
func prose(html string) []int {
	var out []int
	for _, m := range paraTag.FindAllStringSubmatch(html, -1) {
		if strings.Contains(m[1], "<img") {
			continue
		}
		out = append(out, len(strings.Fields(htmlTag.ReplaceAllString(m[1], " "))))
	}
	return out
}
