package domain

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/services/calendar/internal/events"
	"ngac-platform/services/calendar/internal/store"
)

// noticing turns on notices and gives every person an account, as the database
// has, and returns what is announced.
func (f *fixture) noticing() *events.Recorder {
	for node, p := range f.st.people {
		p.UserID = "user-" + node
	}
	rec := &events.Recorder{}
	f.svc.SetNotices(rec)
	return rec
}

func ofType(rec *events.Recorder, typ string) []events.Notice {
	var out []events.Notice
	for _, n := range rec.Notices() {
		if n.Type == typ {
			out = append(out, n)
		}
	}
	return out
}

func remindedNodes(ns []events.Notice) []string {
	var out []string
	for _, n := range ns {
		for _, r := range n.Recipients {
			out = append(out, r.NodeID)
		}
	}
	return out
}

func remindIn(minutes ...int) *[]ReminderInput {
	list := make([]ReminderInput, len(minutes))
	for i, m := range minutes {
		list[i] = ReminderInput{MinutesBefore: m}
	}
	return &list
}

// the first Monday standup is 2026-11-02 09:00 +07 = 02:00 UTC.
var standup = time.Date(2026, 11, 2, 2, 0, 0, 0, time.UTC)

func (f *fixture) at(t time.Time) { f.svc.now = func() time.Time { return t } }

func TestAReminderFiresOnceForEachPersonWhoHasNotDeclined(t *testing.T) {
	f := newFixture(t)
	rec := f.noticing()
	cal := f.calOf(t, "n-alice", store.KindPersonal)
	in := withGuests(cal, "n-bob", "n-carol")
	in.Reminders = remindIn(10)
	d := f.create(t, "n-alice", in)
	_, err := f.svc.Rsvp(callerCtx("n-carol"), ws, d.EventID, RsvpInput{Response: ResponseDeclined})
	require.NoError(t, err)

	f.at(standup.Add(-10 * time.Minute))
	n, err := f.svc.FireReminders(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 2, n)
	got := ofType(rec, events.TypeReminder)
	assert.ElementsMatch(t, []string{"n-alice", "n-bob"}, remindedNodes(got), "the host and the guest who has not declined; not carol")
	for _, r := range got {
		assert.Equal(t, d.EventID, r.EventID)
		assert.Equal(t, "Standup", r.Title)
		assert.Equal(t, "2026-11-02T02:00:00Z", r.Start)
		assert.Equal(t, "2026-11-02T02:00:00Z", r.OccurrenceStart)
		require.NotNil(t, r.MinutesBefore)
		assert.Equal(t, 10, *r.MinutesBefore)
		assert.Equal(t, f.w.NodeNamed("OA", eventOA(d.EventID)), r.OAID, "recipients are re-checked against the event's own node")
		assert.Empty(t, r.ActorUserID, "nobody caused a reminder")
	}

	// A second pass, and a second service on the same database, add nothing.
	f.at(standup.Add(-9 * time.Minute))
	n, err = f.svc.FireReminders(context.Background())
	require.NoError(t, err)
	assert.Zero(t, n)
	other := NewService(f.st, f.w.Read(), f.w.Write(), f.rec)
	other.SetNotices(rec)
	other.now = f.svc.now
	n, err = other.FireReminders(context.Background())
	require.NoError(t, err)
	assert.Zero(t, n)
	assert.Len(t, ofType(rec, events.TypeReminder), 2)
}

func TestTwoServicesRunningTogetherSendEachReminderOnce(t *testing.T) {
	f := newFixture(t)
	rec := f.noticing()
	cal := f.calOf(t, "n-alice", store.KindPersonal)
	in := withGuests(cal, "n-bob")
	in.Reminders = remindIn(10)
	f.create(t, "n-alice", in)

	other := NewService(f.st, f.w.Read(), f.w.Write(), f.rec)
	other.SetNotices(rec)
	f.at(standup.Add(-5 * time.Minute))
	other.now = f.svc.now
	done := make(chan struct{})
	for _, s := range []*Service{f.svc, other} {
		go func() {
			defer func() { done <- struct{}{} }()
			_, err := s.FireReminders(context.Background())
			assert.NoError(t, err)
		}()
	}
	<-done
	<-done
	assert.Len(t, ofType(rec, events.TypeReminder), 2, "one each for alice and bob")
}

