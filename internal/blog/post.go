// A post that is already built, and the questions a template asks of it.
package blog

import (
	"fmt"
	"html/template"
	"regexp"
	"strings"
	"time"
)

func readingTime(words int) string {
	const perMinute = 165.0
	return fmt.Sprintf("%.1f min", float64(words)/perMinute)
}

// cardTime is readingTime for the index, where a rounded zero shows as "<1",
// because a card should not promise that a post takes no time at all.

func cardTime(words int) string {
	t := readingTime(words)
	if strings.HasPrefix(t, "0.0 ") {
		return "<1 min"
	}
	return t
}

// Post is one markdown file from the posts folder.

type Post struct {
	Slug      string
	Title     string
	Date      time.Time
	Tag       string
	Levels    []Level
	Published bool
	Missing   []string
	File      string

	// Draft keeps a post off the built site. It is still on the index and
	// still readable while the server runs, with a mark, so a writer sees it
	// as a reader would.
	Draft bool
	// Pinned holds a page at the top of the index and takes its furniture
	// away: no tag, no date, no depth count. It is a page, not a post.
	Pinned bool
	// Color names the accent this page takes, from the list in parse.go. An
	// empty one leaves the site's own accent in place.
	Color string
	// Depth is the one this post opens at, by its name in the markdown. It
	// beats the depth the reader last chose, and the address beats it.
	Depth string
}

// Lead gives the one sentence version. A post that does not carry that depth
// gives the shortest depth it has.

func (p *Post) Lead() template.HTML {
	for _, l := range p.Levels {
		if l.Key == "sentence" {
			return l.HTML
		}
	}
	if len(p.Levels) > 0 {
		return firstBlock(p.Levels[0].HTML)
	}
	return ""
}

var firstPara = regexp.MustCompile(`(?s)<p>.*?</p>`)

// firstBlock keeps the opening paragraph only, so a card stays a card when the
// shortest depth a post carries is a long one.

func firstBlock(h template.HTML) template.HTML {
	if m := firstPara.FindString(string(h)); m != "" {
		return template.HTML(m)
	}
	return h
}

// Read gives the reading time of the whole post.

func (p *Post) Read() string { return cardTime(p.Words()) }

// WordRange gives the shortest and the longest depth, as "46–1446".

func (p *Post) WordRange() string {
	lo, hi := p.span()
	if lo == hi {
		return fmt.Sprintf("%d", hi)
	}
	return fmt.Sprintf("%d–%d", lo, hi)
}

// ReadRange gives the reading time of those two depths, as "<1–7 min".

func (p *Post) ReadRange() string {
	lo, hi := p.span()
	short, long := cardTime(lo), cardTime(hi)
	if short == long {
		return long
	}
	return strings.TrimSuffix(short, " min") + "–" + long
}

// span gives the word count of the shortest and of the longest depth.

func (p *Post) span() (int, int) {
	if len(p.Levels) == 0 {
		return 0, 0
	}
	lo, hi := p.Levels[0].Words, p.Levels[0].Words
	for _, l := range p.Levels[1:] {
		if l.Words < lo {
			lo = l.Words
		}
		if l.Words > hi {
			hi = l.Words
		}
	}
	return lo, hi
}

// Words gives the word count of the deepest level.

func (p *Post) Words() int {
	n := 0
	for _, l := range p.Levels {
		n += l.Words
	}
	return n
}

func (p *Post) DateText() string {
	if p.Date.IsZero() {
		return ""
	}
	// the short month: "20 Aug 2026", which the page then sets in lower case
	return p.Date.Format("2 Jan 2006")
}
