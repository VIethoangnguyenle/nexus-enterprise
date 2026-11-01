//go:build realpolicy

package main

import (
	"context"
	"io"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"ngac-platform/ngac"
	"ngac-platform/pkg/policyclient"
	policypb "ngac-platform/proto/policy"
	"ngac-platform/testutil"
)

// The policy clients sign as workspace: the fixture undoes a grant (RemoveAssociation), a write messaging never makes; the backfill itself dials as backfill-managers.
func TestMain(m *testing.M) {
	testutil.PolicyService = "workspace"
	os.Exit(testutil.RunMain(m))
}

// The backfill against the real policy service, signed as the command signs
// (service identity backfill-managers, no user). Another service identity is
// refused and writes nothing. The backfill gives a legacy group whose adding is
// restricted its Managers UA: the owner may add people and a plain member may
// not. A rerun after its edges were lost puts them back.
func TestRealPolicy_BackfillGivesALegacyGroupItsManagers(t *testing.T) {
	ctx := context.Background()
	read, write := testutil.StartPolicy(t)
	pool := testutil.SetupTestDB(t)
	admin := testutil.PolicyAdmin(ctx)
	ws := testutil.SeedWorkspaceGraph(t, write)

	person := func() (id, node string) {
		node = testutil.NewUser(t, write, ws.Members)
		id = "rp-backfill-" + uuid.NewString()
		_, err := pool.Exec(ctx, `INSERT INTO users (id, username, password, ngac_node, display_name) VALUES ($1, $1, '', $2, $1)`, id, node)
		require.NoError(t, err)
		t.Cleanup(func() { pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, id) })
		return id, node
	}
	ownerID, ownerNode := person()
	_, plainNode := person()
	_, err := pool.Exec(ctx, `INSERT INTO workspaces (id, name, owner_id, ngac_pc_id, type) VALUES ($1, 'rp-backfill', $2, $3, 'organization')`,
		ws.WorkspaceID, ownerID, ws.PC)
	require.NoError(t, err)

	// A group from before roles existed: content and members, no Managers UA.
	chID := uuid.NewString()
	content := testutil.NewNode(t, write, ngac.ChannelContentOAName(ngac.ChannelID(chID)), ngac.TypeOA, ws.Channels)
	members := testutil.NewNode(t, write, ngac.ChannelMembersUAName(ngac.ChannelID(chID)), ngac.TypeUA, ws.PC)
	testutil.Associate(t, write, members, content, ngac.ChannelMemberOpsRestricted())
	testutil.Assign(t, write, ownerNode, members)
	testutil.Assign(t, write, plainNode, members)
	_, err = pool.Exec(ctx, `INSERT INTO channels (id, name, channel_type, workspace_id, ngac_oa_id, ngac_ua_id, created_by, created_at, invite_policy)
		VALUES ($1, 'rp-legacy', 'private', $2, $3, $4, $5, now(), $6)`, chID, ws.WorkspaceID, content, members, ownerID, ngac.GroupSettingManagers)
	require.NoError(t, err)
	managersName := ngac.ChannelManagersUAName(ngac.ChannelID(chID))
	managersUA := func() string {
		n, err := read.FindNodeByName(admin, &policypb.FindNodeByNameRequest{Name: managersName, NodeType: ngac.TypeUA})
		if policyclient.IsNotFound(err) {
			return ""
		}
		require.NoError(t, err)
		return n.GetId()
	}
	t.Cleanup(func() {
		bg := context.Background()
		pool.Exec(bg, `DELETE FROM channels WHERE id = $1`, chID)
		pool.Exec(bg, `DELETE FROM workspaces WHERE id = $1`, ws.WorkspaceID)
		if id := managersUA(); id != "" {
			write.DeleteNode(admin, &policypb.DeleteNodeRequest{NodeId: id})
		}
	})
	g := group{ID: chID, CreatedBy: ownerID, WorkspaceID: ws.WorkspaceID, ContentOA: content, MembersUA: members,
		InvitePolicy: ngac.GroupSettingManagers}
	require.False(t, testutil.Allowed(t, read, ownerNode, content, ngac.OpInvite), "the fixture is meaningful: nobody adds people yet")

	// Deny: another service identity reaches none of the methods; nothing is written.
	other := &backfiller{db: pool, write: testutil.DialPolicyAs(t, "backfill-other"), out: io.Discard}
	err = other.backfill(ctx, g, &summary{})
	require.Error(t, err)
	assert.Equal(t, codes.Unauthenticated, status.Code(err), "refused, not failed: %v", err)
	assert.Empty(t, managersUA(), "a refused identity creates nothing")

	// Allow: the backfill's own identity, with no user.
	b := &backfiller{db: pool, write: testutil.DialPolicyAs(t, "backfill-managers"), out: io.Discard}
	var s summary
	require.NoError(t, b.backfill(ctx, g, &s))
	assert.Equal(t, 1, s.ManagersUAsMade)
	ua := managersUA()
	require.NotEmpty(t, ua)
	assert.True(t, testutil.Allowed(t, read, ownerNode, content, ngac.OpInvite), "the owner adds people through Managers")
	assert.False(t, testutil.Allowed(t, read, plainNode, content, ngac.OpInvite), "a plain member still adds nobody")
	assert.True(t, testutil.Allowed(t, read, plainNode, content, ngac.OpRead), "and still reads the group")

	// A run that failed part-way: the UA exists without its PC edge or its grant.
	_, err = write.RemoveAssociation(admin, &policypb.RemoveAssociationRequest{UaId: ua, OaId: content})
	require.NoError(t, err)
	testutil.Unassign(t, write, ua, ws.PC)
	require.False(t, testutil.Allowed(t, read, ownerNode, content, ngac.OpInvite), "the edges are really gone")

	var again summary
	require.NoError(t, b.backfill(ctx, g, &again))
	assert.Equal(t, 0, again.ManagersUAsMade, "the UA is found, not made twice")
	assert.Equal(t, ua, managersUA())
	assert.True(t, testutil.Allowed(t, read, ownerNode, content, ngac.OpInvite), "the rerun puts the edges back")
	assert.False(t, testutil.Allowed(t, read, plainNode, content, ngac.OpInvite))
}
