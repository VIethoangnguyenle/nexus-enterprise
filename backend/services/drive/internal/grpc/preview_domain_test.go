package grpc_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"ngac-platform/ngac"
	docpb "ngac-platform/proto/document"
	pb "ngac-platform/proto/drive"
	"ngac-platform/services/drive/internal/domain"
	"ngac-platform/services/drive/internal/store"
)

// fakeStorage is the document service over an in-memory bucket, with a real
// HTTP server behind the signed URLs it hands out — the way the real one hands
// out URLs for MinIO.
type fakeStorage struct {
	docpb.DocumentStorageServiceClient
	srv *httptest.Server

	// urlBase, when set, replaces the test server's address in the URLs handed
	// out (as the public https host does in production); the server still
	// answers whatever address it is dialed on.
	urlBase string

	mu      sync.Mutex
	objects map[string]fakeObject
	puts    int
	hosts   []string
	deleted []string
}

func (f *fakeStorage) base() string {
	if f.urlBase != "" {
		return f.urlBase
	}
	return f.srv.URL
}

type fakeObject struct {
	data     []byte
	modified time.Time
}

func newFakeStorage(t *testing.T) *fakeStorage {
	t.Helper()
	f := &fakeStorage{objects: map[string]fakeObject{}}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		f.hosts = append(f.hosts, r.Host)
		switch r.Method {
		case http.MethodGet:
			o, ok := f.objects[strings.TrimPrefix(r.URL.Path, "/get/")]
			if !ok {
				http.NotFound(w, r)
				return
			}
			_, _ = w.Write(o.data)
		case http.MethodPut:
			b, _ := io.ReadAll(r.Body)
			f.objects[strings.TrimPrefix(r.URL.Path, "/put/")] = fakeObject{data: b, modified: time.Now()}
			f.puts++
		}
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeStorage) seed(key string, data []byte, modified time.Time) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.objects[key] = fakeObject{data: data, modified: modified}
}

func (f *fakeStorage) get(key string) ([]byte, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	o, ok := f.objects[key]
	return o.data, ok
}

func (f *fakeStorage) keysWithPrefix(prefix string) []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for k := range f.objects {
		if strings.HasPrefix(k, prefix) {
			out = append(out, k)
		}
	}
	return out
}

func (f *fakeStorage) GetObjectInfo(_ context.Context, r *docpb.GetObjectInfoRequest, _ ...grpc.CallOption) (*docpb.ObjectInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	o, ok := f.objects[r.ObjectKey]
	if !ok {
		return nil, status.Error(codes.NotFound, "object not found")
	}
	return &docpb.ObjectInfo{ObjectKey: r.ObjectKey, SizeBytes: int64(len(o.data)), LastModified: timestamppb.New(o.modified)}, nil
}

func (f *fakeStorage) DeleteObject(_ context.Context, r *docpb.DeleteObjectRequest, _ ...grpc.CallOption) (*docpb.Empty, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.objects, r.ObjectKey)
	f.deleted = append(f.deleted, r.ObjectKey)
	return &docpb.Empty{}, nil
}

func (f *fakeStorage) GetUploadURL(_ context.Context, r *docpb.GetUploadURLRequest, _ ...grpc.CallOption) (*docpb.GetUploadURLResponse, error) {
	key := fmt.Sprintf("drive/%s/%s", r.DocId, r.Filename)
	return &docpb.GetUploadURLResponse{UploadUrl: f.base() + "/put/" + key, ObjectKey: key}, nil
}

func (f *fakeStorage) GetDownloadURL(_ context.Context, r *docpb.GetDownloadURLRequest, _ ...grpc.CallOption) (*docpb.GetDownloadURLResponse, error) {
	return &docpb.GetDownloadURLResponse{DownloadUrl: f.base() + "/get/" + r.ObjectKey}, nil
}

// fakeConverter stands in for Gotenberg.
type fakeConverter struct {
	calls atomic.Int32
	// gate, when set, holds every conversion until it is closed.
	gate  chan struct{}
	err   error
	deaf  bool // holds the converter even past its deadline
	extra int  // bytes of padding added to the PDF
	max   int64

	mu    sync.Mutex
	names []string
}

