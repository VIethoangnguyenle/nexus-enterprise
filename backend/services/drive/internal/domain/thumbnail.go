package domain

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	docpb "ngac-platform/proto/document"
	"ngac-platform/services/drive/internal/store"
	"ngac-platform/services/drive/internal/thumbnail"
)

// ThumbnailSizes are the longest sides, in pixels, a thumbnail is made in.
var ThumbnailSizes = []int{320, 640}

// Defaults for ThumbnailConfig.
const (
	DefaultThumbConcurrent  = 4
	DefaultThumbQueueWait   = 5 * time.Second
	DefaultThumbFetchTime   = 20 * time.Second
	DefaultThumbRenderTime  = 10 * time.Second
	DefaultThumbFailureTTL  = 10 * time.Minute
	DefaultThumbMaxImage    = 25 << 20
	DefaultThumbMaxPDF      = 50 << 20
	DefaultThumbMaxVideo    = 100 << 20
	maxThumbnailBytes       = 4 << 20
	thumbnailStoreTimeLimit = 10 * time.Second
)

var (
	// ErrThumbnailUnavailable: this file has no thumbnail — its kind is not one
	// that has them, it is too large to draw one from, or drawing failed. The
	// client shows the file's icon.
	ErrThumbnailUnavailable = errors.New("thumbnail unavailable")
	// ErrThumbnailBusy: no drawing slot freed up in time. Transient; the client
	// may ask again shortly.
	ErrThumbnailBusy = errors.New("thumbnail busy")
	// errRender: the file was fetched and the tool could not draw it. Only this is
	// remembered; a conversion or storage hiccup is tried again next time.
	errRender = errors.New("render failed")
)

// Thumbnail is a file's thumbnail, or, when NotModified, the confirmation that
// the copy the client holds (ETag) is still current.
type Thumbnail struct {
	Data        []byte
	ETag        string
	NotModified bool
}

// ThumbnailConfig bounds the drawing of thumbnails.
type ThumbnailConfig struct {
	// MaxConcurrent is how many thumbnails are drawn at once in this process.
	MaxConcurrent int
	// QueueWait is how long a request waits for a slot before it is ErrThumbnailBusy.
	QueueWait time.Duration
	// FetchTimeout bounds fetching the source; RenderTimeout bounds each external
	// tool run.
	FetchTimeout  time.Duration
	RenderTimeout time.Duration
	// FailureTTL is how long a file that could not be drawn is left alone.
	FailureTTL time.Duration
	// MaxImageBytes, MaxPDFBytes and MaxVideoBytes are the largest sources drawn
	// from (a PDF rendition of an Office file is bounded by the preview limit).
	MaxImageBytes, MaxPDFBytes, MaxVideoBytes int64
	// StorageDialAddr is as for PreviewConfig.
	StorageDialAddr string
}

type thumbnailer struct {
	r      thumbnail.Renderer
	cfg    ThumbnailConfig
	http   *http.Client
	direct bool
	slots  chan struct{}
	sf     singleflight.Group

	mu       sync.Mutex
	failures map[string]time.Time
}

// SetThumbnailer enables thumbnails, drawn by r. Without one every file is
// reported ErrThumbnailUnavailable.
func (s *Service) SetThumbnailer(r thumbnail.Renderer, cfg ThumbnailConfig) {
	if cfg.MaxConcurrent <= 0 {
		cfg.MaxConcurrent = DefaultThumbConcurrent
	}
	if cfg.QueueWait <= 0 {
		cfg.QueueWait = DefaultThumbQueueWait
	}
	if cfg.FetchTimeout <= 0 {
		cfg.FetchTimeout = DefaultThumbFetchTime
	}
	if cfg.RenderTimeout <= 0 {
		cfg.RenderTimeout = DefaultThumbRenderTime
	}
	if cfg.FailureTTL <= 0 {
		cfg.FailureTTL = DefaultThumbFailureTTL
	}
	if cfg.MaxImageBytes <= 0 {
		cfg.MaxImageBytes = DefaultThumbMaxImage
	}
	if cfg.MaxPDFBytes <= 0 {
		cfg.MaxPDFBytes = DefaultThumbMaxPDF
	}
	if cfg.MaxVideoBytes <= 0 {
		cfg.MaxVideoBytes = DefaultThumbMaxVideo
	}
	client, direct := storageClient(cfg.StorageDialAddr)
	s.thumbs = &thumbnailer{
		r: r, cfg: cfg, http: client, direct: direct,
		slots: make(chan struct{}, cfg.MaxConcurrent), failures: map[string]time.Time{},
	}
}

func (t *thumbnailer) failed(key string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	until, ok := t.failures[key]
	if ok && time.Now().After(until) {
		delete(t.failures, key)
		return false
	}
	return ok
}

