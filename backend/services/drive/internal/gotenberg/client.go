// Package gotenberg is the drive service's adapter to a Gotenberg server, the
// internal LibreOffice container that turns Office documents into PDF. The
// document being converted never leaves the deployment.
package gotenberg

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strings"
	"sync"

	"ngac-platform/services/drive/internal/domain"
)

// Client converts documents through a Gotenberg server.
type Client struct {
	baseURL string
	http    *http.Client
}

// New returns a client for the Gotenberg server at baseURL. Deadlines come from
// the context of each call.
func New(baseURL string) *Client {
	return &Client{baseURL: strings.TrimRight(baseURL, "/"), http: &http.Client{}}
}

// ToPDF converts src to PDF and writes the PDF to dst, which receives at most
// maxBytes: a longer answer is domain.ErrPDFTooLarge. filename must carry the document's real extension:
// LibreOffice picks its import filter from it. A server that cannot be reached
// is domain.ErrConverterUnreachable; one that answers but cannot convert the
// document (or runs out of time) is domain.ErrConversionFailed.
func (c *Client) ToPDF(ctx context.Context, filename string, src io.Reader, dst io.Writer, maxBytes int64) error {
	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)
	srcErr := &errReader{r: src}
	go func() {
		pw.CloseWithError(writeForm(mw, filename, srcErr))
	}()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/forms/libreoffice/convert", pr)
	if err != nil {
		pr.Close()
		return fmt.Errorf("build conversion request: %w", err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())

	resp, err := c.http.Do(req)
	if err != nil {
		pr.CloseWithError(err)
		// The document could not be read, or time ran out: the server was reached
		// (or there is no telling), so this is a failed conversion, not an outage.
		if ctx.Err() != nil || srcErr.err() != nil {
			return fmt.Errorf("%w: %v", domain.ErrConversionFailed, err)
		}
		return fmt.Errorf("%w: %v", domain.ErrConverterUnreachable, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		detail, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("%w: gotenberg answered %d: %s", domain.ErrConversionFailed, resp.StatusCode, strings.TrimSpace(string(detail)))
	}
	n, err := io.Copy(dst, io.LimitReader(resp.Body, maxBytes))
	if err != nil {
		return fmt.Errorf("%w: read conversion: %v", domain.ErrConversionFailed, err)
	}
	// One more byte means the answer is longer than allowed. It is never read on.
	if more, _ := io.ReadFull(resp.Body, make([]byte, 1)); more > 0 {
		return fmt.Errorf("%w: converted document exceeds %d bytes", domain.ErrPDFTooLarge, maxBytes)
	}
	if n == 0 {
		return fmt.Errorf("%w: empty conversion", domain.ErrConversionFailed)
	}
	return nil
}

// writeForm streams src as the "files" part of a multipart form.
func writeForm(mw *multipart.Writer, filename string, src io.Reader) error {
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="files"; filename=%q`, sanitizeName(filename)))
	h.Set("Content-Type", "application/octet-stream")
	part, err := mw.CreatePart(h)
	if err != nil {
		return err
	}
	if _, err := io.Copy(part, src); err != nil {
		return err
	}
	return mw.Close()
}

// sanitizeName keeps the extension and drops anything that could confuse the
// form or the server's temp file: path separators and control characters.
func sanitizeName(name string) string {
	name = strings.Map(func(r rune) rune {
		if r < 0x20 || r == '/' || r == '\\' || r == '"' {
			return '_'
		}
		return r
	}, name)
	if name == "" {
		return "document"
	}
	return name
}

// errReader remembers the first non-EOF error of the reader it wraps.
type errReader struct {
	r  io.Reader
	mu sync.Mutex
	e  error
}

func (r *errReader) Read(p []byte) (int, error) {
	n, err := r.r.Read(p)
	if err != nil && err != io.EOF {
		r.mu.Lock()
		r.e = err
		r.mu.Unlock()
	}
	return n, err
}

func (r *errReader) err() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.e
}

var _ domain.Converter = (*Client)(nil)
