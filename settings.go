package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/wusher/baumkuchen/internal/blog"
	"github.com/wusher/baumkuchen/internal/bundle"
)

// The name of the settings file.
const settingsName = "baumkuchen.yml"

// settings are the settings of one site, and where they came from.
type settings struct {
	// file is what was read.
	file blog.File
	// dir is the folder the settings file is in. A path in the file is read
	// from here, so the same file works wherever it is kept.
	dir string
	// from says where the settings came from, for the log.
	from string
	// bytes is the file as it was read, so that a bundle can carry it.
	bytes []byte
	// written holds the names the file wrote down. A setting that is not in
	// it is only the default in the code, and a caller that needs to tell
	// the two apart looks here.
	written map[string]bool
	// inside says the settings came from inside the program. A path in them
	// is a path inside the program, and not a path on the disk of the
	// machine the program runs on.
	inside bool
}

// names says whether the settings file wrote this setting down, rather than
// it standing at the default.
func (s *settings) names(setting string) bool { return s.written[setting] }

// resolve gives the path of a folder named in the settings file.
//
// A path that is absolute is left alone. One that is relative is read from
// the folder the settings file is in, so the settings file and the posts
// travel together.
func (s *settings) resolve(path string) string {
	if path == "" || filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(s.dir, path)
}

// findSettings reads the settings for a run.
//
// The file named on the command line comes first. Then the one in the posts
// folder. Then the one inside a program that carries one. Then the one in
// the working folder. Last are the defaults in the code.
//
// The settings inside the program come before the file in the working
// folder: a bundle is the same site in every folder, and a file in the
// folder the program happens to run in would make it a different site in
// each one. The file named on the command line comes first of all, so that
// a writer can replace them on purpose.
//
// The settings of the site the program was made in are not built into it.
// A copy of the program does not inherit the site it was built from: a build
// from a folder that holds no settings file is published at the root, and
// the links it writes carry no folder. A program that carries settings
// carries them on purpose, and they are the ones inside it.
//
// A file that is named and is not there is a mistake, and it is reported as
// one. A file that is not named and is not there is not a mistake at all:
// the next place in the list is looked at.
func findSettings(explicit, folder string, bundled fs.FS) (*settings, error) {
	if explicit != "" {
		return readSettings(explicit, filepath.Dir(explicit))
	}
	if folder != "" {
		path := filepath.Join(folder, settingsName)
		if _, err := os.Stat(path); err == nil {
			return readSettings(path, folder)
		}
	}
	if bundled != nil {
		name := bundle.Root + "/" + settingsName
		raw, err := fs.ReadFile(bundled, name)
		if err == nil {
			file, err := blog.Load(raw)
			if err != nil {
				return nil, fmt.Errorf("the settings inside the program: %w", err)
			}
			return &settings{
				file:    file,
				dir:     ".",
				from:    "the settings inside the program",
				bytes:   raw,
				written: blog.Written(raw),
				inside:  true,
			}, nil
		}
	}
	if _, err := os.Stat(settingsName); err == nil {
		return readSettings(settingsName, ".")
	}
	// the settings of the site the program was made in are not read: a copy
	// of it does not inherit the site it was built from
	return &settings{
		file:    blog.Defaults(),
		dir:     ".",
		from:    "the defaults in the program",
		bytes:   []byte(starterSettings),
		written: map[string]bool{},
	}, nil
}

// readSettings reads one settings file.
func readSettings(path, dir string) (*settings, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	file, err := blog.Load(raw)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &settings{
		file:    file,
		dir:     dir,
		from:    path,
		bytes:   raw,
		written: blog.Written(raw),
	}, nil
}

// starterSettings is the settings file the init command writes, and the one
// a bundle carries when there is no settings file to carry.
const starterSettings = `# The settings for this site. Every one of them has a working
# default, so this file can be short, or missing altogether. A flag beats
# what is written here:

#   baumkuchen serve --addr 127.0.0.1:3000

# the name of the site, in the tab and as the wordmark
title: Four Depths

# the line under the wordmark
heading:

# the picture the browser shows on the tab
favicon:

# the folder the markdown is read from
posts: posts

# where the built site is written
dist: dist

# the folder the built site is published in, as in /baumkuchen for a project
# page; leave this empty for a custom domain or a user site
base:

# where the server listens
addr: 127.0.0.1:8080
`

// starterPost is the first post the init command writes, so that a new site
// has something in it and the shape of a post is shown.
//
// The post itself is written by the blog package, where the depths are: the
// names of the depths are said in one place only.
var starterPost = blog.StarterPost()
