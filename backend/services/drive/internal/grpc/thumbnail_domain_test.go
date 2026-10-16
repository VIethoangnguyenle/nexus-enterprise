package grpc_test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/ngac"
	pb "ngac-platform/proto/drive"
	"ngac-platform/services/drive/internal/domain"
	"ngac-platform/services/drive/internal/thumbnail"
)

var fakeJPEG = []byte{0xFF, 0xD8, 0xFF, 0xE0, 'f', 'a', 'k', 'e', 0xFF, 0xD9}

// fakeRenderer stands in for the picture and PDF/video tools.
type fakeRenderer struct {
	calls atomic.Int32
	// gate, when set, holds every drawing until it is closed.
	gate chan struct{}
	err  error

	mu    sync.Mutex
	kinds []thumbnail.Kind
	sizes []int
	srcs  [][]byte
}

func (r *fakeRenderer) Render(ctx context.Context, kind thumbnail.Kind, srcPath string, size int) ([]byte, error) {
	r.calls.Add(1)
	b, _ := os.ReadFile(srcPath)
	r.mu.Lock()
	r.kinds = append(r.kinds, kind)
	r.sizes = append(r.sizes, size)
	r.srcs = append(r.srcs, b)
	r.mu.Unlock()
	if r.gate != nil {
		select {
		case <-r.gate:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if r.err != nil {
		return nil, r.err
	}
	return fakeJPEG, nil
}

type thumbFixture struct {
	*previewFixture
	rend *fakeRenderer
}

func newThumbFixture(t *testing.T, cfg domain.ThumbnailConfig) *thumbFixture {
	t.Helper()
	f := &thumbFixture{previewFixture: newPreviewFixture(t, domain.PreviewConfig{}), rend: &fakeRenderer{}}
	f.svc.SetThumbnailer(f.rend, cfg)
	return f
}

func TestThumbnail_DeniedWithoutReadAndLeaksNothing(t *testing.T) {
	f := newThumbFixture(t, domain.ThumbnailConfig{})
	item := f.file("Lương Q3 mật.png", "image/png", []byte("png"))
	stranger := asCaller("", "stranger")

	_, dlErr := f.svc.GetDownloadURL(stranger, &pb.GetDownloadURLRequest{FileId: item.ID})
	_, err := f.svc.GetThumbnail(stranger, item.ID, 320, "")
	require.ErrorIs(t, err, domain.ErrAccessDenied)
	assert.Equal(t, dlErr.Error(), err.Error(), "a thumbnail must refuse in the words a download does")
	assert.NotContains(t, err.Error(), "Lương")
	assert.NotContains(t, err.Error(), "png")
	assert.Zero(t, f.rend.calls.Load(), "a caller who may not read must not cost a drawing")
	assert.Empty(t, f.storage.keysWithPrefix("drive/thumbs/"))
	assert.Contains(t, f.policy.checks, [3]string{"stranger", item.NGACNodeID, ngac.OpRead})

	// An unsupported kind is refused the same way, not with a "no thumbnail" that
	// would tell a stranger what the file is.
	zip := f.file("secret.zip", "application/zip", []byte("z"))
	_, err = f.svc.GetThumbnail(stranger, zip.ID, 320, "")
	assert.ErrorIs(t, err, domain.ErrAccessDenied)
}

func TestThumbnail_DeniedWithoutReadEvenWhenCached(t *testing.T) {
	f := newThumbFixture(t, domain.ThumbnailConfig{})
	item := f.file("a.png", "image/png", []byte("png"))
	_, err := f.svc.GetThumbnail(reader(), item.ID, 320, "")
	require.NoError(t, err)
	require.Len(t, f.storage.keysWithPrefix("drive/thumbs/"), 1)

	_, err = f.svc.GetThumbnail(asCaller("", "stranger"), item.ID, 320, "")
	assert.ErrorIs(t, err, domain.ErrAccessDenied, "a stored thumbnail is no reason to skip the check")
}

func TestThumbnail_UnknownFolderAndPending(t *testing.T) {
	f := newThumbFixture(t, domain.ThumbnailConfig{})
	f.policy.grant("reader", anyObject, ngac.OpRead)

	_, err := f.svc.GetThumbnail(reader(), uuid.New().String(), 320, "")
	assert.ErrorIs(t, err, domain.ErrNotFound)

	folder, err := f.svc.CreateFolder(reader(), &pb.CreateFolderRequest{WorkspaceId: f.ws, Name: "Hồ sơ"})
	require.NoError(t, err)
	_, err = f.svc.GetThumbnail(reader(), folder.Id, 320, "")
	assert.ErrorIs(t, err, domain.ErrNotFound)

	item := f.file("a.png", "image/png", []byte("x"))
	_, err = f.pool.Exec(context.Background(), `UPDATE drive_items SET status = 'pending' WHERE id = $1`, item.ID)
	require.NoError(t, err)
	_, err = f.svc.GetThumbnail(reader(), item.ID, 320, "")
	assert.ErrorIs(t, err, domain.ErrNotFound)
	assert.Zero(t, f.rend.calls.Load())
}

func TestThumbnail_SizeMustBeOneOfTheOffered(t *testing.T) {
	f := newThumbFixture(t, domain.ThumbnailConfig{})
	item := f.file("a.png", "image/png", []byte("x"))
	for _, size := range []int{0, -1, 100, 321, 5000} {
		_, err := f.svc.GetThumbnail(reader(), item.ID, size, "")
		assert.ErrorIs(t, err, domain.ErrInvalidInput, "size %d", size)
	}
	assert.Zero(t, f.rend.calls.Load())
}

func TestThumbnail_DrawnOnceThenServedFromStorage(t *testing.T) {
	f := newThumbFixture(t, domain.ThumbnailConfig{})
	item := f.file("ảnh.png", "image/png", []byte("png-bytes"))

	th, err := f.svc.GetThumbnail(reader(), item.ID, 320, "")
	require.NoError(t, err)
	assert.Equal(t, fakeJPEG, th.Data)
	assert.NotEmpty(t, th.ETag)
	assert.Equal(t, []thumbnail.Kind{thumbnail.Image}, f.rend.kinds)
	assert.Equal(t, []int{320}, f.rend.sizes)
	assert.Equal(t, []byte("png-bytes"), f.rend.srcs[0], "the renderer is given the file's own bytes")

	keys := f.storage.keysWithPrefix("drive/thumbs/" + item.ID + "/")
	require.Len(t, keys, 1)
	assert.Regexp(t, `^drive/thumbs/`+item.ID+`/\d+-9/320\.jpg$`, keys[0])

	again, err := f.svc.GetThumbnail(reader(), item.ID, 320, "")
	require.NoError(t, err)
	assert.Equal(t, fakeJPEG, again.Data)
	assert.Equal(t, th.ETag, again.ETag)
	assert.EqualValues(t, 1, f.rend.calls.Load(), "the second view comes from storage")

	// The other size is its own thumbnail.
	big, err := f.svc.GetThumbnail(reader(), item.ID, 640, "")
	require.NoError(t, err)
	assert.NotEqual(t, th.ETag, big.ETag)
	assert.EqualValues(t, 2, f.rend.calls.Load())
	assert.Len(t, f.storage.keysWithPrefix("drive/thumbs/"+item.ID+"/"), 2)
}

func TestThumbnail_NewContentMakesANewThumbnail(t *testing.T) {
	f := newThumbFixture(t, domain.ThumbnailConfig{})
	item := f.file("a.png", "image/png", []byte("v1"))
	first, err := f.svc.GetThumbnail(reader(), item.ID, 320, "")
	require.NoError(t, err)

	f.storage.seed(*item.ObjectKey, []byte("version two"), time.Now())
	second, err := f.svc.GetThumbnail(reader(), item.ID, 320, "")
	require.NoError(t, err)
	assert.NotEqual(t, first.ETag, second.ETag)
	assert.EqualValues(t, 2, f.rend.calls.Load(), "changed content is drawn again")
	assert.Len(t, f.storage.keysWithPrefix("drive/thumbs/"+item.ID+"/"), 2, "under a new key; the old one is not reused")
}

func TestThumbnail_IfNoneMatchAnswersNotModifiedOnlyToAReader(t *testing.T) {
	f := newThumbFixture(t, domain.ThumbnailConfig{})
	item := f.file("a.png", "image/png", []byte("x"))
	th, err := f.svc.GetThumbnail(reader(), item.ID, 320, "")
	require.NoError(t, err)

	nm, err := f.svc.GetThumbnail(reader(), item.ID, 320, th.ETag)
	require.NoError(t, err)
	assert.True(t, nm.NotModified)
	assert.Empty(t, nm.Data)
	assert.Equal(t, th.ETag, nm.ETag)

	other, err := f.svc.GetThumbnail(reader(), item.ID, 640, th.ETag)
	require.NoError(t, err)
	assert.False(t, other.NotModified, "an ETag of one size does not vouch for the other")

	_, err = f.svc.GetThumbnail(asCaller("", "stranger"), item.ID, 320, th.ETag)
	assert.ErrorIs(t, err, domain.ErrAccessDenied, "knowing the ETag is not the right to read")
}

func TestThumbnail_UnsupportedKindsAreUnavailable(t *testing.T) {
	f := newThumbFixture(t, domain.ThumbnailConfig{})
	for _, tc := range []struct{ name, mime string }{
		{"a.zip", "application/zip"},
		{"a.txt", "text/plain"},
		{"a.mp3", "audio/mpeg"},
		{"a.svg", "image/svg+xml"},
		{"a.csv", "text/csv"},
		{"noext", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			item := f.file(tc.name, tc.mime, []byte("x"))
			_, err := f.svc.GetThumbnail(reader(), item.ID, 320, "")
			assert.ErrorIs(t, err, domain.ErrThumbnailUnavailable)
		})
	}
	assert.Zero(t, f.rend.calls.Load())
}

func TestThumbnail_KindsRoutedToTheirTool(t *testing.T) {
	f := newThumbFixture(t, domain.ThumbnailConfig{})
	for _, tc := range []struct {
		name, mime string
		want       thumbnail.Kind
	}{
		{"a.jpg", "image/jpeg", thumbnail.Image},
		{"a.webp", "image/webp", thumbnail.Image},
		{"scan.pdf", "application/octet-stream", thumbnail.PDF},
		{"clip.mp4", "video/mp4", thumbnail.Video},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := len(f.rend.kinds)
			item := f.file(tc.name, tc.mime, []byte("x"))
			_, err := f.svc.GetThumbnail(reader(), item.ID, 320, "")
			require.NoError(t, err)
			require.Len(t, f.rend.kinds, before+1)
			assert.Equal(t, tc.want, f.rend.kinds[before])
		})
	}
}

