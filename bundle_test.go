package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/wusher/baumkuchen/internal/bundle"
)

// carried gives the files a program carries inside itself: the writing and
// the settings it was made with.
func carried(settings string, posts map[string]string) fstest.MapFS {
	files := fstest.MapFS{}
	for name, body := range posts {
		files[bundle.Root+"/"+name] = &fstest.MapFile{Data: []byte(body)}
	}
	files[bundle.Root+"/"+settingsName] = &fstest.MapFile{Data: []byte(settings)}
	return files
}

// A program that carries its writing reads the writing it carries: the posts
// line in the settings inside it names a folder, and that folder is the one
// the program runs in, not the one it was made in.
func TestAProgramReadsTheWritingItCarries(t *testing.T) {
	// arrange
	working := t.TempDir()
	write(t, filepath.Join(working, "posts", "other.md"),
		"---\ndate: 2026-01-02\n---\n\n# Other\n\n# Sentence\n\nA line.\n")
	files := carried("title: The Inside One\nposts: posts\n", map[string]string{
		"mine.md": "---\ndate: 2026-01-02\n---\n\n# Mine\n\n# Sentence\n\nA line.\n",
	})
	t.Chdir(working)

	// act
	set, err := findSettings("", "", files)
	if err != nil {
		t.Fatal(err)
	}
	w := &work{cmd: &commandLine{name: "serve", given: map[string]bool{}}, bundled: files}
	src, err := w.source(set)
	if err != nil {
		t.Fatal(err)
	}

	// assert
	if src.Label() != "inside the program" {
		t.Errorf("the writing came from %q, want the one inside the program", src.Label())
	}
	posts, err := src.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(posts) != 1 || posts[0] != "mine.md" {
		t.Errorf("the writing is %v, want the one inside the program", posts)
	}
}

// The log says where the writing came from, and a program that carries its
// writing says so in words that name the program.
func TestTheLogSaysTheWritingIsInsideTheProgram(t *testing.T) {
	// arrange
	files := carried("title: The Inside One\nposts: posts\n", map[string]string{
		"mine.md": "---\ndate: 2026-01-02\n---\n\n# Mine\n\n# Sentence\n\nA line.\n",
	})
	t.Chdir(t.TempDir())
	var logged bytes.Buffer
	w := &work{
		cmd:     &commandLine{name: "serve", given: map[string]bool{}},
		bundled: files,
		err:     &logged,
	}
	w.setupLog()

	// act
	if _, err := w.site(false); err != nil {
		t.Fatal(err)
	}

	// assert
	if !strings.Contains(logged.String(), "the posts come from inside the program") {
		t.Errorf("the log does not say where the writing came from: %q", logged.String())
	}
}

// A program that carries its settings is the same site in every folder: the
// settings inside it come before the file in the folder it runs in.
func TestTheSettingsInsideTheProgramComeBeforeTheWorkingFolder(t *testing.T) {
	// arrange
	working := t.TempDir()
	write(t, filepath.Join(working, settingsName), "title: The Working One\n")
	files := carried("title: The Inside One\nposts: posts\n", map[string]string{
		"mine.md": "---\ndate: 2026-01-02\n---\n\n# Mine\n\n# Sentence\n\nA line.\n",
	})
	t.Chdir(working)
	w := &work{cmd: &commandLine{name: "serve", given: map[string]bool{}}, bundled: files}

	// act
	set, err := w.settings()

	// assert
	if err != nil {
		t.Fatal(err)
	}
	if set.file.Title != "The Inside One" {
		t.Errorf("the title is %q, want the one inside the program", set.file.Title)
	}
	if set.from != "the settings inside the program" {
		t.Errorf("the settings came from %q, want the ones inside the program", set.from)
	}
}

// The settings that are named on the command line come first of all, so that
// a writer can replace the settings of a bundle on purpose.
func TestTheNamedSettingsComeBeforeTheOnesInsideTheProgram(t *testing.T) {
	// arrange
	named := filepath.Join(t.TempDir(), "mine.yml")
	write(t, named, "title: The Named One\n")
	files := carried("title: The Inside One\nposts: posts\n", map[string]string{
		"mine.md": "---\ndate: 2026-01-02\n---\n\n# Mine\n\n# Sentence\n\nA line.\n",
	})
	t.Chdir(t.TempDir())
	w := &work{
		cmd: &commandLine{name: "serve", globals: globals{config: named},
			given: map[string]bool{}},
		bundled: files,
	}

	// act
	set, err := w.settings()

	// assert
	if err != nil {
		t.Fatal(err)
	}
	if set.file.Title != "The Named One" {
		t.Errorf("the title is %q, want the one that was named", set.file.Title)
	}
}
