package storage

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"

	"ngac-platform/pkg/httputil"
)

type fakeStore struct {
	buckets   map[string]bool
	existsErr error
	makeErr   error
	statInfo  minio.ObjectInfo
	statErr   error
	removeErr error
	copyInfo  minio.UploadInfo
	copyErr   error

	made    []string
	removed []string
	copied  [][4]string
}

func newFakeStore() *fakeStore { return &fakeStore{buckets: map[string]bool{}} }

func (f *fakeStore) BucketExists(_ context.Context, b string) (bool, error) {
	return f.buckets[b], f.existsErr
}
func (f *fakeStore) MakeBucket(_ context.Context, b string, _ minio.MakeBucketOptions) error {
	if f.makeErr != nil {
		return f.makeErr
	}
	f.buckets[b] = true
	f.made = append(f.made, b)
	return nil
}
func (f *fakeStore) StatObject(context.Context, string, string, minio.StatObjectOptions) (minio.ObjectInfo, error) {
	return f.statInfo, f.statErr
}
func (f *fakeStore) RemoveObject(_ context.Context, b, k string, _ minio.RemoveObjectOptions) error {
	f.removed = append(f.removed, b+"/"+k)
	return f.removeErr
}
func (f *fakeStore) CopyObject(_ context.Context, dst minio.CopyDestOptions, src minio.CopySrcOptions) (minio.UploadInfo, error) {
	f.copied = append(f.copied, [4]string{src.Bucket, src.Object, dst.Bucket, dst.Object})
	return f.copyInfo, f.copyErr
}

type fakeSigner struct {
	err       error
	putExpiry time.Duration
	getExpiry time.Duration
	last      string
}

func (f *fakeSigner) PresignedPutObject(_ context.Context, b, k string, exp time.Duration) (*url.URL, error) {
	f.putExpiry, f.last = exp, b+"/"+k
	if f.err != nil {
		return nil, f.err
	}
	return &url.URL{Scheme: "http", Host: "public", Path: "/" + b + "/" + k, RawQuery: "put=1"}, nil
}
func (f *fakeSigner) PresignedGetObject(_ context.Context, b, k string, exp time.Duration, _ url.Values) (*url.URL, error) {
	f.getExpiry, f.last = exp, b+"/"+k
	if f.err != nil {
		return nil, f.err
	}
	return &url.URL{Scheme: "http", Host: "public", Path: "/" + b + "/" + k, RawQuery: "get=1"}, nil
}

func TestBucketNameIsDerivedFromTheWorkspace(t *testing.T) {
	if got := BucketName("w1"); got != "ws-w1" {
		t.Fatalf("got %q", got)
	}
}

func TestUploadURL_CreatesTheBucketAndKeysTheObjectByDocument(t *testing.T) {
	st, sg := newFakeStore(), &fakeSigner{}
	u, key, err := New(st, sg).UploadURL(context.Background(), "w1", "doc-9", "report.pdf")
	if err != nil {
		t.Fatal(err)
	}
	if key != "drive/doc-9/report.pdf" {
		t.Errorf("key = %q", key)
	}
	if !strings.Contains(u, "ws-w1/drive/doc-9/report.pdf") || !strings.Contains(u, "put=1") {
		t.Errorf("url = %q", u)
	}
	if len(st.made) != 1 || st.made[0] != "ws-w1" {
		t.Errorf("bucket made = %v", st.made)
	}
	if sg.putExpiry != 5*time.Minute {
		t.Errorf("upload URL lives %v, want 5m", sg.putExpiry)
	}
}

// The bucket is only made when it is missing; a failed existence check or a
// failed creation does not stop the signing (the object store says no later).
func TestEnsureBucket_IsBestEffort(t *testing.T) {
	st := newFakeStore()
	st.buckets["ws-w1"] = true
	if _, _, err := New(st, &fakeSigner{}).UploadURL(context.Background(), "w1", "d", "f"); err != nil || len(st.made) != 0 {
		t.Errorf("existing bucket: err=%v made=%v", err, st.made)
	}

	failing := newFakeStore()
	failing.makeErr = errors.New("denied")
	if _, _, err := New(failing, &fakeSigner{}).UploadURL(context.Background(), "w2", "d", "f"); err != nil {
		t.Errorf("a failed MakeBucket must not fail the upload URL: %v", err)
	}

	broken := newFakeStore()
	broken.existsErr = errors.New("unreachable")
	if _, _, err := New(broken, &fakeSigner{}).UploadURL(context.Background(), "w3", "d", "f"); err != nil || len(broken.made) != 0 {
		t.Errorf("unknown bucket state: err=%v made=%v", err, broken.made)
	}
}

func TestUploadURL_SigningFailureIsAnUnclassifiedError(t *testing.T) {
	boom := errors.New("minio 10.0.0.5 unreachable")
	_, _, err := New(newFakeStore(), &fakeSigner{err: boom}).UploadURL(context.Background(), "w1", "d", "f")
	if !errors.Is(err, boom) {
		t.Fatalf("cause lost: %v", err)
	}
	if errors.Is(err, httputil.ErrNotFound) || errors.Is(err, ErrNotUploaded) {
		t.Errorf("a signing failure is nobody's refusal: %v", err)
	}
}

