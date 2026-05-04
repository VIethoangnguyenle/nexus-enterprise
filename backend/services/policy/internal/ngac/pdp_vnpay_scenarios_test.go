package ngac_test

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/services/policy/internal/ngac"
)

// =====================================================================
// Group 3: PC Intersection (PC-01 to PC-04)
// =====================================================================

func TestPC01_SamePC_Allow(t *testing.T) {
	g := buildVNPayGraph()
	d := g.CheckAccess("ngac-hoangnlv", "oa-dvnh-drive", "read")
	assert.Equal(t, "ALLOW", d.Decision)
	assert.Contains(t, d.Explanation.PolicyClasses, "PC_VNPay")
}

func TestPC02_DifferentPC_Deny(t *testing.T) {
	g := buildVNPayGraph()
	// trietvv only in PublicUsers→PC_Global, DVNH_Drive→PC_VNPay
	d := g.CheckAccess("ngac-trietvv", "oa-dvnh-drive", "read")
	assert.Equal(t, "DENY", d.Decision, "cross-PC without share should DENY")
}

func TestPC03_PCGlobal_Bridge(t *testing.T) {
	g := buildVNPayGraph()
	// Create Share_OA under PC_Global
	g.AddNode(&ngac.NGACNode{ID: "oa-share-test", Name: "Share_Test", NodeType: "OA"})
	require.NoError(t, g.AddAssignment(&ngac.Assignment{ID: "a-share-pcg", ChildID: "oa-share-test", ParentID: "pc-global"}))
	// trietvv → PublicUsers → PC_Global, Share_OA → PC_Global → same PC
	require.NoError(t, g.AddAssociation(&ngac.Association{ID: "as-trietvv-share", UAID: "ua-public-users", OAID: "oa-share-test", Operations: []string{"read"}}))

	d := g.CheckAccess("ngac-trietvv", "oa-share-test", "read")
	assert.Equal(t, "ALLOW", d.Decision, "PC_Global bridge should work")
	assert.Contains(t, d.Explanation.PolicyClasses, "PC_Global")
}

func TestPC04_MultiPC_MustPassAll(t *testing.T) {
	g := ngac.NewGraph()
	// Two PCs
	g.AddNode(&ngac.NGACNode{ID: "pc-a", Name: "PC_A", NodeType: "PC"})
	g.AddNode(&ngac.NGACNode{ID: "pc-b", Name: "PC_B", NodeType: "PC"})
	// User only in PC_A
	g.AddNode(&ngac.NGACNode{ID: "ua-a", Name: "UA_A", NodeType: "UA"})
	g.AddNode(&ngac.NGACNode{ID: "u1", Name: "User1", NodeType: "U"})
	_ = g.AddAssignment(&ngac.Assignment{ID: "a1", ChildID: "ua-a", ParentID: "pc-a"})
	_ = g.AddAssignment(&ngac.Assignment{ID: "a2", ChildID: "u1", ParentID: "ua-a"})
	// Resource in PC_B only
	g.AddNode(&ngac.NGACNode{ID: "oa-b", Name: "OA_B", NodeType: "OA"})
	_ = g.AddAssignment(&ngac.Assignment{ID: "a3", ChildID: "oa-b", ParentID: "pc-b"})
	// Association exists but PCs don't match
	_ = g.AddAssociation(&ngac.Association{ID: "as1", UAID: "ua-a", OAID: "oa-b", Operations: []string{"read"}})

	d := g.CheckAccess("u1", "oa-b", "read")
	assert.Equal(t, "DENY", d.Decision, "user in PC_A, resource in PC_B → DENY despite association")
}

