package domain

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/ngac"
	"ngac-platform/services/calendar/internal/store"
)

// withGuests is a new event of alice's with these people invited.
func withGuests(cal string, nodes ...string) EventInput {
	in := newInput(cal)
	in.Attendees = nodes
	return in
}

// logHas reports whether the policy log holds a line with this prefix.
func (f *fixture) logHas(prefix string) bool {
	for _, l := range f.w.Log {
		if strings.HasPrefix(l, prefix) {
			return true
		}
	}
	return false
}

func eventOA(id string) string { return ngac.CalendarEventOAName(ngac.CalendarEventID(id)) }
func eventUA(id string) string { return ngac.CalendarEventGuestsUAName(ngac.CalendarEventID(id)) }

func TestAGuestReadsOnlyTheEventTheyWereInvitedTo(t *testing.T) {
	f := newFixture(t)
	cal := f.calOf(t, "n-alice", store.KindPersonal)
	other := newInput(cal)
	other.Title, other.Start, other.End = "Private chat", "2026-11-03T09:00:00+07:00", "2026-11-03T10:00:00+07:00"
	plain := f.create(t, "n-alice", other)
	d := f.create(t, "n-alice", withGuests(cal, "n-bob"))

	// The graph holds what the spec shapes: the OA under the calendar's, the UA
	// under the policy class, read for guests, the guest in the UA.
	assert.Contains(t, f.w.Created(), "OA "+eventOA(d.EventID))
	assert.Contains(t, f.w.Created(), "UA "+eventUA(d.EventID))
	assert.Contains(t, f.w.Log, "assign "+eventOA(d.EventID)+">"+ngac.CalendarOAName(ngac.CalendarID(cal)))
	assert.Contains(t, f.w.Log, "assign "+eventUA(d.EventID)+">"+ngac.PCName(ws))
	assert.Contains(t, f.w.Log, "associate "+eventUA(d.EventID)+">"+eventOA(d.EventID)+" [read]")
	assert.Contains(t, f.w.Log, "assign "+ngac.UserNodeName("n-bob")+">"+eventUA(d.EventID))
	assert.False(t, f.logHas("create OA "+eventOA(plain.EventID)), "a plain event still has no node")
	assert.Equal(t, 1, d.GuestCount)
	require.Len(t, d.Attendees, 2)
	assert.True(t, d.Attendees[0].IsOrganizer)
	assert.Equal(t, "accepted", d.Attendees[0].Response)
	assert.Equal(t, "needs_action", d.Attendees[1].Response)

	// Allow: the guest reads it, sees it on their own listing although alice's
	// calendar is not on their list, and cannot edit it.
	got, err := f.svc.GetEvent(callerCtx("n-bob"), ws, d.EventID, "")
	require.NoError(t, err)
	assert.Equal(t, "Standup", got.Title)
	assert.False(t, got.CanEdit)
	assert.Equal(t, "needs_action", got.MyResponse)
	listed := f.list(t, "n-bob")
	require.Len(t, listed, 1)
	assert.Equal(t, d.EventID, listed[0].EventID)
	assert.False(t, listed[0].BusyOnly)
	assert.Equal(t, "Alice", listed[0].Organizer.DisplayName)
	assert.Len(t, f.list(t, "n-bob", f.calOf(t, "n-bob", store.KindPersonal)), 1, "also when other calendars are named")

	// Deny: nothing else of the host's calendar; no edit, move or delete.
	_, err = f.svc.GetEvent(callerCtx("n-bob"), ws, plain.EventID, "")
	requireKind(t, err, ErrAccessDenied)
	title := "Hijacked"
	_, err = f.svc.UpdateEvent(callerCtx("n-bob"), ws, d.EventID, ScopeQuery{}, EventPatch{Version: 1, Title: &title})
	requireKind(t, err, ErrAccessDenied)
	requireKind(t, f.svc.DeleteEvent(callerCtx("n-bob"), ws, d.EventID, ScopeQuery{}, 0), ErrAccessDenied)
	_, err = f.svc.CreateEvent(callerCtx("n-bob"), ws, newInput(cal))
	requireKind(t, err, ErrAccessDenied)
	_, err = f.svc.GetEvent(callerCtx("n-carol"), ws, d.EventID, "")
	requireKind(t, err, ErrAccessDenied)
	assert.Empty(t, f.list(t, "n-carol"))
}

