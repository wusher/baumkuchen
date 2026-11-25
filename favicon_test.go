package main

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wusher/baumkuchen/internal/bundle"
)

// A picture named in the settings file is read from the folder the settings
// file is in: the program is run from another folder, and the picture is
// found all the same, because the settings file and the picture travel
// together.
func TestThePictureOnTheTabComesFromTheFolderOfTheSettings(t *testing.T) {
	// arrange
	const png = "\x89PNG\r\n\x1a\nthe picture of the settings file"
	site := t.TempDir()
	write(t, filepath.Join(site, settingsName),
		"title: The Site\nfavicon: ./icon.png\nposts: posts\n")
	write(t, filepath.Join(site, "icon.png"), png)
	write(t, filepath.Join(site, "posts", "mine.md"),
		"---\ndate: 2026-01-02\n---\n\n# Mine\n\n# Sentence\n\nA line.\n")
	away := t.TempDir()
	t.Chdir(away)

	// act: the check, from a folder that holds no picture at all
	var stdout, stderr bytes.Buffer
	code := run([]string{"check", "--config", filepath.Join(site, settingsName)}, &stdout, &stderr)

	// assert
	if code != 0 {
		t.Fatalf("the code is %d, want 0 (the error stream said: %s)", code, stderr.String())
	}

	// act: and the built site, from the same folder
	code = run([]string{"build", "--config", filepath.Join(site, settingsName),
		"--out", filepath.Join(away, "site")}, &stdout, &stderr)

	// assert
	if code != 0 {
		t.Fatalf("the code is %d, want 0 (the error stream said: %s)", code, stderr.String())
	}
	got, err := os.ReadFile(filepath.Join(away, "site", "favicon.png"))
	if err != nil {
		t.Fatalf("the built site holds no picture on the tab: %v", err)
	}
	if string(got) != png {
		t.Errorf("the picture on the tab is %q, want the one beside the settings file", got)
	}
}

// A picture named on the command line is read from the working folder, which
// is where the command line was written: the settings file names a picture
// of its own, and the flag wins.
func TestThePictureOnTheTabFromTheFlagComesFromTheWorkingFolder(t *testing.T) {
	// arrange
	const png = "\x89PNG\r\n\x1a\nthe picture of the working folder"
	site := t.TempDir()
	write(t, filepath.Join(site, settingsName),
		"title: The Site\nfavicon: ./icon.png\nposts: posts\n")
	write(t, filepath.Join(site, "icon.png"), "\x89PNG\r\n\x1a\nthe picture of the settings file")
	write(t, filepath.Join(site, "posts", "mine.md"),
		"---\ndate: 2026-01-02\n---\n\n# Mine\n\n# Sentence\n\nA line.\n")
	away := t.TempDir()
	write(t, filepath.Join(away, "other.png"), png)
	t.Chdir(away)

	// act
	var stdout, stderr bytes.Buffer
	code := run([]string{"build", "--config", filepath.Join(site, settingsName),
		"--favicon", "./other.png", "--out", filepath.Join(away, "site")}, &stdout, &stderr)

	// assert
	if code != 0 {
		t.Fatalf("the code is %d, want 0 (the error stream said: %s)", code, stderr.String())
	}
	got, err := os.ReadFile(filepath.Join(away, "site", "favicon.png"))
	if err != nil {
		t.Fatalf("the built site holds no picture on the tab: %v", err)
	}
	if string(got) != png {
		t.Errorf("the picture on the tab is %q, want the one of the working folder", got)
	}
}

// A bundle carries the picture on the tab, and reads it from inside: the
// program is run in a folder that holds no picture at all.
//
// The bundle is made in another folder, which holds a picture of its own:
// the picture of the settings file is the one that travels.
func TestTheBundleCarriesAndReadsThePictureOnTheTab(t *testing.T) {
	// arrange
	const png = "\x89PNG\r\n\x1a\nthe picture of the settings file"
	site := t.TempDir()
	write(t, filepath.Join(site, settingsName),
		"title: The Site\nfavicon: ./icon.png\nposts: posts\n")
	write(t, filepath.Join(site, "icon.png"), png)
	write(t, filepath.Join(site, "posts", "mine.md"),
		"---\ndate: 2026-01-02\n---\n\n# Mine\n\n# Sentence\n\nA line.\n")
	out := filepath.Join(t.TempDir(), "bundle")
	away := t.TempDir()
	write(t, filepath.Join(away, "icon.png"), "\x89PNG\r\n\x1a\nthe picture of the working folder")
	t.Chdir(away)

	// act
	var stdout, stderr bytes.Buffer
	code := run([]string{"bundle", site, "--config", filepath.Join(site, settingsName),
		"-o", out}, &stdout, &stderr)

	// assert
	if code != 0 {
		t.Fatalf("the code is %d, want 0 (the error stream said: %s)", code, stderr.String())
	}

	// the picture is inside, and it is the one of the settings file
	files, ok, err := bundle.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("the bundle carries nothing")
	}
	picture, err := fs.ReadFile(files, bundle.Root+"/favicon.png")
	if err != nil {
		t.Fatalf("the picture is not inside the bundle: %v", err)
	}
	if string(picture) != png {
		t.Errorf("the picture inside is %q, want the one beside the settings file", picture)
	}
	// the settings inside name the picture that is there
	raw, err := fs.ReadFile(files, bundle.Root+"/"+settingsName)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "favicon: favicon.png") {
		t.Errorf("the settings inside do not name the picture inside: %q", raw)
	}

	// and the program made from the bundle shows it, in a folder with no
	// picture in it
	other := t.TempDir()
	t.Chdir(other)
	var built, problems bytes.Buffer
	w := &work{
		out:     &built,
		err:     &problems,
		cmd:     &commandLine{name: "build", values: map[string]string{"out": filepath.Join(other, "site")}},
		bundled: files,
	}
	w.setupLog()
	if err := w.build(); err != nil {
		t.Fatalf("the bundle did not build the site: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(other, "site", "favicon.png"))
	if err != nil {
		t.Fatalf("the built site holds no picture on the tab: %v", err)
	}
	if string(got) != png {
		t.Errorf("the picture on the tab is %q, want the one inside the bundle", got)
	}
}
