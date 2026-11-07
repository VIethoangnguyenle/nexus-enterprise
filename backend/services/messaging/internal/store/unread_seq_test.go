package store_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mainUnread is the user's unread main-message count for the world's channel.
func (w *threadWorld) mainUnread(t *testing.T, user string) int32 {
	t.Helper()
	counts, err := w.st.GetUnreadCounts(context.Background(), user, []string{w.chID})
	require.NoError(t, err)
	require.Len(t, counts, 1)
	return counts[0].UnreadCount
}

func (w *threadWorld) remove(t *testing.T, id, by string) {
	t.Helper()
	_, err := w.st.SoftDeleteMessage(context.Background(), id, by)
	require.NoError(t, err)
}

// A channel never read counts every live main message; replies are not main
// messages.
func TestUnreadMain_NeverReadCountsEveryLiveMainMessage(t *testing.T) {
	w := newThreadWorld(t, 2)
	a, b := w.users[0], w.users[1]
	root := w.post(t, a, "")
	w.post(t, a, "")
	w.post(t, a, root.ID)
	w.post(t, a, "")
	assert.EqualValues(t, 3, w.mainUnread(t, b))
}

// Reading counts nothing up to the read; what comes after is unread.
func TestUnreadMain_CountsWhatCameAfterTheRead(t *testing.T) {
	w := newThreadWorld(t, 2)
	a, b := w.users[0], w.users[1]
	w.post(t, a, "")
	w.post(t, a, "")
	w.read(t, b)
	assert.Zero(t, w.mainUnread(t, b))
	w.post(t, a, "")
	w.post(t, a, "")
	assert.EqualValues(t, 2, w.mainUnread(t, b))
}

// Deleting an unread message takes it off the count; deleting one already read
// changes nothing; the count never goes below zero.
func TestUnreadMain_DeletedMessagesCountOnlyWhileUnread(t *testing.T) {
	w := newThreadWorld(t, 2)
	a, b := w.users[0], w.users[1]
	seen := w.post(t, a, "")
	w.read(t, b)
	gone := w.post(t, a, "")
	w.post(t, a, "")
	require.EqualValues(t, 2, w.mainUnread(t, b))

	w.remove(t, gone.ID, a)
	assert.EqualValues(t, 1, w.mainUnread(t, b), "an unread message deleted is no longer unread")
	w.remove(t, seen.ID, a)
	assert.EqualValues(t, 1, w.mainUnread(t, b), "a read message deleted changes nothing")

	w.read(t, b)
	assert.Zero(t, w.mainUnread(t, b))
}

// Marking a message unread makes it and everything after it unread again.
func TestUnreadMain_MarkUnreadCountsFromThatMessage(t *testing.T) {
	w := newThreadWorld(t, 2)
	a, b := w.users[0], w.users[1]
	w.post(t, a, "")
	from := w.post(t, a, "")
	w.post(t, a, "")
	w.read(t, b)
	require.Zero(t, w.mainUnread(t, b))

	require.NoError(t, w.st.SetChannelReadBefore(context.Background(), b, w.chID, from.ID))
	assert.EqualValues(t, 2, w.mainUnread(t, b))
}

// Someone who had read everything has read the message they send next; someone
// still behind sees their own message among what they have not read yet.
func TestUnreadMain_OwnMessageIsReadByASenderWhoWasCaughtUp(t *testing.T) {
	w := newThreadWorld(t, 2)
	a, b := w.users[0], w.users[1]
	w.post(t, a, "")
	w.read(t, b)
	w.post(t, b, "")
	assert.Zero(t, w.mainUnread(t, b), "caught up, then sent")

	w.post(t, a, "")
	w.post(t, b, "")
	assert.EqualValues(t, 2, w.mainUnread(t, b), "behind: the message before theirs, and theirs")
}

// With notifications off the count is zero whatever is unread.
func TestUnreadMain_LevelOffCountsNothing(t *testing.T) {
	w := newThreadWorld(t, 2)
	a, b := w.users[0], w.users[1]
	w.post(t, a, "")
	require.NoError(t, w.st.SetNotificationLevel(context.Background(), b, w.chID, "off"))
	assert.Zero(t, w.mainUnread(t, b))
}

// Only the channels asked about are answered: a channel the caller did not pass
// (one they may not read) is never in the answer, whatever happens in it.
func TestUnreadMain_OnlyTheChannelsAskedAboutAreAnswered(t *testing.T) {
	w := newThreadWorld(t, 2)
	other := newThreadWorld(t, 1)
	a, b := w.users[0], w.users[1]
	w.post(t, a, "")
	other.post(t, other.users[0], "", b)

	counts, err := w.st.GetUnreadCounts(context.Background(), b, []string{w.chID})
	require.NoError(t, err)
	require.Len(t, counts, 1)
	assert.Equal(t, w.chID, counts[0].ChannelID)
	assert.EqualValues(t, 1, counts[0].UnreadCount)
	assert.Zero(t, counts[0].MentionCount, "a mention in a channel not asked about is not counted")
}