func TestOnlyPeopleOfTheWorkspaceCanBeInvited(t *testing.T) {
	f := newFixture(t)
	cal := f.calOf(t, "n-alice", store.KindPersonal)
	before := f.w.Mutations()

	_, err := f.svc.CreateEvent(callerCtx("n-alice"), ws, withGuests(cal, "n-bob", "n-gus"))
	requireKind(t, err, ErrAccessDenied)
	_, err = f.svc.CreateEvent(callerCtx("n-alice"), ws, withGuests(cal, "gus@example.com"))
	requireKind(t, err, ErrInvalidInput)
	_, err = f.svc.CreateEvent(callerCtx("n-alice"), ws, withGuests(cal, "n-nobody"))
	requireKind(t, err, ErrAccessDenied)
	assert.Empty(t, f.st.events, "nothing was stored")
	assert.Equal(t, before, f.w.Mutations(), "and nothing written to the graph")

	// The same holds when a guest is added to an event that exists.
	d := f.create(t, "n-alice", newInput(cal))
	_, err = f.svc.AddGuests(callerCtx("n-alice"), ws, d.EventID, GuestsInput{Attendees: []string{"n-gus"}})
	requireKind(t, err, ErrAccessDenied)
	_, err = f.svc.UpdateEvent(callerCtx("n-alice"), ws, d.EventID, ScopeQuery{}, EventPatch{Version: 1, Attendees: &[]string{"n-gus"}})
	requireKind(t, err, ErrAccessDenied)
	assert.Empty(t, f.st.events[d.EventID].NgacOAID)
}

func TestGuestsInviteOthersOnlyWhenTheHostAllowsIt(t *testing.T) {
	f := newFixture(t)
	cal := f.calOf(t, "n-alice", store.KindPersonal)
	d := f.create(t, "n-alice", withGuests(cal, "n-bob"))

	// Deny: a guest the host did not allow to invite.
	_, err := f.svc.AddGuests(callerCtx("n-bob"), ws, d.EventID, GuestsInput{Attendees: []string{"n-carol"}})
	requireKind(t, err, ErrAccessDenied)
	_, err = f.svc.AddGuests(callerCtx("n-carol"), ws, d.EventID, GuestsInput{Attendees: []string{"n-carol"}})
	requireKind(t, err, ErrAccessDenied)

	// The host allows it: the association gains invite in the graph.
	on := true
	_, err = f.svc.UpdateEvent(callerCtx("n-alice"), ws, d.EventID, ScopeQuery{}, EventPatch{Version: 1, GuestsCanInvite: &on})
	require.NoError(t, err)
	assert.Equal(t, []string{ngac.OpRead, ngac.OpInvite}, f.w.Assocs[[2]string{f.st.events[d.EventID].GuestsUAID, f.st.events[d.EventID].NgacOAID}])

	// Allow: the guest invites a colleague, who may read and not edit.
	got, err := f.svc.AddGuests(callerCtx("n-bob"), ws, d.EventID, GuestsInput{Attendees: []string{"n-carol"}})
	require.NoError(t, err)
	assert.Equal(t, 2, got.GuestCount)
	read, err := f.svc.GetEvent(callerCtx("n-carol"), ws, d.EventID, "")
	require.NoError(t, err)
	assert.False(t, read.CanEdit)
	title := "x"
	_, err = f.svc.UpdateEvent(callerCtx("n-carol"), ws, d.EventID, ScopeQuery{}, EventPatch{Version: 2, Title: &title})
	requireKind(t, err, ErrAccessDenied)

	// Deny: a guest still invites only people of the workspace.
	_, err = f.svc.AddGuests(callerCtx("n-bob"), ws, d.EventID, GuestsInput{Attendees: []string{"n-gus"}})
	requireKind(t, err, ErrAccessDenied)
	_, err = f.svc.AddGuests(callerCtx("n-bob"), ws, d.EventID, GuestsInput{Attendees: []string{"x@example.com"}})
	requireKind(t, err, ErrInvalidInput)
	_, err = f.svc.AddGuests(callerCtx("n-bob"), ws, d.EventID, GuestsInput{})
	requireKind(t, err, ErrInvalidInput)

	// The host turns it off: the next invitation by a guest is refused again.
	off := false
	_, err = f.svc.UpdateEvent(callerCtx("n-alice"), ws, d.EventID, ScopeQuery{}, EventPatch{Version: 2, GuestsCanInvite: &off})
	require.NoError(t, err)
	assert.Equal(t, []string{ngac.OpRead}, f.w.Assocs[[2]string{f.st.events[d.EventID].GuestsUAID, f.st.events[d.EventID].NgacOAID}])
	_, err = f.svc.AddGuests(callerCtx("n-bob"), ws, d.EventID, GuestsInput{Attendees: []string{"n-olga"}})
	requireKind(t, err, ErrAccessDenied)

	// A stale database flag alone does not grant it either: the graph must agree.
	f.st.events[d.EventID].GuestsCanInvite = true
	_, err = f.svc.AddGuests(callerCtx("n-bob"), ws, d.EventID, GuestsInput{Attendees: []string{"n-olga"}})
	requireKind(t, err, ErrAccessDenied)
}

