// How a markdown file becomes a post: the five depth names, the front matter,
// the block markers, and the walk that cuts a file into depths.
package blog

import (
	"bytes"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	gmhtml "github.com/yuin/goldmark/renderer/html"
)

// A post is published when it holds one or more of these depth sections. The
// order of this list is the order of the depth slider, whichever ones a post
// carries. Missing keeps the rest, for the startup log.
var levelSpec = []struct {
	Key   string
	Head  string // the normalized H2 text that the file must contain
	Label string
}{
	{"sentence", "sentence", "Sentence"},
	{"paragraph", "paragraph", "Paragraph"},
	{"paragraphs", "short", "Short"},
	{"page", "medium", "Medium"},
	{"pages", "long", "Long"},
}

// The five names above are the only headings that publish a depth. The one
// alias here catches a common misspelling; the older, longer names
// ("one sentence", "three paragraphs", "one page", "three pages") do not work.

var headAlias = map[string]string{
	"sentance": "sentence",
}

// Level is one depth of one post.

type Level struct {
	Key   string
	Label string
	HTML  template.HTML
	Words int
	Read  string
}

// readingTime turns a word count into "0.2 min" or "8.8 min", at the usual
// reading speed for prose of about 165 words a minute.

var md = goldmark.New(
	goldmark.WithExtensions(extension.Typographer, extension.Footnote, extension.Table),
	goldmark.WithRendererOptions(gmhtml.WithUnsafe()),
)

var (
	fenceRe    = regexp.MustCompile("^\\s{0,3}(```|~~~)")
	nonWord    = regexp.MustCompile(`[^a-z0-9]+`)
	centerOpen = regexp.MustCompile(`(?i)^:::[ \t]*center[ \t]*$`)
	blockClose = regexp.MustCompile(`^:::[ \t]*$`)
	arrowLine  = regexp.MustCompile(`^->([ \t]|$)`)
)

func normalizeHead(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.Trim(s, "#*_ ")
	s = nonWord.ReplaceAllString(s, " ")
	s = strings.TrimSpace(s)
	if a, ok := headAlias[s]; ok {
		return a
	}
	return s
}

// expandBlocks turns either of these
//
//	-> one line in the middle
//
//	::: center
//	anything at all
//	:::
//
// into a div around the same markdown. The arrow is a marker at the head of a
// line, the way > marks a quotation, and its block ends at the first line
// without one. The fence wraps a group of blocks at once. A blank line on each side of the two
// html lines is what keeps markdown reading what is between them, so a
// picture, a list or a heading in the middle is still a picture, a list or a
// heading. A block left open at the end of a depth is closed here. Nothing
// inside a code fence is touched.

func expandBlocks(src string) string {
	lines := strings.Split(src, "\n")
	out := make([]string, 0, len(lines)+4)
	inFence, open, arrow := false, false, false
	for _, line := range lines {
		if fenceRe.MatchString(line) {
			inFence = !inFence
			out = append(out, line)
			continue
		}
		if !inFence {
			t := strings.TrimSpace(line)
			// a run of arrow lines, which ends at the first line without one,
			// the way a quotation ends at a blank line
			if !open {
				if arrowLine.MatchString(t) {
					if !arrow {
						out = append(out, `<div class="center">`, "")
						arrow = true
					}
					out = append(out, strings.TrimSpace(arrowLine.ReplaceAllString(t, "")))
					continue
				}
				if arrow {
					out = append(out, "", "</div>")
					arrow = false
				}
			}
			if !open && centerOpen.MatchString(t) {
				out = append(out, `<div class="center">`, "")
				open = true
				continue
			}
			if open && blockClose.MatchString(t) {
				out = append(out, "", "</div>")
				open = false
				continue
			}
		}
		out = append(out, line)
	}
	if arrow || open {
		out = append(out, "", "</div>")
	}
	return strings.Join(out, "\n")
}

// proseWords counts the words a reader reads, so a picture and its caption do
// not swell the length of a depth. A caption is the italic line under a picture.

func proseWords(src string) int {
	n := 0
	lines := strings.Split(src, "\n")
	for i, line := range lines {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "![") {
			continue
		}
		if strings.HasPrefix(t, ":::") { // a block marker, not a word
			continue
		}
		if arrowLine.MatchString(t) { // the arrow is a marker, the rest is prose
			t = strings.TrimSpace(arrowLine.ReplaceAllString(t, ""))
		}
		if i > 0 && strings.HasPrefix(t, "*") && strings.HasSuffix(t, "*") &&
			strings.HasPrefix(strings.TrimSpace(lines[i-1]), "![") {
			continue
		}
		n += len(strings.Fields(t))
	}
	return n
}

// section holds the raw markdown under one H2.