func TestAReminderFiresOnlyInItsWindow(t *testing.T) {
	f := newFixture(t)
	rec := f.noticing()
	cal := f.calOf(t, "n-alice", store.KindPersonal)
	in := newInput(cal)
	in.Reminders = remindIn(30, 0)
	f.create(t, "n-alice", in)

	for _, tc := range []struct {
		name string
		now  time.Time
		want int
	}{
		{"too early", standup.Add(-31 * time.Minute), 0},
		{"the 30 minute reminder", standup.Add(-25 * time.Minute), 1},
		{"nothing new in between", standup.Add(-5 * time.Minute), 0},
		{"at the start", standup, 1},
	} {
		f.at(tc.now)
		n, err := f.svc.FireReminders(context.Background())
		require.NoError(t, err, tc.name)
		assert.Equal(t, tc.want, n, tc.name)
	}
	assert.Len(t, ofType(rec, events.TypeReminder), 2)

	// Long past its time a reminder is not sent: the event is under way.
	f2 := newFixture(t)
	rec2 := f2.noticing()
	in2 := newInput(f2.calOf(t, "n-alice", store.KindPersonal))
	in2.Reminders = remindIn(10)
	f2.create(t, "n-alice", in2)
	f2.at(standup.Add(-10*time.Minute + reminderGrace + time.Minute))
	n, err := f2.svc.FireReminders(context.Background())
	require.NoError(t, err)
	assert.Zero(t, n)
	assert.Empty(t, ofType(rec2, events.TypeReminder))
}

func TestARepeatingEventRemindsOccurrenceByOccurrence(t *testing.T) {
	f := newFixture(t)
	rec := f.noticing()
	cal := f.calOf(t, "n-alice", store.KindPersonal)
	in := weekly(cal)
	in.Attendees = []string{"n-bob"}
	in.Reminders = remindIn(10)
	d := f.create(t, "n-alice", in)

	nextMonday := standup.AddDate(0, 0, 7)
	f.at(nextMonday.Add(-10 * time.Minute))
	n, err := f.svc.FireReminders(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 2, n)
	for _, r := range ofType(rec, events.TypeReminder) {
		assert.Equal(t, d.EventID, r.EventID, "the series")
		assert.Equal(t, "2026-11-09T02:00:00Z", r.OccurrenceStart)
		assert.Equal(t, "2026-11-09T02:00:00Z", r.Start)
	}

	// The week after is a new occurrence and a new reminder.
	f.at(nextMonday.AddDate(0, 0, 7).Add(-10 * time.Minute))
	n, err = f.svc.FireReminders(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 2, n)
}

func TestAnAnswerToOneOccurrenceSilencesOnlyThatOne(t *testing.T) {
	f := newFixture(t)
	rec := f.noticing()
	cal := f.calOf(t, "n-alice", store.KindPersonal)
	in := weekly(cal)
	in.Attendees = []string{"n-bob"}
	in.Reminders = remindIn(10)
	d := f.create(t, "n-alice", in)
	_, err := f.svc.Rsvp(callerCtx("n-bob"), ws, d.EventID, RsvpInput{Response: ResponseDeclined, Scope: ScopeThis, Occurrence: "2026-11-02T09:00:00+07:00"})
	require.NoError(t, err)

	f.at(standup.Add(-10 * time.Minute))
	_, err = f.svc.FireReminders(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []string{"n-alice"}, remindedNodes(ofType(rec, events.TypeReminder)), "bob declined this occurrence")

	f.at(standup.AddDate(0, 0, 7).Add(-10 * time.Minute))
	_, err = f.svc.FireReminders(context.Background())
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"n-alice", "n-alice", "n-bob"}, remindedNodes(ofType(rec, events.TypeReminder)), "but the next one reminds him")
}

func TestAGuestTakenOffTheEventIsNotReminded(t *testing.T) {
	f := newFixture(t)
	rec := f.noticing()
	cal := f.calOf(t, "n-alice", store.KindPersonal)
	in := withGuests(cal, "n-bob", "n-carol")
	in.Reminders = remindIn(10)
	d := f.create(t, "n-alice", in)
	keep := []string{"n-bob"}
	_, err := f.svc.UpdateEvent(callerCtx("n-alice"), ws, d.EventID, ScopeQuery{}, EventPatch{Version: 1, Attendees: &keep})
	require.NoError(t, err)

	f.at(standup.Add(-10 * time.Minute))
	_, err = f.svc.FireReminders(context.Background())
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"n-alice", "n-bob"}, remindedNodes(ofType(rec, events.TypeReminder)))
}