func TestAHostAddsGuestsToAnExistingEvent(t *testing.T) {
	f := newFixture(t)
	cal := f.calOf(t, "n-alice", store.KindPersonal)
	d := f.create(t, "n-alice", newInput(cal))
	assert.False(t, f.logHas("create OA "+eventOA(d.EventID)))

	got, err := f.svc.AddGuests(callerCtx("n-alice"), ws, d.EventID, GuestsInput{Attendees: []string{"n-bob", "n-bob", "n-alice"}})
	require.NoError(t, err)
	assert.Equal(t, 1, got.GuestCount, "a person is invited once, and the organizer is not a guest")
	assert.True(t, f.logHas("create OA "+eventOA(d.EventID)))
	assert.Equal(t, f.w.NodeNamed("OA", eventOA(d.EventID)), f.st.events[d.EventID].NgacOAID)

	// The second guest joins the existing nodes.
	nodes := len(f.w.Created())
	_, err = f.svc.AddGuests(callerCtx("n-alice"), ws, d.EventID, GuestsInput{Attendees: []string{"n-carol"}})
	require.NoError(t, err)
	assert.Len(t, f.w.Created(), nodes, "no second pair of nodes")
	_, err = f.svc.GetEvent(callerCtx("n-carol"), ws, d.EventID, "")
	require.NoError(t, err)

	// Deny: a colleague who is not a guest cannot invite.
	_, err = f.svc.AddGuests(callerCtx("n-olga"), ws, d.EventID, GuestsInput{Attendees: []string{"n-olga"}})
	requireKind(t, err, ErrAccessDenied)
	_, err = f.svc.AddGuests(callerCtx("n-alice"), ws, "missing", GuestsInput{Attendees: []string{"n-bob"}})
	requireKind(t, err, ErrNotFound)
}

func TestRemovedGuestReadsNothing(t *testing.T) {
	f := newFixture(t)
	cal := f.calOf(t, "n-alice", store.KindPersonal)
	d := f.create(t, "n-alice", withGuests(cal, "n-bob", "n-carol"))
	_, err := f.svc.GetEvent(callerCtx("n-bob"), ws, d.EventID, "")
	require.NoError(t, err)

	// The list sent back is the guests wanted: Bob is not on it.
	got, err := f.svc.UpdateEvent(callerCtx("n-alice"), ws, d.EventID, ScopeQuery{}, EventPatch{Version: 1, Attendees: &[]string{"n-carol"}})
	require.NoError(t, err)
	assert.Equal(t, 1, got.GuestCount)
	assert.True(t, f.logHas("unassign "+ngac.UserNodeName("n-bob")+">"+eventUA(d.EventID)))

	_, err = f.svc.GetEvent(callerCtx("n-bob"), ws, d.EventID, "")
	requireKind(t, err, ErrAccessDenied)
	assert.Empty(t, f.list(t, "n-bob"), "and the event leaves their listing")
	_, err = f.svc.Rsvp(callerCtx("n-bob"), ws, d.EventID, RsvpInput{Response: "accepted"})
	requireKind(t, err, ErrAccessDenied)
	_, err = f.svc.GetEvent(callerCtx("n-carol"), ws, d.EventID, "")
	require.NoError(t, err, "the others keep it")

	// The last guest going takes the event's nodes with them.
	_, err = f.svc.UpdateEvent(callerCtx("n-alice"), ws, d.EventID, ScopeQuery{}, EventPatch{Version: got.Version, Attendees: &[]string{}})
	require.NoError(t, err)
	assert.True(t, f.logHas("delete "+eventOA(d.EventID)))
	assert.True(t, f.logHas("delete "+eventUA(d.EventID)))
	assert.Empty(t, f.st.events[d.EventID].NgacOAID)
	_, err = f.svc.GetEvent(callerCtx("n-carol"), ws, d.EventID, "")
	requireKind(t, err, ErrAccessDenied)
	again, err := f.svc.GetEvent(callerCtx("n-alice"), ws, d.EventID, "")
	require.NoError(t, err)
	require.Len(t, again.Attendees, 1, "the organizer stays")
}

