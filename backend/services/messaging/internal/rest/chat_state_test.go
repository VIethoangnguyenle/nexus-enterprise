package rest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"ngac-platform/pkg/httputil"
	policypb "ngac-platform/proto/policy"
	"ngac-platform/services/messaging/internal/domain"
	"ngac-platform/services/messaging/internal/store"
)

// countingPolicy counts the questions put to the policy service: every read of
// the store is preceded by one, so a 304 that asks none read nothing.
type countingPolicy struct {
	answerPolicy
	asked *atomic.Int64
}

func (p countingPolicy) CheckAccess(ctx context.Context, in *policypb.CheckAccessRequest, o ...grpc.CallOption) (*policypb.AccessDecision, error) {
	p.asked.Add(1)
	return p.answerPolicy.CheckAccess(ctx, in, o...)
}

func (p countingPolicy) BatchCheckAccess(ctx context.Context, in *policypb.BatchCheckAccessRequest, o ...grpc.CallOption) (*policypb.BatchAccessResult, error) {
	p.asked.Add(1)
	return p.answerPolicy.BatchCheckAccess(ctx, in, o...)
}

type memVersions struct {
	mu   sync.Mutex
	vals map[string]int64
}

func (m *memVersions) Get(_ context.Context, keys ...string) ([]int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]int64, len(keys))
	for i, k := range keys {
		out[i] = m.vals[k]
	}
	return out, nil
}

func (m *memVersions) Bump(_ context.Context, keys ...string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.vals == nil {
		m.vals = map[string]int64{}
	}
	for _, k := range keys {
		m.vals[k]++
	}
	return nil
}

// stateRig is the production router over a counting policy.
type stateRig struct {
	c     *chat
	e     *echo.Echo
	svc   *domain.Service
	asked *atomic.Int64
}

func newStateRig(t *testing.T, c *chat, allow bool, v httputil.Versions) *stateRig {
	t.Helper()
	asked := &atomic.Int64{}
	policy := countingPolicy{answerPolicy: answerPolicy{allow: allow}, asked: asked}
	svc := domain.NewService(store.NewStore(c.pool), policy, nil, namedUser{}, nil)
	svc.SetVersions(v)
	h := NewHandler(svc, nil, nil)
	e := httputil.NewEcho("messaging")
	h.RegisterRoutes(e, "secret")
	return &stateRig{c: c, e: e, svc: svc, asked: asked}
}

func (r *stateRig) token(t *testing.T, user, tenant string) string {
	t.Helper()
	tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, &httputil.Claims{
		UserID: user, NGACNodeID: r.c.node, TenantID: tenant,
		RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))},
	}).SignedString([]byte("secret"))
	require.NoError(t, err)
	return tok
}

func (r *stateRig) do(t *testing.T, method, path, token, inm, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	if token != "" {
		req.Header.Set(echo.HeaderAuthorization, "Bearer "+token)
	}
	if inm != "" {
		req.Header.Set("If-None-Match", inm)
	}
	rec := httptest.NewRecorder()
	r.e.ServeHTTP(rec, req)
	return rec
}

func (r *stateRig) statePath() string { return "/api/workspaces/" + r.c.ws + "/chat-state" }

func (r *stateRig) get(t *testing.T, inm string) *httptest.ResponseRecorder {
	t.Helper()
	return r.do(t, http.MethodGet, r.statePath(), r.token(t, r.c.user, r.c.ws), inm, "")
}

