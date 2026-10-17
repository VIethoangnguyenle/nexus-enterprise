package domain

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"ngac-platform/ngac"
	"ngac-platform/pkg/grpcauth"
	"ngac-platform/pkg/policyclient"
	"ngac-platform/pkg/provision"
	"ngac-platform/pkg/realtime"
	policypb "ngac-platform/proto/policy"
	"ngac-platform/services/calendar/internal/events"
	"ngac-platform/services/calendar/internal/store"
)

const (
	// maxGuests bounds the people one event is shared with, and a group invited at
	// once.
	maxGuests = 200
	// maxAnswerRunes bounds the note that goes with an answer.
	maxAnswerRunes = 500
	// oooComment is the note a guest's automatic refusal carries.
	oooComment = "Đang vắng mặt"
)

// Answers a guest gives, as calendar_attendees.response holds them.
const (
	ResponseNeedsAction = "needs_action"
	ResponseAccepted    = "accepted"
	ResponseDeclined    = "declined"
	ResponseTentative   = "tentative"
)

// --- who may be invited -----------------------------------------------------

// resolveGuests checks the people an event is to be offered to and expands a group
// into its members. A guest must be an active member of the workspace by the
// database and by the graph: anyone else (an address, a person of another
// workspace) is refused and nobody is added. Nodes in skip are already guests, so
// they are passed through unchecked (one who left the workspace can stay on the
// list until removed).
func (s *Service) resolveGuests(ctx context.Context, ws *store.Workspace, who grpcauth.Caller, nodes []string, channelID string, skip map[string]bool) ([]string, error) {
	seen := map[string]bool{}
	var out []string
	for _, n := range nodes {
		n = strings.TrimSpace(n)
		if n == "" || seen[n] {
			continue
		}
		seen[n] = true
		if !skip[n] {
			if strings.Contains(n, "@") {
				return nil, invalid("choose people of this workspace; an email address cannot be invited")
			}
			if _, err := s.requirePerson(ctx, ws, n); err != nil {
				return nil, err
			}
		}
		out = append(out, n)
	}
	if channelID != "" {
		members, err := s.channelGuests(ctx, ws, who, channelID)
		if err != nil {
			return nil, err
		}
		for _, m := range members {
			if !seen[m] {
				seen[m] = true
				out = append(out, m)
			}
		}
	}
	if len(out) > maxGuests {
		return nil, invalid("an event has at most %d guests", maxGuests)
	}
	return out, nil
}

// channelGuests lists the members of a group conversation for an invitation. The
// caller needs read on the conversation's content OA; the member list the database
// keeps is only a cache, so each member is checked against the graph, and anyone
// who has left the workspace or the group is left out.
func (s *Service) channelGuests(ctx context.Context, ws *store.Workspace, who grpcauth.Caller, channelID string) ([]string, error) {
	ch, err := s.store.Channel(ctx, channelID)
	switch {
	case errors.Is(err, store.ErrNotFound):
		return nil, notFound("conversation not found")
	case err != nil:
		return nil, internal("load conversation", err)
	case ch.WorkspaceID != ws.ID:
		return nil, notFound("conversation not found")
	}
	if err := s.authorize(ctx, who.NGACNodeID, &store.Calendar{NgacOAID: ch.ContentOAID}, ngac.OpRead); err != nil {
		return nil, err
	}
	cached, err := s.store.ChannelMembers(ctx, ch.ID)
	if err != nil {
		return nil, internal("list group members", err)
	}
	if len(cached) > maxGuests {
		return nil, invalid("a group of more than %d people cannot be invited at once", maxGuests)
	}
	people, err := s.store.People(ctx, ws.ID, cached)
	if err != nil {
		return nil, internal("name group members", err)
	}
	client := policyclient.New(s.policyRead)
	var out []string
	for _, m := range cached {
		if people[m] == nil {
			continue
		}
		in, err := s.inWorkspace(ctx, s.policyWrite, ws, m)
		if err != nil {
			return nil, internal("check group member", err)
		}
		if !in {
			continue
		}
		if ok, _ := client.Check(ctx, m, ch.ContentOAID, ngac.OpRead); ok {
			out = append(out, m)
		}
	}
	return out, nil
}

// --- the event's nodes in the graph -------------------------------------------

