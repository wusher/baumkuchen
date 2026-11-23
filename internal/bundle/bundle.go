// Package bundle puts the content inside a copy of the program, so that one
// file carries everything it needs: the program, the templates, the stylesheet
// and the posts.
//
// The file is the program, then a zip of the content, then a short trailer
// that says how long the zip is. Nothing is hidden: a program without that
// trailer simply has no content inside it, and the content is only ever put
// there on purpose, by naming the folder to put in.
package bundle

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// magic marks the end of a program that carries content.
var magic = [16]byte{'B', 'A', 'U', 'M', 'K', 'U', 'C', 'H', 'E', 'N', 'B', 'U', 'N', 'D', 'L', '1'}

// trailerSize is the length of the mark at the end: the magic, the length of
// the zip, and the length of the whole file.
const trailerSize = 32

// Root is the folder inside the zip where the content sits.
const Root = "content"

// Write copies the program at exe to out, with the content in dir inside it,
// and gives the number of files it put in.
//
// The zip is written the same way every time: the files in order, with the
// same times on them, so the same program and the same content give the same
// file twice.
func Write(exe, out, dir string) (int, error) {
	return WriteWith(exe, out, dir, nil)
}

// WriteWith copies the program at exe to out, with the content in dir inside
// it, and the named files beside it.
//
// The content the program already carries is taken away first: a bundle of a
// bundle would carry two of everything, and only the last one would be read.
func WriteWith(exe, out, dir string, extra map[string][]byte) (int, error) {
	program, err := os.ReadFile(exe)
	if err != nil {
		return 0, fmt.Errorf("read the program: %w", err)
	}
	program = StripBytes(program)
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	n, err := addContent(zw, dir, extra)
	if err != nil {
		return 0, err
	}
	for _, name := range sortedNames(extra) {
		head := &zip.FileHeader{Name: name, Method: zip.Deflate}
		head.SetMode(0o644)
		w, err := zw.CreateHeader(head)
		if err != nil {
			return 0, err
		}
		if _, err := w.Write(extra[name]); err != nil {
			return 0, err
		}
		n++
	}
	if err := zw.Close(); err != nil {
		return 0, err
	}

	whole := make([]byte, 0, len(program)+buf.Len()+trailerSize)
	whole = append(whole, program...)
	whole = append(whole, buf.Bytes()...)
	whole = append(whole, trailer(buf.Len(), len(whole)+trailerSize)...)

	// the file is written beside the one it replaces and moved into place, so
	// a copy that goes wrong never leaves half a program behind
	tmp, err := os.CreateTemp(filepath.Dir(out), ".bundle-*")
	if err != nil {
		return 0, fmt.Errorf("write %s: %w", out, err)
	}
	name := tmp.Name()
	defer func() { _ = os.Remove(name) }()
	if _, err := tmp.Write(whole); err != nil {
		_ = tmp.Close()
		return 0, fmt.Errorf("write %s: %w", out, err)
	}
	if err := tmp.Close(); err != nil {
		return 0, fmt.Errorf("write %s: %w", out, err)
	}
	if err := os.Chmod(name, 0o755); err != nil {
		return 0, fmt.Errorf("write %s: %w", out, err)
	}
	if err := os.Rename(name, out); err != nil {
		return 0, fmt.Errorf("write %s: %w", out, err)
	}
	return n, nil
}

// sortedNames gives the names of the extra files in order, so the zip that
// comes out is the same every time.
func sortedNames(extra map[string][]byte) []string {
	names := make([]string, 0, len(extra))
	for name := range extra {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Strip gives the program at path with the content it carries taken away.
//
// A program that carries nothing comes back as it is.
func Strip(path string) ([]byte, error) {
	whole, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return StripBytes(whole), nil
}

// StripBytes gives the program bytes with the content inside them taken away.
func StripBytes(whole []byte) []byte {
	if len(whole) < trailerSize {
		return whole
	}
	mark := whole[len(whole)-trailerSize:]
	if !bytes.Equal(mark[:len(magic)], magic[:]) {
		return whole
	}
	zipLen := binary.LittleEndian.Uint64(mark[len(magic) : len(magic)+8])
	total := binary.LittleEndian.Uint64(mark[len(magic)+8:])
	if total != uint64(len(whole)) {
		return whole
	}
	start := int64(len(whole)) - trailerSize - int64(zipLen)
	if start < 0 {
		return whole
	}
	return whole[:start]
}

// addContent puts every file of the folder into the zip.
//
// A file that is also given beside the content is left out: the one given
// beside it is the one that is wanted, and two files of one name in a zip is
// a thing to read twice.
func addContent(zw *zip.Writer, dir string, extra map[string][]byte) (int, error) {
	files, err := walk(dir)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, file := range files {
		rel, err := filepath.Rel(dir, file)
		if err != nil {
			return n, err
		}
		name := Root + "/" + filepath.ToSlash(rel)
		if _, taken := extra[name]; taken {
			continue
		}
		data, err := os.ReadFile(file)
		if err != nil {
			return n, fmt.Errorf("read %s: %w", file, err)
		}
		head := &zip.FileHeader{Name: name, Method: zip.Deflate}
		head.SetMode(0o644)
		w, err := zw.CreateHeader(head)
		if err != nil {
			return n, err
		}
		if _, err := w.Write(data); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}

// walk gives every file in the folder, in order, with the hidden ones left
// out: they are not part of the site.
func walk(dir string) ([]string, error) {
	var out []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		name := filepath.Base(path)
		if info.IsDir() {
			if path != dir && strings.HasPrefix(name, ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(name, ".") || !info.Mode().IsRegular() {
			return nil
		}
		out = append(out, path)
		return nil
	})
	sort.Strings(out)
	return out, err
}

// trailer gives the mark that says where the zip is.
func trailer(zipLen, total int) []byte {
	out := make([]byte, 0, trailerSize)
	out = append(out, magic[:]...)
	length := make([]byte, 8)
	binary.LittleEndian.PutUint64(length, uint64(zipLen))
	out = append(out, length...)
	binary.LittleEndian.PutUint64(length, uint64(total))
	out = append(out, length...)
	return out
}

// Open gives the content inside the program at path. The second answer says
// whether there was any: a program without content answers no, and that is
// not a fault.
func Open(path string) (fs.FS, bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, false, err
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil {
		return nil, false, err
	}
	size := info.Size()
	if size < trailerSize {
		return nil, false, nil
	}
	mark := make([]byte, trailerSize)
	if _, err := f.ReadAt(mark, size-trailerSize); err != nil {
		return nil, false, nil
	}
	if !bytes.Equal(mark[:len(magic)], magic[:]) {
		return nil, false, nil
	}
	zipLen := binary.LittleEndian.Uint64(mark[len(magic) : len(magic)+8])
	total := binary.LittleEndian.Uint64(mark[len(magic)+8:])
	if total != uint64(size) {
		return nil, false, fmt.Errorf("%s: the bundle mark does not match the file", path)
	}
	start := int64(size) - trailerSize - int64(zipLen)
	if start < 0 {
		return nil, false, fmt.Errorf("%s: the bundle mark points outside the file", path)
	}
	// the zip is held in memory, so the file can be closed and the content
	// stays readable for as long as the site needs it
	data := make([]byte, zipLen)
	if _, err := f.ReadAt(data, start); err != nil {
		return nil, false, fmt.Errorf("%s: read the content inside: %w", path, err)
	}
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, false, fmt.Errorf("%s: read the content inside: %w", path, err)
	}
	return reader, true, nil
}