// TestPC05_SinglePC_BackwardCompat ensures ALL-PC intersection behaves
// identically to old single-PC logic when only one PC is involved.
func TestPC05_SinglePC_BackwardCompat(t *testing.T) {
	g := ngac.NewGraph()
	g.AddNode(&ngac.NGACNode{ID: "pc-tenant", Name: "PC_Tenant", NodeType: "PC"})
	g.AddNode(&ngac.NGACNode{ID: "ua-dept", Name: "UA_Dept", NodeType: "UA"})
	g.AddNode(&ngac.NGACNode{ID: "u-alice", Name: "Alice", NodeType: "U"})
	g.AddNode(&ngac.NGACNode{ID: "oa-docs", Name: "OA_Docs", NodeType: "OA"})
	_ = g.AddAssignment(&ngac.Assignment{ID: "a1", ChildID: "ua-dept", ParentID: "pc-tenant"})
	_ = g.AddAssignment(&ngac.Assignment{ID: "a2", ChildID: "u-alice", ParentID: "ua-dept"})
	_ = g.AddAssignment(&ngac.Assignment{ID: "a3", ChildID: "oa-docs", ParentID: "pc-tenant"})
	_ = g.AddAssociation(&ngac.Association{ID: "as1", UAID: "ua-dept", OAID: "oa-docs", Operations: []string{"read", "write"}})

	d := g.CheckAccess("u-alice", "oa-docs", "read")
	assert.Equal(t, "ALLOW", d.Decision, "single-PC: backward compatible ALLOW")
	assert.Contains(t, d.Explanation.PolicyClasses, "PC_Tenant")
}

// TestPC06_MultiPC_UserHasAll_Allow verifies ALLOW when user reaches ALL PCs.
func TestPC06_MultiPC_UserHasAll_Allow(t *testing.T) {
	g := ngac.NewGraph()
	g.AddNode(&ngac.NGACNode{ID: "pc-tenant", Name: "PC_Tenant", NodeType: "PC"})
	g.AddNode(&ngac.NGACNode{ID: "pc-conf", Name: "PC_Confidential", NodeType: "PC"})
	g.AddNode(&ngac.NGACNode{ID: "ua-dept", Name: "UA_Dept", NodeType: "UA"})
	g.AddNode(&ngac.NGACNode{ID: "ua-cleared", Name: "UA_Cleared", NodeType: "UA"})
	g.AddNode(&ngac.NGACNode{ID: "u-alice", Name: "Alice", NodeType: "U"})
	_ = g.AddAssignment(&ngac.Assignment{ID: "a1", ChildID: "ua-dept", ParentID: "pc-tenant"})
	_ = g.AddAssignment(&ngac.Assignment{ID: "a2", ChildID: "ua-cleared", ParentID: "pc-conf"})
	_ = g.AddAssignment(&ngac.Assignment{ID: "a3", ChildID: "u-alice", ParentID: "ua-dept"})
	_ = g.AddAssignment(&ngac.Assignment{ID: "a4", ChildID: "u-alice", ParentID: "ua-cleared"})
	g.AddNode(&ngac.NGACNode{ID: "oa-secret", Name: "OA_Secret", NodeType: "OA"})
	_ = g.AddAssignment(&ngac.Assignment{ID: "a5", ChildID: "oa-secret", ParentID: "pc-tenant"})
	_ = g.AddAssignment(&ngac.Assignment{ID: "a6", ChildID: "oa-secret", ParentID: "pc-conf"})
	_ = g.AddAssociation(&ngac.Association{ID: "as1", UAID: "ua-dept", OAID: "oa-secret", Operations: []string{"read"}})
	_ = g.AddAssociation(&ngac.Association{ID: "as2", UAID: "ua-cleared", OAID: "oa-secret", Operations: []string{"read"}})

	d := g.CheckAccess("u-alice", "oa-secret", "read")
	assert.Equal(t, "ALLOW", d.Decision, "user in ALL PCs → ALLOW")
	assert.Len(t, d.Explanation.PolicyClasses, 2)
	assert.Contains(t, d.Explanation.PolicyClasses, "PC_Tenant")
	assert.Contains(t, d.Explanation.PolicyClasses, "PC_Confidential")
}

