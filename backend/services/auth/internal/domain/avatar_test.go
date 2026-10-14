package domain_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"runtime"
	"strings"
	"testing"
	"time"

	"ngac-platform/services/auth/internal/domain"
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
	binary.Write(&tiff, binary.BigEndian, uint32(8))           // IFD0 offset
	binary.Write(&tiff, binary.BigEndian, uint16(1))           // one entry
	binary.Write(&tiff, binary.BigEndian, uint16(0x0112))      // Orientation
	binary.Write(&tiff, binary.BigEndian, uint16(3))           // SHORT
	binary.Write(&tiff, binary.BigEndian, uint32(1))           // count
	binary.Write(&tiff, binary.BigEndian, orientation)         // value
	binary.Write(&tiff, binary.BigEndian, uint16(0))           // padding
	binary.Write(&tiff, binary.BigEndian, uint32(0))           // no next IFD
	payload := append([]byte("Exif\x00\x00"), tiff.Bytes()...) //nolint:gocritic
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

func avatarService(t *testing.T) (*domain.Service, string) {
	t.Helper()
	w := newFakeWorld()
	svc := w.service(t)
	return svc, w.addUser("avatar@example.test").ID
}

func TestNormalizeAvatar_MakesThreeSquareJPEGs(t *testing.T) {
	out, err := domain.NormalizeAvatar(encodePNG(t, quadrants(600, 300)))
	if err != nil {
		t.Fatal(err)
	}
	for size, b := range map[int][]byte{1024: out.Large, 192: out.Medium, 64: out.Small} {
		img := decodeJPEG(t, b)
		if got := img.Bounds(); got.Dx() != size || got.Dy() != size {
			t.Errorf("size %d: got %v", size, got)
		}
	}
	if len(out.Small) >= len(out.Medium) || len(out.Medium) >= len(out.Large) {
		t.Errorf("the small sizes must be smaller files: %d / %d / %d", len(out.Small), len(out.Medium), len(out.Large))
	}
}

func TestNormalizeAvatar_DropsEXIFButKeepsTheWayUp(t *testing.T) {
	src := encodeJPEG(t, quadrants(400, 400))
	tl, tr, bl, br := image.Pt(100, 100), image.Pt(900, 100), image.Pt(100, 900), image.Pt(900, 900)
	for _, tc := range []struct {
		orientation    uint16
		redAt, greenAt image.Point // where the red and green quarters must end up in the 1024 output
	}{
		{1, tl, tr}, // as stored
		{2, tr, tl}, // mirrored left to right
		{3, br, bl}, // half a turn
		{4, bl, br}, // mirrored top to bottom
		{5, tl, bl}, // mirrored, then a quarter turn counter-clockwise
		{6, tr, br}, // a quarter turn clockwise: top-left goes top-right
		{7, br, tr}, // mirrored, then a quarter turn clockwise
		{8, bl, tl}, // a quarter turn counter-clockwise: top-left goes bottom-left
	} {
		out, err := domain.NormalizeAvatar(withOrientation(src, tc.orientation))
		if err != nil {
			t.Fatalf("orientation %d: %v", tc.orientation, err)
		}
		if bytes.Contains(out.Large, []byte("Exif")) {
			t.Errorf("orientation %d: EXIF survived the re-encode", tc.orientation)
		}
		img := decodeJPEG(t, out.Large)
		if !isRed(img.At(tc.redAt.X, tc.redAt.Y)) {
			t.Errorf("orientation %d: red quarter is not at %v", tc.orientation, tc.redAt)
		}
		if !isGreen(img.At(tc.greenAt.X, tc.greenAt.Y)) {
			t.Errorf("orientation %d: green quarter is not at %v", tc.orientation, tc.greenAt)
		}
	}
}

func TestNormalizeAvatar_TransparencyBecomesWhite(t *testing.T) {
	out, err := domain.NormalizeAvatar(encodePNG(t, image.NewNRGBA(image.Rect(0, 0, 200, 200))))
	if err != nil {
		t.Fatal(err)
	}
	r, g, b, _ := decodeJPEG(t, out.Medium).At(5, 5).RGBA()
	if r < 0xF000 || g < 0xF000 || b < 0xF000 {
		t.Errorf("a transparent picture must come out white, got %x %x %x", r, g, b)
	}
}