func (c *fakeConverter) ToPDF(ctx context.Context, name string, src io.Reader, dst io.Writer, max int64) error {
	c.calls.Add(1)
	c.mu.Lock()
	c.names = append(c.names, name)
	c.max = max
	c.mu.Unlock()
	if c.gate != nil {
		select {
		case <-c.gate:
		case <-ctx.Done():
			if !c.deaf {
				return fmt.Errorf("%w: %v", domain.ErrConversionFailed, ctx.Err())
			}
			<-c.gate
		}
	}
	if c.err != nil {
		return c.err
	}
	b, err := io.ReadAll(src)
	if err != nil {
		return fmt.Errorf("%w: %v", domain.ErrConversionFailed, err)
	}
	out := append([]byte("%PDF-fake:"), b...)
	if c.extra > 0 {
		out = append(out, make([]byte, c.extra)...)
	}
	if int64(len(out)) > max {
		return domain.ErrPDFTooLarge
	}
	_, err = dst.Write(out)
	return err
}

type previewFixture struct {
	t       *testing.T
	svc     *domain.Service
	pool    *pgxpool.Pool
	policy  *rulePolicyRead
	storage *fakeStorage
	conv    *fakeConverter
	ws      string
}

func newPreviewFixture(t *testing.T, cfg domain.PreviewConfig) *previewFixture {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), testDBURL())
	require.NoError(t, err)
	if err := pool.Ping(context.Background()); err != nil {
		pool.Close()
		t.Skipf("test DB not available: %v", err)
	}
	t.Cleanup(pool.Close)
	f := &previewFixture{
		t: t, pool: pool, policy: newRulePolicy(), storage: newFakeStorage(t),
		conv: &fakeConverter{}, ws: getTestWorkspaceID(t, pool),
	}
	f.svc = domain.NewService(store.NewStore(pool), f.policy, &mockPolicyWrite{}, f.storage)
	f.svc.SetConverter(f.conv, cfg)
	return f
}

// file stores an active file of data with the given name and recorded MIME
// type, readable by "reader" only.
func (f *previewFixture) file(name, mime string, data []byte) *store.DriveItem {
	f.t.Helper()
	id := uuid.New().String()
	key := "drive/" + id + "/" + name
	size := int64(len(data))
	item := &store.DriveItem{
		ID: id, WorkspaceID: f.ws, DriveContext: "workspace", ItemType: "file", Name: name,
		MimeType: &mime, SizeBytes: &size, ObjectKey: &key, NGACNodeID: "oa-" + id,
		OwnerID: getTestUserID(f.t, f.pool), Status: "active",
	}
	require.NoError(f.t, store.NewStore(f.pool).InsertItem(context.Background(), item))
	f.storage.seed(key, data, time.Now().Add(-time.Hour))
	f.policy.grant("reader", item.NGACNodeID, ngac.OpRead)
	return item
}

func reader() context.Context { return asCaller("", "reader") }

func TestPreview_AuthorizationMatchesDownload(t *testing.T) {
	f := newPreviewFixture(t, domain.PreviewConfig{})
	item := f.file("Q3.docx", "application/vnd.openxmlformats-officedocument.wordprocessingml.document", []byte("doc"))
	stranger := asCaller("", "stranger")

	_, dlErr := f.svc.GetDownloadURL(stranger, &pb.GetDownloadURLRequest{FileId: item.ID})
	_, pvErr := f.svc.GetPreview(stranger, item.ID)
	require.ErrorIs(t, dlErr, domain.ErrAccessDenied)
	require.ErrorIs(t, pvErr, domain.ErrAccessDenied)
	assert.Equal(t, dlErr.Error(), pvErr.Error(), "a preview must refuse in the words a download does")
	assert.Zero(t, f.conv.calls.Load(), "a caller who may not read must not cost a conversion")
	assert.Empty(t, f.storage.keysWithPrefix("drive/previews/"))

	// The right checked is read, on the file's own node.
	assert.Contains(t, f.policy.checks, [3]string{"stranger", item.NGACNodeID, ngac.OpRead})
}

