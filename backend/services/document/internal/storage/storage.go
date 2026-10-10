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
	"net/url"
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
func (s *Service) UploadURL(ctx context.Context, workspaceID, docID, filename string) (uploadURL, objectKey string, err error) {
	bucket := BucketName(workspaceID)
	s.ensureBucket(ctx, bucket)

	key := fmt.Sprintf("drive/%s/%s", docID, filename)
	u, err := s.signer.PresignedPutObject(ctx, bucket, key, uploadURLTTL)
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
func (s *Service) DownloadURL(ctx context.Context, workspaceID, objectKey string) (string, error) {
	u, err := s.signer.PresignedGetObject(ctx, BucketName(workspaceID), objectKey, downloadURLTTL, nil)
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
