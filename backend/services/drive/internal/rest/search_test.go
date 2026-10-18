package rest

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/pkg/httputil"
	pb "ngac-platform/proto/drive"
	"ngac-platform/services/drive/internal/domain"
)

// searchSpy captures search calls so the transport test stays independent of
// domain and policy. It embeds the DriveService interface, satisfying
// every other method with a nil-receiver panic (which is fine: none of them
// are called in these tests).
type searchSpy struct {
	DriveService
	calledWS    string
	calledQuery string
	calledLimit int
	result      *pb.DriveItemList
	err         error
}

func (s *searchSpy) SearchItems(_ context.Context, workspaceID, query string, limit int) (*pb.DriveItemList, error) {
	s.calledWS = workspaceID
	s.calledQuery = query
	s.calledLimit = limit
	if s.err != nil {
		return nil, s.err
	}
	if s.result != nil {
		return s.result, nil
	}
	return &pb.DriveItemList{}, nil
}

// searchRoutes sets up Echo the same way the main tests do, with only the
// search route and the auth middleware.
func searchRoutes(sp *searchSpy, signedIn bool) *echo.Echo {
	h := NewHandler(sp, nil)
	e := httputil.NewEcho("drive")
	as := func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if signedIn {
				httputil.SetClaims(c, &httputil.Claims{UserID: "u", NGACNodeID: "node-u"})
			}
			return next(c)
		}
	}
	e.GET("/api/drive/search", h.Search, as)
	return e
}

func doSearch(e *echo.Echo, qs string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/api/drive/search?"+qs, nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

// --- Deny tests (TDD: these must fail before implementation) ---

func TestSearch_AnonymousIsDenied(t *testing.T) {
	sp := &searchSpy{}
	e := searchRoutes(sp, false)
	rec := doSearch(e, "workspace_id=ws-1&q=report")
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestSearch_MissingWorkspaceID_IsBadRequest(t *testing.T) {
	sp := &searchSpy{}
	e := searchRoutes(sp, true)
	rec := doSearch(e, "q=report")
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestSearch_MissingQuery_IsBadRequest(t *testing.T) {
	sp := &searchSpy{}
	e := searchRoutes(sp, true)
	rec := doSearch(e, "workspace_id=ws-1")
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestSearch_NonMember_Gets403(t *testing.T) {
	// The domain returns ErrAccessDenied for a non-member; REST maps it to 403.
	sp := &searchSpy{err: domain.ErrAccessDenied}
	e := searchRoutes(sp, true)
	rec := doSearch(e, "workspace_id=ws-OTHER&q=secret")
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

// --- Success tests ---

func TestSearch_ForwardsParamsToService(t *testing.T) {
	sp := &searchSpy{}
	e := searchRoutes(sp, true)
	rec := doSearch(e, "workspace_id=ws-1&q=report")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "ws-1", sp.calledWS)
	assert.Equal(t, "report", sp.calledQuery)
	assert.Equal(t, 20, sp.calledLimit) // default limit
}

func TestSearch_ReturnsItems(t *testing.T) {
	sp := &searchSpy{
		result: &pb.DriveItemList{
			Items: []*pb.DriveItem{
				{Id: "item-1", Name: "Monthly Report", WorkspaceId: "ws-1", ItemType: "file"},
			},
		},
	}
	e := searchRoutes(sp, true)
	rec := doSearch(e, "workspace_id=ws-1&q=report")
	require.Equal(t, http.StatusOK, rec.Code)

	var resp struct {
		Items []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"items"`
	}
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	require.Len(t, resp.Items, 1)
	assert.Equal(t, "Monthly Report", resp.Items[0].Name)
}

func TestSearch_CustomLimit(t *testing.T) {
	sp := &searchSpy{}
	e := searchRoutes(sp, true)
	rec := doSearch(e, "workspace_id=ws-1&q=test&limit=5")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, 5, sp.calledLimit)
}

func TestSearch_LimitCappedAt50(t *testing.T) {
	sp := &searchSpy{}
	e := searchRoutes(sp, true)
	rec := doSearch(e, "workspace_id=ws-1&q=test&limit=999")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, 50, sp.calledLimit)
}

func TestSearch_InvalidLimitIsBadRequest(t *testing.T) {
	sp := &searchSpy{}
	e := searchRoutes(sp, true)
	rec := doSearch(e, "workspace_id=ws-1&q=test&limit=abc")
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}
