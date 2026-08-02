package ngac_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	vocab "ngac-platform/ngac"
	"ngac-platform/services/policy/internal/ngac"
)

// buildWorkspaceBootstrapGraph replicates the exact node/assignment/association topology
// created by workspace.Service.CreateWorkspace
// (backend/services/workspace/internal/domain/service.go L74-L148), then adds a
// user invited via InviteMember (assigned to the Members UA).
func buildWorkspaceBootstrapGraph(t *testing.T) *ngac.Graph {
	t.Helper()
	g := ngac.NewGraph()

	for _, n := range []*ngac.NGACNode{
		{ID: "pc", Name: "PC_ws", NodeType: "PC"},
		{ID: "owners", Name: "ws_Owners", NodeType: "UA"},
		{ID: "members", Name: "ws_Members", NodeType: "UA"},
		{ID: "mgmt", Name: "ws_Mgmt", NodeType: "OA"},
		{ID: "docs", Name: "ws_Documents", NodeType: "OA"},
		{ID: "channels", Name: "ws_Channels", NodeType: "OA"},
		{ID: "creator", Name: "Creator", NodeType: "U"},
		{ID: "invitee", Name: "Invitee", NodeType: "U"},
	} {
		g.AddNode(n)
	}

	// Assignments, in the same order as CreateWorkspace.
	for i, a := range []struct{ child, parent string }{
		{"owners", "pc"},
		{"members", "pc"},
		{"mgmt", "pc"},
		{"docs", "pc"},
		{"channels", "pc"},
		{"owners", "members"}, // owners inherit member grants, never the reverse
		{"creator", "owners"},
		{"invitee", "members"}, // InviteMember
	} {
		require.NoError(t, g.AddAssignment(&ngac.Assignment{
			ID: string(rune('a' + i)), ChildID: a.child, ParentID: a.parent,
		}))
	}

	allOwnerOps := []string{"read", "write", "approve", "upload", "share", "manage", "invite", "create_channel"}
	for i, a := range []struct {
		ua, oa string
		ops    []string
	}{
		{"owners", "mgmt", allOwnerOps},
		{"owners", "docs", allOwnerOps},
		{"owners", "channels", allOwnerOps},
		{"members", "docs", []string{"read"}},
		{"members", "channels", []string{"read", "write", "create_channel"}},
	} {
		require.NoError(t, g.AddAssociation(&ngac.Association{
			ID: "assoc" + string(rune('a'+i)), UAID: a.ua, OAID: a.oa, Operations: a.ops,
		}))
	}

	return g
}

// A plain member must not inherit owner privileges. The workspace bootstrap
// intends Members to hold only read-on-docs and the channel ops; the owner-only
// operations live on the Owners UA.
func TestWorkspaceMemberDoesNotInheritOwnerPrivileges(t *testing.T) {
	g := buildWorkspaceBootstrapGraph(t)

	// Sanity: the creator IS an owner and may manage the workspace.
	assert.Equal(t, "ALLOW", g.CheckAccess("creator", "mgmt", "manage").Decision,
		"creator is assigned to Owners UA and must be able to manage")

	// Sanity: the intended member grant works.
	assert.Equal(t, "ALLOW", g.CheckAccess("invitee", "docs", "read").Decision,
		"members are granted read on Documents")

	// The actual boundary: owner-only operations must be DENIED to a member.
	for _, tc := range []struct{ object, op string }{
		{"mgmt", "manage"},
		{"mgmt", "invite"},
		{"docs", "write"},
		{"docs", "approve"},
		{"docs", "share"},
		{"channels", "manage"},
	} {
		d := g.CheckAccess("invitee", tc.object, tc.op)
		assert.Equalf(t, "DENY", d.Decision,
			"member must NOT have %q on %s (got %s via %v)",
			tc.op, tc.object, d.Decision, d.Explanation.Path)
	}
}

// The inheritance runs the other way: an owner picks up the member grants on
// top of the owner ones. This is what the Owners→Members assignment is for, and
// asserting it stops a "fix" that simply deletes the assignment.
func TestWorkspaceOwnerInheritsMemberGrants(t *testing.T) {
	g := buildWorkspaceBootstrapGraph(t)

	// create_channel is granted to Members only — the owner set does contain it,
	// so use the association that exists solely on the member side of channels.
	assert.Equal(t, "ALLOW", g.CheckAccess("creator", "channels", "create_channel").Decision,
		"owner must reach the member grants through Owners→Members")
	assert.Equal(t, "ALLOW", g.CheckAccess("creator", "docs", "read").Decision,
		"owner must retain read on Documents")
}

