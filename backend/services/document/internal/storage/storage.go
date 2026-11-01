// Package storage is the document service's object-storage logic: which bucket
// a workspace's files live in, how an object is keyed, how long a signed URL
// lives, and what counts as an upload that did not happen. The gRPC server is a
// thin adapter over it. It does no authorization: drive decides who may touch a
// file before calling.
package storage

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"

	"ngac-platform/pkg/httputil"
)

// Signed URL lifetimes.
const (
	uploadURLTTL   = 5 * time.Minute
	downloadURLTTL = 15 * time.Minute
)

// ErrNotUploaded: the object the caller says it uploaded is not in the store.
var ErrNotUploaded = errors.New("file not uploaded")

// notFound is httputil.ErrNotFound with a message written for the caller.
type notFound struct{ msg string }

func (e *notFound) Error() string        { return e.msg }
func (e *notFound) Is(target error) bool { return target == httputil.ErrNotFound }

// invalidInput is httputil.ErrInvalidInput with a message written for the caller.
type invalidInput struct{ msg string }

func (e *invalidInput) Error() string        { return e.msg }
func (e *invalidInput) Is(target error) bool { return target == httputil.ErrInvalidInput }

// isNoSuchKey reports whether the store answered that the object is not there.
func isNoSuchKey(err error) bool {
	code := minio.ToErrorResponse(err).Code
	return code == "NoSuchKey" || code == "NotFound"
}

// Client is the part of *minio.Client the service uses for server-side work.
type Client interface {
	BucketExists(ctx context.Context, bucket string) (bool, error)
	MakeBucket(ctx context.Context, bucket string, opts minio.MakeBucketOptions) error
	StatObject(ctx context.Context, bucket, object string, opts minio.StatObjectOptions) (minio.ObjectInfo, error)
	RemoveObject(ctx context.Context, bucket, object string, opts minio.RemoveObjectOptions) error
	CopyObject(ctx context.Context, dst minio.CopyDestOptions, src minio.CopySrcOptions) (minio.UploadInfo, error)
}

// Presigner signs URLs against the store's public endpoint, which is not the
// address this service reaches it on.
type Presigner interface {
	PresignedPutObject(ctx context.Context, bucket, object string, expires time.Duration) (*url.URL, error)
	PresignHeader(ctx context.Context, method, bucket, object string, expires time.Duration, reqParams url.Values, extraHeaders http.Header) (*url.URL, error)
	PresignedGetObject(ctx context.Context, bucket, object string, expires time.Duration, reqParams url.Values) (*url.URL, error)
}

// Service is the object-storage logic.
type Service struct {
	client Client
	signer Presigner
}

// New returns a Service over the given store clients.
func New(c Client, p Presigner) *Service { return &Service{client: c, signer: p} }

// BucketName derives a workspace's bucket.
func BucketName(workspaceID string) string { return fmt.Sprintf("ws-%s", workspaceID) }

// ensureBucket creates the workspace's bucket when it is missing. It is best
// effort: if the store's answer is unclear the signing or copy that follows
// reports the real failure.
func (s *Service) ensureBucket(ctx context.Context, bucket string) {
	exists, err := s.client.BucketExists(ctx, bucket)
	if err != nil || exists {
		return
	}
	if err := s.client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
		slog.Warn("make bucket failed", "bucket", bucket, "error", err)
	}
}

// UploadURL returns a signed PUT URL for a new object of document docID, and the
// key the object will have.
//
// With a size (0 or more) the URL takes exactly that many bytes: the size is
// signed in as the request's Content-Length, which the object store checks, so
// the stored object cannot be larger than the size the drive checked against
// the quota, and the URL cannot later be used to put a bigger object in its
// place. A browser sends a file's own length, so a client that declares the
// file's real size needs nothing more. A negative size is refused. A nil size
// takes an object of any size: only the server's own renditions (previews,
// thumbnails), whose size is not known when they ask, use that; an upload by a
// person always names its size.
func (s *Service) UploadURL(ctx context.Context, workspaceID, docID, filename string, size *int64) (uploadURL, objectKey string, err error) {
	if size != nil && *size < 0 {
		return "", "", &invalidInput{msg: "size_bytes must not be negative"}
	}
	bucket := BucketName(workspaceID)
	s.ensureBucket(ctx, bucket)

	key := fmt.Sprintf("drive/%s/%s", docID, filename)
	var u *url.URL
	if size == nil {
		u, err = s.signer.PresignedPutObject(ctx, bucket, key, uploadURLTTL)
	} else {
		// The declared size and "only if nothing is there yet" are signed: the URL
		// takes exactly that many bytes, once. The browser sends both headers.
		u, err = s.signer.PresignHeader(ctx, http.MethodPut, bucket, key, uploadURLTTL, nil,
			http.Header{"Content-Length": []string{strconv.FormatInt(*size, 10)}, "If-None-Match": []string{"*"}})
	}
	if err != nil {
		return "", "", fmt.Errorf("generate upload URL: %w", err)
	}
	return u.String(), key, nil
}

// Confirm checks that the object exists and returns its size and content type.
// An object that is not there is ErrNotUploaded.
func (s *Service) Confirm(ctx context.Context, workspaceID, objectKey string) (size int64, contentType string, err error) {
	info, err := s.client.StatObject(ctx, BucketName(workspaceID), objectKey, minio.StatObjectOptions{})
	if err != nil {
		if isNoSuchKey(err) {
			return 0, "", ErrNotUploaded
		}
		// The store being unreachable is not "the file was not uploaded", and its
		// error text names hosts and buckets: it goes to the log only.
		return 0, "", fmt.Errorf("stat object: %w", err)
	}
	return info.Size, info.ContentType, nil
}

