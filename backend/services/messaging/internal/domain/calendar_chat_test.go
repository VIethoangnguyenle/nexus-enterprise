package domain_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/ngac"
	"ngac-platform/services/messaging/internal/domain"
)

func (f *fixture) calendarPost(chID string, kind string) domain.CalendarChatPost {
	return domain.CalendarChatPost{
		Kind: kind, WorkspaceID: f.wsID, ChannelID: chID, EventID: "ev-1", Title: "Họp tuần",
		ActorUserID: f.userID, ActorNodeID: userNode, ActorName: "Lan", Zone: "Asia/Ho_Chi_Minh",
		Start: time.Date(2026, 11, 2, 3, 0, 0, 0, time.UTC), Stamp: 1000,
	}
}

func TestCalendarChat_ACardIsPostedOnceFromTheCreator(t *testing.T) {
	f := newFixture(t)
	chID, oaID := f.insertChannel(t, "calendar-chat")
	f.read.allow[grant{userNode, oaID, ngac.OpRead}] = true
	ctx := context.Background()

	msg, err := f.svc.PostCalendarChat(ctx, f.calendarPost(chID, domain.CalendarChatCard))
	require.NoError(t, err)
	require.NotNil(t, msg)
	assert.Equal(t, "calendar_event", msg.LinkedEntityType)
	assert.Equal(t, "ev-1", msg.LinkedEntityId)
	assert.Equal(t, "system", msg.MessageType)
	assert.Equal(t, f.userID, msg.SenderId)
	assert.Equal(t, chID, msg.ChannelId)

	// The topic delivers at least once: the same notice again stores nothing.
	again, err := f.svc.PostCalendarChat(ctx, f.calendarPost(chID, domain.CalendarChatCard))
	require.NoError(t, err)
	assert.Nil(t, again)
	var n int
	require.NoError(t, f.pool.QueryRow(ctx, `SELECT count(*) FROM messages WHERE channel_id = $1`, chID).Scan(&n))
	assert.Equal(t, 1, n)
}

func TestCalendarChat_AMoveAndACancellationAreShortLines(t *testing.T) {
	f := newFixture(t)
	chID, oaID := f.insertChannel(t, "calendar-lines")
	f.read.allow[grant{userNode, oaID, ngac.OpRead}] = true
	ctx := context.Background()

	moved, err := f.svc.PostCalendarChat(ctx, f.calendarPost(chID, domain.CalendarChatMoved))
	require.NoError(t, err)
	require.NotNil(t, moved)
	assert.Equal(t, "Lan đã dời “Họp tuần” sang 10:00 02/11", moved.Content, "worded in the event's own zone")
	assert.Empty(t, moved.LinkedEntityType, "a line, not a card")

	// A second move of the same event is a new line; the same notice again is not.
	next := f.calendarPost(chID, domain.CalendarChatMoved)
	next.Stamp = 2000
	second, err := f.svc.PostCalendarChat(ctx, next)
	require.NoError(t, err)
	assert.NotNil(t, second)
	dup, err := f.svc.PostCalendarChat(ctx, next)
	require.NoError(t, err)
	assert.Nil(t, dup)

	gone, err := f.svc.PostCalendarChat(ctx, f.calendarPost(chID, domain.CalendarChatCancelled))
	require.NoError(t, err)
	assert.Equal(t, "Lan đã huỷ “Họp tuần”", gone.Content)

	occ := f.calendarPost(chID, domain.CalendarChatCancelled)
	occ.Occurrence = time.Date(2026, 11, 9, 2, 0, 0, 0, time.UTC)
	one, err := f.svc.PostCalendarChat(ctx, occ)
	require.NoError(t, err)
	assert.Equal(t, "Lan đã huỷ buổi 09:00 09/11 của “Họp tuần”", one.Content)
}

func TestCalendarChat_RefusesWhereItMustNotPost(t *testing.T) {
	f := newFixture(t)
	chID, oaID := f.insertChannel(t, "calendar-deny")
	ctx := context.Background()

	// The actor no longer reads the group: nothing is posted in their name.
	_, err := f.svc.PostCalendarChat(ctx, f.calendarPost(chID, domain.CalendarChatCard))
	require.ErrorIs(t, err, domain.ErrAccessDenied)

	f.read.allow[grant{userNode, oaID, ngac.OpRead}] = true
	// A notice that names another workspace's group is not trusted.
	other := f.calendarPost(chID, domain.CalendarChatCard)
	other.WorkspaceID = "another-workspace"
	_, err = f.svc.PostCalendarChat(ctx, other)
	require.ErrorIs(t, err, domain.ErrInvalidInput)

	// A conversation that does not exist, and a line of an unknown kind.
	_, err = f.svc.PostCalendarChat(ctx, f.calendarPost("no-such-channel", domain.CalendarChatCard))
	require.ErrorIs(t, err, domain.ErrNotFound)
	_, err = f.svc.PostCalendarChat(ctx, f.calendarPost(chID, "other"))
	require.ErrorIs(t, err, domain.ErrInvalidInput)

	var n int
	require.NoError(t, f.pool.QueryRow(ctx, `SELECT count(*) FROM messages WHERE channel_id = $1`, chID).Scan(&n))
	assert.Zero(t, n)
}
