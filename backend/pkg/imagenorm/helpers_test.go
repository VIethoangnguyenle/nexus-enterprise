package imagenorm

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"
)

// quadrants is a w×h picture whose top-left quarter is red, top-right quarter
// green and the rest blue, so a test can tell both turns and mirrors apart.
func quadrants(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := color.RGBA{0, 0, 255, 255}
			switch {
			case x < w/2 && y < h/2:
				c = color.RGBA{255, 0, 0, 255}
			case x >= w/2 && y < h/2:
				c = color.RGBA{0, 255, 0, 255}
			}
			img.SetRGBA(x, y, c)
		}
	}
	return img
}

func encodePNG(t *testing.T, img image.Image) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func encodeJPEG(t *testing.T, img image.Image) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := jpeg.Encode(&b, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// withOrientation puts an EXIF APP1 segment carrying the Orientation tag right
// after the JPEG's start-of-image marker, the way a phone camera writes it.
func withOrientation(jpg []byte, orientation uint16) []byte {
	var tiff bytes.Buffer
	tiff.WriteString("MM\x00\x2a")
	binary.Write(&tiff, binary.BigEndian, uint32(8))      // IFD0 offset
	binary.Write(&tiff, binary.BigEndian, uint16(1))      // one entry
	binary.Write(&tiff, binary.BigEndian, uint16(0x0112)) // Orientation
	binary.Write(&tiff, binary.BigEndian, uint16(3))      // SHORT
	binary.Write(&tiff, binary.BigEndian, uint32(1))      // count
	binary.Write(&tiff, binary.BigEndian, orientation)    // value
	binary.Write(&tiff, binary.BigEndian, uint16(0))      // padding
	binary.Write(&tiff, binary.BigEndian, uint32(0))      // no next IFD
	payload := append([]byte("Exif\x00\x00"), tiff.Bytes()...)
	seg := []byte{0xFF, 0xE1, byte((len(payload) + 2) >> 8), byte(len(payload) + 2)}
	seg = append(seg, payload...)
	out := append([]byte{}, jpg[:2]...)
	out = append(out, seg...)
	return append(out, jpg[2:]...)
}

func decodeJPEG(t *testing.T, b []byte) image.Image {
	t.Helper()
	img, format, err := image.Decode(bytes.NewReader(b))
	if err != nil || format != "jpeg" {
		t.Fatalf("output is not a JPEG: %v %q", err, format)
	}
	return img
}

func isRed(c color.Color) bool {
	r, g, b, _ := c.RGBA()
	return r > 0xA000 && g < 0x6000 && b < 0x6000
}

func isGreen(c color.Color) bool {
	r, g, b, _ := c.RGBA()
	return g > 0xA000 && r < 0x6000 && b < 0x6000
}

// pngHeader is the signature and IHDR chunk of a PNG of the given size, bit
// depth and colour type, with no pixel data: enough for DecodeConfig.
func pngHeader(w, h int, bitDepth, colorType byte) []byte {
	var ihdr bytes.Buffer
	ihdr.WriteString("IHDR")
	binary.Write(&ihdr, binary.BigEndian, uint32(w))
	binary.Write(&ihdr, binary.BigEndian, uint32(h))
	ihdr.Write([]byte{bitDepth, colorType, 0, 0, 0})
	var out bytes.Buffer
	out.WriteString("\x89PNG\r\n\x1a\n")
	binary.Write(&out, binary.BigEndian, uint32(ihdr.Len()-4))
	out.Write(ihdr.Bytes())
	binary.Write(&out, binary.BigEndian, crc32.ChecksumIEEE(ihdr.Bytes()))
	return out.Bytes()
}
