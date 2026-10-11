package domain_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/ngac"
	"ngac-platform/services/messaging/internal/domain"
	"ngac-platform/services/messaging/internal/store"
	"ngac-platform/testutil"
)

// editWith edits the world's seed message and waits for the push it starts.
func (w *pushWorld) editWith(t *testing.T, mentions []string) {
	t.Helper()
	_, err := w.f.svc.EditMessage(context.Background(), w.msgID, w.f.userID, userNode, "sửa lại", "markdown", mentions)
	require.NoError(t, err)
	domain.WaitPushForTest(w.f.svc)
}

func (w *pushWorld) pushedTo() map[string]int {
	got := map[string]int{}
	for _, m := range w.sender.all() {
		for _, id := range m.ExternalIDs {
			got[id]++
		}
	}
	return got
}

func (w *pushWorld) follows(t *testing.T) bool {
	t.Helper()
	thread, err := w.f.svc.GetThread(context.Background(), w.msgID, w.other, w.otherNode)
	require.NoError(t, err)
	return thread.Messages[0].Following
}

func (w *pushWorld) storedMentions(t *testing.T) []string {
	t.Helper()
	var ids []string
	require.NoError(t, w.f.pool.QueryRow(context.Background(),
		`SELECT COALESCE(mentions, '{}') FROM messages WHERE id = $1`, w.msgID).Scan(&ids))
	return ids
}

func TestEditMentions_OnlyTheNewlyMentionedAreNotified(t *testing.T) {
	w := newPushWorld(t, false)
	third, thirdNode := testutil.CreateUser(t, w.f.pool)
	w.f.read.allow[grant{thirdNode, w.oaID, ngac.OpRead}] = true

	w.editWith(t, []string{w.other})
	assert.Equal(t, map[string]int{w.other: 1}, w.pushedTo(), "a mention added by an edit is pushed")

	// Re-saving with the same mentions tells nobody again.
	w.editWith(t, []string{w.other})
	assert.Equal(t, map[string]int{w.other: 1}, w.pushedTo())

	// Adding a second person pushes only that person.
	w.editWith(t, []string{w.other, third})
	assert.Equal(t, map[string]int{w.other: 1, third: 1}, w.pushedTo())
	assert.ElementsMatch(t, []string{w.other, third}, w.storedMentions(t))

	// Removing a mention notifies nobody, and the stored list shrinks.
	w.editWith(t, []string{third})
	assert.Equal(t, map[string]int{w.other: 1, third: 1}, w.pushedTo())
	assert.Equal(t, []string{third}, w.storedMentions(t))

	// An edit that says nothing about mentions leaves them as they are.
	w.editWith(t, nil)
	assert.Equal(t, []string{third}, w.storedMentions(t))
	assert.Equal(t, map[string]int{w.other: 1, third: 1}, w.pushedTo())

	// An explicit empty list clears them.
	w.editWith(t, []string{})
	assert.Empty(t, w.storedMentions(t))
}

func TestEditMentions_AddedWithoutReadOrWithPushOffGetNothing(t *testing.T) {
	w := newPushWorld(t, false)
	ctx := context.Background()

	stranger, _ := testutil.CreateUser(t, w.f.pool)
	w.editWith(t, []string{stranger})
	assert.Empty(t, w.pushedTo(), "no read on the channel, no push")
	assert.Equal(t, []string{stranger}, w.storedMentions(t), "the mention is still stored")

	prefs := store.DefaultPushPreferences
	prefs.Mention = false
	require.NoError(t, w.f.svc.SavePushPreferences(ctx, w.other, prefs))
	w.editWith(t, []string{stranger, w.other})
	assert.Empty(t, w.pushedTo(), "the kind is off")
}

func TestEditMentions_OffStillLetsAGroupMentionThroughButNotADirectOne(t *testing.T) {
	group := newPushWorld(t, false)
	require.NoError(t, group.f.svc.SetNotificationLevel(context.Background(), group.chID, group.other, group.otherNode, domain.NotifyOff))
	group.editWith(t, []string{group.other})
	assert.Equal(t, 1, group.pushedTo()[group.other], "a mention gets through under off in a group")

	dm := newPushWorld(t, true)
	require.NoError(t, dm.f.svc.SetNotificationLevel(context.Background(), dm.chID, dm.other, dm.otherNode, domain.NotifyOff))
	dm.editWith(t, []string{dm.other})
	assert.Empty(t, dm.pushedTo(), "a direct message under off is silent")
}

func TestEditMentions_AnEditPushesNothingButTheMention(t *testing.T) {
	// "all" on a group and a direct message each push every new message; an edit
	// is not a new message and must not push on those grounds.
	w := newPushWorld(t, true)
	w.editWith(t, nil)
	assert.Empty(t, w.pushedTo())
}

func TestEditMentions_NotWhileViewing(t *testing.T) {
	w := newPushWorld(t, false)
	w.view[w.other+"/"+w.chID] = true
	w.editWith(t, []string{w.other})
	assert.Empty(t, w.pushedTo())
}

func TestEditMentions_AtMostTwentyAreNotified(t *testing.T) {
	w := newPushWorld(t, false)
	var ids []string
	for i := 0; i < domain.MaxMentionPushes+5; i++ {
		id, node := testutil.CreateUser(t, w.f.pool)
		w.f.read.allow[grant{node, w.oaID, ngac.OpRead}] = true
		ids = append(ids, id)
	}
	w.editWith(t, ids)
	assert.Len(t, w.pushedTo(), domain.MaxMentionPushes)
}

func TestEditMentions_AMentionInARootMakesTheMentionedFollowItsThread(t *testing.T) {
	w := newPushWorld(t, false)
	w.editWith(t, []string{w.other})
	assert.True(t, w.follows(t), "mentioned in a root, follows its thread")
}

func TestEditMentions_AMentionInAReplyMakesTheMentionedFollowTheThread(t *testing.T) {
	w := newPushWorld(t, false)
	ctx := context.Background()
	reply, err := w.f.svc.SendMessage(ctx, domain.SendMessageInput{
		ChannelID: w.chID, SenderID: w.f.userID, SenderNodeID: userNode, Content: "trả lời", ParentMessageID: w.msgID,
	})
	require.NoError(t, err)
	_, err = w.f.svc.EditMessage(ctx, reply.Id, w.f.userID, userNode, "trả lời lại", "markdown", []string{w.other})
	require.NoError(t, err)
	domain.WaitPushForTest(w.f.svc)

	assert.True(t, w.follows(t))
	assert.Equal(t, map[string]int{w.other: 1}, w.pushedTo(), "pushed as a mention, once")
}
