// Package content is where the posts and the pictures come from.
//
// A site can be built from a folder on disk, which is what a writer works in,
// or from a set of files carried inside the program, which is what a copy of
// the program needs when it runs away from the folder it was made in. Both
// answer the same questions, so nothing above this package has to know which
// one it is holding.
package content

import "io/fs"

// MediaDir is the folder the pictures live in, beside the posts.
const MediaDir = "images"

// Source is one place the content comes from.
//
// Every path it gives is relative to the root of the content and uses forward
// slashes, whatever the system, so the same content reads the same way
// everywhere and a file inside the program looks the same as a file on disk.
type Source interface {
	// Label names the source, so a log says where the content came from.
	Label() string
	// List gives every markdown file it holds, in order. The pictures and
	// the hidden folders are left out: they are not posts.
	List() ([]string, error)
	// Read gives the bytes of one file, by the path List gave.
	Read(path string) ([]byte, error)
	// Stat gives the size and the time of one file. The time is what the
	// site watches: a file that changed is read again.
	Stat(path string) (fs.FileInfo, error)
	// Media gives the pictures, rooted at their own folder.
	Media() (fs.FS, error)
	// Exists says whether the content is there at all. A folder that is not
	// there is a mistake worth naming, not an empty site.
	Exists() bool
}
