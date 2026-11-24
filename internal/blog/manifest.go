// The manifest: the list of the files a build wrote.
//
// A build writes a whole site into one folder. A clean that empties that
// folder cannot tell the files a build made from the files the writer put
// there, so it takes the files of the last build from a list the build left
// behind, and removes those and nothing else.
package blog

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// manifestName is the file in the folder of a build that holds the list of
// the files the build wrote.
const manifestName = ".baumkuchen-build"

// writeManifest writes the list of the files a build wrote.
//
// The list holds one relative path on each line, in the order the files are
// sorted, so the same build always writes the same list.
func writeManifest(dir string, files []string) error {
	names := append([]string(nil), files...)
	sort.Strings(names)
	var b strings.Builder
	for _, name := range names {
		b.WriteString(name)
		b.WriteString("\n")
	}
	return os.WriteFile(filepath.Join(dir, manifestName), []byte(b.String()), 0o644)
}

// readManifest reads the list of the files a build wrote.
//
// A line that is not a plain relative path is a fault, and the whole list
// is refused: a list that cannot be trusted must not be used to remove
// anything.
func readManifest(dir string) ([]string, error) {
	raw, err := os.ReadFile(filepath.Join(dir, manifestName))
	if err != nil {
		return nil, err
	}
	var names []string
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			continue
		}
		if err := checkManifestName(line); err != nil {
			return nil, err
		}
		names = append(names, line)
	}
	return names, nil
}

// checkManifestName says whether a line in the manifest names a file inside
// the folder of the build.
//
// A name that is absolute, or that climbs out of the folder with "..", is
// not a name of a file the build wrote, and is refused.
func checkManifestName(name string) error {
	if name == "" {
		return fmt.Errorf("the manifest names an empty file")
	}
	if filepath.IsAbs(name) || strings.HasPrefix(name, "/") {
		return fmt.Errorf("the manifest names %s, which is not inside the build", name)
	}
	for _, part := range strings.Split(filepath.ToSlash(name), "/") {
		if part == ".." {
			return fmt.Errorf("the manifest names %s, which climbs out of the build", name)
		}
	}
	return nil
}

// manifestTargets gives the files a manifest names that are there to be
// removed.
//
// The folders above a file must be real folders. A folder that is a link
// could point out of the build, and removing a file through it would take
// a file that has nothing to do with the build. The whole list is refused
// when that happens, so nothing is removed at all. A folder that is not a
// folder is refused the same way.
//
// A file that is not there is not a fault: it was removed already, or the
// build never wrote it.
func manifestTargets(dir string, names []string) ([]string, error) {
	var targets []string
	for _, name := range names {
		parts := strings.Split(filepath.ToSlash(name), "/")
		here := dir
		linked := false
		missing := false
		for _, part := range parts[:max(len(parts)-1, 0)] {
			here = filepath.Join(here, part)
			info, err := os.Lstat(here)
			if err != nil {
				missing = true
				break
			}
			if info.Mode()&os.ModeSymlink != 0 {
				linked = true
				break
			}
			if !info.IsDir() {
				return nil, fmt.Errorf("the manifest names %s, and %s is not a folder: "+
					"nothing was removed", name, here)
			}
		}
		if linked {
			return nil, fmt.Errorf("the manifest names %s, and %s is a link: "+
				"nothing was removed", name, here)
		}
		if missing {
			continue
		}
		targets = append(targets, filepath.Join(dir, filepath.FromSlash(name)))
	}
	return targets, nil
}

// removeEmptyFolders removes the folders a removed file was in, as far up as
// they are empty and as far down as the folder of the build.
//
// A folder that still holds anything is left alone, and so is the folder of
// the build itself.
func removeEmptyFolders(dir string, folders []string) error {
	sorted := append([]string(nil), folders...)
	sort.Slice(sorted, func(i, j int) bool {
		return strings.Count(sorted[i], string(filepath.Separator)) >
			strings.Count(sorted[j], string(filepath.Separator))
	})
	for _, folder := range sorted {
		for folder != dir && folder != "" && folder != "." {
			if err := os.Remove(folder); err != nil {
				break // it still holds something, or it is not there
			}
			folder = filepath.Dir(folder)
		}
	}
	return nil
}
