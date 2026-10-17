package domain

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/ngac"
	"ngac-platform/pkg/realtime"
	"ngac-platform/services/calendar/internal/store"
)

const (
	saigon     = "Asia/Ho_Chi_Minh"
	rangeStart = "2026-11-01T00:00:00+07:00"
	rangeEnd   = "2026-12-01T00:00:00+07:00"
)

// calOf returns the id of a person's calendar of a kind, creating it by listing.
func (f *fixture) calOf(t *testing.T, node, kind string) string {
	t.Helper()
	got, err := f.svc.ListCalendars(callerCtx(node), ws)
	require.NoError(t, err)
	require.NotNil(t, kinds(got)[kind])
	return kinds(got)[kind].ID
}

// groupCal creates the calendar of the "team" group conversation.
func (f *fixture) groupCal(t *testing.T) string {
	t.Helper()
	v, err := f.svc.EnsureChannelCalendar(callerCtx("n-alice"), ws, "ch-team")
	require.NoError(t, err)
	return v.ID
}

func newInput(cal string) EventInput {
	return EventInput{CalendarID: cal, Title: "Standup", TZID: saigon, Start: "2026-11-02T09:00:00+07:00", End: "2026-11-02T10:00:00+07:00",
		Kind: "event", Transparency: "busy", Visibility: "default", GuestsCanSeeList: true, Location: "Room 1"}
}

func weekly(cal string) EventInput {
	in := newInput(cal)
	rule := "FREQ=WEEKLY;BYDAY=MO"
	in.RRule = &rule
	return in
}

func (f *fixture) create(t *testing.T, node string, in EventInput) *EventDetailView {
	t.Helper()
	d, err := f.svc.CreateEvent(callerCtx(node), ws, in)
	require.NoError(t, err)
	return d
}

func (f *fixture) list(t *testing.T, node string, cals ...string) []*OccurrenceView {
	t.Helper()
	got, err := f.svc.ListEvents(callerCtx(node), ws, ListEventsQuery{From: rangeStart, To: rangeEnd, Calendars: cals})
	require.NoError(t, err)
	return got
}

func titles(os []*OccurrenceView) []string {
	var out []string
	for _, o := range os {
		out = append(out, o.Title+"@"+o.Start[:16])
	}
	return out
}

func sp(s string) *string { return &s }

func TestCreateReadAndListASingleEvent(t *testing.T) {
	f := newFixture(t)
	cal := f.calOf(t, "n-alice", store.KindPersonal)
	before := f.w.Mutations()

	d := f.create(t, "n-alice", newInput(cal))
	assert.Equal(t, "Standup", d.Title)
	assert.Equal(t, "2026-11-02T09:00:00+07:00", d.Start)
	assert.Equal(t, saigon, d.TZID)
	assert.True(t, d.IsOrganizer)
	assert.True(t, d.CanEdit)
	assert.False(t, d.IsRecurring)
	assert.Equal(t, 1, d.Version)
	assert.Equal(t, []ReminderInput{{MinutesBefore: 10}}, d.Reminders, "the creator's default reminder applies when none is sent")
	require.Len(t, d.Attendees, 1)
	assert.True(t, d.Attendees[0].IsOrganizer)
	assert.Equal(t, "Alice", d.Organizer.DisplayName)
	assert.Equal(t, before, f.w.Mutations(), "a plain event has no graph node")

	got := f.list(t, "n-alice", cal)
	require.Len(t, got, 1)
	assert.Equal(t, d.EventID, got[0].EventID)
	assert.Equal(t, "Room 1", got[0].Location)
	assert.Equal(t, "accepted", got[0].MyResponse)
	assert.Len(t, f.list(t, "n-alice"), 1, "an unnamed list covers the calendars on the caller's list")

	one, err := f.svc.GetEvent(callerCtx("n-alice"), ws, d.EventID, "")
	require.NoError(t, err)
	assert.Equal(t, d.Title, one.Title)
}

func TestAllDayEventsUseDates(t *testing.T) {
	f := newFixture(t)
	cal := f.calOf(t, "n-alice", store.KindPersonal)
	in := newInput(cal)
	in.AllDay, in.Start, in.End = true, "2026-11-10", "2026-11-12"
	d := f.create(t, "n-alice", in)
	assert.Equal(t, "2026-11-10", d.Start)
	assert.Equal(t, "2026-11-12", d.End, "the end of an all-day event is exclusive")
	assert.Equal(t, "2026-11-10", d.OccurrenceStart)

	in.End = "2026-11-10"
	_, err := f.svc.CreateEvent(callerCtx("n-alice"), ws, in)
	requireKind(t, err, ErrInvalidInput)
}

