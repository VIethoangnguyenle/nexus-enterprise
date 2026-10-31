package rest

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/pkg/httputil"
	pb "ngac-platform/proto/drive"
	"ngac-platform/services/drive/internal/domain"
)

// starsSpy records what reaches the domain for the star, recent and quick
// access routes. Every other DriveService method is the embedded nil (panics).
type starsSpy struct {
	DriveService

	called  []string
	err     error
	list    *pb.DriveItemList
	entries []*domain.QuickAccessEntry
}

func (s *starsSpy) note(what string) { s.called = append(s.called, what) }

func (s *starsSpy) StarItem(_ context.Context, itemID string) error {
	s.note("star " + itemID)
	return s.err
}

func (s *starsSpy) UnstarItem(_ context.Context, itemID string) error {
	s.note("unstar " + itemID)
	return s.err
}

func (s *starsSpy) ListStarredItems(_ context.Context, ws string) (*pb.DriveItemList, error) {
	s.note("starred " + ws)
	if s.err != nil {
		return nil, s.err
	}
	return s.list, nil
}

func (s *starsSpy) RecentFiles(_ context.Context, ws string) (*pb.DriveItemList, error) {
	s.note("recent " + ws)
	if s.err != nil {
		return nil, s.err
	}
	return s.list, nil
}

func (s *starsSpy) QuickAccessItems(_ context.Context, ws string) ([]*domain.QuickAccessEntry, error) {
	s.note("quick " + ws)
	if s.err != nil {
		return nil, s.err
	}
	return s.entries, nil
}

// starsRoutes mounts the routes the way RegisterRoutes does, with a fake sign-in.
func starsRoutes(svc DriveService, signedIn bool) *echo.Echo {
	h := NewHandler(svc, nil)
	e := httputil.NewEcho("drive")
	as := func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if signedIn {
				httputil.SetClaims(c, &httputil.Claims{UserID: "u", NGACNodeID: "node-u"})
			}
			return next(c)
		}
	}
	e.POST("/api/drive/items/:itemId/star", h.StarItem, as)
	e.DELETE("/api/drive/items/:itemId/star", h.UnstarItem, as)
	e.GET("/api/drive/starred", h.ListStarred, as)
	e.GET("/api/drive/recent", h.RecentFiles, as)
	e.GET("/api/drive/quick-access", h.QuickAccess, as)
	return e
}

func doStars(e *echo.Echo, method, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(method, path, nil))
	return rec
}

var starRoutes = []struct{ method, path string }{
	{http.MethodPost, "/api/drive/items/i1/star"},
	{http.MethodDelete, "/api/drive/items/i1/star"},
	{http.MethodGet, "/api/drive/starred?workspace_id=ws-1"},
	{http.MethodGet, "/api/drive/recent?workspace_id=ws-1"},
	{http.MethodGet, "/api/drive/quick-access?workspace_id=ws-1"},
}

func TestStarRoutes_AnonymousNeverReachesTheDomain(t *testing.T) {
	for _, r := range starRoutes {
		sp := &starsSpy{}
		rec := doStars(starsRoutes(sp, false), r.method, r.path)
		assert.Equal(t, http.StatusUnauthorized, rec.Code, r.path)
		assert.Empty(t, sp.called, r.path)
	}
}

func TestStarRoutes_DomainDenialIs403(t *testing.T) {
	for _, r := range starRoutes {
		sp := &starsSpy{err: domain.ErrAccessDenied}
		rec := doStars(starsRoutes(sp, true), r.method, r.path)
		assert.Equal(t, http.StatusForbidden, rec.Code, r.path)
	}
}

