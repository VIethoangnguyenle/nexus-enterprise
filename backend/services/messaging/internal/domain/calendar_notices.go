// Package domain — what calendar events ask of the messaging service: whether a
// person may still read an event, a name for the person who acted, and the web
// push for an invitation or a reminder.
package domain

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"ngac-platform/ngac"
	"ngac-platform/pkg/push"
)

// CalendarPushKind is why a calendar event is pushed.
type CalendarPushKind string

const (
	CalendarPushInvitation CalendarPushKind = "invitation"
	CalendarPushReminder   CalendarPushKind = "reminder"
)

// CalendarPush is an event to push to one person's devices.
type CalendarPush struct {
	Kind      CalendarPushKind
	EventID   string
	Title     string
	ActorName string
	Start     time.Time
	AllDay    bool
	// MinutesBefore is how long before the start a reminder fell due.
	MinutesBefore int
}

// CanRead reports whether the person's user node holds read on a node. A missing
// node or a policy error is a refusal.
func (s *Service) CanRead(ctx context.Context, userNodeID, nodeID string) bool {
	if userNodeID == "" || nodeID == "" {
		return false
	}
	return s.checkAccess(ctx, userNodeID, nodeID, ngac.OpRead) == nil
}

// UserDisplayName is the name an account is shown by, "" when unknown.
func (s *Service) UserDisplayName(ctx context.Context, userID string) string {
	if userID == "" {
		return ""
	}
	name, err := s.store.DisplayName(ctx, userID)
	if err != nil {
		slog.Warn("could not name a person for a calendar notification", "error", err)
		return ""
	}
	return name
}

// PushCalendar sends a calendar push to a person's devices. It does nothing when
// push is off, when the person turned off pushes addressed to them (the same
// switch as direct messages) or while it is their quiet hours. It returns at
// once; the push is sent in the background.
func (s *Service) PushCalendar(ctx context.Context, userID string, p CalendarPush) {
	if s.push == nil || userID == "" || p.EventID == "" {
		return
	}
	s.later(ctx, func(ctx context.Context) {
		prefs, err := s.store.GetPushPreferences(ctx, userID)
		if err != nil || !prefs.Direct || inQuietHours(prefs, s.pushNow()) {
			return
		}
		s.send(ctx, calendarPushMessage(userID, p, prefs.TimeZone))
	})
}

// calendarPushMessage words the push in the person's own time zone.
func calendarPushMessage(userID string, p CalendarPush, zone string) push.Message {
	loc, err := time.LoadLocation(zone)
	if err != nil {
		loc = time.UTC
	}
	local := p.Start.In(loc)
	m := push.Message{
		ExternalIDs: []string{userID}, CollapseID: "calendar-" + string(p.Kind) + "-" + p.EventID, Title: p.Title,
		URL: "/calendar?mode=week&date=" + local.Format("2006-01-02"),
	}
	switch {
	case p.Kind == CalendarPushInvitation && p.ActorName != "":
		m.Body = fmt.Sprintf("%s mời bạn tới sự kiện này", p.ActorName)
	case p.Kind == CalendarPushInvitation:
		m.Body = "Bạn được mời tới sự kiện này"
	case p.AllDay:
		m.Body = "Diễn ra cả ngày"
	default:
		m.Body = "Bắt đầu lúc " + local.Format("15:04")
	}
	return m
}
