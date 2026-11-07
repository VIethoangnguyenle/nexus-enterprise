package store_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/services/messaging/internal/store"
)

// Pages by number meet exactly, even where messages share a send time: nothing
// is skipped or shown twice. A deleted message with replies stays as the
// thread's placeholder; one without is left out.
func TestListMessages_PagesByNumberMeetExactly(t *testing.T) {
	w := newThreadWorld(t, 1)
	ctx := context.Background()
	a := w.users[0]

	// Seven messages, the middle three sent at the same moment.
	var ids []string
	for i := range 7 {
		if i < 3 || i > 4 {
			w.clock = w.clock.Add(1e9)
		}
		m := &store.Message{ID: "m-" + uuid.NewString(), ChannelID: w.chID, SenderID: a, Content: "x",
			MessageType: "user", CreatedAt: w.clock}
		require.NoError(t, w.st.InsertMessage(ctx, m))
		ids = append(ids, m.ID)
	}

	var got []string
	var before int64
	for {
		page, more, err := w.st.ListMessages(ctx, w.chID, before, nil, 2)
		require.NoError(t, err)
		for _, m := range page {
			got = append(got, m.ID)
			before = m.Seq
		}
		if !more {
			break
		}
	}
	want := make([]string, len(ids))
	for i, id := range ids {
		want[len(ids)-1-i] = id
	}
	assert.Equal(t, want, got, "newest first, every message once")
}

func TestListMessages_DeletedRootStaysOnlyWhileItHasReplies(t *testing.T) {
	w := newThreadWorld(t, 1)
	a := w.users[0]
	kept := w.post(t, a, "")
	w.post(t, a, kept.ID)
	dropped := w.post(t, a, "")
	w.remove(t, kept.ID, a)
	w.remove(t, dropped.ID, a)

	page, more, err := w.st.ListMessages(context.Background(), w.chID, 0, nil, 50)
	require.NoError(t, err)
	assert.False(t, more)
	require.Len(t, page, 1)
	assert.Equal(t, kept.ID, page[0].ID)
	assert.True(t, page[0].Deleted)
}

// A thread is read a page of replies at a time, oldest first, after a reply
// number; the root comes with every page.
func TestGetThread_PagesThroughReplies(t *testing.T) {
	w := newThreadWorld(t, 2)
	ctx := context.Background()
	a, b := w.users[0], w.users[1]
	root := w.post(t, a, "")
	var replies []string
	for range 5 {
		replies = append(replies, w.post(t, b, root.ID).ID)
	}

	var got []string
	var after int64
	for {
		page, more, err := w.st.GetThread(ctx, root.ID, after, 2)
		require.NoError(t, err)
		require.NotEmpty(t, page)
		assert.Equal(t, root.ID, page[0].ID, "the root leads every page")
		for _, m := range page[1:] {
			got = append(got, m.ID)
			after = m.ReplySeq
		}
		if !more {
			break
		}
	}
	assert.Equal(t, replies, got)
}
