package rest

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/pkg/httputil"
	pb "ngac-platform/proto/messaging"
	"ngac-platform/services/messaging/internal/domain"
	"ngac-platform/services/messaging/internal/store"
)

// liveRecorder records which live events a request caused.
type liveRecorder struct {
	Broadcaster
	roots   chan *pb.Message
	replies chan *pb.ThreadReplyEvent
}

func newLiveRecorder() *liveRecorder {
	return &liveRecorder{roots: make(chan *pb.Message, 4), replies: make(chan *pb.ThreadReplyEvent, 4)}
}

func (l *liveRecorder) BroadcastToChannel(_ string, m *pb.Message)               { l.roots <- m }
func (l *liveRecorder) BroadcastThreadReply(_ string, ev *pb.ThreadReplyEvent)   { l.replies <- ev }
func (l *liveRecorder) SendThreadStates(string, map[string]*pb.ThreadStateEvent) {}

func (c *chat) echoWithHub(hub Broadcaster) *echo.Echo {
	svc := domain.NewService(store.NewStore(c.pool), answerPolicy{allow: true}, nil, namedUser{}, nil)
	h := NewHandler(svc, nil, hub)
	e := httputil.NewEcho("messaging")
	as := func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(ctx echo.Context) error {
			httputil.SetClaims(ctx, &httputil.Claims{UserID: c.user, NGACNodeID: c.node, TenantID: c.ws})
			return next(ctx)
		}
	}
	e.POST("/api/channels/:chId/polls", h.CreatePoll, as)
	e.POST("/api/channels/:chId/tasks", h.CreateTask, as)
	return e
}

func (c *chat) cardRows(t *testing.T) (messages, polls, tasks int) {
	t.Helper()
	require.NoError(t, c.pool.QueryRow(context.Background(),
		`SELECT (SELECT count(*) FROM messages WHERE channel_id = $1),
		        (SELECT count(*) FROM polls WHERE channel_id = $1),
		        (SELECT count(*) FROM chat_tasks WHERE channel_id = $1)`, c.chID).Scan(&messages, &polls, &tasks))
	return
}

// A poll or task posted with parent_message_id is a reply in that thread, and
// goes out live as the thread's reply event, not as a main-stream message.
func TestCardRoutes_WithParentAreRepliesAnnouncedAsThreadReplies(t *testing.T) {
	c := newChat(t)
	ctx := context.Background()
	t.Cleanup(func() {
		c.pool.Exec(ctx, `DELETE FROM thread_follows WHERE root_message_id = $1`, c.msgID)
		c.pool.Exec(ctx, `DELETE FROM thread_participants WHERE message_id = $1`, c.msgID)
	})
	hub := newLiveRecorder()
	e := c.echoWithHub(hub)

	for name, tc := range map[string]struct{ path, body, kind string }{
		"poll": {"/api/channels/" + c.chID + "/polls", `{"question":"lunch?","options":["pho","bun"],"parent_message_id":"` + c.msgID + `"}`, "poll"},
		"task": {"/api/channels/" + c.chID + "/tasks", `{"title":"book a room","parent_message_id":"` + c.msgID + `"}`, "task"},
	} {
		rec := call(e, http.MethodPost, tc.path, tc.body)
		require.Equal(t, http.StatusCreated, rec.Code, "%s: %s", name, rec.Body)
		cardMsgID := decode(t, rec)["message_id"].(string)

		var parent, linked string
		require.NoError(t, c.pool.QueryRow(ctx,
			`SELECT parent_message_id, linked_entity_type FROM messages WHERE id = $1`, cardMsgID).Scan(&parent, &linked))
		assert.Equal(t, c.msgID, parent, name)
		assert.Equal(t, tc.kind, linked, name)

		select {
		case ev := <-hub.replies:
			assert.Equal(t, cardMsgID, ev.Message.Id, name)
			assert.Equal(t, c.msgID, ev.ParentMessageId, name)
			assert.Equal(t, tc.kind, ev.Message.LinkedEntityType, name)
		case <-time.After(5 * time.Second):
			t.Fatalf("%s: the thread reply event never went out", name)
		}
	}
	assert.Empty(t, hub.roots, "a card in a thread is not a main-stream message")
	var replies int
	require.NoError(t, c.pool.QueryRow(ctx, `SELECT reply_count FROM messages WHERE id = $1`, c.msgID).Scan(&replies))
	assert.Equal(t, 2, replies)
}

