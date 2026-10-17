//go:build realpolicy

package domain_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/services/auth/internal/auth"
	"ngac-platform/services/auth/internal/domain"
	"ngac-platform/services/auth/internal/store"
	"ngac-platform/testutil"
)

// The name resolver lets in whoever the real graph holds under the workspace's Members UA
// and whose row is active, and nobody else. Each denial is asked on the same two
// workspaces as the allowed member.

type resolverWorld struct {
	t    *testing.T
	svc  *domain.Service
	w    *fakeWorld
	st   *contactsStore
	W, V testutil.WorkspaceGraph
}

func newResolverWorld(t *testing.T) *resolverWorld {
	t.Helper()
	auth.SetJWTSecret("test-secret-key-for-testing-only")
	read, write := testutil.StartPolicy(t)
	w := newFakeWorld()
	st := &contactsStore{fakeWorld: w, resolved: []store.ResolvedPerson{{UserID: "named", DisplayName: "Có Tên"}}}
	r := &resolverWorld{t: t, w: w, st: st,
		svc: domain.NewService(st, nil, read, write, &scopeWorkspace{}, &fakeMessaging{w: w})}
	r.W = testutil.SeedWorkspaceGraph(t, write)
	r.V = testutil.SeedWorkspaceGraph(t, write)
	return r
}

func (r *resolverWorld) person(row string, g testutil.WorkspaceGraph, parents ...string) person {
	r.t.Helper()
	_, write := testutil.StartPolicy(r.t)
	u := r.w.addUser(uniqueEmail())
	u.NGACNodeID = testutil.NewUser(r.t, write, parents...)
	if row != "" {
		require.NoError(r.t, r.w.InsertTenantUser(context.Background(), g.WorkspaceID, u.ID, "member", row, u.NGACNodeID))
	}
	return person{id: u.ID, node: u.NGACNodeID}
}

func (r *resolverWorld) resolve(p person, g testutil.WorkspaceGraph) ([]domain.ResolvedPerson, error) {
	return r.svc.ResolvePeople(context.Background(), p.id, p.node, g.WorkspaceID, []string{"named"}, nil)
}

func TestRealPolicy_ResolveNeedsMembershipInTheGraphAndAnActiveRow(t *testing.T) {
	r := newResolverWorld(t)
	member := r.person("active", r.W, r.W.Members)
	owner := r.person("active", r.W, r.W.Owners)
	rowOnly := r.person("active", r.W) // an active row, but under nothing
	otherWorkspace := r.person("active", r.W, r.V.Members)
	tombstoned := r.person("removed", r.W, r.W.Members) // still in the graph, row says removed
	noRow := r.person("", r.W, r.W.Members)             // graph member whose row is missing

	// allow: members of W, an owner (Owners sit under Members), and a graph member with no row yet
	for name, p := range map[string]person{"member": member, "owner": owner, "graph member without a row": noRow} {
		got, err := r.resolve(p, r.W)
		require.NoError(t, err, name)
		require.Len(t, got, 1, name)
	}

	// deny: the same call on the same workspace
	for name, p := range map[string]person{"row without graph membership": rowOnly, "member of another workspace only": otherWorkspace} {
		got, err := r.resolve(p, r.W)
		assert.ErrorIs(t, err, domain.ErrAccessDenied, name)
		assert.Nil(t, got, name)
	}
	_, err := r.resolve(tombstoned, r.W)
	assert.ErrorIs(t, err, domain.ErrAccessDenied, "removed row, even with the edge still there")

	// a member of W has no say over V
	_, err = r.resolve(member, r.V)
	assert.ErrorIs(t, err, domain.ErrAccessDenied, "deny: cross-workspace")
	assert.Equal(t, r.W.WorkspaceID, r.st.resolveWS, "the lookup only ever ran for the workspace the caller belongs to")
}

func TestRealPolicy_ResolveStopsTheMomentTheGraphEdgeGoes(t *testing.T) {
	r := newResolverWorld(t)
	_, write := testutil.StartPolicy(t)
	p := r.person("active", r.W, r.W.Members)

	_, err := r.resolve(p, r.W)
	require.NoError(t, err, "allow: before the removal")

	testutil.Unassign(t, write, p.node, r.W.Members)
	_, err = r.resolve(p, r.W)
	assert.ErrorIs(t, err, domain.ErrAccessDenied, "deny: the writer is asked, so a removal made a moment ago counts")
}
