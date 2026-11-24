package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// buildWith runs the build command and gives back the code it left with.
func buildWith(t *testing.T, argv ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := run(argv, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

// writing makes a folder of writing with one post in it.
func writing(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	write(t, filepath.Join(dir, "one.md"),
		"---\ndate: 2026-01-02\n---\n\n# One\n\n# Sentence\n\nA line.\n")
	return dir
}

// The build refuses to write the site into the writing, and says which two
// folders it compared.
func TestTheBuildRefusesToWriteTheSiteIntoTheWriting(t *testing.T) {
	// arrange
	posts := writing(t)
	out := filepath.Join(posts, "site")
	t.Chdir(t.TempDir())

	// act
	code, _, stderr := buildWith(t, "build", posts, "-o", out)

	// assert
	if code != 2 {
		t.Fatalf("the code is %d, want 2 (the error stream said: %s)", code, stderr)
	}
	if !strings.Contains(stderr, out) || !strings.Contains(stderr, posts) {
		t.Errorf("the message does not name both folders: %q", stderr)
	}
}

// The build writes nothing when it refuses, and a clean alongside it removes
// nothing either: the writing is still whole.
func TestTheBuildRefusesAndWritesNothing(t *testing.T) {
	// arrange
	posts := writing(t)
	t.Chdir(t.TempDir())

	// act
	code, _, stderr := buildWith(t, "build", posts, "-o", posts)
	cleanCode, _, cleanStderr := buildWith(t, "build", posts, "-o", posts, "--clean")

	// assert
	if code != 2 {
		t.Fatalf("the code is %d, want 2 (the error stream said: %s)", code, stderr)
	}
	if cleanCode != 2 {
		t.Fatalf("the code with --clean is %d, want 2 (the error stream said: %s)",
			cleanCode, cleanStderr)
	}
	// the writing is still there, and nothing was written into it
	if _, err := os.Stat(filepath.Join(posts, "one.md")); err != nil {
		t.Errorf("the writing is gone: %v", err)
	}
	entries, err := os.ReadDir(posts)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() != "one.md" {
			t.Errorf("the build wrote %q into the writing", e.Name())
		}
	}
}

// The site may be written into the writing folder only under a name that
// says it is a build folder: "dist", or a name that begins with a dot.
func TestTheSiteMayGoIntoTheWritingOnlyUnderABuildName(t *testing.T) {
	// arrange
	cases := []struct {
		name string
		out  string
		want int
	}{
		{name: "a plain name", out: "site", want: 2},
		{name: "the usual name", out: "dist", want: 0},
		{name: "a hidden name", out: ".build", want: 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// arrange
			posts := writing(t)
			out := filepath.Join(posts, c.out)
			t.Chdir(t.TempDir())

			// act
			code, _, stderr := buildWith(t, "build", posts, "-o", out)

			// assert
			if code != c.want {
				t.Fatalf("the code is %d, want %d (the error stream said: %s)",
					code, c.want, stderr)
			}
			if c.want == 0 {
				if _, err := os.Stat(filepath.Join(out, "index.html")); err != nil {
					t.Errorf("the site was not written: %v", err)
				}
			}
		})
	}
}

// The build refuses a folder that holds the writing: a clean of that folder
// would reach the writing.
func TestTheBuildRefusesAFolderThatHoldsTheWriting(t *testing.T) {
	// arrange
	posts := writing(t)
	out := filepath.Dir(posts)
	t.Chdir(t.TempDir())

	// act
	code, _, stderr := buildWith(t, "build", posts, "-o", out)

	// assert
	if code != 2 {
		t.Fatalf("the code is %d, want 2 (the error stream said: %s)", code, stderr)
	}
	if !strings.Contains(stderr, out) || !strings.Contains(stderr, posts) {
		t.Errorf("the message does not name both folders: %q", stderr)
	}
}