func TestCreateValidation(t *testing.T) {
	f := newFixture(t)
	cal := f.calOf(t, "n-alice", store.KindPersonal)
	bad := func(_ string, mut func(*EventInput)) {
		in := newInput(cal)
		mut(&in)
		_, err := f.svc.CreateEvent(callerCtx("n-alice"), ws, in)
		requireKind(t, err, ErrInvalidInput)
	}
	bad("empty title", func(in *EventInput) { in.Title = "   " })
	bad("long title", func(in *EventInput) { in.Title = string(make([]rune, 201)) + "x" })
	bad("end before start", func(in *EventInput) { in.End = in.Start })
	bad("no tz", func(in *EventInput) { in.TZID = "" })
	bad("unknown tz", func(in *EventInput) { in.TZID = "Mars/Olympus" })
	bad("bad start", func(in *EventInput) { in.Start = "tomorrow" })
	bad("bad kind", func(in *EventInput) { in.Kind = "party" })
	bad("bad visibility", func(in *EventInput) { in.Visibility = "secret" })
	bad("bad rule", func(in *EventInput) { in.RRule = sp("FREQ=HOURLY") })
	bad("count too big", func(in *EventInput) { in.RRule = sp("FREQ=DAILY;COUNT=731") })
	bad("negative reminder", func(in *EventInput) { in.Reminders = &[]ReminderInput{{MinutesBefore: -1}} })
	bad("email guest", func(in *EventInput) { in.Attendees = []string{"bob@example.com"} })

	_, err := f.svc.CreateEvent(callerCtx("n-alice"), ws, EventInput{CalendarID: "missing"})
	requireKind(t, err, ErrNotFound)

	none := f.create(t, "n-alice", func() EventInput { in := newInput(cal); in.Reminders = &[]ReminderInput{}; return in }())
	assert.Empty(t, none.Reminders, "an empty list means no reminder")
}

func TestWritingNeedsWriteOnTheCalendar(t *testing.T) {
	f := newFixture(t)
	aliceCal := f.calOf(t, "n-alice", store.KindPersonal)
	wsCal := f.calOf(t, "n-alice", store.KindWorkspace)

	// Deny: a colleague with no share; an ordinary member on the workspace calendar.
	_, err := f.svc.CreateEvent(callerCtx("n-bob"), ws, newInput(aliceCal))
	requireKind(t, err, ErrAccessDenied)
	_, err = f.svc.CreateEvent(callerCtx("n-alice"), ws, newInput(wsCal))
	requireKind(t, err, ErrAccessDenied)
	// Allow: the owner of the workspace; a member of the group on its calendar.
	f.create(t, "n-olga", newInput(wsCal))
	f.create(t, "n-alice", newInput(f.groupCal(t)))
	// Deny: someone outside the group.
	_, err = f.svc.CreateEvent(callerCtx("n-bob"), ws, newInput(f.groupCal(t)))
	requireKind(t, err, ErrAccessDenied)
	// A person outside the workspace reaches nothing at all.
	_, err = f.svc.CreateEvent(callerCtx("n-gus"), ws, newInput(aliceCal))
	requireKind(t, err, ErrAccessDenied)
}

func TestViewShareReadsButCannotChange(t *testing.T) {
	f := newFixture(t)
	cal := f.calOf(t, "n-alice", store.KindPersonal)
	d := f.create(t, "n-alice", newInput(cal))
	_, err := f.svc.PutShare(callerCtx("n-alice"), ws, cal, "n-bob", ngac.CalendarShareView)
	require.NoError(t, err)

	got := f.list(t, "n-bob", cal)
	require.Len(t, got, 1)
	assert.Equal(t, "Standup", got[0].Title, "a view share sees every detail")
	assert.False(t, got[0].BusyOnly)
	assert.False(t, got[0].CanEdit)
	one, err := f.svc.GetEvent(callerCtx("n-bob"), ws, d.EventID, "")
	require.NoError(t, err)
	assert.False(t, one.CanEdit)

	// Deny: the share is read-only.
	title := "Hijacked"
	_, err = f.svc.UpdateEvent(callerCtx("n-bob"), ws, d.EventID, ScopeQuery{}, EventPatch{Version: 1, Title: &title})
	requireKind(t, err, ErrAccessDenied)
	requireKind(t, f.svc.DeleteEvent(callerCtx("n-bob"), ws, d.EventID, ScopeQuery{}, 0), ErrAccessDenied)
	_, err = f.svc.CreateEvent(callerCtx("n-bob"), ws, newInput(cal))
	requireKind(t, err, ErrAccessDenied)
	still, err := f.svc.GetEvent(callerCtx("n-alice"), ws, d.EventID, "")
	require.NoError(t, err)
	assert.Equal(t, "Standup", still.Title)
	assert.Equal(t, 1, still.Version)

	// Allow: an edit share changes it.
	_, err = f.svc.PutShare(callerCtx("n-alice"), ws, cal, "n-bob", ngac.CalendarShareEdit)
	require.NoError(t, err)
	changed, err := f.svc.UpdateEvent(callerCtx("n-bob"), ws, d.EventID, ScopeQuery{}, EventPatch{Version: 1, Title: &title})
	require.NoError(t, err)
	assert.Equal(t, "Hijacked", changed.Title)
	assert.True(t, changed.CanEdit)
	assert.Equal(t, "Alice", changed.Organizer.DisplayName, "the organizer stays the creator")
	require.NoError(t, f.svc.DeleteEvent(callerCtx("n-bob"), ws, d.EventID, ScopeQuery{}, 0))
	assert.Empty(t, f.list(t, "n-alice", cal))
}

