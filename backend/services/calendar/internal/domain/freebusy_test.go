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

const (
	dayFrom = "2026-11-02T00:00:00+07:00"
	dayTo   = "2026-11-03T00:00:00+07:00"
)

func (f *fixture) freeBusy(t *testing.T, caller string, users ...string) []*PersonFreeBusy {
	t.Helper()
	got, err := f.svc.FreeBusy(callerCtx(caller), ws, users, dayFrom, dayTo)
	require.NoError(t, err)
	return got
}

// onNov2 is an event input from one clock time to another on 2 Nov 2026.
func onNov2(cal, title, from, to string) EventInput {
	in := newInput(cal)
	in.Title, in.Start, in.End = title, "2026-11-02T"+from+":00+07:00", "2026-11-02T"+to+":00+07:00"
	return in
}

func spans(p *PersonFreeBusy) []string {
	var out []string
	for _, b := range p.Busy {
		out = append(out, fmt.Sprintf("%s-%s %s", b.Start[11:16], b.End[11:16], b.Title))
	}
	return out
}

func TestColleaguesSeeWhenSomeoneIsBusyAndNeverWhy(t *testing.T) {
	f := newFixture(t)
	cal := f.calOf(t, "n-alice", store.KindPersonal)
	f.create(t, "n-alice", onNov2(cal, "Budget", "09:00", "10:00"))
	free := onNov2(cal, "Lunch", "12:00", "13:00")
	free.Transparency = TransparencyFree
	f.create(t, "n-alice", free)
	secret := onNov2(cal, "Doctor", "15:00", "16:00")
	secret.Visibility = VisibilityPrivate
	f.create(t, "n-alice", secret)
	f.calOf(t, "n-bob", store.KindPersonal)

	// A colleague with no share: ranges only; free time is not busy time.
	got := f.freeBusy(t, "n-carol", "n-alice", "n-bob")
	require.Len(t, got, 2)
	assert.Equal(t, "Alice", got[0].Person.DisplayName)
	assert.Equal(t, []string{"09:00-10:00 ", "15:00-16:00 "}, spans(got[0]))
	assert.Empty(t, got[1].Busy)
	assert.NotNil(t, got[1].Busy, "an empty list, not null")
	assert.Equal(t, "08:00", got[0].WorkStart, "default working hours")
	assert.Equal(t, "17:30", got[0].WorkEnd)

	// The owner sees her titles; a view share reads all but a private event.
	assert.Equal(t, []string{"09:00-10:00 Budget", "15:00-16:00 Doctor"}, spans(f.freeBusy(t, "n-alice", "n-alice")[0]))
	_, err := f.svc.PutShare(callerCtx("n-alice"), ws, cal, "n-bob", ngac.CalendarShareView)
	require.NoError(t, err)
	assert.Equal(t, []string{"09:00-10:00 Budget", "15:00-16:00 "}, spans(f.freeBusy(t, "n-bob", "n-alice")[0]))
}

func TestFreeBusyOfAnotherWorkspaceIsRefused(t *testing.T) {
	f := newFixture(t)
	f.calOf(t, "n-alice", store.KindPersonal)

	_, err := f.svc.FreeBusy(callerCtx("n-alice"), ws, []string{"n-alice", "n-gus"}, dayFrom, dayTo)
	requireKind(t, err, ErrAccessDenied)
	_, err = f.svc.FreeBusy(callerCtx("n-gus"), ws, []string{"n-alice"}, dayFrom, dayTo)
	requireKind(t, err, ErrAccessDenied)
	_, err = f.svc.SuggestSlots(callerCtx("n-alice"), ws, []string{"n-gus"}, 30, "2026-11-02")
	requireKind(t, err, ErrAccessDenied)
	_, err = f.svc.SuggestSlots(callerCtx("n-gus"), ws, []string{"n-alice"}, 30, "2026-11-02")
	requireKind(t, err, ErrAccessDenied)
	// A person who is on the database list but not in the graph is not a member either.
	f.st.people["n-ghost"] = &store.Person{NodeID: "n-ghost", DisplayName: "Ghost"}
	_, err = f.svc.FreeBusy(callerCtx("n-alice"), ws, []string{"n-ghost"}, dayFrom, dayTo)
	requireKind(t, err, ErrAccessDenied)
}

