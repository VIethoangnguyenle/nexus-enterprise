package domain_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pb "ngac-platform/proto/messaging"
	"ngac-platform/services/messaging/internal/domain"
)

func unreadOf(t *testing.T, k *threadTalk) *pb.ChannelUnread {
	t.Helper()
	counts, err := k.f.svc.GetUnreadCounts(context.Background(), k.f.userID, userNode, k.f.wsID)
	require.NoError(t, err)
	for _, c := range counts {
		if c.ChannelId == k.chID {
			return c
		}
	}
	t.Fatalf("channel %s not counted", k.chID)
	return nil
}

// Each person chooses, per conversation, how much of it counts: everything,
// the main messages and the threads they follow (the default), or nothing.
func TestNotificationLevel_DecidesWhatCounts(t *testing.T) {
	k := newThreadTalk(t) // a root by the caller, one reply by someone else
	ctx := context.Background()
	svc := k.f.svc

	level, err := svc.GetNotificationLevel(ctx, k.chID, k.f.userID, userNode)
	require.NoError(t, err)
	assert.Equal(t, domain.NotifyMain, level, "the default")
	def := unreadOf(t, k)
	assert.Equal(t, domain.NotifyMain, def.NotificationLevel)
	assert.EqualValues(t, 1, def.ThreadUnreadCount, "the starter follows the thread")

	// Not following the thread: by default its replies no longer count...
	_, err = svc.UnfollowThread(ctx, k.rootID, k.f.userID, userNode)
	require.NoError(t, err)
	assert.EqualValues(t, 0, unreadOf(t, k).ThreadUnreadCount)

	// ...but with "all" every reply in the conversation does.
	require.NoError(t, svc.SetNotificationLevel(ctx, k.chID, k.f.userID, userNode, domain.NotifyAll))
	all := unreadOf(t, k)
	assert.Equal(t, domain.NotifyAll, all.NotificationLevel)
	assert.EqualValues(t, 1, all.ThreadUnreadCount)

	// "off": nothing counts, and the screen is told so it can show it muted.
	require.NoError(t, svc.SetNotificationLevel(ctx, k.chID, k.f.userID, userNode, domain.NotifyOff))
	off := unreadOf(t, k)
	assert.Equal(t, domain.NotifyOff, off.NotificationLevel)
	assert.Zero(t, off.UnreadCount)
	assert.Zero(t, off.ThreadUnreadCount)

	level, err = svc.GetNotificationLevel(ctx, k.chID, k.f.userID, userNode)
	require.NoError(t, err)
	assert.Equal(t, domain.NotifyOff, level)
}

func TestNotificationLevel_Deny(t *testing.T) {
	k := newThreadTalk(t)
	ctx := context.Background()
	svc := k.f.svc

	// Deny: no read on the conversation, no level — neither read nor set.
	_, err := svc.GetNotificationLevel(ctx, k.chID, "someone", outsider)
	require.ErrorIs(t, err, domain.ErrAccessDenied)
	require.ErrorIs(t, svc.SetNotificationLevel(ctx, k.chID, "someone", outsider, domain.NotifyOff), domain.ErrAccessDenied)

	// A level nobody defined is refused.
	for _, bad := range []string{"", "loud", "OFF"} {
		require.ErrorIs(t, svc.SetNotificationLevel(ctx, k.chID, k.f.userID, userNode, bad), domain.ErrInvalidInput, "level %q", bad)
	}
	require.ErrorIs(t, svc.SetNotificationLevel(ctx, k.chID, "", userNode, domain.NotifyOff), domain.ErrInvalidInput)
}
