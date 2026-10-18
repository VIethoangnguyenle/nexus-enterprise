package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/services/calendar/internal/events"
	"ngac-platform/services/calendar/internal/store"
)

func defaultSettingsInput() SettingsView {
	return SettingsView{
		TZID: saigon, WorkStart: "08:00", WorkEnd: "17:30",
		DefaultReminder: 10, DefaultView: "week", WeekStart: 1,
		DefaultDuration: 30, NotifyInvites: true, NotifyChanges: true,
		NotifyReminders: true,
	}
}

// full sends every field of v, the way a client that knows all of them does.
func full(v SettingsView) SettingsPatch {
	visible := v.VisibleCalendars
	return SettingsPatch{
		TZID: &v.TZID, WorkStart: &v.WorkStart, WorkEnd: &v.WorkEnd,
		DefaultReminder: &v.DefaultReminder, DefaultView: &v.DefaultView,
		WeekStart: &v.WeekStart, VisibleCalendars: &visible,
		DefaultDuration: &v.DefaultDuration, NotifyInvites: &v.NotifyInvites,
		NotifyChanges: &v.NotifyChanges, NotifyReminders: &v.NotifyReminders,
	}
}

// A client that knows only the zone, the working hours and the reminder (the
// desktop dialog) must not turn the notifications off or reset the week start,
// the default view, the visible calendars or the default duration.
func TestSettingsSaveKeepsFieldsTheBodyLeavesOut(t *testing.T) {
	f := newFixture(t)
	cal := f.calOf(t, "n-alice", store.KindPersonal)
	ctx := callerCtx("n-alice")
	in := defaultSettingsInput()
	in.DefaultView, in.WeekStart, in.DefaultDuration = "month", 0, 60
	in.NotifyInvites, in.NotifyChanges, in.NotifyReminders = false, false, false
	in.VisibleCalendars = []string{cal}
	_, err := f.svc.UpdateSettings(ctx, ws, full(in))
	require.NoError(t, err)

	tz, start, end, reminder := "Asia/Bangkok", "09:00", "18:00", 15
	got, err := f.svc.UpdateSettings(ctx, ws, SettingsPatch{TZID: &tz, WorkStart: &start, WorkEnd: &end, DefaultReminder: &reminder})
	require.NoError(t, err)
	want := SettingsView{
		TZID: tz, WorkStart: start, WorkEnd: end, DefaultReminder: reminder,
		DefaultView: "month", WeekStart: 0, DefaultDuration: 60,
		VisibleCalendars: []string{cal},
	}
	assert.Equal(t, want, *got)
	stored, err := f.svc.GetSettings(ctx, ws)
	require.NoError(t, err)
	assert.Equal(t, want, *stored)

	// The other way round: a body with only the new fields keeps the zone and hours.
	on := true
	got, err = f.svc.UpdateSettings(ctx, ws, SettingsPatch{NotifyInvites: &on})
	require.NoError(t, err)
	assert.Equal(t, tz, got.TZID)
	assert.Equal(t, start, got.WorkStart)
	assert.True(t, got.NotifyInvites)
	assert.False(t, got.NotifyChanges)

	// An empty body changes nothing and is still checked against membership.
	_, err = f.svc.UpdateSettings(callerCtx("n-gus"), ws, SettingsPatch{})
	requireKind(t, err, ErrAccessDenied)
}

// A merged value is still checked: a bad field fails even when the rest is stored.
func TestSettingsMergedValueIsStillChecked(t *testing.T) {
	f := newFixture(t)
	ctx := callerCtx("n-alice")
	bad := "17:00"
	_, err := f.svc.UpdateSettings(ctx, ws, SettingsPatch{WorkEnd: &bad, WorkStart: ptr("18:00")})
	requireKind(t, err, ErrInvalidInput)
	_, err = f.svc.UpdateSettings(ctx, ws, SettingsPatch{WorkEnd: ptr("07:00")})
	requireKind(t, err, ErrInvalidInput)
	week := 3
	_, err = f.svc.UpdateSettings(ctx, ws, SettingsPatch{WeekStart: &week})
	requireKind(t, err, ErrInvalidInput)
	got, err := f.svc.GetSettings(ctx, ws)
	require.NoError(t, err)
	assert.Equal(t, "17:30", got.WorkEnd, "a rejected save stores nothing")
}

