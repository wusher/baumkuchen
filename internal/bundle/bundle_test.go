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
