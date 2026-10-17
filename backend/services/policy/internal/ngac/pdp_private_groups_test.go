package ngac_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	vocab "ngac-platform/ngac"
	"ngac-platform/services/policy/internal/ngac"
)

// Test vectors for private groups (docs/specs/tenant-ngac-init, chat-calls).
//
// A group is readable only by the people in its own Members UA. The workspace
// Members and Owners UAs are associated with the Channels OA, whose grants every
// group's content, drive and calendar inherit, so what they hold there is what
// decides whether a group is private. Each deny is paired with an allow on the
// same graph, so an unknown node cannot fake the deny.
//
//	PC ← ws_Channels ← Ch_c1_Content ← Ch_c1_Drive
//	                                 ← Cal_c1 (channel calendar)
//	PC ← Ch_c1_Members  (chUser)
//	"invitee" is a plain workspace member, "creator" an owner; neither is in c1.

// privateGroupGraph is the workspace bootstrap plus group c1 with its drive and
// calendar, wired as messaging and drive wire them.
func privateGroupGraph(t *testing.T) *ngac.Graph {
	t.Helper()
	g := buildChannelGraph(t, vocab.ChannelMemberOps())

	g.AddNode(&ngac.NGACNode{ID: "chDrive", Name: vocab.ChannelDriveName("c1"), NodeType: "OA"})
	g.AddNode(&ngac.NGACNode{ID: "chCal", Name: vocab.CalendarOAName("c1cal"), NodeType: "OA"})
	require.NoError(t, g.AddAssignment(&ngac.Assignment{ID: "pg1", ChildID: "chDrive", ParentID: "chContent"}))
	require.NoError(t, g.AddAssignment(&ngac.Assignment{ID: "pg2", ChildID: "chCal", ParentID: "chContent"}))
	require.NoError(t, g.AddAssociation(&ngac.Association{
		ID: "pgdrive", UAID: "chMembers", OAID: "chDrive", Operations: vocab.ChannelDriveOps(),
	}))
	return g
}

func decideOn(g *ngac.Graph, user, oa, op string) string { return g.CheckAccess(user, oa, op).Decision }

// A workspace member who is not in the group is denied everything about it; the
// group's own member keeps what they had.
func TestPrivateGroup_WorkspaceMemberNotInGroupIsDenied(t *testing.T) {
	g := privateGroupGraph(t)

	for _, tc := range []struct{ name, oa, op string }{
		{"read messages", "chContent", vocab.OpRead},
		{"post messages", "chContent", vocab.OpWrite},
		{"invite someone", "chContent", vocab.OpInvite},
		{"read the group's files", "chDrive", vocab.OpRead},
		{"write the group's files", "chDrive", vocab.OpWrite},
		{"upload to the group's drive", "chDrive", vocab.OpUpload},
		{"read the group's calendar", "chCal", vocab.OpRead},
		{"write the group's calendar", "chCal", vocab.OpWrite},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, "ALLOW", decideOn(g, "chUser", tc.oa, tc.op), "the group's own member")
			assert.Equal(t, "DENY", decideOn(g, "invitee", tc.oa, tc.op), "a workspace member outside the group")
		})
	}
}

// Owners moderate groups they are not in but do not read them. manage is theirs
// (and no plain member's); read, write and invite come only from being in the group.
func TestPrivateGroup_OwnerNotInGroupModeratesButCannotRead(t *testing.T) {
	g := privateGroupGraph(t)

	assert.Equal(t, "ALLOW", decideOn(g, "creator", "chContent", vocab.OpManage), "owners hold manage on every group")
	assert.Equal(t, "DENY", decideOn(g, "invitee", "chContent", vocab.OpManage), "a plain member does not")
	assert.Equal(t, "DENY", decideOn(g, "chUser", "chContent", vocab.OpManage), "being in the group does not either")

	for _, tc := range []struct{ oa, op string }{
		{"chContent", vocab.OpRead}, {"chContent", vocab.OpWrite}, {"chContent", vocab.OpInvite},
		{"chDrive", vocab.OpRead}, {"chDrive", vocab.OpWrite}, {"chDrive", vocab.OpUpload},
		{"chCal", vocab.OpRead}, {"chCal", vocab.OpWrite},
	} {
		assert.Equalf(t, "DENY", decideOn(g, "creator", tc.oa, tc.op), "an owner outside the group must not %s %s", tc.op, tc.oa)
	}

	// The same owner, once in the group, reads it like any member.
	require.NoError(t, g.AddAssignment(&ngac.Assignment{ID: "own-in", ChildID: "creator", ParentID: "chMembers"}))
	assert.Equal(t, "ALLOW", decideOn(g, "creator", "chContent", vocab.OpRead))
	assert.Equal(t, "ALLOW", decideOn(g, "creator", "chDrive", vocab.OpRead))
}