func TestChatStateRoute_AllowAnswersEveryPart(t *testing.T) {
	c := newChat(t) // one message in one channel
	r := newStateRig(t, c, true, &memVersions{})

	rec := r.get(t, "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.NotEmpty(t, rec.Header().Get("ETag"))
	body := decode(t, rec)
	for _, part := range []string{"channels", "dms", "unread", "activity", "sections", "mention_unread_count", "unread_thread_count", "unread_threads"} {
		assert.Contains(t, body, part)
	}
	channels, _ := body["channels"].([]any)
	var ids []string
	for _, ch := range channels {
		ids = append(ids, ch.(map[string]any)["id"].(string))
	}
	assert.Contains(t, ids, c.chID)
	assert.IsType(t, []any{}, body["dms"], "a list is an array, never null")
	activity, _ := body["activity"].([]any)
	require.NotEmpty(t, activity)
	assert.Equal(t, c.chID, activity[0].(map[string]any)["channel_id"])
}

func TestChatStateRoute_DenyListsNoConversation(t *testing.T) {
	c := newChat(t)
	r := newStateRig(t, c, false, &memVersions{})

	rec := r.get(t, "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	body := decode(t, rec)
	for _, part := range []string{"channels", "dms", "unread", "activity", "sections", "unread_threads"} {
		assert.Empty(t, body[part], part+" must hold nothing the caller cannot read")
	}
}

func TestChatStateRoute_NoSessionIs401AndAnotherWorkspaceIs403(t *testing.T) {
	c := newChat(t)
	r := newStateRig(t, c, true, &memVersions{})

	assert.Equal(t, http.StatusUnauthorized, r.do(t, http.MethodGet, r.statePath(), "", "", "").Code)

	own := r.get(t, "")
	tag := own.Header().Get("ETag")
	foreign := r.do(t, http.MethodGet, "/api/workspaces/someone-elses/chat-state", r.token(t, c.user, c.ws), tag, "")
	assert.Equal(t, http.StatusForbidden, foreign.Code, "a matching tag must not wave through a workspace that is not the caller's")
	assert.Empty(t, foreign.Header().Get("ETag"))
}

func TestChatStateRoute_UnchangedIs304AndReadsNothing(t *testing.T) {
	c := newChat(t)
	r := newStateRig(t, c, true, &memVersions{})

	first := r.get(t, "")
	tag := first.Header().Get("ETag")
	require.NotEmpty(t, tag)
	askedAfterFirst := r.asked.Load()
	require.Positive(t, askedAfterFirst, "building the state asks the policy service")

	again := r.get(t, tag)
	assert.Equal(t, http.StatusNotModified, again.Code)
	assert.Empty(t, again.Body.String())
	assert.Equal(t, askedAfterFirst, r.asked.Load(), "a 304 reaches neither the policy service nor the store")
}

func TestChatStateRoute_TagChangesAfterTheMatchingWrite(t *testing.T) {
	c := newChat(t)
	r := newStateRig(t, c, true, &memVersions{})
	tag := r.get(t, "").Header().Get("ETag")

	// The caller's own read mark: a write path of their own state.
	mark := r.do(t, http.MethodPost, "/api/channels/"+c.chID+"/read", r.token(t, c.user, c.ws), "",
		`{"last_message_id":"`+c.msgID+`"}`)
	require.Equal(t, http.StatusOK, mark.Code, mark.Body.String())
	after := r.get(t, tag)
	assert.Equal(t, http.StatusOK, after.Code, "after a write the old tag no longer matches")
	assert.NotEqual(t, tag, after.Header().Get("ETag"))

	// A change announced by the domain moves the tenant's counter.
	tag = after.Header().Get("ETag")
	r.svc.BumpChat(c.ws)
	assert.Equal(t, http.StatusOK, r.get(t, tag).Code)
}

func TestChatStateRoute_RefusedWriteKeepsTheTag(t *testing.T) {
	c := newChat(t)
	r := newStateRig(t, c, false, &memVersions{})
	tag := r.get(t, "").Header().Get("ETag")

	mark := r.do(t, http.MethodPost, "/api/channels/"+c.chID+"/read", r.token(t, c.user, c.ws), "",
		`{"last_message_id":"`+c.msgID+`"}`)
	require.Equal(t, http.StatusForbidden, mark.Code)
	assert.Equal(t, http.StatusNotModified, r.get(t, tag).Code, "a refused write changed nothing")
}

func TestChatStateRoute_OtherUserOrTenantNeverMatches(t *testing.T) {
	c := newChat(t)
	r := newStateRig(t, c, true, &memVersions{})
	tag := r.get(t, "").Header().Get("ETag")
	require.NotEmpty(t, tag)

	// Another person of the same workspace, holding the first person's tag.
	other := r.do(t, http.MethodGet, r.statePath(), r.token(t, "another-user", c.ws), tag, "")
	assert.Equal(t, http.StatusOK, other.Code)
	assert.NotEqual(t, tag, other.Header().Get("ETag"))

	// The same person in another tenant, on that tenant's own path.
	elsewhere := r.do(t, http.MethodGet, "/api/workspaces/other-tenant/chat-state", r.token(t, c.user, "other-tenant"), tag, "")
	assert.Equal(t, http.StatusOK, elsewhere.Code)
	assert.NotEqual(t, tag, elsewhere.Header().Get("ETag"))
}

func TestChatStateRoute_WithoutRedisAlwaysAnswersInFull(t *testing.T) {
	c := newChat(t)
	r := newStateRig(t, c, true, nil)

	rec := r.get(t, `"anything"`)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Empty(t, rec.Header().Get("ETag"))
}