// wireEventNodes puts an event's OA and guest UA into the graph the way the spec
// shapes them: the OA beneath its calendar's OA, the UA beneath the workspace's
// policy class, the UA associated with what a guest holds, and the members in the
// UA. It sets the event's node ids. Every write is an upsert, so a run repeated
// after a failure continues; prov undoes the nodes this run created.
func (s *Service) wireEventNodes(ctx context.Context, prov *provision.Creator, ws *store.Workspace, cal *store.Calendar, e *store.Event, members []string) error {
	id := ngac.CalendarEventID(e.ID)
	oa, err := prov.EnsureNode(ctx, s.policyRead, &policypb.CreateNodeRequest{Name: ngac.CalendarEventOAName(id), NodeType: ngac.TypeOA})
	if err != nil {
		return fmt.Errorf("create event OA: %w", err)
	}
	ua, err := prov.EnsureNode(ctx, s.policyRead, &policypb.CreateNodeRequest{Name: ngac.CalendarEventGuestsUAName(id), NodeType: ngac.TypeUA})
	if err != nil {
		return fmt.Errorf("create event guests UA: %w", err)
	}
	if err := prov.Assign(ctx, oa.GetId(), cal.NgacOAID); err != nil {
		return fmt.Errorf("assign event under its calendar: %w", err)
	}
	if err := prov.Assign(ctx, ua.GetId(), ws.PCNodeID); err != nil {
		return fmt.Errorf("assign guests under the policy class: %w", err)
	}
	if err := prov.Associate(ctx, ua.GetId(), oa.GetId(), ngac.CalendarGuestOps(e.GuestsCanInvite)); err != nil {
		return fmt.Errorf("grant guests the event: %w", err)
	}
	for _, m := range members {
		if err := prov.Assign(ctx, m, ua.GetId()); err != nil {
			return fmt.Errorf("add a guest: %w", err)
		}
	}
	e.NgacOAID, e.GuestsUAID = oa.GetId(), ua.GetId()
	return nil
}

// dropEventNodes removes an event's OA and UA once it has no guest and no call, so
// a plain event has no node again.
func (s *Service) dropEventNodes(ctx context.Context, e *store.Event) error {
	for _, node := range []string{e.GuestsUAID, e.NgacOAID} {
		if node == "" {
			continue
		}
		if _, err := s.policyWrite.DeleteNode(ctx, &policypb.DeleteNodeRequest{NodeId: node}); err != nil {
			return internal("delete the event's access nodes", err)
		}
	}
	if err := s.store.SetEventNodes(ctx, e.ID, "", ""); err != nil {
		return storeErr("forget the event's access nodes", err)
	}
	e.NgacOAID, e.GuestsUAID = "", ""
	return nil
}

// unassign takes people out of a guest UA, best effort: it is the undo of a write
// that failed, and a failure here is logged for an operator.
func (s *Service) unassign(ctx context.Context, ua string, nodes []string) {
	for _, n := range nodes {
		if _, err := s.policyWrite.RemoveAssignment(ctx, &policypb.RemoveAssignmentRequest{ChildId: n, ParentId: ua}); err != nil {
			slog.Error("could not take a guest out of an event after a failed write", "guestsUA", ua, "error", err)
		}
	}
}

// --- adding and removing guests ---------------------------------------------

