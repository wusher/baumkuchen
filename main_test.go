package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/wusher/baumkuchen/internal/blog"
)

// write puts a file in a folder, and makes the folders on the way.
func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// The command line is read the way it is written: the command, the folder and
// the flags, in any order.
func TestReadArgs(t *testing.T) {
	cases := []struct {
		name    string
		argv    []string
		command string
		folder  string
		out     string
		wantErr string
	}{
		{name: "nothing at all", command: "serve"},
		{name: "the default command with a folder", argv: []string{"~/blog"},
			command: "serve", folder: "~/blog"},
		{name: "serve", argv: []string{"serve"}, command: "serve"},
		{name: "build", argv: []string{"build"}, command: "build"},
		{name: "bundle", argv: []string{"bundle"}, command: "bundle"},
		{name: "check", argv: []string{"check"}, command: "check"},
		{name: "init", argv: []string{"init"}, command: "init"},
		{name: "version", argv: []string{"version"}, command: "version"},
		{name: "a command with a folder", argv: []string{"build", "~/blog"},
			command: "build", folder: "~/blog"},
		{name: "a flag before the folder", argv: []string{"serve", "--addr", ":9000", "~/blog"},
			command: "serve", folder: "~/blog"},
		{name: "a flag after the folder", argv: []string{"serve", "~/blog", "--addr", ":9000"},
			command: "serve", folder: "~/blog"},
		{name: "a flag with its value joined", argv: []string{"build", "--out=out"},
			command: "build", out: "out"},
		{name: "a short flag", argv: []string{"build", "-o", "out"},
			command: "build", out: "out"},
		{name: "a global flag before the command", argv: []string{"--config", "my.yml", "check"},
			command: "check"},
		{name: "a global flag after the folder", argv: []string{"check", "~/blog", "--config", "my.yml"},
			command: "check", folder: "~/blog"},
		{name: "a flag of another command", argv: []string{"serve", "--json"}, wantErr: "unknown flag"},
		{name: "an unknown flag", argv: []string{"--nope"}, wantErr: "unknown flag"},
		{name: "a flag that needs a value", argv: []string{"build", "--out"}, wantErr: "needs a value"},
		{name: "too many folders", argv: []string{"serve", "a", "b"}, wantErr: "at most one folder"},
		{name: "a folder named like a command", argv: []string{"serve", "build"}, wantErr: "./build"},
		{name: "a folder named like a command, written out", argv: []string{"./build"},
			command: "serve", folder: "./build"},
		{name: "the removed export flag", argv: []string{"-export", "out"}, wantErr: "is gone"},
		{name: "the removed bundle flag", argv: []string{"-bundle", "out"}, wantErr: "is gone"},
		{name: "the removed posts flag", argv: []string{"-posts", "~/blog"}, wantErr: "is gone"},
		{name: "the removed version flag", argv: []string{"-version"}, wantErr: "is gone"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// act
			cmd, err := readArgs(c.argv)

			// assert
			if c.wantErr != "" {
				if err == nil {
					t.Fatalf("no error, want one about %q", c.wantErr)
				}
				if !strings.Contains(err.Error(), c.wantErr) {
					t.Fatalf("the error is %q, want it to mention %q", err, c.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if cmd.name != c.command {
				t.Errorf("the command is %q, want %q", cmd.name, c.command)
			}
			if cmd.folder != c.folder {
				t.Errorf("the folder is %q, want %q", cmd.folder, c.folder)
			}
			if c.out != "" {
				got, ok := cmd.value("out")
				if !ok {
					t.Fatal("the out flag was not given")
				}
				if got != c.out {
					t.Errorf("out = %q, want %q", got, c.out)
				}
			}
		})
	}
}

// The removed flags say what to write instead, and stop with the second code.
func TestRemovedFlagsSayWhatToWrite(t *testing.T) {
	cases := map[string]string{
		"export":  "baumkuchen build -o DIR",
		"bundle":  "baumkuchen bundle -o FILE",
		"posts":   "baumkuchen serve FOLDER",
		"version": "baumkuchen version",
	}
	for name, hint := range cases {
		t.Run(name, func(t *testing.T) {
			// act
			var stdout, stderr bytes.Buffer
			code := run([]string{"-" + name, "x"}, &stdout, &stderr)

			// assert
			if code != 2 {
				t.Errorf("the code is %d, want 2", code)
			}
			if !strings.Contains(stderr.String(), hint) {
				t.Errorf("the message does not say %q: %s", hint, stderr.String())
			}
		})
	}
}

// The program leaves with the code the situation calls for.
func TestRunExitCodes(t *testing.T) {
	cases := []struct {
		name string
		argv []string
		want int
	}{
		{name: "an unknown flag", argv: []string{"--nope"}, want: 2},
		{name: "a flag of another command", argv: []string{"serve", "--json"}, want: 2},
		{name: "the removed export flag", argv: []string{"-export", "out"}, want: 2},
		{name: "the removed posts flag", argv: []string{"-posts", "~/blog"}, want: 2},
		{name: "the removed bundle flag", argv: []string{"-bundle", "x"}, want: 2},
		{name: "the removed version flag", argv: []string{"-version"}, want: 2},
		{name: "a folder named like a command", argv: []string{"serve", "build"}, want: 2},
		{name: "a bundle with nowhere to write", argv: []string{"bundle"}, want: 2},
		{name: "the version", argv: []string{"version"}, want: 0},
		{name: "the help", argv: []string{"--help"}, want: 0},
		{name: "the help of one command", argv: []string{"build", "--help"}, want: 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// act
			var stdout, stderr bytes.Buffer
			got := run(c.argv, &stdout, &stderr)

			// assert
			if got != c.want {
				t.Errorf("the code is %d, want %d (the error stream said: %s)",
					got, c.want, stderr.String())
			}
		})
	}
}

// The settings come from the file that was named, then the one in the posts
// folder, then the one in the working folder, then the ones in the code.
func TestFindSettings(t *testing.T) {
	// arrange
	working := t.TempDir()
	folder := t.TempDir()
	named := filepath.Join(t.TempDir(), "named.yml")
	write(t, filepath.Join(working, settingsName), "title: The Working One\n")
	write(t, filepath.Join(folder, settingsName), "title: The Folder One\n")
	write(t, named, "title: The Named One\n")
	t.Chdir(working)

	// act + assert
	cases := []struct {
		name     string
		explicit string
		folder   string
		want     string
	}{
		{name: "the file that was named", explicit: named, folder: folder, want: "The Named One"},
		{name: "the file in the posts folder", folder: folder, want: "The Folder One"},
		{name: "the file in the working folder", want: "The Working One"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			set, err := findSettings(c.explicit, c.folder, nil)
			if err != nil {
				t.Fatal(err)
			}
			if set.file.Title != c.want {
				t.Errorf("the title is %q, want %q", set.file.Title, c.want)
			}
		})
	}

	// with no file at all, the defaults in the code stand
	empty := t.TempDir()
	t.Chdir(empty)
	set, err := findSettings("", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if set.file.Addr != "127.0.0.1:8080" {
		t.Errorf("the address is %q, want the loopback one", set.file.Addr)
	}
}

// A settings file that says nothing is a legal file: the defaults stand.
func TestEmptySettingsFileIsLegal(t *testing.T) {
	// arrange
	dir := t.TempDir()
	path := filepath.Join(dir, settingsName)
	write(t, path, "\n\n")

	// act
	set, err := findSettings(path, "", nil)

	// assert
	if err != nil {
		t.Fatal(err)
	}
	if set.file.Title == "" || set.file.Addr == "" {
		t.Errorf("the defaults did not stand: %+v", set.file)
	}
}

// A path in the settings file is read from the folder the file is in.
func TestSettingsPathsAreRelativeToTheFile(t *testing.T) {
	// arrange
	dir := t.TempDir()
	write(t, filepath.Join(dir, settingsName), "posts: writing\ndist: site\n")
	t.Chdir(t.TempDir())

	// act
	set, err := findSettings(filepath.Join(dir, settingsName), "", nil)

	// assert
	if err != nil {
		t.Fatal(err)
	}
	if got := set.resolve(set.file.Posts); got != filepath.Join(dir, "writing") {
		t.Errorf("the posts are at %q, want %q", got, filepath.Join(dir, "writing"))
	}
	if got := set.resolve(set.file.Dist); got != filepath.Join(dir, "site") {
		t.Errorf("the site is at %q, want %q", got, filepath.Join(dir, "site"))
	}
}

// The settings the program was built with are not read: a copy of the program
// does not inherit the site it came from. With no settings file anywhere, the
// defaults in the code stand, and the site is published at the root.
func TestTheSettingsDoNotComeFromInsideTheProgram(t *testing.T) {
	// arrange
	t.Chdir(t.TempDir())

	// act
	set, err := findSettings("", "", nil)

	// assert
	if err != nil {
		t.Fatal(err)
	}
	if set.file.Base != "" {
		t.Errorf("the base is %q, want the root: the settings of the site the "+
			"program was built from must not be read", set.file.Base)
	}
	if set.from != "the defaults in the program" {
		t.Errorf("the settings came from %q, want the defaults in the code", set.from)
	}
}

// A build from a folder that holds no settings file is published at the root:
// the links carry no folder, because no settings file was read.
func TestABuildFromAnEmptyFolderCarriesNoFolder(t *testing.T) {
	// arrange
	dir := t.TempDir()
	posts := filepath.Join(dir, "posts")
	write(t, filepath.Join(posts, "one.md"),
		"---\ndate: 2026-01-02\ntag: earth\n---\n\n# One\n\n# Sentence\n\nA line about it.\n")
	out := filepath.Join(dir, "site")
	t.Chdir(t.TempDir())

	// act
	var stdout, stderr bytes.Buffer
	code := run([]string{"build", "-o", out, posts}, &stdout, &stderr)

	// assert
	if code != 0 {
		t.Fatalf("the code is %d, want 0 (the error stream said: %s)", code, stderr.String())
	}
	index, err := os.ReadFile(filepath.Join(out, "index.html"))
	if err != nil {
		t.Fatalf("the build did not write the index: %v", err)
	}
	if strings.Contains(string(index), "/baumkuchen/") {
		t.Errorf("the built site carries a folder it was never given: %s", index)
	}
	if !strings.Contains(string(index), `href="/post/one"`) {
		t.Errorf("the index does not link to the post at the root: %s", index)
	}
}

// The program does not guess that it is in the project folder: the templates
// and the stylesheet come from inside the program, and only --dev asks for the
// ones on disk.
func TestTheProgramDoesNotGuessTheFolder(t *testing.T) {
	// arrange
	dir := t.TempDir()
	write(t, filepath.Join(dir, "templates", "layout.html"), "the layout")
	write(t, filepath.Join(dir, "static", "style.css"), "body {}")
	t.Chdir(dir)

	// act
	plain := &work{cmd: &commandLine{name: "serve", given: map[string]bool{}}}
	templates, _, live, err := plain.files()

	// assert
	if err != nil {
		t.Fatal(err)
	}
	if live {
		t.Error("the program turned the live reload on by itself")
	}
	if _, err := fs.Stat(templates, "templates/layout.html"); err != nil {
		t.Errorf("the templates are not the ones inside the program: %v", err)
	}

	// and with --dev they are the ones on disk
	dev := &work{cmd: &commandLine{name: "serve", given: map[string]bool{"dev": true}}}
	templates, _, live, err = dev.files()
	if err != nil {
		t.Fatal(err)
	}
	if !live {
		t.Error("--dev did not turn the live reload on")
	}
	if _, err := fs.Stat(templates, "templates/layout.html"); err != nil {
		t.Errorf("--dev did not read the templates from disk: %v", err)
	}
}

// The drafts are shown on the loopback address, and held back anywhere else.
func TestLoopback(t *testing.T) {
	cases := map[string]bool{
		"127.0.0.1:8080": true,
		"localhost:8080": true,
		"[::1]:8080":     true,
		":8080":          false,
		"0.0.0.0:8080":   false,
		"192.168.1.5:80": false,
	}
	for addr, want := range cases {
		t.Run(addr, func(t *testing.T) {
			if got := loopback(addr); got != want {
				t.Errorf("loopback(%q) = %v, want %v", addr, got, want)
			}
		})
	}
}

// The wording of the operating system is not the same everywhere: on Linux
// the port that is taken says one thing, and on Windows another. Both mean
// the same thing.
func TestAddrInUse(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{name: "the linux wording",
			err:  errors.New("listen tcp 127.0.0.1:80: bind: address already in use"),
			want: true},
		{name: "the windows wording",
			err: errors.New("bind: Only one usage of each socket address " +
				"(protocol/network address/port) is normally permitted."),
			want: true},
		{name: "another fault",
			err:  errors.New("listen tcp 127.0.0.1:80: bind: permission denied"),
			want: false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := addrInUse(c.err); got != c.want {
				t.Errorf("addrInUse(%v) = %v, want %v", c.err, got, c.want)
			}
		})
	}
}

