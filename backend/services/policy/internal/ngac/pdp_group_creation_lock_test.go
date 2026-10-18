package ngac_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	vocab "ngac-platform/ngac"
	"ngac-platform/services/policy/internal/ngac"
)

// Test vectors for the "Ai được tạo nhóm = chỉ quản trị viên" lock
// (docs/specs/user-settings, docs/specs/tenant-ngac-init).
//
// The lock removes the right instead of prohibiting it: create_channel leaves the
// Members → Channels association and every custom role's Channels grant, while
// Owners keep their own association. Unlocking restores the Members association
// only. Each vector applies the same graph change the workspace service writes
// (vocab.LockedChannelOps / UnlockedMemberChannelOps) to a graph built as
// CreateWorkspace builds it, plus a custom role granted the whole Channels area.
//
//	"invitee"  plain member            "roleUser" member who also holds role "role"
//	                                   "roleOnly" holds role "role" and nothing else
//	"creator"  owner                   "stranger" a member of another workspace

const (
	assocMembersChannels = "assoce" // buildWorkspaceBootstrapGraph: {"members", "channels"}
	assocRoleChannels    = "role-channels"
)

func groupCreationGraph(t *testing.T) *ngac.Graph {
	t.Helper()
	g := buildWorkspaceBootstrapGraph(t)
	g.AddNode(&ngac.NGACNode{ID: "role", Name: "Moderator", NodeType: "UA"})
	g.AddNode(&ngac.NGACNode{ID: "roleUser", Name: "RoleUser", NodeType: "U"})
	require.NoError(t, g.AddAssignment(&ngac.Assignment{ID: "gr1", ChildID: "role", ParentID: "pc"}))
	require.NoError(t, g.AddAssignment(&ngac.Assignment{ID: "gr2", ChildID: "roleUser", ParentID: "role"}))
	g.AddNode(&ngac.NGACNode{ID: "roleOnly", Name: "RoleOnly", NodeType: "U"})
	require.NoError(t, g.AddAssignment(&ngac.Assignment{ID: "gr4", ChildID: "roleOnly", ParentID: "role"}))
	// roleUser is also a plain member of the workspace.
	require.NoError(t, g.AddAssignment(&ngac.Assignment{ID: "gr3", ChildID: "roleUser", ParentID: "members"}))
	require.NoError(t, g.AddAssociation(&ngac.Association{
		ID: assocRoleChannels, UAID: "role", OAID: "channels", Operations: vocab.AreaOps(vocab.AreaChannels),
	}))
	return g
}

// lockGroupCreation applies what the workspace service writes when it locks.
func lockGroupCreation(t *testing.T, g *ngac.Graph) {
	t.Helper()
	// Members → Channels holds create_channel only, so the grant goes away.
	require.Empty(t, vocab.LockedChannelOps(vocab.MemberChannelOps()))
	g.RemoveAssociationByID(assocMembersChannels)
	// The custom role keeps whatever else the area offers.
	left := vocab.LockedChannelOps(vocab.AreaOps(vocab.AreaChannels))
	if len(left) == 0 {
		g.RemoveAssociationByID(assocRoleChannels)
		return
	}
	require.NoError(t, g.AddAssociation(&ngac.Association{ID: assocRoleChannels, UAID: "role", OAID: "channels", Operations: left}))
}

func TestGroupCreationLock_BeforeLockMemberAndRoleHolderCanCreate(t *testing.T) {
	g := groupCreationGraph(t)
	for _, u := range []string{"invitee", "roleUser", "roleOnly", "creator"} {
		assert.Equal(t, "ALLOW", decideOn(g, u, "channels", vocab.OpCreateChannel), u)
	}
}

func TestGroupCreationLock_LockedDeniesPlainMember(t *testing.T) {
	g := groupCreationGraph(t)
	lockGroupCreation(t, g)
	assert.Equal(t, "DENY", decideOn(g, "invitee", "channels", vocab.OpCreateChannel))
}

