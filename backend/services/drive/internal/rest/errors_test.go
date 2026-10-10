package rest

import (
	"context"
	"errors"
	"github.com/labstack/echo/v4"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"ngac-platform/pkg/httputil"
	pb "ngac-platform/proto/drive"
	"ngac-platform/services/drive/internal/domain"
)

type failingDrive struct {
	DriveService
	err error
}

func (f *failingDrive) GetItem(context.Context, *pb.GetItemRequest) (*pb.DriveItem, error) {
	return nil, f.err
}

func get(t *testing.T, err error) *httptest.ResponseRecorder {
	t.Helper()
	e := httputil.NewEcho("drive")
	h := NewHandler(&failingDrive{err: err}, nil)
	e.GET("/api/drive/items/:itemId", func(c echo.Context) error {
		httputil.SetClaims(c, &httputil.Claims{UserID: "u", NGACNodeID: "n"})
		return h.GetItem(c)
	})
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/drive/items/i1", nil))
	return rec
}

// A failure of ours never shows its text: the client gets a generic message and
// a request ID, the log gets the cause.
func TestInternalFailuresAreSanitised(t *testing.T) {
	for name, err := range map[string]error{
		"database":       errors.New(`load folder: ERROR: relation "drive_items" does not exist (SQLSTATE 42P01)`),
		"quota":          errors.New("storage quota exceeded: 10.0.0.5"),
		"classified 500": domain.ErrQuotaExceeded,
		"aborted":        domain.ErrAborted,
	} {
		rec := get(t, err)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("%s: status = %d", name, rec.Code)
		}
		body := rec.Body.String()
		for _, leak := range []string{"SQLSTATE", "drive_items", "10.0.0.5"} {
			if strings.Contains(body, leak) {
				t.Errorf("%s: body leaks %q: %s", name, leak, body)
			}
		}
		if !strings.Contains(body, `"internal error"`) || !strings.Contains(body, `"request_id"`) {
			t.Errorf("%s: body = %s", name, body)
		}
	}
}

func TestRefusalsKeepTheirMessageAndStatus(t *testing.T) {
	rec := get(t, httputil.ErrAccessDenied)
	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "access denied") {
		t.Errorf("%d %s", rec.Code, rec.Body)
	}
}
