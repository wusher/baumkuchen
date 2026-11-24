package blog

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/wusher/baumkuchen/internal/content"
)

const defaultFavicon = "favicon.svg"

type favicon struct {
	name      string
	mediaType string
	data      []byte
}

// FaviconName gives the name the picture on the tab has.
//
// The name is always favicon.svg, favicon.png or favicon.ico: the tab shows
// one picture at one address, whatever the file was called.
func FaviconName(file string) (string, error) {
	switch strings.ToLower(filepath.Ext(file)) {
	case ".svg":
		return "favicon.svg", nil
	case ".png":
		return "favicon.png", nil
	case ".ico":
		return "favicon.ico", nil
	}
	return "", fmt.Errorf("favicon %q: use an SVG, PNG, or ICO file", file)
}

// faviconType gives the kind of picture a name promises.
func faviconType(ext string) string {
	switch ext {
	case ".png":
		return "image/png"
	case ".ico":
		return "image/x-icon"
	}
	return "image/svg+xml"
}

// loadFavicon reads the picture on the tab.
//
// The picture is named by the settings. A name that is not there names a
// file on the disk of the machine the program runs on. A program that
// carries its settings carries the picture with it, and the name is read
// inside the program.
//
// With no name at all, the picture in the writing is used: favicon.svg
// beside the posts, if it is there.
func loadFavicon(src content.Source, file string, inside bool) (favicon, error) {
	if file == "" {
		data, err := src.Read(defaultFavicon)
		if errors.Is(err, fs.ErrNotExist) {
			return favicon{}, nil
		}
		if err != nil {
			return favicon{}, fmt.Errorf("read %s from %s: %w", defaultFavicon, src.Label(), err)
		}
		if !looksLike(".svg", data) {
			return favicon{}, fmt.Errorf("%s in %s does not read as an SVG", defaultFavicon, src.Label())
		}
		return favicon{name: defaultFavicon, mediaType: "image/svg+xml", data: data}, nil
	}

	name, err := FaviconName(file)
	if err != nil {
		return favicon{}, err
	}
	ext := filepath.Ext(name)
	var data []byte
	if inside {
		// the picture travels inside the program, and is read from there
		data, err = src.Read(file)
		if err != nil {
			return favicon{}, fmt.Errorf("read the favicon %s from inside the program: %w", file, err)
		}
	} else {
		data, err = os.ReadFile(file)
		if err != nil {
			return favicon{}, fmt.Errorf("read favicon: %w", err)
		}
	}
	// The bytes must be the picture the name promises. The favicon is served
	// and exported as it is, so a settings file that named some other kind of
	// file would otherwise publish that file's contents under a picture's name.
	if !looksLike(ext, data) {
		return favicon{}, fmt.Errorf("favicon %q does not read as a %s file", file, strings.TrimPrefix(ext, "."))
	}
	return favicon{name: name, mediaType: faviconType(ext), data: data}, nil
}

// looksLike says whether the bytes begin the way the format begins.
func looksLike(ext string, data []byte) bool {
	switch ext {
	case ".png":
		return bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n"))
	case ".ico":
		return bytes.HasPrefix(data, []byte{0, 0, 1, 0})
	case ".svg":
		head := data
		if len(head) > 512 {
			head = head[:512]
		}
		return bytes.Contains(bytes.ToLower(head), []byte("<svg"))
	}
	return false
}

func (f favicon) serve(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", f.mediaType)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "sandbox; default-src 'none'; style-src 'unsafe-inline'")
	http.ServeContent(w, r, f.name, time.Time{}, bytes.NewReader(f.data))
}
