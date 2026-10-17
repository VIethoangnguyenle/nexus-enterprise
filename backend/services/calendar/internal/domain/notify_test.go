package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/pkg/realtime"
	"ngac-platform/services/calendar/internal/events"
	"ngac-platform/services/calendar/internal/store"
)

func TestInvitingTellsEachGuestButNotTheHost(t *testing.T) {
	f := newFixture(t)
	rec := f.noticing()
	cal := f.calOf(t, "n-alice", store.KindPersonal)
	d := f.create(t, "n-alice", withGuests(cal, "n-bob", "n-carol"))

	got := ofType(rec, events.TypeInvited)
	require.Len(t, got, 1)
	n := got[0]
	assert.ElementsMatch(t, []events.Recipient{{NodeID: "n-bob", UserID: "user-n-bob"}, {NodeID: "n-carol", UserID: "user-n-carol"}}, n.Recipients)
	assert.Equal(t, d.EventID, n.EventID)
	assert.Equal(t, "Standup", n.Title)
	assert.Equal(t, "2026-11-02T02:00:00Z", n.Start)
	assert.Equal(t, "2026-11-02T03:00:00Z", n.End)
	assert.Equal(t, "n-alice", n.ActorNodeID)
	assert.Equal(t, "user-n-alice", n.ActorUserID)
	assert.Equal(t, ws, n.WorkspaceID)
	assert.Equal(t, f.w.NodeNamed("OA", eventOA(d.EventID)), n.OAID)

	// Later guests are told on their own, and the ones already invited are not told again.
	list := []string{"n-bob", "n-carol", "n-olga"}
	_, err := f.svc.UpdateEvent(callerCtx("n-alice"), ws, d.EventID, ScopeQuery{}, EventPatch{Version: 1, Attendees: &list})
	require.NoError(t, err)
	got = ofType(rec, events.TypeInvited)
	require.Len(t, got, 2)
	assert.Equal(t, []events.Recipient{{NodeID: "n-olga", UserID: "user-n-olga"}}, got[1].Recipients)
	assert.Empty(t, ofType(rec, events.TypeUpdated), "adding a guest does not move the event")

	_, err = f.svc.AddGuests(callerCtx("n-alice"), ws, d.EventID, GuestsInput{Attendees: []string{"n-bob"}})
	require.NoError(t, err)
	assert.Len(t, ofType(rec, events.TypeInvited), 2, "bob was invited already and is not told again")
}

func TestAnEventWithNoGuestsNotifiesNobody(t *testing.T) {
	f := newFixture(t)
	rec := f.noticing()
	cal := f.calOf(t, "n-alice", store.KindPersonal)
	d := f.create(t, "n-alice", newInput(cal))
	title := "Renamed"
	_, err := f.svc.UpdateEvent(callerCtx("n-alice"), ws, d.EventID, ScopeQuery{}, EventPatch{Version: 1, Title: &title})
	require.NoError(t, err)
	require.NoError(t, f.svc.DeleteEvent(callerCtx("n-alice"), ws, d.EventID, ScopeQuery{}, 0))
	assert.Empty(t, rec.Notices())
}

func TestMovingAnEventTellsItsGuestsAndAnEditOfTheTitleDoesNot(t *testing.T) {
	f := newFixture(t)
	rec := f.noticing()
	cal := f.calOf(t, "n-alice", store.KindPersonal)
	d := f.create(t, "n-alice", withGuests(cal, "n-bob", "n-carol"))

	title := "Renamed"
	_, err := f.svc.UpdateEvent(callerCtx("n-alice"), ws, d.EventID, ScopeQuery{}, EventPatch{Version: 1, Title: &title})
	require.NoError(t, err)
	assert.Empty(t, ofType(rec, events.TypeUpdated), "a rename is not a move")

	start, end := "2026-11-02T10:00:00+07:00", "2026-11-02T11:00:00+07:00"
	_, err = f.svc.UpdateEvent(callerCtx("n-alice"), ws, d.EventID, ScopeQuery{}, EventPatch{Version: 2, Start: &start, End: &end})
	require.NoError(t, err)
	got := ofType(rec, events.TypeUpdated)
	require.Len(t, got, 1)
	assert.ElementsMatch(t, []string{"n-bob", "n-carol"}, remindedNodes(got))
	assert.Equal(t, "2026-11-02T03:00:00Z", got[0].Start)
	assert.Empty(t, got[0].OccurrenceStart)

	place := "Room 2"
	_, err = f.svc.UpdateEvent(callerCtx("n-alice"), ws, d.EventID, ScopeQuery{}, EventPatch{Version: 3, Location: &place})
	require.NoError(t, err)
	assert.Len(t, ofType(rec, events.TypeUpdated), 2, "a new place is a change guests need to hear of")
}

