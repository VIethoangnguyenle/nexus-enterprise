package ngac_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/services/policy/internal/ngac"
)

// buildVNPayGraph creates the VNPay org graph from ngac-practical-guide.md Section 4.
func buildVNPayGraph() *ngac.Graph {
	g := ngac.NewGraph()

	// --- Policy Classes ---
	nodes := []*ngac.NGACNode{
		{ID: "pc-vnpay", Name: "PC_VNPay", NodeType: "PC"},
		{ID: "pc-global", Name: "PC_Global", NodeType: "PC"},

		// --- Workspace-level UA/OA ---
		{ID: "ua-owners", Name: "VNPay_Owners", NodeType: "UA"},
		{ID: "ua-members", Name: "VNPay_Members", NodeType: "UA"},
		{ID: "oa-docs", Name: "VNPay_Docs", NodeType: "OA"},
		{ID: "oa-channels", Name: "VNPay_Channels", NodeType: "OA"},
		{ID: "oa-drive-root", Name: "VNPay_DriveRoot", NodeType: "OA"},

		// --- Regions ---
		{ID: "ua-mn", Name: "MienNam_Region", NodeType: "UA"},
		{ID: "ua-mb", Name: "MienBac_Region", NodeType: "UA"},
		{ID: "oa-mn-res", Name: "MienNam_Resources", NodeType: "OA"},
		{ID: "oa-mb-res", Name: "MienBac_Resources", NodeType: "OA"},

		// --- DVNH Dept (Mien Nam) ---
		{ID: "ua-dvnh-dept", Name: "DVNH_Dept", NodeType: "UA"},
		{ID: "ua-dvnh-chief", Name: "DVNH_Chief", NodeType: "UA"},
		{ID: "oa-dvnh-res", Name: "DVNH_Resources", NodeType: "OA"},
		{ID: "oa-dvnh-drive", Name: "DVNH_Drive", NodeType: "OA"},
		{ID: "oa-dvnh-approval", Name: "DVNH_ApprovalScope", NodeType: "OA"},

		// --- Teams under DVNH ---
		{ID: "ua-appsrv1", Name: "AppSrv1_Team", NodeType: "UA"},
		{ID: "ua-appsrv1-lead", Name: "AppSrv1_Lead", NodeType: "UA"},

		// --- AI Dept ---
		{ID: "ua-ai-dept", Name: "AI_Dept", NodeType: "UA"},
		{ID: "oa-ai-res", Name: "AI_Resources", NodeType: "OA"},

		// --- Noi Vu Dept ---
		{ID: "ua-noivu-dept", Name: "NoiVu_Dept", NodeType: "UA"},

		// --- Channel DVNH ---
		{ID: "oa-ch-dvnh-content", Name: "Ch_dvnh_Content", NodeType: "OA"},
		{ID: "ua-ch-dvnh-members", Name: "Ch_dvnh_Members", NodeType: "UA"},

		// --- Global nodes ---
		{ID: "ua-public-users", Name: "PublicUsers", NodeType: "UA"},
		{ID: "oa-public-docs", Name: "PublicDocs", NodeType: "OA"},

		// --- Users ---
		{ID: "ngac-hoangnlv", Name: "hoangnlv", NodeType: "U"},
		{ID: "ngac-namdx", Name: "namdx", NodeType: "U"},
		{ID: "ngac-nguyenntn", Name: "nguyenntn", NodeType: "U"},
		{ID: "ngac-ducnm", Name: "ducnm", NodeType: "U"},
		{ID: "ngac-thuynt", Name: "thuynt", NodeType: "U"},
		{ID: "ngac-hoangbm", Name: "hoangbm", NodeType: "U"},
		{ID: "ngac-trietvv", Name: "trietvv", NodeType: "U"},
	}
	for _, n := range nodes {
		g.AddNode(n)
	}

	// --- Assignments (containment) ---
	assignments := []*ngac.Assignment{
		// Workspace → PC
		{ID: "a-owners-pc", ChildID: "ua-owners", ParentID: "pc-vnpay"},
		{ID: "a-members-pc", ChildID: "ua-members", ParentID: "pc-vnpay"},
		{ID: "a-docs-pc", ChildID: "oa-docs", ParentID: "pc-vnpay"},
		{ID: "a-chs-pc", ChildID: "oa-channels", ParentID: "pc-vnpay"},
		{ID: "a-drive-pc", ChildID: "oa-drive-root", ParentID: "pc-vnpay"},
		// Regions → Members
		{ID: "a-mn-members", ChildID: "ua-mn", ParentID: "ua-members"},
		{ID: "a-mb-members", ChildID: "ua-mb", ParentID: "ua-members"},
		{ID: "a-mn-res-docs", ChildID: "oa-mn-res", ParentID: "oa-docs"},
		{ID: "a-mb-res-docs", ChildID: "oa-mb-res", ParentID: "oa-docs"},
		// DVNH → MienNam
		{ID: "a-dvnh-mn", ChildID: "ua-dvnh-dept", ParentID: "ua-mn"},
		{ID: "a-dvnh-chief-dept", ChildID: "ua-dvnh-chief", ParentID: "ua-dvnh-dept"},
		{ID: "a-dvnh-res-mn", ChildID: "oa-dvnh-res", ParentID: "oa-mn-res"},
		{ID: "a-dvnh-drive-res", ChildID: "oa-dvnh-drive", ParentID: "oa-dvnh-res"},
		{ID: "a-dvnh-appr-res", ChildID: "oa-dvnh-approval", ParentID: "oa-dvnh-res"},
		// Teams → DVNH
		{ID: "a-appsrv1-dvnh", ChildID: "ua-appsrv1", ParentID: "ua-dvnh-dept"},
		{ID: "a-appsrv1lead-t1", ChildID: "ua-appsrv1-lead", ParentID: "ua-appsrv1"},
		// AI → MienNam
		{ID: "a-ai-mn", ChildID: "ua-ai-dept", ParentID: "ua-mn"},
		{ID: "a-ai-res-mn", ChildID: "oa-ai-res", ParentID: "oa-mn-res"},
		// NoiVu → MienNam
		{ID: "a-noivu-mn", ChildID: "ua-noivu-dept", ParentID: "ua-mn"},
		// Channel DVNH → PC
		{ID: "a-ch-dvnh-c-pc", ChildID: "oa-ch-dvnh-content", ParentID: "pc-vnpay"},
		{ID: "a-ch-dvnh-m-pc", ChildID: "ua-ch-dvnh-members", ParentID: "pc-vnpay"},
		// Global
		{ID: "a-pu-pcg", ChildID: "ua-public-users", ParentID: "pc-global"},
		{ID: "a-pd-pcg", ChildID: "oa-public-docs", ParentID: "pc-global"},
		// Users → UAs
		{ID: "a-hoangnlv-dvnh", ChildID: "ngac-hoangnlv", ParentID: "ua-dvnh-dept"},
		{ID: "a-namdx-lead", ChildID: "ngac-namdx", ParentID: "ua-appsrv1-lead"},
		{ID: "a-nguyenntn-chief", ChildID: "ngac-nguyenntn", ParentID: "ua-dvnh-chief"},
		{ID: "a-ducnm-ai", ChildID: "ngac-ducnm", ParentID: "ua-ai-dept"},
		{ID: "a-thuynt-noivu", ChildID: "ngac-thuynt", ParentID: "ua-noivu-dept"},
		{ID: "a-hoangbm-appsrv1", ChildID: "ngac-hoangbm", ParentID: "ua-appsrv1"},
		// Channel members
		{ID: "a-hoangnlv-ch", ChildID: "ngac-hoangnlv", ParentID: "ua-ch-dvnh-members"},
		{ID: "a-namdx-ch", ChildID: "ngac-namdx", ParentID: "ua-ch-dvnh-members"},
		// All users → PublicUsers
		{ID: "a-hoangnlv-pu", ChildID: "ngac-hoangnlv", ParentID: "ua-public-users"},
		{ID: "a-trietvv-pu", ChildID: "ngac-trietvv", ParentID: "ua-public-users"},
		{ID: "a-ducnm-pu", ChildID: "ngac-ducnm", ParentID: "ua-public-users"},
	}
	for _, a := range assignments {
		_ = g.AddAssignment(a)
	}

	// --- Associations (permissions) ---
	associations := []*ngac.Association{
		{ID: "as-owners-docs", UAID: "ua-owners", OAID: "oa-docs", Operations: []string{"read", "write", "manage", "share"}},
		{ID: "as-members-docs", UAID: "ua-members", OAID: "oa-docs", Operations: []string{"read"}},
		{ID: "as-members-chs", UAID: "ua-members", OAID: "oa-channels", Operations: []string{"read", "write", "create_channel"}},
		{ID: "as-dvnh-res", UAID: "ua-dvnh-dept", OAID: "oa-dvnh-res", Operations: []string{"read", "write", "upload"}},
		{ID: "as-chief-res", UAID: "ua-dvnh-chief", OAID: "oa-dvnh-res", Operations: []string{"read", "write", "upload", "manage", "approve", "share"}},
		{ID: "as-lead-drive", UAID: "ua-appsrv1-lead", OAID: "oa-dvnh-drive", Operations: []string{"manage"}},
		{ID: "as-ai-res", UAID: "ua-ai-dept", OAID: "oa-ai-res", Operations: []string{"read", "write", "upload"}},
		{ID: "as-ch-members", UAID: "ua-ch-dvnh-members", OAID: "oa-ch-dvnh-content", Operations: []string{"read", "write"}},
		{ID: "as-pu-pd", UAID: "ua-public-users", OAID: "oa-public-docs", Operations: []string{"read"}},
	}
	for _, a := range associations {
		_ = g.AddAssociation(a)
	}

	return g
}