// A custom role may be given every operation the channels area offers and still
// reaches no group; create_channel is the one thing it does confer.
func TestPrivateGroup_CustomRoleWithEveryChannelsOpCannotReadGroups(t *testing.T) {
	g := privateGroupGraph(t)
	g.AddNode(&ngac.NGACNode{ID: "role", Name: "Moderator", NodeType: "UA"})
	g.AddNode(&ngac.NGACNode{ID: "roleUser", Name: "RoleUser", NodeType: "U"})
	require.NoError(t, g.AddAssignment(&ngac.Assignment{ID: "r1", ChildID: "role", ParentID: "pc"}))
	require.NoError(t, g.AddAssignment(&ngac.Assignment{ID: "r2", ChildID: "roleUser", ParentID: "role"}))
	require.NoError(t, g.AddAssociation(&ngac.Association{
		ID: "rassoc", UAID: "role", OAID: "channels", Operations: vocab.AreaOps(vocab.AreaChannels),
	}))

	assert.Equal(t, "ALLOW", decideOn(g, "roleUser", "channels", vocab.OpCreateChannel), "the grant itself works")
	for _, oa := range []string{"chContent", "chDrive", "chCal"} {
		for _, op := range []string{vocab.OpRead, vocab.OpWrite, vocab.OpInvite, vocab.OpManage} {
			assert.Equalf(t, "DENY", decideOn(g, "roleUser", oa, op), "a role on Channels must not %s %s", op, oa)
		}
	}
}

// Intersection principle: a person in the group's UA who reaches the content
// through a different policy class is denied, even with an association on their side.
func TestPrivateGroup_GroupMemberThroughAnotherPolicyClassIsDenied(t *testing.T) {
	g := privateGroupGraph(t)

	g.AddNode(&ngac.NGACNode{ID: "pc2", Name: "PC_ws2", NodeType: "PC"})
	g.AddNode(&ngac.NGACNode{ID: "channels2", Name: "ws2_Channels", NodeType: "OA"})
	g.AddNode(&ngac.NGACNode{ID: "ch2x", Name: "Ch_x_Content", NodeType: "OA"})
	require.NoError(t, g.AddAssignment(&ngac.Assignment{ID: "i1", ChildID: "channels2", ParentID: "pc2"}))
	require.NoError(t, g.AddAssignment(&ngac.Assignment{ID: "i2", ChildID: "ch2x", ParentID: "channels2"}))
	require.NoError(t, g.AddAssociation(&ngac.Association{ID: "iassoc", UAID: "chMembers", OAID: "ch2x", Operations: []string{"read"}}))

	assert.Equal(t, "ALLOW", decideOn(g, "chUser", "chContent", vocab.OpRead), "same policy class")
	assert.Equal(t, "DENY", decideOn(g, "chUser", "ch2x", vocab.OpRead),
		"the user side reaches PC_ws and the content reaches PC_ws2: no shared class")
}