func TestPreview_UnknownFileIsNotFoundAndFolderHasNoPreview(t *testing.T) {
	f := newPreviewFixture(t, domain.PreviewConfig{})
	f.policy.grant("reader", anyObject, ngac.OpRead)

	_, err := f.svc.GetPreview(reader(), uuid.New().String())
	assert.ErrorIs(t, err, domain.ErrNotFound)

	folder, err := f.svc.CreateFolder(reader(), &pb.CreateFolderRequest{WorkspaceId: f.ws, Name: "Hồ sơ"})
	require.NoError(t, err)
	_, err = f.svc.GetPreview(reader(), folder.Id)
	assert.ErrorIs(t, err, domain.ErrNotFound, "a folder has no content to show")
}

func TestPreview_PendingUploadHasNoPreview(t *testing.T) {
	f := newPreviewFixture(t, domain.PreviewConfig{})
	item := f.file("a.png", "image/png", []byte("x"))
	_, err := f.pool.Exec(context.Background(), `UPDATE drive_items SET status = 'pending' WHERE id = $1`, item.ID)
	require.NoError(t, err)
	_, err = f.svc.GetPreview(reader(), item.ID)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestPreview_NativeKinds(t *testing.T) {
	f := newPreviewFixture(t, domain.PreviewConfig{})
	for _, tc := range []struct{ name, recorded, wantMime string }{
		{"ảnh.png", "image/png", "image/png"},
		{"hợp đồng.pdf", "application/pdf", "application/pdf"},
		{"demo.mp4", "video/mp4", "video/mp4"},
		{"ghi âm.mp3", "audio/mpeg", "audio/mpeg"},
		{"ghi chú.txt", "text/plain; charset=utf-8", "text/plain"},
		{"cfg.json", "application/json", "application/json"},
		{"bảng.csv", "text/csv", "text/csv"},
		{"export.csv", "application/vnd.ms-excel", "text/csv"}, // mislabelled export
		{"scan.pdf", "application/octet-stream", "application/pdf"},
		{"README.md", "", "text/markdown"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			item := f.file(tc.name, tc.recorded, []byte("content"))
			p, err := f.svc.GetPreview(reader(), item.ID)
			require.NoError(t, err)
			assert.Equal(t, domain.PreviewNative, p.Kind)
			assert.Equal(t, tc.wantMime, p.Mime)
			assert.Equal(t, f.storage.srv.URL+"/get/"+*item.ObjectKey, p.URL, "the original, signed for reading")
		})
	}
	assert.Zero(t, f.conv.calls.Load(), "a natively shown file is never converted")
}

func TestPreview_UnsupportedTypes(t *testing.T) {
	f := newPreviewFixture(t, domain.PreviewConfig{})
	for _, tc := range []struct{ name, mime string }{
		{"lưu trữ.zip", "application/zip"},
		{"app.exe", "application/octet-stream"},
		{"no-extension", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			item := f.file(tc.name, tc.mime, []byte("x"))
			p, err := f.svc.GetPreview(reader(), item.ID)
			require.NoError(t, err)
			assert.Equal(t, &domain.Preview{Kind: domain.PreviewUnavailable, Reason: domain.ReasonUnsupported}, p)
		})
	}
	assert.Zero(t, f.conv.calls.Load())
}

const docxMime = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"

func TestPreview_OfficeIsConvertedOnceAndCached(t *testing.T) {
	f := newPreviewFixture(t, domain.PreviewConfig{})
	item := f.file("Báo cáo Q3.docx", docxMime, []byte("word-bytes"))

	p, err := f.svc.GetPreview(reader(), item.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.PreviewPDF, p.Kind)
	assert.Equal(t, docxMime, p.SourceMime)
	assert.Empty(t, p.Reason)

	keys := f.storage.keysWithPrefix("drive/previews/" + item.ID + "/")
	require.Len(t, keys, 1, "the rendition is stored under the previews prefix, keyed by the file")
	assert.Equal(t, f.storage.srv.URL+"/get/"+keys[0], p.URL)
	pdf, _ := f.storage.get(keys[0])
	assert.Equal(t, "%PDF-fake:word-bytes", string(pdf), "the converter got the file's bytes")
	assert.Equal(t, []string{"Báo cáo Q3.docx"}, f.conv.names, "the converter must see the real extension")

	// The second view is the cached one.
	p2, err := f.svc.GetPreview(reader(), item.ID)
	require.NoError(t, err)
	assert.Equal(t, p, p2)
	assert.EqualValues(t, 1, f.conv.calls.Load(), "a cached rendition is reused")
	assert.Equal(t, 1, f.storage.puts)
}

