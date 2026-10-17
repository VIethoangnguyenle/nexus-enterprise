package store

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	from = time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	to   = time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)
	day  = time.Date(2026, 11, 10, 9, 0, 0, 0, time.UTC)
)

func TestRangeAttendedStaysInItsWorkspace(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	here := w.event(0, "Here", day, "")
	there := w.event(1, "There", day, "")
	series := w.event(1, "Weekly there", day, "FREQ=WEEKLY")

	masters, overrides, err := w.st.RangeAttended(ctx, w.ws[0], w.node, from, to)
	require.NoError(t, err)
	assert.Equal(t, []string{here.ID}, ids(masters))
	assert.Empty(t, overrides)

	masters, _, err = w.st.RangeAttended(ctx, w.ws[1], w.node, from, to)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{there.ID, series.ID}, ids(masters))

	masters, _, err = w.st.RangeAttended(ctx, w.ws[0], "someone-else", from, to)
	require.NoError(t, err)
	assert.Empty(t, masters, "only the events a person attends")
}

func TestRangeAttendedKeepsAMovedExceptionWithItsSeriesAndNothingElse(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	series := w.event(0, "Weekly", time.Date(2026, 9, 1, 9, 0, 0, 0, time.UTC), "FREQ=WEEKLY")
	orig := time.Date(2026, 9, 8, 9, 0, 0, 0, time.UTC)
	moved := *series
	moved.ID, moved.RecurringEventID, moved.OriginalStart, moved.RRule = series.ID+"-x", series.ID, &orig, ""
	moved.StartsAt, moved.EndsAt, moved.Version = day, day.Add(time.Hour), 1
	require.NoError(t, w.st.InsertEvent(ctx, &moved, nil))
	// The same shape in the other workspace, for the same person.
	other := w.event(1, "Weekly there", time.Date(2026, 9, 1, 9, 0, 0, 0, time.UTC), "FREQ=WEEKLY")
	otherMoved := moved
	otherMoved.ID, otherMoved.WorkspaceID, otherMoved.CalendarID, otherMoved.RecurringEventID = other.ID+"-x", w.ws[1], w.cal[1], other.ID
	require.NoError(t, w.st.InsertEvent(ctx, &otherMoved, nil))

	masters, overrides, err := w.st.RangeAttended(ctx, w.ws[0], w.node, from, to)
	require.NoError(t, err)
	assert.Equal(t, []string{series.ID}, ids(masters), "the series of an exception moved into the range comes with it")
	assert.Equal(t, []string{moved.ID}, ids(overrides))
}

func TestRangeEventsCoversTheNamedCalendarsOnly(t *testing.T) {
	w := newWorld(t)
	here := w.event(0, "Here", day, "")
	w.event(1, "There", day, "")
	w.event(0, "Out of range", day.AddDate(1, 0, 0), "")
	finished := w.event(0, "Finished series", time.Date(2026, 1, 5, 9, 0, 0, 0, time.UTC), "FREQ=WEEKLY;COUNT=2")
	w.exec(`UPDATE calendar_events SET recurrence_until = $2 WHERE id = $1`, finished.ID, time.Date(2026, 1, 13, 0, 0, 0, 0, time.UTC))

	masters, _, err := w.st.RangeEvents(context.Background(), []string{w.cal[0]}, from, to)
	require.NoError(t, err)
	assert.Equal(t, []string{here.ID}, ids(masters))
	masters, _, err = w.st.RangeEvents(context.Background(), nil, from, to)
	require.NoError(t, err)
	assert.Empty(t, masters)
}

func TestSearchEventsStaysInItsWorkspace(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	here := w.event(0, "Budget review", day, "")
	w.event(1, "Budget review elsewhere", day, "")

	// By calendar and by attendance alike: the person attends both.
	got, err := w.st.SearchEvents(ctx, w.ws[0], []string{w.cal[0]}, w.node, "budget", 10)
	require.NoError(t, err)
	assert.Equal(t, []string{here.ID}, ids(got))
	got, err = w.st.SearchEvents(ctx, w.ws[0], nil, w.node, "budget", 10)
	require.NoError(t, err)
	assert.Equal(t, []string{here.ID}, ids(got))
	got, err = w.st.SearchEvents(ctx, w.ws[0], []string{w.cal[1]}, "someone-else", "budget", 10)
	require.NoError(t, err)
	assert.Empty(t, got, "a calendar of another workspace is not searched")

	got, err = w.st.SearchEvents(ctx, w.ws[0], nil, w.node, "100%_", 10)
	require.NoError(t, err)
	assert.Empty(t, got, "pattern characters in the term are literal")
}

// Searching events by a guest's name matches the people who are in the workspace;
// a guest who was removed from it can no longer be found by their name.
func TestSearchEventsByGuestNameSkipsRemovedMembers(t *testing.T) {
	w := newWorld(t)
	ctx := context.Background()
	guestUser, guestNode := "st-guest-"+w.sfx, "st-gnode-"+w.sfx
	w.exec(`INSERT INTO ngac_nodes (id, name, node_type) VALUES ($1, $1, 'U')`, guestNode)
	w.exec(`INSERT INTO users (id, username, password, ngac_node, display_name) VALUES ($1, $1, '', $2, $3)`, guestUser, guestNode, "Zorbaline "+w.sfx)
	w.exec(`INSERT INTO tenant_users (tenant_id, user_id, ngac_node_id) VALUES ($1, $2, $3)`, w.ws[0], guestUser, guestNode)
	t.Cleanup(func() {
		c := context.Background()
		w.pool.Exec(c, `DELETE FROM tenant_users WHERE user_id = $1`, guestUser)
		w.pool.Exec(c, `DELETE FROM users WHERE id = $1`, guestUser)
		w.pool.Exec(c, `DELETE FROM ngac_nodes WHERE id = $1`, guestNode)
	})
	ev := w.event(0, "Planning", day, "")
	w.exec(`INSERT INTO calendar_attendees (event_id, user_node_id) VALUES ($1, $2)`, ev.ID, guestNode)

	got, err := w.st.SearchEvents(ctx, w.ws[0], []string{w.cal[0]}, w.node, "Zorbaline", 10)
	require.NoError(t, err)
	assert.Equal(t, []string{ev.ID}, ids(got), "a member guest is found by name")

	w.exec(`UPDATE tenant_users SET status = 'removed' WHERE tenant_id = $1 AND user_id = $2`, w.ws[0], guestUser)
	got, err = w.st.SearchEvents(ctx, w.ws[0], []string{w.cal[0]}, w.node, "Zorbaline", 10)
	require.NoError(t, err)
	assert.Empty(t, got, "a removed guest is no longer found by name")

	got, err = w.st.SearchEvents(ctx, w.ws[0], []string{w.cal[0]}, w.node, "Planning", 10)
	require.NoError(t, err)
	assert.Equal(t, []string{ev.ID}, ids(got), "the event is still found by its title")
}
