package ngac_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/services/policy/internal/ngac"
)

// fileShareGraph is the shape the drive builds when one file in a folder is
// shared. Files have no node until they are shared; a shared file gets an OA of
// its own beneath its folder's, and only that OA goes under the share OA, so the
// share reaches the file and nothing else in the folder:
//
//	PC_Global <- PublicUsers <- {alice, bob, mallory}
//	PC_ws     <- Members     <- {alice, bob}           (mallory is another tenant's)
//	PC_ws     <- General                               (what members may read)
//	PC_ws     <- Private <- File                       (the folder and its shared file)
//	PC_Global <- ShareOA <- File
//
// alice and mallory each sit in a personal UA associated with the share OA.
// Members hold read on General only, never on Private, so whatever alice can do
// in Private comes from the share.
func fileShareGraph(t *testing.T, shareOps []string, shareWholeFolder bool) *ngac.Graph {
	t.Helper()
	g := ngac.NewGraph()
	for id, typ := range map[string]string{
		"pcg": "PC", "pcws": "PC", "public": "UA", "members": "UA",
		"alice": "U", "bob": "U", "mallory": "U",
		"alice-ua": "UA", "mallory-ua": "UA",
		"general": "OA", "private": "OA", "file": "OA", "shareoa": "OA",
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
	assign("general", "pcws")
	assign("private", "pcws")
	assign("file", "private")
	assign("shareoa", "pcg")
	if shareWholeFolder {
		// The shape a file share used to produce: the file had no node, so its
		// folder's OA went under the share OA.
		assign("private", "shareoa")
	} else {
		assign("file", "shareoa")
	}

	require.NoError(t, g.AddAssociation(&ngac.Association{ID: "m", UAID: "members", OAID: "general", Operations: []string{"read"}}))
	require.NoError(t, g.AddAssociation(&ngac.Association{ID: "s", UAID: "alice-ua", OAID: "shareoa", Operations: shareOps}))
	// As if the share had also been made to someone of another tenant: the
	// policy classes, not the association, must stop them.
	require.NoError(t, g.AddAssociation(&ngac.Association{ID: "x", UAID: "mallory-ua", OAID: "shareoa", Operations: shareOps}))
	return g
}

// Deny first: the share target reaches the shared file and not the folder it
// sits in, so every other file of the folder (authorized on the folder's OA)
// stays unreadable to them.
func TestFileShare_TheRestOfTheFolderStaysUnreadableToTheShareTarget(t *testing.T) {
	g := fileShareGraph(t, []string{"read", "write"}, false)

	for _, op := range []string{"read", "write"} {
		assert.Equal(t, ngac.DecisionDeny, g.CheckAccess("alice", "private", op).Decision,
			"the folder (and every unshared file in it) is not reachable through a file share: %s", op)
	}
}

func TestFileShare_GrantsTheSharedFileToTheTarget(t *testing.T) {
	g := fileShareGraph(t, []string{"read", "write"}, false)

	assert.Equal(t, ngac.DecisionAllow, g.CheckAccess("alice", "file", "read").Decision)
	assert.Equal(t, ngac.DecisionAllow, g.CheckAccess("alice", "file", "write").Decision)
}

func TestFileShare_OneHopShortIsDenied(t *testing.T) {
	g := fileShareGraph(t, []string{"read"}, false)

	// bob is a member of the same workspace but not the share's target.
	for _, op := range []string{"read", "write"} {
		assert.Equal(t, ngac.DecisionDeny, g.CheckAccess("bob", "file", op).Decision, op)
	}
	// A read share grants nothing else.
	for _, op := range []string{"write", "share", "manage", "upload"} {
		assert.Equal(t, ngac.DecisionDeny, g.CheckAccess("alice", "file", op).Decision, op)
	}
}

func TestFileShare_AnotherTenantIsDeniedByPolicyClass(t *testing.T) {
	g := fileShareGraph(t, []string{"read", "write"}, false)

	// mallory's UA is associated, but mallory does not reach PC_ws, which the
	// file also reaches.
	for _, op := range []string{"read", "write"} {
		assert.Equal(t, ngac.DecisionDeny, g.CheckAccess("mallory", "file", op).Decision, op)
		assert.Equal(t, ngac.DecisionDeny, g.CheckAccess("mallory", "private", op).Decision, op)
	}
}

func TestFileShare_AProhibitionOverridesTheShare(t *testing.T) {
	g := fileShareGraph(t, []string{"read", "write"}, false)
	require.NoError(t, g.AddProhibition(&ngac.Prohibition{
		Name: "no-writes-on-file", SubjectID: "alice-ua",
		Operations: []string{"write"}, TargetOAIDs: []string{"file"},
	}))
	e := ngac.NewDecisionEngine(g, nil)

	require.Equal(t, ngac.DecisionAllow, g.CheckAccess("alice", "file", "write").Decision,
		"precondition: the share grants write")
	assert.Equal(t, ngac.DecisionDeny, decide(t, e, "alice", "file", "write").Decision)
	assert.Equal(t, ngac.DecisionAllow, decide(t, e, "alice", "file", "read").Decision)
}

// Why the file needs its own OA: with the folder's OA under the share OA, the
// same share hands the target the whole folder.
func TestFileShare_SharingTheFoldersOAWouldGrantTheWholeFolder(t *testing.T) {
	g := fileShareGraph(t, []string{"read", "write"}, true)

	assert.Equal(t, ngac.DecisionAllow, g.CheckAccess("alice", "private", "read").Decision)
	assert.Equal(t, ngac.DecisionAllow, g.CheckAccess("alice", "private", "write").Decision)
}