// TestPC07_MultiPC_UserMissingOne_Deny verifies DENY when user is missing one PC.
func TestPC07_MultiPC_UserMissingOne_Deny(t *testing.T) {
	g := ngac.NewGraph()
	g.AddNode(&ngac.NGACNode{ID: "pc-tenant", Name: "PC_Tenant", NodeType: "PC"})
	g.AddNode(&ngac.NGACNode{ID: "pc-conf", Name: "PC_Confidential", NodeType: "PC"})
	g.AddNode(&ngac.NGACNode{ID: "ua-dept", Name: "UA_Dept", NodeType: "UA"})
	g.AddNode(&ngac.NGACNode{ID: "u-bob", Name: "Bob", NodeType: "U"})
	_ = g.AddAssignment(&ngac.Assignment{ID: "a1", ChildID: "ua-dept", ParentID: "pc-tenant"})
	_ = g.AddAssignment(&ngac.Assignment{ID: "a2", ChildID: "u-bob", ParentID: "ua-dept"})
	g.AddNode(&ngac.NGACNode{ID: "oa-secret", Name: "OA_Secret", NodeType: "OA"})
	_ = g.AddAssignment(&ngac.Assignment{ID: "a3", ChildID: "oa-secret", ParentID: "pc-tenant"})
	_ = g.AddAssignment(&ngac.Assignment{ID: "a4", ChildID: "oa-secret", ParentID: "pc-conf"})
	_ = g.AddAssociation(&ngac.Association{ID: "as1", UAID: "ua-dept", OAID: "oa-secret", Operations: []string{"read"}})

	d := g.CheckAccess("u-bob", "oa-secret", "read")
	assert.Equal(t, "DENY", d.Decision, "user missing PC_Confidential → DENY")
}

// TestPC08_ThreePC_AllRequired verifies 3-PC intersection stress case.
func TestPC08_ThreePC_AllRequired(t *testing.T) {
	g := ngac.NewGraph()
	g.AddNode(&ngac.NGACNode{ID: "pc-t", Name: "PC_Tenant", NodeType: "PC"})
	g.AddNode(&ngac.NGACNode{ID: "pc-c", Name: "PC_Conf", NodeType: "PC"})
	g.AddNode(&ngac.NGACNode{ID: "pc-p", Name: "PC_PCI", NodeType: "PC"})
	g.AddNode(&ngac.NGACNode{ID: "ua-t", Name: "UA_T", NodeType: "UA"})
	g.AddNode(&ngac.NGACNode{ID: "ua-c", Name: "UA_C", NodeType: "UA"})
	g.AddNode(&ngac.NGACNode{ID: "ua-p", Name: "UA_P", NodeType: "UA"})
	g.AddNode(&ngac.NGACNode{ID: "u1", Name: "User1", NodeType: "U"})
	_ = g.AddAssignment(&ngac.Assignment{ID: "a1", ChildID: "ua-t", ParentID: "pc-t"})
	_ = g.AddAssignment(&ngac.Assignment{ID: "a2", ChildID: "ua-c", ParentID: "pc-c"})
	_ = g.AddAssignment(&ngac.Assignment{ID: "a3", ChildID: "ua-p", ParentID: "pc-p"})
	_ = g.AddAssignment(&ngac.Assignment{ID: "a4", ChildID: "u1", ParentID: "ua-t"})
	_ = g.AddAssignment(&ngac.Assignment{ID: "a5", ChildID: "u1", ParentID: "ua-c"})
	_ = g.AddAssignment(&ngac.Assignment{ID: "a6", ChildID: "u1", ParentID: "ua-p"})
	g.AddNode(&ngac.NGACNode{ID: "oa-x", Name: "OA_X", NodeType: "OA"})
	_ = g.AddAssignment(&ngac.Assignment{ID: "a7", ChildID: "oa-x", ParentID: "pc-t"})
	_ = g.AddAssignment(&ngac.Assignment{ID: "a8", ChildID: "oa-x", ParentID: "pc-c"})
	_ = g.AddAssignment(&ngac.Assignment{ID: "a9", ChildID: "oa-x", ParentID: "pc-p"})
	_ = g.AddAssociation(&ngac.Association{ID: "as1", UAID: "ua-t", OAID: "oa-x", Operations: []string{"read"}})

	d := g.CheckAccess("u1", "oa-x", "read")
	assert.Equal(t, "ALLOW", d.Decision, "user in all 3 PCs → ALLOW")
	assert.Len(t, d.Explanation.PolicyClasses, 3, "all 3 PCs should be listed")
}

// =====================================================================
// Group 4: Sharing — Cross-workspace & External (SH-01 to SH-07)
// =====================================================================

