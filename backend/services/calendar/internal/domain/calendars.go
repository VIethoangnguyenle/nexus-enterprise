package domain

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"ngac-platform/ngac"
	"ngac-platform/pkg/grpcauth"
	"ngac-platform/pkg/provision"
	"ngac-platform/pkg/realtime"
	policypb "ngac-platform/proto/policy"
	"ngac-platform/services/calendar/internal/store"
)

// calendarNamespace seeds the calendar IDs. A calendar's ID is derived from what
// makes it unique (its kind and owner), so a creation that is retried, or that
// two requests race, names the same OA and the same row.
var calendarNamespace = uuid.MustParse("6f0c8c1e-3b0a-4d6e-9d5a-7f2f4c1b8a10")

// Palette index a calendar starts with, by kind.
var defaultColor = map[string]int16{store.KindPersonal: 0, store.KindWorkspace: 1, store.KindChannel: 2}

// target is the calendar to find or create.
type target struct {
	kind    string
	owner   string         // user node, personal calendars
	channel *store.Channel // group calendars
}

func (t target) channelID() string {
	if t.channel == nil {
		return ""
	}
	return t.channel.ID
}

func (t target) id(wsID string) ngac.CalendarID {
	key := t.kind + "/" + wsID + "/" + t.owner + "/" + t.channelID()
	return ngac.CalendarID(uuid.NewSHA1(calendarNamespace, []byte(key)).String())
}

// ensureCalendar returns the calendar of a kind, creating it the first time it
// is needed. created reports whether this call made it.
//
// Creation is idempotent and race-safe: the ID is derived from the target, so
// every attempt names the same OA, the node lookup reuses one that exists, the
// graph edges are upserts, and a unique index keeps one row. A request that
// loses the insert reads the winner's row; the nodes are the same ones.
func (s *Service) ensureCalendar(ctx context.Context, ws *store.Workspace, t target) (cal *store.Calendar, created bool, err error) {
	cal, err = s.store.CalendarFor(ctx, ws.ID, t.kind, t.owner, t.channelID())
	if err == nil {
		return cal, false, nil
	}
	if !errors.Is(err, store.ErrNotFound) {
		return nil, false, internal("find calendar", err)
	}

	id := t.id(ws.ID)
	prov := provision.NewCreator(s.policyWrite)
	oa, err := prov.EnsureNode(ctx, s.policyRead, &policypb.CreateNodeRequest{
		Name: ngac.CalendarOAName(id), NodeType: ngac.TypeOA,
	})
	if err != nil {
		return nil, false, prov.Fail(ctx, fmt.Errorf("create calendar OA: %w", err))
	}
	if err := s.wireCalendar(ctx, prov, ws, t, oa.GetId()); err != nil {
		return nil, false, prov.Fail(ctx, err)
	}

	row := &store.Calendar{
		ID: string(id), WorkspaceID: ws.ID, Kind: t.kind, OwnerNodeID: t.owner, ChannelID: t.channelID(),
		Color: defaultColor[t.kind], NgacOAID: oa.GetId(),
	}
	switch err := s.store.InsertCalendar(ctx, row); {
	case err == nil:
		prov.Done()
	case errors.Is(err, store.ErrAlreadyExists):
		// Raced: the winner's row names the very same OA, which must stay.
		prov.Done()
	default:
		return nil, false, prov.Fail(ctx, err)
	}
	cal, err = s.store.CalendarFor(ctx, ws.ID, t.kind, t.owner, t.channelID())
	if err != nil {
		return nil, false, internal("read calendar", err)
	}
	return cal, true, nil
}

// wireCalendar puts a calendar's OA into the graph the way the spec shapes it
// for its kind.
func (s *Service) wireCalendar(ctx context.Context, prov *provision.Creator, ws *store.Workspace, t target, oaID string) error {
	switch t.kind {
	case store.KindPersonal:
		if err := prov.Assign(ctx, oaID, ws.PCNodeID); err != nil {
			return fmt.Errorf("assign calendar under the policy class: %w", err)
		}
		ua, err := provision.EnsurePersonalUA(ctx, s.policyRead, s.policyWrite, t.owner)
		if err != nil {
			return err
		}
		if err := prov.Associate(ctx, ua, oaID, ngac.CalendarOwnerOps()); err != nil {
			return fmt.Errorf("grant the owner the calendar: %w", err)
		}
	case store.KindWorkspace:
		members, err := s.namedUA(ctx, ngac.MembersUAName(ngac.WorkspaceID(ws.ID)))
		if err != nil {
			return err
		}
		owners, err := s.namedUA(ctx, ngac.OwnersUAName(ngac.WorkspaceID(ws.ID)))
		if err != nil {
			return err
		}
		if err := prov.Assign(ctx, oaID, ws.PCNodeID); err != nil {
			return fmt.Errorf("assign calendar under the policy class: %w", err)
		}
		if err := prov.Associate(ctx, members, oaID, []string{ngac.OpRead}); err != nil {
			return fmt.Errorf("grant members the calendar: %w", err)
		}
		if err := prov.Associate(ctx, owners, oaID, ngac.CalendarOwnerOps()); err != nil {
			return fmt.Errorf("grant owners the calendar: %w", err)
		}
	case store.KindChannel:
		// Beneath the conversation's content OA, so its members inherit what they
		// hold there and nobody else holds anything.
		if err := prov.Assign(ctx, oaID, t.channel.ContentOAID); err != nil {
			return fmt.Errorf("assign calendar under the conversation: %w", err)
		}
	default:
		return fmt.Errorf("calendar of kind %q", t.kind)
	}
	return nil
}

