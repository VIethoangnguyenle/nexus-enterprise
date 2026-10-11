package domain_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/ngac"
	"ngac-platform/pkg/push"
	"ngac-platform/services/messaging/internal/domain"
	"ngac-platform/services/messaging/internal/store"
	"ngac-platform/testutil"
)

type recordingSender struct {
	mu   sync.Mutex
	sent []push.Message
}

func (r *recordingSender) Send(_ context.Context, m push.Message) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sent = append(r.sent, m)
	return nil
}
func (r *recordingSender) Enabled() bool { return true }
func (r *recordingSender) all() []push.Message {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]push.Message(nil), r.sent...)
}

type viewing map[string]bool

func (v viewing) IsViewing(userID, channelID string) bool { return v[userID+"/"+channelID] }

// pushWorld is a channel with a sender (the fixture's caller) and one other
// member who reads it.
type pushWorld struct {
	*talk
	other, otherNode string
	sender           *recordingSender
	view             viewing
}

func newPushWorld(t *testing.T, dm bool) *pushWorld {
	t.Helper()
	k := newTalk(t, "push")
	ctx := context.Background()
	other, otherNode := testutil.CreateUser(t, k.f.pool)
	k.f.read.allow[grant{otherNode, k.oaID, ngac.OpRead}] = true
	if dm {
		_, err := k.f.pool.Exec(ctx, `UPDATE channels SET channel_type = 'dm' WHERE id = $1`, k.chID)
		require.NoError(t, err)
		for _, n := range []string{"u-" + k.f.userID, otherNode} {
			_, err = k.f.pool.Exec(ctx, `INSERT INTO channel_members (channel_id, ngac_node_id) VALUES ($1, $2)`, k.chID, n)
			require.NoError(t, err)
		}
		t.Cleanup(func() { k.f.pool.Exec(ctx, `DELETE FROM channel_members WHERE channel_id = $1`, k.chID) })
	}
	w := &pushWorld{talk: k, other: other, otherNode: otherNode, sender: &recordingSender{}, view: viewing{}}
	k.f.svc.SetPush(w.sender, w.view)
	return w
}

func (w *pushWorld) send(t *testing.T, content string, mentions ...string) {
	t.Helper()
	msg, err := w.f.svc.SendMessage(context.Background(), domain.SendMessageInput{
		ChannelID: w.chID, SenderID: w.f.userID, SenderNodeID: userNode, Content: content, Mentions: mentions,
	})
	require.NoError(t, err)
	w.f.svc.PushForMessage(context.Background(), msg)
}

func TestPush_DirectMessageReachesTheOtherSideOnly(t *testing.T) {
	w := newPushWorld(t, true)
	w.send(t, "chào bạn")
	sent := w.sender.all()
	require.Len(t, sent, 1)
	assert.Equal(t, []string{w.other}, sent[0].ExternalIDs, "never the sender")
	assert.Equal(t, "testuser", sent[0].Title, "a direct message is titled by its sender")
	assert.Equal(t, "chào bạn", sent[0].Body)
	assert.Equal(t, "/channels/"+w.chID, sent[0].URL)
}

func TestPush_MentionAndTheDenyCases(t *testing.T) {
	w := newPushWorld(t, false)
	ctx := context.Background()

	w.send(t, "hello", w.other)
	require.Len(t, w.sender.all(), 1, "a mention in a group is pushed")

	// Deny: mentioned but cannot read the channel (removed, or never a member).
	stranger, _ := testutil.CreateUser(t, w.f.pool)
	w.send(t, "hello", stranger)
	assert.Len(t, w.sender.all(), 1, "no read on the channel, no push")

	// A plain group message concerns nobody in particular.
	w.send(t, "không nhắc ai")
	assert.Len(t, w.sender.all(), 1)

	// Deny: mentions switched off.
	prefs := store.DefaultPushPreferences
	prefs.Mention = false
	require.NoError(t, w.f.svc.SavePushPreferences(ctx, w.other, prefs))
	w.send(t, "hello again", w.other)
	assert.Len(t, w.sender.all(), 1, "the kind is off")
}

