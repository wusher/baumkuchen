package main

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/wusher/baumkuchen/internal/blog"
	"github.com/wusher/baumkuchen/internal/bundle"
)

// A buffer that the server writes to while the test reads it.
type safeBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *safeBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *safeBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// A port of 0 is a port the system picks: the server says which one it got,
// and the address it says answers. The drafts are shown on the loopback
// address, and held back when the server answers anyone.
func TestTheServerSaysTheAddressItGot(t *testing.T) {
	cases := []struct {
		name      string
		addr      string
		wantDraft int
	}{
		{name: "the loopback address", addr: "127.0.0.1:0", wantDraft: http.StatusOK},
		{name: "an address anyone can reach", addr: "0.0.0.0:0", wantDraft: http.StatusNotFound},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// arrange
			dir := t.TempDir()
			posts := filepath.Join(dir, "posts")
			write(t, filepath.Join(posts, "one.md"),
				"---\ndate: 2026-01-02\n---\n\n# One\n\n# Sentence\n\nA line.\n")
			write(t, filepath.Join(posts, "_drafts", "hidden.md"),
				"---\ndate: 2026-01-01\n---\n\n# Hidden\n\n# Sentence\n\nA line.\n")
			write(t, filepath.Join(posts, "notes.md"), "# No Depth Here\n\nnothing.\n")
			out := &safeBuffer{}
			logged := &safeBuffer{}
			w := &work{
				out: out,
				err: logged,
				cmd: &commandLine{name: "serve", folder: posts,
					values: map[string]string{"addr": c.addr}},
			}
			w.setupLog()
			go w.serve()

			// act
			url := waitForTheAddress(t, out)

			// assert
			if got := statusCode(t, url+"/"); got != http.StatusOK {
				t.Errorf("the address the server gave answered %d, want 200", got)
			}
			if got := statusCode(t, url+"/post/hidden"); got != c.wantDraft {
				t.Errorf("the draft answered %d, want %d", got, c.wantDraft)
			}
			// a file that carries no depth is named when the server starts
			if !strings.Contains(logged.String(), "notes") {
				t.Errorf("the file with no depth was not named: %q", logged.String())
			}
		})
	}
}

// The server waits for the address to be said, and gives it back.
func waitForTheAddress(t *testing.T, out *safeBuffer) string {
	t.Helper()
	for i := 0; i < 100; i++ {
		_, rest, found := strings.Cut(out.String(), "the blog is on ")
		if found {
			url, _, _ := strings.Cut(rest, "\n")
			return strings.TrimSpace(url)
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("the server never said where it was: %q", out.String())
	return ""
}

// statusCode gives the code one address answers with.
func statusCode(t *testing.T, url string) int {
	t.Helper()
	res, err := http.Get(url)
	if err != nil {
		t.Fatalf("%s did not answer: %v", url, err)
	}
	defer res.Body.Close()
	_, _ = io.Copy(io.Discard, res.Body)
	return res.StatusCode
}

// A port that is taken is named, and the run stops with the first code.
func TestThePortThatIsTakenIsNamed(t *testing.T) {
	// arrange
	dir := t.TempDir()
	posts := filepath.Join(dir, "posts")
	write(t, filepath.Join(posts, "one.md"),
		"---\ndate: 2026-01-02\n---\n\n# One\n\n# Sentence\n\nA line.\n")
	taken, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer taken.Close()
	addr := taken.Addr().String()

	// act
	var stdout, stderr bytes.Buffer
	code := run([]string{"serve", posts, "--addr", addr}, &stdout, &stderr)

	// assert
	if code != 1 {
		t.Errorf("the code is %d, want 1 (the error stream said: %s)", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), addr) {
		t.Errorf("the message does not name the address that is taken: %q", stderr.String())
	}
	if !strings.Contains(stderr.String(), "is already taken") {
		t.Errorf("the message does not say that the address is taken: %q", stderr.String())
	}
}

// The wording of the fault is the one of the machine, and the address is put
// in front of it.
func TestListenErrorNamesTheAddress(t *testing.T) {
	// arrange
	taken := errors.New("listen tcp 127.0.0.1:8080: bind: address already in use")
	other := errors.New("listen tcp 127.0.0.1:8080: bind: permission denied")

	// act
	named := listenError("127.0.0.1:8080", taken)
	plain := listenError("127.0.0.1:8080", other)

	// assert
	if !strings.Contains(named.Error(), "127.0.0.1:8080 is already taken") {
		t.Errorf("the message is %q, want the address in front of it", named.Error())
	}
	if !strings.Contains(named.Error(), "address already in use") {
		t.Errorf("the message is %q, want the wording of the machine in it", named.Error())
	}
	if plain.Error() != other.Error() {
		t.Errorf("the message is %q, want the fault as it is: %q", plain.Error(), other.Error())
	}
}

// A bundle made from a bundle carries one set of writing, and not two.
func TestABundleFromABundle(t *testing.T) {
	// arrange
	site := t.TempDir()
	write(t, filepath.Join(site, settingsName), "title: The Inside One\nposts: posts\n")
	write(t, filepath.Join(site, "mine.md"),
		"---\ndate: 2026-01-02\n---\n\n# Mine\n\n# Sentence\n\nA line.\n")
	first := filepath.Join(t.TempDir(), "first")
	t.Chdir(site)
	var stdout, stderr bytes.Buffer
	if code := run([]string{"bundle", ".", "-o", first}, &stdout, &stderr); code != 0 {
		t.Fatalf("the code is %d, want 0 (the error stream said: %s)", code, stderr.String())
	}
	files, ok, err := bundle.Open(first)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("the first bundle carries nothing")
	}
	second := filepath.Join(t.TempDir(), "second")
	w := &work{
		out:     &stdout,
		err:     &stderr,
		cmd:     &commandLine{name: "bundle", values: map[string]string{"out": second}},
		bundled: files,
	}
	w.setupLog()

	// act
	err = w.bundle()

	// assert
	if err != nil {
		t.Fatalf("the bundle from a bundle did not finish: %v", err)
	}
	carried, ok, err := bundle.Open(second)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("the bundle from a bundle carries nothing")
	}
	names := carriedNames(t, carried)
	if names["mine.md"] != 1 {
		t.Errorf("the writing is in the bundle %d times, want 1: %v",
			names["mine.md"], names)
	}
	if names[settingsName] != 1 {
		t.Errorf("the settings are in the bundle %d times, want 1: %v",
			names[settingsName], names)
	}
	// the bundle that was made first is taken away before the second one
	// goes in, so the two are the same size
	firstInfo, err := os.Stat(first)
	if err != nil {
		t.Fatal(err)
	}
	secondInfo, err := os.Stat(second)
	if err != nil {
		t.Fatal(err)
	}
	if secondInfo.Size() != firstInfo.Size() {
		t.Errorf("the bundle from a bundle is %d bytes, want %d: the old bundle is still in it",
			secondInfo.Size(), firstInfo.Size())
	}
}

