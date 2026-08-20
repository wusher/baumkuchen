package blog

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// pattern draws a picture with detail in it, so that a smaller file is a real
// saving and not just an empty field of one colour.
func pattern(w, h int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{
				R: uint8((x*7 + y*3) % 256),
				G: uint8((x*x + y) % 256),
				B: uint8((x ^ y) % 256),
				A: 255,
			})
		}
	}
	return img
}

func jpegFile(t *testing.T, dir, name string, w, h, quality int) string {
	t.Helper()
	var b bytes.Buffer
	if err := jpeg.Encode(&b, pattern(w, h), &jpeg.Options{Quality: quality}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, b.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// smooth draws the kind of picture a PNG normally holds: a drawing with flat
// runs of colour, not the grain of a photograph.
func smooth(w, h int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{
				R: uint8(x * 255 / w),
				G: uint8(y * 255 / h),
				B: 128,
				A: 255,
			})
		}
	}
	return img
}

func pngFile(t *testing.T, dir, name string, w, h int) string {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, smooth(w, h)); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, b.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// size gives the length of a file, and stops the test if it is not there.
func size(t *testing.T, path string) int64 {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info.Size()
}

// bounds gives the width and the height of a picture on disk.
func bounds(t *testing.T, path string) (int, int) {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		t.Fatalf("%s is not a picture: %v", filepath.Base(path), err)
	}
	return cfg.Width, cfg.Height
}

// A wide picture comes out of the build no wider than the limit, and smaller.
func TestOptimizeShrinksAWidePicture(t *testing.T) {
	// arrange
	from := jpegFile(t, t.TempDir(), "wide.jpg", 3000, 2000, 95)
	to := filepath.Join(t.TempDir(), "wide.jpg")

	// act
	src, dst, err := optimizeImage(from, to)

	// assert
	if err != nil {
		t.Fatal(err)
	}
	w, h := bounds(t, to)
	if w != maxImageWidth {
		t.Errorf("the picture is %d px wide, want %d", w, maxImageWidth)
	}
	if want := 2000 * maxImageWidth / 3000; h != want {
		t.Errorf("the height is %d px, want %d: the shape changed", h, want)
	}
	if dst >= src {
		t.Errorf("the build wrote %d bytes from %d: it saved nothing", dst, src)
	}
	if got := size(t, to); got != dst {
		t.Errorf("it reports %d bytes but wrote %d", dst, got)
	}
}

// A file that is not a picture is copied byte for byte.
func TestOptimizeLeavesANonPictureAlone(t *testing.T) {
	// arrange
	dir := t.TempDir()
	want := "{\"credit\": \"someone\"}\n"
	from := write(t, dir, "CREDITS.json", want)
	to := filepath.Join(t.TempDir(), "CREDITS.json")

	// act
	if _, _, err := optimizeImage(from, to); err != nil {
		t.Fatal(err)
	}

	// assert
	if got := readFile(t, to); got != want {
		t.Errorf("the file changed: got %q, want %q", got, want)
	}
}

// A picture that is already small must never come out larger than it went in.
func TestOptimizeNeverGrowsAFile(t *testing.T) {
	// arrange
	from := jpegFile(t, t.TempDir(), "small.jpg", 200, 200, 15)
	to := filepath.Join(t.TempDir(), "small.jpg")

	// act
	src, dst, err := optimizeImage(from, to)

	// assert
	if err != nil {
		t.Fatal(err)
	}
	if dst > src {
		t.Errorf("the build grew the file, from %d bytes to %d", src, dst)
	}
	before, err := os.ReadFile(from)
	if err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(to)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Error("a picture it cannot improve must be copied as it is")
	}
}

// A PNG stays a PNG, and is scaled the same way.
func TestOptimizeKeepsAPNGAPNG(t *testing.T) {
	// arrange
	from := pngFile(t, t.TempDir(), "wide.png", 2400, 1200)
	to := filepath.Join(t.TempDir(), "wide.png")

	// act
	src, dst, err := optimizeImage(from, to)

	// assert
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(to)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	_, format, err := image.DecodeConfig(f)
	if err != nil {
		t.Fatal(err)
	}
	if format != "png" {
		t.Errorf("the picture came out as %s, want png: the link in the post would break", format)
	}
	if w, _ := bounds(t, to); w != maxImageWidth {
		t.Errorf("the picture is %d px wide, want %d", w, maxImageWidth)
	}
	if dst >= src {
		t.Errorf("the build wrote %d bytes from %d: it saved nothing", dst, src)
	}
}

// A picture that is not there stops the build, with a word about which one.
func TestOptimizeRefusesAMissingSource(t *testing.T) {
	// arrange
	from := filepath.Join(t.TempDir(), "no-such-file.jpg")
	to := filepath.Join(t.TempDir(), "copy.jpg")

	// act
	_, _, err := optimizeImage(from, to)

	// assert
	if err == nil {
		t.Fatal("a picture that is not there must give an error")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("no-such-file")) {
		t.Errorf("the error does not name the file: %v", err)
	}
}

// The build puts the smaller picture in the media folder, not the original.
func TestExportOptimizesTheMedia(t *testing.T) {
	// arrange
	dir := t.TempDir()
	postFile(t, dir, "one.md", "---\ndate: 2026-01-01\n---\n", "One")
	images := filepath.Join(dir, "images")
	if err := os.MkdirAll(images, 0o755); err != nil {
		t.Fatal(err)
	}
	from := jpegFile(t, images, "big.jpg", 3000, 2000, 95)
	site := testSite(t, dir)
	out := t.TempDir()

	// act
	if _, err := site.Export(out); err != nil {
		t.Fatal(err)
	}

	// assert
	built := filepath.Join(out, "media", "big.jpg")
	if size(t, built) >= size(t, from) {
		t.Error("the build copied the picture without making it smaller")
	}
	if w, _ := bounds(t, built); w != maxImageWidth {
		t.Errorf("the built picture is %d px wide, want %d", w, maxImageWidth)
	}
	if size(t, from) == 0 {
		t.Error("the build changed the source picture; it must only write to the build folder")
	}
}
