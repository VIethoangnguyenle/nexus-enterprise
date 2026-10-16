package imagenorm

import (
	"bytes"
	"errors"
	"image"
	"image/gif"
	"runtime"
	"testing"
	"time"
)

var avatarSizes = []int{1024, 192, 64}

func TestSquare_MakesOneSquareJPEGPerSize(t *testing.T) {
	out, err := Square(encodePNG(t, quadrants(600, 300)), avatarSizes, 85)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != len(avatarSizes) {
		t.Fatalf("got %d pictures, want %d", len(out), len(avatarSizes))
	}
	for i, size := range avatarSizes {
		got := decodeJPEG(t, out[i]).Bounds()
		if got.Dx() != size || got.Dy() != size {
			t.Errorf("size %d: got %v", size, got)
		}
	}
	if len(out[2]) >= len(out[1]) || len(out[1]) >= len(out[0]) {
		t.Errorf("the small sizes must be smaller files: %d / %d / %d", len(out[2]), len(out[1]), len(out[0]))
	}
}

func TestSquare_SingleSize(t *testing.T) {
	out, err := Square(encodePNG(t, quadrants(300, 500)), []int{256}, 80)
	if err != nil || len(out) != 1 {
		t.Fatalf("out=%d err=%v", len(out), err)
	}
	if got := decodeJPEG(t, out[0]).Bounds(); got.Dx() != 256 || got.Dy() != 256 {
		t.Errorf("got %v", got)
	}
}

func TestSquare_RejectsBadSizes(t *testing.T) {
	src := encodePNG(t, quadrants(100, 100))
	for name, sizes := range map[string][]int{
		"none":       nil,
		"zero":       {0},
		"negative":   {-4},
		"ascending":  {64, 192},
		"duplicated": {64, 64},
	} {
		if _, err := Square(src, sizes, 85); !errors.Is(err, ErrBadSizes) {
			t.Errorf("%s: err = %v, want ErrBadSizes", name, err)
		}
	}
}

func TestSquare_DropsEXIFButKeepsTheWayUp(t *testing.T) {
	src := encodeJPEG(t, quadrants(400, 400))
	tl, tr, bl, br := image.Pt(100, 100), image.Pt(900, 100), image.Pt(100, 900), image.Pt(900, 900)
	for _, tc := range []struct {
		orientation    uint16
		redAt, greenAt image.Point
	}{
		{1, tl, tr}, {2, tr, tl}, {3, br, bl}, {4, bl, br},
		{5, tl, bl}, {6, tr, br}, {7, br, tr}, {8, bl, tl},
	} {
		out, err := Square(withOrientation(src, tc.orientation), avatarSizes, 85)
		if err != nil {
			t.Fatalf("orientation %d: %v", tc.orientation, err)
		}
		if bytes.Contains(out[0], []byte("Exif")) {
			t.Errorf("orientation %d: EXIF survived the re-encode", tc.orientation)
		}
		img := decodeJPEG(t, out[0])
		if !isRed(img.At(tc.redAt.X, tc.redAt.Y)) {
			t.Errorf("orientation %d: red quarter is not at %v", tc.orientation, tc.redAt)
		}
		if !isGreen(img.At(tc.greenAt.X, tc.greenAt.Y)) {
			t.Errorf("orientation %d: green quarter is not at %v", tc.orientation, tc.greenAt)
		}
	}
}

func TestSquare_TransparencyBecomesWhite(t *testing.T) {
	out, err := Square(encodePNG(t, image.NewNRGBA(image.Rect(0, 0, 200, 200))), avatarSizes, 85)
	if err != nil {
		t.Fatal(err)
	}
	r, g, b, _ := decodeJPEG(t, out[1]).At(5, 5).RGBA()
	if r < 0xF000 || g < 0xF000 || b < 0xF000 {
		t.Errorf("a transparent picture must come out white, got %x %x %x", r, g, b)
	}
}

func TestSquare_RefusesWhatIsNotAJPEGOrPNG(t *testing.T) {
	var g bytes.Buffer
	if err := gif.Encode(&g, quadrants(20, 20), nil); err != nil {
		t.Fatal(err)
	}
	truncated := encodePNG(t, quadrants(300, 300))
	truncated = truncated[:len(truncated)/2]
	for name, b := range map[string][]byte{
		"text named .png": []byte("<svg onload=alert(1)>"),
		"empty":           nil,
		"gif":             g.Bytes(),
		"cut-off png":     truncated,
	} {
		if _, err := Square(b, avatarSizes, 85); !errors.Is(err, ErrNotImage) {
			t.Errorf("%s: err = %v, want ErrNotImage", name, err)
		}
	}
}

func TestSquare_RefusesAPictureTooBigToDecodeWithoutDecodingIt(t *testing.T) {
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	_, err := Square(pngHeader(10000, 10000, 8, 6), avatarSizes, 85)
	runtime.ReadMemStats(&after)
	if !errors.Is(err, ErrTooManyPixels) {
		t.Fatalf("err = %v, want ErrTooManyPixels", err)
	}
	if grew := after.TotalAlloc - before.TotalAlloc; grew > 1<<20 {
		t.Errorf("refusing allocated %d bytes", grew)
	}
}

// A 12 MP phone photo is the common case and must fit the memory a small
// service has (192 MB in production, shared with everything else it does).
func TestSquare_PhonePhotoStaysInBudget(t *testing.T) {
	if testing.Short() {
		t.Skip("encodes a 12 MP JPEG")
	}
	src := encodeJPEG(t, quadrants(4032, 3024))
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	start := time.Now()
	if _, err := Square(src, avatarSizes, 85); err != nil {
		t.Fatal(err)
	}
	took := time.Since(start)
	runtime.ReadMemStats(&after)
	allocated := after.TotalAlloc - before.TotalAlloc
	t.Logf("12 MP JPEG: allocated %d MB in %v", allocated>>20, took)
	if allocated > 120<<20 {
		t.Errorf("allocated %d MB, want under 120 MB", allocated>>20)
	}
}
