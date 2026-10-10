package grpc_test

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "ngac-platform/proto/document"
	dgrpc "ngac-platform/services/document/internal/grpc"
	"ngac-platform/services/document/internal/storage"
)

type failingStore struct{ err error }

func (f failingStore) BucketExists(context.Context, string) (bool, error) { return true, nil }
func (f failingStore) MakeBucket(context.Context, string, minio.MakeBucketOptions) error {
	return nil
}
func (f failingStore) StatObject(context.Context, string, string, minio.StatObjectOptions) (minio.ObjectInfo, error) {
	return minio.ObjectInfo{}, f.err
}
func (f failingStore) RemoveObject(context.Context, string, string, minio.RemoveObjectOptions) error {
	return f.err
}
func (f failingStore) CopyObject(context.Context, minio.CopyDestOptions, minio.CopySrcOptions) (minio.UploadInfo, error) {
	return minio.UploadInfo{}, f.err
}

type failingSigner struct{ err error }

func (f failingSigner) PresignedPutObject(context.Context, string, string, time.Duration) (*url.URL, error) {
	return nil, f.err
}
func (f failingSigner) PresignedGetObject(context.Context, string, string, time.Duration, url.Values) (*url.URL, error) {
	return nil, f.err
}

func server(err error) *dgrpc.DocumentStorageServer {
	return dgrpc.NewDocumentStorageServer(storage.New(failingStore{err}, failingSigner{err}))
}

// A missing upload and a missing object are the caller's to hear about; every
// other failure of the object store is ours, and its text (an address, a
// bucket) stays in the log.
func TestStorageServer_MapsRefusalsAndHidesInternals(t *testing.T) {
	ctx := context.Background()
	leaky := errors.New("dial tcp 10.0.0.5:9000: connection refused")

	missing := minio.ErrorResponse{Code: "NoSuchKey", Message: "10.0.0.5:9000"}
	_, err := server(missing).ConfirmUpload(ctx, &pb.ConfirmUploadRequest{WorkspaceId: "w", ObjectKey: "k"})
	if status.Code(err) != codes.FailedPrecondition || status.Convert(err).Message() != "file not uploaded" {
		t.Errorf("confirm: %v", err)
	}

	_, err = server(missing).GetObjectInfo(ctx, &pb.GetObjectInfoRequest{WorkspaceId: "w", ObjectKey: "k"})
	if status.Code(err) != codes.NotFound || status.Convert(err).Message() != "object not found" {
		t.Errorf("info: %v", err)
	}

	// An outage while checking is ours: Internal, nothing of the host.
	_, err = server(leaky).ConfirmUpload(ctx, &pb.ConfirmUploadRequest{WorkspaceId: "w", ObjectKey: "k"})
	if status.Code(err) != codes.Internal || strings.Contains(err.Error(), "10.0.0.5") && status.Convert(err).Message() != "internal error" {
		t.Errorf("confirm during an outage: %v", err)
	}

	calls := map[string]func() error{
		"upload": func() error {
			_, e := server(leaky).GetUploadURL(ctx, &pb.GetUploadURLRequest{WorkspaceId: "w"})
			return e
		},
		"download": func() error {
			_, e := server(leaky).GetDownloadURL(ctx, &pb.GetDownloadURLRequest{WorkspaceId: "w"})
			return e
		},
		"delete": func() error {
			_, e := server(leaky).DeleteObject(ctx, &pb.DeleteObjectRequest{WorkspaceId: "w"})
			return e
		},
		"copy": func() error { _, e := server(leaky).CopyObject(ctx, &pb.CopyObjectRequest{}); return e },
	}
	for name, call := range calls {
		err := call()
		if status.Code(err) != codes.Internal {
			t.Errorf("%s: code = %v, want Internal", name, status.Code(err))
		}
		if msg := status.Convert(err).Message(); strings.Contains(msg, "10.0.0.5") || msg != "internal error" {
			t.Errorf("%s: wire message = %q", name, msg)
		}
	}
}
