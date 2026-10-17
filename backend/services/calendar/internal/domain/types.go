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

// maxColor is the highest palette index a person may pick for a calendar.
const maxColor = 31

// CalendarView is a calendar as the caller's list shows it. It carries no
// identifier a screen would display: Owner.NodeID is the key a picker sends back.
type CalendarView struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// Kind is "personal", "workspace" or "channel".
	Kind    string `json:"kind"`
	Color   int    `json:"color"`
	Visible bool   `json:"visible"`
	// Access is what the caller holds on the calendar (see the Access constants).
	Access string `json:"access"`
	// Own is true for the caller's own personal calendar.
	Own bool `json:"own"`
	// Owner is the person a personal calendar belongs to.
	Owner *store.Person `json:"owner,omitempty"`
	// ChannelID lets the conversation's Calendar tab find its calendar.
	ChannelID string `json:"channelId,omitempty"`
}

// ShareView is one share of a personal calendar.
type ShareView struct {
	Person    *store.Person `json:"person"`
	Level     string        `json:"level"`
	CreatedAt time.Time     `json:"createdAt"`
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
}