// =====================================================================
// Group 1: Graph Mechanics (GM-01 to GM-11)
// =====================================================================

func TestGM01_CreateValidNodeTypes(t *testing.T) {
	g := ngac.NewGraph()
	for _, nt := range []string{"U", "UA", "OA", "O", "PC"} {
		g.AddNode(&ngac.NGACNode{ID: "n-" + nt, Name: "Test_" + nt, NodeType: nt})
		assert.NotNil(t, g.GetNode("n-"+nt), "node type %s should exist", nt)
	}
}

func TestGM07_CycleDetection(t *testing.T) {
	g := ngac.NewGraph()
	g.AddNode(&ngac.NGACNode{ID: "ua-a", Name: "UA_A", NodeType: "UA"})
	g.AddNode(&ngac.NGACNode{ID: "ua-b", Name: "UA_B", NodeType: "UA"})
	require.NoError(t, g.AddAssignment(&ngac.Assignment{ID: "a1", ChildID: "ua-a", ParentID: "ua-b"}))
	err := g.AddAssignment(&ngac.Assignment{ID: "a2", ChildID: "ua-b", ParentID: "ua-a"})
	assert.Error(t, err, "should detect cycle")
}

func TestGM06_InvalidAssignmentType(t *testing.T) {
	g := ngac.NewGraph()
	g.AddNode(&ngac.NGACNode{ID: "u1", Name: "U1", NodeType: "U"})
	g.AddNode(&ngac.NGACNode{ID: "oa1", Name: "OA1", NodeType: "OA"})
	err := g.AddAssignment(&ngac.Assignment{ID: "bad", ChildID: "u1", ParentID: "oa1"})
	assert.Error(t, err, "U→OA assignment should be rejected")
}