func TestFreeBusyInputIsChecked(t *testing.T) {
	f := newFixture(t)
	ctx := callerCtx("n-alice")
	for name, args := range map[string][3]string{
		"bad from":  {"n-bob", "soon", dayTo},
		"bad to":    {"n-bob", dayFrom, "later"},
		"reversed":  {"n-bob", dayTo, dayFrom},
		"too long":  {"n-bob", dayFrom, "2027-02-02T00:00:00+07:00"},
		"no people": {"", dayFrom, dayTo},
	} {
		_, err := f.svc.FreeBusy(ctx, ws, []string{args[0]}, args[1], args[2])
		assert.ErrorIs(t, err, ErrInvalidInput, name)
	}
	many := make([]string, maxFreeBusyPeople+1)
	for i := range many {
		many[i] = fmt.Sprintf("n-%d", i)
	}
	_, err := f.svc.FreeBusy(ctx, ws, many, dayFrom, dayTo)
	assert.ErrorIs(t, err, ErrInvalidInput)
}

func TestInvitationsCountAsBusyUnlessDeclined(t *testing.T) {
	f := newFixture(t)
	bobCal := f.calOf(t, "n-bob", store.KindPersonal)
	f.calOf(t, "n-alice", store.KindPersonal)
	d := f.create(t, "n-bob", func() EventInput {
		in := onNov2(bobCal, "Review", "09:00", "10:00")
		in.Attendees = []string{"n-alice"}
		return in
	}())

	// Alice is busy for an event on Bob's calendar, and sees its title as a guest.
	assert.Equal(t, []string{"09:00-10:00 Review"}, spans(f.freeBusy(t, "n-alice", "n-alice")[0]))
	// Carol is a colleague of neither event: ranges only.
	assert.Equal(t, []string{"09:00-10:00 "}, spans(f.freeBusy(t, "n-carol", "n-alice")[0]))

	_, err := f.svc.Rsvp(callerCtx("n-alice"), ws, d.EventID, RsvpInput{Response: "tentative"})
	require.NoError(t, err)
	assert.Len(t, f.freeBusy(t, "n-carol", "n-alice")[0].Busy, 1, "tentative is still busy")
	_, err = f.svc.Rsvp(callerCtx("n-alice"), ws, d.EventID, RsvpInput{Response: "declined"})
	require.NoError(t, err)
	assert.Empty(t, f.freeBusy(t, "n-carol", "n-alice")[0].Busy, "declined frees the time")
	assert.Len(t, f.freeBusy(t, "n-carol", "n-bob")[0].Busy, 1, "the host stays busy")
}

func TestARepeatingEventIsExpandedAndADeclinedOccurrenceFreesOnlyItself(t *testing.T) {
	f := newFixture(t)
	cal := f.calOf(t, "n-alice", store.KindPersonal)
	in := weekly(cal)
	in.Attendees = []string{"n-bob"}
	d := f.create(t, "n-alice", in)
	month := func(who string) []string {
		got, err := f.svc.FreeBusy(callerCtx("n-carol"), ws, []string{who}, rangeStart, rangeEnd)
		require.NoError(t, err)
		var out []string
		for _, b := range got[0].Busy {
			out = append(out, b.Start[:10])
		}
		return out
	}
	assert.Equal(t, []string{"2026-11-02", "2026-11-09", "2026-11-16", "2026-11-23", "2026-11-30"}, month("n-bob"))

	second := f.list(t, "n-bob")[1].OccurrenceStart
	_, err := f.svc.Rsvp(callerCtx("n-bob"), ws, d.EventID, RsvpInput{Response: "declined", Scope: ScopeThis, Occurrence: second})
	require.NoError(t, err)
	assert.Equal(t, []string{"2026-11-02", "2026-11-16", "2026-11-23", "2026-11-30"}, month("n-bob"))
	assert.Len(t, month("n-alice"), 5, "the host is busy on all of them")
}

