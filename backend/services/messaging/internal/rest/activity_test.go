package rest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/pkg/httputil"
	"ngac-platform/services/messaging/internal/domain"
	"ngac-platform/services/messaging/internal/store"
)

// activityRouter is the production router over a policy that allows or refuses
// every read, and a request helper that signs in as the chat's caller or not.
func (c *chat) activityRouter(t *testing.T, allow bool) func(path string, authed bool) *httptest.ResponseRecorder {
	t.Helper()
	h := NewHandler(domain.NewService(store.NewStore(c.pool), answerPolicy{allow: allow}, nil, namedUser{}, nil), nil, nil)
	e := httputil.NewEcho("messaging")
	h.RegisterRoutes(e, "secret")
	tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, &httputil.Claims{
		UserID: c.user, NGACNodeID: c.node, TenantID: c.ws,
		RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))},
	}).SignedString([]byte("secret"))
	require.NoError(t, err)
	return func(path string, authed bool) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, path, strings.NewReader(""))
		if authed {
			req.Header.Set(echo.HeaderAuthorization, "Bearer "+tok)
		}
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		return rec
	}
}

func TestActivityRoutes_NoSessionIs401(t *testing.T) {
	c := newChat(t)
	get := c.activityRouter(t, true)
	for _, p := range []string{"/api/channels/activity", "/api/threads/followed"} {
		assert.Equal(t, http.StatusUnauthorized, get(p, false).Code, p)
	}
}

func TestConversationActivityRoute_AnswersTheLatestMessagePerConversation(t *testing.T) {
	c := newChat(t) // one message, "findable text"
	rec := c.activityRouter(t, true)("/api/channels/activity", true)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Header().Get(echo.HeaderContentType), "json")
	channels, _ := decode(t, rec)["channels"].([]any)
	require.Len(t, channels, 1)
	item := channels[0].(map[string]any)
	assert.Equal(t, c.chID, item["channel_id"], "the route, not a channel called \"activity\"")
	latest := item["latest"].(map[string]any)
	assert.Equal(t, c.msgID, latest["id"])
	assert.Equal(t, "findable text", latest["preview"])
	assert.Equal(t, "text", latest["kind"])
	assert.Equal(t, c.user, latest["sender_id"])
	assert.NotEmpty(t, latest["created_at"])
	assert.NotContains(t, item, "latest_reply", "no reply, no key")
}

func TestConversationActivityRoute_NamesNothingWithoutRead(t *testing.T) {
	c := newChat(t)
	rec := c.activityRouter(t, false)("/api/channels/activity", true)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Empty(t, decode(t, rec)["channels"])
}

func TestFollowedThreadsRoute_AnswersTheCallersThreads(t *testing.T) {
	c := newChat(t)
	ctx := context.Background()
	_, err := c.pool.Exec(ctx, `INSERT INTO messages (id, channel_id, sender_id, content, message_type, parent_message_id, created_at)
		VALUES ($1, $2, $3, 'a reply', 'user', $4, now())`, c.msgID+"-reply", c.chID, c.user, c.msgID)
	require.NoError(t, err)
	_, err = c.pool.Exec(ctx, `UPDATE messages SET reply_count = 1, last_reply_at = now() WHERE id = $1`, c.msgID)
	require.NoError(t, err)
	t.Cleanup(func() { c.pool.Exec(ctx, `DELETE FROM messages WHERE parent_message_id = $1`, c.msgID) })

	get := c.activityRouter(t, true)
	rec := get("/api/threads/followed?limit=5", true)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	msgs, _ := decode(t, rec)["messages"].([]any)
	require.Len(t, msgs, 1)
	assert.Equal(t, c.msgID, msgs[0].(map[string]any)["id"])

	assert.Equal(t, http.StatusBadRequest, get("/api/threads/followed?limit=many", true).Code)
	assert.Equal(t, http.StatusBadRequest, get("/api/threads/followed?before=yesterday", true).Code)

	denied := c.activityRouter(t, false)("/api/threads/followed", true)
	require.Equal(t, http.StatusOK, denied.Code)
	assert.Empty(t, decode(t, denied)["messages"], "no read, no thread")
}