func TestAStrangerReadsNoDetailsAndSeesBusyBlocks(t *testing.T) {
	f := newFixture(t)
	cal := f.calOf(t, "n-alice", store.KindPersonal)
	d := f.create(t, "n-alice", newInput(cal))
	free := newInput(cal)
	free.Title, free.Transparency, free.Start, free.End = "Lunch (free)", TransparencyFree, "2026-11-03T12:00:00+07:00", "2026-11-03T13:00:00+07:00"
	f.create(t, "n-alice", free)

	// Free/busy only: a colleague sees a block with no title and no place.
	got := f.list(t, "n-carol", cal)
	require.Len(t, got, 1, "free time is not busy time")
	assert.True(t, got[0].BusyOnly)
	assert.Equal(t, "Bận", got[0].Title)
	assert.Empty(t, got[0].Location)
	assert.False(t, got[0].CanEdit)
	assert.Nil(t, got[0].Organizer)
	assert.Equal(t, 0, got[0].GuestCount)
	assert.False(t, got[0].HasCall)

	// The details themselves are refused.
	_, err := f.svc.GetEvent(callerCtx("n-carol"), ws, d.EventID, "")
	requireKind(t, err, ErrAccessDenied)

	// Not on anyone's list: a group calendar the caller is not in.
	_, err = f.svc.ListEvents(callerCtx("n-bob"), ws, ListEventsQuery{From: rangeStart, To: rangeEnd, Calendars: []string{f.groupCal(t)}})
	requireKind(t, err, ErrAccessDenied)
	_, err = f.svc.GetEvent(callerCtx("n-gus"), ws, d.EventID, "")
	requireKind(t, err, ErrAccessDenied)
	_, err = f.svc.GetEvent(callerCtx("n-alice"), ws, "missing", "")
	requireKind(t, err, ErrNotFound)
}

func TestPrivateEventsAreHiddenFromShares(t *testing.T) {
	f := newFixture(t)
	cal := f.calOf(t, "n-alice", store.KindPersonal)
	in := newInput(cal)
	in.Visibility, in.Title = VisibilityPrivate, "Doctor"
	d := f.create(t, "n-alice", in)
	_, err := f.svc.PutShare(callerCtx("n-alice"), ws, cal, "n-bob", ngac.CalendarShareView)
	require.NoError(t, err)

	got := f.list(t, "n-bob", cal)
	require.Len(t, got, 1)
	assert.True(t, got[0].BusyOnly, "a view share does not read a private event")
	assert.Equal(t, "Bận", got[0].Title)
	_, err = f.svc.GetEvent(callerCtx("n-bob"), ws, d.EventID, "")
	requireKind(t, err, ErrAccessDenied)

	// An editor who is not the organizer cannot change what they cannot read.
	_, err = f.svc.PutShare(callerCtx("n-alice"), ws, cal, "n-bob", ngac.CalendarShareEdit)
	require.NoError(t, err)
	title := "x"
	_, err = f.svc.UpdateEvent(callerCtx("n-bob"), ws, d.EventID, ScopeQuery{}, EventPatch{Version: 1, Title: &title})
	requireKind(t, err, ErrAccessDenied)
	requireKind(t, f.svc.DeleteEvent(callerCtx("n-bob"), ws, d.EventID, ScopeQuery{}, 0), ErrAccessDenied)

	// The owner and a manager do read it.
	assert.Equal(t, "Doctor", f.list(t, "n-alice", cal)[0].Title)
	_, err = f.svc.PutShare(callerCtx("n-alice"), ws, cal, "n-bob", ngac.CalendarShareManage)
	require.NoError(t, err)
	assert.Equal(t, "Doctor", f.list(t, "n-bob", cal)[0].Title)
}