func ptr[T any](v T) *T { return &v }

func TestSettingsDefaults(t *testing.T) {
	f := newFixture(t)
	got, err := f.svc.GetSettings(callerCtx("n-alice"), ws)
	require.NoError(t, err)
	assert.Equal(t, "agenda", got.DefaultView)
	assert.Equal(t, 1, got.WeekStart)
	assert.Equal(t, 30, got.DefaultDuration)
	assert.True(t, got.NotifyInvites)
	assert.True(t, got.NotifyChanges)
	assert.True(t, got.NotifyReminders)
	assert.Nil(t, got.VisibleCalendars)
}

func TestSettingsInvalidDefaultView(t *testing.T) {
	f := newFixture(t)
	in := defaultSettingsInput()
	in.DefaultView = "invalid"
	_, err := f.svc.UpdateSettings(callerCtx("n-alice"), ws, full(in))
	requireKind(t, err, ErrInvalidInput)
}

func TestSettingsInvalidWeekStart(t *testing.T) {
	f := newFixture(t)
	in := defaultSettingsInput()
	in.WeekStart = 3
	_, err := f.svc.UpdateSettings(callerCtx("n-alice"), ws, full(in))
	requireKind(t, err, ErrInvalidInput)
}

func TestSettingsInvalidDefaultDuration(t *testing.T) {
	f := newFixture(t)
	for _, dur := range []int{4, 1441} {
		in := defaultSettingsInput()
		in.DefaultDuration = dur
		_, err := f.svc.UpdateSettings(callerCtx("n-alice"), ws, full(in))
		requireKind(t, err, ErrInvalidInput)
	}
}

func TestSettingsValidUpdate(t *testing.T) {
	f := newFixture(t)
	in := defaultSettingsInput()
	in.DefaultView = "month"
	in.WeekStart = 0
	in.DefaultDuration = 60
	in.NotifyInvites = false
	in.NotifyChanges = false
	in.NotifyReminders = false
	got, err := f.svc.UpdateSettings(callerCtx("n-alice"), ws, full(in))
	require.NoError(t, err)
	assert.Equal(t, "month", got.DefaultView)
	assert.Equal(t, 0, got.WeekStart)
	assert.Equal(t, 60, got.DefaultDuration)
	assert.False(t, got.NotifyInvites)
	assert.False(t, got.NotifyChanges)
	assert.False(t, got.NotifyReminders)

	// Re-read to verify persistence.
	got2, err := f.svc.GetSettings(callerCtx("n-alice"), ws)
	require.NoError(t, err)
	assert.Equal(t, "month", got2.DefaultView)
	assert.False(t, got2.NotifyInvites)
}

func TestSettingsVisibleCalendarsUnreadable(t *testing.T) {
	f := newFixture(t)
	in := defaultSettingsInput()
	in.VisibleCalendars = []string{"nonexistent-calendar-id"}
	_, err := f.svc.UpdateSettings(callerCtx("n-alice"), ws, full(in))
	requireKind(t, err, ErrAccessDenied)
}

func TestSettingsNonmemberDenied(t *testing.T) {
	f := newFixture(t)
	_, err := f.svc.GetSettings(callerCtx("n-gus"), ws)
	requireKind(t, err, ErrAccessDenied)

	in := defaultSettingsInput()
	_, err = f.svc.UpdateSettings(callerCtx("n-gus"), ws, full(in))
	requireKind(t, err, ErrAccessDenied)
}

// --- Notification gating tests ---