func TestStarItem_MissingItemIs404(t *testing.T) {
	sp := &starsSpy{err: domain.ErrNotFound}
	rec := doStars(starsRoutes(sp, true), http.MethodPost, "/api/drive/items/gone/star")
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestListRoutes_WorkspaceIsRequired(t *testing.T) {
	for _, p := range []string{"/api/drive/starred", "/api/drive/recent", "/api/drive/quick-access"} {
		sp := &starsSpy{}
		rec := doStars(starsRoutes(sp, true), http.MethodGet, p)
		assert.Equal(t, http.StatusBadRequest, rec.Code, p)
		assert.Empty(t, sp.called, p)
	}
}

func TestStarAndUnstar_PassTheItemAndAnswer204(t *testing.T) {
	sp := &starsSpy{}
	e := starsRoutes(sp, true)
	assert.Equal(t, http.StatusNoContent, doStars(e, http.MethodPost, "/api/drive/items/i1/star").Code)
	assert.Equal(t, http.StatusNoContent, doStars(e, http.MethodDelete, "/api/drive/items/i1/star").Code)
	assert.Equal(t, []string{"star i1", "unstar i1"}, sp.called)
}

func TestListStarred_ReturnsTheItems(t *testing.T) {
	sp := &starsSpy{list: &pb.DriveItemList{Items: []*pb.DriveItem{{Id: "a", Name: "Biên bản.docx"}}}}
	rec := doStars(starsRoutes(sp, true), http.MethodGet, "/api/drive/starred?workspace_id=ws-1")
	require.Equal(t, http.StatusOK, rec.Code)
	var body struct {
		Items []struct{ Name string } `json:"items"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Len(t, body.Items, 1)
	assert.Equal(t, "Biên bản.docx", body.Items[0].Name)
	assert.Equal(t, []string{"starred ws-1"}, sp.called)
}

type fakeNames map[string]string

func (f fakeNames) DisplayNames(_ context.Context, keys []string) (map[string]string, error) {
	out := map[string]string{}
	for _, k := range keys {
		if n, ok := f[k]; ok {
			out[k] = n
		}
	}
	return out, nil
}

// The sharer is named, and their node id never reaches the client.
func TestQuickAccess_NamesTheSharerWithoutSendingTheirNode(t *testing.T) {
	sp := &starsSpy{entries: []*domain.QuickAccessEntry{
		{Item: &pb.DriveItem{Id: "a", Name: "Ảnh khai trương.jpg", OwnerId: "user-lan"}, Reason: domain.ReasonShared, SharedBy: "node-lan"},
		{Item: &pb.DriveItem{Id: "b", Name: "Đối soát.xlsx", OwnerId: "u"}, Reason: domain.ReasonYours},
	}}
	svc := WithOwnerNames(sp, fakeNames{"node-lan": "Thu Lan", "user-lan": "Thu Lan", "u": "Minh Đức"})
	rec := doStars(starsRoutes(svc, true), http.MethodGet, "/api/drive/quick-access?workspace_id=ws-1")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.NotContains(t, rec.Body.String(), "node-lan")

	var body struct {
		Entries []struct {
			Item struct {
				Name      string `json:"name"`
				OwnerName string `json:"owner_name"`
			} `json:"item"`
			Reason       string `json:"reason"`
			SharedByName string `json:"shared_by_name"`
		} `json:"entries"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Len(t, body.Entries, 2)
	assert.Equal(t, "shared", body.Entries[0].Reason)
	assert.Equal(t, "Thu Lan", body.Entries[0].SharedByName)
	assert.Equal(t, "Thu Lan", body.Entries[0].Item.OwnerName)
	assert.Equal(t, "yours", body.Entries[1].Reason)
	assert.Empty(t, body.Entries[1].SharedByName)
	assert.True(t, strings.Contains(rec.Body.String(), `"entries"`))
}

func TestQuickAccess_EmptyIsAnEmptyList(t *testing.T) {
	sp := &starsSpy{}
	rec := doStars(starsRoutes(WithOwnerNames(sp, fakeNames{}), true), http.MethodGet, "/api/drive/quick-access?workspace_id=ws-1")
	require.Equal(t, http.StatusOK, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Contains(t, body, "entries")
}