func TestThumbnail_TooLargeSourceIsUnavailable(t *testing.T) {
	f := newThumbFixture(t, domain.ThumbnailConfig{MaxImageBytes: 4})
	item := f.file("big.png", "image/png", []byte("more than four bytes"))
	_, err := f.svc.GetThumbnail(reader(), item.ID, 320, "")
	assert.ErrorIs(t, err, domain.ErrThumbnailUnavailable)
	assert.Zero(t, f.rend.calls.Load())
}

func TestThumbnail_FailureIsUnavailableAndNotRetriedAtOnce(t *testing.T) {
	f := newThumbFixture(t, domain.ThumbnailConfig{})
	f.rend.err = errors.New("pdftoppm: Syntax Error")
	item := f.file("broken.pdf", "application/pdf", []byte("not a pdf"))

	_, err := f.svc.GetThumbnail(reader(), item.ID, 320, "")
	assert.ErrorIs(t, err, domain.ErrThumbnailUnavailable)
	_, err = f.svc.GetThumbnail(reader(), item.ID, 320, "")
	assert.ErrorIs(t, err, domain.ErrThumbnailUnavailable)
	assert.EqualValues(t, 1, f.rend.calls.Load(), "a file that cannot be drawn is left alone for a while")
	assert.Empty(t, f.storage.keysWithPrefix("drive/thumbs/"))
}

