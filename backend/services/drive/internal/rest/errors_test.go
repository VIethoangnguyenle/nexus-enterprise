package rest

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/labstack/echo/v4"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"ngac-platform/pkg/httputil"
	pb "ngac-platform/proto/drive"
	"ngac-platform/services/drive/internal/domain"
	"ngac-platform/services/drive/internal/reason"
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
		"quota text":     errors.New("storage quota exceeded: 10.0.0.5"),
		"lock timed out": domain.ErrUnavailable,
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

// A full quota and an item changed under the caller are refusals with a reason
// the screen can act on, not a failure of ours.
func TestQuotaAndAbortedRefusalsCarryAReason(t *testing.T) {
	for name, tc := range map[string]struct {
		err        error
		wantStatus int
		wantReason string
	}{
		"quota":   {domain.ErrQuotaExceeded, http.StatusRequestEntityTooLarge, reason.QuotaExceeded},
		"aborted": {domain.ErrAborted, http.StatusConflict, reason.ItemChanged},
	} {
		rec := get(t, tc.err)
		if rec.Code != tc.wantStatus {
			t.Fatalf("%s: status = %d, want %d", name, rec.Code, tc.wantStatus)
		}
		var body map[string]string
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("%s: %v: %s", name, err, rec.Body)
		}
		if body["reason"] != tc.wantReason || body["message"] == "" {
			t.Errorf("%s: body = %v, want reason %q and a message", name, body, tc.wantReason)
		}
	}
}

// Only the class decides the answer: a database error that merely mentions the
// quota stays a 500.
func TestUnclassifiedErrorsMentioningQuotaStayInternal(t *testing.T) {
	if rec := get(t, errors.New("check quota: connection refused")); rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", rec.Code)
	}
}