func TestACallKeepsTheEventsNodes(t *testing.T) {
	f := newFixture(t)
	cal := f.calOf(t, "n-alice", store.KindPersonal)
	in := newInput(cal)
	in.HasCall = true
	d := f.create(t, "n-alice", in)
	assert.NotEmpty(t, f.st.events[d.EventID].NgacOAID, "a call needs the event's OA even with no guest")

	off := false
	_, err := f.svc.UpdateEvent(callerCtx("n-alice"), ws, d.EventID, ScopeQuery{}, EventPatch{Version: 1, HasCall: &off})
	require.NoError(t, err)
	assert.Empty(t, f.st.events[d.EventID].NgacOAID, "no guest and no call: no node")
	on := true
	_, err = f.svc.UpdateEvent(callerCtx("n-alice"), ws, d.EventID, ScopeQuery{}, EventPatch{Version: 2, HasCall: &on})
	require.NoError(t, err)
	assert.NotEmpty(t, f.st.events[d.EventID].NgacOAID)
}

func TestInvitingAGroupExpandsItsVerifiedMembers(t *testing.T) {
	f := newFixture(t)
	cal := f.calOf(t, "n-alice", store.KindPersonal)
	// Carol is on the group's cached member list but not in the graph.
	f.st.members["ch-team"] = []string{"n-alice", "n-olga", "n-carol"}

	in := newInput(cal)
	in.InviteChannelID = "ch-team"
	d := f.create(t, "n-alice", in)
	assert.Equal(t, 1, d.GuestCount, "Olga only: Alice organizes, Carol does not read the group")
	_, err := f.svc.GetEvent(callerCtx("n-olga"), ws, d.EventID, "")
	require.NoError(t, err)
	_, err = f.svc.GetEvent(callerCtx("n-carol"), ws, d.EventID, "")
	requireKind(t, err, ErrAccessDenied)

	// Deny: someone who cannot read the group cannot invite it.
	bobCal := f.calOf(t, "n-bob", store.KindPersonal)
	in = newInput(bobCal)
	in.InviteChannelID = "ch-team"
	_, err = f.svc.CreateEvent(callerCtx("n-bob"), ws, in)
	requireKind(t, err, ErrAccessDenied)
	in.InviteChannelID = "ch-nowhere"
	_, err = f.svc.CreateEvent(callerCtx("n-bob"), ws, in)
	requireKind(t, err, ErrNotFound)
	_, err = f.svc.AddGuests(callerCtx("n-alice"), ws, d.EventID, GuestsInput{InviteChannelID: "ch-nowhere"})
	requireKind(t, err, ErrNotFound)
}

