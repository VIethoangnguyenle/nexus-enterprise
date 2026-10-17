//go:build realpolicy

package domain_test

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/ngac"
	policypb "ngac-platform/proto/policy"

	"ngac-platform/services/auth/internal/auth"
	"ngac-platform/services/auth/internal/domain"
	"ngac-platform/services/auth/internal/store"
	"ngac-platform/testutil"
)

func TestMain(m *testing.M) { os.Exit(testutil.RunMain(m)) }

// These tests run the auth service's refresh against the real policy service. A
// refresh re-checks the person's membership of the tenant named by their token:
// a person who was removed (or whose row says so, or whom the graph no longer
// holds) is moved to the tenant they still belong to, or to none, and the session
// goes on. Each denial is paired with a person of the same fixture who stays.

type refreshWorld struct {
	t   *testing.T
	svc *domain.Service
	w   *fakeWorld
	// two workspaces in the real graph, with their rows in the fake store
	W, V testutil.WorkspaceGraph
}

func newRefreshWorld(t *testing.T) (*refreshWorld, *domain.Service) {
	t.Helper()
	auth.SetJWTSecret("test-secret-key-for-testing-only")
	read, write := testutil.StartPolicy(t)
	w := newFakeWorld()
	rdb := testRedisClient(t)
	svc := domain.NewService(w, rdb, read, write, &fakeWorkspace{w: w}, &fakeMessaging{w: w})
	r := &refreshWorld{t: t, svc: svc, w: w}
	r.W = testutil.SeedWorkspaceGraph(t, write)
	r.V = testutil.SeedWorkspaceGraph(t, write)
	for _, g := range []testutil.WorkspaceGraph{r.W, r.V} {
		w.workspaces[g.WorkspaceID] = &store.Tenant{ID: g.WorkspaceID, Name: "ws " + g.WorkspaceID}
	}
	return r, svc
}

type person struct {
	id, node string
}

// person makes a user with a node under the given UAs of the real graph.
func (r *refreshWorld) person(parents ...string) person {
	r.t.Helper()
	_, write := testutil.StartPolicy(r.t)
	u := r.w.addUser(uniqueEmail())
	u.NGACNodeID = testutil.NewUser(r.t, write, parents...)
	return person{id: u.ID, node: u.NGACNodeID}
}

func (r *refreshWorld) row(p person, g testutil.WorkspaceGraph, role, status string) {
	r.t.Helper()
	r.rowIn(p, g.WorkspaceID, role, status)
}

func (r *refreshWorld) rowIn(p person, ws, role, status string) {
	r.t.Helper()
	require.NoError(r.t, r.w.InsertTenantUser(context.Background(), ws, p.id, role, status, p.node))
}

func (r *refreshWorld) tokenFor(p person, g testutil.WorkspaceGraph) string {
	r.t.Helper()
	return r.tokenIn(p, g.WorkspaceID)
}

func (r *refreshWorld) tokenIn(p person, ws string) string {
	r.t.Helper()
	tok, err := r.svc.AttachRefreshToken(context.Background(), domain.RefreshIdentity{
		UserID: p.id, Username: "u-" + p.id, NGACNodeID: p.node, TenantID: ws, SessionID: "s-" + uuid.NewString(),
	})
	require.NoError(r.t, err)
	return tok
}

func (r *refreshWorld) refresh(token string) (next, tenant string, err error) {
	_, next, tenant, err = r.svc.RefreshSession(context.Background(), token)
	return next, tenant, err
}

// The tenant a person is moved to when they were removed from the one in their token,
// and the one they keep when they were not — on the same two workspaces.
func TestRealPolicy_RefreshMovesARemovedMemberToTheirOtherTenant(t *testing.T) {
	r, _ := newRefreshWorld(t)
	_, write := testutil.StartPolicy(t)

	stays := r.person(r.W.Members, r.V.Members)
	r.row(stays, r.W, "member", "active")
	r.row(stays, r.V, "member", "active")
	removed := r.person(r.W.Members, r.V.Members)
	r.row(removed, r.W, "member", "active")
	r.row(removed, r.V, "member", "active")

	// Allow: a member of both keeps the tenant of their token.
	tok := r.tokenFor(stays, r.W)
	next, tenant, err := r.refresh(tok)
	require.NoError(t, err)
	assert.Equal(t, r.W.WorkspaceID, tenant)
	_, tenant, err = r.refresh(next)
	require.NoError(t, err, "the rotated token keeps working")
	assert.Equal(t, r.W.WorkspaceID, tenant)

	// Deny: detached from W in the graph and tombstoned in the table.
	tok = r.tokenFor(removed, r.W)
	testutil.Unassign(t, write, removed.node, r.W.Members)
	r.w.members[r.W.WorkspaceID+"|"+removed.id].Status = "removed"
	out, err := r.svc.RefreshSessionFull(context.Background(), tok)
	require.NoError(t, err, "a removal is not a failure of the session")
	assert.True(t, out.Moved, "the client is told it was a membership move")
	tok = out.Refresh
	_, _ = next, tenant
	next, tenant, err = r.refresh(tok)
	assert.Equal(t, r.V.WorkspaceID, tenant, "moved to the tenant they still belong to")
	next, tenant, err = r.refresh(next)
	require.NoError(t, err, "the session family goes on")
	assert.Equal(t, r.V.WorkspaceID, tenant, "and stays there: the refresh token now names V")
}

