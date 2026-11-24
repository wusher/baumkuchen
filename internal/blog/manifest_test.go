package blog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The build leaves a manifest behind: the list of the files it wrote, one on
// each line, relative to the folder of the build.
func TestTheBuildLeavesAManifest(t *testing.T) {
	// arrange
	site, _ := siteWithPosts(t)
	out := t.TempDir()

	// act
	if _, err := site.Export(out); err != nil {
		t.Fatal(err)
	}

	// assert
	raw, err := os.ReadFile(filepath.Join(out, ".baumkuchen-build"))
	if err != nil {
		t.Fatalf("the manifest is not there: %v", err)
	}
	lines := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
	if len(lines) < 6 {
		t.Fatalf("the manifest holds %d line(s), want at least 6: %s", len(lines), raw)
	}
	for _, want := range []string{
		"index.html",
		"404.html",
		"post/open/index.html",
		"static/style.css",
		"static/app.js",
		"media/pic.jpg",
	} {
		found := false
		for _, line := range lines {
			if line == want {
				found = true
			}
		}
		if !found {
			t.Errorf("the manifest does not name %q:\n%s", want, raw)
		}
	}
	// and every line names a file that is there
	for _, line := range lines {
		if _, err := os.Stat(filepath.Join(out, filepath.FromSlash(line))); err != nil {
			t.Errorf("the manifest names %q, which is not there: %v", line, err)
		}
	}
}

// A build with no picture still leaves a manifest: it is still a build, and
// its clean must know it is one.
func TestTheManifestIsWrittenWhenThereAreNoPictures(t *testing.T) {
	// arrange
	dir := t.TempDir()
	postFile(t, dir, "one.md", "---\ndate: 2026-01-01\n---\n", "One")
	site := testSite(t, dir)
	out := t.TempDir()

	// act
	if _, err := site.Export(out); err != nil {
		t.Fatal(err)
	}

	// assert
	raw, err := os.ReadFile(filepath.Join(out, ".baumkuchen-build"))
	if err != nil {
		t.Fatalf("the manifest is not there: %v", err)
	}
	if !strings.Contains(string(raw), "index.html") {
		t.Errorf("the manifest does not name the index:\n%s", raw)
	}
}

// The clean removes the files of the build and leaves everything else alone.
func TestTheCleanRemovesOnlyWhatTheManifestNames(t *testing.T) {
	// arrange
	site, _ := siteWithPosts(t)
	out := t.TempDir()
	if _, err := site.Export(out); err != nil {
		t.Fatal(err)
	}
	// a file of the writer's own, in the folder of the build and inside it
	write(t, out, "CNAME", "example.com\n")
	write(t, filepath.Join(out, "post"), "notes.md", "a note of the writer's")

	// act
	if err := CleanBuild(out); err != nil {
		t.Fatal(err)
	}

	// assert
	for _, gone := range []string{"index.html", "404.html", "post/open/index.html",
		"static/style.css", "media/pic.jpg"} {
		if _, err := os.Stat(filepath.Join(out, filepath.FromSlash(gone))); err == nil {
			t.Errorf("%s is still there: the clean did not remove it", gone)
		}
	}
	// the files of the writer's own are still there
	if _, err := os.Stat(filepath.Join(out, "CNAME")); err != nil {
		t.Errorf("the clean took a file that was not the build's: %v", err)
	}
	if _, err := os.Stat(filepath.Join(out, "post", "notes.md")); err != nil {
		t.Errorf("the clean took a file that was not the build's: %v", err)
	}
}

