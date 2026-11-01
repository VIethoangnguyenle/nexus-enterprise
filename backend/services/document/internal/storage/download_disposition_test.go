package storage

import (
	"context"
	"errors"
	"testing"

	"github.com/minio/minio-go/v7"
)

// Anything but a known-safe type, malformed types included, is never served inline
// from the storage host, which is same-site with the app: its download URL is
// signed to come back as an attachment of a type the browser does not render.
func TestDownloadURL_ActiveContentIsServedAsAnAttachment(t *testing.T) {
	for _, tc := range []struct {
		name, key, contentType string
	}{
		{"html by type", "drive/d/page", "text/html; charset=utf-8"},
		{"xhtml", "drive/d/page", "application/xhtml+xml"},
		{"svg by type", "drive/d/logo", "image/svg+xml"},
		{"xml", "drive/d/feed", "application/xml"},
		{"atom (+xml)", "drive/d/feed", "application/atom+xml"},
		{"script", "drive/d/x", "text/javascript"},
		{"html by name, harmless type claimed", "drive/d/x.html", "image/png"},
		{"svg by name, upper case", "drive/d/LOGO.SVG", "image/png"},
		// Malformed parameters: the browser still takes the type before the ';'.
		{"html, bad parameter", "drive/d/x.txt", "text/html; x"},
		{"html, empty parameter", "drive/d/x.txt", "text/html;;"},
		{"html, parameter without value", "drive/d/x.txt", "text/html;charset"},
		{"svg, unterminated quote", "drive/d/x.txt", `image/svg+xml; a="`},
		{"html, upper case", "drive/d/x.txt", "TEXT/HTML"},
		// Anything not known to be safe inline.
		{"unparseable", "drive/d/x.txt", ";;;"},
		{"empty type", "drive/d/x.bin", ""},
		{"octet-stream", "drive/d/x.bin", "application/octet-stream"},
		{"json", "drive/d/x.json", "application/json"},
		{"markdown", "drive/d/x.md", "text/markdown"},
		{"css", "drive/d/x.css", "text/css"},
		{"multipart", "drive/d/x", "multipart/x-mixed-replace; boundary=a"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st := newFakeStore()
			st.statInfo = minio.ObjectInfo{ContentType: tc.contentType}
			sg := &fakeSigner{}
			if _, err := New(st, sg).DownloadURL(context.Background(), "w1", tc.key); err != nil {
				t.Fatal(err)
			}
			if got := sg.getParams.Get("response-content-type"); got != "application/octet-stream" {
				t.Errorf("response-content-type = %q", got)
			}
			if got := sg.getParams.Get("response-content-disposition"); len(got) < len("attachment") || got[:len("attachment")] != "attachment" {
				t.Errorf("response-content-disposition = %q", got)
			}
		})
	}
}

// A type the store cannot be asked about is treated as active: fail closed.
func TestDownloadURL_UnknownTypeIsServedAsAnAttachment(t *testing.T) {
	st := newFakeStore()
	st.statErr = errors.New("store unreachable")
	sg := &fakeSigner{}
	if _, err := New(st, sg).DownloadURL(context.Background(), "w1", "drive/d/f.pdf"); err != nil {
		t.Fatal(err)
	}
	if sg.getParams.Get("response-content-disposition") == "" {
		t.Error("an object whose type is unknown must not be served inline")
	}
}

// Only types known to be safe inline (the ones the file viewer shows with an
// img, video, audio or PDF frame, and plain text) keep opening inline.
func TestDownloadURL_PassiveContentStaysInline(t *testing.T) {
	for _, tc := range []struct{ key, contentType string }{
		{"drive/d/f.pdf", "application/pdf"},
		{"drive/d/a.png", "image/png"},
		{"drive/d/a.jpg", "image/jpeg; charset=binary"},
		{"drive/d/a.gif", "image/gif"},
		{"drive/d/a.webp", "image/webp"},
		{"drive/d/v.mp4", "video/mp4"},
		{"drive/d/v.webm", "video/webm"},
		{"drive/d/s.mp3", "audio/mpeg"},
		{"drive/d/notes.txt", "text/plain; charset=utf-8"},
	} {
		st := newFakeStore()
		st.statInfo = minio.ObjectInfo{ContentType: tc.contentType}
		sg := &fakeSigner{}
		if _, err := New(st, sg).DownloadURL(context.Background(), "w1", tc.key); err != nil {
			t.Fatal(err)
		}
		if len(sg.getParams) != 0 {
			t.Errorf("%s (%s) signed with %v, want no overrides", tc.key, tc.contentType, sg.getParams)
		}
	}
}

func TestDownloadURL_AttachmentNameIsTheObjectsAndEncoded(t *testing.T) {
	st := newFakeStore()
	st.statInfo = minio.ObjectInfo{ContentType: "text/html"}
	sg := &fakeSigner{}
	if _, err := New(st, sg).DownloadURL(context.Background(), "w1", `drive/d/Báo "cáo".html`); err != nil {
		t.Fatal(err)
	}
	want := `attachment; filename*=UTF-8''B%C3%A1o%20%22c%C3%A1o%22.html`
	if got := sg.getParams.Get("response-content-disposition"); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
