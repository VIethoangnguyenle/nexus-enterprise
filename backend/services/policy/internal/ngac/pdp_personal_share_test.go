package ngac_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/services/policy/internal/ngac"
)

// personalShareGraph is the shape a drive share to one person produces:
//
//	PC_Global <- PublicUsers <- {alice, bob, mallory}
//	PC_ws     <- Members     <- {alice, bob}        (mallory is outside the workspace)
//	PC_ws     <- Docs <- Folder
//	PC_Global <- ShareOA <- Folder                   (Folder reaches both policy classes)
//	alice and mallory each sit in a personal UA; the share is associated from those.
func personalShareGraph(t *testing.T, ops []string) *ngac.Graph {
	t.Helper()
	g := ngac.NewGraph()
	for id, typ := range map[string]string{
		"pcg": "PC", "pcws": "PC", "public": "UA", "members": "UA",
		"alice": "U", "bob": "U", "mallory": "U",
		"alice-ua": "UA", "mallory-ua": "UA",
		"docs": "OA", "folder": "OA", "shareoa": "OA",
	} {
		g.AddNode(&ngac.NGACNode{ID: id, Name: id, NodeType: typ})
	}
	n := 0
	assign := func(child, parent string) {
		n++
		require.NoError(t, g.AddAssignment(&ngac.Assignment{ID: string(rune('a' + n)), ChildID: child, ParentID: parent}))
	}
	assign("public", "pcg")
	assign("members", "pcws")
	for _, u := range []string{"alice", "bob", "mallory"} {
		assign(u, "public")
	}
	assign("alice", "members")
	assign("bob", "members")
	assign("alice", "alice-ua")
	assign("mallory", "mallory-ua")
	assign("docs", "pcws")
	assign("folder", "docs")
	assign("shareoa", "pcg")
	assign("folder", "shareoa")

	// Members may only read Documents; the share is what adds more for alice.
	require.NoError(t, g.AddAssociation(&ngac.Association{ID: "m", UAID: "members", OAID: "docs", Operations: []string{"read"}}))
	require.NoError(t, g.AddAssociation(&ngac.Association{ID: "s", UAID: "alice-ua", OAID: "shareoa", Operations: ops}))
	// Mallory's personal UA is associated too, as if the share had been made to
	// them - the policy classes, not the association, must stop them.
	require.NoError(t, g.AddAssociation(&ngac.Association{ID: "x", UAID: "mallory-ua", OAID: "shareoa", Operations: ops}))
	return g
}

func TestPersonalShare_WriteShareGrantsReadAndWriteToThatPersonOnly(t *testing.T) {
	g := personalShareGraph(t, []string{"read", "write"})

	assert.Equal(t, ngac.DecisionAllow, g.CheckAccess("alice", "folder", "read").Decision)
	assert.Equal(t, ngac.DecisionAllow, g.CheckAccess("alice", "folder", "write").Decision)

	// One hop short: another member holds the Members grant (read) and nothing
	// from the share.
	assert.Equal(t, ngac.DecisionAllow, g.CheckAccess("bob", "folder", "read").Decision)
	assert.Equal(t, ngac.DecisionDeny, g.CheckAccess("bob", "folder", "write").Decision)
}

func TestPersonalShare_DoesNotGrantOperationsOutsideTheShare(t *testing.T) {
	g := personalShareGraph(t, []string{"read"})

	assert.Equal(t, ngac.DecisionDeny, g.CheckAccess("alice", "folder", "write").Decision, "a read share grants no write")
	for _, op := range []string{"share", "manage", "approve", "upload", "invite"} {
		assert.Equal(t, ngac.DecisionDeny, g.CheckAccess("alice", "folder", op).Decision, op)
	}
}

func TestPersonalShare_OutsideTheWorkspaceIsDeniedByPolicyClass(t *testing.T) {
	g := personalShareGraph(t, []string{"read", "write"})

	// Mallory is in an associated personal UA but does not reach PC_ws, which
	// the folder also reaches: the intersection rule denies every operation.
	for _, op := range []string{"read", "write"} {
		assert.Equal(t, ngac.DecisionDeny, g.CheckAccess("mallory", "folder", op).Decision, op)
	}
}

func TestPersonalShare_AssociationMustStartFromAUA(t *testing.T) {
	g := personalShareGraph(t, []string{"read"})
	// The user node itself cannot be the source - which is why the personal UA exists.
	err := g.ValidateAssociation(&ngac.Association{ID: "bad", UAID: "alice", OAID: "shareoa", Operations: []string{"read"}})
	require.Error(t, err)
	assert.ErrorIs(t, err, ngac.ErrInvalidAssociation)
	assert.NoError(t, g.ValidateAssociation(&ngac.Association{ID: "ok", UAID: "alice-ua", OAID: "shareoa", Operations: []string{"read"}}))
}