// #general has no special graph shape: it is a group whose Members UA holds every
// workspace member as a direct U edge. Workspace membership alone grants nothing
// in it, which is why every admission must write the edge.
func TestPrivateGroup_WorkspaceWideChannelIsReadableOnlyThroughDirectEdges(t *testing.T) {
	g := privateGroupGraph(t)
	g.AddNode(&ngac.NGACNode{ID: "genContent", Name: vocab.ChannelContentOAName("gen"), NodeType: "OA"})
	g.AddNode(&ngac.NGACNode{ID: "genMembers", Name: vocab.ChannelMembersUAName("gen"), NodeType: "UA"})
	require.NoError(t, g.AddAssignment(&ngac.Assignment{ID: "g1", ChildID: "genContent", ParentID: "channels"}))
	require.NoError(t, g.AddAssignment(&ngac.Assignment{ID: "g2", ChildID: "genMembers", ParentID: "pc"}))
	require.NoError(t, g.AddAssociation(&ngac.Association{ID: "gassoc", UAID: "genMembers", OAID: "genContent", Operations: vocab.ChannelMemberOps()}))

	// Admission writes the direct edge for the invitee; the owner has one too.
	require.NoError(t, g.AddAssignment(&ngac.Assignment{ID: "g3", ChildID: "invitee", ParentID: "genMembers"}))
	require.NoError(t, g.AddAssignment(&ngac.Assignment{ID: "g4", ChildID: "creator", ParentID: "genMembers"}))

	// Another workspace's member, and a workspace member whose edge was never written.
	g.AddNode(&ngac.NGACNode{ID: "pc2", Name: "PC_ws2", NodeType: "PC"})
	g.AddNode(&ngac.NGACNode{ID: "members2", Name: "ws2_Members", NodeType: "UA"})
	g.AddNode(&ngac.NGACNode{ID: "outsider", Name: "Outsider", NodeType: "U"})
	g.AddNode(&ngac.NGACNode{ID: "latecomer", Name: "Latecomer", NodeType: "U"})
	require.NoError(t, g.AddAssignment(&ngac.Assignment{ID: "g5", ChildID: "members2", ParentID: "pc2"}))
	require.NoError(t, g.AddAssignment(&ngac.Assignment{ID: "g6", ChildID: "outsider", ParentID: "members2"}))
	require.NoError(t, g.AddAssignment(&ngac.Assignment{ID: "g7", ChildID: "latecomer", ParentID: "members"}))

	for _, op := range []string{vocab.OpRead, vocab.OpWrite} {
		assert.Equal(t, "ALLOW", decideOn(g, "invitee", "genContent", op), "member with a direct edge: "+op)
		assert.Equal(t, "ALLOW", decideOn(g, "creator", "genContent", op), "owner with a direct edge: "+op)
		assert.Equal(t, "DENY", decideOn(g, "outsider", "genContent", op), "another workspace's member: "+op)
		assert.Equal(t, "DENY", decideOn(g, "latecomer", "genContent", op), "a member whose edge is missing: "+op)
	}

	// Leaving the workspace strips the edge, and the access goes with it.
	require.Equal(t, "ALLOW", decideOn(g, "invitee", "genContent", vocab.OpRead))
	g.RemoveAssignment("invitee", "genMembers")
	assert.Equal(t, "DENY", decideOn(g, "invitee", "genContent", vocab.OpRead))
	assert.Equal(t, "ALLOW", decideOn(g, "creator", "genContent", vocab.OpRead), "the others keep theirs")
}

// A prohibition on a group member overrides the group's ALLOW.
func TestPrivateGroup_ProhibitionOverridesGroupMembership(t *testing.T) {
	g := privateGroupGraph(t)
	require.NoError(t, g.AddProhibition(&ngac.Prohibition{
		ID: "pp1", Name: "mute-chuser", SubjectID: "chUser",
		Operations: []string{vocab.OpWrite}, TargetOAIDs: []string{"chContent"},
	}))
	engine := ngac.NewDecisionEngine(g, nil)
	decide := func(user, op string) string {
		return engine.Decide(context.Background(), ngac.AccessRequest{UserNodeID: user, ObjectNodeID: "chContent", Operation: op}).Decision
	}

	require.Equal(t, "ALLOW", g.CheckAccess("chUser", "chContent", vocab.OpWrite).Decision, "the association alone allows it")
	assert.Equal(t, "DENY", decide("chUser", vocab.OpWrite), "the prohibition overrides")
	assert.Equal(t, "ALLOW", decide("chUser", vocab.OpRead), "only the prohibited operation")
	assert.Equal(t, "DENY", decide("invitee", vocab.OpRead), "and a non-member stays denied")
}
