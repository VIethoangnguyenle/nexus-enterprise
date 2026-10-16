package httputil_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/labstack/echo/v4"

	"ngac-platform/pkg/httputil"
)

func eq[T comparable](t *testing.T, got, want T, msg string) {
	t.Helper()
	if got != want {
		t.Fatalf("%s: got %v, want %v", msg, got, want)
	}
}

func ne[T comparable](t *testing.T, got, notWant T, msg string) {
	t.Helper()
	if got == notWant {
		t.Fatalf("%s: got %v, want anything else", msg, got)
	}
}

func noErr(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

type memVersions struct {
	mu   sync.Mutex
	vals map[string]int64
	fail bool
}

func newMem() *memVersions { return &memVersions{vals: map[string]int64{}} }

func (m *memVersions) Get(_ context.Context, keys ...string) ([]int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.fail {
		return nil, errors.New("redis down")
	}
	out := make([]int64, len(keys))
	for i, k := range keys {
		out[i] = m.vals[k]
	}
	return out, nil
}

func (m *memVersions) Bump(_ context.Context, keys ...string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, k := range keys {
		m.vals[k]++
	}
	return nil
}

// rig serves GET /data scoped by the X-Tenant / X-User headers and counts how
// often the handler (the store behind it) runs.
type rig struct {
	e     *echo.Echo
	hits  int
	v     httputil.Versions
	clock time.Time
	// status is what the handler answers.
	status int
}

func newRig(v httputil.Versions) *rig {
	r := &rig{v: v, clock: time.Unix(1_800_000_000, 0), status: http.StatusOK}
	mw := httputil.ConditionalGET(httputil.ETagConfig{
		Versions: v,
		Keys: func(c echo.Context) ([]string, bool) {
			return []string{
				httputil.VersionKey("t", c.Request().Header.Get("X-Tenant")),
				httputil.VersionKey("t", c.Request().Header.Get("X-Tenant"), "u", c.Request().Header.Get("X-User")),
			}, true
		},
		Now: func() time.Time { return r.clock },
	})
	r.e = echo.New()
	r.e.GET("/data", func(c echo.Context) error {
		r.hits++
		return c.JSON(r.status, map[string]string{"ok": "yes"})
	}, mw)
	r.e.GET("/other", func(c echo.Context) error {
		r.hits++
		return c.JSON(http.StatusOK, map[string]string{"ok": "other"})
	}, mw)
	r.e.POST("/data", func(c echo.Context) error { return c.NoContent(http.StatusNoContent) }, mw)
	return r
}

func (r *rig) get(tenant, user, inm string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/data", nil)
	req.Header.Set("X-Tenant", tenant)
	req.Header.Set("X-User", user)
	if inm != "" {
		req.Header.Set("If-None-Match", inm)
	}
	rec := httptest.NewRecorder()
	r.e.ServeHTTP(rec, req)
	return rec
}

func TestConditionalGET_MatchIs304AndSkipsTheStore(t *testing.T) {
	r := newRig(newMem())
	first := r.get("t1", "u1", "")
	eq(t, first.Code, http.StatusOK, "")
	tag := first.Header().Get("ETag")
	ne(t, tag, "", "first response has an ETag")
	eq(t, r.hits, 1, "")

	again := r.get("t1", "u1", tag)
	eq(t, again.Code, http.StatusNotModified, "")
	eq(t, again.Body.String(), "", "")
	eq(t, again.Header().Get("ETag"), tag, "")
	eq(t, r.hits, 1, "a 304 must not run the handler")

	weak := r.get("t1", "u1", `W/`+tag+`, "other"`)
	eq(t, weak.Code, http.StatusNotModified, "a listed or weak validator still matches")
}

func TestConditionalGET_BumpChangesTheTag(t *testing.T) {
	v := newMem()
	r := newRig(v)
	tag := r.get("t1", "u1", "").Header().Get("ETag")

	noErr(t, v.Bump(context.Background(), httputil.VersionKey("t", "t1")))
	after := r.get("t1", "u1", tag)
	eq(t, after.Code, http.StatusOK, "tenant-wide change invalidates")
	ne(t, after.Header().Get("ETag"), tag, "")

	tag = after.Header().Get("ETag")
	noErr(t, v.Bump(context.Background(), httputil.VersionKey("t", "t1", "u", "u1")))
	eq(t, r.get("t1", "u1", tag).Code, http.StatusOK, "the user's own change invalidates")
}

func TestConditionalGET_OtherTenantOrUserNeverMatches(t *testing.T) {
	r := newRig(newMem())
	tag := r.get("t1", "u1", "").Header().Get("ETag")

	for _, who := range [][2]string{{"t2", "u1"}, {"t1", "u2"}, {"t2", "u2"}} {
		rec := r.get(who[0], who[1], tag)
		if rec.Code != http.StatusOK {
			t.Fatalf("%v must not be answered with another scope's tag: got %d", who, rec.Code)
		}
		ne(t, rec.Header().Get("ETag"), tag, "")
	}
}

func TestConditionalGET_NoRedisMeansNoETag(t *testing.T) {
	r := newRig(nil)
	rec := r.get("t1", "u1", `"anything"`)
	eq(t, rec.Code, http.StatusOK, "")
	eq(t, rec.Header().Get("ETag"), "", "")
}

func TestConditionalGET_UnreachableCountersAnswerInFull(t *testing.T) {
	v := newMem()
	r := newRig(v)
	tag := r.get("t1", "u1", "").Header().Get("ETag")

	v.fail = true
	rec := r.get("t1", "u1", tag)
	eq(t, rec.Code, http.StatusOK, "never a 304 on a guess")
	eq(t, rec.Header().Get("ETag"), "", "")
}

func TestConditionalGET_OnlyASuccessfulAnswerCarriesATag(t *testing.T) {
	r := newRig(newMem())
	r.status = http.StatusForbidden
	rec := r.get("t1", "u1", "")
	eq(t, rec.Code, http.StatusForbidden, "")
	eq(t, rec.Header().Get("ETag"), "", "an error must never be revalidated into a 304")
}

func TestConditionalGET_TagExpiresEvenWithoutABump(t *testing.T) {
	r := newRig(newMem())
	tag := r.get("t1", "u1", "").Header().Get("ETag")

	r.clock = r.clock.Add(httputil.DefaultETagLifetime + time.Second)
	rec := r.get("t1", "u1", tag)
	eq(t, rec.Code, http.StatusOK, "a forgotten bump heals within the lifetime")
}

func TestConditionalGET_TagOfOnePathNeverAnswersForAnother(t *testing.T) {
	r := newRig(newMem())
	tag := r.get("t1", "u1", "").Header().Get("ETag")

	req := httptest.NewRequest(http.MethodGet, "/other", nil)
	req.Header.Set("X-Tenant", "t1")
	req.Header.Set("X-User", "u1")
	req.Header.Set("If-None-Match", tag)
	rec := httptest.NewRecorder()
	r.e.ServeHTTP(rec, req)
	eq(t, rec.Code, http.StatusOK, "another resource is answered, not waved through")
}

func TestConditionalGET_OnlyGETIsConditional(t *testing.T) {
	r := newRig(newMem())
	req := httptest.NewRequest(http.MethodPost, "/data", nil)
	req.Header.Set("If-None-Match", `"x"`)
	rec := httptest.NewRecorder()
	r.e.ServeHTTP(rec, req)
	eq(t, rec.Code, http.StatusNoContent, "")
	eq(t, rec.Header().Get("ETag"), "", "")
}
