package main

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/wusher/baumkuchen/internal/blog"
	"github.com/wusher/baumkuchen/internal/bundle"
	"github.com/wusher/baumkuchen/internal/content"
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
	w := &work{cmd: &commandLine{name: "serve", values: map[string]string{}}, bundled: files}
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
		cmd:     &commandLine{name: "serve", values: map[string]string{}},
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
	w := &work{cmd: &commandLine{name: "serve", values: map[string]string{}}, bundled: files}

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
		cmd:     &commandLine{name: "serve", values: map[string]string{"config": named}},
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

// The picture on the tab travels inside the program: it is put inside, and
// the settings that go inside name it there. A program that carries it shows
// it at its address, in a folder that holds nothing at all.
func TestTheBundleCarriesThePictureOnTheTab(t *testing.T) {
	// arrange
	const icon = `<svg xmlns="http://www.w3.org/2000/svg"><circle r="7"/></svg>`
	site := t.TempDir()
	write(t, filepath.Join(site, settingsName), "title: The Inside One\nfavicon: icon.svg\n")
	write(t, filepath.Join(site, "icon.svg"), icon)
	write(t, filepath.Join(site, "mine.md"),
		"---\ndate: 2026-01-02\n---\n\n# Mine\n\n# Sentence\n\nA line.\n")
	out := filepath.Join(t.TempDir(), "bundle")
	t.Chdir(site)

	// act
	var stdout, stderr bytes.Buffer
	code := run([]string{"bundle", ".", "-o", out}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("the code is %d, want 0 (the error stream said: %s)", code, stderr.String())
	}

	// assert
	// the picture is inside, and the settings inside name it there
	files, ok, err := bundle.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("the bundle carries nothing")
	}
	picture, err := fs.ReadFile(files, bundle.Root+"/favicon.svg")
	if err != nil {
		t.Fatalf("the picture is not inside the bundle: %v", err)
	}
	if string(picture) != icon {
		t.Errorf("the picture inside is %q, want the one of the writer", picture)
	}
	raw, err := fs.ReadFile(files, bundle.Root+"/"+settingsName)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "favicon: favicon.svg") {
		t.Errorf("the settings inside do not name the picture inside: %q", raw)
	}

	// and the site made from the bundle shows the picture at its address
	made, err := blog.New(blog.Config{
		Content:       content.NewFS(files, bundle.Root),
		Templates:     fs.FS(templateFiles),
		Static:        fs.FS(staticFiles),
		Favicon:       "favicon.svg",
		FaviconInside: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	ln, err := made.Listen("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go made.ServeOn(ln)
	got := fetch(t, "http://"+ln.Addr().String()+"/favicon.svg")
	if got != icon {
		t.Errorf("the picture at the address is %q, want the one of the writer", got)
	}
}

// A picture that is not there is a mistake, and the bundle says which one.
func TestTheBundleRefusesAMissingPicture(t *testing.T) {
	// arrange
	site := t.TempDir()
	write(t, filepath.Join(site, settingsName), "title: The Inside One\nfavicon: missing.svg\n")
	write(t, filepath.Join(site, "mine.md"),
		"---\ndate: 2026-01-02\n---\n\n# Mine\n\n# Sentence\n\nA line.\n")
	out := filepath.Join(t.TempDir(), "bundle")
	t.Chdir(site)

	// act
	var stdout, stderr bytes.Buffer
	code := run([]string{"bundle", ".", "-o", out}, &stdout, &stderr)

	// assert
	if code != 1 {
		t.Fatalf("the code is %d, want 1 (the error stream said: %s)", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "missing.svg") {
		t.Errorf("the message does not name the picture: %q", stderr.String())
	}
	if _, err := os.Stat(out); err == nil {
		t.Error("the bundle was written even though the picture was not there")
	}
}
