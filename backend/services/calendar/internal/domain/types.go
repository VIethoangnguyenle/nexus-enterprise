package domain

import (
	"context"
	"time"

	"ngac-platform/ngac"
	"ngac-platform/services/calendar/internal/store"
)

// How much of a calendar the caller holds, named for the highest operation they
// hold on it. freebusy is not a grant: it is what a colleague of the workspace
// always gets of a personal calendar, busy ranges and nothing else.
const (
	AccessFreeBusy = "freebusy"
	AccessRead     = ngac.OpRead
	AccessWrite    = ngac.OpWrite
	AccessManage   = ngac.OpManage
)

// Palette indexes a calendar may be drawn in: the eight person hues, or the
// accent colour (accentColor), which workspace calendars use by default.
const (
	accentColor = -1
	maxColor    = 7
)

// ChannelRef is the conversation a group calendar belongs to.
type ChannelRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// CalendarView is a calendar as the caller's list shows it. It carries no
// identifier a screen would display: Owner.NodeID is the key a picker sends back.
type CalendarView struct {
	ID string `json:"id"`
	// Kind is "personal", "workspace" or "channel".
	Kind string `json:"kind"`
	Name string `json:"name"`
	// Color is a person hue index 0..7, or -1 for the accent.
	Color   int  `json:"color"`
	Visible bool `json:"visible"`
	// Access is what the caller holds on the calendar (see the Access constants).
	Access string `json:"access"`
	// IsMine is true for the caller's own personal calendar.
	IsMine bool `json:"isMine"`
	// Owner is the person a personal calendar belongs to.
	Owner *PersonRef `json:"owner,omitempty"`
	// Channel lets the conversation's Calendar tab find its calendar.
	Channel *ChannelRef `json:"channel,omitempty"`
}

// ShareView is one share of a personal calendar.
type ShareView struct {
	Grantee *PersonRef `json:"grantee"`
	Level   string     `json:"level"`
}

// SubscriptionInput changes a person's settings for a calendar; nil fields stay.
type SubscriptionInput struct {
	Color   *int
	Visible *bool
}

// Store is what the domain needs of the database. *store.Store implements it; a
// test uses one in memory.
type Store interface {
	Workspace(ctx context.Context, id string) (*store.Workspace, error)
	PersonByNode(ctx context.Context, workspaceID, nodeID string) (*store.Person, error)
	People(ctx context.Context, workspaceID string, nodeIDs []string) (map[string]*store.Person, error)
	Channel(ctx context.Context, id string) (*store.Channel, error)

	CalendarByID(ctx context.Context, id string) (*store.Calendar, error)
	CalendarFor(ctx context.Context, workspaceID, kind, ownerNodeID, channelID string) (*store.Calendar, error)
	InsertCalendar(ctx context.Context, c *store.Calendar) error
	MemberChannelCalendars(ctx context.Context, workspaceID, userNodeID string) ([]*store.Calendar, error)

	Subscriptions(ctx context.Context, workspaceID, userNodeID string) ([]*store.Subscription, error)
	SubscriptionOf(ctx context.Context, userNodeID, calendarID string) (*store.Subscription, error)
	SaveSubscription(ctx context.Context, userNodeID, calendarID string, color *int16, visible *bool) error
	DeleteSubscription(ctx context.Context, userNodeID, calendarID string) (bool, error)

	Shares(ctx context.Context, calendarID string) ([]*store.Share, error)
	ShareOf(ctx context.Context, calendarID, granteeNodeID string) (*store.Share, error)
	SaveShare(ctx context.Context, sh *store.Share) error
	DeleteShare(ctx context.Context, calendarID, granteeNodeID string) (bool, error)

	EventByID(ctx context.Context, id string) (*store.Event, error)
	OverrideOf(ctx context.Context, seriesID string, original time.Time) (*store.Event, error)
	Overrides(ctx context.Context, seriesID string) ([]*store.Event, error)
	RangeEvents(ctx context.Context, calendarIDs []string, from, to time.Time) (masters, overrides []*store.Event, err error)
	RangeAttended(ctx context.Context, userNodeID string, from, to time.Time) (masters, overrides []*store.Event, err error)
	SearchEvents(ctx context.Context, calendarIDs []string, userNodeID, term string, limit int) ([]*store.Event, error)
	InsertEvent(ctx context.Context, e *store.Event, reminders []int) error
	UpdateEvent(ctx context.Context, e *store.Event, version int, ch store.EventChange) error
	CutSeries(ctx context.Context, series *store.Event, version int, cutFrom time.Time, tail *store.Event, tailReminders *[]int, keepAnswers bool) error
	DeleteEvent(ctx context.Context, id string) error
	Attendees(ctx context.Context, eventID string) ([]*store.Attendee, error)
	Attendee(ctx context.Context, eventID, userNodeID string) (*store.Attendee, error)
	AddAttendees(ctx context.Context, eventID string, add []store.Attendee) error
	RemoveAttendees(ctx context.Context, eventID string, nodes []string) error
	SetResponse(ctx context.Context, eventID, userNodeID, response, comment string) error
	SetOccurrenceResponse(ctx context.Context, eventID, userNodeID string, original time.Time, response, comment string) error
	OccurrenceResponses(ctx context.Context, userNodeID string, eventIDs []string) (map[store.OccKey]string, error)
	OccurrenceAnswers(ctx context.Context, eventID string, original time.Time) (map[string]*store.Attendee, error)
	SetEventNodes(ctx context.Context, eventID, oaID, uaID string) error
	ChannelMembers(ctx context.Context, channelID string) ([]string, error)
	OutOfOffice(ctx context.Context, workspaceID string, nodes []string, from, to time.Time) (map[string]bool, error)
	Settings(ctx context.Context, workspaceID string, nodes []string) (map[string]store.Settings, error)
	SaveSettings(ctx context.Context, workspaceID, userNodeID string, st store.Settings) error
	MyResponses(ctx context.Context, userNodeID string, eventIDs []string) (map[string]string, error)
	GuestCounts(ctx context.Context, eventIDs []string) (map[string]int, error)
	Reminders(ctx context.Context, eventID string) ([]int, error)
	ReminderEvents(ctx context.Context, from, to time.Time) (masters, overrides []*store.Event, err error)
	ReminderSpecs(ctx context.Context, eventIDs []string) (map[string][]store.ReminderSpec, error)
	ClaimReminder(ctx context.Context, eventID, userNodeID string, occurrence time.Time, minutes int) (bool, error)
	DefaultReminder(ctx context.Context, workspaceID, userNodeID string) (int, error)
}