func TestAGuestReadsTheEventThroughItsOwnOA(t *testing.T) {
	f := newFixture(t)
	cal := f.calOf(t, "n-alice", store.KindPersonal)
	d := f.create(t, "n-alice", newInput(cal))

	// Phase 06 makes the event OA; here one is written by hand.
	f.w.Seed("oa-ev", ngac.CalendarEventOAName(ngac.CalendarEventID(d.EventID)), "OA", nil)
	f.w.Assign("oa-ev", f.st.calendars[cal].NgacOAID)
	f.st.events[d.EventID].NgacOAID = "oa-ev"
	f.w.Allow("n-carol", "oa-ev", ngac.OpRead)

	got, err := f.svc.GetEvent(callerCtx("n-carol"), ws, d.EventID, "")
	require.NoError(t, err)
	assert.Equal(t, "Standup", got.Title, "a guest reads the event")
	assert.False(t, got.CanEdit)

	// Deny: a guest edits nothing, and a colleague who is not a guest still reads nothing.
	title := "x"
	_, err = f.svc.UpdateEvent(callerCtx("n-carol"), ws, d.EventID, ScopeQuery{}, EventPatch{Version: 1, Title: &title})
	requireKind(t, err, ErrAccessDenied)
	requireKind(t, f.svc.DeleteEvent(callerCtx("n-carol"), ws, d.EventID, ScopeQuery{}, 0), ErrAccessDenied)
	_, err = f.svc.GetEvent(callerCtx("n-bob"), ws, d.EventID, "")
	requireKind(t, err, ErrAccessDenied)
}

func TestDeletingAnEventRemovesItsAccessNodes(t *testing.T) {
	f := newFixture(t)
	cal := f.calOf(t, "n-alice", store.KindPersonal)
	d := f.create(t, "n-alice", newInput(cal))
	f.w.Seed("oa-ev", ngac.CalendarEventOAName(ngac.CalendarEventID(d.EventID)), "OA", nil)
	f.w.Seed("ua-ev", ngac.CalendarEventGuestsUAName(ngac.CalendarEventID(d.EventID)), "UA", nil)
	f.st.events[d.EventID].NgacOAID, f.st.events[d.EventID].GuestsUAID = "oa-ev", "ua-ev"

	require.NoError(t, f.svc.DeleteEvent(callerCtx("n-alice"), ws, d.EventID, ScopeQuery{}, 0))
	assert.NotContains(t, f.w.Nodes, "oa-ev")
	assert.NotContains(t, f.w.Nodes, "ua-ev")
	assert.Empty(t, f.st.events)
	_, err := f.svc.GetEvent(callerCtx("n-alice"), ws, d.EventID, "")
	requireKind(t, err, ErrNotFound)
}

func TestStaleVersionIsAConflictWithAMessageToShow(t *testing.T) {
	f := newFixture(t)
	cal := f.calOf(t, "n-alice", store.KindPersonal)
	d := f.create(t, "n-alice", newInput(cal))
	first, second := "First", "Second"
	got, err := f.svc.UpdateEvent(callerCtx("n-alice"), ws, d.EventID, ScopeQuery{}, EventPatch{Version: 1, Title: &first})
	require.NoError(t, err)
	assert.Equal(t, 2, got.Version)

	_, err = f.svc.UpdateEvent(callerCtx("n-alice"), ws, d.EventID, ScopeQuery{}, EventPatch{Version: 1, Title: &second})
	requireKind(t, err, ErrAlreadyExists)
	assert.Equal(t, "Sự kiện vừa được người khác sửa. Tải lại rồi thử lại.", err.Error())
	_, err = f.svc.UpdateEvent(callerCtx("n-alice"), ws, d.EventID, ScopeQuery{}, EventPatch{Title: &second})
	requireKind(t, err, ErrInvalidInput)
	requireKind(t, f.svc.DeleteEvent(callerCtx("n-alice"), ws, d.EventID, ScopeQuery{}, 1), ErrAlreadyExists)
	assert.Equal(t, "First", f.list(t, "n-alice", cal)[0].Title)
}

