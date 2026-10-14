package domain_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/pkg/push"
	"ngac-platform/services/messaging/internal/domain"
	"ngac-platform/services/messaging/internal/store"
)

// withPush turns web push on for the world and returns what it sent.
func (w *callWorld) withPush() (*recordingSender, viewing) {
	sender, view := &recordingSender{}, viewing{}
	w.f.svc.SetPush(sender, view)
	return sender, view
}

func (w *callWorld) pushed(sender *recordingSender) []push.Message {
	domain.WaitPushForTest(w.f.svc)
	return sender.all()
}

func TestCallPush_RingsTheCalleeThenMissedReplacesIt(t *testing.T) {
	w := newCallWorld(t)
	w.asDM(t)
	sender, _ := w.withPush()
	id := w.start(t, "video")

	sent := w.pushed(sender)
	require.Len(t, sent, 1)
	ring := sent[0]
	assert.Equal(t, []string{w.peer}, ring.ExternalIDs, "the callee, never the caller")
	assert.Equal(t, "testuser", ring.Title, "titled by the caller")
	assert.Equal(t, "đang gọi video cho bạn", ring.Body)
	assert.Equal(t, "/calls/"+id+"?answer=video", ring.URL, "answering opens the call")
	assert.Equal(t, "call-"+id, ring.CollapseID)
	assert.Equal(t, []push.Button{{ID: "answer", Text: "Trả lời", URL: "/calls/" + id + "?answer=video"}}, ring.Buttons,
		"the notification answers from its own button")

	_, err := w.f.svc.ExpireRingingCalls(context.Background(), time.Now().Add(time.Minute))
	require.NoError(t, err)
	sent = w.pushed(sender)
	require.Len(t, sent, 2)
	missed := sent[1]
	assert.Equal(t, []string{w.peer}, missed.ExternalIDs)
	assert.Equal(t, "📹 Cuộc gọi nhỡ", missed.Body)
	assert.Equal(t, "call-"+id, missed.CollapseID, "it replaces the ringing notification")
	assert.Equal(t, "/channels/"+w.chID, missed.URL, "a missed call opens the conversation to call back")
	assert.Empty(t, missed.Buttons, "nothing left to answer")
}

func TestCallPush_CancelledLeavesAMissedCallDeclinedNothing(t *testing.T) {
	w := newCallWorld(t)
	w.asDM(t)
	ctx := context.Background()
	sender, _ := w.withPush()

	id := w.start(t, "audio")
	_, err := w.f.svc.CancelCall(ctx, id, w.f.userID, userNode)
	require.NoError(t, err)
	sent := w.pushed(sender)
	require.Len(t, sent, 2)
	assert.Equal(t, "📞 Cuộc gọi nhỡ", sent[1].Body)

	id = w.start(t, "audio")
	_, err = w.f.svc.DeclineCall(ctx, id, w.peer, w.peerNode)
	require.NoError(t, err)
	assert.Len(t, w.pushed(sender), 3, "the callee declined it themself: only the ring was pushed")
}

// Deny: no ring push when the person turned incoming calls off, in their quiet
// hours, while they have the conversation open (the app rings), or for a
// group call (which rings nobody).
func TestCallPush_Deny(t *testing.T) {
	ctx := context.Background()

	w := newCallWorld(t)
	w.asDM(t)
	sender, view := w.withPush()
	prefs := store.DefaultPushPreferences
	prefs.IncomingCall = false
	require.NoError(t, w.f.svc.SavePushPreferences(ctx, w.peer, prefs))
	id := w.start(t, "video")
	assert.Empty(t, w.pushed(sender), "incoming calls switched off")
	_, err := w.f.svc.CancelCall(ctx, id, w.f.userID, userNode)
	require.NoError(t, err)
	assert.Empty(t, w.pushed(sender), "nor the missed call")

	prefs = store.DefaultPushPreferences
	prefs.QuietEnabled, prefs.TimeZone = true, "UTC"
	now := time.Now().UTC()
	prefs.QuietStart, prefs.QuietEnd = now.Hour()*60, (now.Hour()*60+120)%1440
	require.NoError(t, w.f.svc.SavePushPreferences(ctx, w.peer, prefs))
	id = w.start(t, "video")
	assert.Empty(t, w.pushed(sender), "quiet hours")
	_, err = w.f.svc.CancelCall(ctx, id, w.f.userID, userNode)
	require.NoError(t, err)

	require.NoError(t, w.f.svc.SavePushPreferences(ctx, w.peer, store.DefaultPushPreferences))
	view[w.peer+"/"+w.chID] = true
	w.start(t, "video")
	assert.Empty(t, w.pushed(sender), "the conversation is open: the app rings")

	g := newCallWorld(t)
	gs, _ := g.withPush()
	g.start(t, "video")
	assert.Empty(t, g.pushed(gs), "a group call rings nobody")
}

