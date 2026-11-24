// The two folders a build works with, and the rules that keep them apart.
//
// A build writes a whole site into one folder. The writing it reads must
// never be in the way of that: the site and the writing are different
// things, and a folder that holds one must not hold the other.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// buildPath holds the two folders a build works with.
//
// Both are absolute. The writing folder is often a link out of the
// repository, so the link is followed and the folder the link points at is
// the folder the writing is really in: that is the folder a comparison has
// to be made against.
type buildPath struct {
	// out is the folder the site is written into.
	out string
	// posts is the folder the writing is read from.
	posts string
}

// checkBuildPaths refuses a build that would put the site and the writing in
// the same place, or in one another's way.
//
// The rules are these:
//
//   - the site may not be written into the writing itself;
//   - the site may not be written into a folder that holds the writing,
//     because a clean of that folder would reach the writing;
//   - the site may be written inside the writing folder only when the
//     folder it is written into is named "dist" or begins with a dot, so
//     that it is plainly a build folder and not more writing.
//
// The message names both folders, so the writer can see what was compared.
func checkBuildPaths(out, posts string) error {
	o, err := resolvePath(out)
	if err != nil {
		return err
	}
	p, err := resolvePath(posts)
	if err != nil {
		return err
	}
	switch {
	case o == p:
		return usagef("the site would be written into the writing itself: "+
			"the output %s and the writing %s are the same folder", o, p)
	case isInside(p, o):
		return usagef("the site would be written over the writing: "+
			"the output %s holds the writing at %s", o, p)
	case isInside(o, p) && !isBuildFolder(o):
		return usagef("the site would be written into the writing: "+
			"the output %s is inside the writing at %s, and only a folder "+
			"named \"dist\" or one that begins with a dot may be written there",
			o, p)
	}
	return nil
}

// isBuildFolder says whether a folder is plainly a place a build writes to.
//
// The name "dist" is the one the settings use, and a name that begins with
// a dot is hidden: both say the folder holds what a build made, and not
// writing.
func isBuildFolder(path string) bool {
	name := filepath.Base(path)
	return name == "dist" || strings.HasPrefix(name, ".")
}

// isInside says whether a folder is in another one.
//
// The folder itself is not inside itself: a folder that holds the writing is
// not inside the writing, it is around it.
func isInside(inner, outer string) bool {
	rel, err := filepath.Rel(outer, inner)
	if err != nil {
		return false
	}
	if rel == "." || rel == ".." {
		return false
	}
	return !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// resolvePath gives a path as an absolute one with every link in it
// followed.
//
// A path that is not there yet is resolved as far as it goes: the folders
// above it are followed, and the rest is kept as it was written. That way a
// folder that is about to be made is compared with the folder that is
// there.
//
// A link that points nowhere is still a link, and is followed too, so that a
// folder that is only a link is compared with the folder it points at.
func resolvePath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return resolveFrom(abs, 0)
}

// resolveFrom follows the links in a path, giving up after too many of them.
func resolveFrom(abs string, depth int) (string, error) {
	if depth > 64 {
		return "", fmt.Errorf("too many links in %s", abs)
	}
	rest := ""
	cur := abs
	for {
		resolved, err := filepath.EvalSymlinks(cur)
		if err == nil {
			if rest == "" {
				return filepath.Clean(resolved), nil
			}
			return filepath.Join(resolved, rest), nil
		}
		if link, lerr := os.Readlink(cur); lerr == nil {
			target := link
			if !filepath.IsAbs(target) {
				target = filepath.Join(filepath.Dir(cur), target)
			}
			return resolveFrom(filepath.Join(target, rest), depth+1)
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return filepath.Join(abs, rest), nil
		}
		rest = filepath.Join(filepath.Base(cur), rest)
		cur = parent
	}
}
