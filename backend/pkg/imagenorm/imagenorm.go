// Package imagenorm turns an uploaded JPEG or PNG into square JPEGs of fixed
// sizes: the centre square of the picture, the right way up, on white where it
// was transparent. Nothing of the original file but its pixels survives, so no
// EXIF and no location. It is pure: no storage, no limits on who may call it.
package imagenorm

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png" // registers the PNG decoder with image.Decode
)

// DecodeBudget is how much memory decoding one picture may take. A service
// with 192 MB leaves room for everything else it does while a picture is
// decoded. It admits a 12 MP phone photo and refuses the small file that claims
// to be 10000×10000.
const DecodeBudget = 48 << 20

var (
	// ErrNotImage means the bytes are not a JPEG or PNG picture, whatever the
	// file's name or declared type says.
	ErrNotImage = errors.New("the file is not a JPEG or PNG picture")
	// ErrTooManyPixels means the picture is too large to decode safely.
	ErrTooManyPixels = errors.New("the picture has too many pixels")
	// ErrBadSizes means the requested sizes are not a non-empty, strictly
	// descending list of positive numbers.
	ErrBadSizes = errors.New("sizes must be positive and strictly descending")
)

// Square returns the centre square of the picture b, once per entry of sizes,
// as JPEG at the given quality. sizes must be strictly descending: the first is
// made from the source and each later one from the one before it, which keeps
// the memory of a large source to the first step.
func Square(b []byte, sizes []int, quality int) ([][]byte, error) {
	if len(sizes) == 0 {
		return nil, ErrBadSizes
	}
	for i, s := range sizes {
		if s <= 0 || (i > 0 && s >= sizes[i-1]) {
			return nil, ErrBadSizes
		}
	}
	if err := checkDecodeBudget(b); err != nil {
		return nil, err
	}
	src, _, err := image.Decode(bytes.NewReader(b))
	if err != nil {
		return nil, ErrNotImage
	}

	// The centre square is the same square whichever way the picture is
	// meant to be turned, so the turn is applied to the small result instead
	// of to a full-size copy.
	r := src.Bounds()
	side := min(r.Dx(), r.Dy())
	x0, y0 := r.Min.X+(r.Dx()-side)/2, r.Min.Y+(r.Dy()-side)/2
	square := image.Rectangle{Min: image.Pt(x0, y0), Max: image.Pt(x0+side, y0+side)}

	imgs := make([]*image.RGBA, len(sizes))
	imgs[0] = orient(onWhite(halveTo(src, square, sizes[0]), sizes[0]), jpegOrientation(b))
	for i := 1; i < len(sizes); i++ {
		prev := imgs[i-1]
		imgs[i] = halveTo(prev, prev.Bounds(), sizes[i])
	}

	out := make([][]byte, len(imgs))
	for i, img := range imgs {
		var buf bytes.Buffer
		if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality}); err != nil {
			return nil, fmt.Errorf("encode picture: %w", err)
		}
		out[i] = buf.Bytes()
	}
	return out, nil
}
