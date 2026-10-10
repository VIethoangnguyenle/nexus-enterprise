package ngac_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	vocab "ngac-platform/ngac"
	"ngac-platform/services/policy/internal/ngac"
)

// Test vectors for asset authorization (docs/specs/asset-authorization).
//
// The graph holds attributes, not objects. An asset is authorized on the OA of
// its type, so the tree these vectors build is the one the asset service
// creates:
//
//	PC_ws1 ← ws1_Assets ← ws1_Category_hardware ← ws1_Type_laptop / ws1_Type_phone
//	                   ← ws1_Category_software ← ws1_Type_license
//
// with the Owners UA associated to the Assets OA and nothing built under a
// second policy class.

func buildAssetTree(t *testing.T) *ngac.Graph {
	t.Helper()
	g := ngac.NewGraph()
	ws1, ws2 := vocab.WorkspaceID("ws1"), vocab.WorkspaceID("ws2")

	for _, n := range []*ngac.NGACNode{
		{ID: "pc1", Name: vocab.PCName(ws1), NodeType: "PC"},
		{ID: "pc2", Name: vocab.PCName(ws2), NodeType: "PC"},
		{ID: "owners1", Name: vocab.OwnersUAName(ws1), NodeType: "UA"},
		{ID: "members1", Name: vocab.MembersUAName(ws1), NodeType: "UA"},
		{ID: "dept1", Name: vocab.DeptUAName("d1"), NodeType: "UA"},
		{ID: "owners2", Name: vocab.OwnersUAName(ws2), NodeType: "UA"},
		{ID: "assets1", Name: vocab.AssetsOAName(ws1), NodeType: "OA"},
		{ID: "cat-hw", Name: vocab.AssetCategoryOAName(ws1, "hardware"), NodeType: "OA"},
		{ID: "cat-sw", Name: vocab.AssetCategoryOAName(ws1, "software"), NodeType: "OA"},
		{ID: "type-laptop", Name: vocab.AssetTypeOAName(ws1, "laptop"), NodeType: "OA"},
		{ID: "type-phone", Name: vocab.AssetTypeOAName(ws1, "phone"), NodeType: "OA"},
		{ID: "type-license", Name: vocab.AssetTypeOAName(ws1, "license"), NodeType: "OA"},
		{ID: "assets2", Name: vocab.AssetsOAName(ws2), NodeType: "OA"},
		{ID: "type-other", Name: vocab.AssetTypeOAName(ws2, "laptop"), NodeType: "OA"},
		{ID: "owner1", Name: "owner1", NodeType: "U"},
		{ID: "member1", Name: "member1", NodeType: "U"},
		{ID: "deptmember", Name: "deptmember", NodeType: "U"},
		{ID: "owner2", Name: "owner2", NodeType: "U"},
	} {
		g.AddNode(n)
	}
	for i, a := range [][2]string{
		{"owners1", "pc1"}, {"members1", "pc1"}, {"dept1", "pc1"}, {"owners1", "members1"},
		{"assets1", "pc1"}, {"cat-hw", "assets1"}, {"cat-sw", "assets1"},
		{"type-laptop", "cat-hw"}, {"type-phone", "cat-hw"}, {"type-license", "cat-sw"},
		{"owner1", "owners1"}, {"member1", "members1"}, {"deptmember", "dept1"},
		{"owners2", "pc2"}, {"assets2", "pc2"}, {"type-other", "assets2"}, {"owner2", "owners2"},
	} {
		require.NoError(t, g.AddAssignment(&ngac.Assignment{ID: string(rune('a' + i)), ChildID: a[0], ParentID: a[1]}))
	}
	for i, a := range []struct {
		ua, oa string
		ops    []string
	}{
		{"owners1", "assets1", vocab.AllOwnerOps()},
		{"owners2", "assets2", vocab.AllOwnerOps()},
	} {
		require.NoError(t, g.AddAssociation(&ngac.Association{ID: "assoc" + string(rune('a'+i)), UAID: a.ua, OAID: a.oa, Operations: a.ops}))
	}
	return g
}

func allow(g *ngac.Graph, user, object, op string) bool {
	return g.CheckAccess(user, object, op).Decision == vocab.DecisionAllow
}

