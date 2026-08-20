package blog

import (
	"html/template"
	"strings"
	"testing"
)

// What a card shows about length: the shortest and the longest depth, the
// whole thing added up, and a time that never reads as zero.
func TestRangesAndCounts(t *testing.T) {
	// arrange
	cases := []struct {
		name                         string
		depths                       map[string]int
		wantWordRange, wantReadRange string
		wantWords                    int
		wantRead                     string
	}{
		{"three depths", map[string]int{"Sentence": 30, "Short": 300, "Long": 1650},
			"30–1650", "0.2–10.0 min", 1980, "12.0 min"},
		{"one depth", map[string]int{"Sentence": 20},
			"20", "0.1 min", 20, "0.1 min"},
		{"under a tenth of a minute", map[string]int{"Sentence": 3},
			"3", "<1 min", 3, "<1 min"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := depthFile(t, t.TempDir(), "p.md", c.depths)

			// act
			wordRange, readRange, words, read := p.WordRange(), p.ReadRange(), p.Words(), p.Read()

			// assert
			if wordRange != c.wantWordRange {
				t.Errorf("WordRange = %q, want %q", wordRange, c.wantWordRange)
			}
			if readRange != c.wantReadRange {
				t.Errorf("ReadRange = %q, want %q", readRange, c.wantReadRange)
			}
			if words != c.wantWords {
				t.Errorf("Words = %d, want %d", words, c.wantWords)
			}
			if read != c.wantRead {
				t.Errorf("Read = %q, want %q", read, c.wantRead)
			}
		})
	}
}

// A post with no depth at all answers with zeroes instead of dividing by none.
func TestRangesWithNoDepth(t *testing.T) {
	// arrange
	path := write(t, t.TempDir(), "bare.md", "# Only A Title\n\nnothing under it.\n")

	// act
	p, err := ParsePost(path)

	// assert
	if err != nil {
		t.Fatal(err)
	}
	if p.Published {
		t.Fatal("a file with no depth must not publish")
	}
	if got := p.WordRange(); got != "0" {
		t.Errorf("WordRange = %q, want %q", got, "0")
	}
	if got := p.Words(); got != 0 {
		t.Errorf("Words = %d, want 0", got)
	}
	if got := p.Lead(); got != "" {
		t.Errorf("Lead = %q, want empty", got)
	}
}

// The lead is the sentence depth. Without one it is the first block of the
// shortest depth the post carries, and never more than that.
func TestLead(t *testing.T) {
	// arrange
	dir := t.TempDir()
	withSentence := write(t, dir, "s.md",
		"# T\n\n# Sentence\n\nThe one sentence.\n\n# Short\n\nA longer version.\n")
	withoutSentence := write(t, dir, "d.md",
		"# T\n\n# Short\n\nFirst block.\n\nSecond block, which must stay off the card.\n")

	// act
	a, err1 := ParsePost(withSentence)
	b, err2 := ParsePost(withoutSentence)

	// assert
	if err1 != nil || err2 != nil {
		t.Fatal(err1, err2)
	}
	if !strings.Contains(string(a.Lead()), "The one sentence") {
		t.Errorf("lead = %q, want the sentence depth", a.Lead())
	}
	if strings.Contains(string(a.Lead()), "A longer version") {
		t.Errorf("lead = %q, want the sentence depth only", a.Lead())
	}
	lead := string(b.Lead())
	if !strings.Contains(lead, "First block") {
		t.Errorf("lead = %q, want the first block", lead)
	}
	if strings.Contains(lead, "Second block") {
		t.Errorf("lead = %q, want one block only", lead)
	}
}

// Html with no paragraph in it comes back whole.
func TestFirstBlockWithNoParagraph(t *testing.T) {
	// arrange
	html := template.HTML("<ul><li>one</li></ul>")

	// act
	got := firstBlock(html)

	// assert
	if got != html {
		t.Errorf("firstBlock = %q, want the whole thing", got)
	}
}

// The date is printed short. Without one the file's own time is used, and a
// post with no date at all prints nothing.
func TestDateText(t *testing.T) {
	// arrange
	dir := t.TempDir()
	dated := depthFile(t, dir, "dated.md", map[string]int{"Sentence": 4})
	undatedPath := write(t, dir, "undated.md", "# T\n\n# Sentence\n\nno front matter here.\n")

	// act
	undated, err := ParsePost(undatedPath)
	empty := &Post{}

	// assert
	if err != nil {
		t.Fatal(err)
	}
	if got := dated.DateText(); got != "4 May 2026" {
		t.Errorf("DateText = %q, want %q", got, "4 May 2026")
	}
	if undated.Date.IsZero() {
		t.Error("a file with no date must fall back to the file's own time")
	}
	if undated.DateText() == "" {
		t.Error("DateText must print that fallback")
	}
	if got := empty.DateText(); got != "" {
		t.Errorf("a post with no date at all gave %q, want empty", got)
	}
}