func TestAPersonsOwnRemindersReplaceTheEventsOwn(t *testing.T) {
	f := newFixture(t)
	rec := f.noticing()
	cal := f.calOf(t, "n-alice", store.KindPersonal)
	in := withGuests(cal, "n-bob")
	in.Reminders = remindIn(10)
	d := f.create(t, "n-alice", in)
	f.st.userReminders = map[string]map[string][]int{d.EventID: {"n-bob": {60}}}

	f.at(standup.Add(-10 * time.Minute))
	_, err := f.svc.FireReminders(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []string{"n-alice"}, remindedNodes(ofType(rec, events.TypeReminder)), "bob asked for an hour before, not ten minutes")
	f.at(standup.Add(-60 * time.Minute))
	_, err = f.svc.FireReminders(context.Background())
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"n-alice", "n-bob"}, remindedNodes(ofType(rec, events.TypeReminder)))
}

func TestACancelledOccurrenceIsNotReminded(t *testing.T) {
	f := newFixture(t)
	rec := f.noticing()
	cal := f.calOf(t, "n-alice", store.KindPersonal)
	in := weekly(cal)
	in.Reminders = remindIn(10)
	d := f.create(t, "n-alice", in)
	require.NoError(t, f.svc.DeleteEvent(callerCtx("n-alice"), ws, d.EventID, ScopeQuery{Scope: ScopeThis, Occurrence: "2026-11-02T09:00:00+07:00"}, 0))

	f.at(standup.Add(-10 * time.Minute))
	n, err := f.svc.FireReminders(context.Background())
	require.NoError(t, err)
	assert.Zero(t, n)
	assert.Empty(t, ofType(rec, events.TypeReminder))
}

func TestAReminderWhoseNoticeCouldNotBePublishedIsRetried(t *testing.T) {
	f := newFixture(t)
	rec := f.noticing()
	cal := f.calOf(t, "n-alice", store.KindPersonal)
	in := withGuests(cal, "n-bob")
	in.Reminders = remindIn(10)
	f.create(t, "n-alice", in)
	f.at(standup.Add(-10 * time.Minute))

	rec.Err = errors.New("broker down")
	n, err := f.svc.FireReminders(context.Background())
	require.NoError(t, err)
	assert.Zero(t, n, "nothing was delivered")
	assert.Empty(t, ofType(rec, events.TypeReminder))
	assert.Empty(t, f.st.claimed, "and no claim is left behind to hide it")

	// The broker is back on the next pass, still within the grace.
	rec.Err = nil
	f.at(standup.Add(-9 * time.Minute))
	n, err = f.svc.FireReminders(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 2, n)
	assert.ElementsMatch(t, []string{"n-alice", "n-bob"}, remindedNodes(ofType(rec, events.TypeReminder)))
	n, err = f.svc.FireReminders(context.Background())
	require.NoError(t, err)
	assert.Zero(t, n, "and once delivered, not again")
}

func TestWithoutABrokerNoReminderIsClaimed(t *testing.T) {
	f := newFixture(t)
	cal := f.calOf(t, "n-alice", store.KindPersonal)
	in := newInput(cal)
	in.Reminders = remindIn(10)
	f.create(t, "n-alice", in)
	f.at(standup.Add(-10 * time.Minute))

	n, err := f.svc.FireReminders(context.Background())
	require.NoError(t, err)
	assert.Zero(t, n)
	assert.Empty(t, f.st.claimed, "a reminder nobody can send stays due")

	rec := f.noticing()
	n, err = f.svc.FireReminders(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	assert.Len(t, ofType(rec, events.TypeReminder), 1)
}

func TestAPassLoadsOnlyEventsNearEnoughToBeDue(t *testing.T) {
	f := newFixture(t)
	rec := f.noticing()
	cal := f.calOf(t, "n-alice", store.KindPersonal)
	for i := 0; i < 30; i++ {
		far := newInput(cal)
		far.Start, far.End = "2026-12-15T09:00:00+07:00", "2026-12-15T10:00:00+07:00"
		far.Reminders = remindIn(10)
		f.create(t, "n-alice", far)
	}
	long := newInput(cal)
	long.Title, long.Start, long.End = "Offsite", "2026-11-16T09:00:00+07:00", "2026-11-16T10:00:00+07:00"
	long.Reminders = remindIn(14 * 24 * 60)
	f.create(t, "n-alice", long)

	// Two weeks before the offsite, the long reminder is due and only it is loaded.
	f.at(time.Date(2026, 11, 16, 2, 0, 0, 0, time.UTC).Add(-14 * 24 * time.Hour))
	n, err := f.svc.FireReminders(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	assert.Equal(t, 1, f.st.loaded, "the 30 events weeks away are left in the database")
	got := ofType(rec, events.TypeReminder)
	require.Len(t, got, 1)
	assert.Equal(t, "Offsite", got[0].Title)
	require.NotNil(t, got[0].MinutesBefore)
	assert.Equal(t, 14*24*60, *got[0].MinutesBefore)
}
