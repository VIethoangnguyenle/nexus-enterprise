package storage

import (
	"context"
	"net/url"
	"testing"
	"time"
)

func presign(t *testing.T, pub PublicEndpoint) *url.URL {
	t.Helper()
	c, err := NewPresignClient(pub, "access", "secret")
	if err != nil {
		t.Fatal(err)
	}
	// A cancelled context proves signing makes no network call: any request to
	// discover the region would fail.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	u, err := c.PresignedPutObject(ctx, "ws-1", "drive/doc/a b.txt", time.Minute)
	if err != nil {
		t.Fatalf("presign: %v", err)
	}
	return u
}

func TestNewPresignClient_PublicHTTPS(t *testing.T) {
	u := presign(t, PublicEndpoint{Host: "storage.nexus.zaneng.xyz", Secure: true})
	if u.Scheme != "https" || u.Host != "storage.nexus.zaneng.xyz" {
		t.Fatalf("url = %s, want https on the public host", u)
	}
	if u.Path != "/ws-1/drive/doc/a b.txt" {
		t.Errorf("path = %q, want path-style bucket addressing", u.Path)
	}
	if u.Query().Get("X-Amz-Signature") == "" || u.Query().Get("X-Amz-SignedHeaders") != "host" {
		t.Errorf("not a signed URL: %s", u)
	}
}

func TestNewPresignClient_DevDefaultStaysPlainHTTP(t *testing.T) {
	u := presign(t, PublicEndpoint{Host: "localhost:9100"})
	if u.Scheme != "http" || u.Host != "localhost:9100" {
		t.Fatalf("url = %s, want http on localhost:9100", u)
	}
}

func TestNewPresignClient_RejectsAPathInTheHost(t *testing.T) {
	if _, err := NewPresignClient(PublicEndpoint{Host: "example.test/storage"}, "a", "b"); err == nil {
		t.Fatal("a path in the endpoint cannot be signed against and must be refused")
	}
}