func TestThumbnail_MissingToolIsUnavailableNotAnError(t *testing.T) {
	f := newThumbFixture(t, domain.ThumbnailConfig{})
	f.rend.err = errors.Join(thumbnail.ErrToolMissing, errors.New("ffmpeg"))
	item := f.file("clip.mp4", "video/mp4", []byte("x"))
	_, err := f.svc.GetThumbnail(reader(), item.ID, 320, "")
	assert.ErrorIs(t, err, domain.ErrThumbnailUnavailable)
}

func TestThumbnail_NoRendererConfiguredIsUnavailable(t *testing.T) {
	f := newPreviewFixture(t, domain.PreviewConfig{})
	item := f.file("a.png", "image/png", []byte("x"))
	_, err := f.svc.GetThumbnail(reader(), item.ID, 320, "")
	assert.ErrorIs(t, err, domain.ErrThumbnailUnavailable)
}

func TestThumbnail_TooBusyIsBusyNotUnavailable(t *testing.T) {
	f := newThumbFixture(t, domain.ThumbnailConfig{MaxConcurrent: 1, QueueWait: 50 * time.Millisecond})
	f.rend.gate = make(chan struct{})
	a := f.file("a.png", "image/png", []byte("a"))
	b := f.file("b.png", "image/png", []byte("b"))

	done := make(chan error, 1)
	go func() {
		_, err := f.svc.GetThumbnail(reader(), a.ID, 320, "")
		done <- err
	}()
	require.Eventually(t, func() bool { return f.rend.calls.Load() == 1 }, 3*time.Second, 5*time.Millisecond)

	_, err := f.svc.GetThumbnail(reader(), b.ID, 320, "")
	require.ErrorIs(t, err, domain.ErrThumbnailBusy)
	assert.NotErrorIs(t, err, domain.ErrThumbnailUnavailable)

	close(f.rend.gate)
	require.NoError(t, <-done)
	// Busy is not remembered as a failure: once there is room, b is drawn.
	_, err = f.svc.GetThumbnail(reader(), b.ID, 320, "")
	assert.NoError(t, err)
}