func TestInviteToCall_PushesWritersOnly(t *testing.T) {
	w := newCallWorld(t)
	ctx := context.Background()
	sender, _ := w.withPush()
	id := w.start(t, "video")

	// Deny: someone without write in the conversation invites nobody.
	err := w.f.svc.InviteToCall(ctx, id, w.reader, w.readerNode, []string{w.peer})
	require.ErrorIs(t, err, domain.ErrAccessDenied)
	assert.Empty(t, w.pushed(sender))

	require.NoError(t, w.f.svc.InviteToCall(ctx, id, w.f.userID, userNode, []string{w.peer, w.reader, "nobody"}))
	sent := w.pushed(sender)
	require.Len(t, sent, 1, "the reader cannot join, an unknown id is nobody")
	assert.Equal(t, []string{w.peer}, sent[0].ExternalIDs)
	assert.Equal(t, "testuser mời bạn vào cuộc gọi", sent[0].Body)
	assert.Equal(t, "/calls/"+id, sent[0].URL)
	assert.Equal(t, []push.Button{{ID: "join", Text: "Tham gia", URL: "/calls/" + id}}, sent[0].Buttons)

	var invite *domain.CallChange
	for i := range w.calls.changes {
		if w.calls.changes[i].Type == domain.CallChangeInvite {
			invite = &w.calls.changes[i]
		}
	}
	require.NotNil(t, invite, "the invitee's open app is told too")
	assert.Equal(t, []string{w.peer}, invite.Users)
}

// A person @mentioned in the first two minutes of a group call is invited to
// it; later the mention is an ordinary one.
func TestCallPush_AMentionEarlyInAGroupCallInvites(t *testing.T) {
	w := newCallWorld(t)
	ctx := context.Background()
	sender, _ := w.withPush()
	domain.SetPushWindowForTest(w.f.svc, time.Millisecond)
	id := w.start(t, "video")

	msg, err := w.f.svc.SendMessage(ctx, domain.SendMessageInput{
		ChannelID: w.chID, SenderID: w.f.userID, SenderNodeID: userNode, Content: "vào gọi nhé", Mentions: []string{w.peer},
	})
	require.NoError(t, err)
	w.f.svc.PushForMessage(ctx, msg)
	sent := w.pushed(sender)
	require.Len(t, sent, 1)
	assert.Equal(t, "testuser mời bạn vào cuộc gọi", sent[0].Body)
	assert.Equal(t, "/calls/"+id, sent[0].URL)

	_, err = w.f.pool.Exec(ctx, `UPDATE calls SET created_at = NOW() - INTERVAL '3 minutes' WHERE id = $1`, id)
	require.NoError(t, err)
	time.Sleep(20 * time.Millisecond) // the first push's gathering window closes
	msg, err = w.f.svc.SendMessage(ctx, domain.SendMessageInput{
		ChannelID: w.chID, SenderID: w.f.userID, SenderNodeID: userNode, Content: "còn ai không", Mentions: []string{w.peer},
	})
	require.NoError(t, err)
	w.f.svc.PushForMessage(ctx, msg)
	require.Eventually(t, func() bool { return len(w.pushed(sender)) >= 2 }, 2*time.Second, 10*time.Millisecond)
	assert.Contains(t, w.pushed(sender)[1].Body, "còn ai không", "past two minutes a mention is a mention")
}
