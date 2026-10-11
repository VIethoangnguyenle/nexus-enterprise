package rest

import (
	"context"
	"net/http"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestThreadRoutes_AnswerByAccess(t *testing.T) {
	c := newChat(t)
	ctx := context.Background()
	_, err := c.pool.Exec(ctx, `INSERT INTO messages (id, channel_id, sender_id, content, message_type, parent_message_id, created_at)
		VALUES ($1, $2, $3, 'a reply', 'user', $4, now())`, c.msgID+"-reply", c.chID, c.user, c.msgID)
	require.NoError(t, err)
	_, err = c.pool.Exec(ctx, `UPDATE messages SET reply_count = 1, last_reply_at = now() WHERE id = $1`, c.msgID)
	require.NoError(t, err)
	t.Cleanup(func() {
		c.pool.Exec(ctx, `DELETE FROM thread_follows WHERE root_message_id = $1`, c.msgID)
		c.pool.Exec(ctx, `DELETE FROM messages WHERE parent_message_id = $1`, c.msgID)
	})
	follow := "/api/messages/" + c.msgID + "/follow"
	read := "/api/messages/" + c.msgID + "/thread/read"
	threads := "/api/channels/" + c.chID + "/threads"

	denied := c.echo(false)
	for _, r := range [][2]string{
		{http.MethodPost, follow}, {http.MethodDelete, follow}, {http.MethodPost, read}, {http.MethodGet, threads},
	} {
		assert.Equal(t, http.StatusForbidden, call(denied, r[0], r[1], "").Code, "%s %s without read", r[0], r[1])
	}

	allowed := c.echo(true)
	assert.Equal(t, http.StatusOK, call(allowed, http.MethodPost, follow, "").Code)
	assert.Equal(t, http.StatusOK, call(allowed, http.MethodPost, read, "").Code)
	assert.Equal(t, http.StatusOK, call(allowed, http.MethodDelete, follow, "").Code)
	rec := call(allowed, http.MethodGet, threads+"?filter=all", "")
	require.Equal(t, http.StatusOK, rec.Code)
	msgs, _ := decode(t, rec)["messages"].([]any)
	require.Len(t, msgs, 1)
	assert.Equal(t, c.msgID, msgs[0].(map[string]any)["id"])

	assert.Equal(t, http.StatusBadRequest, call(allowed, http.MethodGet, threads+"?filter=everything", "").Code)
	assert.Equal(t, http.StatusBadRequest, call(allowed, http.MethodGet, threads+"?limit=many", "").Code)
	assert.Equal(t, http.StatusNotFound, call(allowed, http.MethodPost, "/api/messages/no-such-message/follow", "").Code)
}

// The production router serves the thread routes, not only the test router.
func TestRegisterRoutes_MountsTheThreadRoutes(t *testing.T) {
	e := echo.New()
	NewHandler(nil, nil, nil).RegisterRoutes(e, "secret")
	mounted := map[string]bool{}
	for _, r := range e.Routes() {
		mounted[r.Method+" "+r.Path] = true
	}
	for _, want := range []string{
		"POST /api/messages/:msgId/follow",
		"DELETE /api/messages/:msgId/follow",
		"POST /api/messages/:msgId/thread/read",
		"GET /api/channels/:chId/threads",
		"PATCH /api/messages/:msgId",
		"DELETE /api/messages/:msgId",
		"POST /api/messages/:msgId/unread",
		"GET /api/workspaces/:id/chat-sections",
		"POST /api/workspaces/:id/chat-sections",
		"PATCH /api/workspaces/:id/chat-sections/:sectionId",
		"DELETE /api/workspaces/:id/chat-sections/:sectionId",
		"PUT /api/workspaces/:id/chat-sections/:sectionId/channels/:chId",
		"DELETE /api/workspaces/:id/chat-sections/channels/:chId",
		"GET /api/me/push-preferences",
		"PUT /api/me/push-preferences",
	} {
		assert.True(t, mounted[want], want)
	}
}
