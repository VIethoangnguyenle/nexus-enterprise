package events

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	pb "ngac-platform/proto/messaging"
	"ngac-platform/services/messaging/internal/domain"
)

// CalendarTopic carries the calendar service's notices and, on the same topic,
// its realtime frames. A notice has "record": "notice"; the realtime consumer
// skips those and this one skips the frames.
const CalendarTopic = "calendar.events"

const calendarNoticeRecord = "notice"

// Calendar notice types, as the calendar service publishes them.
const (
	calendarInvited   = "event.invited"
	calendarUpdated   = "event.updated"
	calendarCancelled = "event.cancelled"
	calendarRsvp      = "event.rsvp"
	calendarReminder  = "event.reminder"
	calendarShared    = "calendar.shared"
	calendarCreated   = "event.created"
)

// Notification types stored for them.
const (
	NotifCalendarInvitation = "calendar_invitation"
	NotifCalendarUpdated    = "calendar_updated"
	NotifCalendarCancelled  = "calendar_cancelled"
	NotifCalendarRsvp       = "calendar_rsvp"
	NotifCalendarReminder   = "calendar_reminder"
	NotifCalendarShared     = "calendar_shared"
)

// Entity types a calendar notification links to.
const (
	entityCalendarEvent = "calendar_event"
	entityCalendar      = "calendar"
)

// reminderMaxAge is how late a reminder may arrive and still be shown: one that
// sat in a backlog past it is about something already under way.
const reminderMaxAge = 15 * time.Minute

// CalendarNotice matches the record the calendar service publishes.
type CalendarNotice struct {
	Record      string `json:"record"`
	Type        string `json:"type"`
	WorkspaceID string `json:"workspace_id"`
	EventID     string `json:"event_id"`
	CalendarID  string `json:"calendar_id"`
	// ChannelID is the group of a group calendar; Announce marks a notice for that
	// group's chat (it has no recipients).
	ChannelID   string `json:"channel_id"`
	Announce    bool   `json:"announce"`
	Tzid        string `json:"tzid"`
	ActorNodeID string `json:"actor_node_id"`
	OAID        string `json:"oa_id"`
	ActorUserID string `json:"actor_user_id"`
	Recipients  []struct {
		NodeID string `json:"node_id"`
		UserID string `json:"user_id"`
	} `json:"recipients"`
	Title           string `json:"title"`
	Start           string `json:"start"`
	End             string `json:"end"`
	AllDay          bool   `json:"all_day"`
	OccurrenceStart string `json:"occurrence_start"`
	Response        string `json:"response"`
	MinutesBefore   *int   `json:"minutes_before"`
	At              int64  `json:"at"`
}

// CalendarGate is what the notices need beyond storing a row: whether a person
// may still read the event, who the actor is, and the push to devices.
// *domain.Service implements it.
type CalendarGate interface {
	CanRead(ctx context.Context, userNodeID, nodeID string) bool
	UserDisplayName(ctx context.Context, userID string) string
	PushCalendar(ctx context.Context, userID string, p domain.CalendarPush)
	PostCalendarChat(ctx context.Context, p domain.CalendarChatPost) (*pb.Message, error)
}

// MessageAnnouncer sends a new message to the people who have the conversation
// open. The WebSocket hub implements it.
type MessageAnnouncer interface {
	BroadcastToChannel(channelID string, msg *pb.Message)
}

// SetCalendar wires the checks and the push. Without them calendar notices are
// dropped: nothing may be told to someone who has not been checked.
func (c *Consumer) SetCalendar(g CalendarGate) { c.calendar = g }

// CalendarBody is what a calendar notification's body holds: names and times for
// the client to write its sentence from. It is JSON, and the only identifier in it is
// the workspace the link opens; the screen never prints it.
type CalendarBody struct {
	// Workspace is the workspace the event is in, for the link that opens it; it is
	// never shown.
	Workspace  string `json:"ws,omitempty"`
	Actor      string `json:"actor,omitempty"`
	Start      string `json:"start,omitempty"`
	End        string `json:"end,omitempty"`
	AllDay     bool   `json:"allDay,omitempty"`
	Occurrence string `json:"occurrence,omitempty"`
	Response   string `json:"response,omitempty"`
}

// IsCalendarNotice reports whether a calendar.events record is a notice and not a
// realtime frame.
func IsCalendarNotice(value []byte) bool {
	var probe struct {
		Record string `json:"record"`
	}
	return json.Unmarshal(value, &probe) == nil && probe.Record == calendarNoticeRecord
}