func TestRepeatingEventExpandsAndDescribesItself(t *testing.T) {
	f := newFixture(t)
	cal := f.calOf(t, "n-alice", store.KindPersonal)
	d := f.create(t, "n-alice", weekly(cal))
	assert.True(t, d.IsRecurring)
	assert.Equal(t, "Hằng tuần vào thứ Hai", d.RecurrenceText)
	assert.Equal(t, "FREQ=WEEKLY;BYDAY=MO", d.RRule)

	got := f.list(t, "n-alice", cal)
	assert.Equal(t, []string{"Standup@2026-11-02T09:00", "Standup@2026-11-09T09:00", "Standup@2026-11-16T09:00", "Standup@2026-11-23T09:00", "Standup@2026-11-30T09:00"}, titles(got))
	assert.Equal(t, got[1].Start, got[1].OccurrenceStart)
	assert.Equal(t, "Hằng tuần vào thứ Hai", got[1].RecurrenceText)

	// A repeating event needs a scope to change; one that names no occurrence is refused.
	title := "x"
	_, err := f.svc.UpdateEvent(callerCtx("n-alice"), ws, d.EventID, ScopeQuery{}, EventPatch{Version: 1, Title: &title})
	requireKind(t, err, ErrInvalidInput)
	_, err = f.svc.UpdateEvent(callerCtx("n-alice"), ws, d.EventID, ScopeQuery{Scope: ScopeThis}, EventPatch{Version: 1, Title: &title})
	requireKind(t, err, ErrInvalidInput)
	_, err = f.svc.UpdateEvent(callerCtx("n-alice"), ws, d.EventID, ScopeQuery{Scope: "some", Occurrence: got[1].OccurrenceStart}, EventPatch{Version: 1, Title: &title})
	requireKind(t, err, ErrInvalidInput)
	_, err = f.svc.UpdateEvent(callerCtx("n-alice"), ws, d.EventID, ScopeQuery{Scope: ScopeThis, Occurrence: "2026-11-10T09:00:00+07:00"}, EventPatch{Version: 1, Title: &title})
	requireKind(t, err, ErrNotFound) // a Tuesday is not an occurrence
}

func TestEditScopes(t *testing.T) {
	f := newFixture(t)
	cal := f.calOf(t, "n-alice", store.KindPersonal)
	d := f.create(t, "n-alice", weekly(cal))
	occ := func() []*OccurrenceView { return f.list(t, "n-alice", cal) }
	nth := func(i int) string { return occ()[i].OccurrenceStart }
	ctx := callerCtx("n-alice")

	// This one: only the second Monday changes, and moves an hour later.
	title, start, end := "Moved standup", "2026-11-09T10:00:00+07:00", "2026-11-09T11:00:00+07:00"
	got, err := f.svc.UpdateEvent(ctx, ws, d.EventID, ScopeQuery{Scope: ScopeThis, Occurrence: nth(1)}, EventPatch{Version: 1, Title: &title, Start: &start, End: &end})
	require.NoError(t, err)
	assert.Equal(t, d.EventID, got.EventID, "an occurrence keeps the id of its series")
	assert.Equal(t, "2026-11-09T09:00:00+07:00", got.OccurrenceStart, "and its place in the series")
	assert.Equal(t, start, got.Start)
	assert.Equal(t, []string{"Standup@2026-11-02T09:00", "Moved standup@2026-11-09T10:00", "Standup@2026-11-16T09:00", "Standup@2026-11-23T09:00", "Standup@2026-11-30T09:00"}, titles(occ()))
	exceptionVersion := occ()[1].Version
	assert.Equal(t, 1, exceptionVersion)

	// All: the series title changes, the edited occurrence is kept and counted.
	all := "Daily sync"
	got, err = f.svc.UpdateEvent(ctx, ws, d.EventID, ScopeQuery{Scope: ScopeAll, Occurrence: nth(0)}, EventPatch{Version: 1, Title: &all})
	require.NoError(t, err)
	assert.Equal(t, 1, got.PreservedOverrides)
	assert.Equal(t, 2, got.Version)
	assert.Equal(t, []string{"Daily sync@2026-11-02T09:00", "Moved standup@2026-11-09T10:00", "Daily sync@2026-11-16T09:00", "Daily sync@2026-11-23T09:00", "Daily sync@2026-11-30T09:00"}, titles(occ()))

	// Following: from the 23rd on it is at 14:00 with a new name; the earlier ones are untouched.
	fol, fstart, fend := "Afternoon sync", "2026-11-23T14:00:00+07:00", "2026-11-23T15:00:00+07:00"
	got, err = f.svc.UpdateEvent(ctx, ws, d.EventID, ScopeQuery{Scope: ScopeFollowing, Occurrence: nth(3)}, EventPatch{Version: 2, Title: &fol, Start: &fstart, End: &fend})
	require.NoError(t, err)
	assert.NotEqual(t, d.EventID, got.EventID, "the tail is a new event")
	assert.Equal(t, fstart, got.Start)
	assert.Equal(t, []string{"Daily sync@2026-11-02T09:00", "Moved standup@2026-11-09T10:00", "Daily sync@2026-11-16T09:00", "Afternoon sync@2026-11-23T14:00", "Afternoon sync@2026-11-30T14:00"}, titles(occ()))
	assert.Equal(t, "Hằng tuần vào thứ Hai", got.RecurrenceText)
	require.Len(t, got.Attendees, 1, "the guests are copied to the tail")
	assert.Equal(t, []ReminderInput{{MinutesBefore: 10}}, got.Reminders)
	old, err := f.svc.GetEvent(ctx, ws, d.EventID, "")
	require.NoError(t, err)
	assert.Equal(t, "FREQ=WEEKLY;BYDAY=MO;UNTIL=20261123T015959Z", old.RRule, "the old series ends one second before the split")
}