// carriedNames counts how many times each name is in a bundle.
func carriedNames(t *testing.T, files fs.FS) map[string]int {
	t.Helper()
	names := map[string]int{}
	err := fs.WalkDir(files, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		names[strings.TrimPrefix(path, bundle.Root+"/")]++
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return names
}

// A folder with no markdown in it is not a bundle, and nothing is written.
func TestTheBundleRefusesAFolderWithNoWriting(t *testing.T) {
	// arrange
	dir := t.TempDir()
	out := filepath.Join(t.TempDir(), "bundle")
	t.Chdir(dir)

	// act
	var stdout, stderr bytes.Buffer
	code := run([]string{"bundle", dir, "-o", out}, &stdout, &stderr)

	// assert
	if code != 1 {
		t.Errorf("the code is %d, want 1 (the error stream said: %s)", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "holds no markdown") {
		t.Errorf("the message does not say that there is nothing to put in it: %q", stderr.String())
	}
	if _, err := os.Stat(out); err == nil {
		t.Error("the bundle was written even though there was nothing to put in it")
	}
}

// The address comes from the flag, then from the settings, and last from the
// default in the code.
func TestTheAddressComesFromTheFlagTheSettingsAndTheCode(t *testing.T) {
	// arrange
	w := &work{cmd: &commandLine{name: "serve", values: map[string]string{}}}
	set := &settings{file: blog.File{Addr: "127.0.0.1:9999"}}

	// act
	fromCode := w.addr(&settings{file: blog.File{}})
	fromSettings := w.addr(set)
	w.cmd.values["addr"] = "127.0.0.1:1234"
	fromFlag := w.addr(set)

	// assert
	if fromCode != blog.Defaults().Addr {
		t.Errorf("the address from the code is %q, want %q", fromCode, blog.Defaults().Addr)
	}
	if fromSettings != "127.0.0.1:9999" {
		t.Errorf("the address from the settings is %q, want the one in them", fromSettings)
	}
	if fromFlag != "127.0.0.1:1234" {
		t.Errorf("the address from the flag is %q, want the one on the command line", fromFlag)
	}
}

// The commit name and the day are shortened the way the build writes them.
func TestTheCommitAndTheDayAreShortened(t *testing.T) {
	// arrange + act
	commit := shortSHA("d2163f5e372f5f539d977e48d4f5a9c6ded2d0bd")
	short := shortSHA("abc")
	day := shortTime("2026-11-23T21:25:13Z")
	shortDay := shortTime("2026")

	// assert
	if commit != "d2163f5" {
		t.Errorf("the commit is %q, want the first seven characters", commit)
	}
	if short != "abc" {
		t.Errorf("a short commit is %q, want it as it is", short)
	}
	if day != "2026-11-23T21:25:13" {
		t.Errorf("the day is %q, want it without the Z", day)
	}
	if shortDay != "2026" {
		t.Errorf("a short day is %q, want it as it is", shortDay)
	}
}