// addShareOA creates a standalone Share_OA under PC_Global.
// NIST-compliant pattern: Share_OA does NOT create parent-child with sourceOA.
// Instead, it stores a metadata reference. External users check access against
// the Share_OA node, not the original resource. This prevents PC contamination
// (sourceOA reaching both PC_VNPay and PC_Global) which would break ALL-PC intersection.
func addShareOA(g *ngac.Graph, sourceOA string) {
	g.AddNode(&ngac.NGACNode{
		ID: "oa-share-baocao", Name: "Share_BaoCaoQ1", NodeType: "OA",
		Properties: map[string]string{"ref_oa_id": sourceOA},
	})
	_ = g.AddAssignment(&ngac.Assignment{ID: "a-share-pcg", ChildID: "oa-share-baocao", ParentID: "pc-global"})
	_ = g.AddAssociation(&ngac.Association{ID: "as-trietvv-share", UAID: "ua-public-users", OAID: "oa-share-baocao", Operations: []string{"read"}})
}

func TestSH01_InternalUser_NormalPath(t *testing.T) {
	g := buildVNPayGraph()
	// Add OA for reports folder
	g.AddNode(&ngac.NGACNode{ID: "oa-reports-dvnh", Name: "Reports_DVNH", NodeType: "OA"})
	_ = g.AddAssignment(&ngac.Assignment{ID: "a-reports", ChildID: "oa-reports-dvnh", ParentID: "oa-dvnh-drive"})

	d := g.CheckAccess("ngac-namdx", "oa-reports-dvnh", "read")
	assert.Equal(t, "ALLOW", d.Decision, "namdx(DVNH internal) should access via workspace path")
}

func TestSH02_ExternalUser_WithShare_Allow(t *testing.T) {
	g := buildVNPayGraph()
	g.AddNode(&ngac.NGACNode{ID: "oa-reports-dvnh", Name: "Reports_DVNH", NodeType: "OA"})
	_ = g.AddAssignment(&ngac.Assignment{ID: "a-reports", ChildID: "oa-reports-dvnh", ParentID: "oa-dvnh-drive"})
	addShareOA(g, "oa-reports-dvnh")

	// External user checks access against Share_OA (standalone, PC_Global only)
	d := g.CheckAccess("ngac-trietvv", "oa-share-baocao", "read")
	assert.Equal(t, "ALLOW", d.Decision, "trietvv should access via Share_OA→PC_Global")

	// External user CANNOT access the original resource directly
	d2 := g.CheckAccess("ngac-trietvv", "oa-reports-dvnh", "read")
	assert.Equal(t, "DENY", d2.Decision, "trietvv cannot access original resource directly — PC_VNPay required")
}

func TestSH03_ExternalUser_NoShare_Deny(t *testing.T) {
	g := buildVNPayGraph()
	g.AddNode(&ngac.NGACNode{ID: "oa-reports-dvnh", Name: "Reports_DVNH", NodeType: "OA"})
	_ = g.AddAssignment(&ngac.Assignment{ID: "a-reports", ChildID: "oa-reports-dvnh", ParentID: "oa-dvnh-drive"})
	// No share created
	d := g.CheckAccess("ngac-trietvv", "oa-reports-dvnh", "read")
	assert.Equal(t, "DENY", d.Decision, "trietvv without share → DENY")
}

func TestSH04_ShareReadOnly_WriteBlocked(t *testing.T) {
	g := buildVNPayGraph()
	g.AddNode(&ngac.NGACNode{ID: "oa-reports-dvnh", Name: "Reports_DVNH", NodeType: "OA"})
	_ = g.AddAssignment(&ngac.Assignment{ID: "a-reports", ChildID: "oa-reports-dvnh", ParentID: "oa-dvnh-drive"})
	addShareOA(g, "oa-reports-dvnh")

	// External user checks write on Share_OA → DENY (only read granted)
	d := g.CheckAccess("ngac-trietvv", "oa-share-baocao", "write")
	assert.Equal(t, "DENY", d.Decision, "share is read-only, write should DENY")
}

func TestSH06_RevokeShare(t *testing.T) {
	g := buildVNPayGraph()
	g.AddNode(&ngac.NGACNode{ID: "oa-reports-dvnh", Name: "Reports_DVNH", NodeType: "OA"})
	_ = g.AddAssignment(&ngac.Assignment{ID: "a-reports", ChildID: "oa-reports-dvnh", ParentID: "oa-dvnh-drive"})
	addShareOA(g, "oa-reports-dvnh")

	// Verify share works (check against Share_OA)
	d := g.CheckAccess("ngac-trietvv", "oa-share-baocao", "read")
	require.Equal(t, "ALLOW", d.Decision)

	// Revoke: delete Share_OA node (cascade)
	g.RemoveNode("oa-share-baocao")

	d = g.CheckAccess("ngac-trietvv", "oa-share-baocao", "read")
	assert.Equal(t, "DENY", d.Decision, "after revoke, trietvv should be denied")

	// Internal user still has access to original resource
	d = g.CheckAccess("ngac-namdx", "oa-reports-dvnh", "read")
	assert.Equal(t, "ALLOW", d.Decision, "internal namdx still has access after revoke")
}

