package content

import (
	"io/fs"
	"path"
	"sort"
	"strings"
)

// FS is the content in a set of files that is already in hand: a zip carried
// inside the program, or the files a test makes for itself.
//
// The root is the folder inside the file set where the content begins. It is
// empty when the set holds the content at its top.
type FS struct {
	files fs.FS
	root  string
}

// NewFS gives the content held in the file set, under the folder at root.
func NewFS(files fs.FS, root string) *FS {
	return &FS{files: files, root: strings.Trim(root, "/")}
}

// Label names the file set, so a log says where the content came from.
func (f *FS) Label() string {
	if f.root == "" {
		return "inside the program"
	}
	return "inside the program, under " + f.root
}

// Exists always says yes: a file set that is open holds its content.
func (f *FS) Exists() bool { return true }

// List gives every markdown file in the set, in order, with the pictures and
// the hidden folders stepped over, as on disk.
func (f *FS) List() ([]string, error) {
	var out []string
	root := f.root
	if root == "" {
		root = "."
	}
	err := fs.WalkDir(f.files, root, func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if name == root {
				return nil
			}
			if entry.Name() == MediaDir || strings.HasPrefix(entry.Name(), ".") {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.EqualFold(path.Ext(entry.Name()), ".md") {
			return nil
		}
		rel := name
		if f.root != "" {
			rel = strings.TrimPrefix(name, f.root+"/")
		}
		out = append(out, rel)
		return nil
	})
	sort.Strings(out)
	return out, err
}

// Read gives the bytes of one file.
func (f *FS) Read(name string) ([]byte, error) {
	return fs.ReadFile(f.files, f.join(name))
}

// Stat gives the size and the time of one file.
func (f *FS) Stat(name string) (fs.FileInfo, error) {
	return fs.Stat(f.files, f.join(name))
}

// Media gives the pictures, rooted at their own folder.
func (f *FS) Media() (fs.FS, error) {
	return fs.Sub(f.files, f.join(MediaDir))
}

// join puts the root in front of a path inside the content.
func (f *FS) join(name string) string {
	if f.root == "" {
		return name
	}
	return f.root + "/" + name
}
