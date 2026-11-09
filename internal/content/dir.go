package content

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Dir is the content in a folder on disk.
//
// The folder is often a link: the posts live outside the repository, and the
// link points at them. The link is followed every time the content is read,
// so every path inside is a real one and a link that is moved is followed
// where it now points.
type Dir struct {
	root string
}

// NewDir gives the content in the folder at root. The folder does not have to
// be there yet: the error comes when the content is read, and it names the
// folder.
func NewDir(root string) *Dir {
	if root == "" {
		root = "."
	}
	return &Dir{root: root}
}

// Label names the folder, so a log says where the content came from.
func (d *Dir) Label() string { return d.root }

// Exists says whether the folder is there. A link that points at nothing is
// not there, either.
func (d *Dir) Exists() bool {
	info, err := os.Stat(d.root)
	return err == nil && info.IsDir()
}

// resolved gives the folder with any link followed, so the walk starts at the
// real folder and the paths it gives are the real ones.
func (d *Dir) resolved() string {
	if resolved, err := filepath.EvalSymlinks(d.root); err == nil {
		return resolved
	}
	return d.root
}

// List gives every markdown file under the folder, in order.
//
// The pictures folder is stepped over, and so is anything hidden: a folder a
// person cannot see is not one they meant to publish from.
func (d *Dir) List() ([]string, error) {
	var out []string
	root := d.resolved()
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := entry.Name()
		if entry.IsDir() {
			if path == root {
				return nil
			}
			if name == MediaDir || strings.HasPrefix(name, ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.EqualFold(filepath.Ext(name), ".md") {
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			out = append(out, filepath.ToSlash(rel))
		}
		return nil
	})
	sort.Strings(out)
	return out, err
}

// Read gives the bytes of one file.
func (d *Dir) Read(path string) ([]byte, error) {
	return os.ReadFile(d.local(path))
}

// Stat gives the size and the time of one file.
func (d *Dir) Stat(path string) (fs.FileInfo, error) {
	return os.Stat(d.local(path))
}

// Media gives the pictures, rooted at their own folder. A site with no
// pictures is not a fault: the folder simply is not there.
func (d *Dir) Media() (fs.FS, error) {
	return os.DirFS(filepath.Join(d.resolved(), MediaDir)), nil
}

// local turns a path with forward slashes into one for this system.
func (d *Dir) local(path string) string {
	return filepath.Join(d.resolved(), filepath.FromSlash(path))
}