// namedUA resolves a well-known UA by name; absent or unreachable is a failure
// of ours, never an allow.
func (s *Service) namedUA(ctx context.Context, name string) (string, error) {
	n, err := s.policyRead.FindNodeByName(ctx, &policypb.FindNodeByNameRequest{Name: name, NodeType: ngac.TypeUA})
	if err != nil || n.GetId() == "" {
		return "", fmt.Errorf("find %s: %w", name, err)
	}
	return n.GetId(), nil
}

// ListCalendars returns the caller's calendars in a workspace: their own, the
// workspace's, the group calendars of conversations they are in, and the ones
// they added. Their own and the workspace's are created on first use.
func (s *Service) ListCalendars(ctx context.Context, wsID string) ([]*CalendarView, error) {
	who, ws, err := s.requireMember(ctx, wsID)
	if err != nil {
		return nil, err
	}

	var cals []*store.Calendar
	seen := map[string]bool{}
	add := func(c *store.Calendar) {
		if !seen[c.ID] {
			seen[c.ID] = true
			cals = append(cals, c)
		}
	}

	own, _, err := s.ensureCalendar(ctx, ws, target{kind: store.KindPersonal, owner: who.NGACNodeID})
	if err != nil {
		return nil, err
	}
	add(own)
	if ws.Type != workspaceTypePersonal {
		shared, _, err := s.ensureCalendar(ctx, ws, target{kind: store.KindWorkspace})
		if err != nil {
			return nil, err
		}
		add(shared)
		groups, err := s.store.MemberChannelCalendars(ctx, ws.ID, who.NGACNodeID)
		if err != nil {
			return nil, internal("list group calendars", err)
		}
		for _, c := range groups {
			add(c)
		}
	}
	subs, err := s.store.Subscriptions(ctx, ws.ID, who.NGACNodeID)
	if err != nil {
		return nil, internal("list subscriptions", err)
	}
	settings := map[string]*store.Subscription{}
	for _, sub := range subs {
		settings[sub.Calendar.ID] = sub
		add(sub.Calendar)
	}

	held, err := s.accessLevels(ctx, who.NGACNodeID, cals)
	if err != nil {
		return nil, err
	}
	out := make([]*CalendarView, 0, len(cals))
	for _, c := range cals {
		if access := accessTo(held, c); access != "" {
			out = append(out, viewOf(c, access, settings[c.ID], who.NGACNodeID))
		}
	}
	return out, nil
}

// viewOf shapes a calendar for the caller, with their own colour and visibility
// where they set any.
func viewOf(c *store.Calendar, access string, sub *store.Subscription, callerNode string) *CalendarView {
	v := &CalendarView{
		ID: c.ID, Name: c.Name, Kind: c.Kind, Color: int(c.Color), Visible: true, Access: access,
		Own: c.Kind == store.KindPersonal && c.OwnerNodeID == callerNode, Owner: c.Owner, ChannelID: c.ChannelID,
	}
	if sub != nil {
		v.Visible = sub.Visible
		if sub.Color != nil {
			v.Color = int(*sub.Color)
		}
	}
	return v
}

// viewFor builds the view of one calendar for the caller from what is stored.
func (s *Service) viewFor(ctx context.Context, who grpcauth.Caller, c *store.Calendar) (*CalendarView, error) {
	held, err := s.accessLevels(ctx, who.NGACNodeID, []*store.Calendar{c})
	if err != nil {
		return nil, err
	}
	access := accessTo(held, c)
	if access == "" {
		return nil, denied("no read access")
	}
	sub, err := s.store.SubscriptionOf(ctx, who.NGACNodeID, c.ID)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return nil, internal("load subscription", err)
	}
	if errors.Is(err, store.ErrNotFound) {
		sub = nil
	}
	return viewOf(c, access, sub, who.NGACNodeID), nil
}

