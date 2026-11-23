// Baumkuchen is a blog that serves markdown from a folder, at up to five
// lengths.
//
// The program has a few commands:
//
//	baumkuchen                     # serve the posts, live, on the loopback address
//	baumkuchen serve ~/blog        # serve the posts in another folder
//	baumkuchen build -o out        # write the static site, then stop
//	baumkuchen check --json        # say what is wrong with the posts
//	baumkuchen bundle -o blog      # one file with the program and the posts in it
//	baumkuchen init my-blog        # a new site, with a settings file
//	baumkuchen version             # say which build this is
//
// The templates, the stylesheet and the script are built into it, so a copy
// of it runs anywhere, without the folder it was made in. The posts are not
// built into it: they come from a folder you name, so a site is never a copy
// of somebody else's writing by accident.
//
// Everything else is in internal/blog. This file holds the two embedded
// folders, because //go:embed reads its paths from the folder of the package
// that asks for them.
package main

import (
	"embed"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime/debug"
)

//go:embed templates/*.html
var templateFiles embed.FS

//go:embed static/*
var staticFiles embed.FS

// The build puts its own name in here, so a copy of the program can say which
// one it is.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

// versionLine gives the name of the build.
//
// The build puts the version, the commit and the day into the program when it
// is made. When it was not made that way, the information the Go toolchain
// keeps about the build is used instead, so the program still says something
// true about itself.
func versionLine() string {
	v, c, d := version, commit, date
	if v == "dev" || v == "" {
		if info, ok := debug.ReadBuildInfo(); ok {
			if info.Main.Version != "" && info.Main.Version != "(devel)" {
				v = info.Main.Version
			}
			for _, s := range info.Settings {
				switch s.Key {
				case "vcs.revision":
					if c == "none" || c == "" {
						c = shortSHA(s.Value)
					}
				case "vcs.time":
					if d == "unknown" || d == "" {
						d = shortTime(s.Value)
					}
				}
			}
		}
	}
	return fmt.Sprintf("baumkuchen %s (%s, %s)", v, c, d)
}

// shortSHA gives the first few characters of a commit name.
func shortSHA(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

// shortTime gives the day and the hour of a time, as the build writes it.
func shortTime(stamp string) string {
	if len(stamp) > 19 {
		return stamp[:19]
	}
	return stamp
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run does what the command line asks, and gives the code to leave with.
//
// The codes are the usual ones: 0 when all went well, 1 when something went
// wrong, and 2 when the command line itself was wrong.
func run(argv []string, stdout, stderr io.Writer) int {
	err := dispatch(argv, stdout, stderr)
	switch {
	case err == nil:
		return 0
	case errors.Is(err, errQuiet):
		return 1
	}
	var usage usageError
	if errors.As(err, &usage) {
		fmt.Fprintf(stderr, "baumkuchen: %s\n", usage.Error())
		return 2
	}
	fmt.Fprintf(stderr, "baumkuchen: %v\n", err)
	return 1
}
