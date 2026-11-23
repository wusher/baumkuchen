// The settings file: the few things that change from site to site, and from
// one machine to the next.
package blog

import (
	"bytes"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// File is what baumkuchen.yml holds. Every field has a working default, so a
// site with no settings file still runs.
type File struct {
	// Title is the name of the site, in the tab and beside every page name.
	Title string `yaml:"title"`
	// Heading is the line under the wordmark: a strapline, or a subtitle.
	Heading string `yaml:"heading"`
	// Favicon is the picture the browser shows on the tab.
	Favicon string `yaml:"favicon"`
	// Posts is the folder the markdown is read from.
	Posts string `yaml:"posts"`
	// Dist is the folder the built site is written to.
	Dist string `yaml:"dist"`
	// Base is the folder the built site is published in, as in /baumkuchen
	// for a project page. Empty means the root.
	Base string `yaml:"base"`
	// Addr is where the server listens.
	Addr string `yaml:"addr"`
}

// Set puts a value on a setting by its name.
//
// It is how a flag beats the file: the flag is named, and the setting it
// names takes its value.
func (f *File) Set(name, value string) {
	switch name {
	case "title":
		f.Title = value
	case "heading":
		f.Heading = value
	case "favicon":
		f.Favicon = value
	case "posts":
		f.Posts = value
	case "dist":
		f.Dist = value
	case "base":
		f.Base = value
	case "addr":
		f.Addr = value
	}
}

// Defaults are what a site holds before its settings file is read.
//
// The address is the loopback one: a server that answers only this machine
// is the safe first answer, and a writer who wants it on the network says so.
func Defaults() File {
	return File{
		Title: "Four Depths",
		Posts: "posts",
		Dist:  "dist",
		Base:  "",
		Addr:  "127.0.0.1:8080",
	}
}

// LoadFile reads the settings, on top of the defaults. A file that is not
// there is not a fault: the defaults stand, and the caller is told so it can
// keep quiet or say something.
func LoadFile(path string) (File, bool, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Defaults(), false, nil
		}
		return Defaults(), false, err
	}
	file, err := Load(raw)
	if err != nil {
		return Defaults(), true, fmt.Errorf("%s: %w", path, err)
	}
	return file, true, nil
}

// Load reads the settings from what is written, on top of the defaults. It is
// how the settings inside the program are read: the same file, wherever it
// came from.
//
// An empty file is a legal file: it says nothing, so the defaults stand.
func Load(raw []byte) (File, error) {
	file := Defaults()
	if len(bytes.TrimSpace(raw)) == 0 {
		return file, nil
	}
	// UnmarshalStrict in spirit: an unknown key is a typo, and a typo that is
	// swallowed is a setting that quietly does nothing.
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)
	if err := dec.Decode(&file); err != nil {
		return Defaults(), err
	}
	return file, nil
}
