// Package domain — what a group's calendar says into the group's chat: the card
// of a new event and the short lines about its moves and cancellation.
package domain

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	pb "ngac-platform/proto/messaging"
	"ngac-platform/services/messaging/internal/store"
)

// linkedCalendarEvent is the linked_entity_type of an event card. The card holds
// only the event's id; its details are read live from the calendar service under
// the viewer's own access.
const linkedCalendarEvent = "calendar_event"

// calendarChatNamespace derives the ids of the messages a calendar notice makes,
// so a notice delivered twice makes one message.
var calendarChatNamespace = uuid.MustParse("6f1c1f0e-3b7a-4a55-9a0b-2d1c1b6a7e11")

// Kinds of line a group's calendar writes into its chat.
const (
	CalendarChatCard      = "card"
	CalendarChatMoved     = "moved"
	CalendarChatCancelled = "cancelled"
)

// CalendarChatPost is one thing to say in a group's chat about an event on the
// group's calendar.
type CalendarChatPost struct {
	Kind        string
	WorkspaceID string
	ChannelID   string
	EventID     string
	Title       string
	// ActorUserID and ActorNodeID are whoever caused it; the message is theirs.
	ActorUserID string
	ActorNodeID string
	ActorName   string
	// Start is where the event (or the occurrence) now is; Occurrence names the
	// occurrence of a repeating event the line is about, "" for the whole event.
	Start      time.Time
	AllDay     bool
	Occurrence time.Time
	// Zone is the IANA zone the event was made in; times are worded in it.
	Zone string
	// Stamp tells one announcement from the next one about the same event (the
	// notice's own time): the same notice delivered again has the same stamp.
	Stamp int64
}

// PostCalendarChat writes the line into the group's chat as a system message of
// the actor, and returns it. It returns nil when the same notice was posted
// before. Nothing is posted into a conversation that is not a group of the
// event's workspace, nor for an actor who can no longer read it.
func (s *Service) PostCalendarChat(ctx context.Context, p CalendarChatPost) (*pb.Message, error) {
	if p.ChannelID == "" || p.EventID == "" || p.ActorUserID == "" {
		return nil, ErrInvalidInput
	}
	ch, err := s.loadChannel(ctx, p.ChannelID)
	if err != nil {
		return nil, err
	}
	if ch.ChannelType == "dm" || ch.WorkspaceID == "" || ch.WorkspaceID != p.WorkspaceID {
		return nil, fmt.Errorf("%w: not a group of this workspace", ErrInvalidInput)
	}
	if !s.CanRead(ctx, p.ActorNodeID, ch.NGACOaID) {
		return nil, fmt.Errorf("%w: the actor does not read this group", ErrAccessDenied)
	}

	msg := &store.Message{
		ID:        uuid.NewSHA1(calendarChatNamespace, []byte(calendarChatKey(p))).String(),
		ChannelID: ch.ID, SenderID: p.ActorUserID, ContentFormat: "plain", MessageType: "system",
		CreatedAt: time.Now(),
	}
	switch p.Kind {
	case CalendarChatCard:
		msg.Content = "📅 " + p.Title
		msg.LinkedEntityType, msg.LinkedEntityID = linkedCalendarEvent, p.EventID
	case CalendarChatMoved, CalendarChatCancelled:
		msg.Content = calendarLine(p)
	default:
		return nil, fmt.Errorf("%w: unknown calendar line", ErrInvalidInput)
	}
	stored, err := s.store.InsertMessageOnce(ctx, msg)
	if err != nil {
		return nil, err
	}
	if !stored {
		return nil, nil
	}
	s.BumpChat(ch.WorkspaceID)
	return s.announcementProto(ctx, msg), nil
}

// calendarChatKey is what a post is the same post by.
func calendarChatKey(p CalendarChatPost) string {
	if p.Kind == CalendarChatCard {
		return "calendar-card|" + p.EventID
	}
	return fmt.Sprintf("calendar-%s|%s|%s|%d", p.Kind, p.EventID, p.Occurrence.UTC().Format(time.RFC3339), p.Stamp)
}

// calendarLine words a move or a cancellation as a plain line.
func calendarLine(p CalendarChatPost) string {
	who := strings.TrimSpace(p.ActorName)
	if who == "" {
		who = "Một thành viên"
	}
	loc, err := time.LoadLocation(p.Zone)
	if err != nil || p.Zone == "" {
		loc = time.FixedZone("UTC+7", 7*3600)
	}
	title := "“" + p.Title + "”"
	if !p.Occurrence.IsZero() {
		title = "một buổi của " + title
	}
	if p.Kind == CalendarChatCancelled {
		if !p.Occurrence.IsZero() {
			return fmt.Sprintf("%s đã huỷ buổi %s của %s", who, calendarWhen(p.Occurrence.In(loc), p.AllDay), "“"+p.Title+"”")
		}
		return fmt.Sprintf("%s đã huỷ %s", who, title)
	}
	return fmt.Sprintf("%s đã dời %s sang %s", who, title, calendarWhen(p.Start.In(loc), p.AllDay))
}

func calendarWhen(t time.Time, allDay bool) string {
	if allDay {
		return "ngày " + t.Format("02/01")
	}
	return t.Format("15:04 02/01")
}