// A program that carries its settings reads them from inside itself.
func TestFindSettingsInsideTheProgram(t *testing.T) {
	// arrange
	files := fstest.MapFS{
		"content/baumkuchen.yml": &fstest.MapFile{Data: []byte("title: The Inside One\n")},
	}
	t.Chdir(t.TempDir())

	// act
	set, err := findSettings("", "", files)

	// assert
	if err != nil {
		t.Fatal(err)
	}
	if set.file.Title != "The Inside One" {
		t.Errorf("the title is %q, want the one inside the program", set.file.Title)
	}
}

// The posts come from the folder named on the command line, then from the
// environment, then from the settings, and last from the ones carried inside
// the program.
func TestThePostsComeFromTheRightPlace(t *testing.T) {
	// arrange
	dir := t.TempDir()
	other := t.TempDir()
	t.Setenv("BAUMKUCHEN_POSTS", "")
	set := &settings{file: blog.File{Posts: dir}, dir: "."}

	// act + assert
	// the folder named on the command line comes first
	named := &work{cmd: &commandLine{name: "serve", folder: dir, given: map[string]bool{}}}
	src, err := named.source(set)
	if err != nil {
		t.Fatal(err)
	}
	if !src.Exists() || src.Label() != dir {
		t.Errorf("the posts came from %q, want %q", src.Label(), dir)
	}

	// then the one in the settings
	fromFile := &work{cmd: &commandLine{name: "serve", given: map[string]bool{}}}
	src, err = fromFile.source(set)
	if err != nil {
		t.Fatal(err)
	}
	if src.Label() != dir {
		t.Errorf("the posts came from %q, want %q", src.Label(), dir)
	}

	// then the environment, when the folder was not named
	t.Setenv("BAUMKUCHEN_POSTS", other)
	fromEnv := &work{cmd: &commandLine{name: "serve", given: map[string]bool{}}}
	src, err = fromEnv.source(set)
	if err != nil {
		t.Fatal(err)
	}
	if src.Label() != other {
		t.Errorf("the posts came from %q, want %q", src.Label(), other)
	}
	t.Setenv("BAUMKUCHEN_POSTS", "")

	// a folder that is not there is an error, and says which one
	missing := &work{cmd: &commandLine{name: "serve",
		folder: filepath.Join(dir, "no-such-folder"), given: map[string]bool{}}}
	if _, err := missing.source(set); err == nil {
		t.Error("a folder that is not there must give an error")
	}

	// and a program with no posts at all says so
	none := &work{cmd: &commandLine{name: "serve", given: map[string]bool{}}}
	if _, err := none.source(&settings{file: blog.File{}, dir: "."}); err == nil {
		t.Error("no posts at all must give an error")
	}
}