// Two folders side by side are the ordinary case, and the build goes ahead.
func TestTwoFoldersSideBySideAreAllowed(t *testing.T) {
	// arrange
	posts := writing(t)
	out := filepath.Join(t.TempDir(), "site")
	t.Chdir(t.TempDir())

	// act
	code, stdout, stderr := buildWith(t, "build", posts, "-o", out)

	// assert
	if code != 0 {
		t.Fatalf("the code is %d, want 0 (the error stream said: %s)", code, stderr)
	}
	if _, err := os.Stat(filepath.Join(out, "index.html")); err != nil {
		t.Errorf("the site was not written: %v", err)
	}
	if !strings.Contains(stdout, "file(s) written to") {
		t.Errorf("the summary is not on the standard output: %q", stdout)
	}
}

// A link is followed before the folders are compared, so a folder that is
// only a link is compared with the folder it points at.
func TestALinkIsFollowedBeforeTheFoldersAreCompared(t *testing.T) {
	// arrange
	real := writing(t)
	link := filepath.Join(t.TempDir(), "linked")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("links are not made here: %v", err)
	}
	t.Chdir(t.TempDir())

	// act
	code, _, stderr := buildWith(t, "build", link, "-o", link)

	// assert
	if code != 2 {
		t.Fatalf("the code is %d, want 2 (the error stream said: %s)", code, stderr)
	}
	// the message names the folder the link points at, not the link
	if !strings.Contains(stderr, real) {
		t.Errorf("the message does not name the folder the link points at: %q", stderr)
	}
}

// A link inside the writing is followed too, so a link that pretends to be
// somewhere else does not smuggle the site into the writing.
func TestALinkInsideTheWritingIsFollowed(t *testing.T) {
	// arrange
	posts := writing(t)
	inner := filepath.Join(posts, "inner")
	if err := os.MkdirAll(inner, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(posts, "elsewhere")
	if err := os.Symlink(inner, link); err != nil {
		t.Skipf("links are not made here: %v", err)
	}
	t.Chdir(t.TempDir())

	// act
	code, _, stderr := buildWith(t, "build", posts, "-o", filepath.Join(link, "site"))

	// assert
	if code != 2 {
		t.Fatalf("the code is %d, want 2 (the error stream said: %s)", code, stderr)
	}
	// the message names the folder the link points at, not the link
	if !strings.Contains(stderr, inner) {
		t.Errorf("the message does not name the folder the link points at: %q", stderr)
	}
}

// A link that points out of the writing is followed too, so a site written
// through it lands outside the writing and is allowed.
func TestALinkOutOfTheWritingIsFollowed(t *testing.T) {
	// arrange
	posts := writing(t)
	other := t.TempDir()
	link := filepath.Join(posts, "elsewhere")
	if err := os.Symlink(other, link); err != nil {
		t.Skipf("links are not made here: %v", err)
	}
	t.Chdir(t.TempDir())

	// act
	code, _, stderr := buildWith(t, "build", posts, "-o", filepath.Join(link, "site"))

	// assert
	if code != 0 {
		t.Fatalf("the code is %d, want 0 (the error stream said: %s)", code, stderr)
	}
	if _, err := os.Stat(filepath.Join(other, "site", "index.html")); err != nil {
		t.Errorf("the site was not written where the link points: %v", err)
	}
}

// A folder that is not there yet is a folder the build makes, and the
// comparison is made against the folder that holds it.
func TestAFolderThatIsNotThereYetIsAllowed(t *testing.T) {
	// arrange
	posts := writing(t)
	out := filepath.Join(t.TempDir(), "not", "made", "yet", "site")
	t.Chdir(t.TempDir())

	// act
	code, _, stderr := buildWith(t, "build", posts, "-o", out)

	// assert
	if code != 0 {
		t.Fatalf("the code is %d, want 0 (the error stream said: %s)", code, stderr)
	}
	if _, err := os.Stat(filepath.Join(out, "index.html")); err != nil {
		t.Errorf("the site was not written: %v", err)
	}
}

// The paths are compared as they are written and as they are on disk: a
// folder named two ways is the same folder.
func TestTheSameFolderNamedTwoWaysIsTheSameFolder(t *testing.T) {
	// arrange
	posts := writing(t)
	t.Chdir(t.TempDir())

	// act
	code, _, stderr := buildWith(t, "build", posts, "-o", posts+string(filepath.Separator))

	// assert
	if code != 2 {
		t.Fatalf("the code is %d, want 2 (the error stream said: %s)", code, stderr)
	}
}