// The owner's grant on the Assets OA reaches every type OA beneath it, for
// every owner operation.
func TestAssetTree_OwnerHoldsEveryOperationOnEveryTypeOA(t *testing.T) {
	g := buildAssetTree(t)
	for _, oa := range []string{"type-laptop", "type-phone", "type-license"} {
		for _, op := range vocab.AllOwnerOps() {
			assert.Truef(t, allow(g, "owner1", oa, op), "owner must hold %s on %s", op, oa)
		}
	}
}

// One hop short: a workspace member, and a department member, hold nothing on
// the asset tree because nothing is associated for them.
func TestAssetTree_MemberAndDepartmentMemberAreDenied(t *testing.T) {
	g := buildAssetTree(t)
	for _, user := range []string{"member1", "deptmember"} {
		for _, oa := range []string{"type-laptop", "type-phone", "type-license", "assets1"} {
			for _, op := range vocab.AllOwnerOps() {
				assert.Falsef(t, allow(g, user, oa, op), "%s must not hold %s on %s", user, op, oa)
			}
		}
	}
}

// The intersection principle across tenants: the other workspace's owner holds
// every operation on its own tree, none on this one, even with an association
// aimed straight at this workspace's type OA.
func TestAssetTree_OtherWorkspaceIsDeniedEvenWithAnAssociation(t *testing.T) {
	g := buildAssetTree(t)
	require.True(t, allow(g, "owner2", "type-other", "manage"), "precondition: owner2 owns its own tree")
	require.NoError(t, g.AddAssociation(&ngac.Association{
		ID: "stray", UAID: "owners2", OAID: "type-laptop", Operations: vocab.AllOwnerOps(),
	}))

	for _, op := range vocab.AllOwnerOps() {
		assert.Falsef(t, allow(g, "owner2", "type-laptop", op), "%s on another workspace's type OA", op)
	}
	assert.False(t, allow(g, "owner1", "type-other", "read"), "and the reverse")
}

// A grant on a category reaches the types under it and no others; a grant on
// one type reaches no other type; and a grant carries only the operations it
// names.
func TestAssetTree_GrantsReachDownwardOnlyAndOnlyTheirOperations(t *testing.T) {
	g := buildAssetTree(t)
	require.NoError(t, g.AddAssociation(&ngac.Association{
		ID: "hw-read", UAID: "members1", OAID: "cat-hw", Operations: []string{"read"},
	}))
	require.NoError(t, g.AddAssociation(&ngac.Association{
		ID: "phone-write", UAID: "dept1", OAID: "type-phone", Operations: []string{"write"},
	}))

	// category grant → both hardware types
	assert.True(t, allow(g, "member1", "type-laptop", "read"))
	assert.True(t, allow(g, "member1", "type-phone", "read"))
	// ... not the sibling category, not the parent, not other operations
	assert.False(t, allow(g, "member1", "type-license", "read"), "software is another category")
	assert.False(t, allow(g, "member1", "assets1", "read"), "a grant does not reach up")
	assert.False(t, allow(g, "member1", "type-laptop", "write"))
	assert.False(t, allow(g, "member1", "type-laptop", "manage"))
	// type grant → that type only
	assert.True(t, allow(g, "deptmember", "type-phone", "write"))
	assert.False(t, allow(g, "deptmember", "type-laptop", "write"), "a grant on one type is not a grant on its sibling")
	assert.False(t, allow(g, "deptmember", "type-phone", "read"))
}

// Prohibitions override an ALLOW at the type OA, and only an ALLOW.
func TestAssetTree_ProhibitionOverridesAllowOnTheTypeOA(t *testing.T) {
	g := buildAssetTree(t)
	require.NoError(t, g.AddProhibition(&ngac.Prohibition{
		Name: "no-phone-manage", SubjectID: "owners1", Operations: []string{"manage"}, TargetOAIDs: []string{"type-phone"},
	}))
	e := ngac.NewDecisionEngine(g, nil)

	assert.Equal(t, vocab.DecisionDeny, decide(t, e, "owner1", "type-phone", "manage").Decision, "prohibited")
	assert.Equal(t, vocab.DecisionAllow, decide(t, e, "owner1", "type-phone", "read").Decision, "another operation is untouched")
	assert.Equal(t, vocab.DecisionAllow, decide(t, e, "owner1", "type-laptop", "manage").Decision, "another type is untouched")
	d := decide(t, e, "member1", "type-phone", "manage")
	assert.Equal(t, vocab.DecisionDeny, d.Decision)
	assert.Nil(t, d.Explanation.ProhibitionDenied, "a DENY that never was an ALLOW is not a prohibition denial")
}