func TestPush_NotWhileViewingNorInQuietHours(t *testing.T) {
	w := newPushWorld(t, true)
	ctx := context.Background()

	w.view[w.other+"/"+w.chID] = true
	w.send(t, "đang xem")
	assert.Empty(t, w.sender.all(), "looking at the conversation: no push")
	delete(w.view, w.other+"/"+w.chID)

	prefs := store.DefaultPushPreferences
	prefs.QuietEnabled = true
	prefs.TimeZone = "UTC"
	now := time.Now().UTC()
	prefs.QuietStart = now.Hour() * 60
	prefs.QuietEnd = (now.Hour()*60 + 120) % 1440
	require.NoError(t, w.f.svc.SavePushPreferences(ctx, w.other, prefs))
	w.send(t, "đêm khuya")
	assert.Empty(t, w.sender.all(), "quiet hours: no push")

	prefs.QuietEnabled = false
	require.NoError(t, w.f.svc.SavePushPreferences(ctx, w.other, prefs))
	w.send(t, "sáng rồi")
	assert.Len(t, w.sender.all(), 1)
}

// Messages arriving close together become one follow-up that replaces the
// first notification on the device.
func TestPush_GathersMessagesOfOneConversation(t *testing.T) {
	w := newPushWorld(t, true)
	domain.SetPushWindowForTest(w.f.svc, 200*time.Millisecond)
	w.send(t, "một")
	w.send(t, "hai")
	w.send(t, "ba")
	require.Len(t, w.sender.all(), 1, "the first goes at once")

	require.Eventually(t, func() bool { return len(w.sender.all()) == 2 }, 3*time.Second, 20*time.Millisecond)
	sent := w.sender.all()
	assert.Equal(t, "3 tin mới", sent[1].Body)
	assert.Equal(t, sent[0].CollapseID, sent[1].CollapseID, "it replaces the earlier one")
}

// A reply reaches the thread's followers, not someone who unfollowed it.
func TestPush_AReplyReachesTheThreadsFollowers(t *testing.T) {
	k := newThreadTalk(t) // root by the caller, a reply by the replier
	ctx := context.Background()
	sender := &recordingSender{}
	k.f.svc.SetPush(sender, viewing{})

	reply, err := k.f.svc.SendMessage(ctx, domain.SendMessageInput{
		ChannelID: k.chID, SenderID: k.replier, SenderNodeID: replierNode, Content: "thêm ý", ParentMessageID: k.rootID,
	})
	require.NoError(t, err)
	k.f.svc.PushForMessage(ctx, reply)
	sent := sender.all()
	require.Len(t, sent, 1)
	assert.Equal(t, []string{k.f.userID}, sent[0].ExternalIDs, "the starter follows the thread")
	assert.Equal(t, "/channels/"+k.chID+"?thread="+k.rootID, sent[0].URL)

	_, err = k.f.svc.UnfollowThread(ctx, k.rootID, k.f.userID, userNode)
	require.NoError(t, err)
	domain.SetPushWindowForTest(k.f.svc, time.Millisecond)
	time.Sleep(20 * time.Millisecond) // let the first window close
	again, err := k.f.svc.SendMessage(ctx, domain.SendMessageInput{
		ChannelID: k.chID, SenderID: k.replier, SenderNodeID: replierNode, Content: "nữa", ParentMessageID: k.rootID,
	})
	require.NoError(t, err)
	k.f.svc.PushForMessage(ctx, again)
	assert.Len(t, sender.all(), 1, "unfollowed: no push")
}

func TestSavePushPreferences_RejectsNonsense(t *testing.T) {
	w := newPushWorld(t, false)
	ctx := context.Background()
	p := store.DefaultPushPreferences
	p.QuietStart = 2000
	require.ErrorIs(t, w.f.svc.SavePushPreferences(ctx, w.other, p), domain.ErrInvalidInput)
	p = store.DefaultPushPreferences
	p.TimeZone = "Mars/Olympus"
	require.ErrorIs(t, w.f.svc.SavePushPreferences(ctx, w.other, p), domain.ErrInvalidInput)
	require.ErrorIs(t, w.f.svc.SavePushPreferences(ctx, "", store.DefaultPushPreferences), domain.ErrAccessDenied)
}

// hangingSender never answers: it returns only when its context ends.
type hangingSender struct{ calls atomic.Int32 }

func (h *hangingSender) Send(ctx context.Context, _ push.Message) error {
	h.calls.Add(1)
	<-ctx.Done()
	return ctx.Err()
}
func (h *hangingSender) Enabled() bool { return true }