func TestNotifyInvitesSwitchGatesInvitedNotice(t *testing.T) {
	f := newFixture(t)
	rec := f.noticing()

	// Save bob's settings with NotifyInvites=false.
	in := defaultSettingsInput()
	in.NotifyInvites = false
	f.st.settings["n-bob"] = store.Settings{
		TZID: in.TZID, WorkStart: in.WorkStart, WorkEnd: in.WorkEnd,
		DefaultReminder: in.DefaultReminder, DefaultView: "agenda",
		WeekStart: 1, DefaultDuration: 30,
		NotifyInvites: false, NotifyChanges: true, NotifyReminders: true,
	}

	cal := f.calOf(t, "n-alice", store.KindPersonal)
	f.create(t, "n-alice", withGuests(cal, "n-bob", "n-carol"))

	got := ofType(rec, events.TypeInvited)
	require.Len(t, got, 1)
	// Bob should be filtered out; only carol should receive.
	assert.Equal(t, []string{"n-carol"}, remindedNodes(got))
}

func TestNotifyChangesSwitchGatesUpdatedNotice(t *testing.T) {
	f := newFixture(t)
	rec := f.noticing()

	cal := f.calOf(t, "n-alice", store.KindPersonal)
	d := f.create(t, "n-alice", withGuests(cal, "n-bob", "n-carol"))

	// Now set bob's NotifyChanges=false.
	f.st.settings["n-bob"] = store.Settings{
		TZID: "Asia/Ho_Chi_Minh", WorkStart: "08:00", WorkEnd: "17:30",
		DefaultReminder: 10, DefaultView: "agenda",
		WeekStart: 1, DefaultDuration: 30,
		NotifyInvites: true, NotifyChanges: false, NotifyReminders: true,
	}

	// Move the event (a change that would normally notify bob).
	start, end := "2026-11-02T10:00:00+07:00", "2026-11-02T11:00:00+07:00"
	_, err := f.svc.UpdateEvent(callerCtx("n-alice"), ws, d.EventID, ScopeQuery{}, EventPatch{Version: 1, Start: &start, End: &end})
	require.NoError(t, err)

	updated := ofType(rec, events.TypeUpdated)
	require.Len(t, updated, 1)
	// Only carol should be notified; bob opted out of changes.
	assert.Equal(t, []string{"n-carol"}, remindedNodes(updated))
}

func TestNotifyChangesSwitchGatesCancelledNotice(t *testing.T) {
	f := newFixture(t)
	rec := f.noticing()

	cal := f.calOf(t, "n-alice", store.KindPersonal)
	d := f.create(t, "n-alice", withGuests(cal, "n-bob", "n-carol"))

	// Set bob's NotifyChanges=false.
	f.st.settings["n-bob"] = store.Settings{
		TZID: "Asia/Ho_Chi_Minh", WorkStart: "08:00", WorkEnd: "17:30",
		DefaultReminder: 10, DefaultView: "agenda",
		WeekStart: 1, DefaultDuration: 30,
		NotifyInvites: true, NotifyChanges: false, NotifyReminders: true,
	}

	// Delete the event (cancellation notice).
	require.NoError(t, f.svc.DeleteEvent(callerCtx("n-alice"), ws, d.EventID, ScopeQuery{}, 0))

	cancelled := ofType(rec, events.TypeCancelled)
	require.Len(t, cancelled, 1)
	// Only carol; bob opted out.
	assert.Equal(t, []string{"n-carol"}, remindedNodes(cancelled))
}

func TestAllNotifySwitchesEnabledSendsToAll(t *testing.T) {
	f := newFixture(t)
	rec := f.noticing()

	// All defaults: all notify switches are true.
	cal := f.calOf(t, "n-alice", store.KindPersonal)
	f.create(t, "n-alice", withGuests(cal, "n-bob", "n-carol"))

	got := ofType(rec, events.TypeInvited)
	require.Len(t, got, 1)
	assert.ElementsMatch(t, []string{"n-bob", "n-carol"}, remindedNodes(got))
}
