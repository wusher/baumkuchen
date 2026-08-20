package blog

import (
	"strings"
	"testing"
)

// A fence wraps a group of blocks, and what is inside it stays markdown.
func TestCentreFenceKeepsMarkdown(t *testing.T) {
	// arrange
	body := strings.Join([]string{
		"# A Title", "", "# Sentence", "",
		"::: center", "",
		"A **bold** word and a [link](https://example.com).", "",
		"- one", "- two", "",
		":::", "",
		"Plain text after it.",
	}, "\n")
	path := write(t, t.TempDir(), "mid.md", body)

	// act
	p, err := ParsePost(path)

	// assert
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Levels) != 1 {
		t.Fatalf("levels = %d, want 1", len(p.Levels))
	}
	html := string(p.Levels[0].HTML)
	if !strings.Contains(html, `<div class="center">`) {
		t.Errorf("no centred div in:\n%s", html)
	}
	for _, want := range []string{"<strong>bold</strong>", `<a href="https://example.com"`, "<li>one</li>"} {
		if !strings.Contains(html, want) {
			t.Errorf("the markdown inside was not read: %s missing from\n%s", want, html)
		}
	}
	if !strings.Contains(html, "Plain text after it.") {
		t.Error("the text after the block is gone")
	}
	if strings.Contains(html, ":::") {
		t.Error("a marker reached the page")
	}
	if strings.Count(html, "<div") != strings.Count(html, "</div>") {
		t.Errorf("the div is not balanced:\n%s", html)
	}
	if p.Levels[0].Words != 14 {
		t.Errorf("words = %d, want 14 (the two markers do not count)", p.Levels[0].Words)
	}
}

// A fence that is never closed still ends, and does not eat the page.
func TestCentreFenceLeftOpenIsClosed(t *testing.T) {
	// arrange
	path := write(t, t.TempDir(), "open.md", "# T\n\n# Sentence\n\n::: center\n\nOnly this.\n")

	// act
	p, err := ParsePost(path)

	// assert
	if err != nil {
		t.Fatal(err)
	}
	html := string(p.Levels[0].HTML)
	if strings.Count(html, "<div") != strings.Count(html, "</div>") {
		t.Errorf("the div is not balanced:\n%s", html)
	}
}

// The arrow marks a line, or a run of them, and the run ends at the first line
// without one.
func TestArrowCentresARun(t *testing.T) {
	// arrange
	body := strings.Join([]string{
		"# T", "", "# Sentence", "",
		"-> **one** line in the middle",
		"-> and a [link](https://example.com)", "",
		"An ordinary paragraph after it.",
	}, "\n")
	path := write(t, t.TempDir(), "arrow.md", body)

	// act
	p, err := ParsePost(path)

	// assert
	if err != nil {
		t.Fatal(err)
	}
	html := string(p.Levels[0].HTML)
	if !strings.Contains(html, `<div class="center">`) {
		t.Errorf("no centred div in:\n%s", html)
	}
	if strings.Contains(html, "-&gt;") || strings.Contains(html, "->") {
		t.Errorf("a marker reached the page:\n%s", html)
	}
	if !strings.Contains(html, "<strong>one</strong>") {
		t.Errorf("the markdown after the marker was not read:\n%s", html)
	}
	i, j := strings.Index(html, "</div>"), strings.Index(html, "An ordinary paragraph")
	if i < 0 || j < 0 || j < i {
		t.Errorf("the run did not end at the blank line:\n%s", html)
	}
	if strings.Count(html, "<div") != strings.Count(html, "</div>") {
		t.Errorf("the div is not balanced:\n%s", html)
	}
	if p.Levels[0].Words != 13 {
		t.Errorf("words = %d, want 13 (the arrows do not count)", p.Levels[0].Words)
	}
}

// Inside a code fence a marker is text, whichever marker it is.
func TestMarkersInCodeStayText(t *testing.T) {
	// arrange
	cases := map[string]string{
		"the fence": "::: center",
		"the arrow": "-> not a marker",
	}

	for name, marker := range cases {
		t.Run(name, func(t *testing.T) {
			path := write(t, t.TempDir(), "code.md",
				"# T\n\n# Sentence\n\n```\n"+marker+"\n```\n")

			// act
			p, err := ParsePost(path)

			// assert
			if err != nil {
				t.Fatal(err)
			}
			html := string(p.Levels[0].HTML)
			if strings.Contains(html, `<div class="center">`) {
				t.Errorf("a marker inside a fence made a block:\n%s", html)
			}
			if !strings.Contains(html, strings.ReplaceAll(marker, ">", "&gt;")) {
				t.Errorf("the fence lost its text:\n%s", html)
			}
		})
	}
}