func TestWorkingHoursComeFromTheSettings(t *testing.T) {
	f := newFixture(t)
	f.calOf(t, "n-alice", store.KindPersonal)
	ctx := callerCtx("n-alice")

	def, err := f.svc.GetSettings(ctx, ws)
	require.NoError(t, err)
	assert.Equal(t, SettingsView{TZID: "Asia/Ho_Chi_Minh", WorkStart: "08:00", WorkEnd: "17:30", DefaultReminder: 10}, *def)

	saved, err := f.svc.UpdateSettings(ctx, ws, SettingsView{TZID: "Asia/Bangkok", WorkStart: "09:30", WorkEnd: "18:00", DefaultReminder: 0})
	require.NoError(t, err)
	assert.Equal(t, "09:30", saved.WorkStart)
	got := f.freeBusy(t, "n-bob", "n-alice")[0]
	assert.Equal(t, "09:30", got.WorkStart)
	assert.Equal(t, "18:00", got.WorkEnd)
	cal := f.calOf(t, "n-alice", store.KindPersonal)
	made := f.create(t, "n-alice", newInput(cal))
	assert.Empty(t, made.Reminders, "the saved default reminder (none) applies to a new event")

	for name, bad := range map[string]SettingsView{
		"zone":      {TZID: "Mars/Olympus", WorkStart: "08:00", WorkEnd: "17:00"},
		"no zone":   {WorkStart: "08:00", WorkEnd: "17:00"},
		"clock":     {TZID: "Asia/Bangkok", WorkStart: "8am", WorkEnd: "17:00"},
		"one digit": {TZID: "Asia/Bangkok", WorkStart: "8:00", WorkEnd: "17:00"},
		"backward":  {TZID: "Asia/Bangkok", WorkStart: "17:00", WorkEnd: "08:00"},
		"empty day": {TZID: "Asia/Bangkok", WorkStart: "08:00", WorkEnd: "08:00"},
		"reminder":  {TZID: "Asia/Bangkok", WorkStart: "08:00", WorkEnd: "17:00", DefaultReminder: -1},
	} {
		_, err := f.svc.UpdateSettings(ctx, ws, bad)
		assert.ErrorIs(t, err, ErrInvalidInput, name)
	}
	_, err = f.svc.UpdateSettings(callerCtx("n-gus"), ws, *saved)
	requireKind(t, err, ErrAccessDenied)
	_, err = f.svc.GetSettings(callerCtx("n-gus"), ws)
	requireKind(t, err, ErrAccessDenied)
}

func slotsOf(slots []TimeSlot) []string {
	out := []string{}
	for _, s := range slots {
		out = append(out, s.Start[11:16]+"-"+s.End[11:16])
	}
	return out
}

func TestSuggestedTimesAreTheEarliestEveryoneIsFree(t *testing.T) {
	f := newFixture(t)
	aliceCal := f.calOf(t, "n-alice", store.KindPersonal)
	bobCal := f.calOf(t, "n-bob", store.KindPersonal)
	f.create(t, "n-alice", onNov2(aliceCal, "A", "09:00", "10:00"))
	f.create(t, "n-bob", onNov2(bobCal, "B", "10:00", "11:30"))
	ctx := callerCtx("n-alice")

	got, err := f.svc.SuggestSlots(ctx, ws, []string{"n-alice", "n-bob"}, 60, "2026-11-02")
	require.NoError(t, err)
	assert.Equal(t, []string{"08:00-09:00", "11:30-12:30", "12:00-13:00"}, slotsOf(got), "at most three, on the half hour")
	assert.Equal(t, "2026-11-02T08:00:00+07:00", got[0].Start)

	// One person alone has more room; a free event does not block.
	free := onNov2(aliceCal, "Lunch", "08:00", "09:00")
	free.Transparency = TransparencyFree
	f.create(t, "n-alice", free)
	got, err = f.svc.SuggestSlots(ctx, ws, []string{"n-alice"}, 30, "2026-11-02")
	require.NoError(t, err)
	assert.Equal(t, []string{"08:00-08:30", "08:30-09:00", "10:00-10:30"}, slotsOf(got))

	// The last slot must end inside the working day (17:30).
	late := onNov2(aliceCal, "Late", "08:00", "17:00")
	f.create(t, "n-alice", late)
	got, err = f.svc.SuggestSlots(ctx, ws, []string{"n-alice"}, 30, "2026-11-02")
	require.NoError(t, err)
	assert.Equal(t, []string{"17:00-17:30"}, slotsOf(got))
	got, err = f.svc.SuggestSlots(ctx, ws, []string{"n-alice"}, 60, "2026-11-02")
	require.NoError(t, err)
	assert.Equal(t, []string{}, slotsOf(got), "nothing fits: an empty list, not an error")
}

