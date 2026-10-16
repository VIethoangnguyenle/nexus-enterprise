package rest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/pkg/httputil"
	"ngac-platform/services/drive/internal/domain"
)

type thumbSpy struct {
	DriveService
	got struct {
		id, ifNoneMatch string
		size            int
	}
	calls int
	out   *domain.Thumbnail
	err   error
}

func (s *thumbSpy) GetThumbnail(_ context.Context, id string, size int, inm string) (*domain.Thumbnail, error) {
	s.calls++
	s.got.id, s.got.size, s.got.ifNoneMatch = id, size, inm
	return s.out, s.err
}

func thumbRoutes(sp *thumbSpy, signedIn bool) *echo.Echo {
	h := NewHandler(sp, nil)
	e := httputil.NewEcho("drive")
	e.GET("/files/:fileId/thumbnail", h.GetThumbnail, func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if signedIn {
				httputil.SetClaims(c, &httputil.Claims{UserID: "u", NGACNodeID: "node-u"})
			}
			return next(c)
		}
	})
	return e
}

func getThumb(e *echo.Echo, url string, hdr ...string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, url, nil)
	if len(hdr) == 2 {
		req.Header.Set(hdr[0], hdr[1])
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

var jpegStub = []byte{0xFF, 0xD8, 0xFF, 0xD9}

func TestThumbnail_OKServesJPEGWithCacheHeaders(t *testing.T) {
	sp := &thumbSpy{out: &domain.Thumbnail{Data: jpegStub, ETag: `"t320-1-2"`}}
	rec := getThumb(thumbRoutes(sp, true), "/files/f1/thumbnail?size=640")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "image/jpeg", rec.Header().Get("Content-Type"))
	assert.Equal(t, "private, max-age=86400", rec.Header().Get("Cache-Control"))
	assert.Equal(t, `"t320-1-2"`, rec.Header().Get("ETag"))
	assert.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, jpegStub, rec.Body.Bytes())
	assert.Equal(t, "f1", sp.got.id)
	assert.Equal(t, 640, sp.got.size)
}

func TestThumbnail_SizeDefaultsTo320(t *testing.T) {
	sp := &thumbSpy{out: &domain.Thumbnail{Data: jpegStub, ETag: `"e"`}}
	require.Equal(t, http.StatusOK, getThumb(thumbRoutes(sp, true), "/files/f1/thumbnail").Code)
	assert.Equal(t, 320, sp.got.size)
}

func TestThumbnail_NotModifiedCarriesNoBody(t *testing.T) {
	sp := &thumbSpy{out: &domain.Thumbnail{ETag: `"e"`, NotModified: true}}
	rec := getThumb(thumbRoutes(sp, true), "/files/f1/thumbnail", "If-None-Match", `"e"`)
	require.Equal(t, http.StatusNotModified, rec.Code)
	assert.Equal(t, `"e"`, sp.got.ifNoneMatch)
	assert.Equal(t, `"e"`, rec.Header().Get("ETag"))
	assert.Equal(t, "private, max-age=86400", rec.Header().Get("Cache-Control"))
	assert.Empty(t, rec.Body.Bytes())
}

func TestThumbnail_BadSizeIsRejectedBeforeTheDomain(t *testing.T) {
	sp := &thumbSpy{}
	e := thumbRoutes(sp, true)
	assert.Equal(t, http.StatusBadRequest, getThumb(e, "/files/f1/thumbnail?size=abc").Code)
	assert.Zero(t, sp.calls)
}

func TestThumbnail_DomainRefusalsMapToStatuses(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		code int
	}{
		{"invalid size", domain.ErrInvalidInput, http.StatusBadRequest},
		{"denied", domain.ErrAccessDenied, http.StatusForbidden},
		{"not found", domain.ErrNotFound, http.StatusNotFound},
		{"unavailable", domain.ErrThumbnailUnavailable, http.StatusNotFound},
		{"busy", domain.ErrThumbnailBusy, http.StatusServiceUnavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := getThumb(thumbRoutes(&thumbSpy{err: tc.err}, true), "/files/f1/thumbnail?size=320")
			assert.Equal(t, tc.code, rec.Code)
			assert.Empty(t, rec.Header().Get("ETag"), "a refusal carries no validators")
			assert.NotContains(t, rec.Header().Get("Cache-Control"), "max-age=86400")
		})
	}
}

func TestThumbnail_UnavailableNamesTheReasonForTheScreen(t *testing.T) {
	rec := getThumb(thumbRoutes(&thumbSpy{err: domain.ErrThumbnailUnavailable}, true), "/files/f1/thumbnail")
	require.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Body.String(), `"reason":"thumbnail_unavailable"`)
}

func TestThumbnail_BusyTellsWhenToComeBack(t *testing.T) {
	rec := getThumb(thumbRoutes(&thumbSpy{err: domain.ErrThumbnailBusy}, true), "/files/f1/thumbnail")
	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.Equal(t, "2", rec.Header().Get("Retry-After"))
}

func TestThumbnail_DeniedRevealsNothingOfTheFile(t *testing.T) {
	rec := getThumb(thumbRoutes(&thumbSpy{err: domain.ErrAccessDenied}, true), "/files/f1/thumbnail")
	require.Equal(t, http.StatusForbidden, rec.Code)
	assert.NotContains(t, rec.Body.String(), "thumbnail")
	assert.NotContains(t, rec.Body.String(), "jpeg")
}

func TestThumbnail_AnonymousIsRefusedBeforeTheDomain(t *testing.T) {
	sp := &thumbSpy{out: &domain.Thumbnail{Data: jpegStub, ETag: `"e"`}}
	rec := getThumb(thumbRoutes(sp, false), "/files/f1/thumbnail")
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Zero(t, sp.calls)
}

func TestThumbnail_RouteIsMounted(t *testing.T) {
	e := echo.New()
	NewHandler(&thumbSpy{}, nil).RegisterRoutes(e, "secret")
	var found bool
	for _, r := range e.Routes() {
		if r.Method == http.MethodGet && r.Path == "/api/drive/files/:fileId/thumbnail" {
			found = true
		}
	}
	assert.True(t, found)
}
