//go:build realpolicy

package domain_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/services/auth/internal/domain"
	"ngac-platform/services/auth/internal/store"
	"ngac-platform/testutil"
)

// Against the real graph: a tenant-scoped token (sign-in, switching, the switcher's
// list) is for a tenant the graph holds the person in. The row alone is not enough.

type gateWorld struct {
	*joinWorld
	person  *store.User
	drifted testutil.WorkspaceGraph // owned on paper, but the graph does not hold the person
	healthy testutil.WorkspaceGraph // a row and the graph agree
}

func newGateWorld(t *testing.T) *gateWorld {
	t.Helper()
	j := newJoinWorld(t)
	ctx := context.Background()
	g := &gateWorld{joinWorld: j}
	g.drifted = j.workspace("Drifted", "", false)
	g.healthy = j.workspace("Healthy", "", false)
	g.person = j.user(g.healthy.Members)
	require.NoError(t, j.w.InsertTenantUser(ctx, g.drifted.WorkspaceID, g.person.ID, "owner", "active", g.person.NGACNodeID))
	require.NoError(t, j.w.InsertTenantUser(ctx, g.healthy.WorkspaceID, g.person.ID, "member", "active", g.person.NGACNodeID))
	require.NoError(t, j.w.LinkIdentity(ctx, domain.ProviderGoogle, "sub-"+g.person.ID, g.person.ID, g.person.Email))
	return g
}

func (g *gateWorld) signIn() *domain.SigninResult {
	g.t.Helper()
	res, err := g.svc.SignInWithGoogle(context.Background(), googleIdentity("sub-"+g.person.ID, g.person.Email, ""))
	require.NoError(g.t, err, "a sign-in does not fail on a tenant question")
	return res
}

func TestRealPolicy_SignInSkipsATenantTheGraphDoesNotHold(t *testing.T) {
	g := newGateWorld(t)

	res := g.signIn()
	assert.Equal(t, g.healthy.WorkspaceID, res.DefaultTenantID, "the owned tenant is not the default: the graph does not hold the person there")
	assert.Equal(t, []string{g.healthy.WorkspaceID}, tenantIDs(res.Tenants))
	assert.Equal(t, g.healthy.WorkspaceID, tenantOf(t, res.Token))

	// Allow, same fixture: once the graph holds them in the owned tenant it is the default again.
	testutil.Assign(t, g.raw, g.person.NGACNodeID, g.drifted.Members)
	res = g.signIn()
	assert.Equal(t, g.drifted.WorkspaceID, res.DefaultTenantID)
	assert.Len(t, res.Tenants, 2)
}

func TestRealPolicy_SignInWithThePolicyDownIsTenantlessNotAnError(t *testing.T) {
	g := newGateWorld(t)

	g.write.down.Store(true)
	res := g.signIn()
	assert.Empty(t, res.DefaultTenantID)
	assert.Empty(t, tenantOf(t, res.Token))
	assert.Empty(t, res.Tenants, "the rows alone do not stand in for the graph")
	assert.NotEmpty(t, res.Token, "the session itself is issued")

	g.write.down.Store(false)
	assert.Equal(t, g.healthy.WorkspaceID, g.signIn().DefaultTenantID, "with the policy back the tenant is found")
}

func TestRealPolicy_SwitchTenantFollowsTheGraph(t *testing.T) {
	g := newGateWorld(t)
	ctx := context.Background()
	p := g.person

	_, _, _, err := g.svc.SwitchTenant(ctx, p.ID, p.NGACNodeID, p.Username, g.drifted.WorkspaceID)
	assert.ErrorIs(t, err, domain.ErrAccessDenied, "a workspace the person is not a graph member of")

	token, _, info, err := g.svc.SwitchTenant(ctx, p.ID, p.NGACNodeID, p.Username, g.healthy.WorkspaceID)
	require.NoError(t, err, "a workspace they are")
	assert.Equal(t, g.healthy.WorkspaceID, tenantOf(t, token))
	assert.Equal(t, g.healthy.WorkspaceID, info.ID)

	g.write.down.Store(true)
	token, _, _, err = g.svc.SwitchTenant(ctx, p.ID, p.NGACNodeID, p.Username, g.healthy.WorkspaceID)
	assert.ErrorIs(t, err, domain.ErrUnavailable)
	assert.False(t, errors.Is(err, domain.ErrAccessDenied), "an outage is not a refusal")
	assert.Empty(t, token)
}

func TestRealPolicy_TheSwitcherListsOnlyWorkspacesTheGraphHolds(t *testing.T) {
	g := newGateWorld(t)
	ctx := context.Background()
	p := g.person

	list, err := g.svc.ListMyWorkspacesAt(ctx, p.ID, p.NGACNodeID, g.healthy.WorkspaceID)
	require.NoError(t, err)
	assert.Equal(t, []string{g.healthy.WorkspaceID}, workspaceIDs(list), "the drifted workspace is not offered")

	testutil.Assign(t, g.raw, p.NGACNodeID, g.drifted.Members)
	list, err = g.svc.ListMyWorkspacesAt(ctx, p.ID, p.NGACNodeID, g.healthy.WorkspaceID)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{g.healthy.WorkspaceID, g.drifted.WorkspaceID}, workspaceIDs(list))

	// An outage shows the workspace the caller is in, never the unfiltered rows.
	g.write.down.Store(true)
	list, err = g.svc.ListMyWorkspacesAt(ctx, p.ID, p.NGACNodeID, g.healthy.WorkspaceID)
	require.NoError(t, err, "an outage does not fail the list")
	assert.Equal(t, []string{g.healthy.WorkspaceID}, workspaceIDs(list))
}

// A graph member with no row (the join's row write failed) is listed by the next
// sign-in, as owner under Owners and as member under Members; a removed row is not
// resurrected.
func TestRealPolicy_SignInListsAGraphMemberWhoHasNoRow(t *testing.T) {
	g := newGateWorld(t)
	ctx := context.Background()
	owned := g.workspace("Owned", "", false)
	joined := g.workspace("Joined", "", false)
	gone := g.workspace("Gone", "", false)
	testutil.Assign(t, g.raw, g.person.NGACNodeID, owned.Owners)
	testutil.Assign(t, g.raw, g.person.NGACNodeID, joined.Members)
	testutil.Assign(t, g.raw, g.person.NGACNodeID, gone.Members)
	require.NoError(t, g.w.InsertTenantUser(ctx, gone.WorkspaceID, g.person.ID, "member", "removed", g.person.NGACNodeID))

	res := g.signIn()

	assert.Equal(t, owned.WorkspaceID, res.DefaultTenantID, "the owned workspace is the default")
	assert.ElementsMatch(t, []string{g.healthy.WorkspaceID, owned.WorkspaceID, joined.WorkspaceID}, tenantIDs(res.Tenants))
	require.NotNil(t, g.w.membership(owned.WorkspaceID, g.person.ID))
	assert.Equal(t, "owner", g.w.membership(owned.WorkspaceID, g.person.ID).Role)
	assert.Equal(t, "member", g.w.membership(joined.WorkspaceID, g.person.ID).Role)
	assert.Equal(t, "removed", g.w.membership(gone.WorkspaceID, g.person.ID).Status, "a removal is not undone by signing in")
}