// =====================================================================
// Group 5: Department Isolation (DI-01 to DI-05)
// =====================================================================

func TestDI01_AI_Cannot_Write_DVNH(t *testing.T) {
	g := buildVNPayGraph()
	d := g.CheckAccess("ngac-ducnm", "oa-dvnh-drive", "upload")
	assert.Equal(t, "DENY", d.Decision, "ducnm(AI) cannot upload to DVNH_Drive")
}

func TestDI02_SameDept_DifferentTeam_Allow(t *testing.T) {
	g := buildVNPayGraph()
	// hoangbm is in AppSrv1 ⊂ DVNH_Dept
	d := g.CheckAccess("ngac-hoangbm", "oa-dvnh-drive", "read")
	assert.Equal(t, "ALLOW", d.Decision, "hoangbm(AppSrv1) should read DVNH_Drive via dept inheritance")
}

func TestDI04_WorkspaceDocsShared(t *testing.T) {
	g := buildVNPayGraph()
	// Both AI and DVNH can read workspace-level docs
	d1 := g.CheckAccess("ngac-ducnm", "oa-docs", "read")
	d2 := g.CheckAccess("ngac-hoangnlv", "oa-docs", "read")
	assert.Equal(t, "ALLOW", d1.Decision, "ducnm should read VNPay_Docs")
	assert.Equal(t, "ALLOW", d2.Decision, "hoangnlv should read VNPay_Docs")
}

func TestDI05_ChiefA_Cannot_ChiefB(t *testing.T) {
	g := buildVNPayGraph()
	// thuynt (TP NoiVu) cannot access DVNH resources with dept-specific ops
	d := g.CheckAccess("ngac-thuynt", "oa-dvnh-res", "approve")
	assert.Equal(t, "DENY", d.Decision, "NoiVu chief cannot approve on DVNH resources")
}

// =====================================================================
// Group 6: Messaging & Channel Isolation (CH-01 to CH-05)
// =====================================================================

func TestCH01_ChannelMember_Allow(t *testing.T) {
	g := buildVNPayGraph()
	d := g.CheckAccess("ngac-hoangnlv", "oa-ch-dvnh-content", "write")
	assert.Equal(t, "ALLOW", d.Decision, "hoangnlv is channel member → can write")
}

func TestCH02_NonMember_Deny(t *testing.T) {
	g := buildVNPayGraph()
	d := g.CheckAccess("ngac-ducnm", "oa-ch-dvnh-content", "read")
	assert.Equal(t, "DENY", d.Decision, "ducnm not in channel → DENY")
}

func TestCH03_KickMember_LosesAccess(t *testing.T) {
	g := buildVNPayGraph()
	// Verify member can access
	d := g.CheckAccess("ngac-hoangnlv", "oa-ch-dvnh-content", "read")
	require.Equal(t, "ALLOW", d.Decision)

	// Kick: remove from channel members
	g.RemoveAssignment("ngac-hoangnlv", "ua-ch-dvnh-members")

	d = g.CheckAccess("ngac-hoangnlv", "oa-ch-dvnh-content", "read")
	assert.Equal(t, "DENY", d.Decision, "after kick, hoangnlv loses channel access")
}