func TestShiftingAWholeSeriesKeepsItsWallClockAndExceptions(t *testing.T) {
	f := newFixture(t)
	cal := f.calOf(t, "n-alice", store.KindPersonal)
	in := weekly(cal)
	in.TZID, in.Start, in.End = "America/New_York", "2026-10-26T09:00:00-04:00", "2026-10-26T10:00:00-04:00"
	d := f.create(t, "n-alice", in)
	ctx := callerCtx("n-alice")
	list := func() []*OccurrenceView {
		got, err := f.svc.ListEvents(ctx, ws, ListEventsQuery{From: "2026-10-25T00:00:00-04:00", To: "2026-11-18T00:00:00-05:00", Calendars: []string{cal}})
		require.NoError(t, err)
		return got
	}
	got := list()
	require.Len(t, got, 4)
	assert.Equal(t, "2026-11-02T09:00:00-05:00", got[1].Start, "09:00 local after the clocks went back")

	// Cancel the second, then drag the third one hour later: the series follows.
	require.NoError(t, f.svc.DeleteEvent(ctx, ws, d.EventID, ScopeQuery{Scope: ScopeThis, Occurrence: got[1].OccurrenceStart}, 0))
	start, end := "2026-11-09T10:00:00-05:00", "2026-11-09T11:00:00-05:00"
	res, err := f.svc.UpdateEvent(ctx, ws, d.EventID, ScopeQuery{Scope: ScopeAll, Occurrence: got[2].OccurrenceStart}, EventPatch{Version: 1, Start: &start, End: &end})
	require.NoError(t, err)
	assert.Equal(t, 0, res.PreservedOverrides, "a cancelled occurrence is kept but is not an edit")
	assert.Equal(t, "2026-10-26T10:00:00-04:00", f.mustMaster(t, d.EventID).Start)
	after := list()
	assert.Equal(t, []string{"2026-10-26T10:00:00-04:00", "2026-11-09T10:00:00-05:00", "2026-11-16T10:00:00-05:00"}, []string{after[0].Start, after[1].Start, after[2].Start},
		"the 2 November occurrence stays cancelled and the rest moved with the series")
}

func (f *fixture) mustMaster(t *testing.T, id string) *EventDetailView {
	t.Helper()
	d, err := f.svc.GetEvent(callerCtx("n-alice"), ws, id, "")
	require.NoError(t, err)
	return d
}

func TestFollowingFromTheFirstOccurrenceChangesTheWholeSeries(t *testing.T) {
	f := newFixture(t)
	cal := f.calOf(t, "n-alice", store.KindPersonal)
	d := f.create(t, "n-alice", weekly(cal))
	title := "Renamed"
	got, err := f.svc.UpdateEvent(callerCtx("n-alice"), ws, d.EventID, ScopeQuery{Scope: ScopeFollowing, Occurrence: d.OccurrenceStart}, EventPatch{Version: 1, Title: &title})
	require.NoError(t, err)
	assert.Equal(t, d.EventID, got.EventID, "no split when nothing precedes")
	assert.Len(t, f.st.events, 1)
}

