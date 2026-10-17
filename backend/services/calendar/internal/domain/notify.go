package domain

import (
	"context"
	"log/slog"
	"time"

	"ngac-platform/pkg/grpcauth"
	"ngac-platform/services/calendar/internal/events"
	"ngac-platform/services/calendar/internal/store"
)

// SetNotices turns on notices to the people an event concerns. Without it the
// service changes events and announces them to open screens only.
func (s *Service) SetNotices(p events.Publisher) { s.notices = p }

// noticeExtras are the parts of a notice that only some kinds carry.
type noticeExtras struct {
	// occurrence is the occurrence of a repeating event the notice is about.
	occurrence time.Time
	// start and end are where the occurrence now is, when it is not where it was.
	start, end time.Time
	response   string
	comment    string
	minutes    *int
	// actor stands for whoever caused it; zero for a reminder.
	actor grpcauth.Caller
	// announce makes the notice one for the group's chat, not for people: it is
	// sent with no recipients, and only for a group calendar's event that is not private.
	announce bool
	// toActor keeps the actor among the recipients (a reminder is for them too).
	toActor bool
}

// oaOf is the node a recipient must still read: the event's own, else its
// calendar's.
func oaOf(cal *store.Calendar, e *store.Event) string {
	if e != nil && e.NgacOAID != "" {
		return e.NgacOAID
	}
	if cal != nil {
		return cal.NgacOAID
	}
	return ""
}

// notify tells people about an event after the change is committed. The actor is
// never told about their own doing, a person with no account is skipped, and a
// notice nobody is for is not sent (except the announcement of an event on a
// group's calendar, which has no audience by design).
func (s *Service) notify(ctx context.Context, typ string, ws *store.Workspace, cal *store.Calendar, e *store.Event, nodes []string, x noticeExtras) {
	if s.notices == nil {
		return
	}
	if typ == events.TypeCreated {
		x.announce = true
	}
	if x.announce && (cal.Kind != store.KindChannel || e == nil || e.Visibility == VisibilityPrivate) {
		return
	}
	var recipients []events.Recipient
	if nodes = distinct(nodes...); len(nodes) > 0 {
		people, err := s.store.People(ctx, ws.ID, nodes)
		if err != nil {
			slog.Warn("could not name the people to notify", "type", typ, "event", eventID(e), "error", err)
			return
		}
		for _, n := range nodes {
			if p := people[n]; p != nil && p.UserID != "" && (x.toActor || n != x.actor.NGACNodeID) {
				recipients = append(recipients, events.Recipient{NodeID: n, UserID: p.UserID})
			}
		}
	}
	if len(recipients) == 0 && !x.announce {
		return
	}
	n := events.Notice{
		Type: typ, TenantID: ws.ID, WorkspaceID: ws.ID, CalendarID: cal.ID, OAID: oaOf(cal, e),
		ActorNodeID: x.actor.NGACNodeID, ActorUserID: x.actor.UserID, Recipients: recipients,
		Response: x.response, Comment: x.comment, MinutesBefore: x.minutes, Announce: x.announce,
	}
	if cal.Kind == store.KindChannel {
		n.ChannelID = cal.ChannelID
	}
	if e != nil {
		n.EventID, n.Title, n.AllDay, n.Tzid = e.ID, e.Title, e.AllDay, e.TZID
		n.Start, n.End = e.StartsAt.UTC().Format(time.RFC3339), e.EndsAt.UTC().Format(time.RFC3339)
	}
	if !x.occurrence.IsZero() && e != nil {
		// The notice is about this occurrence: its times, not the series'.
		n.OccurrenceStart = x.occurrence.UTC().Format(time.RFC3339)
		n.Start = n.OccurrenceStart
		n.End = x.occurrence.Add(e.EndsAt.Sub(e.StartsAt)).UTC().Format(time.RFC3339)
	}
	if !x.start.IsZero() {
		n.Start, n.End = x.start.UTC().Format(time.RFC3339), x.end.UTC().Format(time.RFC3339)
	}
	s.notices.Publish(n)
}

func eventID(e *store.Event) string {
	if e == nil {
		return ""
	}
	return e.ID
}

// movedOrReplaced reports whether a change moved an event in time or place: the
// time or the place of what the guests were told differs from what they now are.
// For one occurrence (or the occurrences from one on) the baseline is where that
// occurrence was in its series; for a whole event it is the event before.
func movedOrReplaced(before store.Event, scope string, in instance) bool {
	startWas, endWas := before.StartsAt, before.EndsAt
	startNow, endNow, place := in.Master.StartsAt, in.Master.EndsAt, in.Master.Location
	if before.RRule != "" && scope != ScopeAll {
		startWas, endWas = in.Original, in.Original.Add(before.EndsAt.Sub(before.StartsAt))
		startNow, endNow, place = in.Start, in.End, in.Row.Location
	}
	return !startNow.Equal(startWas) || !endNow.Equal(endWas) || place != before.Location
}

// movedInTime reports whether a change moved an event or one of its occurrences
// in time (not merely in place): what a group's chat is told.
func movedInTime(before store.Event, scope string, in instance) bool {
	startWas, endWas := before.StartsAt, before.EndsAt
	startNow, endNow := in.Master.StartsAt, in.Master.EndsAt
	if before.RRule != "" && scope != ScopeAll {
		startWas, endWas = in.Original, in.Original.Add(before.EndsAt.Sub(before.StartsAt))
		startNow, endNow = in.Start, in.End
	}
	return !startNow.Equal(startWas) || !endNow.Equal(endWas)
}