func TestPreview_ReplacedContentGetsANewRendition(t *testing.T) {
	f := newPreviewFixture(t, domain.PreviewConfig{})
	item := f.file("plan.pptx", "", []byte("v1"))
	first, err := f.svc.GetPreview(reader(), item.ID)
	require.NoError(t, err)

	f.storage.seed(*item.ObjectKey, []byte("v2-longer"), time.Now())
	second, err := f.svc.GetPreview(reader(), item.ID)
	require.NoError(t, err)

	assert.NotEqual(t, first.URL, second.URL, "the old rendition must not be served for new content")
	assert.EqualValues(t, 2, f.conv.calls.Load())
	assert.Equal(t, "pptx", strings.TrimPrefix(f.conv.names[1], "plan."))
}

func TestPreview_OfficeKnownOnlyByMimeGetsAnExtension(t *testing.T) {
	f := newPreviewFixture(t, domain.PreviewConfig{})
	item := f.file("scan-no-ext", "application/msword", []byte("x"))
	p, err := f.svc.GetPreview(reader(), item.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.PreviewPDF, p.Kind)
	assert.Equal(t, []string{"scan-no-ext.doc"}, f.conv.names)
}

func TestPreview_ConversionFailureIsAnAnswerNotAnError(t *testing.T) {
	f := newPreviewFixture(t, domain.PreviewConfig{})
	f.conv.err = fmt.Errorf("%w: corrupt file", domain.ErrConversionFailed)
	item := f.file("hỏng.docx", docxMime, []byte("x"))

	p, err := f.svc.GetPreview(reader(), item.ID)
	require.NoError(t, err)
	assert.Equal(t, &domain.Preview{Kind: domain.PreviewUnavailable, Reason: domain.ReasonConversionFailed}, p)
	assert.Empty(t, f.storage.keysWithPrefix("drive/previews/"), "a failure leaves no rendition behind")

	// Remembered: a bad file is not run through the converter on every view.
	f.conv.err = nil
	p, err = f.svc.GetPreview(reader(), item.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.ReasonConversionFailed, p.Reason)
	assert.EqualValues(t, 1, f.conv.calls.Load())
}

func TestPreview_FailureIsForgottenAfterItsTime(t *testing.T) {
	f := newPreviewFixture(t, domain.PreviewConfig{FailureTTL: 150 * time.Millisecond})
	f.conv.err = fmt.Errorf("%w: corrupt", domain.ErrConversionFailed)
	item := f.file("hỏng.docx", docxMime, []byte("x"))
	_, err := f.svc.GetPreview(reader(), item.ID)
	require.NoError(t, err)

	f.conv.err = nil
	time.Sleep(250 * time.Millisecond)
	p, err := f.svc.GetPreview(reader(), item.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.PreviewPDF, p.Kind, "after its time the file is tried again")
}

func TestPreview_AnOutageIsNotRemembered(t *testing.T) {
	f := newPreviewFixture(t, domain.PreviewConfig{})
	f.conv.err = fmt.Errorf("%w: refused", domain.ErrConverterUnreachable)
	item := f.file("a.docx", docxMime, []byte("x"))
	_, err := f.svc.GetPreview(reader(), item.ID)
	require.NoError(t, err)

	f.conv.err = nil
	p, err := f.svc.GetPreview(reader(), item.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.PreviewPDF, p.Kind, "the converter coming back is noticed at once")
}

func TestPreview_UnreachableConverterIsUnavailableNotAnError(t *testing.T) {
	f := newPreviewFixture(t, domain.PreviewConfig{})
	f.conv.err = fmt.Errorf("%w: connection refused", domain.ErrConverterUnreachable)
	item := f.file("a.pptx", "", []byte("x"))
	p, err := f.svc.GetPreview(reader(), item.ID)
	require.NoError(t, err)
	assert.Equal(t, &domain.Preview{Kind: domain.PreviewUnavailable, Reason: domain.ReasonConversionUnavailable}, p)
}

