package domain

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"ngac-platform/ngac"
	"ngac-platform/pkg/grpcauth"
	docpb "ngac-platform/proto/document"
	"ngac-platform/services/drive/internal/store"
)

// Preview kinds and the reasons a file has none.
const (
	PreviewNative      = "native"
	PreviewPDF         = "pdf"
	PreviewSheet       = "sheet"
	PreviewUnavailable = "unavailable"

	ReasonUnsupported           = "unsupported"
	ReasonTooLarge              = "too_large"
	ReasonConversionFailed      = "conversion_failed"
	ReasonConversionUnavailable = "conversion_unavailable"
	ReasonConversionBusy        = "conversion_busy"
)

// Preview is what the client needs to show a file. Kind native: URL is the
// original, shown by the browser as Mime. Kind pdf: URL is a PDF rendition of a
// file of type SourceMime. Kind unavailable: there is nothing to show, Reason
// says why, and the client offers the download instead.
type Preview struct {
	Kind       string `json:"kind"`
	URL        string `json:"url,omitempty"`
	Mime       string `json:"mime,omitempty"`
	SourceMime string `json:"source_mime,omitempty"`
	Reason     string `json:"reason,omitempty"`
}

// Converter turns an Office document into PDF.
type Converter interface {
	// ToPDF converts src, whose file name is filename, writing the PDF to dst. At
	// most maxBytes are written: a longer PDF is ErrPDFTooLarge. It fails with
	// ErrConverterUnreachable when the converter cannot be reached and with
	// ErrConversionFailed when it can but the document cannot be converted.
	ToPDF(ctx context.Context, filename string, src io.Reader, dst io.Writer, maxBytes int64) error
}

var (
	// ErrConverterUnreachable: the conversion server did not answer.
	ErrConverterUnreachable = errors.New("converter unreachable")
	// ErrConversionFailed: the document could not be converted (or in time).
	ErrConversionFailed = errors.New("conversion failed")
	// ErrPDFTooLarge: the PDF would be larger than a rendition may be.
	ErrPDFTooLarge = errors.New("converted document too large")
	// errConversionBusy: no conversion slot freed up in time.
	errConversionBusy = errors.New("conversion busy")
)

// Defaults for PreviewConfig.
const (
	DefaultMaxConvertBytes = 50 << 20
	DefaultMaxPDFBytes     = 80 << 20
	DefaultMaxConcurrent   = 2
	// DefaultConvertTimeout stays under the edge's 60 s read timeout, so a slow
	// conversion is answered "unavailable" instead of a gateway error.
	DefaultConvertTimeout = 45 * time.Second
	DefaultFailureTTL     = 10 * time.Minute
)

// PreviewConfig bounds the conversion of Office files.
type PreviewConfig struct {
	// MaxConvertBytes is the largest source file that is converted.
	MaxConvertBytes int64
	// MaxPDFBytes is the largest rendition that is kept.
	MaxPDFBytes int64
	// MaxConcurrent is how many conversions run at once in this process; the
	// rest wait for a slot until their time is up.
	MaxConcurrent int
	// ConvertTimeout bounds one conversion: waiting for a slot, fetching the
	// source, converting it and storing the PDF.
	ConvertTimeout time.Duration
	// FailureTTL is how long a file that could not be converted is left alone
	// before it is tried again.
	FailureTTL time.Duration
	// StorageDialAddr, when set, is where the signed storage URLs are really
	// fetched: the connection goes to this host:port over plain http while the
	// URL's host (which the signature covers) is sent unchanged as Host. It lets
	// the server reach the object store on the internal network instead of
	// through the public address.
	StorageDialAddr string
}

type previewer struct {
	conv   Converter
	cfg    PreviewConfig
	http   *http.Client
	slots  chan struct{}
	sf     singleflight.Group
	direct bool

	mu       sync.Mutex
	failures map[string]failure
}

type failure struct {
	reason string
	until  time.Time
}

// SetConverter enables previews of Office files. Without a converter they are
// reported unavailable, never an error.
func (s *Service) SetConverter(c Converter, cfg PreviewConfig) {
	if cfg.MaxConvertBytes <= 0 {
		cfg.MaxConvertBytes = DefaultMaxConvertBytes
	}
	if cfg.MaxPDFBytes <= 0 {
		cfg.MaxPDFBytes = DefaultMaxPDFBytes
	}
	if cfg.MaxConcurrent <= 0 {
		cfg.MaxConcurrent = DefaultMaxConcurrent
	}
	if cfg.ConvertTimeout <= 0 {
		cfg.ConvertTimeout = DefaultConvertTimeout
	}
	if cfg.FailureTTL <= 0 {
		cfg.FailureTTL = DefaultFailureTTL
	}
	client, direct := storageClient(cfg.StorageDialAddr)
	p := &previewer{
		conv: c, cfg: cfg, slots: make(chan struct{}, cfg.MaxConcurrent),
		failures: map[string]failure{}, http: client, direct: direct,
	}
	s.preview = p
}

