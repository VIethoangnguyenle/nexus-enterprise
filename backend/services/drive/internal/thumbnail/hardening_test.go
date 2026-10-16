package thumbnail

import (
	"context"
	"encoding/binary"
	"image/color"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// jpegFrameOnly is a JPEG that is nothing but a start marker and a frame
// header: enough to claim a size, far too little to decode.
func jpegFrameOnly(sof byte, w, h int) []byte {
	b := []byte{0xFF, 0xD8, 0xFF, sof, 0, 17, 8, 0, 0, 0, 0, 3, 1, 0x22, 0, 2, 0x11, 1, 3, 0x11, 1,
		// Without a start of scan, DecodeConfig keeps reading past the header.
		0xFF, 0xDA, 0x00, 0x02}
	binary.BigEndian.PutUint16(b[7:], uint16(h))
	binary.BigEndian.PutUint16(b[9:], uint16(w))
	return b
}

func TestImage_ProgressiveJPEGOverTheBudgetIsRefusedBeforeDecoding(t *testing.T) {
	// 4000x4000 progressive: ~136 MB to decode. The bytes cannot be decoded at
	// all, so ErrTooManyPixels (not ErrNotImage) proves the header was judged
	// first.
	src := writeFile(t, "p.jpg", jpegFrameOnly(0xC2, 4000, 4000))
	_, err := NewExec().Render(context.Background(), Image, src, 320)
	assert.ErrorIs(t, err, ErrTooManyPixels)

	// The same size as a baseline JPEG is within the budget; here it fails only
	// because there is no picture data behind the header.
	src = writeFile(t, "b.jpg", jpegFrameOnly(0xC0, 4000, 4000))
	_, err = NewExec().Render(context.Background(), Image, src, 320)
	assert.ErrorIs(t, err, ErrNotImage)
}

func TestImage_GIFOverTheBudgetIsRefused(t *testing.T) {
	g := []byte("GIF89a\x00\x00\x00\x00\x00\x00\x00")
	binary.LittleEndian.PutUint16(g[6:], 5000)
	binary.LittleEndian.PutUint16(g[8:], 5000)
	_, err := NewExec().Render(context.Background(), Image, writeFile(t, "g.gif", g), 320)
	assert.ErrorIs(t, err, ErrTooManyPixels)
}

func TestImage_DecodesAreLimitedAndWaitForTheContext(t *testing.T) {
	for i := 0; i < maxImageDecodes; i++ {
		imageSlots <- struct{}{}
	}
	defer func() {
		for i := 0; i < maxImageDecodes; i++ {
			<-imageSlots
		}
	}()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	src := writeFile(t, "ok.png", pngOf(t, 10, 10, color.White))
	_, err := NewExec().Render(ctx, Image, src, 320)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestSysProcAttr_DropsPrivilegesOnlyWhenRoot(t *testing.T) {
	a := sysProcAttr(0, 65534, 65534)
	require.NotNil(t, a)
	require.NotNil(t, a.Credential)
	assert.Equal(t, uint32(65534), a.Credential.Uid)
	assert.Equal(t, uint32(65534), a.Credential.Gid)
	assert.Empty(t, a.Credential.Groups)
	assert.False(t, a.Credential.NoSetGroups, "supplementary groups are cleared")

	assert.Nil(t, sysProcAttr(1000, 65534, 65534), "a non-root service cannot and need not drop")
	assert.Nil(t, sysProcAttr(0, 0, 0), "uid 0 configured means no drop")
}

func TestToolEnv_DroppedToolGetsItsOwnHome(t *testing.T) {
	t.Setenv("HOME", "/root")
	t.Setenv("SECRET_TOKEN", "x")
	env := toolEnv(true, "/work")
	assert.Contains(t, env, "HOME=/work")
	assert.NotContains(t, env, "HOME=/root")
	for _, kv := range env {
		assert.NotContains(t, kv, "SECRET_TOKEN")
	}
}

func TestVideoArgs_RestrictProtocolAndFormat(t *testing.T) {
	args := videoArgs("in", "out", "scale", "1")
	i := indexOf(args, "-protocol_whitelist")
	require.GreaterOrEqual(t, i, 0)
	assert.Equal(t, "file", args[i+1])
	j := indexOf(args, "-format_whitelist")
	require.GreaterOrEqual(t, j, 0)
	assert.Equal(t, videoFormats, args[j+1])
	assert.Less(t, max(i, j), indexOf(args, "-i"), "input options must precede -i")
}

func indexOf(s []string, v string) int {
	for i := range s {
		if s[i] == v {
			return i
		}
	}
	return -1
}

func TestVideo_PlaylistNamedMP4IsRefused(t *testing.T) {
	needTool(t, "ffmpeg")
	dir := t.TempDir()
	seg := filepath.Join(dir, "seg.ts")
	gen := exec.Command("ffmpeg", "-nostdin", "-loglevel", "error", "-f", "lavfi", "-i",
		"testsrc=duration=2:size=320x240:rate=10", "-c:v", "mpeg2video", "-f", "mpegts", seg)
	if out, err := gen.CombinedOutput(); err != nil {
		t.Skipf("cannot make a test segment here: %v %s", err, out)
	}
	// The files they name sit beside them, so without the format allowlist the
	// relative reference would resolve and a frame would be drawn.
	pl := "#EXTM3U\n#EXT-X-TARGETDURATION:3\n#EXT-X-VERSION:3\n#EXTINF:2.0,\nseg.ts\n#EXT-X-ENDLIST\n"
	_, err := NewExec().Render(context.Background(), Video, inDir(t, dir, "evil.mp4", pl), 320)
	require.Error(t, err, "an HLS playlist must not be followed")

	concat := "ffconcat version 1.0\nfile 'seg.ts'\n"
	_, err = NewExec().Render(context.Background(), Video, inDir(t, dir, "evil2.mp4", concat), 320)
	require.Error(t, err, "a concat script must not be followed")

	// The segment itself is a real video, but not in an allowed container.
	_, err = NewExec().Render(context.Background(), Video, seg, 320)
	require.Error(t, err, "MPEG-TS is not an allowed container")
}

// inDir writes a file next to the others in dir.
func inDir(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(p, []byte(content), 0o600))
	return p
}

func TestVideo_AllowedContainersStillWork(t *testing.T) {
	needTool(t, "ffmpeg")
	for _, tc := range []struct{ ext, codec string }{{"mkv", "mpeg4"}, {"avi", "mpeg4"}, {"mov", "mpeg4"}, {"webm", "libvpx"}} {
		src := filepath.Join(t.TempDir(), "v."+tc.ext)
		gen := exec.Command("ffmpeg", "-nostdin", "-loglevel", "error", "-f", "lavfi", "-i",
			"testsrc=duration=2:size=320x240:rate=10", "-c:v", tc.codec, src)
		if out, err := gen.CombinedOutput(); err != nil {
			t.Logf("skip %s: %v %s", tc.ext, err, out)
			continue
		}
		out, err := NewExec().Render(context.Background(), Video, src, 160)
		require.NoError(t, err, tc.ext)
		assert.Equal(t, 160, decodeJPEG(t, out).Bounds().Dx(), tc.ext)
	}
}

func TestExec_StagesTheSourceForADroppedTool(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("staging for another user only happens when running as root")
	}
	src := writeFile(t, "a.pdf", []byte(minimalPDF))
	e := &Exec{euid: 0, toolUID: 65534, toolGID: 65534}
	dir, staged, err := e.workDir(src)
	require.NoError(t, err)
	defer os.RemoveAll(dir)
	assert.Equal(t, filepath.Join(dir, "input"), staged)
	_, err = os.Stat(staged)
	assert.NoError(t, err)
}