// The build writes into the folder it was given, or into the one in the
// settings when the flag was given with nothing after it.
func TestTheBuildFolder(t *testing.T) {
	// arrange
	set := &settings{file: blog.File{Dist: "dist"}, dir: "/tmp/site"}

	// act + assert
	w := &work{cmd: &commandLine{name: "build", given: map[string]bool{}}}
	if got := w.outFolder(set); got != filepath.Join("/tmp/site", "dist") {
		t.Errorf("the site goes to %q, want the one in the settings", got)
	}
	named := &work{cmd: &commandLine{name: "build", given: map[string]bool{"out": true}}}
	named.cmd.flags = flag.NewFlagSet("build", flag.ContinueOnError)
	named.cmd.flags.String("out", "", "")
	if err := named.cmd.flags.Set("out", "out"); err != nil {
		t.Fatal(err)
	}
	if got := named.outFolder(set); got != "out" {
		t.Errorf("the site goes to %q, want the one that was named", got)
	}
}

// The drafts are shown on the loopback address, and held back anywhere else.
func TestDraftsFollowTheAddress(t *testing.T) {
	// arrange
	dir := t.TempDir()
	write(t, filepath.Join(dir, "one.md"),
		"---\ndate: 2026-01-02\ndraft: true\n---\n\n# One\n\n# Sentence\n\nA line about it.\n")
	t.Chdir(t.TempDir())

	// act + assert
	cases := []struct {
		addr string
		want bool
	}{
		{addr: "127.0.0.1:8080", want: true},
		{addr: "0.0.0.0:8080", want: false},
	}
	for _, c := range cases {
		t.Run(c.addr, func(t *testing.T) {
			w := &work{cmd: &commandLine{name: "serve", given: map[string]bool{}}}
			if got := w.drafts(c.addr); got != c.want {
				t.Errorf("drafts on %s = %v, want %v", c.addr, got, c.want)
			}
		})
	}

	// the flags say so either way
	w := &work{cmd: &commandLine{name: "serve", given: map[string]bool{"drafts": true}}}
	if !w.drafts("0.0.0.0:8080") {
		t.Error("--drafts must show the drafts anywhere")
	}
	w = &work{cmd: &commandLine{name: "serve", given: map[string]bool{"no-drafts": true}}}
	if w.drafts("127.0.0.1:8080") {
		t.Error("--no-drafts must hide the drafts even on the loopback address")
	}
}