// SetSubscription changes the caller's colour and visibility for a calendar they
// can see. Only the fields given change.
func (s *Service) SetSubscription(ctx context.Context, wsID, calID string, in SubscriptionInput) (*CalendarView, error) {
	who, _, err := s.requireMember(ctx, wsID)
	if err != nil {
		return nil, err
	}
	cal, err := s.loadCalendar(ctx, wsID, calID)
	if err != nil {
		return nil, err
	}
	if in.Color != nil && (*in.Color < 0 || *in.Color > maxColor) {
		return nil, invalid("color must be between 0 and %d", maxColor)
	}
	if _, err := s.viewFor(ctx, who, cal); err != nil { // the caller must be able to see it
		return nil, err
	}
	var color *int16
	if in.Color != nil {
		c := int16(*in.Color)
		color = &c
	}
	if err := s.store.SaveSubscription(ctx, who.NGACNodeID, cal.ID, color, in.Visible); err != nil {
		return nil, internal("save subscription", err)
	}
	return s.viewFor(ctx, who, cal)
}

// Subscribe adds a colleague's personal calendar to the caller's list. The
// colleague must be of the same workspace; the caller sees at least when they
// are busy.
func (s *Service) Subscribe(ctx context.Context, wsID, personNodeID string) (*CalendarView, error) {
	who, ws, err := s.requireMember(ctx, wsID)
	if err != nil {
		return nil, err
	}
	if _, err := s.requirePerson(ctx, ws, personNodeID); err != nil {
		return nil, err
	}
	cal, _, err := s.ensureCalendar(ctx, ws, target{kind: store.KindPersonal, owner: personNodeID})
	if err != nil {
		return nil, err
	}
	if personNodeID != who.NGACNodeID {
		if err := s.store.SaveSubscription(ctx, who.NGACNodeID, cal.ID, nil, nil); err != nil {
			return nil, internal("save subscription", err)
		}
	}
	return s.viewFor(ctx, who, cal)
}

// Unsubscribe takes a colleague's calendar off the caller's list. The caller's
// own calendar, the workspace's and their groups' are always on it: those are
// hidden, not removed.
func (s *Service) Unsubscribe(ctx context.Context, wsID, calID string) error {
	who, _, err := s.requireMember(ctx, wsID)
	if err != nil {
		return err
	}
	cal, err := s.loadCalendar(ctx, wsID, calID)
	if err != nil {
		return err
	}
	if cal.Kind != store.KindPersonal || cal.OwnerNodeID == who.NGACNodeID {
		return invalid("this calendar is always on your list; hide it instead")
	}
	if _, err := s.store.DeleteSubscription(ctx, who.NGACNodeID, cal.ID); err != nil {
		return internal("delete subscription", err)
	}
	return nil
}

// EnsureChannelCalendar returns the calendar of a group conversation, creating
// it the first time it is asked for. It needs write on the conversation's content
// OA, which only its members hold; a direct message has no calendar.
func (s *Service) EnsureChannelCalendar(ctx context.Context, wsID, channelID string) (*CalendarView, error) {
	who, ws, err := s.requireMember(ctx, wsID)
	if err != nil {
		return nil, err
	}
	ch, err := s.store.Channel(ctx, channelID)
	switch {
	case errors.Is(err, store.ErrNotFound):
		return nil, notFound("conversation not found")
	case err != nil:
		return nil, internal("load conversation", err)
	case ch.WorkspaceID != ws.ID:
		return nil, notFound("conversation not found")
	}
	if err := s.authorize(ctx, who.NGACNodeID, &store.Calendar{NgacOAID: ch.ContentOAID}, ngac.OpWrite); err != nil {
		return nil, err
	}
	if ch.Type == "dm" {
		return nil, invalid("a direct message has no calendar")
	}
	if ws.Type == workspaceTypePersonal {
		return nil, invalid("a personal workspace has only the personal calendar")
	}
	cal, created, err := s.ensureCalendar(ctx, ws, target{kind: store.KindChannel, channel: ch})
	if err != nil {
		return nil, err
	}
	if created {
		e := realtime.For(ctx, realtime.DomainCalendar, realtime.KindCreated, ws.ID, cal.ID)
		e.ChannelID = ch.ID
		s.emit(e)
	}
	return s.viewFor(ctx, who, cal)
}

func logRevertFailure(what string, err error) {
	slog.Error("could not restore a calendar share after a failed write", "what", what, "error", err)
}