// storageClient is the HTTP client for signed storage URLs. With a dial address
// the connection goes there (direct is true, see directURL); without one it is
// an ordinary client.
func storageClient(dialAddr string) (c *http.Client, direct bool) {
	if dialAddr == "" {
		return &http.Client{}, false
	}
	d := &net.Dialer{Timeout: 10 * time.Second}
	return &http.Client{Transport: &http.Transport{
		DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
			return d.DialContext(ctx, network, dialAddr)
		},
	}}, true
}

func (p *previewer) storageURL(signed string) (string, error) { return directURL(signed, p.direct) }

// directURL is the address to request for a signed storage URL. With a dial
// override the hop is plain http (the object store listens in clear on the
// internal network); the host stays, so the Host header is still the one the
// signature covers.
func directURL(signed string, direct bool) (string, error) {
	if !direct {
		return signed, nil
	}
	u, err := url.Parse(signed)
	if err != nil {
		return "", err
	}
	u.Scheme = "http"
	return u.String(), nil
}

func (p *previewer) failed(key string) (string, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	f, ok := p.failures[key]
	if ok && time.Now().After(f.until) {
		delete(p.failures, key)
		return "", false
	}
	return f.reason, ok
}

func (p *previewer) remember(key, reason string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	now := time.Now()
	if len(p.failures) >= 4096 {
		for k, f := range p.failures {
			if now.After(f.until) {
				delete(p.failures, k)
			}
		}
	}
	p.failures[key] = failure{reason: reason, until: now.Add(p.cfg.FailureTTL)}
}

func unavailablePreview(reason string) *Preview {
	return &Preview{Kind: PreviewUnavailable, Reason: reason}
}

// GetPreview says how to show a file: the original, a PDF rendition of it, or
// not at all. The right it takes is the one a download takes — read on the
// file — and a refusal reads the same, so a preview reveals nothing a download
// would not.
//
// A conversion that cannot be done is an answer (unavailable), not an error:
// the client then offers the download.
func (s *Service) GetPreview(ctx context.Context, fileID string) (*Preview, error) {
	item, err := s.readableFile(ctx, fileID)
	if err != nil {
		return nil, err
	}
	var mimeType string
	if item.MimeType != nil {
		mimeType = *item.MimeType
	}
	class, mime, convName := classifyPreview(item.Name, mimeType)

	switch class {
	case previewNative, previewSheet:
		url, err := s.downloadURL(ctx, item.WorkspaceID, *item.ObjectKey)
		if err != nil {
			return nil, err
		}
		kind := PreviewNative
		if class == previewSheet {
			kind = PreviewSheet
		}
		return &Preview{Kind: kind, URL: url, Mime: mime}, nil

	case previewOffice:
		return s.officePreview(ctx, item.WorkspaceID, item.ID, *item.ObjectKey, convName, mime, item.SizeBytes)
	}
	return unavailablePreview(ReasonUnsupported), nil
}

// readableFile loads the stored file fileID for a caller who may read it. The
// right is the one a download takes — read on the file — and a refusal reads the
// same everywhere, so a preview or thumbnail reveals nothing a download would
// not. Only a stored file has content: a folder, or an upload that was never
// confirmed, is not found.
func (s *Service) readableFile(ctx context.Context, fileID string) (*store.DriveItem, error) {
	item, err := s.store.GetItem(ctx, fileID)
	if err != nil || item == nil {
		return nil, notFound("file not found")
	}
	if err := s.checkAccess(ctx, grpcauth.CallerFrom(ctx).NGACNodeID, item.NGACNodeID, ngac.OpRead); err != nil {
		return nil, err
	}
	if item.ItemType != "file" || item.ObjectKey == nil || item.Status == "pending" {
		return nil, notFound("file not found")
	}
	return item, nil
}