// The build writes the site, and the summary of it goes to the standard
// output while the log goes to the error stream.
func TestBuildWritesTheSite(t *testing.T) {
	// arrange
	dir := t.TempDir()
	posts := filepath.Join(dir, "posts")
	write(t, filepath.Join(posts, "one.md"),
		"---\ndate: 2026-01-02\ntag: earth\n---\n\n# One\n\n# Sentence\n\nA line about it.\n")
	out := filepath.Join(dir, "site")
	t.Chdir(t.TempDir())

	// act
	var stdout, stderr bytes.Buffer
	code := run([]string{"build", "-o", out, posts}, &stdout, &stderr)

	// assert
	if code != 0 {
		t.Fatalf("the code is %d, want 0 (the error stream said: %s)", code, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(out, "index.html")); err != nil {
		t.Fatalf("the build did not write the index: %v", err)
	}
	if _, err := os.Stat(filepath.Join(out, "post", "one", "index.html")); err != nil {
		t.Fatalf("the build did not write the post: %v", err)
	}
	if !strings.Contains(stdout.String(), "file(s) written to") {
		t.Errorf("the summary is not on the standard output: %q", stdout.String())
	}
	if strings.Contains(stdout.String(), "the settings come from") {
		t.Error("the log is on the standard output, where the result belongs")
	}
	if !strings.Contains(stderr.String(), "the settings come from") {
		t.Errorf("the log is not on the error stream: %q", stderr.String())
	}
}

// The check says what is wrong with the posts, and the JSON holds the same
// report as the text.
func TestCheckReports(t *testing.T) {
	// arrange
	dir := t.TempDir()
	posts := filepath.Join(dir, "posts")
	write(t, filepath.Join(posts, "good.md"),
		"---\ndate: 2026-01-02\n---\n\n# Good\n\n# Sentence\n\nA line about it.\n")
	write(t, filepath.Join(posts, "nodepth.md"), "# Nothing\n\nJust a title.\n")
	write(t, filepath.Join(posts, "baddate.md"),
		"---\ndate: yesterday\n---\n\n# Bad Date\n\n# Sentence\n\nA line about it.\n")
	write(t, filepath.Join(posts, "picture.md"),
		"---\ndate: 2026-01-02\n---\n\n# Picture\n\n# Sentence\n\n![alt](/media/nope.jpg)\n")
	t.Chdir(t.TempDir())

	// act
	var stdout, stderr bytes.Buffer
	code := run([]string{"check", "--json", posts}, &stdout, &stderr)

	// assert
	if code != 1 {
		t.Errorf("the code is %d, want 1 (the error stream said: %s)", code, stderr.String())
	}
	var report map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("the report is not JSON: %v\n%s", err, stdout.String())
	}
	if report["published"] != float64(3) {
		t.Errorf("the report says %v published, want 3", report["published"])
	}
	if report["errors"].(float64) < 3 {
		t.Errorf("the report found %v error(s), want at least 3", report["errors"])
	}
	kinds := map[string]bool{}
	for _, f := range report["findings"].([]any) {
		kinds[f.(map[string]any)["kind"].(string)] = true
	}
	for _, want := range []string{"missing-depths", "bad-date", "missing-picture"} {
		if !kinds[want] {
			t.Errorf("the report does not mention %q: %s", want, stdout.String())
		}
	}
}