func TestPreview_NoConverterConfigured(t *testing.T) {
	f := newPreviewFixture(t, domain.PreviewConfig{})
	svc := domain.NewService(store.NewStore(f.pool), f.policy, &mockPolicyWrite{}, f.storage)
	item := f.file("a.docx", docxMime, []byte("x"))
	p, err := svc.GetPreview(reader(), item.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.ReasonConversionUnavailable, p.Reason)
}

func TestPreview_TooLargeIsNotConverted(t *testing.T) {
	f := newPreviewFixture(t, domain.PreviewConfig{MaxConvertBytes: 10})

	t.Run("by the recorded size", func(t *testing.T) {
		item := f.file("big.docx", docxMime, []byte("0123456789ABCDEF"))
		p, err := f.svc.GetPreview(reader(), item.ID)
		require.NoError(t, err)
		assert.Equal(t, &domain.Preview{Kind: domain.PreviewUnavailable, Reason: domain.ReasonTooLarge}, p)
	})
	t.Run("by the stored size when the record is stale", func(t *testing.T) {
		item := f.file("grew.docx", docxMime, []byte("tiny"))
		f.storage.seed(*item.ObjectKey, []byte("0123456789ABCDEF"), time.Now())
		p, err := f.svc.GetPreview(reader(), item.ID)
		require.NoError(t, err)
		assert.Equal(t, domain.ReasonTooLarge, p.Reason)
	})
	assert.Zero(t, f.conv.calls.Load())

	// The limit does not apply to what the browser shows itself.
	item := f.file("big.pdf", "application/pdf", []byte("0123456789ABCDEF"))
	p, err := f.svc.GetPreview(reader(), item.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.PreviewNative, p.Kind)
}

func TestPreview_ConversionTimeout(t *testing.T) {
	f := newPreviewFixture(t, domain.PreviewConfig{ConvertTimeout: 100 * time.Millisecond})
	f.conv.gate = make(chan struct{}) // never opened: the converter hangs
	item := f.file("slow.docx", docxMime, []byte("x"))

	start := time.Now()
	p, err := f.svc.GetPreview(reader(), item.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.ReasonConversionFailed, p.Reason)
	assert.Less(t, time.Since(start), 5*time.Second)
}

func TestPreview_ConcurrentFirstViewsConvertOnce(t *testing.T) {
	f := newPreviewFixture(t, domain.PreviewConfig{})
	f.conv.gate = make(chan struct{})
	item := f.file("shared.docx", docxMime, []byte("x"))

	const viewers = 8
	results := make([]*domain.Preview, viewers)
	errs := make([]error, viewers)
	var wg sync.WaitGroup
	for i := 0; i < viewers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i], errs[i] = f.svc.GetPreview(reader(), item.ID)
		}()
	}
	// Let every viewer reach the converter's door before it opens.
	require.Eventually(t, func() bool { return f.conv.calls.Load() >= 1 }, 5*time.Second, 5*time.Millisecond)
	time.Sleep(200 * time.Millisecond)
	close(f.conv.gate)
	wg.Wait()

	for i := range results {
		require.NoError(t, errs[i])
		assert.Equal(t, domain.PreviewPDF, results[i].Kind)
	}
	assert.EqualValues(t, 1, f.conv.calls.Load(), "simultaneous first views share one conversion")
	assert.Equal(t, 1, f.storage.puts)
}

func TestPreview_StorageOutageIsAnErrorNotAnAnswer(t *testing.T) {
	f := newPreviewFixture(t, domain.PreviewConfig{})
	item := f.file("a.png", "image/png", []byte("x"))
	broken := &brokenStorage{fakeStorage: f.storage}
	svc := domain.NewService(store.NewStore(f.pool), f.policy, &mockPolicyWrite{}, broken)
	_, err := svc.GetPreview(reader(), item.ID)
	require.Error(t, err)
	assert.False(t, errors.Is(err, domain.ErrNotFound) || errors.Is(err, domain.ErrAccessDenied))
}

type brokenStorage struct{ *fakeStorage }

func (*brokenStorage) GetDownloadURL(context.Context, *docpb.GetDownloadURLRequest, ...grpc.CallOption) (*docpb.GetDownloadURLResponse, error) {
	return nil, status.Error(codes.Unavailable, "document service down")
}