func (s *Service) downloadURL(ctx context.Context, workspaceID, objectKey string) (string, error) {
	resp, err := s.docStorage.GetDownloadURL(ctx, &docpb.GetDownloadURLRequest{WorkspaceId: workspaceID, ObjectKey: objectKey})
	if err != nil {
		return "", fmt.Errorf("get download url: %w", err)
	}
	return resp.DownloadUrl, nil
}

// officePreview serves the PDF rendition of an Office file, making it first if
// it does not exist.
func (s *Service) officePreview(ctx context.Context, workspaceID, fileID, objectKey, convName, sourceMime string, recordedSize *int64) (*Preview, error) {
	p := s.preview
	if p == nil || p.conv == nil {
		return unavailablePreview(ReasonConversionUnavailable), nil
	}
	if recordedSize != nil && *recordedSize > p.cfg.MaxConvertBytes {
		return unavailablePreview(ReasonTooLarge), nil
	}

	// The rendition is keyed by the file and by the content's identity (when it
	// was stored and how big it is), so replacing the content makes a new key
	// and the old rendition is never served for it.
	src, err := s.docStorage.GetObjectInfo(ctx, &docpb.GetObjectInfoRequest{WorkspaceId: workspaceID, ObjectKey: objectKey})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, notFound("file not found")
		}
		return nil, fmt.Errorf("stat source: %w", err)
	}
	if src.SizeBytes > p.cfg.MaxConvertBytes {
		return unavailablePreview(ReasonTooLarge), nil
	}
	slot, err := s.docStorage.GetUploadURL(ctx, &docpb.GetUploadURLRequest{
		WorkspaceId: workspaceID, DocId: previewDocID(fileID, src),
		Filename: previewFilename, MimeType: "application/pdf",
	})
	if err != nil {
		return nil, fmt.Errorf("get preview slot: %w", err)
	}

	exists, err := s.objectExists(ctx, workspaceID, slot.ObjectKey)
	if err != nil {
		return nil, err
	}
	if !exists {
		if reason, ok := p.failed(slot.ObjectKey); ok {
			return unavailablePreview(reason), nil
		}
		// Concurrent first views of a file share one conversion. It runs on its
		// own context: the viewer that happened to start it going away must not
		// fail the others.
		_, err, _ := p.sf.Do(slot.ObjectKey, func() (any, error) {
			return nil, s.convert(ctx, workspaceID, objectKey, convName, slot)
		})
		if err != nil {
			var reason string
			switch {
			case errors.Is(err, ErrConverterUnreachable):
				slog.Warn("preview: converter unreachable", "file", fileID, "error", err)
				return unavailablePreview(ReasonConversionUnavailable), nil
			case errors.Is(err, errConversionBusy):
				slog.Warn("preview: no conversion slot free in time", "file", fileID)
				return unavailablePreview(ReasonConversionBusy), nil
			case errors.Is(err, ErrPDFTooLarge):
				reason = ReasonTooLarge
			case errors.Is(err, ErrConversionFailed):
				reason = ReasonConversionFailed
			default:
				return nil, err
			}
			slog.Warn("preview: conversion failed", "file", fileID, "reason", reason, "error", err)
			p.remember(slot.ObjectKey, reason)
			return unavailablePreview(reason), nil
		}
	}

	url, err := s.downloadURL(ctx, workspaceID, slot.ObjectKey)
	if err != nil {
		return nil, err
	}
	return &Preview{Kind: PreviewPDF, URL: url, SourceMime: sourceMime}, nil
}

func (s *Service) objectExists(ctx context.Context, workspaceID, objectKey string) (bool, error) {
	_, err := s.docStorage.GetObjectInfo(ctx, &docpb.GetObjectInfoRequest{WorkspaceId: workspaceID, ObjectKey: objectKey})
	switch {
	case err == nil:
		return true, nil
	case status.Code(err) == codes.NotFound:
		return false, nil
	}
	return false, fmt.Errorf("stat preview: %w", err)
}