func (t *thumbnailer) remember(key string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := time.Now()
	if len(t.failures) >= 4096 {
		for k, until := range t.failures {
			if now.After(until) {
				delete(t.failures, k)
			}
		}
	}
	t.failures[key] = now.Add(t.cfg.FailureTTL)
}

// thumbSource is how a file's thumbnail is drawn.
type thumbSource struct {
	kind     thumbnail.Kind
	office   bool // drawn from the PDF rendition
	convName string
	maxBytes int64
}

// thumbSourceFor decides how a file is drawn, or false when its kind has no
// thumbnail.
func (t *thumbnailer) thumbSourceFor(name, mimeType string) (thumbSource, bool) {
	class, mime, convName := classifyPreview(name, mimeType)
	switch class {
	case previewNative:
		switch {
		case mime == "image/jpeg", mime == "image/png", mime == "image/gif", mime == "image/webp":
			return thumbSource{kind: thumbnail.Image, maxBytes: t.cfg.MaxImageBytes}, true
		case mime == "application/pdf":
			return thumbSource{kind: thumbnail.PDF, maxBytes: t.cfg.MaxPDFBytes}, true
		case strings.HasPrefix(mime, "video/"):
			return thumbSource{kind: thumbnail.Video, maxBytes: t.cfg.MaxVideoBytes}, true
		}
	case previewOffice:
		return thumbSource{kind: thumbnail.PDF, office: true, convName: convName}, true
	case previewSheet:
		// Spreadsheets are shown as a table, not converted, but a thumbnail is
		// drawn from the same conversion an Office file gets.
		if _, ok := sheetExt[strings.TrimPrefix(strings.ToLower(path.Ext(name)), ".")]; ok {
			return thumbSource{kind: thumbnail.PDF, office: true, convName: name}, true
		}
		if ext, ok := officeMimeExt[mime]; ok {
			return thumbSource{kind: thumbnail.PDF, office: true, convName: name + "." + ext}, true
		}
	}
	return thumbSource{}, false
}

// thumbnailVersion names the content a thumbnail was drawn from (when it was
// stored and how big it is), so replacing the content makes a new key.
func thumbnailVersion(src *docpb.ObjectInfo) string {
	return fmt.Sprintf("%d-%d", src.LastModified.AsTime().UnixNano(), src.SizeBytes)
}

func thumbnailETag(version string, size int) string {
	return fmt.Sprintf(`"t%d-%s"`, size, version)
}

func thumbnailDocID(fileID, version string) string { return "thumbs/" + fileID + "/" + version }

func thumbnailFilename(size int) string { return fmt.Sprintf("%d.jpg", size) }

func validThumbnailSize(size int) bool {
	for _, s := range ThumbnailSizes {
		if s == size {
			return true
		}
	}
	return false
}

// GetThumbnail returns a JPEG of file fileID whose longest side is size
// (ThumbnailSizes), drawn on first request and kept in storage. The right it
// takes, and how it refuses, is the preview's: read on the file, checked before
// anything about the file is looked at. ifNoneMatch is the client's ETag, if it
// has a copy.
func (s *Service) GetThumbnail(ctx context.Context, fileID string, size int, ifNoneMatch string) (*Thumbnail, error) {
	if !validThumbnailSize(size) {
		return nil, invalid("size must be one of %v", ThumbnailSizes)
	}
	item, err := s.readableFile(ctx, fileID)
	if err != nil {
		return nil, err
	}
	t := s.thumbs
	if t == nil {
		return nil, ErrThumbnailUnavailable
	}
	var mimeType string
	if item.MimeType != nil {
		mimeType = *item.MimeType
	}
	src, ok := t.thumbSourceFor(item.Name, mimeType)
	if !ok {
		return nil, ErrThumbnailUnavailable
	}
	if src.maxBytes > 0 && item.SizeBytes != nil && *item.SizeBytes > src.maxBytes {
		return nil, ErrThumbnailUnavailable
	}

	info, err := s.docStorage.GetObjectInfo(ctx, &docpb.GetObjectInfoRequest{WorkspaceId: item.WorkspaceID, ObjectKey: *item.ObjectKey})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, notFound("file not found")
		}
		return nil, fmt.Errorf("stat source: %w", err)
	}
	if src.maxBytes > 0 && info.SizeBytes > src.maxBytes {
		return nil, ErrThumbnailUnavailable
	}
	version := thumbnailVersion(info)
	etag := thumbnailETag(version, size)
	if ifNoneMatch != "" && etagMatches(ifNoneMatch, etag) {
		return &Thumbnail{ETag: etag, NotModified: true}, nil
	}

	slot, err := s.docStorage.GetUploadURL(ctx, &docpb.GetUploadURLRequest{
		WorkspaceId: item.WorkspaceID, DocId: thumbnailDocID(item.ID, version),
		Filename: thumbnailFilename(size), MimeType: "image/jpeg",
	})
	if err != nil {
		return nil, fmt.Errorf("get thumbnail slot: %w", err)
	}

	if data, ok := s.cachedThumbnail(ctx, t, item.WorkspaceID, slot.ObjectKey); ok {
		return &Thumbnail{Data: data, ETag: etag}, nil
	}
	if t.failed(slot.ObjectKey) {
		return nil, ErrThumbnailUnavailable
	}

	// Concurrent first views of a file share one drawing. It runs on its own
	// context: the viewer that happened to start it going away must not fail the
	// others.
	v, err, _ := t.sf.Do(slot.ObjectKey, func() (any, error) {
		return s.drawThumbnail(ctx, t, item, src, size, slot)
	})
	if err != nil {
		if errors.Is(err, ErrThumbnailBusy) {
			slog.Warn("thumbnail: no drawing slot free in time", "file", item.ID)
			return nil, err
		}
		slog.Warn("thumbnail: not drawn", "file", item.ID, "kind", src.kind.String(), "size", size, "error", err)
		if errors.Is(err, errRender) {
			t.remember(slot.ObjectKey)
		}
		return nil, ErrThumbnailUnavailable
	}
	return &Thumbnail{Data: v.([]byte), ETag: etag}, nil
}