func TestPreview_RenditionOverTheCapIsTooLargeAndRemembered(t *testing.T) {
	f := newPreviewFixture(t, domain.PreviewConfig{MaxPDFBytes: 64})
	f.conv.extra = 1000
	item := f.file("deck.pptx", "", []byte("slides"))

	p, err := f.svc.GetPreview(reader(), item.ID)
	require.NoError(t, err)
	assert.Equal(t, &domain.Preview{Kind: domain.PreviewUnavailable, Reason: domain.ReasonTooLarge}, p)
	assert.EqualValues(t, 64, f.conv.max, "the converter is told the cap")
	assert.Empty(t, f.storage.keysWithPrefix("drive/previews/"))

	_, err = f.svc.GetPreview(reader(), item.ID)
	require.NoError(t, err)
	assert.EqualValues(t, 1, f.conv.calls.Load(), "too large is remembered like any failed conversion")
}

func TestPreview_RenditionIsSpooledToDiskAndTheTempFileRemoved(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	f := newPreviewFixture(t, domain.PreviewConfig{})
	item := f.file("a.docx", docxMime, []byte("x"))
	_, err := f.svc.GetPreview(reader(), item.ID)
	require.NoError(t, err)
	left, _ := os.ReadDir(tmp)
	assert.Empty(t, left, "no temporary file is left behind")

	f.conv.err = fmt.Errorf("%w: boom", domain.ErrConversionFailed)
	item2 := f.file("b.docx", docxMime, []byte("y"))
	_, err = f.svc.GetPreview(reader(), item2.ID)
	require.NoError(t, err)
	left, _ = os.ReadDir(tmp)
	assert.Empty(t, left, "nor after a failure")
}

func TestPreview_ConversionsAreLimitedAndAWaiterGivesUpBusy(t *testing.T) {
	f := newPreviewFixture(t, domain.PreviewConfig{MaxConcurrent: 1, ConvertTimeout: 400 * time.Millisecond})
	f.conv.gate = make(chan struct{})
	f.conv.deaf = true
	a := f.file("a.docx", docxMime, []byte("a"))
	b := f.file("b.docx", docxMime, []byte("b"))

	done := make(chan *domain.Preview, 1)
	go func() {
		p, _ := f.svc.GetPreview(reader(), a.ID)
		done <- p
	}()
	require.Eventually(t, func() bool { return f.conv.calls.Load() == 1 }, 5*time.Second, 5*time.Millisecond)

	p, err := f.svc.GetPreview(reader(), b.ID)
	require.NoError(t, err)
	assert.Equal(t, &domain.Preview{Kind: domain.PreviewUnavailable, Reason: domain.ReasonConversionBusy}, p)
	assert.EqualValues(t, 1, f.conv.calls.Load(), "the second document never reached the converter")
	close(f.conv.gate) // a finishes: its slot is released
	<-done

	// Busy is not a verdict on the file: with a free slot it converts.
	f.conv.gate = nil
	p, err = f.svc.GetPreview(reader(), b.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.PreviewPDF, p.Kind)
}

func TestPreview_StorageIsReachedAtTheDialOverrideWithThePublicHost(t *testing.T) {
	f := newPreviewFixture(t, domain.PreviewConfig{})
	f.storage.urlBase = "https://storage.example.test"
	f.svc.SetConverter(f.conv, domain.PreviewConfig{StorageDialAddr: f.storage.srv.Listener.Addr().String()})
	item := f.file("a.docx", docxMime, []byte("x"))

	p, err := f.svc.GetPreview(reader(), item.ID)
	require.NoError(t, err)
	require.Equal(t, domain.PreviewPDF, p.Kind, "the https URL was fetched straight from the override, in plain http")

	f.storage.mu.Lock()
	defer f.storage.mu.Unlock()
	require.NotEmpty(t, f.storage.hosts)
	for _, h := range f.storage.hosts {
		assert.Equal(t, "storage.example.test", h, "the signed host is sent unchanged")
	}
	assert.Equal(t, 1, f.storage.puts)
}

