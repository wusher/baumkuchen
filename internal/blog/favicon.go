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

func loadFavicon(src content.Source, file string) (favicon, error) {
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

	ext := strings.ToLower(filepath.Ext(file))
	var mediaType string
	switch ext {
	case ".svg":
		mediaType = "image/svg+xml"
	case ".png":
		mediaType = "image/png"
	case ".ico":
		mediaType = "image/x-icon"
	default:
		return favicon{}, fmt.Errorf("favicon %q: use an SVG, PNG, or ICO file", file)
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return favicon{}, fmt.Errorf("read favicon: %w", err)
	}
	// The bytes must be the picture the name promises. The favicon is served
	// and exported as it is, so a settings file that named some other kind of
	// file would otherwise publish that file's contents under a picture's name.
	if !looksLike(ext, data) {
		return favicon{}, fmt.Errorf("favicon %q does not read as a %s file", file, strings.TrimPrefix(ext, "."))
	}
	return favicon{name: "favicon" + ext, mediaType: mediaType, data: data}, nil
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
