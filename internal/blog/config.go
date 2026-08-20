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

// Defaults are what a site holds before its settings file is read.
func Defaults() File {
	return File{Title: "Four depths", Posts: "posts", Dist: "dist", Base: "", Addr: ":8080"}
}

// LoadFile reads the settings, on top of the defaults. A file that is not
// there is not a fault: the defaults stand, and the caller is told so it can
// keep quiet or say something.
func LoadFile(path string) (File, bool, error) {
	cfg := Defaults()
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, false, nil
		}
		return cfg, false, err
	}
	// UnmarshalStrict in spirit: an unknown key is a typo, and a typo that is
	// swallowed is a setting that quietly does nothing.
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)
	if err := dec.Decode(&cfg); err != nil {
		return Defaults(), true, fmt.Errorf("%s: %w", path, err)
	}
	return cfg, true, nil
}
