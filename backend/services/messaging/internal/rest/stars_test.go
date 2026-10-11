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
	"ngac-platform/testutil"
)

func (c *chat) starCleanup(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		c.pool.Exec(context.Background(), `DELETE FROM starred_conversations WHERE channel_id = $1`, c.chID)
	})
}

func TestStarRoutes_DeniedWithoutRead(t *testing.T) {
	c := newChat(t)
	c.starCleanup(t)
	e := c.echo(false)
	for _, r := range [][2]string{
		{http.MethodPut, "/api/channels/" + c.chID + "/star"},
		{http.MethodDelete, "/api/channels/" + c.chID + "/star"},
		{http.MethodPut, "/api/messages/" + c.msgID + "/star"},
		{http.MethodDelete, "/api/messages/" + c.msgID + "/star"},
	} {
		assert.Equal(t, http.StatusForbidden, call(e, r[0], r[1], "").Code, "%s %s without read", r[0], r[1])
	}
	var n int
	require.NoError(t, c.pool.QueryRow(context.Background(), `SELECT count(*) FROM starred_messages WHERE message_id = $1`, c.msgID).Scan(&n))
	assert.Zero(t, n)

	// The feeds answer with what the caller reads: nothing.
	for _, p := range []string{"/api/messages/starred", "/api/messages/mentions"} {
		rec := call(e, http.MethodGet, p, "")
		require.Equal(t, http.StatusOK, rec.Code, p)
		assert.Empty(t, decode(t, rec)["messages"], p)
	}
}

func TestStarRoutes_StarUnstarAndFeed(t *testing.T) {
	c := newChat(t)
	c.starCleanup(t)
	e := c.echo(true)
	star := "/api/messages/" + c.msgID + "/star"
	conv := "/api/channels/" + c.chID + "/star"

	for i := 0; i < 2; i++ {
		assert.Equal(t, http.StatusNoContent, call(e, http.MethodPut, star, "").Code, "idempotent")
		assert.Equal(t, http.StatusNoContent, call(e, http.MethodPut, conv, "").Code, "idempotent")
	}
	rec := call(e, http.MethodGet, "/api/messages/starred?limit=5", "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	msgs, _ := decode(t, rec)["messages"].([]any)
	require.Len(t, msgs, 1)
	m := msgs[0].(map[string]any)
	assert.Equal(t, c.msgID, m["id"])
	assert.Equal(t, true, m["starred"])
	assert.NotEmpty(t, m["starred_at"])

	rec = call(e, http.MethodGet, "/api/channels/"+c.chID, "")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, true, decode(t, rec)["starred"])
	rec = call(e, http.MethodGet, "/api/channels/unread", "")
	require.Equal(t, http.StatusOK, rec.Code)
	var starred bool
	for _, it := range decode(t, rec)["channels"].([]any) {
		if it.(map[string]any)["channel_id"] == c.chID {
			starred, _ = it.(map[string]any)["starred"].(bool)
		}
	}
	assert.True(t, starred)

	assert.Equal(t, http.StatusNoContent, call(e, http.MethodDelete, star, "").Code)
	assert.Equal(t, http.StatusNoContent, call(e, http.MethodDelete, star, "").Code, "idempotent")
	assert.Equal(t, http.StatusNoContent, call(e, http.MethodDelete, conv, "").Code)
	rec = call(e, http.MethodGet, "/api/channels/"+c.chID, "")
	assert.Nil(t, decode(t, rec)["starred"])
	assert.Empty(t, decode(t, call(e, http.MethodGet, "/api/messages/starred", ""))["messages"])

	// Bad requests.
	assert.Equal(t, http.StatusNotFound, call(e, http.MethodPut, "/api/messages/no-such/star", "").Code)
	assert.Equal(t, http.StatusNotFound, call(e, http.MethodPut, "/api/channels/no-such/star", "").Code)
	assert.Equal(t, http.StatusBadRequest, call(e, http.MethodGet, "/api/messages/starred?limit=many", "").Code)
	assert.Equal(t, http.StatusBadRequest, call(e, http.MethodGet, "/api/messages/starred?before=yesterday", "").Code)
	assert.Equal(t, http.StatusBadRequest, call(e, http.MethodGet, "/api/messages/mentions?before=yesterday", "").Code)

	// A deleted message cannot be starred.
	_, err := c.pool.Exec(context.Background(), `UPDATE messages SET deleted_at = now() WHERE id = $1`, c.msgID)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, call(e, http.MethodPut, star, "").Code)
}

