package thumbnail

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/gif" // registers the GIF decoder with image.Decode
	"image/jpeg"
	_ "image/png" // registers the PNG decoder with image.Decode
	"ngac-platform/pkg/imagenorm"
	"os"

	xdraw "golang.org/x/image/draw"
	_ "golang.org/x/image/webp" // registers the WebP decoder with image.Decode
)

const (
	// decodeBudget bounds the memory one picture may take to decode, estimated
	// from its header (pixels, a progressive JPEG's coefficients, the first
	// working copy). The drive runs in 512 MB with a soft limit far below that,
	// and at most maxImageDecodes decode at once, so 2 x (64 MiB + the 25 MiB
	// source held in memory) stays well under the container. A 24 MP baseline
	// photo fits; a progressive one of 12 MP or more does not and gets an icon.
	decodeBudget = 64 << 20
	// maxImageDecodes is how many pictures decode at the same time, however many
	// thumbnails are being drawn (PDF and video work is not held up by it).
	maxImageDecodes = 2
	jpegQuality     = 80
)

// imageSlots limits concurrent in-process decodes, apart from the service-wide
// thumbnail slots that also cover the external tools.
var imageSlots = make(chan struct{}, maxImageDecodes)

// checkBudget refuses, from the header alone, a picture whose decode would take
// more than decodeBudget. GIF and WebP are estimated at 5 bytes a pixel (the
// pixels and a working copy); JPEG and PNG by the estimate the avatar path uses.
func checkBudget(b []byte) error {
	cfg, format, err := image.DecodeConfig(bytes.NewReader(b))
	if err != nil || cfg.Width <= 0 || cfg.Height <= 0 {
		return ErrNotImage
	}
	switch format {
	case "jpeg", "png":
		switch err := imagenorm.CheckDecodeBudget(b, decodeBudget); {
		case errors.Is(err, imagenorm.ErrTooManyPixels):
			return ErrTooManyPixels
		case err != nil:
			return ErrNotImage
		}
	default:
		if 5*int64(cfg.Width)*int64(cfg.Height) > decodeBudget {
			return ErrTooManyPixels
		}
	}
	return nil
}

// renderImage scales the picture in srcPath so its longest side is size,
// keeping its proportions, the right way up (EXIF orientation applied), on
// white where it was transparent. Nothing of the original but its pixels
// survives: no EXIF, no location.
func renderImage(ctx context.Context, srcPath string, size int) ([]byte, error) {
	b, err := os.ReadFile(srcPath)
	if err != nil {
		return nil, fmt.Errorf("read picture: %w", err)
	}
	if err := checkBudget(b); err != nil {
		return nil, err
	}
	select {
	case imageSlots <- struct{}{}:
		defer func() { <-imageSlots }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	src, _, err := image.Decode(bytes.NewReader(b))
	if err != nil {
		return nil, ErrNotImage
	}

	r := src.Bounds()
	w, h := fit(r.Dx(), r.Dy(), size)
	small := orient(onWhite(halveTo(src, r, w, h)), imagenorm.JPEGOrientation(b))

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, small, &jpeg.Options{Quality: jpegQuality}); err != nil {
		return nil, fmt.Errorf("encode thumbnail: %w", err)
	}
	return buf.Bytes(), nil
}

// fit is the size of a w x h picture whose longest side is at most size.
func fit(w, h, size int) (int, int) {
	long := max(w, h)
	if long <= size {
		return w, h
	}
	return max(1, w*size/long), max(1, h*size/long)
}

// halveTo scales the part r of src to w x h. A large picture is halved step by
// step first: each halving with ApproxBiLinear is a 2x2 average, so quality
// holds without the scratch memory the kernel scalers need.
func halveTo(src image.Image, r image.Rectangle, w, h int) *image.RGBA {
	for r.Dx() > 2*w && r.Dy() > 2*h {
		half := image.NewRGBA(image.Rect(0, 0, r.Dx()/2, r.Dy()/2))
		xdraw.ApproxBiLinear.Scale(half, half.Bounds(), src, r, draw.Src, nil)
		src, r = half, half.Bounds()
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	xdraw.ApproxBiLinear.Scale(dst, dst.Bounds(), src, r, draw.Src, nil)
	return dst
}

// onWhite lays img over white, so a transparent picture does not turn black
// once it is a JPEG.
func onWhite(img *image.RGBA) *image.RGBA {
	dst := image.NewRGBA(img.Bounds())
	draw.Draw(dst, dst.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	draw.Draw(dst, dst.Bounds(), img, image.Point{}, draw.Over)
	return dst
}

// orient turns img the way its EXIF Orientation (1-8) says it is meant to be
// seen. Orientations 5-8 swap width and height.
func orient(img *image.RGBA, o int) *image.RGBA {
	if o < 2 || o > 8 {
		return img
	}
	W, H := img.Bounds().Dx(), img.Bounds().Dy()
	dw, dh := W, H
	if o >= 5 {
		dw, dh = H, W
	}
	dst := image.NewRGBA(image.Rect(0, 0, dw, dh))
	for y := 0; y < dh; y++ {
		for x := 0; x < dw; x++ {
			var sx, sy int
			switch o {
			case 2: // mirrored
				sx, sy = W-1-x, y
			case 3: // half a turn
				sx, sy = W-1-x, H-1-y
			case 4: // mirrored upside down
				sx, sy = x, H-1-y
			case 5: // mirrored, a quarter turn counter-clockwise
				sx, sy = y, x
			case 6: // a quarter turn clockwise
				sx, sy = y, H-1-x
			case 7: // mirrored, a quarter turn clockwise
				sx, sy = W-1-y, H-1-x
			case 8: // a quarter turn counter-clockwise
				sx, sy = W-1-y, x
			}
			dst.SetRGBA(x, y, img.RGBAAt(sx, sy))
		}
	}
	return dst
}
