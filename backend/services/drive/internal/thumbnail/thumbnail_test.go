package thumbnail

import (
	"bytes"
	"context"
	"encoding/binary"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeFile(t *testing.T, name string, b []byte) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(p, b, 0o600))
	return p
}

func pngOf(t *testing.T, w, h int, c color.Color) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))
	return buf.Bytes()
}

func decodeJPEG(t *testing.T, b []byte) image.Image {
	t.Helper()
	img, err := jpeg.Decode(bytes.NewReader(b))
	require.NoError(t, err, "the thumbnail must be a JPEG")
	return img
}

func TestImage_KeepsAspectAndScalesLongestSide(t *testing.T) {
	src := writeFile(t, "wide.png", pngOf(t, 1000, 500, color.RGBA{200, 30, 30, 255}))
	out, err := NewExec().Render(context.Background(), Image, src, 320)
	require.NoError(t, err)
	b := decodeJPEG(t, out).Bounds()
	assert.Equal(t, 320, b.Dx())
	assert.Equal(t, 160, b.Dy(), "a wide picture is not cropped square")

	tall := writeFile(t, "tall.png", pngOf(t, 300, 900, color.RGBA{0, 0, 200, 255}))
	out, err = NewExec().Render(context.Background(), Image, tall, 640)
	require.NoError(t, err)
	b = decodeJPEG(t, out).Bounds()
	assert.Equal(t, 213, b.Dx())
	assert.Equal(t, 640, b.Dy())
}

func TestImage_SmallPictureIsNotEnlarged(t *testing.T) {
	src := writeFile(t, "small.png", pngOf(t, 100, 60, color.White))
	out, err := NewExec().Render(context.Background(), Image, src, 320)
	require.NoError(t, err)
	b := decodeJPEG(t, out).Bounds()
	assert.Equal(t, 100, b.Dx())
	assert.Equal(t, 60, b.Dy())
}

func TestImage_TransparencyLiesOnWhite(t *testing.T) {
	src := writeFile(t, "clear.png", pngOf(t, 50, 50, color.NRGBA{0, 0, 0, 0}))
	out, err := NewExec().Render(context.Background(), Image, src, 320)
	require.NoError(t, err)
	r, g, b, _ := decodeJPEG(t, out).At(25, 25).RGBA()
	assert.Greater(t, r>>8, uint32(240))
	assert.Greater(t, g>>8, uint32(240))
	assert.Greater(t, b>>8, uint32(240))
}

// withOrientation inserts an EXIF Orientation segment into a JPEG.
func withOrientation(t *testing.T, j []byte, o uint16) []byte {
	t.Helper()
	tiff := []byte{'M', 'M', 0, 42, 0, 0, 0, 8, 0, 1, 0x01, 0x12, 0, 3, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0}
	binary.BigEndian.PutUint16(tiff[18:], o)
	seg := append([]byte("Exif\x00\x00"), tiff...)
	app1 := append([]byte{0xFF, 0xE1, byte((len(seg) + 2) >> 8), byte(len(seg) + 2)}, seg...)
	return append(append(append([]byte{}, j[:2]...), app1...), j[2:]...)
}

func TestImage_ExifOrientationIsApplied(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, jpeg.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 400, 200)), nil))
	src := writeFile(t, "rot.jpg", withOrientation(t, buf.Bytes(), 6))
	out, err := NewExec().Render(context.Background(), Image, src, 320)
	require.NoError(t, err)
	b := decodeJPEG(t, out).Bounds()
	assert.Equal(t, 160, b.Dx(), "a quarter turn swaps the sides")
	assert.Equal(t, 320, b.Dy())
}

func TestImage_RejectsGarbageAndHugeHeaders(t *testing.T) {
	bad := writeFile(t, "x.png", []byte("not a picture at all"))
	_, err := NewExec().Render(context.Background(), Image, bad, 320)
	assert.ErrorIs(t, err, ErrNotImage)

	// A tiny PNG whose header claims 20000x20000.
	b := pngOf(t, 4, 4, color.White)
	binary.BigEndian.PutUint32(b[16:], 20000)
	binary.BigEndian.PutUint32(b[20:], 20000)
	_, err = NewExec().Render(context.Background(), Image, writeFile(t, "big.png", b), 320)
	assert.Error(t, err)
}

