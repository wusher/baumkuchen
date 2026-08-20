package blog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// One depth is enough to publish, and the file name is the slug.
func TestOneDepthPublishes(t *testing.T) {
	// arrange
	dir := t.TempDir()
	path := write(t, dir, "why-rivers-meander.md", "# A Quote\n\n# Sentence\n\nOnly this.\n")

	// act
	p, err := ParsePost(path)

	// assert
	if err != nil {
		t.Fatal(err)
	}
	if !p.Published {
		t.Fatalf("want published, got draft, missing: %v", p.Missing)
	}
	if got := keys(p); len(got) != 1 || got[0] != "sentence" {
		t.Fatalf("levels = %v, want [sentence]", got)
	}
	if p.Slug != "why-rivers-meander" {
		t.Errorf("slug = %q, want the file name", p.Slug)
	}
	if p.Title != "A Quote" {
		t.Errorf("title = %q, want the first heading", p.Title)
	}
}

// A post that skips depths, and writes them out of order, still reads in the
// order the five names are declared in.
func TestGappedPostKeepsSpecOrder(t *testing.T) {
	// arrange
	dir := t.TempDir()
	path := write(t, dir, "gap.md", strings.Join([]string{
		"# Gapped",
		"",
		"# Long", "", "Long text.", "",
		"# Sentence", "", "Short text.", "",
		"# Short", "", "Middle text.", "",
	}, "\n"))

	// act
	p, err := ParsePost(path)

	// assert
	if err != nil {
		t.Fatal(err)
	}
	if !p.Published {
		t.Fatal("want published, got draft")
	}
	want := []string{"sentence", "paragraphs", "pages"}
	if got := keys(p); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("levels = %v, want %v", got, want)
	}
	if strings.Join(p.Missing, ",") != "Paragraph,Medium" {
		t.Fatalf("missing = %v, want [Paragraph Medium]", p.Missing)
	}
}

// A file with no depth heading, and a heading with nothing under it, both stay
// off the site.
func TestNoDepthDoesNotPublish(t *testing.T) {
	// arrange
	dir := t.TempDir()
	bare := write(t, dir, "empty.md", "# Nothing\n\nJust prose, with no depth heading.\n")
	hollow := write(t, dir, "hollow.md", "# Nothing\n\n# Sentence\n\n# Short\n")

	// act
	bareP, err1 := ParsePost(bare)
	hollowP, err2 := ParsePost(hollow)

	// assert
	if err1 != nil || err2 != nil {
		t.Fatal(err1, err2)
	}
	if bareP.Published {
		t.Error("a file with no depth heading must not publish")
	}
	if hollowP.Published {
		t.Error("a heading with nothing under it is not a depth")
	}
	if len(hollowP.Missing) != 5 {
		t.Errorf("missing lists %d depths, want all 5", len(hollowP.Missing))
	}
}

// A picture and its caption do not count toward the length of a depth.
func TestPictureAndCaptionAreNotProse(t *testing.T) {
	// arrange
	dir := t.TempDir()
	path := write(t, dir, "pic.md", "# Pic\n\n# Sentence\n\nFour words go here.\n\n"+
		"![A long piece of alternative text](/media/x.jpg)\n*A caption that runs on for a while.*\n")

	// act
	p, err := ParsePost(path)

	// assert
	if err != nil {
		t.Fatal(err)
	}
	if got := p.Levels[0].Words; got != 4 {
		t.Fatalf("words = %d, want 4", got)
	}
}

// The front matter is the block at the top, and only when it is closed.
func TestFrontMatter(t *testing.T) {
	// arrange
	cases := []struct {
		name     string
		src      string
		wantKeys int
		bodyHead string
	}{
		{"closed", "---\ndate: 2026-01-01\ntag: earth\n---\n\n# A Title\n", 2, "\n# A Title"},
		{"never closed", "---\ndate: 2026-01-01\n\n# A Title\n", 0, "---"},
		{"none at all", "# A Title\n\ntext\n", 0, "# A Title"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// act
			meta, body := splitFrontMatter(c.src)

			// assert
			if len(meta) != c.wantKeys {
				t.Errorf("meta = %v, want %d key(s)", meta, c.wantKeys)
			}
			if !strings.HasPrefix(body, c.bodyHead) {
				t.Errorf("body starts %q, want it to start %q", head(body), c.bodyHead)
			}
		})
	}
}

