package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/ngac"
	"ngac-platform/services/calendar/internal/store"
)

// A private event with guests and a call: the event's OA sits under its
// calendar's, so everyone who reads the calendar reads the OA too. Only the
// people the event names (and the owner or a manager of the calendar) may see
// it, whatever they hold on the calendar.
func TestAPrivateEventWithGuestsIsReadOnlyByItsGuestsOrganizerAndManagers(t *testing.T) {
	f := newFixture(t)
	cal := f.calOf(t, "n-alice", store.KindPersonal)
	in := withGuests(cal, "n-carol")
	in.Visibility, in.Title, in.HasCall = VisibilityPrivate, "Doctor", true
	d := f.create(t, "n-alice", in)
	_, err := f.svc.PutShare(callerCtx("n-alice"), ws, cal, "n-bob", ngac.CalendarShareView)
	require.NoError(t, err)

	// Deny: a view share reads the calendar and so the event's OA, but is not on the event.
	_, err = f.svc.GetEvent(callerCtx("n-bob"), ws, d.EventID, "")
	requireKind(t, err, ErrAccessDenied)
	got := f.list(t, "n-bob", cal)
	require.Len(t, got, 1)
	assert.True(t, got[0].BusyOnly)
	assert.Equal(t, "Bận", got[0].Title)
	assert.False(t, got[0].HasCall)
	assert.NotEqual(t, d.EventID, got[0].EventID, "a busy block does not hand out the event's id")
	assert.NotEmpty(t, got[0].EventID, "but it keeps a key the screen can tell blocks apart by")
	_, err = f.svc.GetEvent(callerCtx("n-bob"), ws, got[0].EventID, "")
	requireKind(t, err, ErrNotFound)

	// An edit share is still not a guest.
	_, err = f.svc.PutShare(callerCtx("n-alice"), ws, cal, "n-bob", ngac.CalendarShareEdit)
	require.NoError(t, err)
	_, err = f.svc.GetEvent(callerCtx("n-bob"), ws, d.EventID, "")
	requireKind(t, err, ErrAccessDenied)

	// Allow: the guest, the organizer and (once shared with manage) a manager.
	one, err := f.svc.GetEvent(callerCtx("n-carol"), ws, d.EventID, "")
	require.NoError(t, err)
	assert.Equal(t, "Doctor", one.Title)
	assert.True(t, one.HasCall)
	one, err = f.svc.GetEvent(callerCtx("n-alice"), ws, d.EventID, "")
	require.NoError(t, err)
	assert.Equal(t, "Doctor", one.Title)
	_, err = f.svc.PutShare(callerCtx("n-alice"), ws, cal, "n-bob", ngac.CalendarShareManage)
	require.NoError(t, err)
	one, err = f.svc.GetEvent(callerCtx("n-bob"), ws, d.EventID, "")
	require.NoError(t, err)
	assert.Equal(t, "Doctor", one.Title)
}

// The same event, not private, is read by whoever reads the calendar.
func TestAnOrdinaryEventWithGuestsStaysReadableByCalendarReaders(t *testing.T) {
	f := newFixture(t)
	cal := f.calOf(t, "n-alice", store.KindPersonal)
	in := withGuests(cal, "n-carol")
	in.HasCall = true
	d := f.create(t, "n-alice", in)
	_, err := f.svc.PutShare(callerCtx("n-alice"), ws, cal, "n-bob", ngac.CalendarShareView)
	require.NoError(t, err)

	one, err := f.svc.GetEvent(callerCtx("n-bob"), ws, d.EventID, "")
	require.NoError(t, err)
	assert.Equal(t, "Standup", one.Title)
	got := f.list(t, "n-bob", cal)
	require.Len(t, got, 1)
	assert.Equal(t, d.EventID, got[0].EventID)
	assert.False(t, got[0].BusyOnly)
}

// A guest removed from a private event loses it.
func TestAGuestRemovedFromAPrivateEventLosesIt(t *testing.T) {
	f := newFixture(t)
	cal := f.calOf(t, "n-alice", store.KindPersonal)
	in := withGuests(cal, "n-carol")
	in.Visibility = VisibilityPrivate
	d := f.create(t, "n-alice", in)
	_, err := f.svc.PutShare(callerCtx("n-alice"), ws, cal, "n-carol", ngac.CalendarShareView)
	require.NoError(t, err)
	_, err = f.svc.GetEvent(callerCtx("n-carol"), ws, d.EventID, "")
	require.NoError(t, err)

	none := []string{}
	_, err = f.svc.UpdateEvent(callerCtx("n-alice"), ws, d.EventID, ScopeQuery{}, EventPatch{Version: 1, Attendees: &none})
	require.NoError(t, err)
	_, err = f.svc.GetEvent(callerCtx("n-carol"), ws, d.EventID, "")
	requireKind(t, err, ErrAccessDenied)
}
