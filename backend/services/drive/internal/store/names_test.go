package store_test

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/services/drive/internal/store"
)

func testDBURL() string {
	if url := os.Getenv("TEST_DATABASE_URL"); url != "" {
		return url
	}
	return "postgres://ngac:ngac_secret@localhost:5432/ngac?sslmode=disable"
}

func newStore(t *testing.T) (*store.Store, *pgxpool.Pool) {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), testDBURL())
	require.NoError(t, err)
	if err := pool.Ping(context.Background()); err != nil {
		pool.Close()
		t.Skipf("test DB not available: %v", err)
	}
	t.Cleanup(pool.Close)
	return store.NewStore(pool), pool
}

// A drive item records its owner as a user id (files) or as an NGAC node id
// (folders). Both must resolve to the person's display name, and a person
// without one falls back to their username so the UI never shows an id.
func TestDisplayNames_ResolvesUserIDAndNodeID(t *testing.T) {
	s, pool := newStore(t)
	ctx := context.Background()
	sfx := uuid.NewString()
	named := "names-test-named-" + sfx
	namedNode := "names-test-node-" + sfx
	bare := "names-test-bare-" + sfx

	// users.ngac_node references ngac_nodes.
	_, err := pool.Exec(ctx, `INSERT INTO ngac_nodes (id, name, node_type) VALUES ($1, $1, 'U')`, namedNode)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO users (id, username, password, ngac_node, display_name) VALUES ($1, $1, '', $2, 'Lê Thị Hoa')`, named, namedNode)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO users (id, username, password, display_name) VALUES ($1, $1, '', '')`, bare)
	require.NoError(t, err)
	t.Cleanup(func() {
		c := context.Background()
		_, _ = pool.Exec(c, `DELETE FROM users WHERE id = ANY($1)`, []string{named, bare})
		_, _ = pool.Exec(c, `DELETE FROM ngac_nodes WHERE id = $1`, namedNode)
	})

	got, err := s.DisplayNames(ctx, []string{named, namedNode, bare, "system", "names-test-unknown-" + sfx})
	require.NoError(t, err)

	assert.Equal(t, "Lê Thị Hoa", got[named], "by user id")
	assert.Equal(t, "Lê Thị Hoa", got[namedNode], "by NGAC node id")
	assert.Equal(t, bare, got[bare], "no display name falls back to username")
	_, hasSystem := got["system"]
	assert.False(t, hasSystem, "an owner that is not a person has no name")
	assert.Len(t, got, 3)
}

func TestDisplayNames_EmptyInput(t *testing.T) {
	s, _ := newStore(t)
	got, err := s.DisplayNames(context.Background(), nil)
	require.NoError(t, err)
	assert.Empty(t, got)
}

// Breadcrumbs run from the top of the tree to the folder asked about, whatever
// the ids sort like. Ids here are chosen so that ordering by id gives a
// wrong answer.
func TestGetBreadcrumb_OrderedRootFirst(t *testing.T) {
	s, pool := newStore(t)
	ctx := context.Background()
	sfx := uuid.NewString()
	ws := "crumb-test-ws-" + sfx
	owner := "crumb-test-owner-" + sfx

	_, err := pool.Exec(ctx, `INSERT INTO users (id, username, password) VALUES ($1, $1, '')`, owner)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO workspaces (id, name, owner_id) VALUES ($1, $1, $2)`, ws, owner)
	require.NoError(t, err)
	t.Cleanup(func() {
		c := context.Background()
		_, _ = pool.Exec(c, `DELETE FROM drive_items WHERE workspace_id = $1`, ws)
		_, _ = pool.Exec(c, `DELETE FROM workspaces WHERE id = $1`, ws)
		_, _ = pool.Exec(c, `DELETE FROM users WHERE id = $1`, owner)
	})

	// a (top) -> m -> z (deepest): ordered by id the chain would read from the leaf up.
	ids := map[string]string{"top": "a-" + sfx, "mid": "m-" + sfx, "leaf": "z-" + sfx}
	insert := func(id, name string, parent *string) {
		require.NoError(t, s.InsertItem(ctx, &store.DriveItem{
			ID: id, WorkspaceID: ws, DriveContext: "workspace", ParentID: parent, ItemType: "folder",
			Name: name, NGACNodeID: "node-" + id, OwnerID: owner, Status: "active",
		}))
	}
	insert(ids["top"], "Đối soát", nil)
	insert(ids["mid"], "2026", ptr(ids["top"]))
	insert(ids["leaf"], "Tháng 10", ptr(ids["mid"]))

	crumbs, err := s.GetBreadcrumb(ctx, ids["leaf"])
	require.NoError(t, err)
	names := make([]string, len(crumbs))
	for i, c := range crumbs {
		names[i] = c.Name
	}
	assert.Equal(t, []string{"Đối soát", "2026", "Tháng 10"}, names)
}

func ptr(s string) *string { return &s }
