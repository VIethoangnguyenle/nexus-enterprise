package rest

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"

	"ngac-platform/pkg/httputil"
	pb "ngac-platform/proto/drive"
	"ngac-platform/services/drive/internal/domain"
)

type sharedBySpy struct {
	DriveService
	list          *pb.DriveItemList
	err           error
	gotWS, gotOwn string
	called        bool
}

func (s *sharedBySpy) SharedWithMeBy(_ context.Context, ws, owner string) (*pb.DriveItemList, error) {
	s.called, s.gotWS, s.gotOwn = true, ws, owner
	return s.list, s.err
}

func serveSharedBy(spy *sharedBySpy, path string) *httptest.ResponseRecorder {
	h := NewHandler(spy, nil)
	e := httputil.NewEcho("drive")
	as := func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			httputil.SetClaims(c, &httputil.Claims{UserID: "u1", NGACNodeID: "n1"})
			return next(c)
		}
	}
	e.GET("/api/workspaces/:id/drive/shared-with-me", h.SharedWithMeBy, as)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func TestSharedByRoute_ForwardsWorkspaceAndOwner(t *testing.T) {
	spy := &sharedBySpy{list: &pb.DriveItemList{Items: []*pb.DriveItem{{Id: "i1", Name: "Báo cáo"}}}}
	rec := serveSharedBy(spy, "/api/workspaces/ws-1/drive/shared-with-me?owner=u2")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if spy.gotWS != "ws-1" || spy.gotOwn != "u2" {
		t.Fatalf("forwarded ws=%q owner=%q", spy.gotWS, spy.gotOwn)
	}
	if !strings.Contains(rec.Body.String(), "Báo cáo") {
		t.Fatalf("body lacks the item: %s", rec.Body.String())
	}
}

// Deny: a caller outside the workspace is 403 and gets no item.
func TestSharedByRoute_NonMemberIs403(t *testing.T) {
	spy := &sharedBySpy{err: fmt.Errorf("%w: not a member of this workspace", domain.ErrAccessDenied)}
	rec := serveSharedBy(spy, "/api/workspaces/ws-2/drive/shared-with-me?owner=u2")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403: %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "items") {
		t.Fatalf("a refusal carries no list: %s", rec.Body.String())
	}
}

// Without an owner the question is malformed, and the domain is not asked.
func TestSharedByRoute_MissingOwnerIs400(t *testing.T) {
	spy := &sharedBySpy{}
	rec := serveSharedBy(spy, "/api/workspaces/ws-1/drive/shared-with-me")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if spy.called {
		t.Fatal("the domain must not be asked without an owner")
	}
}