func TestGM09_RemoveNodeCascade(t *testing.T) {
	g := ngac.NewGraph()
	g.AddNode(&ngac.NGACNode{ID: "pc", Name: "PC", NodeType: "PC"})
	g.AddNode(&ngac.NGACNode{ID: "ua", Name: "UA", NodeType: "UA"})
	g.AddNode(&ngac.NGACNode{ID: "oa", Name: "OA", NodeType: "OA"})
	_ = g.AddAssignment(&ngac.Assignment{ID: "a1", ChildID: "ua", ParentID: "pc"})
	_ = g.AddAssociation(&ngac.Association{ID: "as1", UAID: "ua", OAID: "oa", Operations: []string{"read"}})

	g.RemoveNode("ua")
	assert.Nil(t, g.GetNode("ua"))
	assert.False(t, g.IsAssigned("ua", "pc"))
	assert.Empty(t, g.GetAssociationsFromUA("ua"))
}

func TestGM10_FindNodeByNameO1(t *testing.T) {
	g := buildVNPayGraph()
	found := g.FindNodeByName("PC_VNPay", "PC")
	require.NotNil(t, found)
	assert.Equal(t, "pc-vnpay", found.ID)
}

func TestGM11_FindNodeByNameWrongType(t *testing.T) {
	g := buildVNPayGraph()
	assert.Nil(t, g.FindNodeByName("PC_VNPay", "UA"))
}