func TestNormalizeAvatar_RefusesWhatIsNotAJPEGOrPNG(t *testing.T) {
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
		if _, err := domain.NormalizeAvatar(b); !errors.Is(err, domain.ErrAvatarNotImage) {
			t.Errorf("%s: err = %v, want ErrAvatarNotImage", name, err)
		}
	}
}

func TestNormalizeAvatar_RefusesAPictureTooBigToDecodeWithoutDecodingIt(t *testing.T) {
	// A 10000×10000 header with nothing behind it: refusing it must not need
	// the 400 MB a decode would.
	var before runtime.MemStats
	runtime.ReadMemStats(&before)
	_, err := domain.NormalizeAvatar(domain.PNGHeaderForTest(10000, 10000, 8, 6))
	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	if !errors.Is(err, domain.ErrAvatarTooManyPixels) {
		t.Fatalf("err = %v, want ErrAvatarTooManyPixels", err)
	}
	if grew := after.TotalAlloc - before.TotalAlloc; grew > 1<<20 {
		t.Errorf("refusing allocated %d bytes", grew)
	}
}

// A 12 MP phone photo is the common case and must fit the memory the auth
// service has (192 MB in production, shared with sign-in).
func TestNormalizeAvatar_PhonePhotoStaysInBudget(t *testing.T) {
	if testing.Short() {
		t.Skip("encodes a 12 MP JPEG")
	}
	src := encodeJPEG(t, quadrants(4032, 3024))
	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)
	start := time.Now()
	if _, err := domain.NormalizeAvatar(src); err != nil {
		t.Fatal(err)
	}
	took := time.Since(start)
	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	allocated := after.TotalAlloc - before.TotalAlloc
	t.Logf("12 MP JPEG: allocated %d MB in %v", allocated>>20, took)
	if allocated > 120<<20 {
		t.Errorf("allocated %d MB, want under 120 MB", allocated>>20)
	}
}

func TestSetAvatar_PointsTheProfileAtAPictureThatIsServed(t *testing.T) {
	svc, userID := avatarService(t)
	url, err := svc.SetAvatar(context.Background(), userID, encodePNG(t, quadrants(300, 300)))
	if err != nil {
		t.Fatal(err)
	}
	token := strings.TrimPrefix(url, domain.AvatarURLPrefix)
	if token == url || len(token) != 32 {
		t.Fatalf("url = %q, want %s<32 hex>", url, domain.AvatarURLPrefix)
	}
	for _, size := range []int{64, 192, 1024} {
		b, err := svc.Avatar(context.Background(), token, size)
		if err != nil || len(b) == 0 {
			t.Errorf("size %d: %d bytes, %v", size, len(b), err)
		}
	}
	u, _ := svc.GetUserByID(context.Background(), userID)
	if u.AvatarURL != url {
		t.Errorf("profile avatar = %q, want %q", u.AvatarURL, url)
	}

	again, err := svc.SetAvatar(context.Background(), userID, encodePNG(t, quadrants(300, 300)))
	if err != nil {
		t.Fatal(err)
	}
	if again == url {
		t.Error("the same picture uploaded twice must get a new address, so the URL says nothing about the picture")
	}
	if _, err := svc.Avatar(context.Background(), token, 192); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("replaced picture: err = %v, want ErrNotFound", err)
	}
}

func TestSetAvatar_Refusals(t *testing.T) {
	svc, userID := avatarService(t)
	ctx := context.Background()
	if _, err := svc.SetAvatar(ctx, "", encodePNG(t, quadrants(10, 10))); !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("no user: err = %v, want ErrInvalidInput", err)
	}
	if _, err := svc.SetAvatar(ctx, "nobody", encodePNG(t, quadrants(10, 10))); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("unknown user: err = %v, want ErrNotFound", err)
	}
	if _, err := svc.SetAvatar(ctx, userID, []byte("nope")); !errors.Is(err, domain.ErrAvatarNotImage) {
		t.Errorf("not an image: err = %v, want ErrAvatarNotImage", err)
	}
	u, _ := svc.GetUserByID(ctx, userID)
	if u.AvatarURL != "" {
		t.Errorf("a refused upload must change nothing, avatar = %q", u.AvatarURL)
	}
}

