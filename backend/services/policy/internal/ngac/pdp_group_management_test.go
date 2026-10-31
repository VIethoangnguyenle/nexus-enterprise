package ngac_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	vocab "ngac-platform/ngac"
	"ngac-platform/services/policy/internal/ngac"
)

// Test vectors for group management (docs/specs/chat-group-management, chat-mention-all).
//
// The Managers UA Ch_{id}_Managers sits beside Ch_{id}_Members under the PC.
// Managers hold invite on the group's Content OA (ChannelManagerOps). Members
// hold read, write, invite (ChannelMemberOps) or read, write only when invite
// is restricted (ChannelMemberOpsRestricted). A manager has both edges.
//
//	PC ← ws_Channels ← Ch_c1_Content
//	PC ← Ch_c1_Members  (alice: plain member; bob: also a manager)
//	PC ← Ch_c1_Managers (bob)
//	"invitee" is a plain workspace member, "creator" an owner; neither is in c1.
//	"stranger" is a member of another workspace.

const (
	assocManagersContent = "mgr-content-assoc" // Managers → Content: {invite}
)

// groupManagementGraph builds the workspace bootstrap + group c1 with both
// Members UA and Managers UA, wired as chat-group-management describes.
func groupManagementGraph(t *testing.T, memberOps []string) *ngac.Graph {
	t.Helper()
	g := buildChannelGraph(t, memberOps) // adds chContent, chMembers, chUser

	// Rename "chUser" to serve as Alice (plain member).
	// Add Bob: both a member and a manager.
	g.AddNode(&ngac.NGACNode{ID: "bob", Name: "Bob", NodeType: "U"})
	require.NoError(t, g.AddAssignment(&ngac.Assignment{ID: "gm1", ChildID: "bob", ParentID: "chMembers"}))

	// Create the Managers UA.
	g.AddNode(&ngac.NGACNode{
		ID: "chManagers", Name: vocab.ChannelManagersUAName("c1"), NodeType: "UA",
	})
	require.NoError(t, g.AddAssignment(&ngac.Assignment{ID: "gm2", ChildID: "chManagers", ParentID: "pc"}))
	require.NoError(t, g.AddAssignment(&ngac.Assignment{ID: "gm3", ChildID: "bob", ParentID: "chManagers"}))

	// Managers → Content association: {invite}.
	require.NoError(t, g.AddAssociation(&ngac.Association{
		ID: assocManagersContent, UAID: "chManagers", OAID: "chContent",
		Operations: vocab.ChannelManagerOps(),
	}))

	return g
}

// --- Deny-first vectors ---

// Managers UA grants invite; a plain member's read and write are through Members.
func TestGroupManagement_ManagersUAGrantsInvite(t *testing.T) {
	g := groupManagementGraph(t, vocab.ChannelMemberOps())

	// Bob (manager) has invite through both Members and Managers.
	assert.Equal(t, "ALLOW", decideOn(g, "bob", "chContent", vocab.OpInvite), "manager has invite")
	assert.Equal(t, "ALLOW", decideOn(g, "bob", "chContent", vocab.OpRead), "manager reads through Members")
	assert.Equal(t, "ALLOW", decideOn(g, "bob", "chContent", vocab.OpWrite), "manager writes through Members")

	// Alice (plain member) has invite through Members (unrestricted).
	assert.Equal(t, "ALLOW", decideOn(g, "chUser", "chContent", vocab.OpInvite), "plain member invites when unrestricted")
}

// Restricted Members lacks invite → DENY for a plain member.
func TestGroupManagement_RestrictedMembersLacksInviteDENY(t *testing.T) {
	g := groupManagementGraph(t, vocab.ChannelMemberOpsRestricted())

	assert.Equal(t, "DENY", decideOn(g, "chUser", "chContent", vocab.OpInvite),
		"a plain member must be denied invite when restricted")

	// But the manager still has invite through the Managers UA.
	assert.Equal(t, "ALLOW", decideOn(g, "bob", "chContent", vocab.OpInvite),
		"a manager keeps invite through the Managers UA")
}