func TestRsvpNeedsAGuestWhoCanStillReadTheEvent(t *testing.T) {
	f := newFixture(t)
	cal := f.calOf(t, "n-alice", store.KindPersonal)
	d := f.create(t, "n-alice", withGuests(cal, "n-bob"))

	got, err := f.svc.Rsvp(callerCtx("n-bob"), ws, d.EventID, RsvpInput{Response: "tentative", Comment: "Maybe after lunch"})
	require.NoError(t, err)
	assert.Equal(t, "tentative", got.MyResponse)

	// The host sees the answer and the note beside the guest's name.
	seen, err := f.svc.GetEvent(callerCtx("n-alice"), ws, d.EventID, "")
	require.NoError(t, err)
	require.Len(t, seen.Attendees, 2)
	assert.Equal(t, "tentative", seen.Attendees[1].Response)
	assert.Equal(t, "Maybe after lunch", seen.Attendees[1].Comment)

	// The host's screen is told.
	last := f.rec.Events()[len(f.rec.Events())-1]
	assert.Contains(t, last.UserNodeIDs, "n-alice")
	assert.Equal(t, []string{d.EventID}, last.IDs)

	// Deny: not a guest, not even a reader of the calendar's event; outsiders; bad input.
	_, err = f.svc.Rsvp(callerCtx("n-carol"), ws, d.EventID, RsvpInput{Response: "accepted"})
	requireKind(t, err, ErrAccessDenied)
	_, err = f.svc.Rsvp(callerCtx("n-gus"), ws, d.EventID, RsvpInput{Response: "accepted"})
	requireKind(t, err, ErrAccessDenied)
	_, err = f.svc.Rsvp(callerCtx("n-bob"), ws, d.EventID, RsvpInput{Response: "yes"})
	requireKind(t, err, ErrInvalidInput)
	_, err = f.svc.Rsvp(callerCtx("n-bob"), ws, d.EventID, RsvpInput{Response: "accepted", Comment: strings.Repeat("x", 501)})
	requireKind(t, err, ErrInvalidInput)
	_, err = f.svc.Rsvp(callerCtx("n-bob"), ws, "missing", RsvpInput{Response: "accepted"})
	requireKind(t, err, ErrNotFound)
	still, err := f.svc.GetEvent(callerCtx("n-bob"), ws, d.EventID, "")
	require.NoError(t, err)
	assert.Equal(t, "tentative", still.MyResponse, "refused answers changed nothing")

	// A guest whose access was taken away by the graph alone cannot answer.
	delete(f.w.Parents, "n-bob")
	f.w.Assign("n-bob", "ua-members")
	_, err = f.svc.Rsvp(callerCtx("n-bob"), ws, d.EventID, RsvpInput{Response: "declined"})
	requireKind(t, err, ErrAccessDenied)
}

func TestRsvpToOneOccurrenceOrTheSeries(t *testing.T) {
	f := newFixture(t)
	cal := f.calOf(t, "n-alice", store.KindPersonal)
	in := weekly(cal)
	in.Attendees = []string{"n-bob"}
	d := f.create(t, "n-alice", in)
	ctx := callerCtx("n-bob")
	occ := func() []*OccurrenceView { return f.list(t, "n-bob") }
	responses := func() []string {
		var out []string
		for _, o := range occ() {
			out = append(out, o.MyResponse)
		}
		return out
	}
	second := occ()[1].OccurrenceStart

	_, err := f.svc.Rsvp(ctx, ws, d.EventID, RsvpInput{Response: "declined", Comment: "Away", Scope: ScopeThis, Occurrence: second})
	require.NoError(t, err)
	assert.Equal(t, []string{"needs_action", "declined", "needs_action", "needs_action", "needs_action"}, responses(),
		"a declined occurrence is still listed, marked declined")
	one, err := f.svc.GetEvent(callerCtx("n-alice"), ws, d.EventID, second)
	require.NoError(t, err)
	assert.Equal(t, "declined", one.Attendees[1].Response)
	assert.Equal(t, "Away", one.Attendees[1].Comment)
	first, err := f.svc.GetEvent(callerCtx("n-alice"), ws, d.EventID, "")
	require.NoError(t, err)
	assert.Equal(t, "needs_action", first.Attendees[1].Response, "the other occurrences are untouched")

	// All: the series answer replaces the single-occurrence ones.
	_, err = f.svc.Rsvp(ctx, ws, d.EventID, RsvpInput{Response: "accepted", Scope: ScopeAll})
	require.NoError(t, err)
	assert.Equal(t, []string{"accepted", "accepted", "accepted", "accepted", "accepted"}, responses())

	// Validation of the scope.
	_, err = f.svc.Rsvp(ctx, ws, d.EventID, RsvpInput{Response: "accepted"})
	requireKind(t, err, ErrInvalidInput)
	_, err = f.svc.Rsvp(ctx, ws, d.EventID, RsvpInput{Response: "accepted", Scope: ScopeThis})
	requireKind(t, err, ErrInvalidInput)
	_, err = f.svc.Rsvp(ctx, ws, d.EventID, RsvpInput{Response: "accepted", Scope: ScopeFollowing, Occurrence: second})
	requireKind(t, err, ErrInvalidInput)
	_, err = f.svc.Rsvp(ctx, ws, d.EventID, RsvpInput{Response: "accepted", Scope: ScopeThis, Occurrence: "2026-11-03T09:00:00+07:00"})
	requireKind(t, err, ErrNotFound)
	// Deny: a colleague who is not a guest answers nothing, whatever the scope.
	_, err = f.svc.Rsvp(callerCtx("n-carol"), ws, d.EventID, RsvpInput{Response: "declined", Scope: ScopeThis, Occurrence: second})
	requireKind(t, err, ErrAccessDenied)
}

