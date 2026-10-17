package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/services/calendar/internal/store"
)

func bp(b bool) *bool { return &b }

// A screen sends allDay (and the zone) with every edit of an occurrence; that is
// not an attempt to change them.
func TestEditingOneOccurrenceMayRestateTheSeriesTimeSettings(t *testing.T) {
	f := newFixture(t)
	cal := f.calOf(t, "n-alice", store.KindPersonal)
	d := f.create(t, "n-alice", weekly(cal))
	ctx := callerCtx("n-alice")
	occ := f.list(t, "n-alice", cal)[1].OccurrenceStart
	start, end := "2026-11-09T10:00:00+07:00", "2026-11-09T11:00:00+07:00"

	got, err := f.svc.UpdateEvent(ctx, ws, d.EventID, ScopeQuery{Scope: ScopeThis, Occurrence: occ},
		EventPatch{Version: 1, Start: &start, End: &end, AllDay: bp(false), TZID: sp(saigon)})
	require.NoError(t, err)
	assert.Equal(t, start, got.Start)

	// Deny: changing them is still the series' business.
	_, err = f.svc.UpdateEvent(ctx, ws, d.EventID, ScopeQuery{Scope: ScopeThis, Occurrence: occ}, EventPatch{Version: got.Version, AllDay: bp(true)})
	requireKind(t, err, ErrInvalidInput)
	_, err = f.svc.UpdateEvent(ctx, ws, d.EventID, ScopeQuery{Scope: ScopeThis, Occurrence: occ}, EventPatch{Version: got.Version, TZID: sp("America/New_York")})
	requireKind(t, err, ErrInvalidInput)
	rule := "FREQ=DAILY"
	_, err = f.svc.UpdateEvent(ctx, ws, d.EventID, ScopeQuery{Scope: ScopeThis, Occurrence: occ}, EventPatch{Version: got.Version, RRule: OptionalString{Set: true, Value: &rule}})
	requireKind(t, err, ErrInvalidInput)
}

const ordinalRefusal = "Không dời được chuỗi lặp theo thứ trong tháng sang ngày khác. Hãy sửa quy tắc lặp."

// A monthly rule that names "the second Wednesday" or "the 11th" does not follow
// its first event to another day; moving the series there would leave the rule
// and the dates disagreeing.
func TestMovingAMonthlySeriesOffItsAnchorDayIsRefused(t *testing.T) {
	for _, tc := range []struct{ name, rule string }{
		{"ordinal weekday", "FREQ=MONTHLY;BYDAY=2WE"},
		{"day of month", "FREQ=MONTHLY;BYMONTHDAY=11"},
		{"yearly ordinal weekday", "FREQ=YEARLY;BYMONTH=11;BYDAY=2WE"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			cal := f.calOf(t, "n-alice", store.KindPersonal)
			in := newInput(cal)
			in.Start, in.End, in.RRule = "2026-11-11T09:00:00+07:00", "2026-11-11T10:00:00+07:00", sp(tc.rule)
			d := f.create(t, "n-alice", in)
			ctx := callerCtx("n-alice")
			day, dayEnd := "2026-11-12T09:00:00+07:00", "2026-11-12T10:00:00+07:00"

			_, err := f.svc.UpdateEvent(ctx, ws, d.EventID, ScopeQuery{Scope: ScopeAll, Occurrence: "2026-11-11T09:00:00+07:00"}, EventPatch{Version: 1, Start: &day, End: &dayEnd})
			requireKind(t, err, ErrInvalidInput)
			assert.EqualError(t, err, ordinalRefusal)
			// Nothing was written.
			still, err := f.svc.GetEvent(ctx, ws, d.EventID, "")
			require.NoError(t, err)
			assert.Equal(t, 1, still.Version)
			assert.Equal(t, "2026-11-11T09:00:00+07:00", still.Start)

			// The same for "this and the following" from a later occurrence.
			later, laterEnd := "2026-12-10T09:00:00+07:00", "2026-12-10T10:00:00+07:00"
			occ := map[string]string{"FREQ=MONTHLY;BYMONTHDAY=11": "2026-12-11T09:00:00+07:00"}[tc.rule]
			if occ == "" {
				occ = "2026-12-09T09:00:00+07:00"
			}
			if tc.rule == "FREQ=YEARLY;BYMONTH=11;BYDAY=2WE" {
				occ, later, laterEnd = "2027-11-10T09:00:00+07:00", "2027-11-11T09:00:00+07:00", "2027-11-11T10:00:00+07:00"
			}
			_, err = f.svc.UpdateEvent(ctx, ws, d.EventID, ScopeQuery{Scope: ScopeFollowing, Occurrence: occ}, EventPatch{Version: 1, Start: &later, End: &laterEnd})
			requireKind(t, err, ErrInvalidInput)
			assert.EqualError(t, err, ordinalRefusal)

			// Allow: the time of day moves freely, and so does a new rule.
			hour, hourEnd := "2026-11-11T10:00:00+07:00", "2026-11-11T11:00:00+07:00"
			moved, err := f.svc.UpdateEvent(ctx, ws, d.EventID, ScopeQuery{Scope: ScopeAll, Occurrence: "2026-11-11T09:00:00+07:00"}, EventPatch{Version: 1, Start: &hour, End: &hourEnd})
			require.NoError(t, err)
			assert.Equal(t, hour, moved.Start)
			weeklyRule := "FREQ=WEEKLY;BYDAY=TH"
			rerule, err := f.svc.UpdateEvent(ctx, ws, d.EventID, ScopeQuery{Scope: ScopeAll, Occurrence: "2026-11-11T10:00:00+07:00"},
				EventPatch{Version: moved.Version, Start: &day, End: &dayEnd, RRule: OptionalString{Set: true, Value: &weeklyRule}})
			require.NoError(t, err)
			assert.Equal(t, weeklyRule, rerule.RRule)
		})
	}
}