// addGuests invites people to a single event or series: the graph first (so a guest
// never has a row without the access it promises), then the rows. A guest who has
// an out-of-office event over the time has already declined. With nobody to add it
// still creates the event's nodes when it has a call and none yet. It returns the
// people it added.
func (s *Service) addGuests(ctx context.Context, ws *store.Workspace, cal *store.Calendar, e *store.Event, nodes []string) ([]string, error) {
	have, err := s.store.Attendees(ctx, e.ID)
	if err != nil {
		return nil, internal("load guests", err)
	}
	present := map[string]bool{}
	members := make([]string, 0, len(have)+len(nodes))
	for _, a := range have {
		present[a.UserNodeID] = true
		members = append(members, a.UserNodeID)
	}
	var fresh []string
	for _, n := range nodes {
		if !present[n] {
			present[n] = true
			fresh = append(fresh, n)
			members = append(members, n)
		}
	}
	if len(fresh) == 0 && (e.NgacOAID != "" || !e.HasCall) {
		return nil, nil
	}
	if len(members) > maxGuests+1 {
		return nil, invalid("an event has at most %d guests", maxGuests)
	}
	away, err := s.store.OutOfOffice(ctx, ws.ID, fresh, e.StartsAt, e.EndsAt)
	if err != nil {
		return nil, internal("check absences", err)
	}
	rows := make([]store.Attendee, len(fresh))
	for i, n := range fresh {
		rows[i] = store.Attendee{UserNodeID: n, Response: ResponseNeedsAction}
		if away[n] {
			rows[i].Response, rows[i].Comment = ResponseDeclined, oooComment
		}
	}

	if e.NgacOAID == "" {
		prov := provision.NewCreator(s.policyWrite)
		if err := s.wireEventNodes(ctx, prov, ws, cal, e, members); err != nil {
			e.NgacOAID, e.GuestsUAID = "", ""
			return nil, prov.Fail(ctx, err)
		}
		if err := s.store.AddAttendees(ctx, e.ID, rows); err != nil {
			e.NgacOAID, e.GuestsUAID = "", ""
			return nil, prov.Fail(ctx, internal("save guests", err))
		}
		if err := s.store.SetEventNodes(ctx, e.ID, e.NgacOAID, e.GuestsUAID); err != nil {
			if rerr := s.store.RemoveAttendees(ctx, e.ID, fresh); rerr != nil {
				slog.Error("could not take guests back out after a failed write", "event", e.ID, "error", rerr)
			}
			e.NgacOAID, e.GuestsUAID = "", ""
			return nil, prov.Fail(ctx, storeErr("record the event's access nodes", err))
		}
		prov.Done()
		return fresh, nil
	}

	for i, n := range fresh {
		_, err := s.policyWrite.CreateAssignment(ctx, &policypb.CreateAssignmentRequest{ChildId: n, ParentId: e.GuestsUAID})
		if err != nil {
			s.unassign(ctx, e.GuestsUAID, fresh[:i])
			return nil, internal("add a guest", err)
		}
	}
	if err := s.store.AddAttendees(ctx, e.ID, rows); err != nil {
		s.unassign(ctx, e.GuestsUAID, fresh)
		return nil, internal("save guests", err)
	}
	return fresh, nil
}

// removeGuests takes guests out of an event: their access first, so the next
// decision for them is DENY, then their rows and answers. The organizer stays.
func (s *Service) removeGuests(ctx context.Context, e *store.Event, nodes []string) error {
	var gone []string
	for _, n := range nodes {
		if n != e.OrganizerNodeID {
			gone = append(gone, n)
		}
	}
	if len(gone) == 0 {
		return nil
	}
	if e.GuestsUAID != "" {
		for _, n := range gone {
			if _, err := s.policyWrite.RemoveAssignment(ctx, &policypb.RemoveAssignmentRequest{ChildId: n, ParentId: e.GuestsUAID}); err != nil {
				return internal("remove a guest", err)
			}
		}
	}
	return internal("forget guests", s.store.RemoveAttendees(ctx, e.ID, gone))
}

// reconcileNodes brings an event's nodes in line with whether it has guests or a
// call: created for the first, dropped when the last is gone, and the guests'
// association rewritten when the host changed whether guests may invite.
func (s *Service) reconcileNodes(ctx context.Context, ws *store.Workspace, cal *store.Calendar, e *store.Event, inviteFlagSet bool) error {
	counts, err := s.store.GuestCounts(ctx, []string{e.ID})
	if err != nil {
		return internal("count guests", err)
	}
	want := counts[e.ID] > 0 || e.HasCall
	switch {
	case want && e.NgacOAID == "":
		_, err := s.addGuests(ctx, ws, cal, e, nil)
		return err
	case !want && e.NgacOAID != "":
		return s.dropEventNodes(ctx, e)
	case want && inviteFlagSet:
		_, err := s.policyWrite.CreateAssociation(ctx, &policypb.CreateAssociationRequest{
			UaId: e.GuestsUAID, OaId: e.NgacOAID, Operations: ngac.CalendarGuestOps(e.GuestsCanInvite)})
		return internal("update what guests may do", err)
	}
	return nil
}

// guestPlan is a change to who is invited, worked out and checked before anything
// is written.
type guestPlan struct {
	add, remove []string
	// inviteFlagSet: the patch names whether guests may invite others.
	inviteFlagSet bool
	// touched: the patch concerns guests, the invite flag or the call at all.
	touched bool
}

