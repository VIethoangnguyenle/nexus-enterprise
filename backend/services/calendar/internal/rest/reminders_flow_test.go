package rest

import (
	"context"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/services/calendar/internal/store"
)

// The database half of reminders: an event with reminders is found across the
// workspaces, and however many services claim the same reminder at once, exactly
// one is told it is theirs to send. (The scheduler itself is not run here: it
// reads every workspace of the shared test database.)
func TestReminderClaimsAreExclusiveInTheDatabase(t *testing.T) {
	f := newFlow(t)
	cal := f.personalCal("alice")
	code, ev, raw := f.createEvent("alice", eventBody(cal, `,"reminders":[{"minutesBefore":10},{"minutesBefore":60}]`))
	require.Equal(t, http.StatusCreated, code, string(raw))

	st := store.New(f.pool)
	ctx := context.Background()
	start := time.Date(2026, 11, 2, 2, 0, 0, 0, time.UTC)
	masters, _, err := st.ReminderEvents(ctx, start.Add(-time.Minute), start.Add(time.Hour))
	require.NoError(t, err)
	found := false
	for _, m := range masters {
		found = found || m.ID == ev.EventID
	}
	assert.True(t, found, "an event with reminders is found by its time")

	specs, err := st.ReminderSpecs(ctx, []string{ev.EventID})
	require.NoError(t, err)
	assert.Equal(t, []store.ReminderSpec{{Minutes: 60}, {Minutes: 10}}, specs[ev.EventID], "the event's own, longest first")

	var won atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ok, err := st.ClaimReminder(ctx, ev.EventID, f.nodes["alice"], start, 10)
			assert.NoError(t, err)
			if ok {
				won.Add(1)
			}
		}()
	}
	wg.Wait()
	assert.EqualValues(t, 1, won.Load(), "one claim wins")
	assert.Equal(t, 1, f.count(`SELECT COUNT(*) FROM calendar_reminder_deliveries WHERE event_id = $1`, ev.EventID))

	// Another person, another occurrence or other minutes are separate reminders.
	for _, c := range []struct {
		node string
		at   time.Time
		min  int
	}{{f.nodes["bob"], start, 10}, {f.nodes["alice"], start.AddDate(0, 0, 7), 10}, {f.nodes["alice"], start, 60}} {
		ok, err := st.ClaimReminder(ctx, ev.EventID, c.node, c.at, c.min)
		require.NoError(t, err)
		assert.True(t, ok)
	}
}