func head(s string) string {
	if len(s) > 20 {
		return s[:20]
	}
	return s
}

// A heading is matched without case or punctuation, and one misspelling is
// forgiven. Anything else keeps its own words, so the caller can report it.
func TestNormalizeHead(t *testing.T) {
	// arrange
	cases := map[string]string{
		"Sentence":      "sentence",
		"# SHORT":       "short",
		"  Medium  ":    "medium",
		"Long!":         "long",
		"# Sentance":    "sentence",
		"## Two Pages!": "two pages",
	}

	for in, want := range cases {
		t.Run(in, func(t *testing.T) {
			// act
			got := normalizeHead(in)

			// assert
			if got != want {
				t.Errorf("normalizeHead(%q) = %q, want %q", in, got, want)
			}
		})
	}
}

// House style, on the real posts: a Sentence depth stays at or under 35 words,
// and at most one paragraph in four may run to 120 words or more.
func TestRealPostsKeepHouseStyle(t *testing.T) {
	// arrange: the real posts, which sit two folders up
	paths, err := filepath.Glob(filepath.Join("..", "..", "posts", "*.md"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("no posts found: %v", err)
	}

	for _, path := range paths {
		// act
		p, err := ParsePost(path)
		if err != nil {
			t.Fatal(err)
		}

		// assert
		for _, l := range p.Levels {
			paras := prose(string(l.HTML))
			if l.Key == "sentence" {
				words := 0
				for _, n := range paras {
					words += n
				}
				if words > 35 {
					t.Errorf("%s: sentence depth is %d words, want <= 35", p.Slug, words)
				}
			}
			over := 0
			for _, n := range paras {
				if n >= 120 {
					over++
				}
			}
			if allow := len(paras) / 4; over > allow {
				t.Errorf("%s/%s: %d of %d paragraphs reach 120 words, allowed %d",
					p.Slug, l.Label, over, len(paras), allow)
			}
		}
	}
}

// A post may name its own accent. A name that is not one of the known ones is
// dropped, so a typo leaves the page with the site's accent.
func TestColorName(t *testing.T) {
	// arrange
	cases := map[string]string{
		"emerald":   "emerald",
		"  Teal  ":  "teal",
		"ROSE":      "rose",
		"turquoise": "",
		"":          "",
		"#ff0000":   "",
	}

	for in, want := range cases {
		t.Run(in, func(t *testing.T) {
			// act
			got := colorName(in)

			// assert
			if got != want {
				t.Errorf("colorName(%q) = %q, want %q", in, got, want)
			}
		})
	}
}

// The colour reaches the post, and every name in the list has a value in the
// stylesheet.
func TestColorReachesThePostAndTheStylesheet(t *testing.T) {
	// arrange
	dir := t.TempDir()
	good := write(t, dir, "good.md", "---\ncolor: emerald\n---\n\n# T\n\n# Sentence\n\nfine.\n")
	typo := write(t, dir, "typo.md", "---\ncolor: turquoise\n---\n\n# T\n\n# Sentence\n\nfine.\n")

	// act
	a, err1 := ParsePost(good)
	b, err2 := ParsePost(typo)
	css, err3 := os.ReadFile(filepath.Join("..", "..", "static", "style.css"))

	// assert
	if err1 != nil || err2 != nil || err3 != nil {
		t.Fatal(err1, err2, err3)
	}
	if a.Color != "emerald" {
		t.Errorf("Color = %q, want %q", a.Color, "emerald")
	}
	if b.Color != "" {
		t.Errorf("Color = %q, want empty for a name that is not known", b.Color)
	}
	for name := range colorNames {
		want := `[data-color="` + name + `"]`
		if !strings.Contains(string(css), want) {
			t.Errorf("the stylesheet has no rule for %s", want)
		}
	}
}

// A post may say which depth it opens at, by the same names the headings use.
func TestDepthName(t *testing.T) {
	// arrange
	cases := map[string]string{
		"short":    "short",
		" Medium ": "medium",
		"LONG":     "long",
		"Sentance": "sentence",
		"huge":     "",
		"":         "",
		"2":        "",
	}

	for in, want := range cases {
		t.Run(in, func(t *testing.T) {
			// act
			got := depthName(in)

			// assert
			if got != want {
				t.Errorf("depthName(%q) = %q, want %q", in, got, want)
			}
		})
	}
}
