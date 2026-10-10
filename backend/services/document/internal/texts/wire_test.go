package texts_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/ngac"
	"ngac-platform/pkg/httputil"
	"ngac-platform/services/document/internal/rest"
	"ngac-platform/services/document/internal/texts"
)

// The whole path a browser takes: signed token, REST route, service, policy
// answers, database. It pins the wire shapes the screens read.

type wire struct {
	e *echo.Echo
	f *fixture
}

func newWire(t *testing.T) *wire {
	t.Helper()
	f := newFixture(t)
	e := echo.New()
	rest.NewHandler(nil, f.svc).RegisterRoutes(e, httputil.DevJWTSecret)
	return &wire{e: e, f: f}
}

func (w *wire) token(c texts.Caller) string {
	claims := httputil.Claims{
		UserID: c.UserID, NGACNodeID: c.NGACNodeID,
		RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))},
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(httputil.DevJWTSecret))
	require.NoError(w.f.t, err)
	return signed
}

func (w *wire) do(c *texts.Caller, method, path, body string) (int, map[string]any) {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if c != nil {
		req.Header.Set("Authorization", "Bearer "+w.token(*c))
	}
	rec := httptest.NewRecorder()
	w.e.ServeHTTP(rec, req)
	var out map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return rec.Code, out
}

func TestWire_CreateOpenSaveConflictAndDelete(t *testing.T) {
	w := newWire(t)
	f := w.f
	f.policy.grant(f.owner.NGACNodeID, f.docsOA, ngac.OpRead, ngac.OpWrite)
	base := "/api/workspaces/" + f.wsID + "/documents/texts"

	code, doc := w.do(&f.owner, "POST", base, `{"title":"Quy trình"}`)
	require.Equal(t, http.StatusCreated, code)
	id := doc["id"].(string)
	assert.Equal(t, "Lê Thị Hoa", doc["owner_name"])
	assert.EqualValues(t, 1, doc["version"])
	assert.Equal(t, "draft", doc["status"])
	assert.Equal(t, true, doc["can_write"])

	code, got := w.do(&f.owner, "GET", "/api/documents/texts/"+id, "")
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, "", got["content"])

	code, saved := w.do(&f.owner, "PATCH", "/api/documents/texts/"+id, `{"base_version":1,"content":"<p>a</p>","title":"Mới"}`)
	require.Equal(t, http.StatusOK, code)
	assert.EqualValues(t, 2, saved["version"])
	assert.Equal(t, "Mới", saved["title"])

	code, conflict := w.do(&f.owner, "PATCH", "/api/documents/texts/"+id, `{"base_version":1,"content":"<p>stale</p>"}`)
	require.Equal(t, http.StatusConflict, code)
	assert.Equal(t, "version_conflict", conflict["reason"])
	current := conflict["current"].(map[string]any)
	assert.EqualValues(t, 2, current["version"])
	assert.Equal(t, "<p>a</p>", current["content"])

	code, list := w.do(&f.owner, "GET", base, "")
	require.Equal(t, http.StatusOK, code)
	docs := list["documents"].([]any)
	require.Len(t, docs, 1)
	assert.NotContains(t, docs[0], "content")

	code, _ = w.do(&f.owner, "DELETE", "/api/documents/texts/"+id, "")
	assert.Equal(t, http.StatusNoContent, code)
	code, _ = w.do(&f.owner, "GET", "/api/documents/texts/"+id, "")
	assert.Equal(t, http.StatusForbidden, code, "a deleted document answers like a forbidden one")
}

func TestWire_DeniesEveryRouteToSomeoneWithoutTheRight(t *testing.T) {
	w := newWire(t)
	f := w.f
	d := f.doc(f.folderA, f.oaA, "x")
	f.policy.grant(f.reader.NGACNodeID, f.oaA, ngac.OpRead)
	base := "/api/workspaces/" + f.wsID + "/documents/texts"
	path := "/api/documents/texts/" + d.ID

	// A reader can look but not touch.
	code, _ := w.do(&f.reader, "GET", path, "")
	assert.Equal(t, http.StatusOK, code)
	for name, call := range map[string]func() int{
		"create": func() int { c, _ := w.do(&f.reader, "POST", base, `{"folder_id":"`+f.folderA+`"}`); return c },
		"save":   func() int { c, _ := w.do(&f.reader, "PATCH", path, `{"base_version":1,"content":"x"}`); return c },
		"delete": func() int { c, _ := w.do(&f.reader, "DELETE", path, ""); return c },
	} {
		assert.Equal(t, http.StatusForbidden, call(), "reader: %s", name)
	}

	// Someone with no right at all gets nothing, and learns nothing from a 409.
	for name, call := range map[string]func() (int, map[string]any){
		"get": func() (int, map[string]any) { return w.do(&f.other, "GET", path, "") },
		"save": func() (int, map[string]any) {
			return w.do(&f.other, "PATCH", path, `{"base_version":99,"content":"x"}`)
		},
		"delete": func() (int, map[string]any) { return w.do(&f.other, "DELETE", path, "") },
	} {
		c, body := call()
		assert.Equal(t, http.StatusForbidden, c, "outsider: %s", name)
		assert.NotContains(t, body, "current", "outsider: %s", name)
	}
	_, list := w.do(&f.other, "GET", base, "")
	assert.Empty(t, list["documents"])

	// No token, no answer.
	for _, r := range [][2]string{{"GET", path}, {"PATCH", path}, {"DELETE", path}, {"GET", base}, {"POST", base}} {
		c, _ := w.do(nil, r[0], r[1], `{}`)
		assert.Equal(t, http.StatusUnauthorized, c, "%s %s", r[0], r[1])
	}

	// The document is untouched by all of it.
	got, err := f.svc.Get(bg, f.owner, d.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, got.Version)
}