func TestMentionsRoute_FlagsUnread(t *testing.T) {
	c := newChat(t)
	ctx := context.Background()
	other, _ := testutil.CreateUser(t, c.pool)
	t.Cleanup(func() { c.pool.Exec(ctx, `DELETE FROM messages WHERE channel_id = $1`, c.chID) })
	_, err := c.pool.Exec(ctx, `INSERT INTO messages (id, channel_id, sender_id, content, message_type, mentions, created_at)
		VALUES ($1, $2, $3, 'hey', 'user', ARRAY[$4]::text[], now())`, c.msgID+"-m", c.chID, other, c.user)
	require.NoError(t, err)

	rec := call(c.echo(true), http.MethodGet, "/api/messages/mentions", "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	msgs, _ := decode(t, rec)["messages"].([]any)
	require.Len(t, msgs, 1)
	assert.Equal(t, c.msgID+"-m", msgs[0].(map[string]any)["id"])
	assert.Equal(t, true, msgs[0].(map[string]any)["unread"])

	rec = call(c.echo(true), http.MethodGet, "/api/messages/mentions/unread-count", "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.EqualValues(t, 1, decode(t, rec)["count"])
	rec = call(c.echo(false), http.MethodGet, "/api/messages/mentions/unread-count", "")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.EqualValues(t, 0, decode(t, rec)["count"], "without read nothing counts")
}

// A person an edit adds to an old message has the unread mention on all three
// in-app signals; without read on the conversation they get none.
func TestMentionSignals_ForAPersonAddedByAnEdit(t *testing.T) {
	c := newChat(t)
	ctx := context.Background()
	other, _ := testutil.CreateUser(t, c.pool)
	t.Cleanup(func() { c.pool.Exec(ctx, `DELETE FROM messages WHERE channel_id = $1`, c.chID) })
	id := c.msgID + "-e"
	_, err := c.pool.Exec(ctx, `INSERT INTO messages (id, channel_id, sender_id, content, message_type, created_at)
		VALUES ($1, $2, $3, 'hey', 'user', now() - interval '1 hour')`, id, c.chID, other)
	require.NoError(t, err)
	_, err = c.pool.Exec(ctx, `INSERT INTO read_receipts (user_id, channel_id, last_read_at) VALUES ($1, $2, now() - interval '1 minute')`, c.user, c.chID)
	require.NoError(t, err)

	e := c.echo(true)
	count := func(e *echo.Echo) any {
		return decode(t, call(e, http.MethodGet, "/api/messages/mentions/unread-count", ""))["count"]
	}
	assert.EqualValues(t, 0, count(e))

	_, err = store.NewStore(c.pool).UpdateMessageContent(ctx, id, "edited", "edited", "markdown", []string{c.user})
	require.NoError(t, err)

	assert.EqualValues(t, 1, count(e))
	feed, _ := decode(t, call(e, http.MethodGet, "/api/messages/mentions", ""))["messages"].([]any)
	require.Len(t, feed, 1)
	assert.Equal(t, true, feed[0].(map[string]any)["unread"])
	rec := call(e, http.MethodGet, "/api/channels/unread", "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"mention_count":1`)

	// Deny: without read there is no count and no feed item.
	denied := c.echo(false)
	assert.EqualValues(t, 0, count(denied))
	none, _ := decode(t, call(denied, http.MethodGet, "/api/messages/mentions", ""))["messages"].([]any)
	assert.Empty(t, none)

	// Reading past the edit clears it.
	time.Sleep(15 * time.Millisecond)
	require.Equal(t, http.StatusOK, call(e, http.MethodPost, "/api/channels/"+c.chID+"/read", `{}`).Code)
	assert.EqualValues(t, 0, count(e))
}

// The production router: no session is 401, and the two feeds are reached
// instead of being read as a message id.
func TestStarRoutes_ProductionRouter(t *testing.T) {
	c := newChat(t)
	c.starCleanup(t)
	h := NewHandler(domain.NewService(store.NewStore(c.pool), answerPolicy{allow: true}, nil, namedUser{}, nil), nil, nil)
	e := httputil.NewEcho("messaging")
	h.RegisterRoutes(e, "secret")

	tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, &httputil.Claims{
		UserID: c.user, NGACNodeID: c.node, TenantID: c.ws,
		RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))},
	}).SignedString([]byte("secret"))
	require.NoError(t, err)
	send := func(method, path string, authed bool) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(""))
		if authed {
			req.Header.Set(echo.HeaderAuthorization, "Bearer "+tok)
		}
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		return rec
	}

	routes := [][2]string{
		{http.MethodPut, "/api/channels/" + c.chID + "/star"},
		{http.MethodDelete, "/api/channels/" + c.chID + "/star"},
		{http.MethodPut, "/api/messages/" + c.msgID + "/star"},
		{http.MethodDelete, "/api/messages/" + c.msgID + "/star"},
		{http.MethodGet, "/api/messages/starred"},
		{http.MethodGet, "/api/messages/mentions"},
		{http.MethodGet, "/api/messages/mentions/unread-count"},
	}
	for _, r := range routes {
		assert.Equal(t, http.StatusUnauthorized, send(r[0], r[1], false).Code, "%s %s without a session", r[0], r[1])
	}
	assert.Equal(t, http.StatusNoContent, send(http.MethodPut, routes[2][1], true).Code)
	for _, p := range []string{"/api/messages/starred", "/api/messages/mentions"} {
		rec := send(http.MethodGet, p, true)
		require.Equal(t, http.StatusOK, rec.Code, p)
		assert.Contains(t, rec.Header().Get(echo.HeaderContentType), "json")
	}
	rec := send(http.MethodGet, "/api/messages/mentions/unread-count", true)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.EqualValues(t, 0, decode(t, rec)["count"], "the count, not a lookup of a message called \"mentions\"")
	rec = send(http.MethodGet, "/api/messages/starred", true)
	assert.Contains(t, rec.Body.String(), c.msgID, "the feed, not a lookup of a message called \"starred\"")
}
