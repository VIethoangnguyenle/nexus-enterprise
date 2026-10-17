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

func TestCalendarPush_InvitationAndReminderReachTheDevices(t *testing.T) {
	w := newCallWorld(t)
	w.asDM(t)
	sender, _ := w.withPush()
	ctx := context.Background()
	prefs := store.DefaultPushPreferences
	prefs.TimeZone = "Asia/Ho_Chi_Minh"
	require.NoError(t, w.f.svc.SavePushPreferences(ctx, w.peer, prefs))
	start := time.Date(2026, 11, 2, 2, 0, 0, 0, time.UTC) // 09:00 in Ho Chi Minh

	w.f.svc.PushCalendar(ctx, w.peer, domain.CalendarPush{Kind: domain.CalendarPushInvitation, EventID: "e1", Title: "Standup", ActorName: "Alice", Start: start})
	w.f.svc.PushCalendar(ctx, w.peer, domain.CalendarPush{Kind: domain.CalendarPushReminder, EventID: "e1", Title: "Standup", Start: start, MinutesBefore: 10})
	sent := w.pushed(sender)
	require.Len(t, sent, 2)
	// They are sent in the background, so in no fixed order.
	byBody := map[string]push.Message{}
	for _, m := range sent {
		assert.Equal(t, []string{w.peer}, m.ExternalIDs)
		assert.Equal(t, "Standup", m.Title)
		assert.Equal(t, "/calendar?mode=week&date=2026-11-02", m.URL)
		byBody[m.Body] = m
	}
	require.Contains(t, byBody, "Alice mời bạn tới sự kiện này")
	require.Contains(t, byBody, "Bắt đầu lúc 09:00", "in the person's own zone")
	assert.NotEqual(t, byBody["Alice mời bạn tới sự kiện này"].CollapseID, byBody["Bắt đầu lúc 09:00"].CollapseID,
		"a reminder does not replace the invitation")
}

// Deny: nothing is pushed to someone who turned their pushes off or is in quiet
// hours, nor for an event or a person that is not named.
func TestCalendarPush_Deny(t *testing.T) {
	w := newCallWorld(t)
	w.asDM(t)
	sender, _ := w.withPush()
	ctx := context.Background()
	p := domain.CalendarPush{Kind: domain.CalendarPushReminder, EventID: "e1", Title: "Standup", Start: time.Now()}

	prefs := store.DefaultPushPreferences
	prefs.Direct = false
	require.NoError(t, w.f.svc.SavePushPreferences(ctx, w.peer, prefs))
	w.f.svc.PushCalendar(ctx, w.peer, p)
	assert.Empty(t, w.pushed(sender), "pushes addressed to the person are off")

	prefs = store.DefaultPushPreferences
	prefs.QuietEnabled, prefs.TimeZone = true, "UTC"
	now := time.Now().UTC()
	prefs.QuietStart, prefs.QuietEnd = now.Hour()*60, (now.Hour()*60+120)%1440
	require.NoError(t, w.f.svc.SavePushPreferences(ctx, w.peer, prefs))
	w.f.svc.PushCalendar(ctx, w.peer, p)
	assert.Empty(t, w.pushed(sender), "quiet hours")

	require.NoError(t, w.f.svc.SavePushPreferences(ctx, w.peer, store.DefaultPushPreferences))
	w.f.svc.PushCalendar(ctx, "", p)
	w.f.svc.PushCalendar(ctx, w.peer, domain.CalendarPush{Kind: domain.CalendarPushReminder})
	assert.Empty(t, w.pushed(sender))
}
