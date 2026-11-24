package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The help names the value each flag takes, so the reader knows what to
// write: an address is an address, a title is text, a folder is a folder, a
// path is a path and a file is a file.
func TestTheHelpNamesTheValueEachFlagTakes(t *testing.T) {
	// arrange
	cases := map[string][]string{
		"serve":  {"--addr ADDR", "--title TEXT", "--heading TEXT", "--favicon PATH"},
		"build":  {"-o, --out DIR", "--base PATH", "--title TEXT", "--heading TEXT", "--favicon PATH"},
		"bundle": {"-o, --out FILE"},
	}

	// act + assert
	for command, wants := range cases {
		t.Run(command, func(t *testing.T) {
			text := commandUsage(command)
			for _, want := range wants {
				if !strings.Contains(text, want) {
					t.Errorf("the help of %s does not say %q:\n%s", command, want, text)
				}
			}
		})
	}

	// the settings file is a path, wherever it is looked for
	if !strings.Contains(usageText("", false), "--config PATH") {
		t.Error("the help does not say what the config flag takes")
	}
}

// The flag that says nothing was taken away: it did nothing, and a flag that
// does nothing is a trap.
func TestTheVerboseFlagIsGone(t *testing.T) {
	// act
	cmd, err := readArgs([]string{"--verbose"})

	// assert
	if err == nil {
		t.Fatalf("the flag was accepted, want an error: %+v", cmd)
	}
	if !strings.Contains(err.Error(), "unknown flag") {
		t.Errorf("the error is %q, want one about an unknown flag", err.Error())
	}
}

// The help is a command as well as a flag, so it can be asked for on its own.
func TestTheHelpIsACommand(t *testing.T) {
	// arrange
	cases := []struct {
		name string
		argv []string
		want string
	}{
		{name: "the whole program", argv: []string{"help"}, want: "Usage:"},
		{name: "one command", argv: []string{"help", "build"}, want: "baumkuchen build"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// act
			var stdout, stderr bytes.Buffer
			code := run(c.argv, &stdout, &stderr)

			// assert
			if code != 0 {
				t.Fatalf("the code is %d, want 0 (the error stream said: %s)", code, stderr.String())
			}
			if !strings.Contains(stdout.String(), c.want) {
				t.Errorf("the help does not hold %q: %q", c.want, stdout.String())
			}
		})
	}

	// the name of a command that is not there is a mistake on the command line
	var stdout, stderr bytes.Buffer
	if code := run([]string{"help", "nosuchthing"}, &stdout, &stderr); code != 2 {
		t.Errorf("the code is %d, want 2 for a command that is not there", code)
	}
}

// The two flags that say the opposite of each other cannot both be given:
// the writer asked for two things at once.
func TestTheDraftsFlagsCannotBothBeGiven(t *testing.T) {
	// act
	var stdout, stderr bytes.Buffer
	code := run([]string{"serve", "--drafts", "--no-drafts"}, &stdout, &stderr)

	// assert
	if code != 2 {
		t.Errorf("the code is %d, want 2 (the error stream said: %s)", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "cannot be used together") {
		t.Errorf("the message does not say what is wrong: %q", stderr.String())
	}
}

// A flag that was given as --name=false is off, even though it was given.
func TestAFlagGivenAsFalseIsOff(t *testing.T) {
	// act
	cmd, err := readArgs([]string{"serve", "--open=false"})

	// assert
	if err != nil {
		t.Fatal(err)
	}
	if cmd.bool("open") {
		t.Error("--open=false must leave the browser shut")
	}

	// and without the value it is on
	cmd, err = readArgs([]string{"serve", "--open"})
	if err != nil {
		t.Fatal(err)
	}
	if !cmd.bool("open") {
		t.Error("--open must open the browser")
	}
}

// The init command writes a first post as well as the settings file, so a
// new site has something in it and the shape of a post is shown.
func TestInitWritesAFirstPost(t *testing.T) {
	// arrange
	dir := t.TempDir()
	t.Chdir(t.TempDir())

	// act
	var stdout, stderr bytes.Buffer
	if code := run([]string{"init", dir}, &stdout, &stderr); code != 0 {
		t.Fatalf("the code is %d, want 0 (the error stream said: %s)", code, stderr.String())
	}

	// assert
	post := filepath.Join(dir, "posts", "hello.md")
	if _, err := os.Stat(post); err != nil {
		t.Fatalf("the first post is not there: %v", err)
	}
	body, err := os.ReadFile(post)
	if err != nil {
		t.Fatal(err)
	}
	// the post carries every depth, so the site has a full page to show
	for _, want := range []string{"# Sentence", "# Paragraph", "# Short", "# Medium", "# Long"} {
		if !strings.Contains(string(body), want) {
			t.Errorf("the first post does not carry %q:\n%s", want, body)
		}
	}

	// and it is a post the program can read: the site holds one
	var out, errOut bytes.Buffer
	if code := run([]string{"check", dir}, &out, &errOut); code != 0 {
		t.Errorf("the code is %d, want 0 (the error stream said: %s)", code, errOut.String())
	}
	if !strings.Contains(out.String(), "1 published post(s)") {
		t.Errorf("the site does not hold the first post: %q", out.String())
	}

	// the post that is there is not written over: it is the writer's now
	write(t, post, "# Mine\n")
	if code := run([]string{"init", "--force", dir}, &stdout, &stderr); code != 0 {
		t.Fatalf("the code is %d, want 0 with the force", code)
	}
	body, err = os.ReadFile(post)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "# Mine") {
		t.Error("the first post was written over: the writing of the writer is lost")
	}
}

// A folder named on the command line that holds a settings file of its own
// is read through that file: the posts line in it says where the writing is.
func TestANamedFolderHonoursItsPostsLine(t *testing.T) {
	// arrange
	dir := t.TempDir()
	write(t, filepath.Join(dir, settingsName), "title: Mine\nposts: writing\n")
	write(t, filepath.Join(dir, "writing", "one.md"),
		"---\ndate: 2026-01-02\n---\n\n# One\n\n# Sentence\n\nA line.\n")
	t.Chdir(t.TempDir())

	// act
	var stdout, stderr bytes.Buffer
	code := run([]string{"check", dir}, &stdout, &stderr)

	// assert
	if code != 0 {
		t.Fatalf("the code is %d, want 0 (the error stream said: %s)", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "1 published post(s)") {
		t.Errorf("the post in the folder the settings name was not read: %q", stdout.String())
	}
}

// A folder named on the command line that holds no settings file of its own
// is the writing itself, whatever the working folder says.
func TestANamedFolderWithoutSettingsIsTheWriting(t *testing.T) {
	// arrange
	dir := t.TempDir()
	write(t, filepath.Join(dir, "one.md"),
		"---\ndate: 2026-01-02\n---\n\n# One\n\n# Sentence\n\nA line.\n")
	t.Chdir(t.TempDir())

	// act
	var stdout, stderr bytes.Buffer
	code := run([]string{"check", dir}, &stdout, &stderr)

	// assert
	if code != 0 {
		t.Fatalf("the code is %d, want 0 (the error stream said: %s)", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "1 published post(s)") {
		t.Errorf("the post in the folder was not read: %q", stdout.String())
	}
}
