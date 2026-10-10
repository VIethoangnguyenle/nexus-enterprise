package rest

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"ngac-platform/pkg/httputil"
	drivepb "ngac-platform/proto/drive"
)

type failingDrive struct {
	drivepb.DriveServiceClient
	err error
}

func (f failingDrive) ListFolder(context.Context, *drivepb.ListFolderRequest, ...grpc.CallOption) (*drivepb.DriveItemList, error) {
	return nil, f.err
}
func (f failingDrive) ConfirmFile(context.Context, *drivepb.ConfirmFileRequest, ...grpc.CallOption) (*drivepb.DriveItem, error) {
	return nil, f.err
}

func driveEcho(err error) *echo.Echo {
	e := httputil.NewEcho("document")
	h := NewHandler(failingDrive{err: err}, nil)
	e.GET("/api/workspaces/:id/documents", h.ListDocuments)
	e.POST("/api/documents/:docId/confirm", h.ConfirmUpload)
	return e
}

func call(e *echo.Echo, method, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(method, path, nil))
	return rec
}

// What the drive service answers about its own failures never reaches the
// client: not the text of a gRPC Internal, not the text of a transport error.
func TestProxyInternalFailuresAreSanitised(t *testing.T) {
	for name, err := range map[string]error{
		"grpc internal": status.Error(codes.Internal, `insert file: ERROR: relation "drive_items" does not exist (SQLSTATE 42P01)`),
		"unavailable":   status.Error(codes.Unavailable, "dial tcp 10.0.0.5:50057: connect: connection refused"),
		"not a status":  errors.New("pq: connection refused at 10.0.0.5"),
	} {
		rec := call(driveEcho(err), http.MethodGet, "/api/workspaces/w1/documents")
		body := rec.Body.String()
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("%s: status = %d", name, rec.Code)
		}
		for _, leak := range []string{"SQLSTATE", "drive_items", "10.0.0.5"} {
			if strings.Contains(body, leak) {
				t.Errorf("%s: body leaks %q: %s", name, leak, body)
			}
		}
		if !strings.Contains(body, `"request_id"`) || !strings.Contains(body, `"internal error"`) {
			t.Errorf("%s: body = %s", name, body)
		}
	}
}

// A refusal the drive service reports keeps its status, including the ones the
// proxy used to turn into a 500.
func TestProxyKeepsDrivesRefusals(t *testing.T) {
	for code, want := range map[codes.Code]int{
		codes.NotFound:           http.StatusNotFound,
		codes.PermissionDenied:   http.StatusForbidden,
		codes.InvalidArgument:    http.StatusBadRequest,
		codes.FailedPrecondition: http.StatusConflict,
		codes.Unauthenticated:    http.StatusUnauthorized,
	} {
		rec := call(driveEcho(status.Error(code, "no")), http.MethodPost, "/api/documents/d1/confirm")
		if rec.Code != want {
			t.Errorf("%v -> %d, want %d", code, rec.Code, want)
		}
	}
}