// =====================================================================
// Group 2: CheckAccess Core (CA-01 to CA-09)
// =====================================================================

func TestCA01_DirectAccess_Allow(t *testing.T) {
	g := buildVNPayGraph()
	d := g.CheckAccess("ngac-hoangnlv", "oa-dvnh-drive", "read")
	assert.Equal(t, "ALLOW", d.Decision, "hoangnlv(DVNH) should read DVNH_Drive")
}

func TestCA02_WrongOperation_Deny(t *testing.T) {
	g := buildVNPayGraph()
	d := g.CheckAccess("ngac-hoangnlv", "oa-dvnh-res", "approve")
	assert.Equal(t, "DENY", d.Decision, "hoangnlv has no approve permission")
}

func TestCA03_CrossDept_WriteDeny(t *testing.T) {
	g := buildVNPayGraph()
	// ducnm(AI) CAN read DVNH_Drive via VNPay_Members→VNPay_Docs inheritance
	// But ducnm CANNOT write — only DVNH_Dept has [write] on DVNH_Resources
	d := g.CheckAccess("ngac-ducnm", "oa-dvnh-drive", "write")
	assert.Equal(t, "DENY", d.Decision, "ducnm(AI) cannot write to DVNH_Drive")

	d = g.CheckAccess("ngac-ducnm", "oa-dvnh-drive", "manage")
	assert.Equal(t, "DENY", d.Decision, "ducnm(AI) cannot manage DVNH_Drive")
}

func TestCA04_NonexistentNode_Deny(t *testing.T) {
	g := buildVNPayGraph()
	d := g.CheckAccess("nonexistent", "oa-dvnh-drive", "read")
	assert.Equal(t, "DENY", d.Decision)
}

func TestCA05_DeepInheritance_Allow(t *testing.T) {
	g := buildVNPayGraph()
	d := g.CheckAccess("ngac-hoangnlv", "oa-docs", "read")
	assert.Equal(t, "ALLOW", d.Decision, "hoangnlv→DVNH→MN→Members should read VNPay_Docs")
}

func TestCA07_ChiefFullAccess(t *testing.T) {
	g := buildVNPayGraph()
	for _, op := range []string{"read", "write", "manage", "approve", "share"} {
		d := g.CheckAccess("ngac-nguyenntn", "oa-dvnh-res", op)
		assert.Equal(t, "ALLOW", d.Decision, "Chief nguyenntn should have %s", op)
	}
}

func TestCA08_TeamLeadNarrowScope(t *testing.T) {
	g := buildVNPayGraph()
	d := g.CheckAccess("ngac-namdx", "oa-dvnh-drive", "manage")
	assert.Equal(t, "ALLOW", d.Decision, "namdx(Lead) should manage DVNH_Drive")

	d = g.CheckAccess("ngac-namdx", "oa-dvnh-approval", "approve")
	assert.Equal(t, "DENY", d.Decision, "namdx(Lead) should NOT approve on ApprovalScope")
}
