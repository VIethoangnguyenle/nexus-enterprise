package rest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/pkg/httputil"
	"ngac-platform/services/document/internal/texts"
)

type fakeTexts struct {
	err    error
	who    texts.Caller
	change texts.Change
	folder string
	calls  int
	cursor string
	limit  int
	noRead bool
}

func (f *fakeTexts) doc() *texts.Doc {
	return &texts.Doc{ID: "d1", WorkspaceID: "w1", Title: "T", Content: "<p>x</p>", Version: 3, Status: "draft",
		OwnerID: "u1", OwnerName: "Hoa", CreatedAt: time.Unix(0, 0), UpdatedAt: time.Unix(60, 0)}
}
func (f *fakeTexts) Create(_ context.Context, who texts.Caller, _, folder, _ string) (*texts.Doc, error) {
	f.calls++
	f.who, f.folder = who, folder
	return f.doc(), f.err
}
func (f *fakeTexts) Get(_ context.Context, who texts.Caller, _ string) (*texts.Opened, error) {
	f.calls++
	f.who = who
	return &texts.Opened{Doc: f.doc(), CanRead: true, CanWrite: true}, f.err
}
func (f *fakeTexts) List(_ context.Context, who texts.Caller, _, _, cursor string, limit int) (*texts.Page, error) {
	f.calls++
	f.who, f.cursor, f.limit = who, cursor, limit
	return &texts.Page{Docs: []*texts.Opened{{Doc: f.doc(), CanRead: true, CanWrite: true}}, Next: "next-page"}, f.err
}
func (f *fakeTexts) Count(_ context.Context, who texts.Caller, _, scope string) (int, error) {
	f.calls++
	f.who = who
	return 7, f.err
}
func (f *fakeTexts) Update(_ context.Context, who texts.Caller, _ string, ch texts.Change) (*texts.Opened, error) {
	f.calls++
	f.who, f.change = who, ch
	return &texts.Opened{Doc: f.doc(), CanRead: !f.noRead, CanWrite: true}, f.err
}
func (f *fakeTexts) Delete(_ context.Context, who texts.Caller, _ string) error {
	f.calls++
	f.who = who
	return f.err
}

func serve(t *testing.T, svc TextService, authed bool, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	e := echo.New()
	api := e.Group("/api", func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if authed {
				httputil.SetClaims(c, &httputil.Claims{UserID: "u-claims", NGACNodeID: "n-claims", TenantID: "t-claims"})
			}
			return next(c)
		}
	})
	NewHandler(nil, svc).registerTextRoutes(api)
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

var textRoutes = []struct{ method, path, body string }{
	{"GET", "/api/workspaces/w1/documents/texts", ""},
	{"POST", "/api/workspaces/w1/documents/texts", `{"title":"x"}`},
	{"GET", "/api/documents/texts/d1", ""},
	{"PATCH", "/api/documents/texts/d1", `{"base_version":1,"title":"x"}`},
	{"DELETE", "/api/documents/texts/d1", ""},
}

func TestTextRoutes_NoClaimsIs401AndNeverReachesTheService(t *testing.T) {
	for _, r := range textRoutes {
		t.Run(r.method+" "+r.path, func(t *testing.T) {
			f := &fakeTexts{}
			rec := serve(t, f, false, r.method, r.path, r.body)
			assert.Equal(t, http.StatusUnauthorized, rec.Code)
			assert.Zero(t, f.calls)
		})
	}
}

func TestTextRoutes_CallerComesFromClaimsNotTheBody(t *testing.T) {
	f := &fakeTexts{}
	rec := serve(t, f, true, "PATCH", "/api/documents/texts/d1",
		`{"base_version":3,"title":"New","content":"<p>a</p>","user_id":"u-evil","ngac_node_id":"n-evil","owner_id":"u-evil"}`)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, texts.Caller{UserID: "u-claims", NGACNodeID: "n-claims", TenantID: "t-claims"}, f.who)
	assert.Equal(t, 3, f.change.BaseVersion)
	assert.Equal(t, "New", *f.change.Title)
}

func TestTextRoutes_ErrorMapping(t *testing.T) {
	cases := []struct {
		name string
		err  error
		code int
	}{
		{"denied", fmt.Errorf("%w: no write access", httputil.ErrAccessDenied), http.StatusForbidden},
		{"invalid", fmt.Errorf("%w: title is required", httputil.ErrInvalidInput), http.StatusBadRequest},
		{"database failure", errors.New(`pq: relation "text_documents" does not exist`), http.StatusInternalServerError},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := serve(t, &fakeTexts{err: c.err}, true, "GET", "/api/documents/texts/d1", "")
			assert.Equal(t, c.code, rec.Code)
			if c.code == http.StatusInternalServerError || c.code == http.StatusForbidden {
				assert.NotContains(t, rec.Body.String(), "text_documents", "no internal text reaches the client")
				assert.NotContains(t, rec.Body.String(), "no write access")
			}
		})
	}
}

