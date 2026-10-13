package store_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLatestActivity_NewestMainMessageAndNewestReplyPerChannel(t *testing.T) {
	w := newThreadWorld(t, 2)
	ctx := context.Background()
	first := w.post(t, w.users[0], "")
	last := w.post(t, w.users[1], "")
	w.post(t, w.users[1], first.ID) // an older reply
	reply := w.post(t, w.users[0], first.ID)

	main, replies, err := w.st.LatestActivity(ctx, []string{w.chID})
	require.NoError(t, err)
	require.Contains(t, main, w.chID)
	assert.Equal(t, last.ID, main[w.chID].ID, "a reply never stands for the main stream")
	assert.Equal(t, w.users[1], main[w.chID].SenderID)
	require.Contains(t, replies, w.chID)
	assert.Equal(t, reply.ID, replies[w.chID].ID)
	assert.Equal(t, first.ID, replies[w.chID].ParentMessageID)
}

func TestLatestActivity_LeavesOutDeletedMessagesAndEmptyChannels(t *testing.T) {
	w := newThreadWorld(t, 1)
	ctx := context.Background()

	main, replies, err := w.st.LatestActivity(ctx, []string{w.chID})
	require.NoError(t, err)
	assert.Empty(t, main, "a channel with no message has no entry")
	assert.Empty(t, replies)

	kept := w.post(t, w.users[0], "")
	gone := w.post(t, w.users[0], "")
	goneReply := w.post(t, w.users[0], kept.ID)
	for _, id := range []string{gone.ID, goneReply.ID} {
		_, err := w.pool.Exec(ctx, `UPDATE messages SET deleted_at = now() WHERE id = $1`, id)
		require.NoError(t, err)
	}

	main, replies, err = w.st.LatestActivity(ctx, []string{w.chID})
	require.NoError(t, err)
	assert.Equal(t, kept.ID, main[w.chID].ID, "the deleted message is skipped")
	assert.NotContains(t, replies, w.chID, "the deleted reply is skipped")
}

func TestLatestActivity_UsesTheCurrentTextOfAnEditedMessage(t *testing.T) {
	w := newThreadWorld(t, 1)
	ctx := context.Background()
	m := w.post(t, w.users[0], "")
	_, err := w.pool.Exec(ctx, `UPDATE messages SET content = 'after edit', edited_at = now() WHERE id = $1`, m.ID)
	require.NoError(t, err)

	main, _, err := w.st.LatestActivity(ctx, []string{w.chID})
	require.NoError(t, err)
	assert.Equal(t, "after edit", main[w.chID].Content)
	assert.NotNil(t, main[w.chID].EditedAt)
}

func TestLatestActivity_OnlyTheChannelsAsked(t *testing.T) {
	a := newThreadWorld(t, 1)
	b := newThreadWorld(t, 1)
	ctx := context.Background()
	ma := a.post(t, a.users[0], "")
	b.post(t, b.users[0], "")

	main, _, err := a.st.LatestActivity(ctx, []string{a.chID})
	require.NoError(t, err)
	require.Len(t, main, 1)
	assert.Equal(t, ma.ID, main[a.chID].ID)

	main, replies, err := a.st.LatestActivity(ctx, nil)
	require.NoError(t, err)
	assert.Empty(t, main)
	assert.Empty(t, replies)
}

func TestFollowedThreads_AcrossChannelsNewestActivityFirstWithPaging(t *testing.T) {
	a := newThreadWorld(t, 2)
	b := newThreadWorld(t, 2)
	me := a.users[0]
	ctx := context.Background()

	rootA := a.post(t, me, "") // started by the caller: followed implicitly
	a.post(t, a.users[1], rootA.ID)
	rootB := b.post(t, me, "")
	b.post(t, b.users[1], rootB.ID)
	notMine := a.post(t, a.users[1], "") // someone else's thread, not followed
	a.post(t, a.users[1], notMine.ID)
	require.NoError(t, a.st.SetThreadFollow(ctx, me, notMine.ID, false))
	bump := func(id string, delta string) {
		_, err := a.pool.Exec(ctx, `UPDATE messages SET last_reply_at = now() + $2::interval WHERE id = $1`, id, delta)
		require.NoError(t, err)
	}
	bump(rootA.ID, "1 minute")
	bump(rootB.ID, "2 minutes")

	msgs, more, err := a.st.FollowedThreads(ctx, me, []string{a.chID, b.chID}, nil, "", 10)
	require.NoError(t, err)
	assert.False(t, more)
	require.Len(t, msgs, 2)
	assert.Equal(t, rootB.ID, msgs[0].ID, "newest activity first, across channels")
	assert.Equal(t, rootA.ID, msgs[1].ID)
	assert.True(t, msgs[0].Following)

	page, more, err := a.st.FollowedThreads(ctx, me, []string{a.chID, b.chID}, nil, "", 1)
	require.NoError(t, err)
	require.Len(t, page, 1)
	assert.True(t, more)
	next, more, err := a.st.FollowedThreads(ctx, me, []string{a.chID, b.chID}, page[0].LastReplyAt, page[0].ID, 1)
	require.NoError(t, err)
	require.Len(t, next, 1)
	assert.False(t, more)
	assert.Equal(t, rootA.ID, next[0].ID)

	only, _, err := a.st.FollowedThreads(ctx, me, []string{b.chID}, nil, "", 10)
	require.NoError(t, err)
	require.Len(t, only, 1, "only the channels asked for")
	assert.Equal(t, rootB.ID, only[0].ID)

	none, _, err := a.st.FollowedThreads(ctx, me, nil, nil, "", 10)
	require.NoError(t, err)
	assert.Empty(t, none)
}