func TestMovingOneOccurrenceNamesIt(t *testing.T) {
	f := newFixture(t)
	rec := f.noticing()
	cal := f.calOf(t, "n-alice", store.KindPersonal)
	in := weekly(cal)
	in.Attendees = []string{"n-bob"}
	d := f.create(t, "n-alice", in)

	start, end := "2026-11-09T10:00:00+07:00", "2026-11-09T11:00:00+07:00"
	_, err := f.svc.UpdateEvent(callerCtx("n-alice"), ws, d.EventID, ScopeQuery{Scope: ScopeThis, Occurrence: "2026-11-09T09:00:00+07:00"}, EventPatch{Version: 1, Start: &start, End: &end})
	require.NoError(t, err)
	got := ofType(rec, events.TypeUpdated)
	require.Len(t, got, 1)
	assert.Equal(t, d.EventID, got[0].EventID)
	assert.Equal(t, "2026-11-09T02:00:00Z", got[0].OccurrenceStart, "the occurrence that moved, where it was")
	assert.Equal(t, "2026-11-09T03:00:00Z", got[0].Start, "and where it is now")
	assert.Equal(t, []string{"n-bob"}, remindedNodes(got))
}

func TestCancellingTellsTheGuestsIncludingThoseTakenOff(t *testing.T) {
	f := newFixture(t)
	rec := f.noticing()
	cal := f.calOf(t, "n-alice", store.KindPersonal)
	d := f.create(t, "n-alice", withGuests(cal, "n-bob", "n-carol"))

	keep := []string{"n-bob"}
	_, err := f.svc.UpdateEvent(callerCtx("n-alice"), ws, d.EventID, ScopeQuery{}, EventPatch{Version: 1, Attendees: &keep})
	require.NoError(t, err)
	got := ofType(rec, events.TypeCancelled)
	require.Len(t, got, 1)
	assert.Equal(t, []string{"n-carol"}, remindedNodes(got), "carol is told she is off the event")

	require.NoError(t, f.svc.DeleteEvent(callerCtx("n-alice"), ws, d.EventID, ScopeQuery{}, 0))
	got = ofType(rec, events.TypeCancelled)
	require.Len(t, got, 2)
	assert.Equal(t, []string{"n-bob"}, remindedNodes(got[1:]), "only bob is left to be told, never the host")
	assert.Equal(t, "Standup", got[1].Title, "the title survives the event")
}

func TestCancellingOneOccurrenceNamesIt(t *testing.T) {
	f := newFixture(t)
	rec := f.noticing()
	cal := f.calOf(t, "n-alice", store.KindPersonal)
	in := weekly(cal)
	in.Attendees = []string{"n-bob"}
	d := f.create(t, "n-alice", in)
	require.NoError(t, f.svc.DeleteEvent(callerCtx("n-alice"), ws, d.EventID, ScopeQuery{Scope: ScopeThis, Occurrence: "2026-11-09T09:00:00+07:00"}, 0))
	got := ofType(rec, events.TypeCancelled)
	require.Len(t, got, 1)
	assert.Equal(t, "2026-11-09T02:00:00Z", got[0].OccurrenceStart)
}

