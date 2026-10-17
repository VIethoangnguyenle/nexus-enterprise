package testutil

import (
	"context"
	"testing"

	"ngac-platform/ngac"
)

// A fixture workspace must have the topology CreateWorkspace builds (the policy
// class and the Members, Owners and Channels nodes keyed by its id, Members under the
// class and Owners under Members), or a migration's guard stops on a fixture leaked
// by a test, and a test of membership has no Members UA to be under.
func TestCreateWorkspaceBuildsTheIDKeyedTopology(t *testing.T) {
	pool := SetupTestDB(t)
	userID, _ := CreateUser(t, pool)
	wsID, pcID := CreateWorkspace(t, pool, userID)
	id := ngac.WorkspaceID(wsID)

	count := func(q string, args ...any) int {
		t.Helper()
		var n int
		if err := pool.QueryRow(context.Background(), q, args...).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	for _, c := range []struct{ name, typ string }{
		{ngac.PCName(id), "PC"}, {ngac.MembersUAName(id), "UA"}, {ngac.OwnersUAName(id), "UA"}, {ngac.ChannelsOAName(id), "OA"},
	} {
		if n := count(`SELECT count(*) FROM ngac_nodes WHERE name = $1 AND node_type = $2`, c.name, c.typ); n != 1 {
			t.Errorf("%s %s: %d nodes, want 1", c.typ, c.name, n)
		}
	}
	under := func(child, parent string) int {
		return count(`SELECT count(*) FROM ngac_assignments a JOIN ngac_nodes c ON c.id = a.child_id JOIN ngac_nodes p ON p.id = a.parent_id WHERE c.name = $1 AND p.name = $2`, child, parent)
	}
	if under(ngac.MembersUAName(id), ngac.PCName(id)) != 1 || under(ngac.OwnersUAName(id), ngac.MembersUAName(id)) != 1 {
		t.Error("Members must sit under the PC and Owners under Members")
	}
	if n := count(`SELECT count(*) FROM ngac_nodes WHERE id = $1 AND name = $2`, pcID, ngac.PCName(id)); n != 1 {
		t.Error("the PC node keeps its own id and carries the workspace's PC name")
	}
}