// Without parent_message_id a card is a main-stream message, announced to the
// channel exactly as a sent root message is, with no thread event.
func TestCardRoutes_WithoutParentAreAnnouncedAsRootMessages(t *testing.T) {
	c := newChat(t)
	for name, tc := range map[string]struct{ path, body, kind string }{
		"poll": {"/api/channels/" + c.chID + "/polls", `{"question":"q","options":["a","b"]}`, "poll"},
		"task": {"/api/channels/" + c.chID + "/tasks", `{"title":"t"}`, "task"},
	} {
		hub := newLiveRecorder()
		rec := call(c.echoWithHub(hub), http.MethodPost, tc.path, tc.body)
		require.Equal(t, http.StatusCreated, rec.Code, "%s: %s", name, rec.Body)
		cardMsgID := decode(t, rec)["message_id"].(string)
		var parent *string
		require.NoError(t, c.pool.QueryRow(context.Background(),
			`SELECT parent_message_id FROM messages WHERE id = $1`, cardMsgID).Scan(&parent))
		assert.Nil(t, parent, name)
		select {
		case m := <-hub.roots:
			assert.Equal(t, cardMsgID, m.Id, name)
			assert.Equal(t, tc.kind, m.LinkedEntityType, name)
			assert.Empty(t, m.ParentMessageId, name)
		case <-time.After(5 * time.Second):
			t.Fatalf("%s: the root message was never announced", name)
		}
		assert.Empty(t, hub.replies, name)
	}
}

func TestCardRoutes_WithParentDenyCases(t *testing.T) {
	c := newChat(t)
	other := newChat(t)
	ctx := context.Background()
	_, err := c.pool.Exec(ctx, `INSERT INTO messages (id, channel_id, sender_id, content, message_type, parent_message_id, created_at)
		VALUES ($1, $2, $3, 'a reply', 'user', $4, now())`, c.msgID+"-reply", c.chID, c.user, c.msgID)
	require.NoError(t, err)
	_, err = c.pool.Exec(ctx, `INSERT INTO messages (id, channel_id, sender_id, content, message_type, created_at, deleted_at)
		VALUES ($1, $2, $3, 'gone', 'user', now(), now())`, c.msgID+"-gone", c.chID, c.user)
	require.NoError(t, err)

	msgs0, polls0, tasks0 := c.cardRows(t)
	hub := newLiveRecorder()
	allowed := c.echoWithHub(hub)

	body := func(kind, parent string) (path, payload string) {
		if kind == "poll" {
			return "/api/channels/" + c.chID + "/polls", `{"question":"q","options":["a","b"],"parent_message_id":"` + parent + `"}`
		}
		return "/api/channels/" + c.chID + "/tasks", `{"title":"t","parent_message_id":"` + parent + `"}`
	}
	for name, parent := range map[string]string{
		"parent in another channel": other.msgID,
		"parent is itself a reply":  c.msgID + "-reply",
		"parent is deleted":         c.msgID + "-gone",
		"parent does not exist":     "no-such-message",
	} {
		for _, kind := range []string{"poll", "task"} {
			path, payload := body(kind, parent)
			rec := call(allowed, http.MethodPost, path, payload)
			assert.Equal(t, http.StatusBadRequest, rec.Code, "%s, %s: %s", kind, name, rec.Body)
		}
	}
	// The caller may not write the channel: refused before the parent is looked at.
	denied := c.echo(false)
	for _, kind := range []string{"poll", "task"} {
		path, payload := body(kind, c.msgID)
		rec := call(denied, http.MethodPost, path, payload)
		assert.Equal(t, http.StatusForbidden, rec.Code, "%s without access: %s", kind, rec.Body)
	}

	msgs1, polls1, tasks1 := c.cardRows(t)
	assert.Equal(t, [3]int{msgs0, polls0, tasks0}, [3]int{msgs1, polls1, tasks1}, "no message, poll or task written")
	assert.Empty(t, hub.replies)
	assert.Empty(t, hub.roots)
	var replies int
	require.NoError(t, c.pool.QueryRow(ctx, `SELECT reply_count FROM messages WHERE id = $1`, c.msgID).Scan(&replies))
	assert.Equal(t, 0, replies)
}
