package imagenorm

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/draw"

	xdraw "golang.org/x/image/draw"
)

// halveTo scales the part r of src to a size×size picture. A large picture is
// halved step by step first: each halving with ApproxBiLinear is a 2×2 average,
// so quality holds, and unlike the kernel scalers (CatmullRom and friends,
// which keep a float64 buffer of width × source height) it needs no scratch
// memory beyond the result.
func halveTo(src image.Image, r image.Rectangle, size int) *image.RGBA {
	for r.Dx() > 2*size {
		half := image.NewRGBA(image.Rect(0, 0, r.Dx()/2, r.Dy()/2))
		xdraw.ApproxBiLinear.Scale(half, half.Bounds(), src, r, draw.Src, nil)
		src, r = half, half.Bounds()
	}
	dst := image.NewRGBA(image.Rect(0, 0, size, size))
	xdraw.ApproxBiLinear.Scale(dst, dst.Bounds(), src, r, draw.Src, nil)
	return dst
}

// onWhite lays img over white, so a transparent picture does not turn black
// once it is a JPEG.
func onWhite(img *image.RGBA, size int) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, size, size))
	draw.Draw(dst, dst.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
	draw.Draw(dst, dst.Bounds(), img, image.Point{}, draw.Over)
	return dst
}

// orient turns a square picture the way its EXIF Orientation (1–8) says it is
// meant to be seen.
func orient(img *image.RGBA, o int) *image.RGBA {
	if o < 2 || o > 8 {
		return img
	}
	n := img.Bounds().Dx()
	dst := image.NewRGBA(img.Bounds())
	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			var sx, sy int
			switch o {
			case 2: // mirrored
				sx, sy = n-1-x, y
			case 3: // half a turn
				sx, sy = n-1-x, n-1-y
			case 4: // mirrored upside down
				sx, sy = x, n-1-y
			case 5: // mirrored, a quarter turn counter-clockwise
				sx, sy = y, x
			case 6: // a quarter turn clockwise
				sx, sy = y, n-1-x
			case 7: // mirrored, a quarter turn clockwise
				sx, sy = n-1-y, n-1-x
			case 8: // a quarter turn counter-clockwise
				sx, sy = n-1-y, x
			}
			dst.SetRGBA(x, y, img.RGBAAt(sx, sy))
		}
	}
	return dst
}

// checkDecodeBudget refuses, from the header alone, a picture whose decode
// would take more than DecodeBudget. A file that is not a JPEG or PNG
// is ErrNotImage.
func checkDecodeBudget(b []byte) error { return CheckDecodeBudget(b, DecodeBudget) }

// CheckDecodeBudget is checkDecodeBudget with the caller's own budget in bytes,
// for services whose memory and concurrency differ from the avatar path's. It
// estimates a decode from the header (decoded pixels, a progressive JPEG's
// coefficients, the first working copy), so a small file that claims a huge
// picture is refused before anything is allocated.
func CheckDecodeBudget(b []byte, budget int64) error {
	n, err := decodedBytes(b)
	if err != nil {
		return err
	}
	if n > budget {
		return ErrTooManyPixels
	}
	return nil
}

// decodedBytes estimates the memory a decode of b takes: the decoded pixels,
// a progressive JPEG's coefficients, and the first quarter-size working copy
// of the centre square.
func decodedBytes(b []byte) (int64, error) {
	cfg, format, err := image.DecodeConfig(bytes.NewReader(b))
	if err != nil || (format != "jpeg" && format != "png") || cfg.Width <= 0 || cfg.Height <= 0 {
		return 0, ErrNotImage
	}
	w, h := int64(cfg.Width), int64(cfg.Height)
	side := min(w, h)
	working := side * side // (side/2)² RGBA

	if format == "jpeg" {
		samples, progressive, ok := jpegSamples(b)
		if !ok {
			// A frame the decoder reads but this parser cannot: assume the
			// worst, full-size planes decoded progressively.
			samples, progressive = w*h*3, true
		}
		decoded := samples
		if progressive {
			decoded += samples * 4 // one int32 coefficient per sample
		}
		return decoded + working, nil
	}

	var bpp int64
	switch cfg.ColorModel {
	case color.GrayModel:
		bpp = 1
	case color.Gray16Model:
		bpp = 2
	case color.RGBA64Model, color.NRGBA64Model:
		bpp = 8
	default:
		if _, paletted := cfg.ColorModel.(color.Palette); paletted {
			bpp = 1
		} else {
			bpp = 4
		}
	}
	return w*h*bpp + working, nil
}

