package domain

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/ngac"
	"ngac-platform/services/calendar/internal/store"
)

func (f *fixture) search(t *testing.T, node, q string) []*OccurrenceView {
	t.Helper()
	got, err := f.svc.SearchEvents(callerCtx(node), ws, q)
	require.NoError(t, err)
	return got
}

func TestSearchFindsTitlePlaceAndDescriptionIgnoringCase(t *testing.T) {
	f := newFixture(t)
	cal := f.calOf(t, "n-alice", store.KindPersonal)
	in := newInput(cal)
	in.Title, in.Location, in.Description = "Quarterly planning", "Room 7", "Review the BUDGET"
	f.create(t, "n-alice", in)
	other := newInput(cal)
	other.Title, other.Start, other.End = "Lunch", "2026-11-03T12:00:00+07:00", "2026-11-03T13:00:00+07:00"
	f.create(t, "n-alice", other)

	for _, q := range []string{"plan", "ROOM 7", "budget", "  quarterly  "} {
		got := f.search(t, "n-alice", q)
		require.Len(t, got, 1, q)
		assert.Equal(t, "Quarterly planning", got[0].Title)
		assert.False(t, got[0].BusyOnly)
	}
	assert.Empty(t, f.search(t, "n-alice", "nothing like it"))
	assert.Equal(t, []*OccurrenceView{}, f.search(t, "n-alice", "   "), "an empty search finds nothing, without an error")
	_, err := f.svc.SearchEvents(callerCtx("n-alice"), ws, string(make([]rune, 101)))
	requireKind(t, err, ErrInvalidInput)
	_, err = f.svc.SearchEvents(callerCtx("n-gus"), ws, "plan")
	requireKind(t, err, ErrAccessDenied)
}

func TestSearchNeverReachesWhatTheCallerCannotRead(t *testing.T) {
	f := newFixture(t)
	cal := f.calOf(t, "n-alice", store.KindPersonal)
	d := f.create(t, "n-alice", newInput(cal))
	secret := newInput(cal)
	secret.Title, secret.Visibility = "Standup with the doctor", VisibilityPrivate
	f.create(t, "n-alice", secret)

	// A colleague with no share finds neither; a view share finds the open one
	// and not the private one.
	assert.Empty(t, f.search(t, "n-carol", "standup"))
	_, err := f.svc.PutShare(callerCtx("n-alice"), ws, cal, "n-bob", ngac.CalendarShareView)
	require.NoError(t, err)
	got := f.search(t, "n-bob", "standup")
	require.Len(t, got, 1)
	assert.Equal(t, d.EventID, got[0].EventID)
	assert.Len(t, f.search(t, "n-alice", "standup"), 2)
}

func TestSearchGuestNamesOnlyWhereTheListIsVisible(t *testing.T) {
	f := newFixture(t)
	wsCal := f.calOf(t, "n-olga", store.KindWorkspace)
	shown := newInput(wsCal)
	shown.Title, shown.Attendees = "All hands", []string{"n-bob"}
	f.create(t, "n-olga", shown)
	hidden := newInput(wsCal)
	hidden.Title, hidden.Attendees, hidden.GuestsCanSeeList = "Reorg", []string{"n-bob"}, false
	hidden.Start, hidden.End = "2026-11-04T09:00:00+07:00", "2026-11-04T10:00:00+07:00"
	f.create(t, "n-olga", hidden)

	// An ordinary member reads the workspace calendar: they find the event whose
	// list is visible by a guest's name, and not the one whose list is hidden.
	got := f.search(t, "n-carol", "bob")
	require.Len(t, got, 1)
	assert.Equal(t, "All hands", got[0].Title)
	assert.Len(t, f.search(t, "n-olga", "bob"), 2, "the host sees every list")
	assert.Len(t, f.search(t, "n-carol", "reorg"), 1, "the title still matches")
}

func TestSearchIncludesEventsOfAnotherCalendarOnlyForGuests(t *testing.T) {
	f := newFixture(t)
	cal := f.calOf(t, "n-alice", store.KindPersonal)
	in := newInput(cal)
	in.Title, in.Attendees = "Offsite", []string{"n-bob"}
	d := f.create(t, "n-alice", in)

	got := f.search(t, "n-bob", "offsite")
	require.Len(t, got, 1)
	assert.Equal(t, d.EventID, got[0].EventID)
	assert.Equal(t, "Offsite", got[0].Title)
	assert.False(t, got[0].CanEdit)
	assert.Empty(t, f.search(t, "n-carol", "offsite"))

	// Removed from the event, the match is gone.
	_, err := f.svc.UpdateEvent(callerCtx("n-alice"), ws, d.EventID, ScopeQuery{}, EventPatch{Version: 1, Attendees: &[]string{}})
	require.NoError(t, err)
	assert.Empty(t, f.search(t, "n-bob", "offsite"))
}

func TestSearchAnswersWithTheNextOccurrenceOfARepeatingEvent(t *testing.T) {
	f := newFixture(t)
	f.svc.now = func() time.Time { return time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC) }
	cal := f.calOf(t, "n-alice", store.KindPersonal)
	f.create(t, "n-alice", weekly(cal))
	over := weekly(cal)
	over.Title = "Retro"
	over.RRule = sp("FREQ=WEEKLY;BYDAY=MO;COUNT=2")
	f.create(t, "n-alice", over)

	got := f.search(t, "n-alice", "standup")
	require.Len(t, got, 1)
	assert.Equal(t, "2026-11-16T09:00:00+07:00", got[0].Start, "the next Monday after the 10th")
	assert.True(t, got[0].IsRecurring)

	past := f.search(t, "n-alice", "retro")
	require.Len(t, past, 1)
	assert.Equal(t, "2026-11-09T09:00:00+07:00", past[0].Start, "a series that is over answers with its last occurrence")
}

func TestSearchAnswersAtMostFiftyUpcomingFirst(t *testing.T) {
	f := newFixture(t)
	f.svc.now = func() time.Time { return time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC) }
	cal := f.calOf(t, "n-alice", store.KindPersonal)
	for i := 0; i < 60; i++ {
		in := newInput(cal)
		day := time.Date(2026, 9, 1, 9, 0, 0, 0, time.FixedZone("", 7*3600)).AddDate(0, 0, i*2)
		in.Title = fmt.Sprintf("Sync %02d", i)
		in.Start, in.End = day.Format(time.RFC3339), day.Add(time.Hour).Format(time.RFC3339)
		f.create(t, "n-alice", in)
	}
	got := f.search(t, "n-alice", "sync")
	require.Len(t, got, 50)
	assert.True(t, got[0].Start >= "2026-11-10", "upcoming events come first")
}
