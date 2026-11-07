package store_test

import (
	"context"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/services/messaging/internal/store"
)

// seqOf reads the numbers the database gave a stored message.
func seqOf(t *testing.T, w *threadWorld, id string) (seq, replySeq *int64) {
	t.Helper()
	require.NoError(t, w.pool.QueryRow(context.Background(),
		`SELECT seq, reply_seq FROM messages WHERE id = $1`, id).Scan(&seq, &replySeq))
	return seq, replySeq
}

func lastSeqOf(t *testing.T, w *threadWorld) int64 {
	t.Helper()
	var n int64
	require.NoError(t, w.pool.QueryRow(context.Background(),
		`SELECT last_seq FROM channels WHERE id = $1`, w.chID).Scan(&n))
	return n
}

// Main messages are numbered 1, 2, 3… in their channel, and the channel keeps
// the last number given; a reply has no main number but is numbered in its own
// thread, and its root keeps the thread's last number.
func TestSequences_MainMessagesAndRepliesAreNumberedInOrder(t *testing.T) {
	w := newThreadWorld(t, 2)
	a, b := w.users[0], w.users[1]

	m1 := w.post(t, a, "")
	m2 := w.post(t, b, "")
	r1 := w.post(t, b, m1.ID)
	r2 := w.post(t, a, m1.ID)
	m3 := w.post(t, a, "")

	for want, m := range map[int64]*store.Message{1: m1, 2: m2, 3: m3} {
		seq, replySeq := seqOf(t, w, m.ID)
		require.NotNil(t, seq)
		assert.Equal(t, want, *seq)
		assert.Nil(t, replySeq)
		assert.Equal(t, want, m.Seq, "the stored message carries its number back")
	}
	for want, r := range map[int64]*store.Message{1: r1, 2: r2} {
		seq, replySeq := seqOf(t, w, r.ID)
		assert.Nil(t, seq, "a reply takes no number in the main stream")
		require.NotNil(t, replySeq)
		assert.Equal(t, want, *replySeq)
		assert.Equal(t, want, r.ReplySeq)
	}
	assert.EqualValues(t, 3, lastSeqOf(t, w))

	var lastReply int64
	require.NoError(t, w.pool.QueryRow(context.Background(),
		`SELECT last_reply_seq FROM messages WHERE id = $1`, m1.ID).Scan(&lastReply))
	assert.EqualValues(t, 2, lastReply)
}

// The same announcement stored twice is one message, and it does not use up a
// number: the next message follows on with no gap, so a count made by
// subtracting numbers stays exact.
func TestSequences_AnAnnouncementStoredTwiceLeavesNoGap(t *testing.T) {
	w := newThreadWorld(t, 1)
	ctx := context.Background()
	once := &store.Message{ID: "m-" + uuid.NewString(), ChannelID: w.chID, SenderID: w.users[0],
		Content: "event", MessageType: "system", CreatedAt: w.clock}

	stored, err := w.st.InsertMessageOnce(ctx, once)
	require.NoError(t, err)
	assert.True(t, stored)
	stored, err = w.st.InsertMessageOnce(ctx, once)
	require.NoError(t, err)
	assert.False(t, stored, "the second delivery stores nothing")

	next := w.post(t, w.users[0], "")
	seq, _ := seqOf(t, w, next.ID)
	require.NotNil(t, seq)
	assert.EqualValues(t, 2, *seq)
	assert.EqualValues(t, 2, lastSeqOf(t, w))
}

// Many people sending into one channel at once get every number exactly once.
func TestSequences_ConcurrentSendersGetDistinctNumbersWithoutGaps(t *testing.T) {
	w := newThreadWorld(t, 1)
	ctx := context.Background()
	const senders = 50

	var wg sync.WaitGroup
	errs := make(chan error, senders)
	for range senders {
		wg.Add(1)
		go func() {
			defer wg.Done()
			m := &store.Message{ID: "m-" + uuid.NewString(), ChannelID: w.chID, SenderID: w.users[0],
				Content: "hi", MessageType: "user", CreatedAt: w.clock}
			errs <- w.st.InsertMessage(ctx, m)
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}

	var n, distinct, low, high int64
	require.NoError(t, w.pool.QueryRow(ctx,
		`SELECT count(*), count(DISTINCT seq), min(seq), max(seq) FROM messages WHERE channel_id = $1`,
		w.chID).Scan(&n, &distinct, &low, &high))
	assert.EqualValues(t, senders, n)
	assert.EqualValues(t, senders, distinct)
	assert.EqualValues(t, 1, low)
	assert.EqualValues(t, senders, high)
	assert.EqualValues(t, senders, lastSeqOf(t, w))
}
