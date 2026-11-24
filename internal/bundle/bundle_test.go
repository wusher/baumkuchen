package bundle

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
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

// A program with the content inside it gives the content back, and the
// program itself still runs.
func TestWriteAndOpen(t *testing.T) {
	// arrange
	dir := t.TempDir()
	program := write(t, dir, "baumkuchen", "the program itself\n")
	content := t.TempDir()
	write(t, content, "one.md", "# One\n")
	write(t, content, filepath.Join("images", "pic.jpg"), "not a picture")
	out := filepath.Join(dir, "baumkuchen-bundle")

	// act
	n, err := Write(program, out, content)
	if err != nil {
		t.Fatal(err)
	}
	files, ok, err := Open(out)

	// assert
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("the bundle says it holds nothing")
	}
	if n != 2 {
		t.Errorf("%d file(s) went in, want 2", n)
	}
	data, err := fs.ReadFile(files, Root+"/one.md")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "# One\n" {
		t.Errorf("the file came back as %q", data)
	}
	picture, err := fs.ReadFile(files, Root+"/images/pic.jpg")
	if err != nil {
		t.Fatal(err)
	}
	if string(picture) != "not a picture" {
		t.Errorf("the picture came back as %q", picture)
	}
	// the program is still at the front of the file, so it still runs
	whole, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(whole), "the program itself\n") {
		t.Error("the program is not at the front of the bundle")
	}
}

// The same program and the same content give the same file twice.
func TestWriteIsTheSameEveryTime(t *testing.T) {
	// arrange
	dir := t.TempDir()
	program := write(t, dir, "baumkuchen", "the program itself\n")
	content := t.TempDir()
	write(t, content, "one.md", "# One\n")

	// act
	first := filepath.Join(dir, "first")
	second := filepath.Join(dir, "second")
	if _, err := Write(program, first, content); err != nil {
		t.Fatal(err)
	}
	if _, err := Write(program, second, content); err != nil {
		t.Fatal(err)
	}

	// assert
	a, err := os.ReadFile(first)
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(second)
	if err != nil {
		t.Fatal(err)
	}
	if string(a) != string(b) {
		t.Error("two builds of the same thing are not the same file")
	}
}

// A bundle written over the program itself is the program with the writing
// in it, and not the program with the writing in it twice: the old content is
// taken away before the new one goes in.
func TestWriteOverTheProgramItself(t *testing.T) {
	// arrange
	dir := t.TempDir()
	program := write(t, dir, "baumkuchen", "the program itself\n")
	content := t.TempDir()
	write(t, content, "one.md", "# One\n")

	// act
	if _, err := Write(program, program, content); err != nil {
		t.Fatal(err)
	}
	first, err := os.Stat(program)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Write(program, program, content); err != nil {
		t.Fatal(err)
	}
	second, err := os.Stat(program)
	if err != nil {
		t.Fatal(err)
	}

	// assert
	if second.Size() != first.Size() {
		t.Errorf("the bundle written over itself is %d bytes, want %d: the old content is still in it",
			second.Size(), first.Size())
	}
	files, ok, err := Open(program)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("the program says it holds nothing")
	}
	data, err := fs.ReadFile(files, Root+"/one.md")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "# One\n" {
		t.Errorf("the file came back as %q", data)
	}
}

// A bundle of a bundle carries one set of content, not two: the old one is
// taken away before the new one goes in.
func TestWriteStripsTheOldBundle(t *testing.T) {
	// arrange
	dir := t.TempDir()
	program := write(t, dir, "baumkuchen", "the program itself\n")
	content := t.TempDir()
	write(t, content, "one.md", "# One\n")
	first := filepath.Join(dir, "first")
	if _, err := Write(program, first, content); err != nil {
		t.Fatal(err)
	}

	// act
	second := filepath.Join(dir, "second")
	if _, err := Write(first, second, content); err != nil {
		t.Fatal(err)
	}

	// assert
	firstInfo, err := os.Stat(first)
	if err != nil {
		t.Fatal(err)
	}
	secondInfo, err := os.Stat(second)
	if err != nil {
		t.Fatal(err)
	}
	if secondInfo.Size() != firstInfo.Size() {
		t.Errorf("the bundle of a bundle is %d bytes, want %d: the old bundle is still in it",
			secondInfo.Size(), firstInfo.Size())
	}
	files, ok, err := Open(second)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("the bundle says it holds nothing")
	}
	data, err := fs.ReadFile(files, Root+"/one.md")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "# One\n" {
		t.Errorf("the file came back as %q", data)
	}
}

// The settings file is carried inside the bundle, and the one that was in the
// content folder is not carried twice.
func TestWriteWithTheSettings(t *testing.T) {
	// arrange
	dir := t.TempDir()
	program := write(t, dir, "baumkuchen", "the program itself\n")
	content := t.TempDir()
	write(t, content, "one.md", "# One\n")
	write(t, content, "baumkuchen.yml", "title: The Old One\n")
	out := filepath.Join(dir, "with-settings")

	// act
	n, err := WriteWith(program, out, content, map[string][]byte{
		Root + "/baumkuchen.yml": []byte("title: The New One\n"),
	})
	if err != nil {
		t.Fatal(err)
	}
	files, ok, err := Open(out)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("the bundle says it holds nothing")
	}

	// assert
	if n != 2 {
		t.Errorf("%d file(s) went in, want 2", n)
	}
	data, err := fs.ReadFile(files, Root+"/baumkuchen.yml")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "title: The New One\n" {
		t.Errorf("the settings came back as %q", data)
	}
	names, err := fs.Glob(files, Root+"/baumkuchen.yml")
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 1 {
		t.Errorf("the settings are in the bundle %d times, want once", len(names))
	}
}

// A program with no content inside it says so, and that is not a fault.
func TestOpenOnAPlainProgram(t *testing.T) {
	// arrange
	dir := t.TempDir()
	program := write(t, dir, "baumkuchen", "the program itself\n")

	// act
	files, ok, err := Open(program)

	// assert
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("a plain program must not say it holds content")
	}
	if files != nil {
		t.Error("a plain program must not give any files")
	}
}

// A file that is not there is reported, not guessed at.
func TestOpenOnAMissingFile(t *testing.T) {
	// act
	_, _, err := Open(filepath.Join(t.TempDir(), "no-such-program"))

	// assert
	if err == nil {
		t.Error("opening a program that is not there must give an error")
	}
}

// A file whose mark does not match it is reported, not read as content.
func TestOpenOnAMarkedFileThatDoesNotMatch(t *testing.T) {
	// arrange
	dir := t.TempDir()
	program := write(t, dir, "baumkuchen", "the program itself\n")
	whole, err := os.ReadFile(program)
	if err != nil {
		t.Fatal(err)
	}
	broken := filepath.Join(dir, "broken")
	if err := os.WriteFile(broken, append(whole, trailer(10, len(whole)+trailerSize)...), 0o755); err != nil {
		t.Fatal(err)
	}

	// act
	_, _, err = Open(broken)

	// assert
	if err == nil {
		t.Error("a mark that does not match the file must give an error")
	}
}