func TestFollowingKeepsTheRemainingCount(t *testing.T) {
	f := newFixture(t)
	cal := f.calOf(t, "n-alice", store.KindPersonal)
	in := weekly(cal)
	in.RRule = sp("FREQ=WEEKLY;BYDAY=MO;COUNT=5")
	d := f.create(t, "n-alice", in)
	occ := f.list(t, "n-alice", cal)
	require.Len(t, occ, 5)
	title := "Tail"
	got, err := f.svc.UpdateEvent(callerCtx("n-alice"), ws, d.EventID, ScopeQuery{Scope: ScopeFollowing, Occurrence: occ[3].OccurrenceStart}, EventPatch{Version: 1, Title: &title})
	require.NoError(t, err)
	assert.Equal(t, "FREQ=WEEKLY;BYDAY=MO;COUNT=2", got.RRule, "two of the five remain")
	assert.Equal(t, []string{"Standup@2026-11-02T09:00", "Standup@2026-11-09T09:00", "Standup@2026-11-16T09:00", "Tail@2026-11-23T09:00", "Tail@2026-11-30T09:00"}, titles(f.list(t, "n-alice", cal)))
}

func TestReadingAnOccurrenceGivesThatInstanceNotTheSeries(t *testing.T) {
	f := newFixture(t)
	cal := f.calOf(t, "n-alice", store.KindPersonal)
	ctx := callerCtx("n-alice")
	d := f.create(t, "n-alice", weekly(cal))
	occ := f.list(t, "n-alice", cal)

	// A later instance reads at its own times, not the series' first.
	third, err := f.svc.GetEvent(ctx, ws, d.EventID, occ[2].OccurrenceStart)
	require.NoError(t, err)
	assert.Equal(t, occ[2].Start, third.Start)
	assert.Equal(t, occ[2].End, third.End)
	assert.Equal(t, occ[2].OccurrenceStart, third.OccurrenceStart)

	// A moved instance reads at its new times and its own override data.
	start, end, place := "2026-11-10T14:00:00+07:00", "2026-11-10T15:00:00+07:00", "Phòng 2"
	_, err = f.svc.UpdateEvent(ctx, ws, d.EventID, ScopeQuery{Scope: ScopeThis, Occurrence: occ[1].OccurrenceStart},
		EventPatch{Version: 1, Start: &start, End: &end, Location: &place})
	require.NoError(t, err)
	moved, err := f.svc.GetEvent(ctx, ws, d.EventID, occ[1].OccurrenceStart)
	require.NoError(t, err)
	assert.Equal(t, "Phòng 2", moved.Location)
	assert.Equal(t, occ[1].OccurrenceStart, moved.OccurrenceStart)
	assert.NotEqual(t, occ[1].Start, moved.Start)

	// An instant the series never produces is not an occurrence.
	_, err = f.svc.GetEvent(ctx, ws, d.EventID, "2026-11-04T09:00:00+07:00")
	requireKind(t, err, ErrNotFound)
}

func TestDeleteScopes(t *testing.T) {
	f := newFixture(t)
	cal := f.calOf(t, "n-alice", store.KindPersonal)
	ctx := callerCtx("n-alice")
	d := f.create(t, "n-alice", weekly(cal))
	occ := f.list(t, "n-alice", cal)

	require.NoError(t, f.svc.DeleteEvent(ctx, ws, d.EventID, ScopeQuery{Scope: ScopeThis, Occurrence: occ[1].OccurrenceStart}, 0))
	assert.Equal(t, []string{"Standup@2026-11-02T09:00", "Standup@2026-11-16T09:00", "Standup@2026-11-23T09:00", "Standup@2026-11-30T09:00"}, titles(f.list(t, "n-alice", cal)))
	_, err := f.svc.GetEvent(ctx, ws, d.EventID, occ[1].OccurrenceStart)
	requireKind(t, err, ErrNotFound)

	require.NoError(t, f.svc.DeleteEvent(ctx, ws, d.EventID, ScopeQuery{Scope: ScopeFollowing, Occurrence: occ[3].OccurrenceStart}, 0))
	assert.Equal(t, []string{"Standup@2026-11-02T09:00", "Standup@2026-11-16T09:00"}, titles(f.list(t, "n-alice", cal)))

	require.NoError(t, f.svc.DeleteEvent(ctx, ws, d.EventID, ScopeQuery{Scope: ScopeAll}, 0))
	assert.Empty(t, f.list(t, "n-alice", cal))
	assert.Empty(t, f.st.events, "the exceptions go with the series")
	requireKind(t, f.svc.DeleteEvent(ctx, ws, d.EventID, ScopeQuery{Scope: ScopeAll}, 0), ErrNotFound)

	// Following from the first occurrence deletes the whole series.
	again := f.create(t, "n-alice", weekly(cal))
	require.NoError(t, f.svc.DeleteEvent(ctx, ws, again.EventID, ScopeQuery{Scope: ScopeFollowing, Occurrence: again.OccurrenceStart}, 0))
	assert.Empty(t, f.st.events)
}