// planGuests works out what a patch does to the guest list. Attendees, when sent,
// is the whole list of guests wanted (the organizer is always one); InviteChannelID
// adds a group's members to it. Guests belong to the whole series, so a change to
// one occurrence cannot name them.
func (s *Service) planGuests(ctx context.Context, ed *editing, p *EventPatch, scope string) (*guestPlan, error) {
	names := p.Attendees != nil || (p.InviteChannelID != nil && *p.InviteChannelID != "")
	flags := p.GuestsCanInvite != nil || p.HasCall != nil
	plan := &guestPlan{}
	if scope == ScopeThis {
		if names {
			return nil, invalid("guests belong to the whole series, not one occurrence")
		}
		return plan, nil
	}
	if !names && !flags {
		return plan, nil
	}
	plan.touched, plan.inviteFlagSet = true, p.GuestsCanInvite != nil
	if !names {
		return plan, nil
	}
	have, err := s.store.Attendees(ctx, ed.e.ID)
	if err != nil {
		return nil, internal("load guests", err)
	}
	existing := map[string]bool{}
	for _, a := range have {
		existing[a.UserNodeID] = true
	}
	var listed []string
	if p.Attendees != nil {
		listed = *p.Attendees
	}
	channel := ""
	if p.InviteChannelID != nil {
		channel = *p.InviteChannelID
	}
	wanted, err := s.resolveGuests(ctx, ed.ws, ed.who, listed, channel, existing)
	if err != nil {
		return nil, err
	}
	keep := map[string]bool{ed.e.OrganizerNodeID: true}
	for _, n := range wanted {
		keep[n] = true
		if !existing[n] {
			plan.add = append(plan.add, n)
		}
	}
	if p.Attendees != nil {
		for _, a := range have {
			if !a.IsOrganizer && !keep[a.UserNodeID] {
				plan.remove = append(plan.remove, a.UserNodeID)
			}
		}
	}
	return plan, nil
}

// changeGuests applies a plan to the event that now holds the edit.
func (s *Service) changeGuests(ctx context.Context, ws *store.Workspace, cal *store.Calendar, m *store.Event, plan *guestPlan) ([]string, error) {
	if err := s.removeGuests(ctx, m, plan.remove); err != nil {
		return nil, err
	}
	added, err := s.addGuests(ctx, ws, cal, m, plan.add)
	if err != nil {
		return nil, err
	}
	return added, s.reconcileNodes(ctx, ws, cal, m, plan.inviteFlagSet)
}

// AddGuests invites more people to an event. The host (write on the calendar) may;
// so may a guest who holds invite on the event's OA while the host allows guests to
// bring others. Everyone else is refused.
func (s *Service) AddGuests(ctx context.Context, wsID, id string, in GuestsInput) (*EventDetailView, error) {
	who, ws, err := s.requireMember(ctx, wsID)
	if err != nil {
		return nil, err
	}
	e, cal, err := s.loadEvent(ctx, wsID, id)
	if err != nil {
		return nil, err
	}
	level, err := s.levelOn(ctx, who, cal)
	if err != nil {
		return nil, err
	}
	if len(in.Attendees) == 0 && in.InviteChannelID == "" {
		return nil, invalid("name the people or the group to invite")
	}
	if err := s.admitInviter(ctx, who, e, cal, level); err != nil {
		return nil, err
	}
	have, err := s.store.Attendees(ctx, e.ID)
	if err != nil {
		return nil, internal("load guests", err)
	}
	existing := map[string]bool{}
	for _, a := range have {
		existing[a.UserNodeID] = true
	}
	wanted, err := s.resolveGuests(ctx, ws, who, in.Attendees, in.InviteChannelID, existing)
	if err != nil {
		return nil, err
	}
	added, err := s.addGuests(ctx, ws, cal, e, wanted)
	if err != nil {
		return nil, err
	}
	s.announceGuests(ctx, ws, cal, realtime.KindUpdated, distinct(append(added, who.NGACNodeID, e.OrganizerNodeID)...), e.ID)
	s.notify(ctx, events.TypeInvited, ws, cal, e, added, noticeExtras{actor: who})
	in2, err := s.instanceAt(ctx, e, "")
	if err != nil {
		in2 = instance{Row: e, Master: e, Original: e.StartsAt, Start: e.StartsAt, End: e.EndsAt}
	}
	return s.detailView(ctx, ws, who, in2, level, true, 0)
}

// admitInviter admits the caller to invite people to an event.
func (s *Service) admitInviter(ctx context.Context, who grpcauth.Caller, e *store.Event, cal *store.Calendar, level string) error {
	if canWrite(level) {
		responses, err := s.store.MyResponses(ctx, who.NGACNodeID, []string{e.ID})
		if err != nil {
			return internal("load answers", err)
		}
		_, guest := responses[e.ID]
		if !showsDetail(level, e, cal, who.NGACNodeID, guest) {
			return denied("no write access")
		}
		return nil
	}
	// A guest brings others only while the host allows it. The flag is checked in
	// the database and in the graph, so neither alone can grant it.
	if !e.GuestsCanInvite || e.NgacOAID == "" {
		return denied("no invite access")
	}
	if ok, _ := policyclient.New(s.policyRead).Check(ctx, who.NGACNodeID, e.NgacOAID, ngac.OpInvite); !ok {
		return denied("no invite access")
	}
	return nil
}