// The caller's deadline bounds the whole delivery: an unreachable OneSignal
// must not keep a goroutine per message alive for minutes.
func TestPush_DeliveryEndsWithTheCallersDeadline(t *testing.T) {
	w := newPushWorld(t, true)
	hang := &hangingSender{}
	w.f.svc.SetPush(hang, w.view)
	msg, err := w.f.svc.SendMessage(context.Background(), domain.SendMessageInput{
		ChannelID: w.chID, SenderID: w.f.userID, SenderNodeID: userNode, Content: "alo",
	})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	start := time.Now()
	w.f.svc.PushForMessage(ctx, msg)
	assert.Less(t, time.Since(start), 2*time.Second, "delivery stops at the caller's deadline")
	assert.EqualValues(t, 1, hang.calls.Load())
}

// A message pushes to at most MaxMentionPushes mentioned people, however many
// the client names: a writer cannot page a whole channel through mentions.
func TestPush_MentionsAreBounded(t *testing.T) {
	w := newPushWorld(t, false)
	var mentions []string
	for i := 0; i < domain.MaxMentionPushes+5; i++ {
		id, node := testutil.CreateUser(t, w.f.pool)
		w.f.read.allow[grant{node, w.oaID, ngac.OpRead}] = true
		mentions = append(mentions, id)
	}
	w.send(t, "mọi người ơi", mentions...)
	assert.Len(t, w.sender.all(), domain.MaxMentionPushes)
}

// The notification level shapes push: "all" pushes every message of a group,
// "off" silences it except when the person is mentioned, and a direct message
// is only on or off.
func TestPush_FollowsTheNotificationLevel(t *testing.T) {
	ctx := context.Background()

	t.Run("all: every message of the group", func(t *testing.T) {
		w := newPushWorld(t, false)
		_, err := w.f.pool.Exec(ctx, `INSERT INTO channel_members (channel_id, ngac_node_id) VALUES ($1, $2)`, w.chID, w.otherNode)
		require.NoError(t, err)
		t.Cleanup(func() { w.f.pool.Exec(ctx, `DELETE FROM channel_members WHERE channel_id = $1`, w.chID) })

		w.send(t, "tin thường")
		assert.Empty(t, w.sender.all(), "by default a plain group message pushes nobody")

		require.NoError(t, w.f.svc.SetNotificationLevel(ctx, w.chID, w.other, w.otherNode, domain.NotifyAll))
		domain.SetPushWindowForTest(w.f.svc, time.Millisecond)
		w.send(t, "tin thường nữa")
		sent := w.sender.all()
		require.Len(t, sent, 1)
		assert.Equal(t, []string{w.other}, sent[0].ExternalIDs)
	})

	t.Run("off: only a mention gets through", func(t *testing.T) {
		w := newPushWorld(t, false)
		require.NoError(t, w.f.svc.SetNotificationLevel(ctx, w.chID, w.other, w.otherNode, domain.NotifyOff))
		w.send(t, "@ bạn", w.other)
		assert.Len(t, w.sender.all(), 1, "a mention still notifies when the group is off")
	})

	t.Run("off: a followed thread is silent", func(t *testing.T) {
		k := newThreadTalk(t)
		sender := &recordingSender{}
		k.f.svc.SetPush(sender, viewing{})
		require.NoError(t, k.f.svc.SetNotificationLevel(ctx, k.chID, k.f.userID, userNode, domain.NotifyOff))
		reply, err := k.f.svc.SendMessage(ctx, domain.SendMessageInput{
			ChannelID: k.chID, SenderID: k.replier, SenderNodeID: replierNode, Content: "thêm", ParentMessageID: k.rootID,
		})
		require.NoError(t, err)
		k.f.svc.PushForMessage(ctx, reply)
		assert.Empty(t, sender.all())
	})

	t.Run("a direct message is on or off", func(t *testing.T) {
		w := newPushWorld(t, true)
		require.ErrorIs(t, w.f.svc.SetNotificationLevel(ctx, w.chID, w.other, w.otherNode, domain.NotifyAll), domain.ErrInvalidInput)
		require.NoError(t, w.f.svc.SetNotificationLevel(ctx, w.chID, w.other, w.otherNode, domain.NotifyOff))
		w.send(t, "chào", w.other)
		assert.Empty(t, w.sender.all(), "a muted direct message is silent, mention or not")
	})
}
