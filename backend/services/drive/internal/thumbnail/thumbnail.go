// Package thumbnail draws the small JPEG a file is recognised by: a picture
// scaled down, the first page of a PDF, a frame of a video. Pictures are
// decoded in-process; PDFs and videos go through pdftoppm and ffmpeg, run
// without a shell, in a private directory, under the caller's context.
//
// It knows nothing of who may see a file or where files live: it turns a file
// on disk into JPEG bytes.
package thumbnail

import (
	"context"
	"errors"
)

// Kind is what a source file is, and so how its thumbnail is drawn.
type Kind int

const (
	// Image: a JPEG, PNG, GIF or WebP picture.
	Image Kind = iota + 1
	// PDF: the first page.
	PDF
	// Video: a frame about a second in (the first frame of a shorter video).
	Video
)

func (k Kind) String() string {
	switch k {
	case Image:
		return "image"
	case PDF:
		return "pdf"
	case Video:
		return "video"
	}
	return "unknown"
}

// Renderer draws thumbnails.
type Renderer interface {
	// Render draws the file at srcPath, of the given kind, as a JPEG whose longest
	// side is size pixels (a picture already smaller is not enlarged). It stops
	// when ctx ends.
	Render(ctx context.Context, kind Kind, srcPath string, size int) ([]byte, error)
}

var (
	// ErrToolMissing: pdftoppm or ffmpeg is not installed where the service runs.
	ErrToolMissing = errors.New("thumbnail tool not installed")
	// ErrNotImage: the bytes are not a picture this package decodes.
	ErrNotImage = errors.New("not a decodable picture")
	// ErrTooManyPixels: the picture is too large to decode safely.
	ErrTooManyPixels = errors.New("picture has too many pixels")
	// ErrNoOutput: the tool ran but produced no usable picture.
	ErrNoOutput = errors.New("tool produced no picture")
)