// An asset is not a node. Asking about its ID — or about an O node that no
// longer exists — is a DENY, never an ALLOW inherited from its type.
func TestAssetTree_AnAssetIDIsNotAnObject(t *testing.T) {
	g := buildAssetTree(t)
	e := ngac.NewDecisionEngine(g, nil)

	d := decide(t, e, "owner1", "asset-0001", "read")

	assert.Equal(t, vocab.DecisionDeny, d.Decision)
	assert.Equal(t, ngac.DenyReasonNodeNotFound, d.Explanation.Reason)
}

// Why the tree hangs under the workspace PC alone. An access needs the user to
// reach every policy class the object reaches. An Assets OA that is also under
// a global asset-management class which no workspace UA is assigned to would be
// out of reach of its own owners.
func TestAssetTree_ASecondPolicyClassNobodyReachesLocksOutTheOwners(t *testing.T) {
	g := buildAssetTree(t)
	g.AddNode(&ngac.NGACNode{ID: "pc-assets", Name: "PC_AssetManagement", NodeType: "PC"})
	require.NoError(t, g.AddAssignment(&ngac.Assignment{ID: "second", ChildID: "assets1", ParentID: "pc-assets"}))

	assert.False(t, allow(g, "owner1", "type-laptop", "read"),
		"the owner reaches PC_ws1 but not the second class: DENY, which is why the tree has no second class")
}

// The graph resolves a node by exact name and keeps one node per name and type.
// Names keyed by something two tenants can both choose collapse onto one node;
// names keyed by IDs do not. This is the reason every platform-built node name
// takes an ID (backend/ngac/ids.go).
func TestIDKeyedNamesKeepTenantsApart(t *testing.T) {
	g := ngac.NewGraph()
	g.AddNode(&ngac.NGACNode{ID: "dept-a", Name: vocab.DeptUAName("dept-id-a"), NodeType: "UA"})
	g.AddNode(&ngac.NGACNode{ID: "dept-b", Name: vocab.DeptUAName("dept-id-b"), NodeType: "UA"})

	a := g.FindNodeByName(vocab.DeptUAName("dept-id-a"), "UA")
	b := g.FindNodeByName(vocab.DeptUAName("dept-id-b"), "UA")
	require.NotNil(t, a)
	require.NotNil(t, b)
	assert.Equal(t, "dept-a", a.ID)
	assert.Equal(t, "dept-b", b.ID, "two tenants' departments are two nodes")

	// The failure mode the IDs prevent: the second "Sales" shadows the first.
	g.AddNode(&ngac.NGACNode{ID: "sales-a", Name: "Dept_Sales", NodeType: "UA"})
	g.AddNode(&ngac.NGACNode{ID: "sales-b", Name: "Dept_Sales", NodeType: "UA"})
	assert.Equal(t, "sales-b", g.FindNodeByName("Dept_Sales", "UA").ID,
		"with a name-keyed node the last writer wins and tenant A's lookup lands on tenant B's UA")
}

// Why migration 024 deletes the legacy blanket associations before it removes
// the second policy class: with only the workspace class over the tree, a
// Members UA associated with the Assets OA for every operation is live, and a
// plain member would hold manage and approve on every asset.
func TestAssetTree_LegacyBlanketGrantIsLiveOnceTheSecondPolicyClassIsGone(t *testing.T) {
	g := buildAssetTree(t)
	require.NoError(t, g.AddAssociation(&ngac.Association{
		ID: "legacy", UAID: "members1", OAID: "assets1", Operations: vocab.AllOwnerOps(),
	}))
	assert.True(t, allow(g, "member1", "type-laptop", "manage"), "the blanket grant opens the tree to a member")
	assert.True(t, allow(g, "deptmember", "type-laptop", "manage") == false, "a department member without it stays out")

	g.RemoveAssociationByID("legacy") // what the migration does
	for _, op := range vocab.AllOwnerOps() {
		assert.Falsef(t, allow(g, "member1", "type-laptop", op), "%s after the migration", op)
	}
	assert.True(t, allow(g, "owner1", "type-laptop", "manage"), "owners keep their grant")
}