// convert fetches the original, converts it into a temporary file and stores
// that as the PDF in slot. The PDF is never held in memory. Every failure is
// ErrConversionFailed, ErrPDFTooLarge, ErrConverterUnreachable or
// errConversionBusy: from the client's side the file simply has no preview.
func (s *Service) convert(callCtx context.Context, workspaceID, objectKey, convName string, slot *docpb.GetUploadURLResponse) error {
	p := s.preview
	ctx, cancel := context.WithTimeout(context.WithoutCancel(callCtx), p.cfg.ConvertTimeout)
	defer cancel()

	// Only so many documents are open in the converter at once; the others wait
	// for their turn, within their time.
	select {
	case p.slots <- struct{}{}:
		defer func() { <-p.slots }()
	case <-ctx.Done():
		return errConversionBusy
	}

	// Another request may have finished while this one waited its turn.
	if exists, err := s.objectExists(ctx, workspaceID, slot.ObjectKey); err != nil {
		return err
	} else if exists {
		return nil
	}

	srcURL, err := s.downloadURL(ctx, workspaceID, objectKey)
	if err == nil {
		srcURL, err = p.storageURL(srcURL)
	}
	if err != nil {
		return fmt.Errorf("%w: %v", ErrConversionFailed, err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srcURL, nil)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrConversionFailed, err)
	}
	resp, err := p.http.Do(req)
	if err != nil {
		return fmt.Errorf("%w: fetch source: %v", ErrConversionFailed, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: fetch source answered %d", ErrConversionFailed, resp.StatusCode)
	}
	if resp.ContentLength > p.cfg.MaxConvertBytes {
		return fmt.Errorf("%w: source is %d bytes", ErrConversionFailed, resp.ContentLength)
	}

	tmp, err := os.CreateTemp("", "nexus-preview-*.pdf")
	if err != nil {
		return fmt.Errorf("%w: temp file: %v", ErrConversionFailed, err)
	}
	defer func() {
		tmp.Close()
		os.Remove(tmp.Name())
	}()
	if err := p.conv.ToPDF(ctx, convName, io.LimitReader(resp.Body, p.cfg.MaxConvertBytes+1), tmp, p.cfg.MaxPDFBytes); err != nil {
		return err
	}
	size, err := tmp.Seek(0, io.SeekCurrent)
	if err == nil {
		_, err = tmp.Seek(0, io.SeekStart)
	}
	if err != nil {
		return fmt.Errorf("%w: temp file: %v", ErrConversionFailed, err)
	}

	putURL, err := p.storageURL(slot.UploadUrl)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrConversionFailed, err)
	}
	put, err := http.NewRequestWithContext(ctx, http.MethodPut, putURL, io.NopCloser(tmp))
	if err != nil {
		return fmt.Errorf("%w: %v", ErrConversionFailed, err)
	}
	put.ContentLength = size
	put.Header.Set("Content-Type", "application/pdf")
	pr, err := p.http.Do(put)
	if err != nil {
		return fmt.Errorf("%w: store preview: %v", ErrConversionFailed, err)
	}
	defer pr.Body.Close()
	if pr.StatusCode/100 != 2 {
		return fmt.Errorf("%w: store preview answered %d", ErrConversionFailed, pr.StatusCode)
	}
	return nil
}

// removeRendition deletes the PDF rendition of a file about to be deleted for
// good, so its text does not outlive it. The rendition's key follows from the
// original's stored-at time and size, which are read before the original goes.
// Best effort: a failure is logged, never the deletion's.
func (s *Service) removeRendition(ctx context.Context, f *store.DriveItem) {
	if f.ObjectKey == nil {
		return
	}
	var mimeType string
	if f.MimeType != nil {
		mimeType = *f.MimeType
	}
	if class, _, _ := classifyPreview(f.Name, mimeType); class != previewOffice {
		return
	}
	src, err := s.docStorage.GetObjectInfo(ctx, &docpb.GetObjectInfoRequest{WorkspaceId: f.WorkspaceID, ObjectKey: *f.ObjectKey})
	if err != nil {
		if status.Code(err) != codes.NotFound {
			slog.Error("deleted file's preview not removed", "item", f.ID, "err", err)
		}
		return
	}
	slot, err := s.docStorage.GetUploadURL(ctx, &docpb.GetUploadURLRequest{
		WorkspaceId: f.WorkspaceID, DocId: previewDocID(f.ID, src), Filename: previewFilename, MimeType: "application/pdf",
	})
	if err == nil {
		_, err = s.docStorage.DeleteObject(ctx, &docpb.DeleteObjectRequest{WorkspaceId: f.WorkspaceID, ObjectKey: slot.ObjectKey})
	}
	if err != nil {
		slog.Error("deleted file's preview not removed", "item", f.ID, "err", err)
	}
}

const previewFilename = "preview.pdf"

// previewDocID names a rendition by the file and by its content's identity
// (when it was stored and how big it is).
func previewDocID(fileID string, src *docpb.ObjectInfo) string {
	return fmt.Sprintf("previews/%s/%d-%d", fileID, src.LastModified.AsTime().UnixNano(), src.SizeBytes)
}
