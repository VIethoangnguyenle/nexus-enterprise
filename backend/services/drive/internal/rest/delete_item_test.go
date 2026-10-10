package rest

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"ngac-platform/pkg/httputil"
	pb "ngac-platform/proto/drive"
	"ngac-platform/services/drive/internal/reason"
)

type deleteSpy struct {
	DriveService
	err error
}

func (s *deleteSpy) DeleteItem(context.Context, *pb.DeleteItemRequest) (*pb.Empty, error) {
	return &pb.Empty{}, s.err
}

func deleteRequest(t *testing.T, svc DriveService) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	c := echo.New().NewContext(httptest.NewRequest(http.MethodDelete, "/api/drive/items/f1/permanent", nil), rec)
	c.SetParamNames("itemId")
	c.SetParamValues("f1")
	httputil.SetClaims(c, &httputil.Claims{UserID: "u", NGACNodeID: "n"})
	if err := NewHandler(svc, nil).DeleteItem(c); err != nil {
		he, ok := err.(*echo.HTTPError)
		if !ok {
			t.Fatal(err)
		}
		rec.Code = he.Code
	}
	return rec
}

// A folder that still holds text documents answers 409 with a reason the
// screen can act on, not a 500 and not a bare message.
func TestDeleteItem_FolderWithDocumentsIs409WithReason(t *testing.T) {
	rec := deleteRequest(t, &deleteSpy{err: status.Error(codes.FailedPrecondition, reason.FolderHasDocuments)})
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", rec.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["reason"] != reason.FolderHasDocuments || body["error"] == "" {
		t.Errorf("body = %v, want reason %q and a sentence", body, reason.FolderHasDocuments)
	}
}

func TestDeleteItem_OtherRefusalsKeepTheirStatus(t *testing.T) {
	for code, want := range map[codes.Code]int{
		codes.PermissionDenied: http.StatusForbidden,
		codes.NotFound:         http.StatusNotFound,
		codes.Internal:         http.StatusInternalServerError,
	} {
		if got := deleteRequest(t, &deleteSpy{err: status.Error(code, "x")}).Code; got != want {
			t.Errorf("%v -> %d, want %d", code, got, want)
		}
	}
}

func TestDeleteItem_Success(t *testing.T) {
	if got := deleteRequest(t, &deleteSpy{}).Code; got != http.StatusOK {
		t.Errorf("status = %d, want 200", got)
	}
}
