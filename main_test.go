package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"baumkuchen/internal/blog"
)

// The settings file is read when it is there, and the settings inside the
// program stand when it is not.
func TestLoadSettings(t *testing.T) {
	// arrange
	dir := t.TempDir()
	path := filepath.Join(dir, "baumkuchen.yml")
	if err := os.WriteFile(path, []byte("title: A Test Site\naddr: :9111\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(dir, "no-such-file.yml")

	// act
	found, from, err := loadSettings(path, true)
	inside, insideFrom, insideErr := loadSettings(missing, false)
	_, _, namedErr := loadSettings(missing, true)

	// assert
	if err != nil {
		t.Fatal(err)
	}
	if found.Title != "A Test Site" || found.Addr != ":9111" {
		t.Errorf("the settings came back as %+v", found)
	}
	if from != path {
		t.Errorf("the settings came from %q, want %q", from, path)
	}
	if insideErr != nil {
		t.Fatal(insideErr)
	}
	// the settings inside the program are the settings file at the top of
	// the tree, so a copy of the program knows the site
	raw, err := os.ReadFile("baumkuchen.yml")
	if err != nil {
		t.Fatal(err)
	}
	want, err := blog.Load(raw)
	if err != nil {
		t.Fatal(err)
	}
	if inside != want {
		t.Errorf("the settings inside the program are %+v, want %+v", inside, want)
	}
	if !strings.Contains(insideFrom, "inside the program") {
		t.Errorf("the settings came from %q, want the ones inside the program", insideFrom)
	}
	if namedErr == nil {
		t.Error("a file that was named and is not there must give an error")
	}
}

// The posts come from the folder that was named, then from the environment,
// then from the settings.
func TestChooseSource(t *testing.T) {
	// arrange
	dir := t.TempDir()
	other := t.TempDir()
	t.Setenv("BAUMKUCHEN_POSTS", "")

	// act
	named, err := chooseSource(blog.File{Posts: dir}, true)
	fromFile, fileErr := chooseSource(blog.File{Posts: dir}, false)
	_, missingErr := chooseSource(blog.File{Posts: filepath.Join(dir, "no-such-folder")}, true)
	_, noneErr := chooseSource(blog.File{}, false)

	// assert
	if err != nil {
		t.Fatal(err)
	}
	if !named.Exists() || named.Label() != dir {
		t.Errorf("the posts came from %q, want %q", named.Label(), dir)
	}
	if fileErr != nil {
		t.Fatal(fileErr)
	}
	if fromFile.Label() != dir {
		t.Errorf("the posts came from %q, want %q", fromFile.Label(), dir)
	}
	if missingErr == nil {
		t.Error("a folder that is not there must give an error")
	}
	if noneErr == nil {
		t.Error("no posts at all must give an error")
	}

	// the environment is read when the folder was not named
	t.Setenv("BAUMKUCHEN_POSTS", other)
	fromEnv, envErr := chooseSource(blog.File{Posts: dir}, false)
	if envErr != nil {
		t.Fatal(envErr)
	}
	if fromEnv.Label() != other {
		t.Errorf("the posts came from %q, want %q", fromEnv.Label(), other)
	}
}

// The build writes into the folder it was given, or into the one in the
// settings when the flag was given with nothing after it.
func TestExportFolder(t *testing.T) {
	// arrange + assert
	if got := export("", "dist"); got != "" {
		t.Errorf("export(\"\", \"dist\") = %q, want nothing", got)
	}
	if got := export("-", "dist"); got != "dist" {
		t.Errorf("export(\"-\", \"dist\") = %q, want dist", got)
	}
	if got := export("out", "dist"); got != "out" {
		t.Errorf("export(\"out\", \"dist\") = %q, want out", got)
	}
}