// etagMatches reports whether the If-None-Match header names etag (or is *).
func etagMatches(header, etag string) bool {
	for _, c := range strings.Split(header, ",") {
		c = strings.TrimPrefix(strings.TrimSpace(c), "W/")
		if c == "*" || c == etag {
			return true
		}
	}
	return false
}

// cachedThumbnail reads a stored thumbnail. Not there, or not readable, is
// "not cached": the thumbnail is drawn again.
func (s *Service) cachedThumbnail(ctx context.Context, t *thumbnailer, workspaceID, key string) ([]byte, bool) {
	exists, err := s.objectExists(ctx, workspaceID, key)
	if err != nil || !exists {
		if err != nil {
			slog.Warn("thumbnail: cache lookup failed", "key", key, "error", err)
		}
		return nil, false
	}
	fctx, cancel := context.WithTimeout(ctx, t.cfg.FetchTimeout)
	defer cancel()
	url, err := s.downloadURL(fctx, workspaceID, key)
	if err == nil {
		var b []byte
		if b, err = t.get(fctx, url, maxThumbnailBytes); err == nil {
			return b, true
		}
	}
	slog.Warn("thumbnail: stored copy not read", "key", key, "error", err)
	return nil, false
}

// get fetches a signed storage URL, at most limit bytes.
func (t *thumbnailer) get(ctx context.Context, signed string, limit int64) ([]byte, error) {
	return t.fetch(ctx, signed, limit, io.ReadAll)
}

// fetch requests a signed storage URL and hands the body, bounded to limit
// bytes, to read. A body longer than limit is an error.
func (t *thumbnailer) fetch(ctx context.Context, signed string, limit int64, read func(io.Reader) ([]byte, error)) ([]byte, error) {
	u, err := directURL(signed, t.direct)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := t.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("storage answered %d", resp.StatusCode)
	}
	if resp.ContentLength > limit {
		return nil, fmt.Errorf("source is %d bytes", resp.ContentLength)
	}
	return read(io.LimitReader(resp.Body, limit+1))
}

// fetchToFile saves a signed storage URL to path, at most limit bytes.
func (t *thumbnailer) fetchToFile(ctx context.Context, signed, path string, limit int64) error {
	_, err := t.fetch(ctx, signed, limit, func(r io.Reader) ([]byte, error) {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o600)
		if err != nil {
			return nil, err
		}
		n, err := io.Copy(f, r)
		if cerr := f.Close(); err == nil {
			err = cerr
		}
		if err == nil && n > limit {
			err = fmt.Errorf("source is longer than %d bytes", limit)
		}
		return nil, err
	})
	return err
}