// The text report and the JSON report say the same thing.
func TestCheckTextAndJSONAgree(t *testing.T) {
	// arrange
	dir := t.TempDir()
	posts := filepath.Join(dir, "posts")
	write(t, filepath.Join(posts, "one.md"),
		"---\ndate: 2026-01-02\n---\n\n# One\n\n# Sentence\n\nA line about it.\n")
	t.Chdir(t.TempDir())

	// act
	var text, stderr bytes.Buffer
	code := run([]string{"check", posts}, &text, &stderr)
	var raw, rawErr bytes.Buffer
	jsonCode := run([]string{"check", "--json", posts}, &raw, &rawErr)

	// assert
	if code != 0 || jsonCode != 0 {
		t.Fatalf("the codes are %d and %d, want 0 and 0 (the error stream said: %s)",
			code, jsonCode, stderr.String())
	}
	var report map[string]any
	if err := json.Unmarshal(raw.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text.String(), "1 published post(s)") {
		t.Errorf("the text report does not give the count: %q", text.String())
	}
	if report["published"] != float64(1) {
		t.Errorf("the JSON report gives %v published, want 1", report["published"])
	}
}

// The check knows that the placement at the end of a picture's path — #left,
// #right or #center — is not part of the name of the file. A picture that is
// there is not reported missing, and one that is not there is named without
// the placement.
func TestCheckKnowsThePlacementIsNotTheName(t *testing.T) {
	// arrange
	dir := t.TempDir()
	posts := filepath.Join(dir, "posts")
	write(t, filepath.Join(posts, "images", "here.jpg"), "not really a picture")
	write(t, filepath.Join(posts, "one.md"),
		"---\ndate: 2026-01-02\n---\n\n# One\n\n# Sentence\n\n"+
			"![alt](/media/here.jpg#center)\n\n![alt](/media/gone.jpg#center)\n")
	t.Chdir(t.TempDir())

	// act
	var stdout, stderr bytes.Buffer
	code := run([]string{"check", "--json", posts}, &stdout, &stderr)

	// assert
	if code != 1 {
		t.Errorf("the code is %d, want 1 (the error stream said: %s)", code, stderr.String())
	}
	var report map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("the report is not JSON: %v\n%s", err, stdout.String())
	}
	findings := report["findings"].([]any)
	var pictures []map[string]any
	for _, raw := range findings {
		f := raw.(map[string]any)
		if f["kind"] == "missing-picture" {
			pictures = append(pictures, f)
		}
		if msg, _ := f["message"].(string); strings.Contains(msg, "here.jpg") {
			t.Errorf("a picture that is there is reported missing: %q", msg)
		}
	}
	if len(pictures) != 1 {
		t.Fatalf("the report holds %d missing picture(s), want 1: %s", len(pictures), stdout.String())
	}
	msg, _ := pictures[0]["message"].(string)
	if !strings.Contains(msg, "gone.jpg") || strings.Contains(msg, "#") {
		t.Errorf("the finding does not name the picture without the placement: %q", msg)
	}
}