// A plain weekly series is still dragged to another weekday, its BYDAY following.
func TestMovingAWeeklySeriesToAnotherWeekdayStillWorks(t *testing.T) {
	f := newFixture(t)
	cal := f.calOf(t, "n-alice", store.KindPersonal)
	d := f.create(t, "n-alice", weekly(cal))
	start, end := "2026-11-03T09:00:00+07:00", "2026-11-03T10:00:00+07:00"
	got, err := f.svc.UpdateEvent(callerCtx("n-alice"), ws, d.EventID, ScopeQuery{Scope: ScopeAll, Occurrence: "2026-11-02T09:00:00+07:00"}, EventPatch{Version: 1, Start: &start, End: &end})
	require.NoError(t, err)
	assert.Equal(t, "FREQ=WEEKLY;BYDAY=TU", got.RRule)
}

// A guest change that fails after the row was saved must not leave the new
// content behind a failed request.
func TestAFailedGuestChangeRestoresTheEvent(t *testing.T) {
	f := newFixture(t)
	cal := f.calOf(t, "n-alice", store.KindPersonal)
	d := f.create(t, "n-alice", newInput(cal))
	ctx := callerCtx("n-alice")
	title := "Changed"
	patch := EventPatch{Version: 1, Title: &title, HasCall: bp(true), Reminders: remindIn(30)}

	f.w.FailAt = f.w.Mutations() + 2 // inside wiring the nodes the call needs
	_, err := f.svc.UpdateEvent(ctx, ws, d.EventID, ScopeQuery{}, patch)
	require.Error(t, err)
	f.w.FailAt = 0

	got, err := f.svc.GetEvent(ctx, ws, d.EventID, "")
	require.NoError(t, err)
	assert.Equal(t, "Standup", got.Title, "the edit did not stay half done")
	assert.False(t, got.HasCall)
	assert.Equal(t, []ReminderInput{{MinutesBefore: 10}}, got.Reminders)
	assert.Empty(t, f.st.events[d.EventID].NgacOAID)

	// The screen reloads and tries again with the version it now reads.
	patch.Version = got.Version
	again, err := f.svc.UpdateEvent(ctx, ws, d.EventID, ScopeQuery{}, patch)
	require.NoError(t, err)
	assert.Equal(t, "Changed", again.Title)
	assert.True(t, again.HasCall)
}

// The same for a split: the tail made for the change is taken away and the old
// series is whole again.
func TestAFailedGuestChangeOfASplitRestoresTheSeries(t *testing.T) {
	f := newFixture(t)
	cal := f.calOf(t, "n-alice", store.KindPersonal)
	d := f.create(t, "n-alice", weekly(cal))
	ctx := callerCtx("n-alice")
	title := "Tail"

	f.w.FailAt = f.w.Mutations() + 2
	_, err := f.svc.UpdateEvent(ctx, ws, d.EventID, ScopeQuery{Scope: ScopeFollowing, Occurrence: "2026-11-16T09:00:00+07:00"},
		EventPatch{Version: 1, Title: &title, HasCall: bp(true)})
	require.Error(t, err)
	f.w.FailAt = 0

	got := f.list(t, "n-alice", cal)
	require.Len(t, got, 5)
	for _, o := range got {
		assert.Equal(t, "Standup", o.Title)
		assert.Equal(t, d.EventID, o.EventID)
	}
	one, err := f.svc.GetEvent(ctx, ws, d.EventID, "")
	require.NoError(t, err)
	assert.Equal(t, "FREQ=WEEKLY;BYDAY=MO", one.RRule, "the series runs on")
	assert.Len(t, f.st.events, 1, "no tail is left behind")
}
