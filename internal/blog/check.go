package blog

import (
	"fmt"
	"io/fs"
	"net/url"
	"regexp"
	"strings"
)

// SchemaVersion is the shape of the report the check command gives.
//
// It is written down first, so that a script can read it before it reads
// anything else and knows what it is looking at. It changes when the shape
// changes, and for nothing else.
const SchemaVersion = 1

// The levels a finding can have.
const (
	// LevelError is a finding that stops the writing from being right.
	LevelError = "error"
	// LevelWarning is a finding the writer should know about, and that does
	// not stop the site.
	LevelWarning = "warning"
)

// The kinds of thing a finding can be about.
const (
	// KindMissingDepths is a post that does not carry every depth.
	KindMissingDepths = "missing-depths"
	// KindBadDate is a date that cannot be read.
	KindBadDate = "bad-date"
	// KindDuplicateSlug is two files that want the same address.
	KindDuplicateSlug = "duplicate-slug"
	// KindMissingPicture is a picture a post points at that is not there.
	KindMissingPicture = "missing-picture"
)

// Finding is one thing the check command reports.
type Finding struct {
	// Level is "error" or "warning".
	Level string `json:"level"`
	// Kind names the sort of thing it is, so a script can pick it out.
	Kind string `json:"kind"`
	// Slug is the post it is about, when it is about one.
	Slug string `json:"slug,omitempty"`
	// File is the file it is about, when it is about one.
	File string `json:"file,omitempty"`
	// Message says what is wrong, in a sentence.
	Message string `json:"message"`
}

// Report is what the check command gives: the counts of the site, and
// everything it found wrong with it.
type Report struct {
	// Schema is the shape of the report. It is the first field, so a script
	// reads it before anything else.
	Schema int `json:"schema"`
	// Published is the number of posts that carry a depth and are not drafts.
	Published int `json:"published"`
	// Drafts is the number of posts held back.
	Drafts int `json:"drafts"`
	// Incomplete is the number of files that carry no depth at all.
	Incomplete int `json:"incomplete"`
	// Warnings and Errors count the findings.
	Warnings int `json:"warnings"`
	Errors   int `json:"errors"`
	// Findings is every one of them, in the order they were found.
	Findings []Finding `json:"findings"`
}

// Check reads the posts and says what is wrong with them.
//
// The same report is given as text and as JSON: the JSON holds every finding
// the text names, and the same counts.
func (s *Site) Check() Report {
	report := Report{
		Schema:     SchemaVersion,
		Published:  len(s.store.Published()),
		Drafts:     len(s.store.Drafts()),
		Incomplete: len(s.store.Incomplete()),
		Findings:   []Finding{},
	}
	for _, p := range s.store.Index() {
		report.Findings = append(report.Findings, s.depthFindings(p)...)
	}
	for _, p := range s.store.Incomplete() {
		report.Findings = append(report.Findings, s.depthFindings(p)...)
	}
	for _, p := range s.store.Index() {
		report.Findings = append(report.Findings, s.dateFindings(p)...)
	}
	for _, d := range s.store.Duplicates() {
		report.add(Finding{
			Level:   LevelError,
			Kind:    KindDuplicateSlug,
			Slug:    d.Slug,
			File:    d.Second,
			Message: fmt.Sprintf("two files want /post/%s: %s and %s", d.Slug, d.First, d.Second),
		})
	}
	report.Findings = append(report.Findings, s.pictureFindings()...)
	for _, f := range report.Findings {
		switch f.Level {
		case LevelError:
			report.Errors++
		default:
			report.Warnings++
		}
	}
	return report
}

// add puts a finding on the report.
func (r *Report) add(f Finding) {
	r.Findings = append(r.Findings, f)
}

// depthFindings says what a post is missing.
//
// A post that carries no depth at all cannot publish: that is an error,
// unless it is a draft, which is a file being written and not a file gone
// wrong. A post that carries some of the depths may stop early by design, so
// that is a warning.
func (s *Site) depthFindings(p *Post) []Finding {
	if len(p.Missing) == 0 {
		return nil
	}
	if len(p.Missing) == depthCount() {
		if p.Draft {
			return []Finding{{
				Level:   LevelWarning,
				Kind:    KindMissingDepths,
				Slug:    p.Slug,
				File:    p.File,
				Message: "no depth yet: it is a draft, so nothing is written under the headings",
			}}
		}
		return []Finding{{
			Level:   LevelError,
			Kind:    KindMissingDepths,
			Slug:    p.Slug,
			File:    p.File,
			Message: "no depth at all: this file cannot publish",
		}}
	}
	return []Finding{{
		Level:   LevelWarning,
		Kind:    KindMissingDepths,
		Slug:    p.Slug,
		File:    p.File,
		Message: "no depth for: " + strings.Join(p.Missing, ", "),
	}}
}

// dateFindings says what is wrong with the date of a post.
func (s *Site) dateFindings(p *Post) []Finding {
	if p.BadDate == "" {
		return nil
	}
	return []Finding{{
		Level:   LevelError,
		Kind:    KindBadDate,
		Slug:    p.Slug,
		File:    p.File,
		Message: fmt.Sprintf("the date %q cannot be read: it wants a day, as in 2026-01-31", p.BadDate),
	}}
}

// mediaRef finds the pictures a page points at.
//
// The address must begin with /media/: a picture on another host is not ours
// to look for, and the writing does not point at our images folder.
var mediaRef = regexp.MustCompile(`(?:^|[\s"'=(])/media/([^"'\s)]+)`)

// pictureName gives the name of the picture, as the file is named on disk.
//
// A path may end in #left, #right or #center, which says where the picture
// sits on the page. It is not part of the name of the file. A query after the
// name asks for the same picture again, and says nothing about the file. The
// escapes of a URL are read back, so a picture whose name holds a space is
// looked for under the name it has on disk.
func pictureName(path string) string {
	name, _, _ := strings.Cut(path, "#")
	name, _, _ = strings.Cut(name, "?")
	if unescaped, err := url.PathUnescape(name); err == nil {
		return unescaped
	}
	return name
}

// pictureFindings says which pictures the posts point at that are not there.
//
// One picture is reported once for a post, however many depths point at it:
// the writing is wrong in one place, however many times it shows.
func (s *Site) pictureFindings() []Finding {
	var out []Finding
	seen := map[string]bool{}
	for _, p := range s.store.Index() {
		for _, level := range p.Levels {
			for _, m := range mediaRef.FindAllStringSubmatch(string(level.HTML), -1) {
				name := pictureName(m[1])
				key := p.Slug + "\x00" + name
				if seen[key] {
					continue
				}
				seen[key] = true
				if f := s.pictureFinding(p, name); f != nil {
					out = append(out, *f)
				}
			}
		}
	}
	return out
}

// pictureFinding says what is wrong with one picture, if anything.
//
// It gives nothing back when the picture is there.
func (s *Site) pictureFinding(p *Post, name string) *Finding {
	if _, err := fs.Stat(s.media, name); err == nil {
		return nil
	}
	return &Finding{
		Level:   LevelError,
		Kind:    KindMissingPicture,
		Slug:    p.Slug,
		File:    p.File,
		Message: fmt.Sprintf("the picture %s is not in the images folder", name),
	}
}