// DownloadURL returns a signed GET URL for an object.
//
// The store is served from a host that is same-site with the app, and it
// answers with whatever Content-Type the uploader chose. Only types known to be
// safe inline (images other than SVG, PDF, video, audio, plain text; see
// inlineTypes) keep opening inline, so the file viewer's previews still render.
// Every other object, and one whose type cannot be read, is signed to come back
// as an attachment of type application/octet-stream, so an uploaded page can
// never render or run there.
func (s *Service) DownloadURL(ctx context.Context, workspaceID, objectKey string) (string, error) {
	bucket := BucketName(workspaceID)
	var params url.Values
	info, statErr := s.client.StatObject(ctx, bucket, objectKey, minio.StatObjectOptions{})
	if statErr != nil || servedAsAttachment(objectKey, info.ContentType) {
		params = url.Values{}
		params.Set("response-content-disposition", "attachment; filename*=UTF-8''"+encodeRFC5987(path.Base(objectKey)))
		params.Set("response-content-type", "application/octet-stream")
	}
	u, err := s.signer.PresignedGetObject(ctx, bucket, objectKey, downloadURLTTL, params)
	if err != nil {
		return "", fmt.Errorf("generate download URL: %w", err)
	}
	return u.String(), nil
}

// Delete removes an object.
func (s *Service) Delete(ctx context.Context, workspaceID, objectKey string) error {
	if err := s.client.RemoveObject(ctx, BucketName(workspaceID), objectKey, minio.RemoveObjectOptions{}); err != nil {
		return fmt.Errorf("delete object: %w", err)
	}
	return nil
}

// Copy copies an object server-side, creating the destination bucket if needed,
// and returns the copy's size.
func (s *Service) Copy(ctx context.Context, srcWorkspaceID, srcKey, dstWorkspaceID, dstKey string) (int64, error) {
	dstBucket := BucketName(dstWorkspaceID)
	s.ensureBucket(ctx, dstBucket)

	info, err := s.client.CopyObject(ctx,
		minio.CopyDestOptions{Bucket: dstBucket, Object: dstKey},
		minio.CopySrcOptions{Bucket: BucketName(srcWorkspaceID), Object: srcKey})
	if err != nil {
		return 0, fmt.Errorf("copy object: %w", err)
	}
	return info.Size, nil
}

// ObjectInfo describes a stored object.
type ObjectInfo struct {
	Size         int64
	ContentType  string
	LastModified time.Time
}

// Info returns an object's metadata. An object that is not there is
// httputil.ErrNotFound.
func (s *Service) Info(ctx context.Context, workspaceID, objectKey string) (*ObjectInfo, error) {
	info, err := s.client.StatObject(ctx, BucketName(workspaceID), objectKey, minio.StatObjectOptions{})
	if err != nil {
		if isNoSuchKey(err) {
			return nil, &notFound{msg: "object not found"}
		}
		return nil, fmt.Errorf("stat object: %w", err)
	}
	return &ObjectInfo{Size: info.Size, ContentType: info.ContentType, LastModified: info.LastModified}, nil
}

// activeExtensions are names browsers render as documents that can run script.
var activeExtensions = map[string]bool{
	".html": true, ".htm": true, ".shtml": true, ".xhtml": true, ".xht": true,
	".svg": true, ".svgz": true, ".xml": true, ".xsl": true, ".xslt": true,
	".js": true, ".mjs": true,
}

// inlineTypes are the media types served inline: the ones the file viewer
// shows in an img, video or audio element or a PDF frame, and plain text. None
// of them can run script. Every other type is served as an attachment.
var inlineTypes = map[string]bool{
	"image/png": true, "image/jpeg": true, "image/gif": true, "image/webp": true,
	"image/avif": true, "image/bmp": true,
	"application/pdf": true,
	"text/plain":      true,
}

// servedAsAttachment reports whether an object must not be served inline: its
// name is one browsers render as a document, or its stored type is not one of
// inlineTypes. The type is compared by its essence, the part before any ';',
// trimmed and lower-cased, which is what a browser acts on; a malformed
// parameter list therefore cannot hide an HTML or SVG type, and an empty or
// unreadable type is not inline either.
func servedAsAttachment(objectKey, contentType string) bool {
	if activeExtensions[strings.ToLower(path.Ext(objectKey))] {
		return true
	}
	essence, _, _ := strings.Cut(contentType, ";")
	essence = strings.ToLower(strings.TrimSpace(essence))
	switch {
	case inlineTypes[essence]:
		return false
	case strings.HasPrefix(essence, "video/"), strings.HasPrefix(essence, "audio/"):
		return strings.Contains(essence, "xml") // no media type carries markup
	}
	return true
}

// encodeRFC5987 percent-encodes a filename for a filename*= parameter
// (RFC 5987 attr-char), so quotes, separators and non-ASCII letters survive.
func encodeRFC5987(name string) string {
	var b strings.Builder
	for _, c := range []byte(name) {
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9',
			strings.IndexByte("!#$&+-.^_`|~", c) >= 0:
			b.WriteByte(c)
		default:
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}