// put writes a file in a folder, making the folders on the way.
func put(t *testing.T, dir, name, body string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// The clean can be run twice over the same folder: the second run finds
// nothing of the build to remove and leaves the writer's files alone.
func TestTheCleanCanBeRunTwice(t *testing.T) {
	// arrange
	site, _ := siteWithPosts(t)
	out := t.TempDir()
	if _, err := site.Export(out); err != nil {
		t.Fatal(err)
	}
	put(t, out, "CNAME", "example.com\n")
	put(t, filepath.Join(out, "post"), "notes.md", "a note of the writer's")

	// act
	if err := CleanBuild(out); err != nil {
		t.Fatal(err)
	}
	second := CleanBuild(out)

	// assert
	if second != nil {
		t.Errorf("the second clean gave an error: %v", second)
	}
	if _, err := os.Stat(filepath.Join(out, "CNAME")); err != nil {
		t.Errorf("the clean took a file that was not the build's: %v", err)
	}
	if _, err := os.Stat(filepath.Join(out, "post", "notes.md")); err != nil {
		t.Errorf("the clean took a file that was not the build's: %v", err)
	}
}

// The clean leaves the manifest behind, and empty: the folder is still a
// build, so a second clean knows what it is and removes nothing.
func TestTheCleanLeavesAnEmptyManifestBehind(t *testing.T) {
	// arrange
	site, _ := siteWithPosts(t)
	out := t.TempDir()
	if _, err := site.Export(out); err != nil {
		t.Fatal(err)
	}

	// act
	if err := CleanBuild(out); err != nil {
		t.Fatal(err)
	}

	// assert
	raw, err := os.ReadFile(filepath.Join(out, ".baumkuchen-build"))
	if err != nil {
		t.Fatalf("the manifest is gone: %v", err)
	}
	if len(raw) != 0 {
		t.Errorf("the manifest still names files: %s", raw)
	}
}

// The favicon of a build is a file of the build, so the clean removes it:
// the old clean left it behind, and the browser kept showing the old one.
func TestTheCleanRemovesTheFaviconOfTheBuild(t *testing.T) {
	// arrange
	dir := t.TempDir()
	postFile(t, dir, "one.md", "---\ndate: 2026-01-01\n---\n", "One")
	put(t, dir, "favicon.svg", "<svg></svg>")
	site := testSite(t, dir)
	out := t.TempDir()
	if _, err := site.Export(out); err != nil {
		t.Fatal(err)
	}
	icon := filepath.Join(out, "favicon.svg")
	if _, err := os.Stat(icon); err != nil {
		t.Fatalf("the build did not write the favicon: %v", err)
	}

	// act
	if err := CleanBuild(out); err != nil {
		t.Fatal(err)
	}

	// assert
	if _, err := os.Stat(icon); err == nil {
		t.Error("the favicon of the build is still there: the clean left it behind")
	}
}

// A folder that holds files but no manifest is not a build, and is not
// emptied.
func TestTheCleanRefusesAFolderWithNoManifest(t *testing.T) {
	// arrange
	out := t.TempDir()
	write(t, out, "something.txt", "not a build")

	// act
	err := CleanBuild(out)

	// assert
	if err == nil {
		t.Fatal("the clean must refuse a folder that is not a build")
	}
	if _, statErr := os.Stat(filepath.Join(out, "something.txt")); statErr != nil {
		t.Error("the file was removed even though the clean refused")
	}
}

// A manifest that names a file outside the build is refused, and nothing is
// removed at all.
func TestTheCleanRefusesAManifestThatClimbsOut(t *testing.T) {
	// arrange
	out := t.TempDir()
	write(t, out, ".baumkuchen-build", "index.html\n../outside.txt\n")
	write(t, out, "index.html", "a page")
	outside := filepath.Join(filepath.Dir(out), "outside.txt")
	if err := os.WriteFile(outside, []byte("not the build's"), 0o644); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(outside)

	// act
	err := CleanBuild(out)

	// assert
	if err == nil {
		t.Fatal("the clean must refuse a manifest that climbs out")
	}
	if _, statErr := os.Stat(filepath.Join(out, "index.html")); statErr != nil {
		t.Error("a file was removed even though the clean refused")
	}
	if _, statErr := os.Stat(outside); statErr != nil {
		t.Error("a file outside the build was removed")
	}
}

// A manifest that names an absolute path is refused, and nothing is removed
// at all.
func TestTheCleanRefusesAManifestWithAnAbsolutePath(t *testing.T) {
	// arrange
	out := t.TempDir()
	write(t, out, ".baumkuchen-build", "index.html\n/etc/passwd\n")
	write(t, out, "index.html", "a page")

	// act
	err := CleanBuild(out)

	// assert
	if err == nil {
		t.Fatal("the clean must refuse a manifest with an absolute path")
	}
	if _, statErr := os.Stat(filepath.Join(out, "index.html")); statErr != nil {
		t.Error("a file was removed even though the clean refused")
	}
}

// A manifest that names a file reached through a link out of the build is
// refused: the link could point anywhere.
func TestTheCleanRefusesAManifestThatGoesThroughALink(t *testing.T) {
	// arrange
	out := t.TempDir()
	other := t.TempDir()
	link := filepath.Join(out, "away")
	if err := os.Symlink(other, link); err != nil {
		t.Skipf("links are not made here: %v", err)
	}
	write(t, out, ".baumkuchen-build", "index.html\naway/secret.txt\n")
	write(t, out, "index.html", "a page")
	write(t, other, "secret.txt", "not the build's")

	// act
	err := CleanBuild(out)

	// assert
	if err == nil {
		t.Fatal("the clean must refuse a manifest that goes through a link")
	}
	if _, statErr := os.Stat(filepath.Join(other, "secret.txt")); statErr != nil {
		t.Error("a file outside the build was removed")
	}
	if _, statErr := os.Stat(filepath.Join(out, "index.html")); statErr != nil {
		t.Error("a file was removed even though the clean refused")
	}
}

// A folder that still holds a file of the writer's is not emptied: only the
// files the build wrote go.
func TestTheCleanDoesNotEmptyAFolderThatHoldsMore(t *testing.T) {
	// arrange
	site, _ := siteWithPosts(t)
	out := t.TempDir()
	if _, err := site.Export(out); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(out, "post"), "notes.md", "a note of the writer's")

	// act
	if err := CleanBuild(out); err != nil {
		t.Fatal(err)
	}

	// assert
	// the folder of the posts is still there, holding the writer's file
	if _, err := os.Stat(filepath.Join(out, "post", "notes.md")); err != nil {
		t.Errorf("the clean took a file that was not the build's: %v", err)
	}
	// and the page of the build is gone
	if _, err := os.Stat(filepath.Join(out, "post", "open", "index.html")); err == nil {
		t.Error("the page of the build is still there")
	}
	// and the folder of the post itself is gone, it holds nothing
	if _, err := os.Stat(filepath.Join(out, "post", "open")); err == nil {
		t.Error("the empty folder of the post is still there")
	}
}

// The clean removes no more than the files of the build: a folder of files
// the writer made beside them is left whole.
func TestTheCleanLeavesAFolderOfTheWriterAlone(t *testing.T) {
	// arrange
	site, _ := siteWithPosts(t)
	out := t.TempDir()
	if _, err := site.Export(out); err != nil {
		t.Fatal(err)
	}
	put(t, filepath.Join(out, "post", "notes"), "deep.md", "the writer's own")

	// act
	if err := CleanBuild(out); err != nil {
		t.Fatal(err)
	}

	// assert
	if _, err := os.Stat(filepath.Join(out, "post", "notes", "deep.md")); err != nil {
		t.Errorf("the clean took a file that was not the build's: %v", err)
	}
}