func TestTextRoutes_ConflictIs409WithTheCurrentDocument(t *testing.T) {
	f := &fakeTexts{}
	f.err = &texts.ConflictError{Current: f.doc(), Readable: true}

	rec := serve(t, f, true, "PATCH", "/api/documents/texts/d1", `{"base_version":1,"content":"<p>mine</p>"}`)

	require.Equal(t, http.StatusConflict, rec.Code)
	var body struct {
		Error   string `json:"error"`
		Reason  string `json:"reason"`
		Current struct {
			Version int    `json:"version"`
			Content string `json:"content"`
		} `json:"current"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "version_conflict", body.Reason)
	assert.Equal(t, 3, body.Current.Version)
	assert.Equal(t, "<p>x</p>", body.Current.Content)
	assert.NotEmpty(t, body.Error)
}

func TestTextRoutes_ListCarriesNoContent(t *testing.T) {
	rec := serve(t, &fakeTexts{}, true, "GET", "/api/workspaces/w1/documents/texts", "")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.NotContains(t, rec.Body.String(), `"content"`)
	assert.Contains(t, rec.Body.String(), `"owner_name":"Hoa"`)
}

func TestTextRoutes_GetCarriesContentAndWriteFlag(t *testing.T) {
	rec := serve(t, &fakeTexts{}, true, "GET", "/api/documents/texts/d1", "")
	require.Equal(t, http.StatusOK, rec.Code)
	var body struct {
		Content  string `json:"content"`
		CanWrite bool   `json:"can_write"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "<p>x</p>", body.Content)
	assert.True(t, body.CanWrite)
}

func TestTextRoutes_OversizedBodyIsRefused(t *testing.T) {
	f := &fakeTexts{}
	big := `{"base_version":1,"content":"` + strings.Repeat("a", 3<<20) + `"}`
	rec := serve(t, f, true, "PATCH", "/api/documents/texts/d1", big)
	assert.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
	assert.Zero(t, f.calls)
}

func TestTextRoutes_MalformedBodyIs400(t *testing.T) {
	f := &fakeTexts{}
	rec := serve(t, f, true, "PATCH", "/api/documents/texts/d1", `{not json`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Zero(t, f.calls)
}

func TestTextRoutes_ConflictForSomeoneWhoMayNotReadIsTheVersionAlone(t *testing.T) {
	f := &fakeTexts{}
	f.err = &texts.ConflictError{Current: &texts.Doc{ID: "d1", Version: 9}}
	rec := serve(t, f, true, "PATCH", "/api/documents/texts/d1", `{"base_version":1,"content":"x"}`)
	require.Equal(t, http.StatusConflict, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, map[string]any{"version": float64(9)}, body["current"])
	assert.NotContains(t, rec.Body.String(), "content")
}

func TestTextRoutes_SaveByAWriteOnlyCallerCarriesNoText(t *testing.T) {
	f := &fakeTexts{noRead: true}
	rec := serve(t, f, true, "PATCH", "/api/documents/texts/d1", `{"base_version":1,"content":"x"}`)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.NotContains(t, rec.Body.String(), `"content"`)
}

func TestTextRoutes_ListPassesTheCursorAndLimitAndReturnsTheNext(t *testing.T) {
	f := &fakeTexts{}
	rec := serve(t, f, true, "GET", "/api/workspaces/w1/documents/texts?scope=drafts&cursor=abc&limit=25", "")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "abc", f.cursor)
	assert.Equal(t, 25, f.limit)
	assert.Contains(t, rec.Body.String(), `"next_cursor":"next-page"`)
}

func TestTextRoutes_ABadLimitIs400AndNeverReachesTheService(t *testing.T) {
	for _, q := range []string{"limit=0", "limit=-3", "limit=lots"} {
		f := &fakeTexts{}
		rec := serve(t, f, true, "GET", "/api/workspaces/w1/documents/texts?"+q, "")
		assert.Equal(t, http.StatusBadRequest, rec.Code, q)
		assert.Zero(t, f.calls)
	}
}

func TestTextRoutes_CountIsAGuardedRoute(t *testing.T) {
	f := &fakeTexts{}
	rec := serve(t, f, true, "GET", "/api/workspaces/w1/documents/texts/count?scope=drafts", "")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"count":7}`, rec.Body.String())
	assert.Equal(t, texts.Caller{UserID: "u-claims", NGACNodeID: "n-claims", TenantID: "t-claims"}, f.who)

	g := &fakeTexts{}
	rec = serve(t, g, false, "GET", "/api/workspaces/w1/documents/texts/count", "")
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Zero(t, g.calls)
}
