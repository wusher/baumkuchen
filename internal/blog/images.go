// The pictures. A writer puts a full-size photograph beside the posts, and the
// server shows it as it is. The build is the only step that makes it smaller,
// so nothing a writer does can lose the original.
package blog

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"

	"golang.org/x/image/draw"
)

const (
	// maxImageWidth is as wide as a picture needs to be, full screen, on a
	// large display. Anything wider is only weight.
	maxImageWidth = 1600
	// jpegQuality keeps the grain of a photograph at about a third of the
	// bytes. Above this the file grows faster than the picture improves.
	jpegQuality = 82
)

// optimizeImage writes a copy of from at to, made smaller where it can be, and
// gives the size of the source and the size of what it wrote.
//
// It never makes a file worse. A file it cannot read as a picture, a format it
// does not encode, and a picture whose new form is no smaller are all copied
// byte for byte. Only the build folder is written; the source is never touched.
func optimizeImage(from, to string) (int64, int64, error) {
	data, err := os.ReadFile(from)
	if err != nil {
		return 0, 0, fmt.Errorf("read %s: %w", filepath.Base(from), err)
	}
	src, dst, err := optimizeImageBytes(data, to)
	if err != nil {
		return 0, 0, err
	}
	return src, dst, nil
}

// optimizeImageBytes writes the bytes of a picture at to, made smaller where
// it can be, and gives the size of what it was given and the size of what it
// wrote.
//
// It never makes a picture worse. One it cannot read, a format it does not
// encode, and a picture whose new form is no smaller are all written as they
// came.
func optimizeImageBytes(data []byte, to string) (int64, int64, error) {
	src := int64(len(data))
	small, err := shrinkBytes(data)
	if err != nil || int64(len(small)) >= src {
		if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
			return src, 0, err
		}
		if err := os.WriteFile(to, data, 0o644); err != nil {
			return src, 0, err
		}
		return src, src, nil
	}
	if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
		return src, 0, err
	}
	if err := os.WriteFile(to, small, 0o644); err != nil {
		return src, 0, err
	}
	return src, int64(len(small)), nil
}

// shrinkBytes reads a picture from its bytes, scales it down to the limit, and
// encodes it again in the format it came in.
func shrinkBytes(data []byte) ([]byte, error) {
	img, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	img = scaleDown(img, maxImageWidth)

	var out bytes.Buffer
	switch format {
	case "jpeg":
		err = jpeg.Encode(&out, img, &jpeg.Options{Quality: jpegQuality})
	case "png":
		enc := png.Encoder{CompressionLevel: png.BestCompression}
		err = enc.Encode(&out, img)
	default:
		return nil, fmt.Errorf("a picture in %s is a format the build leaves alone", format)
	}
	if err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// scaleDown gives a picture no wider than max, and keeps its shape. A picture
// that is already narrow enough comes back untouched.
func scaleDown(img image.Image, max int) image.Image {
	b := img.Bounds()
	if b.Dx() <= max {
		return img
	}
	h := b.Dy() * max / b.Dx()
	if h < 1 {
		h = 1
	}
	dst := image.NewRGBA(image.Rect(0, 0, max, h))
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, b, draw.Src, nil)
	return dst
}
