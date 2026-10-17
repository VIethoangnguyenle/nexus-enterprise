package store

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mine keeps the events of this test out of what the shared database also holds.
func (w *world) mine(es []*Event) []string {
	var out []string
	for _, e := range es {
		if strings.HasPrefix(e.ID, "st-ev-"+w.sfx) {
			out = append(out, e.ID)
		}
	}
	return out
}

func TestReminderEventsLoadOnlyWhatMayBeDueNow(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	now := time.Date(2026, 11, 10, 9, 0, 0, 0, time.UTC)
	grace := 10 * time.Minute
	const twoWeeks = 14 * 24 * 60

	near := w.event(0, "Near", now.Add(5*time.Minute), "", 10)
	underway := w.event(1, "Under way, in grace", now.Add(-5*time.Minute), "", 0)
	farOnes := make([]string, 0, 40)
	for i := 0; i < 40; i++ {
		farOnes = append(farOnes, w.event(i%2, "Far", now.Add(72*time.Hour).Add(time.Duration(i)*time.Minute), "", 10, 0).ID)
	}
	longNear := w.event(0, "Two weeks ahead, reminded two weeks before", now.Add(13*24*time.Hour), "", twoWeeks)
	longFar := w.event(0, "Three weeks ahead, reminded two weeks before", now.Add(21*24*time.Hour), "", twoWeeks)
	over := w.event(0, "Over", now.Add(-3*time.Hour), "", 10)
	silent := w.event(0, "No reminder", now.Add(5*time.Minute), "")
	running := w.event(1, "Running series", now.AddDate(0, -3, 0), "FREQ=DAILY", 10)
	notBegun := w.event(1, "Series that begins in a month", now.AddDate(0, 1, 0), "FREQ=DAILY", 10)
	finished := w.event(0, "Finished series", now.AddDate(0, -3, 0), "FREQ=DAILY;COUNT=5", 10)
	w.exec(`UPDATE calendar_events SET recurrence_until = $2 WHERE id = $1`, finished.ID, now.AddDate(0, -3, 5))

	masters, overrides, err := w.st.ReminderEvents(ctx, now.Add(-grace), now)
	require.NoError(t, err)
	assert.Empty(t, w.mine(overrides))
	got := w.mine(masters)
	assert.ElementsMatch(t, []string{near.ID, underway.ID, longNear.ID, running.ID}, got)
	for _, id := range append(farOnes, longFar.ID, over.ID, silent.ID, notBegun.ID, finished.ID) {
		assert.NotContains(t, got, id)
	}
}

func TestReminderEventsKeepAnExceptionMovedNearWithItsSeries(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	now := time.Date(2026, 11, 10, 9, 0, 0, 0, time.UTC)
	series := w.event(0, "Weekly", now.AddDate(0, -2, 0), "FREQ=WEEKLY", 10)
	orig := now.AddDate(0, 0, 3)
	moved := *series
	moved.ID, moved.RecurringEventID, moved.OriginalStart, moved.RRule = series.ID+"-x", series.ID, &orig, ""
	moved.StartsAt, moved.EndsAt = now.Add(5*time.Minute), now.Add(65*time.Minute)
	require.NoError(t, w.st.InsertEvent(ctx, &moved, nil))
	far := moved
	far.ID, far.StartsAt, far.EndsAt = series.ID+"-y", now.AddDate(0, 0, 10), now.AddDate(0, 0, 10).Add(time.Hour)
	farOrig := now.AddDate(0, 0, 10)
	far.OriginalStart = &farOrig
	require.NoError(t, w.st.InsertEvent(ctx, &far, nil))

	masters, overrides, err := w.st.ReminderEvents(ctx, now.Add(-10*time.Minute), now)
	require.NoError(t, err)
	assert.Equal(t, []string{series.ID}, w.mine(masters))
	// Every exception of a series that is loaded comes with it, near or not; the
	// caller expands only the window.
	assert.ElementsMatch(t, []string{moved.ID, far.ID}, w.mine(overrides))
}

func TestClaimingAReminderIsExclusiveAndCanBeTakenBack(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	e := w.event(0, "Standup", day, "", 10)

	ok, err := w.st.ClaimReminder(ctx, e.ID, w.node, day, 10)
	require.NoError(t, err)
	assert.True(t, ok)
	ok, err = w.st.ClaimReminder(ctx, e.ID, w.node, day, 10)
	require.NoError(t, err)
	assert.False(t, ok, "a second claim of the same reminder loses")
	ok, err = w.st.ClaimReminder(ctx, e.ID, w.node, day, 0)
	require.NoError(t, err)
	assert.True(t, ok, "other minutes are another reminder")
	ok, err = w.st.ClaimReminder(ctx, e.ID, "someone-else", day, 10)
	require.NoError(t, err)
	assert.True(t, ok, "and another person")
	ok, err = w.st.ClaimReminder(ctx, e.ID, w.node, day.AddDate(0, 0, 7), 10)
	require.NoError(t, err)
	assert.True(t, ok, "and another occurrence")

	require.NoError(t, w.st.UnclaimReminder(ctx, e.ID, w.node, day, 10))
	ok, err = w.st.ClaimReminder(ctx, e.ID, w.node, day, 10)
	require.NoError(t, err)
	assert.True(t, ok, "a claim taken back can be made again")
	require.NoError(t, w.st.UnclaimReminder(ctx, e.ID, w.node, day, 99), "taking back what was never claimed is not an error")
}