func TestSuggestedTimesStayInsideEveryonesWorkingHours(t *testing.T) {
	f := newFixture(t)
	f.calOf(t, "n-alice", store.KindPersonal)
	f.calOf(t, "n-bob", store.KindPersonal)
	_, err := f.svc.UpdateSettings(callerCtx("n-bob"), ws, SettingsView{TZID: "Asia/Ho_Chi_Minh", WorkStart: "10:15", WorkEnd: "11:00", DefaultReminder: 10})
	require.NoError(t, err)

	got, err := f.svc.SuggestSlots(callerCtx("n-alice"), ws, []string{"n-alice", "n-bob"}, 30, "2026-11-02")
	require.NoError(t, err)
	assert.Equal(t, []string{"10:30-11:00"}, slotsOf(got), "Bob works from 10:15; the grid is the half hour")

	// Different zones: Bob's 08:00-17:30 in Berlin is the middle of the night in Saigon.
	_, err = f.svc.UpdateSettings(callerCtx("n-bob"), ws, SettingsView{TZID: "Europe/Berlin", WorkStart: "08:00", WorkEnd: "17:30", DefaultReminder: 10})
	require.NoError(t, err)
	got, err = f.svc.SuggestSlots(callerCtx("n-alice"), ws, []string{"n-alice", "n-bob"}, 30, "2026-11-02")
	require.NoError(t, err)
	assert.Equal(t, []string{"14:00-14:30", "14:30-15:00", "15:00-15:30"}, slotsOf(got),
		"14:00 in Saigon is 08:00 in Berlin, the first moment both are at work")
}

func TestSuggestInputIsChecked(t *testing.T) {
	f := newFixture(t)
	ctx := callerCtx("n-alice")
	for name, c := range map[string]struct {
		users []string
		min   int
		day   string
	}{
		"short":    {[]string{"n-bob"}, 1, "2026-11-02"},
		"long":     {[]string{"n-bob"}, 721, "2026-11-02"},
		"bad day":  {[]string{"n-bob"}, 30, "tomorrow"},
		"no users": {nil, 30, "2026-11-02"},
	} {
		_, err := f.svc.SuggestSlots(ctx, ws, c.users, c.min, c.day)
		assert.ErrorIs(t, err, ErrInvalidInput, name)
	}
}

func TestOnGridRoundsUpToTheHalfHour(t *testing.T) {
	loc, _ := time.LoadLocation("Asia/Kathmandu") // +05:45: the grid is the local clock's
	base := time.Date(2026, 11, 2, 9, 0, 0, 0, loc)
	assert.Equal(t, base, onGrid(base, loc))
	assert.Equal(t, base.Add(30*time.Minute), onGrid(base.Add(time.Minute), loc))
	assert.Equal(t, base.Add(30*time.Minute), onGrid(base.Add(29*time.Minute+59*time.Second), loc))
	assert.Equal(t, base.Add(time.Hour), onGrid(base.Add(31*time.Minute), loc))
}