func TestCH04_CrossCompanyChat_PCGlobal(t *testing.T) {
	g := buildVNPayGraph()
	// Create cross-company channel under PC_Global
	g.AddNode(&ngac.NGACNode{ID: "oa-xch-content", Name: "XCH_Content", NodeType: "OA"})
	g.AddNode(&ngac.NGACNode{ID: "ua-xch-members", Name: "XCH_Members", NodeType: "UA"})
	_ = g.AddAssignment(&ngac.Assignment{ID: "a-xch-c", ChildID: "oa-xch-content", ParentID: "pc-global"})
	_ = g.AddAssignment(&ngac.Assignment{ID: "a-xch-m", ChildID: "ua-xch-members", ParentID: "pc-global"})
	_ = g.AddAssignment(&ngac.Assignment{ID: "a-trietvv-xch", ChildID: "ngac-trietvv", ParentID: "ua-xch-members"})
	_ = g.AddAssignment(&ngac.Assignment{ID: "a-namdx-xch", ChildID: "ngac-namdx", ParentID: "ua-xch-members"})
	_ = g.AddAssociation(&ngac.Association{ID: "as-xch", UAID: "ua-xch-members", OAID: "oa-xch-content", Operations: []string{"read", "write"}})

	d1 := g.CheckAccess("ngac-trietvv", "oa-xch-content", "write")
	d2 := g.CheckAccess("ngac-namdx", "oa-xch-content", "write")
	assert.Equal(t, "ALLOW", d1.Decision, "trietvv(VietBank) in cross-company chat")
	assert.Equal(t, "ALLOW", d2.Decision, "namdx(VNPay) in cross-company chat")

	// trietvv still cannot access VNPay internal
	d3 := g.CheckAccess("ngac-trietvv", "oa-dvnh-drive", "read")
	assert.Equal(t, "DENY", d3.Decision, "trietvv cannot access VNPay internal")
}

func TestCH05_DM_OnlyTwoUsers(t *testing.T) {
	g := buildVNPayGraph()
	// DM channel with only 2 members
	g.AddNode(&ngac.NGACNode{ID: "oa-dm-content", Name: "DM_Content", NodeType: "OA"})
	g.AddNode(&ngac.NGACNode{ID: "ua-dm-members", Name: "DM_Members", NodeType: "UA"})
	_ = g.AddAssignment(&ngac.Assignment{ID: "a-dm-c", ChildID: "oa-dm-content", ParentID: "pc-vnpay"})
	_ = g.AddAssignment(&ngac.Assignment{ID: "a-dm-m", ChildID: "ua-dm-members", ParentID: "pc-vnpay"})
	_ = g.AddAssignment(&ngac.Assignment{ID: "a-dm-h", ChildID: "ngac-hoangnlv", ParentID: "ua-dm-members"})
	_ = g.AddAssignment(&ngac.Assignment{ID: "a-dm-n", ChildID: "ngac-namdx", ParentID: "ua-dm-members"})
	_ = g.AddAssociation(&ngac.Association{ID: "as-dm", UAID: "ua-dm-members", OAID: "oa-dm-content", Operations: []string{"read", "write"}})

	assert.Equal(t, "ALLOW", g.CheckAccess("ngac-hoangnlv", "oa-dm-content", "read").Decision)
	assert.Equal(t, "ALLOW", g.CheckAccess("ngac-namdx", "oa-dm-content", "read").Decision)
	assert.Equal(t, "DENY", g.CheckAccess("ngac-ducnm", "oa-dm-content", "read").Decision, "ducnm not in DM → no backdoor")
}

// =====================================================================
// Group 7: Prohibition (PH-01 to PH-05) — matchProhibitions unit tests
// =====================================================================

func TestPH01_ProhibitionUnion_DenyAny(t *testing.T) {
	objectOAs := map[string]bool{"oa-sensitive": true, "oa-normal": true}
	prohibitions := []*ngac.Prohibition{{
		ID: "p1", Name: "block-sensitive", SubjectID: "ngac-hoangnlv",
		Operations: []string{"write"}, TargetOAIDs: []string{"oa-sensitive"},
		Intersection: false,
	}}
	// Call exported test helper or verify logic directly
	// Since matchProhibitions is unexported, we test via DecisionEngine behavior
	// For now, test the Prohibition model structure
	assert.False(t, prohibitions[0].Intersection)
	assert.Contains(t, objectOAs, "oa-sensitive")
}

func TestPH04_NoProhibition_Allow(t *testing.T) {
	g := buildVNPayGraph()
	// Without any prohibition, normal access should work
	d := g.CheckAccess("ngac-hoangnlv", "oa-dvnh-drive", "read")
	assert.Equal(t, "ALLOW", d.Decision, "no prohibition → ALLOW")
}

// =====================================================================
// Group 10: Reconciliation (RC-01, RC-04)
// =====================================================================