func TestAnAnswerTellsTheHostWithItsWords(t *testing.T) {
	f := newFixture(t)
	rec := f.noticing()
	cal := f.calOf(t, "n-alice", store.KindPersonal)
	d := f.create(t, "n-alice", withGuests(cal, "n-bob"))

	_, err := f.svc.Rsvp(callerCtx("n-bob"), ws, d.EventID, RsvpInput{Response: ResponseTentative, Comment: "maybe late"})
	require.NoError(t, err)
	got := ofType(rec, events.TypeRsvp)
	require.Len(t, got, 1)
	assert.Equal(t, []string{"n-alice"}, remindedNodes(got))
	assert.Equal(t, "tentative", got[0].Response)
	assert.Equal(t, "maybe late", got[0].Comment)
	assert.Equal(t, "n-bob", got[0].ActorNodeID)
	assert.Equal(t, "user-n-bob", got[0].ActorUserID)

	// A refused answer tells nobody.
	_, err = f.svc.Rsvp(callerCtx("n-carol"), ws, d.EventID, RsvpInput{Response: ResponseAccepted})
	requireKind(t, err, ErrAccessDenied)
	assert.Len(t, ofType(rec, events.TypeRsvp), 1)
}

func TestSharingACalendarTellsTheGrantee(t *testing.T) {
	f := newFixture(t)
	rec := f.noticing()
	cal := f.calOf(t, "n-alice", store.KindPersonal)
	_, err := f.svc.PutShare(callerCtx("n-alice"), ws, cal, "n-bob", "view")
	require.NoError(t, err)
	got := ofType(rec, events.TypeShared)
	require.Len(t, got, 1)
	assert.Equal(t, []string{"n-bob"}, remindedNodes(got))
	assert.Equal(t, cal, got[0].CalendarID)
	assert.Empty(t, got[0].EventID)
	assert.Equal(t, "user-n-alice", got[0].ActorUserID)

	// A refused share tells nobody.
	_, err = f.svc.PutShare(callerCtx("n-carol"), ws, cal, "n-bob", "view")
	require.Error(t, err)
	assert.Len(t, ofType(rec, events.TypeShared), 1)
}

func TestAnEventOnAGroupCalendarIsAnnouncedWithItsGroup(t *testing.T) {
	f := newFixture(t)
	rec := f.noticing()
	cal := f.groupCal(t)
	d := f.create(t, "n-alice", newInput(cal))
	got := ofType(rec, events.TypeCreated)
	require.Len(t, got, 1)
	assert.Equal(t, "ch-team", got[0].ChannelID)
	assert.Equal(t, d.EventID, got[0].EventID)
	assert.Empty(t, got[0].Recipients)

	personal := f.calOf(t, "n-alice", store.KindPersonal)
	f.create(t, "n-alice", newInput(personal))
	assert.Len(t, ofType(rec, events.TypeCreated), 1, "only a group's calendar announces creation")
}

func TestSomeoneWithoutAnAccountIsSkippedAndNoNoticesWithoutAPublisher(t *testing.T) {
	f := newFixture(t)
	rec := f.noticing()
	f.st.people["n-bob"].UserID = ""
	cal := f.calOf(t, "n-alice", store.KindPersonal)
	f.create(t, "n-alice", withGuests(cal, "n-bob"))
	assert.Empty(t, rec.Notices(), "bob has no account to notify")

	g := newFixture(t) // no publisher set
	g.create(t, "n-alice", withGuests(g.calOf(t, "n-alice", store.KindPersonal), "n-bob"))
}

func TestRealtimeFramesNameTheirActor(t *testing.T) {
	f := newFixture(t)
	cal := f.calOf(t, "n-alice", store.KindPersonal)
	d := f.create(t, "n-alice", withGuests(cal, "n-bob"))
	_, err := f.svc.Rsvp(callerCtx("n-bob"), ws, d.EventID, RsvpInput{Response: ResponseAccepted})
	require.NoError(t, err)
	require.NoError(t, f.svc.DeleteEvent(callerCtx("n-alice"), ws, d.EventID, ScopeQuery{}, 0))

	frames := f.rec.Events()
	require.NotEmpty(t, frames)
	for _, e := range frames {
		assert.Equal(t, realtime.DomainCalendar, e.Domain)
		assert.NotEmpty(t, e.ActorUserID, "the screen draws the other-person wash only when it knows the actor: %s %v", e.Kind, e.IDs)
		assert.Contains(t, e.IDs, d.EventID)
	}
}