// A user who is one hop short of any workspace UA, and a user whose attributes
// reach a different PC, must both be denied. The intersection principle is
// where cross-tenant leaks live.
func TestWorkspaceOutsiderAndCrossPCDenied(t *testing.T) {
	g := buildWorkspaceBootstrapGraph(t)

	// Outsider: a real U node with no assignment into this workspace at all.
	g.AddNode(&ngac.NGACNode{ID: "outsider", Name: "Outsider", NodeType: "U"})
	for _, obj := range []string{"docs", "channels", "mgmt"} {
		assert.Equalf(t, "DENY", g.CheckAccess("outsider", obj, "read").Decision,
			"unassigned user must not read %s", obj)
	}

	// Cross-PC: a user fully attributed inside a *second* workspace must not
	// reach the first workspace's objects, even though both graphs are shaped
	// identically and an association exists on their own side.
	g.AddNode(&ngac.NGACNode{ID: "pc2", Name: "PC_ws2", NodeType: "PC"})
	g.AddNode(&ngac.NGACNode{ID: "members2", Name: "ws2_Members", NodeType: "UA"})
	g.AddNode(&ngac.NGACNode{ID: "stranger", Name: "Stranger", NodeType: "U"})
	require.NoError(t, g.AddAssignment(&ngac.Assignment{ID: "x1", ChildID: "members2", ParentID: "pc2"}))
	require.NoError(t, g.AddAssignment(&ngac.Assignment{ID: "x2", ChildID: "stranger", ParentID: "members2"}))
	require.NoError(t, g.AddAssociation(&ngac.Association{
		ID: "xassoc", UAID: "members2", OAID: "docs", Operations: []string{"read"},
	}))

	d := g.CheckAccess("stranger", "docs", "read")
	assert.Equal(t, "DENY", d.Decision,
		"user in PC_ws2 must not reach PC_ws objects despite a direct association")
}

// buildChannelGraph extends the workspace bootstrap with one channel, wired the
// way messaging.Service.CreateChannel wires it: a Content OA under the
// workspace's Channels OA, a Members UA under the workspace PC, and an
// association from that Members UA to the Content OA.
//
// memberOps comes from the shared vocabulary rather than a local copy, so a
// change to ngac.ChannelMemberOps() is felt here instead of drifting silently.
func buildChannelGraph(t *testing.T, memberOps []string) *ngac.Graph {
	t.Helper()
	g := buildWorkspaceBootstrapGraph(t)

	g.AddNode(&ngac.NGACNode{ID: "chContent", Name: "Ch_c1_Content", NodeType: "OA"})
	g.AddNode(&ngac.NGACNode{ID: "chMembers", Name: "Ch_c1_Members", NodeType: "UA"})
	g.AddNode(&ngac.NGACNode{ID: "chUser", Name: "ChannelUser", NodeType: "U"})

	require.NoError(t, g.AddAssignment(&ngac.Assignment{ID: "c1", ChildID: "chContent", ParentID: "channels"}))
	require.NoError(t, g.AddAssignment(&ngac.Assignment{ID: "c2", ChildID: "chMembers", ParentID: "pc"}))
	require.NoError(t, g.AddAssignment(&ngac.Assignment{ID: "c3", ChildID: "chUser", ParentID: "chMembers"}))
	require.NoError(t, g.AddAssociation(&ngac.Association{
		ID: "cassoc", UAID: "chMembers", OAID: "chContent", Operations: memberOps,
	}))

	return g
}

// Channel members may bring other people into their own channel, so the
// channel's Members UA must carry invite on its Content OA.
func TestChannelMemberCanInviteIntoOwnChannel(t *testing.T) {
	g := buildChannelGraph(t, vocab.ChannelMemberOps())

	assert.Equal(t, "ALLOW", g.CheckAccess("chUser", "chContent", "invite").Decision,
		"a channel member must be able to add someone to that channel")
	assert.Equal(t, "ALLOW", g.CheckAccess("chUser", "chContent", "read").Decision)
	assert.Equal(t, "ALLOW", g.CheckAccess("chUser", "chContent", "write").Decision)
}

// The grant is scoped to the channel the user actually belongs to. Being a
// member of one channel must not confer invite on another.
func TestChannelMemberCannotInviteIntoOtherChannel(t *testing.T) {
	g := buildChannelGraph(t, vocab.ChannelMemberOps())

	// A second channel in the same workspace, which chUser is NOT a member of.
	g.AddNode(&ngac.NGACNode{ID: "ch2Content", Name: "Ch_c2_Content", NodeType: "OA"})
	g.AddNode(&ngac.NGACNode{ID: "ch2Members", Name: "Ch_c2_Members", NodeType: "UA"})
	require.NoError(t, g.AddAssignment(&ngac.Assignment{ID: "d1", ChildID: "ch2Content", ParentID: "channels"}))
	require.NoError(t, g.AddAssignment(&ngac.Assignment{ID: "d2", ChildID: "ch2Members", ParentID: "pc"}))
	require.NoError(t, g.AddAssociation(&ngac.Association{
		ID: "dassoc", UAID: "ch2Members", OAID: "ch2Content", Operations: vocab.ChannelMemberOps(),
	}))

	assert.Equal(t, "DENY", g.CheckAccess("chUser", "ch2Content", "invite").Decision,
		"membership in c1 must not grant invite on c2")

	// And a plain workspace member, in no channel at all, gets nothing.
	assert.Equal(t, "DENY", g.CheckAccess("invitee", "chContent", "invite").Decision,
		"a workspace member who is not in the channel must not invite into it")
}
