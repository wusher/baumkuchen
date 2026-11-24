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
	// Heading is the homepage h1. Empty hides it.
	Heading string `yaml:"heading"`
	// Favicon overrides favicon.svg in the posts folder with a local file.
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

// Written gives the names of the settings a file wrote down.
//
// A setting that is not in the file is not written: only the default in the
// code stands for it. This tells the two apart, so a caller knows whether a
// line was chosen or only inherited.
func Written(raw []byte) map[string]bool {
	out := map[string]bool{}
	var fields map[string]yaml.Node
	if err := yaml.Unmarshal(raw, &fields); err != nil {
		return out
	}
	for name := range fields {
		out[name] = true
	}
	return out
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

// WithFavicon gives the settings file with the picture on the tab changed.
//
// The file is read and written as YAML, so the comments in it stay. The
// picture is named by the name it has inside the program: a program that
// carries its settings carries the picture with it, and the picture is read
// from there.
//
// A file with no picture on the tab in it is given one.
func WithFavicon(raw []byte, name string) ([]byte, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return nil, err
	}
	if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		// an empty file: there is nothing to change, and the picture is
		// the one in the writing
		return raw, nil
	}
	mapping := doc.Content[0]
	found := false
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		key := mapping.Content[i]
		if key.Value != "favicon" {
			continue
		}
		value := mapping.Content[i+1]
		value.Kind = yaml.ScalarNode
		value.Tag = "!!str"
		value.Value = name
		value.Content = nil
		value.Style = 0
		found = true
	}
	if !found {
		mapping.Content = append(mapping.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "favicon"},
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: name})
	}
	out, err := yaml.Marshal(&doc)
	if err != nil {
		return nil, err
	}
	return out, nil
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
