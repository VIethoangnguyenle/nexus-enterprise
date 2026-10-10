package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/services/drive/internal/store"
)

// treeFixture inserts a chain top -> mid -> leaf of folders in a workspace of
// its own and returns their ids.
func treeFixture(t *testing.T, s *store.Store, pool *pgxpool.Pool) (top, mid, leaf string) {
	t.Helper()
	ctx := context.Background()
	sfx := uuid.NewString()
	ws := "tree-test-ws-" + sfx
	owner := "tree-test-owner-" + sfx
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
	top, mid, leaf = "t-"+sfx, "m-"+sfx, "l-"+sfx
	insert := func(id string, parent *string) {
		require.NoError(t, s.InsertItem(ctx, &store.DriveItem{
			ID: id, WorkspaceID: ws, DriveContext: "workspace", ParentID: parent, ItemType: "folder",
			Name: id, NGACNodeID: "node-" + id, OwnerID: owner, Status: "active",
		}))
	}
	insert(top, nil)
	insert(mid, ptr(top))
	insert(leaf, ptr(mid))
	return top, mid, leaf
}

func TestIsAncestorOrSelf(t *testing.T) {
	s, pool := newStore(t)
	top, mid, leaf := treeFixture(t, s, pool)
	ctx := context.Background()

	for _, tc := range []struct {
		name         string
		ancestor, id string
		want         bool
	}{
		{"itself", mid, mid, true},
		{"parent", mid, leaf, true},
		{"grandparent", top, leaf, true},
		// Deny side: a child is not the ancestor of its parent, and siblings
		// further down do not count as ancestors.
		{"child is not an ancestor of its parent", leaf, mid, false},
		{"leaf is not an ancestor of the top", leaf, top, false},
	} {
		got, err := s.IsAncestorOrSelf(ctx, tc.ancestor, tc.id)
		require.NoError(t, err, tc.name)
		assert.Equal(t, tc.want, got, tc.name)
	}
}

// A parent_id cycle must end the walk, not run it until the statement times out.
func TestTreeWalks_TerminateOnParentCycle(t *testing.T) {
	s, pool := newStore(t)
	top, _, leaf := treeFixture(t, s, pool)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// top -> mid -> leaf -> top
	require.NoError(t, s.UpdateParent(ctx, top, ptr(leaf)))

	crumbs, err := s.GetBreadcrumb(ctx, leaf)
	require.NoError(t, err, "breadcrumb over a cycle must return, not time out")
	assert.LessOrEqual(t, len(crumbs), store.MaxTreeDepth+1)

	found, err := s.IsAncestorOrSelf(ctx, "no-such-folder", leaf)
	require.NoError(t, err)
	assert.False(t, found)
}