func TestThumbnail_ConcurrentFirstViewsShareOneDrawing(t *testing.T) {
	f := newThumbFixture(t, domain.ThumbnailConfig{})
	f.rend.gate = make(chan struct{})
	item := f.file("a.png", "image/png", []byte("a"))

	var wg sync.WaitGroup
	errs := make(chan error, 5)
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := f.svc.GetThumbnail(reader(), item.ID, 320, "")
			errs <- err
		}()
	}
	require.Eventually(t, func() bool { return f.rend.calls.Load() >= 1 }, 3*time.Second, 5*time.Millisecond)
	time.Sleep(100 * time.Millisecond)
	close(f.rend.gate)
	wg.Wait()
	close(errs)
	for err := range errs {
		assert.NoError(t, err)
	}
	assert.EqualValues(t, 1, f.rend.calls.Load())
}

func TestThumbnail_OfficeIsDrawnFromTheCachedPDF(t *testing.T) {
	f := newThumbFixture(t, domain.ThumbnailConfig{})
	item := f.file("Báo cáo.docx", docxMime, []byte("word-bytes"))

	_, err := f.svc.GetThumbnail(reader(), item.ID, 320, "")
	require.NoError(t, err)
	assert.EqualValues(t, 1, f.conv.calls.Load())
	assert.Equal(t, []thumbnail.Kind{thumbnail.PDF}, f.rend.kinds)
	assert.Equal(t, []byte("%PDF-fake:word-bytes"), f.rend.srcs[0], "drawn from the conversion's PDF")

	// The other size reuses the stored PDF instead of converting again.
	_, err = f.svc.GetThumbnail(reader(), item.ID, 640, "")
	require.NoError(t, err)
	assert.EqualValues(t, 1, f.conv.calls.Load())
	assert.EqualValues(t, 2, f.rend.calls.Load())
}