// drawThumbnail makes the thumbnail, keeps it in slot and returns it. Failing to
// keep it is logged, not an error: the viewer still gets the picture.
func (s *Service) drawThumbnail(callCtx context.Context, t *thumbnailer, item *store.DriveItem, src thumbSource, size int, slot *docpb.GetUploadURLResponse) ([]byte, error) {
	budget := t.cfg.QueueWait + t.cfg.FetchTimeout + t.cfg.RenderTimeout + thumbnailStoreTimeLimit
	if src.office && s.preview != nil {
		budget += s.preview.cfg.ConvertTimeout
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(callCtx), budget)
	defer cancel()

	// The source is the file itself, or for an Office file its PDF rendition
	// (made, or found, by the preview's conversion with its own limits).
	var srcURL string
	maxBytes := src.maxBytes
	if src.office {
		var mimeType string
		if item.MimeType != nil {
			mimeType = *item.MimeType
		}
		p, err := s.officePreview(ctx, item.WorkspaceID, item.ID, *item.ObjectKey, src.convName, mimeType, item.SizeBytes)
		if err != nil {
			return nil, fmt.Errorf("office rendition: %w", err)
		}
		if p.Kind != PreviewPDF {
			return nil, fmt.Errorf("office rendition unavailable: %s", p.Reason)
		}
		srcURL, maxBytes = p.URL, s.preview.cfg.MaxPDFBytes
	} else {
		var err error
		if srcURL, err = s.downloadURL(ctx, item.WorkspaceID, *item.ObjectKey); err != nil {
			return nil, err
		}
	}

	select {
	case t.slots <- struct{}{}:
		defer func() { <-t.slots }()
	case <-time.After(t.cfg.QueueWait):
		return nil, ErrThumbnailBusy
	case <-ctx.Done():
		return nil, ErrThumbnailBusy
	}

	dir, err := os.MkdirTemp("", "nexus-thumb-src-*")
	if err != nil {
		return nil, fmt.Errorf("temp dir: %w", err)
	}
	defer os.RemoveAll(dir)
	srcPath := filepath.Join(dir, "source")
	fctx, fcancel := context.WithTimeout(ctx, t.cfg.FetchTimeout)
	err = t.fetchToFile(fctx, srcURL, srcPath, maxBytes)
	fcancel()
	if err != nil {
		return nil, fmt.Errorf("fetch source: %w", err)
	}

	rctx, rcancel := context.WithTimeout(ctx, t.cfg.RenderTimeout)
	data, err := t.r.Render(rctx, src.kind, srcPath, size)
	rcancel()
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errRender, err)
	}
	if len(data) == 0 || len(data) > maxThumbnailBytes {
		return nil, fmt.Errorf("%w: unusable result of %d bytes", errRender, len(data))
	}

	if err := s.storeThumbnail(ctx, t, slot, data); err != nil {
		slog.Warn("thumbnail: not kept", "file", item.ID, "error", err)
	}
	return data, nil
}

func (s *Service) storeThumbnail(ctx context.Context, t *thumbnailer, slot *docpb.GetUploadURLResponse, data []byte) error {
	putURL, err := directURL(slot.UploadUrl, t.direct)
	if err != nil {
		return err
	}
	sctx, cancel := context.WithTimeout(ctx, thumbnailStoreTimeLimit)
	defer cancel()
	req, err := http.NewRequestWithContext(sctx, http.MethodPut, putURL, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "image/jpeg")
	resp, err := t.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("storage answered %d", resp.StatusCode)
	}
	return nil
}

// removeThumbnails deletes the thumbnails of a file about to be deleted for
// good, so its picture does not outlive it. Their keys follow from the
// original's stored-at time and size, read before the original goes. Best
// effort: a failure is logged, never the deletion's.
func (s *Service) removeThumbnails(ctx context.Context, f *store.DriveItem) {
	if f.ObjectKey == nil || s.thumbs == nil {
		return
	}
	var mimeType string
	if f.MimeType != nil {
		mimeType = *f.MimeType
	}
	// A kind that has no thumbnail has none to remove: no lookup for it.
	if _, ok := s.thumbs.thumbSourceFor(f.Name, mimeType); !ok {
		return
	}
	info, err := s.docStorage.GetObjectInfo(ctx, &docpb.GetObjectInfoRequest{WorkspaceId: f.WorkspaceID, ObjectKey: *f.ObjectKey})
	if err != nil {
		if status.Code(err) != codes.NotFound {
			slog.Error("deleted file's thumbnails not removed", "item", f.ID, "err", err)
		}
		return
	}
	version := thumbnailVersion(info)
	for _, size := range ThumbnailSizes {
		slot, err := s.docStorage.GetUploadURL(ctx, &docpb.GetUploadURLRequest{
			WorkspaceId: f.WorkspaceID, DocId: thumbnailDocID(f.ID, version),
			Filename: thumbnailFilename(size), MimeType: "image/jpeg",
		})
		if err == nil {
			_, err = s.docStorage.DeleteObject(ctx, &docpb.DeleteObjectRequest{WorkspaceId: f.WorkspaceID, ObjectKey: slot.ObjectKey})
		}
		if err != nil {
			slog.Error("deleted file's thumbnail not removed", "item", f.ID, "size", size, "err", err)
		}
	}
}