func TestGroupCreationLock_LockedDeniesMemberHoldingACustomChannelsRole(t *testing.T) {
	g := groupCreationGraph(t)
	assert.Equal(t, "ALLOW", decideOn(g, "roleOnly", "channels", vocab.OpCreateChannel), "the role grant works before the lock")
	lockGroupCreation(t, g)
	for _, u := range []string{"roleUser", "roleOnly"} {
		assert.Equal(t, "DENY", decideOn(g, u, "channels", vocab.OpCreateChannel), u+": neither the role nor Members confers it any more")
	}
}

func TestGroupCreationLock_LockedStillAllowsOwner(t *testing.T) {
	g := groupCreationGraph(t)
	lockGroupCreation(t, g)
	assert.Equal(t, "ALLOW", decideOn(g, "creator", "channels", vocab.OpCreateChannel), "Owners keep their own association")
	assert.Equal(t, "ALLOW", decideOn(g, "creator", "channels", vocab.OpManage))
}

// The lock only takes create_channel away: nothing else a member or owner holds moves.
func TestGroupCreationLock_LockedLeavesEverythingElseAlone(t *testing.T) {
	g := groupCreationGraph(t)
	lockGroupCreation(t, g)
	assert.Equal(t, "ALLOW", decideOn(g, "invitee", "docs", vocab.OpRead))
	assert.Equal(t, "ALLOW", decideOn(g, "invitee", "docs", vocab.OpWrite))
	assert.Equal(t, "DENY", decideOn(g, "invitee", "channels", vocab.OpManage))
	assert.Equal(t, "ALLOW", decideOn(g, "creator", "mgmt", vocab.OpManage))
}

// A user of another workspace never gains the right from this workspace's lock or unlock.
func TestGroupCreationLock_OtherWorkspaceUserIsDeniedLockedOrNot(t *testing.T) {
	g := groupCreationGraph(t)
	g.AddNode(&ngac.NGACNode{ID: "pc2", Name: "PC_ws2", NodeType: "PC"})
	g.AddNode(&ngac.NGACNode{ID: "members2", Name: "ws2_Members", NodeType: "UA"})
	g.AddNode(&ngac.NGACNode{ID: "stranger", Name: "Stranger", NodeType: "U"})
	require.NoError(t, g.AddAssignment(&ngac.Assignment{ID: "gx1", ChildID: "members2", ParentID: "pc2"}))
	require.NoError(t, g.AddAssignment(&ngac.Assignment{ID: "gx2", ChildID: "stranger", ParentID: "members2"}))

	assert.Equal(t, "DENY", decideOn(g, "stranger", "channels", vocab.OpCreateChannel))
	lockGroupCreation(t, g)
	assert.Equal(t, "DENY", decideOn(g, "stranger", "channels", vocab.OpCreateChannel))
}

// Unlocking restores the Members association only; the custom role stays without the right.
func TestGroupCreationLock_UnlockRestoresMembersButNotCustomRoles(t *testing.T) {
	g := groupCreationGraph(t)
	lockGroupCreation(t, g)
	require.Equal(t, "DENY", decideOn(g, "invitee", "channels", vocab.OpCreateChannel))

	require.NoError(t, g.AddAssociation(&ngac.Association{
		ID: "assoc-unlocked", UAID: "members", OAID: "channels", Operations: vocab.UnlockedMemberChannelOps(nil),
	}))

	assert.Equal(t, "ALLOW", decideOn(g, "invitee", "channels", vocab.OpCreateChannel), "an ordinary member can create groups again")
	assert.Equal(t, "ALLOW", decideOn(g, "creator", "channels", vocab.OpCreateChannel))
	assert.Equal(t, "DENY", decideOn(g, "roleOnly", "channels", vocab.OpCreateChannel), "the role grant is not restored")
	assert.Equal(t, "ALLOW", decideOn(g, "roleUser", "channels", vocab.OpCreateChannel), "a role holder who is also a member gets it through Members")
}