// The init command writes a settings file, and refuses to write over one
// that is already there.
func TestInitWritesTheSettingsFile(t *testing.T) {
	// arrange
	dir := t.TempDir()
	t.Chdir(t.TempDir())

	// act
	var stdout, stderr bytes.Buffer
	code := run([]string{"init", dir}, &stdout, &stderr)

	// assert
	if code != 0 {
		t.Fatalf("the code is %d, want 0 (the error stream said: %s)", code, stderr.String())
	}
	path := filepath.Join(dir, settingsName)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("the settings file is not there: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "posts")); err != nil {
		t.Fatalf("the posts folder is not there: %v", err)
	}

	// again, without the force: the file that is there stays
	code = run([]string{"init", dir}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("the code is %d, want 1 when the file is already there", code)
	}
	if !strings.Contains(stderr.String(), "--force") {
		t.Errorf("the message does not say how to go on: %q", stderr.String())
	}

	// and with it
	code = run([]string{"init", "--force", dir}, &stdout, &stderr)
	if code != 0 {
		t.Errorf("the code is %d, want 0 with the force", code)
	}
}

// The init command writes a settings file the program can read back.
func TestInitWritesAFileThatIsRead(t *testing.T) {
	// arrange
	dir := t.TempDir()
	t.Chdir(t.TempDir())
	var stdout, stderr bytes.Buffer
	if code := run([]string{"init", dir}, &stdout, &stderr); code != 0 {
		t.Fatalf("the code is %d, want 0", code)
	}

	// act
	set, err := findSettings(filepath.Join(dir, settingsName), "", nil)

	// assert
	if err != nil {
		t.Fatal(err)
	}
	if set.file.Addr != "127.0.0.1:8080" {
		t.Errorf("the address is %q, want the loopback one", set.file.Addr)
	}
}
