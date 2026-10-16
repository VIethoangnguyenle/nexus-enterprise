package imagenorm

import (
	"encoding/binary"
	"errors"
	"testing"
)

// jpegHeader is the start of a JPEG with only a frame header: enough for the
// size estimate, nothing to decode. sof is 0xC0 (baseline) or 0xC2
// (progressive); sampling holds each component's packed h/v factors.
func jpegHeader(sof byte, w, h int, sampling ...byte) []byte {
	seg := []byte{8, byte(h >> 8), byte(h), byte(w >> 8), byte(w), byte(len(sampling))}
	for i, hv := range sampling {
		seg = append(seg, byte(i+1), hv, 0)
	}
	out := []byte{0xFF, 0xD8, 0xFF, sof}
	out = binary.BigEndian.AppendUint16(out, uint16(len(seg)+2))
	out = append(out, seg...)
	// Without a JFIF APP0 segment, DecodeConfig reads on to the start of scan.
	return append(out, 0xFF, 0xDA, 0x00, 0x02)
}

// withFillBeforeSOF inserts one 0xFF fill byte between the start of image and
// the frame header, which JPEG allows and Go's decoder reads.
func withFillBeforeSOF(b []byte) []byte {
	out := append([]byte{}, b[:2]...)
	out = append(out, 0xFF)
	return append(out, b[2:]...)
}

func TestDecodedBytes_FollowsWhatTheDecoderAllocates(t *testing.T) {
	const w, h = 4000, 3000
	px := int64(w * h)
	cases := []struct {
		name string
		b    []byte
		want int64
	}{
		// 4:2:0 baseline: Y at full size, Cb and Cr at a quarter each.
		{"jpeg 4:2:0", jpegHeader(0xC0, w, h, 0x22, 0x11, 0x11), px*3/2 + px},
		{"jpeg 4:4:4", jpegHeader(0xC0, w, h, 0x11, 0x11, 0x11), px*3 + px},
		// A progressive decode also keeps a 4-byte coefficient per sample.
		{"progressive 4:2:0", jpegHeader(0xC2, w, h, 0x22, 0x11, 0x11), px*3/2*5 + px},
		{"png rgba 8-bit", pngHeader(w, h, 8, 6), px*4 + px},
		{"png rgba 16-bit", pngHeader(w, h, 16, 6), px*8 + px},
		{"png gray", pngHeader(w, h, 8, 0), px + px},
	}
	for _, tc := range cases {
		got, err := decodedBytes(tc.b)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		// The working copy is a quarter-size RGBA of the square; allow the
		// estimate that much slack either way.
		if diff := got - tc.want; diff > px/2 || diff < -px/2 {
			t.Errorf("%s: estimate %d MB, want about %d MB", tc.name, got>>20, tc.want>>20)
		}
	}
}

func TestDecodedBytes_BudgetEdges(t *testing.T) {
	for _, tc := range []struct {
		name string
		b    []byte
		ok   bool
	}{
		{"12 MP phone JPEG", jpegHeader(0xC0, 4032, 3024, 0x22, 0x11, 0x11), true},
		{"12 MP progressive JPEG", jpegHeader(0xC2, 4032, 3024, 0x22, 0x11, 0x11), false},
		{"3000² RGBA PNG", pngHeader(3000, 3000, 8, 6), true},
		{"3300² RGBA PNG", pngHeader(3300, 3300, 8, 6), false},
		{"2200² 16-bit PNG", pngHeader(2200, 2200, 16, 6), true},
		{"2500² 16-bit PNG", pngHeader(2500, 2500, 16, 6), false},
		// Fill bytes (0xFF) may precede any marker; a parser that loses its
		// place on them must not end up estimating a progressive picture as
		// baseline.
		{"progressive behind a fill byte", withFillBeforeSOF(jpegHeader(0xC2, 3500, 3500, 0x11, 0x11, 0x11)), false},
		{"baseline behind a fill byte", withFillBeforeSOF(jpegHeader(0xC0, 4032, 3024, 0x22, 0x11, 0x11)), true},
	} {
		err := checkDecodeBudget(tc.b)
		if tc.ok && err != nil {
			t.Errorf("%s: refused: %v", tc.name, err)
		}
		if !tc.ok && !errors.Is(err, ErrTooManyPixels) {
			t.Errorf("%s: err = %v, want ErrTooManyPixels", tc.name, err)
		}
	}
}

func TestJPEGOrientation_ReadsTheTagAndToleratesJunk(t *testing.T) {
	if got := jpegOrientation([]byte{0xFF, 0xD8, 0xFF, 0xE1, 0x00, 0x04, 'E', 'x'}); got != 1 {
		t.Errorf("a cut-off EXIF block reads as upright, got %d", got)
	}
	if got := jpegOrientation([]byte("not a jpeg")); got != 1 {
		t.Errorf("no EXIF reads as upright, got %d", got)
	}
}