type section struct {
	head string
	body []string
}

// splitFrontMatter reads the optional key: value block at the top of a file.

func splitFrontMatter(src string) (map[string]string, string) {
	meta := map[string]string{}
	lines := strings.Split(src, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return meta, src
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			return meta, strings.Join(lines[i+1:], "\n")
		}
		if k, v, ok := strings.Cut(lines[i], ":"); ok {
			meta[strings.ToLower(strings.TrimSpace(k))] = strings.TrimSpace(v)
		}
	}
	// A block that is never closed is not front matter at all. Without this,
	// every "key: value" line in the whole file would be read as a setting,
	// and a line of prose could hide the post.
	return map[string]string{}, src
}

// colorNames are the accents a post may ask for by name. The words are the
// ones Tailwind uses, because they are easy to remember; the values are in
// static/style.css and are this site's own.
var colorNames = map[string]bool{
	"slate":   true,
	"gray":    true,
	"zinc":    true,
	"neutral": true,
	"stone":   true,
	"red":     true,
	"orange":  true,
	"amber":   true,
	"yellow":  true,
	"lime":    true,
	"green":   true,
	"emerald": true,
	"teal":    true,
	"cyan":    true,
	"sky":     true,
	"blue":    true,
	"indigo":  true,
	"violet":  true,
	"purple":  true,
	"fuchsia": true,
	"pink":    true,
	"rose":    true,
}

// colorName reads the color line of the front matter. A name that is not one
// of the known ones is dropped, so a typo leaves the page as it was instead of
// giving it no accent at all.
func colorName(v string) string {
	name := strings.ToLower(strings.TrimSpace(v))
	if colorNames[name] {
		return name
	}
	return ""
}

// depthName reads the depth line of the front matter: the name a post opens
// at. It takes the same words the headings take, so "short" and "Short" and
// the "sentance" misspelling all work. A name that is not one of the five is
// dropped, and the post opens the way every other post does.
func depthName(v string) string {
	name := normalizeHead(v)
	for _, spec := range levelSpec {
		if name == spec.Head {
			return spec.Head
		}
	}
	return ""
}

// truthy reads a front matter flag. A key that is absent, empty or anything
// else is false, so a half-written line never publishes by mistake.

func truthy(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "true", "yes", "on", "1":
		return true
	}
	return false
}

// ParsePost reads one markdown file and cuts it into depth levels.

func ParsePost(path string) (*Post, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	meta, body := splitFrontMatter(string(raw))

	p := &Post{
		Slug:   strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)),
		Tag:    meta["tag"],
		File:   path,
		Draft:  truthy(meta["draft"]),
		Pinned: truthy(meta["pinned"]),
		Color:  colorName(meta["color"]),
		Depth:  depthName(meta["depth"]),
	}
	if d, ok := meta["date"]; ok {
		if t, err := time.Parse("2006-01-02", d); err == nil {
			p.Date = t
		}
	}
	if p.Date.IsZero() {
		if fi, err := os.Stat(path); err == nil {
			p.Date = fi.ModTime()
		}
	}

	var secs []section
	var cur *section
	inFence := false
	for _, line := range strings.Split(body, "\n") {
		if fenceRe.MatchString(line) {
			inFence = !inFence
		}
		if !inFence {
			// Every heading of one # is a marker: the first one is the title,
			// and each one after it names a depth. A heading of two # or more
			// is ordinary text inside the depth it sits in.
			if t := strings.TrimSpace(line); strings.HasPrefix(t, "# ") {
				name := strings.TrimSpace(strings.TrimPrefix(t, "# "))
				if p.Title == "" {
					p.Title = name
					continue
				}
				secs = append(secs, section{head: normalizeHead(name)})
				cur = &secs[len(secs)-1]
				continue
			}
		}
		if cur != nil {
			cur.body = append(cur.body, line)
		}
	}
	if p.Title == "" {
		p.Title = p.Slug
	}

	found := map[string]string{}
	for _, s := range secs {
		found[s.head] = strings.Join(s.body, "\n")
	}

	for _, spec := range levelSpec {
		text, ok := found[spec.Head]
		if !ok || strings.TrimSpace(text) == "" {
			p.Missing = append(p.Missing, spec.Label)
			continue
		}
		var buf bytes.Buffer
		if err := md.Convert([]byte(expandBlocks(text)), &buf); err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		words := proseWords(text)
		p.Levels = append(p.Levels, Level{
			Key:   spec.Key,
			Label: spec.Label,
			HTML:  template.HTML(buf.String()),
			Words: words,
			Read:  readingTime(words),
		})
	}
	p.Published = len(p.Levels) > 0
	return p, nil
}

// Store keeps the parsed posts and reloads them when a file changes.
