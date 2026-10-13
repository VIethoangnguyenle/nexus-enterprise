package rest

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSearchAllRoute_NoSessionIs401(t *testing.T) {
	c := newChat(t)
	assert.Equal(t, http.StatusUnauthorized, c.activityRouter(t, true)("/api/messages/search?q=findable", false).Code)
}

func TestSearchAllRoute_AnswersMatchingMessages(t *testing.T) {
	c := newChat(t) // one message, "findable text"
	ctx := context.Background()
	_, err := c.pool.Exec(ctx, `INSERT INTO messages (id, channel_id, sender_id, content, message_type, parent_message_id, created_at)
		VALUES ($1, $2, $3, 'a findable reply', 'user', $4, now() + interval '1 second')`, c.msgID+"-reply", c.chID, c.user, c.msgID)
	require.NoError(t, err)
	t.Cleanup(func() { c.pool.Exec(ctx, `DELETE FROM messages WHERE parent_message_id = $1`, c.msgID) })

	get := c.activityRouter(t, true)
	rec := get("/api/messages/search?q=findabl&limit=10", true)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	body := decode(t, rec)
	msgs, _ := body["messages"].([]any)
	require.Len(t, msgs, 2, "the route, not a message called \"search\"")
	reply := msgs[0].(map[string]any)
	assert.Equal(t, c.msgID+"-reply", reply["id"])
	assert.Equal(t, c.msgID, reply["parent_message_id"])
	assert.Equal(t, c.msgID, msgs[1].(map[string]any)["id"])

	paged := decode(t, get("/api/messages/search?q=findable&limit=1", true))
	assert.Equal(t, true, paged["has_more"])

	none := decode(t, get("/api/messages/search?q=findable&has_file=true", true))
	assert.Empty(t, none["messages"], "neither message has a file")
}

func TestSearchAllRoute_ShortOrMissingQueryIsAnEmptyList(t *testing.T) {
	c := newChat(t)
	get := c.activityRouter(t, true)
	for _, p := range []string{"/api/messages/search", "/api/messages/search?q=", "/api/messages/search?q=f", "/api/messages/search?q=%20f%20"} {
		rec := get(p, true)
		require.Equal(t, http.StatusOK, rec.Code, p)
		assert.Empty(t, decode(t, rec)["messages"], p)
	}
}

func TestSearchAllRoute_BadInputAndNoReadAccess(t *testing.T) {
	c := newChat(t)
	get := c.activityRouter(t, true)
	assert.Equal(t, http.StatusBadRequest, get("/api/messages/search?q=findable&limit=many", true).Code)
	assert.Equal(t, http.StatusBadRequest, get("/api/messages/search?q=findable&before=yesterday", true).Code)

	denied := c.activityRouter(t, false)("/api/messages/search?q=findable", true)
	require.Equal(t, http.StatusOK, denied.Code)
	assert.Empty(t, decode(t, denied)["messages"], "no read, no hit")
}