// jpegSamples reads a JPEG's frame header: how many samples its components
// hold in all (Cb and Cr are usually a quarter of Y) and whether it is
// progressive.
func jpegSamples(b []byte) (samples int64, progressive, ok bool) {
	for i := 2; i+1 < len(b); {
		if b[i] != 0xFF {
			return 0, false, false
		}
		// Any number of 0xFF fill bytes may come before a marker. Losing
		// count of them here would read the wrong frame, or none.
		for i+1 < len(b) && b[i+1] == 0xFF {
			i++
		}
		if i+4 > len(b) {
			return 0, false, false
		}
		marker := b[i+1]
		length := int(binary.BigEndian.Uint16(b[i+2:]))
		seg := i + 4
		if length < 2 || seg+length-2 > len(b) {
			return 0, false, false
		}
		switch marker {
		case 0xC0, 0xC1, 0xC2:
			if length < 8 {
				return 0, false, false
			}
			h := int64(binary.BigEndian.Uint16(b[seg+1:]))
			w := int64(binary.BigEndian.Uint16(b[seg+3:]))
			n := int(b[seg+5])
			if n == 0 || length < 8+3*n {
				return 0, false, false
			}
			var maxH, maxV int64 = 1, 1
			for c := 0; c < n; c++ {
				hv := b[seg+6+3*c+1]
				if hv>>4 == 0 || hv&0x0F == 0 {
					return 0, false, false
				}
				maxH, maxV = max(maxH, int64(hv>>4)), max(maxV, int64(hv&0x0F))
			}
			for c := 0; c < n; c++ {
				hv := b[seg+6+3*c+1]
				samples += w * h * int64(hv>>4) * int64(hv&0x0F) / (maxH * maxV)
			}
			return samples, marker == 0xC2, true
		case 0xDA: // start of scan: no frame header before it
			return 0, false, false
		}
		i = seg + length - 2
	}
	return 0, false, false
}

// jpegOrientation reads the EXIF Orientation tag of a JPEG. Anything missing
// or malformed reads as 1, upright.
func jpegOrientation(b []byte) int {
	if len(b) < 4 || b[0] != 0xFF || b[1] != 0xD8 {
		return 1
	}
	for i := 2; i+1 < len(b); {
		if b[i] != 0xFF {
			return 1
		}
		for i+1 < len(b) && b[i+1] == 0xFF { // fill bytes before a marker
			i++
		}
		if i+4 > len(b) {
			return 1
		}
		marker := b[i+1]
		length := int(binary.BigEndian.Uint16(b[i+2:]))
		seg, end := i+4, i+2+length
		if length < 2 || end > len(b) || marker == 0xDA {
			return 1
		}
		if marker == 0xE1 && end-seg >= 14 && string(b[seg:seg+6]) == "Exif\x00\x00" {
			return exifOrientation(b[seg+6 : end])
		}
		i = end
	}
	return 1
}

// exifOrientation finds tag 0x0112 in the first IFD of a TIFF block.
func exifOrientation(t []byte) int {
	if len(t) < 8 {
		return 1
	}
	var order binary.ByteOrder
	switch string(t[:2]) {
	case "II":
		order = binary.LittleEndian
	case "MM":
		order = binary.BigEndian
	default:
		return 1
	}
	ifd := int(order.Uint32(t[4:]))
	if ifd < 8 || ifd+2 > len(t) {
		return 1
	}
	count := int(order.Uint16(t[ifd:]))
	for e := 0; e < count; e++ {
		off := ifd + 2 + 12*e
		if off+12 > len(t) {
			return 1
		}
		if order.Uint16(t[off:]) == 0x0112 {
			if o := int(order.Uint16(t[off+8:])); o >= 1 && o <= 8 {
				return o
			}
			return 1
		}
	}
	return 1
}

// JPEGOrientation reports the EXIF Orientation (1-8) of a JPEG; anything missing
// or malformed reads as 1, upright. Shared so other image code turns pictures
// the same way the normaliser does.
func JPEGOrientation(b []byte) int { return jpegOrientation(b) }