func TestRealPolicy_RefreshWithNoTenantLeftIsTenantless(t *testing.T) {
	r, _ := newRefreshWorld(t)
	_, write := testutil.StartPolicy(t)

	only := r.person(r.W.Members)
	r.row(only, r.W, "member", "active")
	// A candidate whose row says removed is not a tenant to land in, even if the graph
	// still holds the person there.
	both := r.person(r.W.Members, r.V.Members)
	r.row(both, r.W, "member", "active")
	r.row(both, r.V, "member", "removed")
	keeper := r.person(r.W.Members)
	r.row(keeper, r.W, "member", "active")

	_, tenant, err := r.refresh(r.tokenFor(keeper, r.W))
	require.NoError(t, err)
	assert.Equal(t, r.W.WorkspaceID, tenant, "allow: an active member of W")

	testutil.Unassign(t, write, only.node, r.W.Members)
	r.w.members[r.W.WorkspaceID+"|"+only.id].Status = "removed"
	next, tenant, err := r.refresh(r.tokenFor(only, r.W))
	require.NoError(t, err)
	assert.Empty(t, tenant, "a person with no other tenant gets a tenant-less session")
	_, tenant, err = r.refresh(next)
	require.NoError(t, err, "a tenant-less session still refreshes")
	assert.Empty(t, tenant)

	testutil.Unassign(t, write, both.node, r.W.Members)
	r.w.members[r.W.WorkspaceID+"|"+both.id].Status = "removed"
	_, tenant, err = r.refresh(r.tokenFor(both, r.W))
	require.NoError(t, err)
	assert.Empty(t, tenant, "a tenant whose row says removed is not a place to land")
}

// The graph and the table disagree. The graph decides what the person can do, so a row
// that still says active does not keep them in a tenant they were detached from, and
// a row that says removed or disabled does not keep a graph member in.
func TestRealPolicy_RefreshFollowsTheGraphAndTheRowTogether(t *testing.T) {
	r, _ := newRefreshWorld(t)

	cases := []struct {
		name       string
		inGraph    bool
		row        string // "" = no row
		wantTenant string
	}{
		{"graph member, active row", true, "active", r.W.WorkspaceID},
		{"graph member, no row yet", true, "", r.W.WorkspaceID},
		{"detached in the graph, row still active", false, "active", r.V.WorkspaceID},
		{"graph member, row removed", true, "removed", r.V.WorkspaceID},
		{"graph member, row disabled", true, "disabled", r.V.WorkspaceID},
		{"graph member, row invited", true, "invited", r.V.WorkspaceID},
		{"detached and no row", false, "", r.V.WorkspaceID},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			parents := []string{r.V.Members}
			if tc.inGraph {
				parents = append(parents, r.W.Members)
			}
			p := r.person(parents...)
			r.row(p, r.V, "member", "active")
			if tc.row != "" {
				r.row(p, r.W, "member", tc.row)
			}
			_, tenant, err := r.refresh(r.tokenFor(p, r.W))
			require.NoError(t, err)
			assert.Equal(t, tc.wantTenant, tenant)
		})
	}
}

// The default tenant a removed person lands in is the one they own, as at sign-in.
func TestRealPolicy_RefreshLandsInTheOwnedTenantFirst(t *testing.T) {
	r, _ := newRefreshWorld(t)
	_, write := testutil.StartPolicy(t)
	third := testutil.SeedWorkspaceGraph(t, write)
	r.w.workspaces[third.WorkspaceID] = &store.Tenant{ID: third.WorkspaceID, Name: "third"}

	p := r.person(r.V.Members, third.Members)
	r.row(p, r.V, "member", "active")
	r.row(p, third, "owner", "active")
	_, tenant, err := r.refresh(r.tokenFor(p, r.W)) // never a member of W
	require.NoError(t, err)
	assert.Equal(t, third.WorkspaceID, tenant)
}

// A node that is only named like a workspace's Members UA (it does not sit under the
// workspace's policy class) is not a Members UA: nobody is a member through it. That is a
// "no" about the workspace, not a failure to ask, so it neither answers 503 forever nor
// stops the refresh from finding the tenant the person does belong to.
func TestRealPolicy_RefreshTreatsALookalikeMembersUAAsNotAMember(t *testing.T) {
	r, _ := newRefreshWorld(t)
	_, write := testutil.StartPolicy(t)

	lookalike := "rp-look-" + uuid.NewString()[:8]
	_, err := write.CreateNode(context.Background(), &policypb.CreateNodeRequest{
		Name: ngac.PCName(ngac.WorkspaceID(lookalike)), NodeType: ngac.TypePC,
		Properties: map[string]string{"scope": "tenant", "tenant_id": lookalike, "workspace_id": lookalike, "workspace": lookalike},
	})
	require.NoError(t, err)
	fake := testutil.NewNode(t, write, ngac.MembersUAName(ngac.WorkspaceID(lookalike)), ngac.TypeUA) // under no policy class
	r.w.workspaces[lookalike] = &store.Tenant{ID: lookalike, Name: "look"}

	p := r.person(fake, r.V.Members)
	r.rowIn(p, lookalike, "owner", "active") // owned, so first in the default order
	r.row(p, r.V, "member", "active")

	// Deny: the token names the lookalike; the person is not a member there.
	out, err := r.svc.RefreshSessionFull(context.Background(), r.tokenIn(p, lookalike))
	require.NoError(t, err, "a lookalike is an answer, not an outage")
	assert.True(t, out.Moved)
	assert.Equal(t, r.V.WorkspaceID, out.TenantID, "and it is not a place to land either, though they own it on paper")

	// Allow: the same person refreshing in the real workspace stays.
	out, err = r.svc.RefreshSessionFull(context.Background(), r.tokenFor(p, r.V))
	require.NoError(t, err)
	assert.False(t, out.Moved)
	assert.Equal(t, r.V.WorkspaceID, out.TenantID)
}
