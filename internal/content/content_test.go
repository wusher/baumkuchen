package content

import (
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

// write puts one file in a folder and gives back its path.
func write(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// The content in a folder: the markdown is found, the pictures and the hidden
// folders are stepped over, and the paths are relative to the folder.
func TestDirListsTheMarkdown(t *testing.T) {
	// arrange
	dir := t.TempDir()
	write(t, dir, "one.md", "# One\n")
	write(t, dir, filepath.Join("posts", "two.md"), "# Two\n")
	write(t, dir, filepath.Join("images", "pic.jpg"), "not a picture")
	write(t, dir, filepath.Join(".hidden", "three.md"), "# Three\n")
	write(t, dir, "notes.txt", "not markdown")

	// act
	src := NewDir(dir)
	list, err := src.List()

	// assert
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(list, ",") != "one.md,posts/two.md" {
		t.Errorf("list = %v, want [one.md posts/two.md]", list)
	}
}

// A file is read by the path the list gave, and told apart from another one
// of the same name in another folder.
func TestDirReadsAndTellsFilesApart(t *testing.T) {
	// arrange
	dir := t.TempDir()
	write(t, dir, filepath.Join("a", "twice.md"), "the first one")
	write(t, dir, filepath.Join("b", "twice.md"), "the second one")
	src := NewDir(dir)

	// act
	first, err := src.Read("a/twice.md")
	if err != nil {
		t.Fatal(err)
	}
	second, err := src.Read("b/twice.md")
	if err != nil {
		t.Fatal(err)
	}

	// assert
	if string(first) != "the first one" || string(second) != "the second one" {
		t.Errorf("the files came back mixed up: %q and %q", first, second)
	}
	info, err := src.Stat("a/twice.md")
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() != int64(len("the first one")) {
		t.Errorf("the size is %d, want %d", info.Size(), len("the first one"))
	}
}

// A folder that is not there is said so, and not guessed at.
func TestDirOnAMissingFolder(t *testing.T) {
	// arrange
	src := NewDir(filepath.Join(t.TempDir(), "no-such-folder"))

	// act
	exists := src.Exists()
	_, err := src.List()

	// assert
	if exists {
		t.Error("a folder that is not there must not say it is")
	}
	if err == nil {
		t.Error("reading a folder that is not there must give an error")
	}
}

// The posts often live behind a link, and the link is followed once.
func TestDirFollowsALink(t *testing.T) {
	// arrange
	real := t.TempDir()
	write(t, real, "one.md", "# One\n")
	link := filepath.Join(t.TempDir(), "posts")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("this system has no links: %v", err)
	}

	// act
	src := NewDir(link)
	list, err := src.List()

	// assert
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0] != "one.md" {
		t.Errorf("list = %v, want [one.md]", list)
	}
	data, err := src.Read("one.md")
	if err != nil || !strings.Contains(string(data), "One") {
		t.Errorf("the file behind the link did not come back: %q %v", data, err)
	}
}

// The pictures are handed over in their own folder, so the site can serve
// them without knowing where the content is.
func TestDirGivesThePictures(t *testing.T) {
	// arrange
	dir := t.TempDir()
	write(t, dir, filepath.Join(MediaDir, "pic.jpg"), "not a picture")
	src := NewDir(dir)

	// act
	media, err := src.Media()

	// assert
	if err != nil {
		t.Fatal(err)
	}
	data, err := readFile(media, "pic.jpg")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "not a picture" {
		t.Errorf("the picture came back as %q", data)
	}
}

// The content in a set of files: the same rules, and the same paths.
func TestFSListsAndReads(t *testing.T) {
	// arrange
	files := fstest.MapFS{
		"content/one.md":           {Data: []byte("# One\n")},
		"content/posts/two.md":     {Data: []byte("# Two\n")},
		"content/images/pic.jpg":   {Data: []byte("not a picture")},
		"content/.hidden/three.md": {Data: []byte("# Three\n")},
		"content/notes.txt":        {Data: []byte("not markdown")},
	}
	src := NewFS(files, "content")

	// act
	list, err := src.List()

	// assert
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(list, ",") != "one.md,posts/two.md" {
		t.Errorf("list = %v, want [one.md posts/two.md]", list)
	}
	data, err := src.Read("posts/two.md")
	if err != nil || !strings.Contains(string(data), "Two") {
		t.Errorf("the file came back as %q %v", data, err)
	}
	media, err := src.Media()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := readFile(media, "pic.jpg"); err != nil {
		t.Errorf("the picture did not come back: %v", err)
	}
}

// A file set with the content at its top is read the same way.
func TestFSWithNoRoot(t *testing.T) {
	// arrange
	files := fstest.MapFS{"one.md": {Data: []byte("# One\n")}}
	src := NewFS(files, "")

	// act
	list, err := src.List()

	// assert
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(list, ",") != "one.md" {
		t.Errorf("list = %v, want [one.md]", list)
	}
}

// readFile gives the bytes of one file in a file set.
func readFile(files fs.FS, name string) ([]byte, error) {
	f, err := files.Open(name)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	return io.ReadAll(f)
}