func TestConfirm_ReturnsTheStoredSizeAndType(t *testing.T) {
	st := newFakeStore()
	st.statInfo = minio.ObjectInfo{Size: 2048, ContentType: "application/pdf"}
	size, ctype, err := New(st, &fakeSigner{}).Confirm(context.Background(), "w1", "k")
	if err != nil || size != 2048 || ctype != "application/pdf" {
		t.Fatalf("got %d %q %v", size, ctype, err)
	}
}

func TestConfirm_AMissingObjectIsNotUploaded(t *testing.T) {
	st := newFakeStore()
	st.statErr = minio.ErrorResponse{Code: "NoSuchKey", Message: "The specified key does not exist."}
	_, _, err := New(st, &fakeSigner{}).Confirm(context.Background(), "w1", "k")
	if !errors.Is(err, ErrNotUploaded) {
		t.Fatalf("want ErrNotUploaded, got %v", err)
	}
	if strings.Contains(err.Error(), "minio.internal") || err.Error() != "file not uploaded" {
		t.Errorf("the store's text reached the message: %q", err)
	}
}

func TestInfo_AMissingObjectIsNotFound(t *testing.T) {
	st := newFakeStore()
	st.statErr = minio.ErrorResponse{Code: "NoSuchKey"}
	_, err := New(st, &fakeSigner{}).Info(context.Background(), "w1", "k")
	if !errors.Is(err, httputil.ErrNotFound) || err.Error() != "object not found" {
		t.Fatalf("want ErrNotFound with a fixed message, got %v", err)
	}

	st = newFakeStore()
	st.statInfo = minio.ObjectInfo{Size: 7, ContentType: "text/plain", LastModified: time.Unix(1700000000, 0)}
	info, err := New(st, &fakeSigner{}).Info(context.Background(), "w1", "k")
	if err != nil || info.Size != 7 || info.ContentType != "text/plain" || info.LastModified.Unix() != 1700000000 {
		t.Fatalf("got %+v %v", info, err)
	}
}

func TestDownloadURL(t *testing.T) {
	sg := &fakeSigner{}
	u, err := New(newFakeStore(), sg).DownloadURL(context.Background(), "w1", "drive/d/f.pdf")
	if err != nil || !strings.Contains(u, "get=1") || sg.getExpiry != 15*time.Minute {
		t.Fatalf("got %q %v expiry %v", u, err, sg.getExpiry)
	}
	if _, err := New(newFakeStore(), &fakeSigner{err: errors.New("x")}).DownloadURL(context.Background(), "w1", "k"); err == nil {
		t.Error("a signing failure must be reported")
	}
}

func TestDelete(t *testing.T) {
	st := newFakeStore()
	if err := New(st, &fakeSigner{}).Delete(context.Background(), "w1", "k"); err != nil {
		t.Fatal(err)
	}
	if len(st.removed) != 1 || st.removed[0] != "ws-w1/k" {
		t.Errorf("removed = %v", st.removed)
	}
	st.removeErr = errors.New("boom")
	if err := New(st, &fakeSigner{}).Delete(context.Background(), "w1", "k"); err == nil {
		t.Error("a failed removal must be reported")
	}
}

func TestCopy_CreatesTheDestinationBucketAndCopiesAcrossWorkspaces(t *testing.T) {
	st := newFakeStore()
	st.copyInfo = minio.UploadInfo{Size: 99}
	size, err := New(st, &fakeSigner{}).Copy(context.Background(), "w1", "a", "w2", "b")
	if err != nil || size != 99 {
		t.Fatalf("got %d %v", size, err)
	}
	if len(st.copied) != 1 || st.copied[0] != [4]string{"ws-w1", "a", "ws-w2", "b"} {
		t.Errorf("copied = %v", st.copied)
	}
	if len(st.made) != 1 || st.made[0] != "ws-w2" {
		t.Errorf("destination bucket made = %v", st.made)
	}
	st.copyErr = errors.New("boom")
	if _, err := New(st, &fakeSigner{}).Copy(context.Background(), "w1", "a", "w2", "b"); err == nil {
		t.Error("a failed copy must be reported")
	}
}

// Anything else the store says is a failure of ours, not a missing file, and its
// text (host, bucket) stays out of the message.
func TestConfirmAndInfo_AnOutageIsNotAMissingFile(t *testing.T) {
	st := newFakeStore()
	st.statErr = errors.New("dial tcp minio.internal:9000: connect: connection refused")
	svc := New(st, &fakeSigner{})

	_, _, err := svc.Confirm(context.Background(), "w1", "k")
	if errors.Is(err, ErrNotUploaded) {
		t.Errorf("an outage must not read as 'not uploaded': %v", err)
	}
	_, err = svc.Info(context.Background(), "w1", "k")
	if errors.Is(err, httputil.ErrNotFound) {
		t.Errorf("an outage must not read as 'not found': %v", err)
	}
}
