package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// signals are the three in-app unread-mention signals of one person.
type signals struct {
	badge, channel int
	feed           map[string]bool // message id -> unread
}

func (w *threadWorld) signals(t *testing.T, user string) signals {
	t.Helper()
	ctx := context.Background()
	badge, err := w.st.CountUnreadMentions(ctx, user, []string{w.chID})
	require.NoError(t, err)
	counts, err := w.st.GetUnreadCounts(ctx, user, []string{w.chID})
	require.NoError(t, err)
	require.Len(t, counts, 1)
	feed, _, err := w.st.MentionsOf(ctx, user, []string{w.chID}, nil, "", 50)
	require.NoError(t, err)
	out := signals{badge: badge, channel: int(counts[0].MentionCount), feed: map[string]bool{}}
	for _, m := range feed {
		out.feed[m.ID] = m.Unread
	}
	return out
}

func (w *threadWorld) edit(t *testing.T, id string, mentions []string) []string {
	t.Helper()
	time.Sleep(15 * time.Millisecond) // the edit lands strictly after the read that came before it
	added, err := w.st.UpdateMessageContent(context.Background(), id, "edited", "edited", "markdown", mentions)
	require.NoError(t, err)
	return added
}

func (w *threadWorld) read(t *testing.T, user string) {
	t.Helper()
	time.Sleep(15 * time.Millisecond)
	require.NoError(t, w.st.UpsertReadReceipt(context.Background(), user, w.chID, ""))
}

// A person an edit adds to a message older than their read position has an
// unread mention in the badge, the channel count and the feed until they read
// past the edit.
func TestEditMentionUnread_AddedByAnEditIsUnreadUntilReadPastIt(t *testing.T) {
	w := newThreadWorld(t, 3)
	author, lan, minh := w.users[0], w.users[1], w.users[2]
	old := w.post(t, author, "", minh)
	w.read(t, lan)
	w.read(t, minh)

	before := w.signals(t, lan)
	assert.Zero(t, before.badge)
	assert.Empty(t, before.feed)

	assert.Equal(t, []string{lan}, w.edit(t, old.ID, []string{minh, lan}))

	got := w.signals(t, lan)
	assert.Equal(t, 1, got.badge)
	assert.Equal(t, 1, got.channel)
	assert.Equal(t, map[string]bool{old.ID: true}, got.feed)

	// Minh was mentioned before the edit and had read it: nothing new for him.
	same := w.signals(t, minh)
	assert.Zero(t, same.badge)
	assert.Zero(t, same.channel)
	assert.Equal(t, map[string]bool{old.ID: false}, same.feed)

	// Reading past the edit clears it.
	w.read(t, lan)
	got = w.signals(t, lan)
	assert.Zero(t, got.badge)
	assert.Zero(t, got.channel)
	assert.Equal(t, map[string]bool{old.ID: false}, got.feed, "still listed, no longer unread")
}

func TestEditMentionUnread_RemovedByALaterEditStopsCounting(t *testing.T) {
	w := newThreadWorld(t, 2)
	author, lan := w.users[0], w.users[1]
	old := w.post(t, author, "")
	w.read(t, lan)
	w.edit(t, old.ID, []string{lan})
	require.Equal(t, 1, w.signals(t, lan).badge)

	w.edit(t, old.ID, []string{})
	got := w.signals(t, lan)
	assert.Zero(t, got.badge)
	assert.Zero(t, got.channel)
	assert.Empty(t, got.feed)

	// Mentioned again after having read past the first edit: unread again.
	w.read(t, lan)
	w.edit(t, old.ID, []string{lan})
	got = w.signals(t, lan)
	assert.Equal(t, 1, got.badge)
	assert.Equal(t, map[string]bool{old.ID: true}, got.feed)
}

// Saving a message again with the same mentions does not make a read mention unread.
func TestEditMentionUnread_AnEditThatAddsNobodyChangesNothing(t *testing.T) {
	w := newThreadWorld(t, 3)
	author, lan, minh := w.users[0], w.users[1], w.users[2]
	old := w.post(t, author, "", lan)
	w.read(t, lan)

	assert.Empty(t, w.edit(t, old.ID, []string{lan}))
	assert.Empty(t, w.edit(t, old.ID, nil))
	assert.Zero(t, w.signals(t, lan).badge)

	// A person who is not mentioned sees nothing.
	w.edit(t, old.ID, []string{lan, minh})
	assert.Zero(t, w.signals(t, author).badge)
	assert.Empty(t, w.signals(t, author).feed)
}

func TestEditMentionUnread_ADeletedMessageNeverCounts(t *testing.T) {
	w := newThreadWorld(t, 2)
	author, lan := w.users[0], w.users[1]
	old := w.post(t, author, "")
	w.read(t, lan)
	w.edit(t, old.ID, []string{lan})
	require.Equal(t, 1, w.signals(t, lan).badge)

	_, err := w.st.SoftDeleteMessage(context.Background(), old.ID, author)
	require.NoError(t, err)
	got := w.signals(t, lan)
	assert.Zero(t, got.badge)
	assert.Zero(t, got.channel)
	assert.Empty(t, got.feed)
}

// A reply is unread against its thread's read position, not the channel's.
func TestEditMentionUnread_AReplyFollowsItsThreadsReadPosition(t *testing.T) {
	w := newThreadWorld(t, 3)
	author, replier, lan := w.users[0], w.users[1], w.users[2]
	root := w.post(t, author, "")
	reply := w.post(t, replier, root.ID)
	ctx := context.Background()
	w.read(t, lan)
	time.Sleep(15 * time.Millisecond)
	require.NoError(t, w.st.MarkThreadRead(ctx, lan, root.ID))
	require.Zero(t, w.signals(t, lan).badge)

	w.edit(t, reply.ID, []string{lan})
	got := w.signals(t, lan)
	assert.Equal(t, 1, got.badge)
	assert.Zero(t, got.channel, "the channel count is of main messages")
	assert.Equal(t, map[string]bool{reply.ID: true}, got.feed)

	time.Sleep(15 * time.Millisecond)
	require.NoError(t, w.st.MarkThreadRead(ctx, lan, root.ID))
	assert.Zero(t, w.signals(t, lan).badge)
}

// The sender's own mention of themselves never counts.
func TestEditMentionUnread_NeverCountsTheSenderThemselves(t *testing.T) {
	w := newThreadWorld(t, 1)
	author := w.users[0]
	old := w.post(t, author, "")
	w.read(t, author)
	w.edit(t, old.ID, []string{author})
	got := w.signals(t, author)
	assert.Zero(t, got.badge)
	assert.Zero(t, got.channel)
	assert.Empty(t, got.feed)
}
