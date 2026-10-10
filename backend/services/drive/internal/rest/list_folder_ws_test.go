package rest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"

	"ngac-platform/pkg/httputil"
	pb "ngac-platform/proto/drive"
)

type listFolderSpy struct {
	DriveService
	got *pb.ListFolderRequest
}

func (s *listFolderSpy) ListFolder(_ context.Context, req *pb.ListFolderRequest) (*pb.DriveItemList, error) {
	s.got = req
	return &pb.DriveItemList{}, nil
}

// GET /api/drive/folders/:folderId forwards the optional ?ws= so the drive can
// refuse a folder of another workspace; without it the request carries none.
func TestListFolder_ForwardsOptionalWorkspace(t *testing.T) {
	for query, want := range map[string]string{"?ws=ws-1": "ws-1", "": ""} {
		spy := &listFolderSpy{}
		h := NewHandler(spy, nil)
		e := echo.New()
		c := e.NewContext(httptest.NewRequest(http.MethodGet, "/api/drive/folders/f1"+query, nil), httptest.NewRecorder())
		c.SetParamNames("folderId")
		c.SetParamValues("f1")
		httputil.SetClaims(c, &httputil.Claims{UserID: "u", NGACNodeID: "n"})

		if err := h.ListFolder(c); err != nil {
			t.Fatal(err)
		}
		if spy.got.GetFolderId() != "f1" || spy.got.GetWorkspaceId() != want {
			t.Errorf("query %q: forwarded folder=%q workspace=%q, want f1 / %q",
				query, spy.got.GetFolderId(), spy.got.GetWorkspaceId(), want)
		}
	}
}