func TestThumbnail_SpreadsheetIsDrawnLikeOffice(t *testing.T) {
	f := newThumbFixture(t, domain.ThumbnailConfig{})
	item := f.file("Sổ.xlsx", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", []byte("sheet"))
	_, err := f.svc.GetThumbnail(reader(), item.ID, 320, "")
	require.NoError(t, err)
	assert.Equal(t, []string{"Sổ.xlsx"}, f.conv.names)
}

func TestThumbnail_OfficeConversionFailureIsUnavailableAndNotRemembered(t *testing.T) {
	f := newThumbFixture(t, domain.ThumbnailConfig{})
	f.conv.err = domain.ErrConverterUnreachable
	item := f.file("a.docx", docxMime, []byte("x"))
	_, err := f.svc.GetThumbnail(reader(), item.ID, 320, "")
	assert.ErrorIs(t, err, domain.ErrThumbnailUnavailable)
	assert.Zero(t, f.rend.calls.Load())

	f.conv.err = nil
	_, err = f.svc.GetThumbnail(reader(), item.ID, 320, "")
	assert.NoError(t, err, "a converter that was down is tried again, not blamed on the file")
}

func TestThumbnail_PermanentDeleteRemovesThumbnails(t *testing.T) {
	f := newThumbFixture(t, domain.ThumbnailConfig{})
	f.policy.grant("reader", anyObject, ngac.OpRead)
	f.policy.grant("reader", anyObject, ngac.OpWrite)
	f.policy.grant("reader", anyObject, ngac.OpManage)
	item := f.file("a.png", "image/png", []byte("x"))
	for _, size := range []int{320, 640} {
		_, err := f.svc.GetThumbnail(reader(), item.ID, size, "")
		require.NoError(t, err)
	}
	require.Len(t, f.storage.keysWithPrefix("drive/thumbs/"+item.ID+"/"), 2)

	_, err := f.svc.TrashItem(reader(), &pb.TrashItemRequest{ItemId: item.ID})
	require.NoError(t, err)
	_, err = f.svc.DeleteItem(reader(), &pb.DeleteItemRequest{ItemId: item.ID})
	require.NoError(t, err)
	assert.Empty(t, f.storage.keysWithPrefix("drive/thumbs/"+item.ID+"/"), "a deleted file's pictures do not outlive it")
}

// The real tools, end to end through the service. Skipped where they are not
// installed.
func TestThumbnail_RealPDFThroughTheService(t *testing.T) {
	if _, err := exec.LookPath("pdftoppm"); err != nil {
		t.Skip("pdftoppm is not installed; this test runs the real tool")
	}
	f := newPreviewFixture(t, domain.PreviewConfig{})
	f.svc.SetThumbnailer(thumbnail.NewExec(), domain.ThumbnailConfig{})
	pdf := "%PDF-1.4\n1 0 obj<</Type/Catalog/Pages 2 0 R>>endobj\n" +
		"2 0 obj<</Type/Pages/Kids[3 0 R]/Count 1>>endobj\n" +
		"3 0 obj<</Type/Page/Parent 2 0 R/MediaBox[0 0 600 300]>>endobj\n" +
		"trailer<</Root 1 0 R/Size 4>>\n%%EOF\n"
	item := f.file("real.pdf", "application/pdf", []byte(pdf))

	th, err := f.svc.GetThumbnail(reader(), item.ID, 320, "")
	require.NoError(t, err)
	require.Greater(t, len(th.Data), 100)
	assert.Equal(t, []byte{0xFF, 0xD8}, th.Data[:2], "a JPEG")
}