func TestMovingAnEventToAnotherCalendarNeedsWriteOnBoth(t *testing.T) {
	f := newFixture(t)
	mine := f.calOf(t, "n-alice", store.KindPersonal)
	workspaceCal := f.calOf(t, "n-alice", store.KindWorkspace)
	group := f.groupCal(t)
	d := f.create(t, "n-alice", newInput(mine))

	_, err := f.svc.UpdateEvent(callerCtx("n-alice"), ws, d.EventID, ScopeQuery{}, EventPatch{Version: 1, CalendarID: &workspaceCal})
	requireKind(t, err, ErrAccessDenied) // read-only there
	moved, err := f.svc.UpdateEvent(callerCtx("n-alice"), ws, d.EventID, ScopeQuery{}, EventPatch{Version: 1, CalendarID: &group})
	require.NoError(t, err)
	assert.Equal(t, group, moved.CalendarID)
	assert.Empty(t, f.list(t, "n-alice", mine))
	assert.Len(t, f.list(t, "n-alice", group), 1)
}

func TestChangesAreAnnouncedToTheRightAudience(t *testing.T) {
	f := newFixture(t)
	mine := f.calOf(t, "n-alice", store.KindPersonal)
	_, err := f.svc.PutShare(callerCtx("n-alice"), ws, mine, "n-bob", ngac.CalendarShareView)
	require.NoError(t, err)
	d := f.create(t, "n-alice", newInput(mine))
	title := "Renamed"
	_, err = f.svc.UpdateEvent(callerCtx("n-alice"), ws, d.EventID, ScopeQuery{}, EventPatch{Version: 1, Title: &title})
	require.NoError(t, err)
	require.NoError(t, f.svc.DeleteEvent(callerCtx("n-alice"), ws, d.EventID, ScopeQuery{}, 0))

	var seen []realtime.Event
	for _, e := range f.rec.Events() {
		if e.Domain == realtime.DomainCalendar && len(e.IDs) == 1 && e.IDs[0] == d.EventID {
			seen = append(seen, e)
		}
	}
	require.Len(t, seen, 3)
	assert.Equal(t, []string{realtime.KindCreated, realtime.KindUpdated, realtime.KindDeleted}, []string{seen[0].Kind, seen[1].Kind, seen[2].Kind})
	assert.Equal(t, []string{"n-alice", "n-bob"}, seen[0].UserNodeIDs, "the owner and who it is shared with, nobody else")
	assert.Equal(t, mine, seen[0].ParentID)

	// The workspace calendar is addressed to the workspace; a group's to the group.
	f.create(t, "n-olga", newInput(f.calOf(t, "n-alice", store.KindWorkspace)))
	wsEvent := f.rec.Events()
	assert.Empty(t, wsEvent[len(wsEvent)-1].UserNodeIDs)
	assert.Empty(t, wsEvent[len(wsEvent)-1].ChannelID)
	f.create(t, "n-alice", newInput(f.groupCal(t)))
	groupEvent := f.rec.Events()
	assert.Equal(t, "ch-team", groupEvent[len(groupEvent)-1].ChannelID)
}

func TestEventsOfAnotherWorkspaceAreNotFound(t *testing.T) {
	f := newFixture(t)
	cal := f.calOf(t, "n-alice", store.KindPersonal)
	d := f.create(t, "n-alice", newInput(cal))
	f.st.events[d.EventID].WorkspaceID = "other"
	_, err := f.svc.GetEvent(callerCtx("n-alice"), ws, d.EventID, "")
	requireKind(t, err, ErrNotFound)

	// A token for another workspace gets nothing here.
	other := context.WithValue(context.Background(), struct{}{}, nil)
	_, err = f.svc.ListEvents(other, ws, ListEventsQuery{From: rangeStart, To: rangeEnd})
	requireKind(t, err, ErrAccessDenied)
}

func TestListRangeIsChecked(t *testing.T) {
	f := newFixture(t)
	for _, q := range []ListEventsQuery{
		{From: "nope", To: rangeEnd},
		{From: rangeStart, To: "nope"},
		{From: rangeEnd, To: rangeStart},
		{From: "2026-01-01T00:00:00Z", To: "2028-01-01T00:00:00Z"},
	} {
		_, err := f.svc.ListEvents(callerCtx("n-alice"), ws, q)
		requireKind(t, err, ErrInvalidInput)
	}
}