func TestTheGuestListIsHiddenWhenTheHostSaysSo(t *testing.T) {
	f := newFixture(t)
	cal := f.calOf(t, "n-alice", store.KindPersonal)
	in := withGuests(cal, "n-bob", "n-carol")
	in.GuestsCanSeeList = false
	d := f.create(t, "n-alice", in)

	names := func(v *EventDetailView) []string {
		var out []string
		for _, a := range v.Attendees {
			out = append(out, a.Person.DisplayName)
		}
		return out
	}
	host, err := f.svc.GetEvent(callerCtx("n-alice"), ws, d.EventID, "")
	require.NoError(t, err)
	assert.Equal(t, []string{"Alice", "Bob", "Carol"}, names(host))
	guest, err := f.svc.GetEvent(callerCtx("n-bob"), ws, d.EventID, "")
	require.NoError(t, err)
	assert.Equal(t, []string{"Alice", "Bob"}, names(guest), "a guest sees themselves and the organizer")

	on := true
	_, err = f.svc.UpdateEvent(callerCtx("n-alice"), ws, d.EventID, ScopeQuery{}, EventPatch{Version: 1, GuestsCanSeeList: &on})
	require.NoError(t, err)
	guest, err = f.svc.GetEvent(callerCtx("n-bob"), ws, d.EventID, "")
	require.NoError(t, err)
	assert.Equal(t, []string{"Alice", "Bob", "Carol"}, names(guest))
}

func TestAnAbsentGuestDeclinesAutomatically(t *testing.T) {
	f := newFixture(t)
	away := newInput(f.calOf(t, "n-carol", store.KindPersonal))
	away.Title, away.Kind, away.AllDay = "Holiday", KindOOO, true
	away.Start, away.End = "2026-11-02", "2026-11-04"
	f.create(t, "n-carol", away)

	cal := f.calOf(t, "n-alice", store.KindPersonal)
	d := f.create(t, "n-alice", withGuests(cal, "n-bob", "n-carol"))
	host, err := f.svc.GetEvent(callerCtx("n-alice"), ws, d.EventID, "")
	require.NoError(t, err)
	byName := map[string]AttendeeView{}
	for _, a := range host.Attendees {
		byName[a.Person.DisplayName] = a
	}
	assert.Equal(t, "declined", byName["Carol"].Response)
	assert.Equal(t, "Đang vắng mặt", byName["Carol"].Comment)
	assert.Equal(t, "needs_action", byName["Bob"].Response)
}

