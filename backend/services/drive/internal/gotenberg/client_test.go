package gotenberg_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/services/drive/internal/domain"
	"ngac-platform/services/drive/internal/gotenberg"
)

func TestToPDFPostsTheDocumentToTheLibreOfficeRoute(t *testing.T) {
	var gotName, gotBody, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		require.NoError(t, r.ParseMultipartForm(1<<20))
		f, h, err := r.FormFile("files")
		require.NoError(t, err)
		b, _ := io.ReadAll(f)
		gotName, gotBody = h.Filename, string(b)
		w.Header().Set("Content-Type", "application/pdf")
		_, _ = w.Write([]byte("%PDF-1.7 fake"))
	}))
	defer srv.Close()

	var pdf bytes.Buffer
	err := gotenberg.New(srv.URL+"/").ToPDF(context.Background(), "Báo cáo Q3.docx", strings.NewReader("docx-bytes"), &pdf, 1<<20)
	require.NoError(t, err)
	assert.Equal(t, "%PDF-1.7 fake", pdf.String())
	assert.Equal(t, "/forms/libreoffice/convert", gotPath)
	assert.Equal(t, "Báo cáo Q3.docx", gotName, "the extension (and name) must reach LibreOffice")
	assert.Equal(t, "docx-bytes", gotBody)
}

func TestToPDFPathSeparatorsInTheNameAreNeutralised(t *testing.T) {
	var gotName string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseMultipartForm(1 << 20)
		_, h, _ := r.FormFile("files")
		gotName = h.Filename
		_, _ = w.Write([]byte("%PDF"))
	}))
	defer srv.Close()
	err := gotenberg.New(srv.URL).ToPDF(context.Background(), "../../etc/x.docx", strings.NewReader("x"), io.Discard, 1<<20)
	require.NoError(t, err)
	assert.NotContains(t, gotName, "/")
	assert.True(t, strings.HasSuffix(gotName, ".docx"))
}

func TestToPDFServerRefusalIsAFailedConversion(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "corrupt file", http.StatusInternalServerError)
	}))
	defer srv.Close()
	err := gotenberg.New(srv.URL).ToPDF(context.Background(), "a.docx", strings.NewReader("x"), io.Discard, 1<<20)
	assert.ErrorIs(t, err, domain.ErrConversionFailed)
}

func TestToPDFEmptyAnswerIsAFailedConversion(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer srv.Close()
	err := gotenberg.New(srv.URL).ToPDF(context.Background(), "a.docx", strings.NewReader("x"), io.Discard, 1<<20)
	assert.ErrorIs(t, err, domain.ErrConversionFailed)
}

func TestToPDFUnreachableServer(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL
	srv.Close() // nothing listens any more
	err := gotenberg.New(url).ToPDF(context.Background(), "a.docx", strings.NewReader("x"), io.Discard, 1<<20)
	assert.ErrorIs(t, err, domain.ErrConverterUnreachable)
}

func TestToPDFTimeoutIsAFailedConversion(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { <-release }))
	defer srv.Close()
	defer close(release)
	ctx, cancel := context.WithTimeout(context.Background(), 50*1e6)
	defer cancel()
	err := gotenberg.New(srv.URL).ToPDF(ctx, "a.docx", strings.NewReader("x"), io.Discard, 1<<20)
	assert.ErrorIs(t, err, domain.ErrConversionFailed)
	assert.NotErrorIs(t, err, domain.ErrConverterUnreachable)
}

func TestToPDFUnreadableSourceIsAFailedConversion(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()
	err := gotenberg.New(srv.URL).ToPDF(context.Background(), "a.docx", iotest.ErrReader(errors.New("storage went away")), io.Discard, 1<<20)
	assert.ErrorIs(t, err, domain.ErrConversionFailed)
	assert.NotErrorIs(t, err, domain.ErrConverterUnreachable)
}

func TestToPDFAnswerOverTheCapIsNotBuffered(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(bytes.Repeat([]byte("p"), 1<<20))
	}))
	defer srv.Close()
	var got bytes.Buffer
	err := gotenberg.New(srv.URL).ToPDF(context.Background(), "a.docx", strings.NewReader("x"), &got, 1000)
	assert.ErrorIs(t, err, domain.ErrPDFTooLarge)
	assert.LessOrEqual(t, got.Len(), 1000, "no more than the cap is ever written out")
}

func TestToPDFAnswerExactlyAtTheCapIsKept(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(bytes.Repeat([]byte("p"), 1000))
	}))
	defer srv.Close()
	var got bytes.Buffer
	require.NoError(t, gotenberg.New(srv.URL).ToPDF(context.Background(), "a.docx", strings.NewReader("x"), &got, 1000))
	assert.Equal(t, 1000, got.Len())
}
