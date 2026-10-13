package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/services/messaging/internal/store"
)

// newCall stores a call of the world's channel with its card and returns it.
func newCall(t *testing.T, w *threadWorld, status string, ringFor time.Duration) (*store.Call, error) {
	t.Helper()
	now := time.Now()
	c := &store.Call{
		ID: uuid.NewString(), ChannelID: w.chID, Kind: "video", Status: status,
		StartedBy: w.users[0], MessageID: uuid.NewString(), CreatedAt: now,
	}
	if ringFor > 0 {
		exp := now.Add(ringFor)
		c.RingExpiresAt = &exp
	}
	msg := &store.Message{
		ID: c.MessageID, ChannelID: w.chID, SenderID: w.users[0], Content: "📹 Cuộc gọi video",
		ContentFormat: "plain", MessageType: "system", LinkedEntityType: "call", LinkedEntityID: c.ID, CreatedAt: now,
	}
	err := w.st.InsertCallWithMessage(context.Background(), msg, c)
	t.Cleanup(func() { w.pool.Exec(context.Background(), `DELETE FROM calls WHERE id = $1`, c.ID) })
	return c, err
}

func TestCalls_OneOpenCallPerConversation(t *testing.T) {
	w := newThreadWorld(t, 1)
	ctx := context.Background()
	first, err := newCall(t, w, "active", 0)
	require.NoError(t, err)
	_, err = newCall(t, w, "active", 0)
	require.ErrorIs(t, err, store.ErrCallOpen, "a second open call is refused")
	var cards int
	require.NoError(t, w.pool.QueryRow(ctx, `SELECT count(*) FROM messages WHERE channel_id = $1 AND linked_entity_type = 'call'`, w.chID).Scan(&cards))
	assert.Equal(t, 1, cards, "and its card is not left behind")

	ended, err := w.st.TransitionCall(ctx, first.ID, []string{"ringing", "active"}, "ended")
	require.NoError(t, err)
	require.NotNil(t, ended)
	assert.NotNil(t, ended.EndedAt)
	again, err := w.st.TransitionCall(ctx, first.ID, []string{"ringing", "active"}, "ended")
	require.NoError(t, err)
	assert.Nil(t, again, "a call moves once")

	_, err = newCall(t, w, "active", 0)
	require.NoError(t, err, "once ended, a new call may start")
}

func TestCalls_ExpireRingingOnce(t *testing.T) {
	w := newThreadWorld(t, 1)
	ctx := context.Background()
	c, err := newCall(t, w, "ringing", 45*time.Second)
	require.NoError(t, err)

	none, err := w.st.ExpireRinging(ctx, time.Now())
	require.NoError(t, err)
	assert.Empty(t, none, "still ringing")
	expired, err := w.st.ExpireRinging(ctx, time.Now().Add(time.Minute))
	require.NoError(t, err)
	require.Len(t, expired, 1)
	assert.Equal(t, c.ID, expired[0].ID)
	assert.Equal(t, "missed", expired[0].Status)
	again, err := w.st.ExpireRinging(ctx, time.Now().Add(time.Minute))
	require.NoError(t, err)
	assert.Empty(t, again)
}

func TestCalls_StaysAndAccess(t *testing.T) {
	w := newThreadWorld(t, 2)
	ctx := context.Background()
	c, err := newCall(t, w, "active", 0)
	require.NoError(t, err)
	u := w.users[1]

	require.NoError(t, w.st.JoinCall(ctx, c.ID, u, "PA_1"))
	require.NoError(t, w.st.JoinCall(ctx, c.ID, u, "PA_1"), "the same connection twice")
	require.NoError(t, w.st.JoinCall(ctx, c.ID, u, "PA_2"))
	present, err := w.st.CallPresent(ctx, c.ID)
	require.NoError(t, err)
	require.Len(t, present, 1, "one person, however many connections")
	require.NoError(t, w.st.LeaveCall(ctx, c.ID, u, "PA_1"))
	present, _ = w.st.CallPresent(ctx, c.ID)
	assert.Len(t, present, 1)
	require.NoError(t, w.st.LeaveCall(ctx, c.ID, u, ""))
	present, _ = w.st.CallPresent(ctx, c.ID)
	assert.Empty(t, present)
	takers, err := w.st.CallTakers(ctx, c.ID)
	require.NoError(t, err)
	assert.EqualValues(t, 1, takers)

	node, tenant, err := w.st.CallAccessNode(ctx, c.ID, u)
	require.NoError(t, err)
	assert.Empty(t, node, "no ticket, no access")
	require.NoError(t, w.st.GrantCallAccess(ctx, c.ID, u, "node-a", "ws-a"))
	require.NoError(t, w.st.GrantCallAccess(ctx, c.ID, u, "node-b", "ws-b"))
	node, tenant, err = w.st.CallAccessNode(ctx, c.ID, u)
	require.NoError(t, err)
	assert.Equal(t, "node-b", node, "the latest ticket counts")
	assert.Equal(t, "ws-b", tenant)
}
