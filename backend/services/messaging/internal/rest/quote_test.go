package rest

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A refused quote answers with a reason the client can branch on; every other
// refusal of a send stays a plain message, so the client keeps a quote that is
// still good.
func TestSendMessage_ARefusedQuoteSaysSo(t *testing.T) {
	c := newChat(t)
	ctx := context.Background()
	send := "/api/channels/" + c.chID + "/messages"
	body := func(quoted string) string {
		return fmt.Sprintf(`{"content":"đồng ý","content_format":"plain","quoted_message_id":%q}`, quoted)
	}

	// Another channel's message, and a deleted one of this channel.
	otherCh := fmt.Sprintf("rest-other-%d", time.Now().UnixNano())
	_, err := c.pool.Exec(ctx, `INSERT INTO channels (id, name, channel_type, workspace_id, created_at) VALUES ($1, 'other', 'workspace', $2, now())`, otherCh, c.ws)
	require.NoError(t, err)
	foreign := otherCh + "-msg"
	_, err = c.pool.Exec(ctx, `INSERT INTO messages (id, channel_id, sender_id, content, content_format, message_type, created_at) VALUES ($1, $2, $3, 'bí mật', 'plain', 'user', now())`, foreign, otherCh, c.user)
	require.NoError(t, err)
	gone := c.chID + "-gone"
	_, err = c.pool.Exec(ctx, `INSERT INTO messages (id, channel_id, sender_id, content, content_format, message_type, deleted_at, created_at) VALUES ($1, $2, $3, 'đã xoá', 'plain', 'user', now(), now())`, gone, c.chID, c.user)
	require.NoError(t, err)
	t.Cleanup(func() {
		c.pool.Exec(ctx, `DELETE FROM messages WHERE channel_id = $1`, otherCh)
		c.pool.Exec(ctx, `DELETE FROM channels WHERE id = $1`, otherCh)
	})

	allowed := c.echo(true)
	for name, quoted := range map[string]string{"deleted": gone, "unknown": "no-such-message", "other channel": foreign} {
		rec := call(allowed, http.MethodPost, send, body(quoted))
		require.Equal(t, http.StatusBadRequest, rec.Code, name)
		got := decode(t, rec)
		assert.Equal(t, "quoted_message_unavailable", got["reason"], name)
		assert.NotEmpty(t, got["message"], name)
		assert.NotContains(t, rec.Body.String(), "bí mật", name+": the refusal must not leak the text")
	}

	// A quote of a live message of this channel goes through.
	assert.Equal(t, http.StatusCreated, call(allowed, http.MethodPost, send, body(c.msgID)).Code)

	// Other 400s carry no reason: the quote is not the problem.
	rec := call(allowed, http.MethodPost, send, `{"content":"","content_format":"plain","quoted_message_id":"`+c.msgID+`"}`)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.NotContains(t, decode(t, rec), "reason")

	// No write access: refused as forbidden before the quote is looked at, so
	// the answer does not tell whether the message exists.
	rec = call(c.echo(false), http.MethodPost, send, body(gone))
	require.Equal(t, http.StatusForbidden, rec.Code)
	assert.NotContains(t, decode(t, rec), "reason")
}
