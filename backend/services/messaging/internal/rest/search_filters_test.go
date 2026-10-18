package rest

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSearchAllRoute_FromFilterKeepsOnlySendersMessages(t *testing.T) {
	c := newChat(t) // one message "findable text" by c.user
	get := c.activityRouter(t, true)
	rec := get("/api/messages/search?q=findable&from="+c.user, true)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	msgs, _ := decode(t, rec)["messages"].([]any)
	require.Len(t, msgs, 1, "from filter keeps only the sender's messages")

	rec = get("/api/messages/search?q=findable&from=nobody", true)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Empty(t, decode(t, rec)["messages"], "wrong sender yields nothing")
}

func TestSearchAllRoute_ChannelIDFilterRestrictsToOneChannel(t *testing.T) {
	c := newChat(t)
	get := c.activityRouter(t, true)

	rec := get("/api/messages/search?q=findable&channel_id="+c.chID, true)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	msgs, _ := decode(t, rec)["messages"].([]any)
	require.Len(t, msgs, 1)

	// An unreadable/non-existent channel returns empty, never an error.
	rec = get("/api/messages/search?q=findable&channel_id=non-existent", true)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Empty(t, decode(t, rec)["messages"])
}

func TestSearchAllRoute_InvalidDateReturns400(t *testing.T) {
	c := newChat(t)
	get := c.activityRouter(t, true)
	assert.Equal(t, http.StatusBadRequest, get("/api/messages/search?q=findable&after=not-a-date", true).Code)
	assert.Equal(t, http.StatusBadRequest, get("/api/messages/search?q=findable&before_date=2025-13-99", true).Code)
}

func TestSearchAllRoute_HasLinkFilter(t *testing.T) {
	c := newChat(t)
	ctx := context.Background()
	_, err := c.pool.Exec(ctx, `INSERT INTO messages (id, channel_id, sender_id, content, content_format, message_type, created_at)
		VALUES ($1, $2, $3, 'findable text https://example.com', 'plain', 'user', now() + interval '1 second')`,
		c.msgID+"-link", c.chID, c.user)
	require.NoError(t, err)
	t.Cleanup(func() { c.pool.Exec(ctx, `DELETE FROM messages WHERE id = $1`, c.msgID+"-link") })

	get := c.activityRouter(t, true)
	rec := get("/api/messages/search?q=findable&has_link=true", true)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	msgs, _ := decode(t, rec)["messages"].([]any)
	require.Len(t, msgs, 1)
	assert.Equal(t, c.msgID+"-link", msgs[0].(map[string]any)["id"])
}

func TestSearchAllRoute_MentionsMeFilter(t *testing.T) {
	c := newChat(t)
	ctx := context.Background()
	_, err := c.pool.Exec(ctx, `INSERT INTO messages (id, channel_id, sender_id, content, content_format, message_type, mentions, created_at)
		VALUES ($1, $2, $3, 'findable mention', 'plain', 'user', $4, now() + interval '2 seconds')`,
		c.msgID+"-mention", c.chID, c.user, []string{c.user})
	require.NoError(t, err)
	t.Cleanup(func() { c.pool.Exec(ctx, `DELETE FROM messages WHERE id = $1`, c.msgID+"-mention") })

	get := c.activityRouter(t, true)
	rec := get("/api/messages/search?q=findable&mentions_me=true", true)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	msgs, _ := decode(t, rec)["messages"].([]any)
	require.Len(t, msgs, 1)
	assert.Equal(t, c.msgID+"-mention", msgs[0].(map[string]any)["id"])
}

func TestSearchAllRoute_FilterOnlySearchNoText(t *testing.T) {
	c := newChat(t)
	get := c.activityRouter(t, true)

	// No text, from=user → should return the message.
	rec := get("/api/messages/search?from="+c.user, true)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	msgs, _ := decode(t, rec)["messages"].([]any)
	require.Len(t, msgs, 1, "filter-only search works without q")

	// No text, no filter → empty.
	rec = get("/api/messages/search", true)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Empty(t, decode(t, rec)["messages"])
}

func TestSearchAllRoute_TimeZone(t *testing.T) {
	c := newChat(t)
	get := c.activityRouter(t, true)
	// A known zone is accepted, and so is none (Asia/Ho_Chi_Minh).
	for _, q := range []string{"&tz=UTC", "&tz=Asia/Ho_Chi_Minh", ""} {
		rec := get("/api/messages/search?q=findable&after=2000-01-01"+q, true)
		require.Equal(t, http.StatusOK, rec.Code, q+rec.Body.String())
		msgs, _ := decode(t, rec)["messages"].([]any)
		require.Len(t, msgs, 1, q)
	}
	// An unknown one is a 400, not a silent fallback.
	assert.Equal(t, http.StatusBadRequest, get("/api/messages/search?q=findable&after=2000-01-01&tz=Mars/Olympus", true).Code)
}

func TestSearchAllRoute_NoHitsIsAnEmptyListNotAnEmptyObject(t *testing.T) {
	c := newChat(t)
	get := c.activityRouter(t, true)
	for _, q := range []string{"q=zzzznothing", "q=n", "", "from=nobody"} {
		rec := get("/api/messages/search?"+q, true)
		require.Equal(t, http.StatusOK, rec.Code, q)
		body := decode(t, rec)
		list, ok := body["messages"].([]any)
		require.True(t, ok, "%q: messages must be a list, got %s", q, rec.Body.String())
		assert.Empty(t, list)
		assert.Equal(t, false, body["has_more"], q)
	}
}

func TestSearchAllRoute_MalformedIDsAre400WellFormedUnknownOnesAreEmpty(t *testing.T) {
	c := newChat(t)
	get := c.activityRouter(t, true)
	for _, bad := range []string{"from=%20", "from=a%20b", "from=%27%3B--", "channel_id=a/b", "channel_id=%20",
		"from=" + strings.Repeat("x", 200)} {
		assert.Equal(t, http.StatusBadRequest, get("/api/messages/search?q=findable&"+bad, true).Code, bad)
	}
	for _, unknown := range []string{"from=nobody", "from=11111111-aaaa-4bbb-8ccc-000000000009", "channel_id=11111111-aaaa-4bbb-8ccc-000000000009"} {
		rec := get("/api/messages/search?q=findable&"+unknown, true)
		require.Equal(t, http.StatusOK, rec.Code, unknown)
		assert.Empty(t, decode(t, rec)["messages"], unknown)
	}
}
