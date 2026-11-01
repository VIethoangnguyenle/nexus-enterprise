package storage

import (
	"context"
	"testing"
)

// An upload URL for a declared size accepts exactly that many bytes: the size
// is a signed Content-Length, which the object store checks against the request,
// so the stored object can neither outgrow the quota the drive charged nor be
// replaced by a bigger one later through the same URL.
func TestUploadURL_ADeclaredSizeIsSignedIntoTheUpload(t *testing.T) {
	for _, size := range []int64{0, 1, 5 << 20} {
		size := size
		sg := &fakeSigner{}
		u, _, err := New(newFakeStore(), sg).UploadURL(context.Background(), "w1", "d", "f.bin", &size)
		if err != nil {
			t.Fatal(err)
		}
		if got := sg.putHeader.Get("Content-Length"); got != itoa(size) {
			t.Errorf("size %d: signed Content-Length = %q", size, got)
		}
		// Write-once: a second PUT through the same URL is refused by the store.
		if got := sg.putHeader.Get("If-None-Match"); got != "*" {
			t.Errorf("size %d: signed If-None-Match = %q, want *", size, got)
		}
		if sg.putExpiry != uploadURLTTL {
			t.Errorf("size %d: lives %v", size, sg.putExpiry)
		}
		if u == "" {
			t.Errorf("size %d: no URL", size)
		}
	}
}

// Deny: a negative size is refused before anything is signed.
func TestUploadURL_ANegativeSizeIsRefused(t *testing.T) {
	sg := &fakeSigner{}
	for _, n := range []int64{-1, -2} {
		if _, _, err := New(newFakeStore(), sg).UploadURL(context.Background(), "w1", "d", "f", &n); err == nil {
			t.Fatalf("size %d must be refused", n)
		}
	}
	if sg.last != "" {
		t.Error("nothing is signed for a refused size")
	}
}

// The server's own renditions (previews, thumbnails) do not know their size
// when they ask for a slot; with no size they keep an unbound URL.
func TestUploadURL_NoSizeIsUnbound(t *testing.T) {
	sg := &fakeSigner{}
	if _, _, err := New(newFakeStore(), sg).UploadURL(context.Background(), "w1", "d", "f", nil); err != nil {
		t.Fatal(err)
	}
	if sg.putHeader != nil {
		t.Errorf("unbound upload signed with %v", sg.putHeader)
	}
}

func itoa(n int64) string {
	const digits = "0123456789"
	if n == 0 {
		return "0"
	}
	var b []byte
	for ; n > 0; n /= 10 {
		b = append([]byte{digits[n%10]}, b...)
	}
	return string(b)
}
