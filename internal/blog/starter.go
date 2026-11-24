// The first post for a new site: it carries every depth, so that a writer
// sees the shape of a post and what a depth says.
//
// The headings are the names of the depths, taken from the table of the
// depths, so that the names are said in one place only.
package blog

import (
	"fmt"
	"strings"
)

// StarterPost gives the first post the init command writes.
//
// The post carries every depth, and each depth has a line of its own under
// it, so that the shape of a post is shown.
func StarterPost() string {
	var b strings.Builder
	b.WriteString("---\ndate: 2026-01-01\ntag: hello\n---\n\n# Hello\n")
	for _, spec := range levelSpec {
		fmt.Fprintf(&b, "\n# %s\n\n%s\n", spec.Label, starterLine(spec.Key))
	}
	return b.String()
}

// starterLine gives the line that shows what a depth says.
func starterLine(key string) string {
	switch key {
	case "sentence":
		return "One sentence is the whole of a post at this depth."
	case "paragraph":
		return "A paragraph says the same thing at a little more length."
	case "paragraphs":
		return "Three paragraphs carry the idea through, so a reader can stop at any of them."
	case "page":
		return "A page holds the whole of it, with the detail that belongs to it."
	case "pages":
		return "Three pages hold everything there is to say about it, and more besides."
	}
	return ""
}