// --- answering --------------------------------------------------------------

// Rsvp records the caller's answer to an invitation: for themselves only, and only
// as a guest who can still read the event. For a repeating event the answer is for
// one occurrence ("this") or for the series ("all"; it also replaces what they
// said about single occurrences).
func (s *Service) Rsvp(ctx context.Context, wsID, id string, in RsvpInput) (*EventDetailView, error) {
	who, ws, err := s.requireMember(ctx, wsID)
	if err != nil {
		return nil, err
	}
	e, cal, err := s.loadEvent(ctx, wsID, id)
	if err != nil {
		return nil, err
	}
	switch in.Response {
	case ResponseAccepted, ResponseDeclined, ResponseTentative:
	default:
		return nil, invalid("response must be accepted, declined or tentative")
	}
	if utf8.RuneCountInString(in.Comment) > maxAnswerRunes {
		return nil, invalid("a note is at most %d characters", maxAnswerRunes)
	}
	if _, err := s.store.Attendee(ctx, e.ID, who.NGACNodeID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, denied("only a guest answers an invitation")
		}
		return nil, internal("load guest", err)
	}
	level, err := s.levelOn(ctx, who, cal)
	if err != nil {
		return nil, err
	}
	if !canRead(level) && !s.readsEventOA(ctx, who.NGACNodeID, e) {
		return nil, denied("no read access")
	}

	var occurrence time.Time
	switch {
	case e.RRule == "" || in.Scope == ScopeAll:
		err = s.store.SetResponse(ctx, e.ID, who.NGACNodeID, in.Response, in.Comment)
	case in.Scope == ScopeThis:
		if in.Occurrence == "" {
			return nil, invalid("occurrence is required for scope %s", ScopeThis)
		}
		orig, _, _, oerr := s.occurrenceFor(ctx, e, in.Occurrence)
		if oerr != nil {
			return nil, oerr
		}
		occurrence = orig
		err = s.store.SetOccurrenceResponse(ctx, e.ID, who.NGACNodeID, orig, in.Response, in.Comment)
	case in.Scope == "":
		return nil, invalid("scope is required for a repeating event")
	default:
		return nil, invalid("scope must be this or all")
	}
	if errors.Is(err, store.ErrNotFound) {
		return nil, denied("only a guest answers an invitation")
	}
	if err != nil {
		return nil, internal("save answer", err)
	}

	// The host's screen learns of the answer; notifying them waits for the
	// notification service.
	s.announceGuests(ctx, ws, cal, realtime.KindUpdated, distinct(e.OrganizerNodeID, who.NGACNodeID), e.ID)
	s.notify(ctx, events.TypeRsvp, ws, cal, e, []string{e.OrganizerNodeID},
		noticeExtras{actor: who, response: in.Response, comment: in.Comment, occurrence: occurrence})

	shown, err := s.instanceAt(ctx, e, in.Occurrence)
	if err != nil {
		shown = instance{Row: e, Master: e, Original: e.StartsAt, Start: e.StartsAt, End: e.EndsAt}
	}
	return s.detailView(ctx, ws, who, shown, level, true, 0)
}

// --- announcing to guests ---------------------------------------------------

// guestNodes lists the people an event is shared with, for the frames that tell
// their screens. An event with no OA has no guests.
func (s *Service) guestNodes(ctx context.Context, e *store.Event) []string {
	if e.NgacOAID == "" {
		return nil
	}
	list, err := s.store.Attendees(ctx, e.SeriesID())
	if err != nil {
		slog.Warn("could not list guests to address a calendar event", "event", e.ID, "error", err)
		return nil
	}
	nodes := make([]string, len(list))
	for i, a := range list {
		nodes[i] = a.UserNodeID
	}
	return nodes
}

// announceGuests tells the screens of some people that an event changed. The frame
// carries ids only.
func (s *Service) announceGuests(ctx context.Context, ws *store.Workspace, cal *store.Calendar, kind string, nodes []string, eventIDs ...string) {
	if len(nodes) == 0 {
		return
	}
	e := realtime.For(ctx, realtime.DomainCalendar, kind, ws.ID, eventIDs...)
	e.ParentID = cal.ID
	e.UserNodeIDs = distinct(nodes...)
	s.emit(e)
}