// handleCalendarEvent stores one notification for each person a calendar notice
// concerns, and pushes an invitation or a reminder to their devices. A record
// that names no person or no event creates nothing, and nobody who cannot read
// the event at this moment is told (except of its cancellation, when the event is
// already gone). The actor is never told.
func (c *Consumer) handleCalendarEvent(ctx context.Context, data []byte) {
	if !IsCalendarNotice(data) {
		return
	}
	var n CalendarNotice
	if err := json.Unmarshal(data, &n); err != nil {
		slog.Warn("failed to unmarshal calendar notice", "error", err)
		return
	}
	if n.Announce {
		c.postCalendarChat(ctx, n)
		return
	}
	notifType, entityType, entityID, ok := calendarEntity(n)
	if !ok || len(n.Recipients) == 0 {
		return
	}
	if c.calendar == nil {
		slog.Warn("calendar notice dropped: no gate wired", "type", n.Type)
		return
	}
	if n.Type == calendarReminder && n.At > 0 && time.Since(time.UnixMilli(n.At)) > reminderMaxAge {
		return
	}

	body := CalendarBody{Workspace: n.WorkspaceID, Start: n.Start, End: n.End, AllDay: n.AllDay, Occurrence: n.OccurrenceStart, Response: n.Response}
	if n.ActorUserID != "" {
		body.Actor = c.calendar.UserDisplayName(ctx, n.ActorUserID)
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		slog.Error("calendar notification body not encoded", "type", n.Type, "error", err)
		return
	}
	title := n.Title
	if title == "" {
		title = "Lịch"
	}
	start, _ := time.Parse(time.RFC3339, n.Start)
	for _, r := range n.Recipients {
		if r.UserID == "" || (r.UserID == n.ActorUserID && n.Type != calendarReminder) {
			continue
		}
		if n.Type != calendarCancelled && !c.calendar.CanRead(ctx, r.NodeID, n.OAID) {
			continue
		}
		c.notify(ctx, r.UserID, notifType, title, string(encoded), entityType, entityID)
		switch n.Type {
		case calendarInvited:
			c.calendar.PushCalendar(ctx, r.UserID, domain.CalendarPush{
				Kind: domain.CalendarPushInvitation, EventID: n.EventID, Title: title, ActorName: body.Actor, Start: start, AllDay: n.AllDay})
		case calendarReminder:
			p := domain.CalendarPush{Kind: domain.CalendarPushReminder, EventID: n.EventID, Title: title, Start: start, AllDay: n.AllDay}
			if n.MinutesBefore != nil {
				p.MinutesBefore = *n.MinutesBefore
			}
			c.calendar.PushCalendar(ctx, r.UserID, p)
		}
	}
}

// calendarEntity maps a notice to the notification it makes and what that opens.
// ok is false for a notice that makes none: an unknown type, one that names no
// event (or, for a share, no calendar).
func calendarEntity(n CalendarNotice) (notifType, entityType, entityID string, ok bool) {
	switch n.Type {
	case calendarInvited:
		notifType = NotifCalendarInvitation
	case calendarUpdated:
		notifType = NotifCalendarUpdated
	case calendarCancelled:
		notifType = NotifCalendarCancelled
	case calendarRsvp:
		notifType = NotifCalendarRsvp
	case calendarReminder:
		notifType = NotifCalendarReminder
	case calendarShared:
		return NotifCalendarShared, entityCalendar, n.CalendarID, n.CalendarID != ""
	default:
		return "", "", "", false
	}
	return notifType, entityCalendarEvent, n.EventID, n.EventID != ""
}

// postCalendarChat writes a group calendar's announcement into the group's chat
// and tells the people who have it open. A notice posted before (the topic
// delivers at least once) makes no second message; one that cannot be posted is
// logged and dropped, as no retry would change the answer.
func (c *Consumer) postCalendarChat(ctx context.Context, n CalendarNotice) {
	if c.calendar == nil {
		slog.Warn("calendar announcement dropped: no gate wired", "type", n.Type)
		return
	}
	post := domain.CalendarChatPost{
		WorkspaceID: n.WorkspaceID, ChannelID: n.ChannelID, EventID: n.EventID, Title: n.Title,
		ActorUserID: n.ActorUserID, ActorNodeID: n.ActorNodeID, ActorName: c.calendar.UserDisplayName(ctx, n.ActorUserID),
		AllDay: n.AllDay, Zone: n.Tzid, Stamp: n.At,
	}
	if n.OccurrenceStart != "" {
		post.Occurrence, _ = time.Parse(time.RFC3339, n.OccurrenceStart)
	}
	switch n.Type {
	case calendarCreated:
		post.Kind = domain.CalendarChatCard
	case calendarUpdated:
		post.Kind = domain.CalendarChatMoved
		post.Start, _ = time.Parse(time.RFC3339, n.Start)
	case calendarCancelled:
		post.Kind = domain.CalendarChatCancelled
	default:
		return
	}
	msg, err := c.calendar.PostCalendarChat(ctx, post)
	if err != nil {
		slog.Warn("calendar announcement not posted", "type", n.Type, "event", n.EventID, "error", err)
		return
	}
	if msg != nil && c.announcer != nil {
		c.announcer.BroadcastToChannel(msg.ChannelId, msg)
	}
}
