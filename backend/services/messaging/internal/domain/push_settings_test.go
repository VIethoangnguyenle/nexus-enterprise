package domain_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/services/messaging/internal/domain"
	"ngac-platform/services/messaging/internal/store"
	"ngac-platform/testutil"
)

// quietNow is preferences whose quiet window contains the present moment.
func quietNow() store.PushPreferences {
	p := store.DefaultPushPreferences
	now := time.Now().UTC()
	p.QuietEnabled, p.TimeZone = true, "UTC"
	p.QuietStart, p.QuietEnd = now.Hour()*60, (now.Hour()*60+120)%1440
	return p
}

// A group where `other` is a recorded member, so a plain message concerns them
// and only the quiet hours / pause decide whether they hear of it.
func newGroupWithMember(t *testing.T) *pushWorld {
	t.Helper()
	w := newPushWorld(t, false)
	_, err := w.f.pool.Exec(context.Background(), `INSERT INTO channel_members (channel_id, ngac_node_id) VALUES ($1, $2)`, w.chID, w.otherNode)
	require.NoError(t, err)
	t.Cleanup(func() {
		w.f.pool.Exec(context.Background(), `DELETE FROM channel_members WHERE channel_id = $1`, w.chID)
	})
	return w
}

func TestPush_QuietHoursLetAMentionThrough(t *testing.T) {
	w := newGroupWithMember(t)
	require.NoError(t, w.f.svc.SavePushPreferences(w.f.callerCtx(), w.other, quietNow()))

	w.send(t, "không nhắc ai")
	assert.Empty(t, w.sender.all(), "quiet hours, not mentioned: not sent")

	third, _ := testutil.CreateUser(t, w.f.pool)
	w.send(t, "nhắc người khác", third)
	assert.Empty(t, w.sender.all(), "quiet hours, someone else mentioned: still not sent")

	w.send(t, "nhắc bạn", w.other)
	sent := w.sender.all()
	require.Len(t, sent, 1, "quiet hours, mentioned by name: sent")
	assert.Equal(t, []string{w.other}, sent[0].ExternalIDs)
}

func TestPush_OutsideQuietHoursAGroupMessageIsSent(t *testing.T) {
	w := newGroupWithMember(t)
	prefs := quietNow()
	prefs.QuietEnabled = false
	require.NoError(t, w.f.svc.SavePushPreferences(w.f.callerCtx(), w.other, prefs))
	w.send(t, "bình thường")
	assert.Len(t, w.sender.all(), 1, "outside quiet hours: sent, so the quiet test above proves the block")
}

func TestPush_QuietHoursKeepTheMentionSwitch(t *testing.T) {
	w := newGroupWithMember(t)
	prefs := quietNow()
	prefs.Mention = false
	require.NoError(t, w.f.svc.SavePushPreferences(w.f.callerCtx(), w.other, prefs))
	w.send(t, "nhắc bạn", w.other)
	assert.Empty(t, w.sender.all(), "a mention is not let through a switch that is off")
}

func TestPush_PausedStopsEverythingMentionsIncluded(t *testing.T) {
	w := newPushWorld(t, true)
	ctx := w.f.callerCtx()
	prefs := store.DefaultPushPreferences
	until := time.Now().Add(time.Hour)
	prefs.PauseUntil = &until
	require.NoError(t, w.f.svc.SavePushPreferences(ctx, w.other, prefs))

	w.send(t, "tin nhắn trực tiếp")
	w.send(t, "nhắc bạn", w.other)
	assert.Empty(t, w.sender.all(), "paused: nothing, a mention neither")

	// Resuming (cleared) delivers again.
	prefs.PauseUntil = nil
	require.NoError(t, w.f.svc.SavePushPreferences(ctx, w.other, prefs))
	w.send(t, "đã bật lại")
	assert.Len(t, w.sender.all(), 1)
}

func TestPush_APauseThatEndedDoesNotSilence(t *testing.T) {
	w := newPushWorld(t, true)
	ctx := w.f.callerCtx()
	prefs := store.DefaultPushPreferences
	past := time.Now().Add(-time.Minute)
	prefs.PauseUntil = &past
	require.NoError(t, w.f.svc.SavePushPreferences(ctx, w.other, prefs))
	got, err := w.f.svc.GetPushPreferences(ctx, w.other)
	require.NoError(t, err)
	assert.Nil(t, got.PauseUntil, "an ended pause reads as not paused")
	w.send(t, "xin chào")
	assert.Len(t, w.sender.all(), 1)
}

func TestPush_PauseIsSavedAndRead(t *testing.T) {
	w := newPushWorld(t, false)
	ctx := w.f.callerCtx()
	prefs := store.DefaultPushPreferences
	until := time.Now().Add(2 * time.Hour).Truncate(time.Second)
	prefs.PauseUntil = &until
	require.NoError(t, w.f.svc.SavePushPreferences(ctx, w.other, prefs))
	got, err := w.f.svc.GetPushPreferences(ctx, w.other)
	require.NoError(t, err)
	require.NotNil(t, got.PauseUntil)
	assert.True(t, got.PauseUntil.Equal(until))
	// Another person's row is untouched.
	mine, err := w.f.svc.GetPushPreferences(ctx, w.f.userID)
	require.NoError(t, err)
	assert.Nil(t, mine.PauseUntil)
}

func TestPush_DirectSwitchOffStopsADirectMessage(t *testing.T) {
	w := newPushWorld(t, true)
	prefs := store.DefaultPushPreferences
	prefs.Direct = false
	require.NoError(t, w.f.svc.SavePushPreferences(w.f.callerCtx(), w.other, prefs))
	w.send(t, "chào")
	assert.Empty(t, w.sender.all(), "direct off: no push")
	// The mention switch is separate: a mention in the DM still counts as one.
	w.send(t, "nhắc", w.other)
	assert.Len(t, w.sender.all(), 1)
}

func TestPush_ThreadReplySwitchOffStopsAFollowedThread(t *testing.T) {
	k := newThreadTalk(t)
	ctx := k.f.callerCtx()
	sender := &recordingSender{}
	k.f.svc.SetPush(sender, viewing{})
	prefs := store.DefaultPushPreferences
	prefs.ThreadReply = false
	require.NoError(t, k.f.svc.SavePushPreferences(ctx, k.f.userID, prefs))
	reply, err := k.f.svc.SendMessage(ctx, domain.SendMessageInput{
		ChannelID: k.chID, SenderID: k.replier, SenderNodeID: replierNode, Content: "thêm ý", ParentMessageID: k.rootID,
	})
	require.NoError(t, err)
	k.f.svc.PushForMessage(ctx, reply)
	assert.Empty(t, sender.all(), "thread replies off: the starter hears nothing")
}

func TestPush_CalendarAndCallsStopWhilePaused(t *testing.T) {
	w := newCallWorld(t)
	w.asDM(t)
	sender, _ := w.withPush()
	ctx := w.f.callerCtx()
	prefs := store.DefaultPushPreferences
	until := time.Now().Add(time.Hour)
	prefs.PauseUntil = &until
	require.NoError(t, w.f.svc.SavePushPreferences(ctx, w.peer, prefs))

	w.f.svc.PushCalendar(ctx, w.peer, domain.CalendarPush{Kind: domain.CalendarPushReminder, EventID: "e1", Title: "Standup", Start: time.Now()})
	w.start(t, "video")
	assert.Empty(t, w.pushed(sender), "paused: no event reminder, no ring")
}
