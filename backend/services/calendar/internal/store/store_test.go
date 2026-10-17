package store

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// The store against the test database; skipped when it is not there. Two
// workspaces share one person's node, so a query that forgets the workspace
// shows in what it returns.

type world struct {
	t    *testing.T
	pool *pgxpool.Pool
	st   *Store
	sfx  string
	ws   [2]string
	cal  [2]string
	user string // the owner row both workspaces need
	node string // the person who attends events in both workspaces
}

func newWorld(t *testing.T) *world {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		url = "postgres://ngac:ngac_secret@localhost:5433/ngac?sslmode=disable"
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Skipf("test DB not available: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Skipf("test DB not available: %v", err)
	}
	t.Cleanup(pool.Close)
	if _, err := pool.Exec(ctx, `SELECT 1 FROM calendar_reminder_deliveries LIMIT 0`); err != nil {
		t.Skipf("calendar schema not applied (make db-migrate): %v", err)
	}

	w := &world{t: t, pool: pool, st: New(pool), sfx: uuid.NewString()[:8]}
	w.user, w.node = "st-user-"+w.sfx, "st-node-"+w.sfx
	w.exec(`INSERT INTO users (id, username, password, display_name) VALUES ($1, $1, '', 'Store Tester')`, w.user)
	for i := range w.ws {
		w.ws[i], w.cal[i] = fmt.Sprintf("st-ws%d-%s", i, w.sfx), fmt.Sprintf("st-cal%d-%s", i, w.sfx)
		oa, pc := fmt.Sprintf("st-oa%d-%s", i, w.sfx), fmt.Sprintf("st-pc%d-%s", i, w.sfx)
		w.exec(`INSERT INTO ngac_nodes (id, name, node_type) VALUES ($1, $1, 'PC'), ($2, $2, 'OA')`, pc, oa)
		w.exec(`INSERT INTO workspaces (id, name, owner_id, type, ngac_pc_id) VALUES ($1, $1, $2, 'organization', $3)`, w.ws[i], w.user, pc)
		w.exec(`INSERT INTO calendars (id, workspace_id, kind, owner_node_id, ngac_oa_id) VALUES ($1, $2, 'personal', $3, $4)`,
			w.cal[i], w.ws[i], fmt.Sprintf("st-owner%d-%s", i, w.sfx), oa)
	}
	t.Cleanup(func() {
		c := context.Background()
		for i := range w.ws {
			pool.Exec(c, `DELETE FROM workspaces WHERE id = $1`, w.ws[i])
			pool.Exec(c, `DELETE FROM ngac_nodes WHERE id IN ($1, $2)`, fmt.Sprintf("st-oa%d-%s", i, w.sfx), fmt.Sprintf("st-pc%d-%s", i, w.sfx))
		}
		pool.Exec(c, `DELETE FROM users WHERE id = $1`, w.user)
	})
	return w
}

func (w *world) exec(q string, args ...any) {
	w.t.Helper()
	_, err := w.pool.Exec(context.Background(), q, args...)
	require.NoError(w.t, err, q)
}

// event stores a single event or series master in workspace i, with node as an
// attendee, and returns it.
func (w *world) event(i int, title string, start time.Time, rrule string, reminders ...int) *Event {
	w.t.Helper()
	e := &Event{
		ID: fmt.Sprintf("st-ev-%s-%s", w.sfx, uuid.NewString()[:8]), WorkspaceID: w.ws[i], CalendarID: w.cal[i],
		OrganizerNodeID: w.node, Title: title, StartsAt: start, EndsAt: start.Add(time.Hour), TZID: "UTC",
		RRule: rrule, Kind: "event", Transparency: "busy", Visibility: "default", GuestsCanSeeList: true,
	}
	if rrule != "" {
		e.RecurrenceUntil = nil
	}
	require.NoError(w.t, w.st.InsertEvent(context.Background(), e, reminders))
	return e
}

func ids(es []*Event) []string {
	out := make([]string, len(es))
	for i, e := range es {
		out[i] = e.ID
	}
	return out
}