// Demoted manager invite → DENY: removing from Managers while invite is restricted.
func TestGroupManagement_DemotedManagerInviteDENY(t *testing.T) {
	g := groupManagementGraph(t, vocab.ChannelMemberOpsRestricted())

	require.Equal(t, "ALLOW", decideOn(g, "bob", "chContent", vocab.OpInvite), "manager before demotion")

	// Demote: remove bob from Managers.
	g.RemoveAssignment("bob", "chManagers")

	assert.Equal(t, "DENY", decideOn(g, "bob", "chContent", vocab.OpInvite),
		"demoted manager must lose invite when restricted")
	// But bob still has read and write through Members.
	assert.Equal(t, "ALLOW", decideOn(g, "bob", "chContent", vocab.OpRead), "demoted manager still reads")
	assert.Equal(t, "ALLOW", decideOn(g, "bob", "chContent", vocab.OpWrite), "demoted manager still writes")
}

// Workspace owner not in group read → DENY.
func TestGroupManagement_WorkspaceOwnerNotInGroupReadDENY(t *testing.T) {
	g := groupManagementGraph(t, vocab.ChannelMemberOps())

	assert.Equal(t, "DENY", decideOn(g, "creator", "chContent", vocab.OpRead),
		"workspace owner outside the group must not read it")
	assert.Equal(t, "DENY", decideOn(g, "creator", "chContent", vocab.OpWrite),
		"workspace owner outside the group must not write to it")
	assert.Equal(t, "DENY", decideOn(g, "creator", "chContent", vocab.OpInvite),
		"workspace owner outside the group must not invite into it")

	// But manage stays (workspace moderation).
	assert.Equal(t, "ALLOW", decideOn(g, "creator", "chContent", vocab.OpManage),
		"workspace owner keeps manage for moderation (rename, remove)")
}

// Other workspace user → DENY (cross-PC intersection principle).
func TestGroupManagement_OtherWorkspaceUserDENY(t *testing.T) {
	g := groupManagementGraph(t, vocab.ChannelMemberOps())

	g.AddNode(&ngac.NGACNode{ID: "pc2", Name: "PC_ws2", NodeType: "PC"})
	g.AddNode(&ngac.NGACNode{ID: "members2", Name: "ws2_Members", NodeType: "UA"})
	g.AddNode(&ngac.NGACNode{ID: "stranger", Name: "Stranger", NodeType: "U"})
	require.NoError(t, g.AddAssignment(&ngac.Assignment{ID: "gm5", ChildID: "members2", ParentID: "pc2"}))
	require.NoError(t, g.AddAssignment(&ngac.Assignment{ID: "gm6", ChildID: "stranger", ParentID: "members2"}))

	for _, op := range []string{vocab.OpRead, vocab.OpWrite, vocab.OpInvite, vocab.OpManage} {
		assert.Equalf(t, "DENY", decideOn(g, "stranger", "chContent", op),
			"another workspace's user must be denied %s on the group", op)
	}
}

// Removed manager: both Members and Managers edges gone → read/invite DENY.
func TestGroupManagement_RemovedManagerReadInviteDENYAfterInvalidation(t *testing.T) {
	g := groupManagementGraph(t, vocab.ChannelMemberOps())

	require.Equal(t, "ALLOW", decideOn(g, "bob", "chContent", vocab.OpRead))
	require.Equal(t, "ALLOW", decideOn(g, "bob", "chContent", vocab.OpInvite))

	// Remove bob entirely: both edges.
	g.RemoveAssignment("bob", "chManagers")
	g.RemoveAssignment("bob", "chMembers")

	assert.Equal(t, "DENY", decideOn(g, "bob", "chContent", vocab.OpRead),
		"removed manager must be denied read after all edges removed")
	assert.Equal(t, "DENY", decideOn(g, "bob", "chContent", vocab.OpInvite),
		"removed manager must be denied invite after all edges removed")
}