func TestAvatar_UnknownOrMalformedIsNotFound(t *testing.T) {
	svc, userID := avatarService(t)
	url, err := svc.SetAvatar(context.Background(), userID, encodePNG(t, quadrants(50, 50)))
	if err != nil {
		t.Fatal(err)
	}
	token := strings.TrimPrefix(url, domain.AvatarURLPrefix)
	for _, c := range []struct {
		token string
		size  int
	}{
		{strings.Repeat("0", 32), 192}, // well formed, nobody's
		{token, 100},                   // not a size we keep
		{"../../etc/passwd", 192},
		{strings.ToUpper(token), 192},
		{token + "0", 192},
	} {
		if _, err := svc.Avatar(context.Background(), c.token, c.size); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("%q/%d: err = %v, want ErrNotFound", c.token, c.size, err)
		}
	}
}

func TestClearAvatar_RemovesItAndRefusesAnUnknownAccount(t *testing.T) {
	svc, userID := avatarService(t)
	ctx := context.Background()
	url, err := svc.SetAvatar(ctx, userID, encodePNG(t, quadrants(50, 50)))
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.ClearAvatar(ctx, userID); err != nil {
		t.Fatal(err)
	}
	u, _ := svc.GetUserByID(ctx, userID)
	if u.AvatarURL != "" {
		t.Errorf("avatar = %q after clearing", u.AvatarURL)
	}
	if _, err := svc.Avatar(ctx, strings.TrimPrefix(url, domain.AvatarURLPrefix), 64); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("cleared picture: err = %v, want ErrNotFound", err)
	}
	if err := svc.ClearAvatar(ctx, "nobody"); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("unknown user: err = %v, want ErrNotFound", err)
	}
}

// One picture is decoded at a time. A second upload waits for the first, and
// gives up with ErrAvatarBusy only when the wait runs out.
func TestSetAvatar_OneDecodeAtATime(t *testing.T) {
	svc, userID := avatarService(t)
	svc.SetAvatarWaitForTest(50 * time.Millisecond)
	pic := encodePNG(t, quadrants(40, 40))

	release := svc.HoldAvatarGateForTest()
	if _, err := svc.SetAvatar(context.Background(), userID, pic); !errors.Is(err, domain.ErrAvatarBusy) {
		t.Fatalf("while another decode runs past the wait: err = %v, want ErrAvatarBusy", err)
	}
	time.AfterFunc(10*time.Millisecond, release)
	if _, err := svc.SetAvatar(context.Background(), userID, pic); err != nil {
		t.Fatalf("a decode that finishes within the wait lets the next one through: %v", err)
	}
}

func TestSetAvatar_TenUploadsAnHour(t *testing.T) {
	svc, w, rdb := otpService(t, nil)
	u := w.addUser("avatar-rl@example.test")
	key := "avatar_rl:" + u.ID
	rdb.Del(context.Background(), key)
	t.Cleanup(func() { rdb.Del(context.Background(), key) })
	pic := encodePNG(t, quadrants(20, 20))

	for i := 0; i < domain.AvatarUploadLimit; i++ {
		if _, err := svc.SetAvatar(context.Background(), u.ID, pic); err != nil {
			t.Fatalf("upload %d: %v", i+1, err)
		}
	}
	_, err := svc.SetAvatar(context.Background(), u.ID, pic)
	if !errors.Is(err, domain.ErrRateLimited) {
		t.Fatalf("upload %d: err = %v, want ErrRateLimited", domain.AvatarUploadLimit+1, err)
	}
	if d, ok := domain.RetryAfter(err); !ok || d <= 0 {
		t.Errorf("a limited upload says when to come back, got %v %v", d, ok)
	}
}