func TestRender_UnknownKindAndBadSize(t *testing.T) {
	_, err := NewExec().Render(context.Background(), Kind(99), "x", 320)
	assert.Error(t, err)
	_, err = NewExec().Render(context.Background(), Image, "x", 0)
	assert.Error(t, err)
}

const minimalPDF = "%PDF-1.4\n1 0 obj<</Type/Catalog/Pages 2 0 R>>endobj\n" +
	"2 0 obj<</Type/Pages/Kids[3 0 R]/Count 1>>endobj\n" +
	"3 0 obj<</Type/Page/Parent 2 0 R/MediaBox[0 0 600 300]>>endobj\n" +
	"trailer<</Root 1 0 R/Size 4>>\n%%EOF\n"

func needTool(t *testing.T, name string) {
	t.Helper()
	if _, err := exec.LookPath(name); err != nil {
		t.Skipf("%s is not installed; this test runs the real tool", name)
	}
}

func TestPDF_FirstPageScaledToSize(t *testing.T) {
	needTool(t, "pdftoppm")
	src := writeFile(t, "a.pdf", []byte(minimalPDF))
	out, err := NewExec().Render(context.Background(), PDF, src, 320)
	require.NoError(t, err)
	b := decodeJPEG(t, out).Bounds()
	assert.Equal(t, 320, b.Dx())
	assert.Equal(t, 160, b.Dy())
}

func TestPDF_BrokenFileFails(t *testing.T) {
	needTool(t, "pdftoppm")
	_, err := NewExec().Render(context.Background(), PDF, writeFile(t, "b.pdf", []byte("nope")), 320)
	assert.Error(t, err)
}

func TestVideo_FrameScaledToSize(t *testing.T) {
	needTool(t, "ffmpeg")
	dir := t.TempDir()
	src := filepath.Join(dir, "v.mp4")
	gen := exec.Command("ffmpeg", "-nostdin", "-loglevel", "error", "-f", "lavfi", "-i",
		"testsrc=duration=2:size=640x360:rate=10", "-c:v", "mpeg4", src)
	if out, err := gen.CombinedOutput(); err != nil {
		t.Skipf("cannot make a test video here: %v %s", err, out)
	}
	out, err := NewExec().Render(context.Background(), Video, src, 320)
	require.NoError(t, err)
	b := decodeJPEG(t, out).Bounds()
	assert.Equal(t, 320, b.Dx())
	assert.Equal(t, 180, b.Dy())
}

func TestVideo_ShorterThanASecondStillGetsAFrame(t *testing.T) {
	needTool(t, "ffmpeg")
	src := filepath.Join(t.TempDir(), "short.mp4")
	gen := exec.Command("ffmpeg", "-nostdin", "-loglevel", "error", "-f", "lavfi", "-i",
		"testsrc=duration=0.5:size=320x240:rate=10", "-c:v", "mpeg4", src)
	if out, err := gen.CombinedOutput(); err != nil {
		t.Skipf("cannot make a test video here: %v %s", err, out)
	}
	out, err := NewExec().Render(context.Background(), Video, src, 640)
	require.NoError(t, err)
	assert.Equal(t, 320, decodeJPEG(t, out).Bounds().Dx(), "a small video is not enlarged")
}

func TestRun_StopsWithContextAndReportsMissingTool(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if _, err := exec.LookPath("sleep"); err == nil {
		start := time.Now()
		err := run(ctx, nil, t.TempDir(), "sleep", "5")
		assert.Error(t, err)
		assert.Less(t, time.Since(start), 3*time.Second)
	}
	err := run(context.Background(), nil, t.TempDir(), "definitely-not-a-real-tool-xyz")
	assert.ErrorIs(t, err, ErrToolMissing)
}