func TestFollowingGivesTheTailItsOwnGuestNodes(t *testing.T) {
	f := newFixture(t)
	cal := f.calOf(t, "n-alice", store.KindPersonal)
	in := weekly(cal)
	in.Attendees = []string{"n-bob"}
	d := f.create(t, "n-alice", in)
	ctx := callerCtx("n-alice")
	third := f.list(t, "n-alice", cal)[2].OccurrenceStart

	// An answer to a later occurrence follows the tail while its time is unchanged.
	_, err := f.svc.Rsvp(callerCtx("n-bob"), ws, d.EventID, RsvpInput{Response: "declined", Scope: ScopeThis, Occurrence: third})
	require.NoError(t, err)

	title := "Afternoon sync"
	tail, err := f.svc.UpdateEvent(ctx, ws, d.EventID, ScopeQuery{Scope: ScopeFollowing, Occurrence: third}, EventPatch{Version: 1, Title: &title})
	require.NoError(t, err)
	require.NotEqual(t, d.EventID, tail.EventID)
	assert.Equal(t, 1, tail.GuestCount)
	assert.True(t, f.logHas("create OA "+eventOA(tail.EventID)))
	assert.Contains(t, f.w.Log, "assign "+ngac.UserNodeName("n-bob")+">"+eventUA(tail.EventID))
	assert.Contains(t, f.w.Log, "associate "+eventUA(tail.EventID)+">"+eventOA(tail.EventID)+" [read]")

	// Allow: the guest reads both halves. Deny: a stranger reads neither.
	for _, id := range []string{d.EventID, tail.EventID} {
		_, err = f.svc.GetEvent(callerCtx("n-bob"), ws, id, "")
		require.NoError(t, err)
		_, err = f.svc.GetEvent(callerCtx("n-carol"), ws, id, "")
		requireKind(t, err, ErrAccessDenied)
	}
	listed := f.list(t, "n-bob")
	require.Len(t, listed, 5)
	assert.Equal(t, "declined", listed[2].MyResponse, "the answer to the third occurrence moved to the tail")
	assert.Equal(t, "Afternoon sync", listed[2].Title)

	// Removing the tail's guest leaves the earlier half alone.
	_, err = f.svc.UpdateEvent(ctx, ws, tail.EventID, ScopeQuery{Scope: ScopeAll}, EventPatch{Version: tail.Version, Attendees: &[]string{}})
	require.NoError(t, err)
	_, err = f.svc.GetEvent(callerCtx("n-bob"), ws, tail.EventID, "")
	requireKind(t, err, ErrAccessDenied)
	_, err = f.svc.GetEvent(callerCtx("n-bob"), ws, d.EventID, "")
	require.NoError(t, err)
}

func TestGuestChangesBelongToTheWholeSeries(t *testing.T) {
	f := newFixture(t)
	cal := f.calOf(t, "n-alice", store.KindPersonal)
	d := f.create(t, "n-alice", weekly(cal))
	second := f.list(t, "n-alice", cal)[1].OccurrenceStart
	_, err := f.svc.UpdateEvent(callerCtx("n-alice"), ws, d.EventID, ScopeQuery{Scope: ScopeThis, Occurrence: second},
		EventPatch{Version: 1, Attendees: &[]string{"n-bob"}})
	requireKind(t, err, ErrInvalidInput)
	assert.Empty(t, f.st.events[d.EventID].NgacOAID)
}

func TestDeletingAnEventWithGuestsRefusesThemAtOnce(t *testing.T) {
	f := newFixture(t)
	cal := f.calOf(t, "n-alice", store.KindPersonal)
	d := f.create(t, "n-alice", withGuests(cal, "n-bob"))
	oa := f.st.events[d.EventID].NgacOAID

	require.NoError(t, f.svc.DeleteEvent(callerCtx("n-alice"), ws, d.EventID, ScopeQuery{}, 0))
	assert.False(t, f.w.Allowed("n-bob", oa, ngac.OpRead), "the guest's access went with the nodes")
	_, err := f.svc.GetEvent(callerCtx("n-bob"), ws, d.EventID, "")
	requireKind(t, err, ErrNotFound)
	assert.Empty(t, f.list(t, "n-bob"))
	frames := f.rec.Events()
	assert.Contains(t, frames[len(frames)-1].UserNodeIDs, "n-bob", "the guest's open screen is told")
}

func TestAFailedInvitationLeavesNothingBehind(t *testing.T) {
	f := newFixture(t)
	cal := f.calOf(t, "n-alice", store.KindPersonal)
	f.w.FailAt = f.w.Mutations() + 4 // somewhere inside wiring the event's nodes
	_, err := f.svc.CreateEvent(callerCtx("n-alice"), ws, withGuests(cal, "n-bob"))
	require.Error(t, err)
	assert.Empty(t, f.st.events, "the event is not left without the guests it was made for")
	assert.False(t, f.logHas("create OA CalEv_") && !f.logHas("delete CalEv_"), "nodes made are removed again")
}