func TestPreview_ScriptCapableTypesAreShownAsText(t *testing.T) {
	f := newPreviewFixture(t, domain.PreviewConfig{})
	for _, tc := range []struct{ name, mime string }{
		{"page.html", "text/html"}, {"page.html", ""}, {"logo.svg", "image/svg+xml"}, {"logo.svg", "application/octet-stream"},
		{"feed.xml", "application/xml"}, {"feed.xml", "text/xml"}, {"x.xhtml", "application/xhtml+xml"},
		{"m.bin", "application/atom+xml"},
	} {
		t.Run(tc.name+"/"+tc.mime, func(t *testing.T) {
			item := f.file(tc.name, tc.mime, []byte("<script>alert(1)</script>"))
			p, err := f.svc.GetPreview(reader(), item.ID)
			require.NoError(t, err)
			assert.Equal(t, domain.PreviewNative, p.Kind)
			assert.Equal(t, "text/plain", p.Mime, "never reported as something a browser runs")
		})
	}
}

func TestPreview_ViewerLeavingDoesNotCancelTheSharedConversion(t *testing.T) {
	f := newPreviewFixture(t, domain.PreviewConfig{})
	f.conv.gate = make(chan struct{})
	item := f.file("shared.docx", docxMime, []byte("x"))

	ctxA, leave := context.WithCancel(reader())
	go func() { _, _ = f.svc.GetPreview(ctxA, item.ID) }()
	require.Eventually(t, func() bool { return f.conv.calls.Load() == 1 }, 5*time.Second, 5*time.Millisecond)

	res := make(chan *domain.Preview, 1)
	go func() {
		p, _ := f.svc.GetPreview(reader(), item.ID)
		res <- p
	}()
	time.Sleep(100 * time.Millisecond)
	leave()
	time.Sleep(50 * time.Millisecond)
	close(f.conv.gate)

	select {
	case p := <-res:
		require.NotNil(t, p)
		assert.Equal(t, domain.PreviewPDF, p.Kind)
	case <-time.After(5 * time.Second):
		t.Fatal("the remaining viewer never got its preview")
	}
}

func TestPreview_PermanentDeleteRemovesTheRendition(t *testing.T) {
	f := newPreviewFixture(t, domain.PreviewConfig{})
	item := f.file("secret.docx", docxMime, []byte("x"))
	f.policy.grant("reader", item.NGACNodeID, ngac.OpWrite)
	_, err := f.svc.GetPreview(reader(), item.ID)
	require.NoError(t, err)
	require.Len(t, f.storage.keysWithPrefix("drive/previews/"+item.ID+"/"), 1)

	_, err = f.svc.DeleteItem(reader(), &pb.DeleteItemRequest{ItemId: item.ID})
	require.NoError(t, err)
	assert.Empty(t, f.storage.keysWithPrefix("drive/previews/"+item.ID+"/"), "the file's text does not outlive it as a PDF")
	_, original := f.storage.get(*item.ObjectKey)
	assert.False(t, original)
}

func TestPreview_SpreadsheetsAreSheetsNotConverted(t *testing.T) {
	f := newPreviewFixture(t, domain.PreviewConfig{})
	xlsx := "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	for _, tc := range []struct{ name, recorded, wantMime string }{
		{"số liệu.xlsx", xlsx, xlsx},
		{"cũ.xls", "application/vnd.ms-excel", "application/vnd.ms-excel"},
		{"bảng.ods", "", "application/vnd.oasis.opendocument.spreadsheet"},
		{"no-extension", "application/vnd.ms-excel", "application/vnd.ms-excel"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			item := f.file(tc.name, tc.recorded, []byte("cells"))
			p, err := f.svc.GetPreview(reader(), item.ID)
			require.NoError(t, err)
			assert.Equal(t, domain.PreviewSheet, p.Kind)
			assert.Equal(t, tc.wantMime, p.Mime)
			assert.Equal(t, f.storage.srv.URL+"/get/"+*item.ObjectKey, p.URL, "the original, signed for reading")
		})
	}
	assert.Zero(t, f.conv.calls.Load(), "a spreadsheet is never converted")
	assert.Empty(t, f.storage.keysWithPrefix("drive/previews/"))

	// Same right as a download.
	item := f.file("kín.xlsx", xlsx, []byte("x"))
	_, err := f.svc.GetPreview(asCaller("", "stranger"), item.ID)
	assert.ErrorIs(t, err, domain.ErrAccessDenied)
}