// Workspace member outside the group is still denied even when Managers UA exists.
func TestGroupManagement_WorkspaceMemberOutsideGroupStillDenied(t *testing.T) {
	g := groupManagementGraph(t, vocab.ChannelMemberOps())

	// invitee is a workspace member but not in the group.
	assert.Equal(t, "DENY", decideOn(g, "invitee", "chContent", vocab.OpRead))
	assert.Equal(t, "DENY", decideOn(g, "invitee", "chContent", vocab.OpWrite))
	assert.Equal(t, "DENY", decideOn(g, "invitee", "chContent", vocab.OpInvite))
}

// After a transfer: the new owner is in Managers, the former owner stays a manager.
func TestGroupManagement_OwnerTransferManagersStay(t *testing.T) {
	g := groupManagementGraph(t, vocab.ChannelMemberOpsRestricted())

	// Add Alice as a new manager (the transfer target).
	require.NoError(t, g.AddAssignment(&ngac.Assignment{ID: "gm-alice-mgr", ChildID: "chUser", ParentID: "chManagers"}))

	// Both the former owner (now just a manager) and the new owner keep invite.
	assert.Equal(t, "ALLOW", decideOn(g, "bob", "chContent", vocab.OpInvite), "former owner stays manager, keeps invite")
	assert.Equal(t, "ALLOW", decideOn(g, "chUser", "chContent", vocab.OpInvite), "new owner (also manager) gets invite")

	// Demote the former owner.
	g.RemoveAssignment("bob", "chManagers")
	assert.Equal(t, "DENY", decideOn(g, "bob", "chContent", vocab.OpInvite),
		"demoted former owner loses invite under restricted invite_policy")
}

// Plain member who holds no special role: all content operations but not manage.
func TestGroupManagement_PlainMemberRightsPreserved(t *testing.T) {
	g := groupManagementGraph(t, vocab.ChannelMemberOps())

	// chUser (Alice) is a plain member with full ChannelMemberOps.
	assert.Equal(t, "ALLOW", decideOn(g, "chUser", "chContent", vocab.OpRead))
	assert.Equal(t, "ALLOW", decideOn(g, "chUser", "chContent", vocab.OpWrite))
	assert.Equal(t, "ALLOW", decideOn(g, "chUser", "chContent", vocab.OpInvite))
	// But not manage — that's workspace owners only.
	assert.Equal(t, "DENY", decideOn(g, "chUser", "chContent", vocab.OpManage))
}

// A group whose creator left before roles arrived: the backfill makes the
// earliest current member owner (put in Managers) and writes nothing for the
// creator, who is still a member of the workspace. The creator stays outside the
// group: read, write and invite DENY; the new owner holds all three.
func TestGroupManagement_CreatorWhoLeftBeforeBackfillStaysDenied(t *testing.T) {
	g := groupManagementGraph(t, vocab.ChannelMemberOps())
	g.AddNode(&ngac.NGACNode{ID: "leaver", Name: "Leaver", NodeType: "U"})
	require.NoError(t, g.AddAssignment(&ngac.Assignment{ID: "gm-leaver", ChildID: "leaver", ParentID: "members"}))
	require.NoError(t, g.AddAssignment(&ngac.Assignment{ID: "gm-new-owner", ChildID: "chUser", ParentID: "chManagers"}))

	for _, op := range []string{vocab.OpRead, vocab.OpWrite, vocab.OpInvite} {
		assert.Equal(t, "DENY", decideOn(g, "leaver", "chContent", op), "creator who left: %s", op)
		assert.Equal(t, "ALLOW", decideOn(g, "chUser", "chContent", op), "new owner: %s", op)
	}
}