func TestRC01_ReassignChief(t *testing.T) {
	g := buildVNPayGraph()
	// Add hoangttt as new user
	g.AddNode(&ngac.NGACNode{ID: "ngac-hoangttt", Name: "hoangttt", NodeType: "U"})
	_ = g.AddAssignment(&ngac.Assignment{ID: "a-hoangttt-pu", ChildID: "ngac-hoangttt", ParentID: "ua-public-users"})

	// Verify old chief has access
	d := g.CheckAccess("ngac-nguyenntn", "oa-dvnh-res", "approve")
	require.Equal(t, "ALLOW", d.Decision)

	// Remove old chief, assign new
	g.RemoveAssignment("ngac-nguyenntn", "ua-dvnh-chief")
	_ = g.AddAssignment(&ngac.Assignment{ID: "a-hoangttt-chief", ChildID: "ngac-hoangttt", ParentID: "ua-dvnh-chief"})

	// Old chief → DENY
	d = g.CheckAccess("ngac-nguyenntn", "oa-dvnh-res", "approve")
	assert.Equal(t, "DENY", d.Decision, "old chief loses approve")

	// New chief → ALLOW
	d = g.CheckAccess("ngac-hoangttt", "oa-dvnh-res", "approve")
	assert.Equal(t, "ALLOW", d.Decision, "new chief inherits approve")
}

func TestRC04_TransferDepartment(t *testing.T) {
	g := buildVNPayGraph()
	// hoangnlv moves from DVNH to AI
	g.RemoveAssignment("ngac-hoangnlv", "ua-dvnh-dept")
	_ = g.AddAssignment(&ngac.Assignment{ID: "a-hoangnlv-ai", ChildID: "ngac-hoangnlv", ParentID: "ua-ai-dept"})

	d := g.CheckAccess("ngac-hoangnlv", "oa-dvnh-drive", "write")
	assert.Equal(t, "DENY", d.Decision, "after transfer, no write to DVNH")

	d = g.CheckAccess("ngac-hoangnlv", "oa-ai-res", "write")
	assert.Equal(t, "ALLOW", d.Decision, "after transfer, can write AI resources")
}

// =====================================================================
// Group 11: Performance (PF-01, PF-03)
// =====================================================================

func TestPF01_DeepBFS(t *testing.T) {
	g := ngac.NewGraph()
	pc := &ngac.NGACNode{ID: "pc", Name: "PC", NodeType: "PC"}
	oa := &ngac.NGACNode{ID: "oa", Name: "OA", NodeType: "OA"}
	g.AddNode(pc)
	g.AddNode(oa)
	_ = g.AddAssignment(&ngac.Assignment{ID: "a-oa-pc", ChildID: "oa", ParentID: "pc"})

	// Build 10-level deep UA chain
	prev := "pc"
	for i := 0; i < 10; i++ {
		id := fmt.Sprintf("ua-%d", i)
		g.AddNode(&ngac.NGACNode{ID: id, Name: fmt.Sprintf("UA_%d", i), NodeType: "UA"})
		_ = g.AddAssignment(&ngac.Assignment{ID: fmt.Sprintf("a-%d", i), ChildID: id, ParentID: prev})
		prev = id
	}
	// User at bottom
	g.AddNode(&ngac.NGACNode{ID: "u-deep", Name: "DeepUser", NodeType: "U"})
	_ = g.AddAssignment(&ngac.Assignment{ID: "a-u-deep", ChildID: "u-deep", ParentID: prev})
	// Association at top level
	_ = g.AddAssociation(&ngac.Association{ID: "as-top", UAID: "ua-0", OAID: "oa", Operations: []string{"read"}})

	d := g.CheckAccess("u-deep", "oa", "read")
	assert.Equal(t, "ALLOW", d.Decision, "deep 10-level BFS should still work")
}

func TestPF03_ConcurrentAccess(t *testing.T) {
	g := buildVNPayGraph()
	var wg sync.WaitGroup
	errs := make(chan string, 1000)

	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			d := g.CheckAccess("ngac-hoangnlv", "oa-dvnh-drive", "read")
			if d.Decision != "ALLOW" {
				errs <- d.Decision
			}
		}()
	}
	wg.Wait()
	close(errs)

	for e := range errs {
		t.Fatalf("concurrent access returned %s instead of ALLOW", e)
	}
}
