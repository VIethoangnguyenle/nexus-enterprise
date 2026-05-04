package grpc

import (
	"testing"

	"ngac-platform/services/policy/internal/ngac"
)

// buildLargeHierarchy creates a graph with many OA nodes to benchmark scope resolution.
//
//	PC1
//	├── Root_OA
//	│   ├── Region_1_OA → Dept_1_1_OA, Dept_1_2_OA, ..., Dept_1_N_OA
//	│   ├── Region_2_OA → Dept_2_1_OA, ..., Dept_2_N_OA
//	│   └── ...
//	├── Admin_UA → assoc to Root_OA [read, approve]
//	├── RegMgr_1_UA → assoc to Region_1_OA [read, approve]
//	└── user_admin (U) → Admin_UA
//	    user_mgr_1 (U) → RegMgr_1_UA
func buildLargeHierarchy(regions, deptsPerRegion int) *ngac.Graph {
	g := ngac.NewGraph()

	g.AddNode(&ngac.NGACNode{ID: "pc1", Name: "PC1", NodeType: "PC"})
	g.AddNode(&ngac.NGACNode{ID: "root_oa", Name: "Root", NodeType: "OA"})
	g.AddAssignment(&ngac.Assignment{ID: "a_root", ChildID: "root_oa", ParentID: "pc1"})

	// Admin
	g.AddNode(&ngac.NGACNode{ID: "admin_ua", Name: "Admin", NodeType: "UA"})
	g.AddAssignment(&ngac.Assignment{ID: "a_admin_ua", ChildID: "admin_ua", ParentID: "pc1"})
	g.AddNode(&ngac.NGACNode{ID: "user_admin", Name: "Admin User", NodeType: "U"})
	g.AddAssignment(&ngac.Assignment{ID: "a_user_admin", ChildID: "user_admin", ParentID: "admin_ua"})
	g.AddAssociation(&ngac.Association{ID: "assoc_admin", UAID: "admin_ua", OAID: "root_oa", Operations: []string{"read", "approve"}})

	for r := 1; r <= regions; r++ {
		regionID := regionOAID(r)
		g.AddNode(&ngac.NGACNode{ID: regionID, Name: regionID, NodeType: "OA"})
		g.AddAssignment(&ngac.Assignment{ID: "a_" + regionID, ChildID: regionID, ParentID: "root_oa"})

		// Region manager
		mgrUA := regionMgrUAID(r)
		mgrUser := regionMgrUserID(r)
		g.AddNode(&ngac.NGACNode{ID: mgrUA, Name: mgrUA, NodeType: "UA"})
		g.AddAssignment(&ngac.Assignment{ID: "a_" + mgrUA, ChildID: mgrUA, ParentID: "pc1"})
		g.AddNode(&ngac.NGACNode{ID: mgrUser, Name: mgrUser, NodeType: "U"})
		g.AddAssignment(&ngac.Assignment{ID: "a_" + mgrUser, ChildID: mgrUser, ParentID: mgrUA})
		g.AddAssociation(&ngac.Association{ID: "assoc_" + mgrUA, UAID: mgrUA, OAID: regionID, Operations: []string{"read", "approve"}})

		for d := 1; d <= deptsPerRegion; d++ {
			deptID := deptOAID(r, d)
			g.AddNode(&ngac.NGACNode{ID: deptID, Name: deptID, NodeType: "OA"})
			g.AddAssignment(&ngac.Assignment{ID: "a_" + deptID, ChildID: deptID, ParentID: regionID})
		}
	}

	return g
}

func regionOAID(r int) string    { return "region_" + itoa(r) + "_oa" }
func regionMgrUAID(r int) string { return "mgr_" + itoa(r) + "_ua" }
func regionMgrUserID(r int) string { return "user_mgr_" + itoa(r) }
func deptOAID(r, d int) string   { return "dept_" + itoa(r) + "_" + itoa(d) + "_oa" }

func itoa(n int) string {
	if n < 10 {
		return string(rune('0' + n))
	}
	return string(rune('0'+n/10)) + string(rune('0'+n%10))
}

// BenchmarkScopeResolution_AdminSmall: 5 regions × 10 departments = 50 leaf OAs
func BenchmarkScopeResolution_AdminSmall(b *testing.B) {
	g := buildLargeHierarchy(5, 10)
	rs := &ReadServer{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		userNode := g.GetNode("user_admin")
		uaIDs := rs.collectUAAncestors(g, "user_admin", userNode)
		targetOAs := rs.collectTargetOAs(g, uaIDs, "read")
		rs.resolveLeafOAs(g, targetOAs)
	}
}

// BenchmarkScopeResolution_AdminLarge: 20 regions × 50 departments = 1000 leaf OAs
func BenchmarkScopeResolution_AdminLarge(b *testing.B) {
	g := buildLargeHierarchy(20, 50)
	rs := &ReadServer{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		userNode := g.GetNode("user_admin")
		uaIDs := rs.collectUAAncestors(g, "user_admin", userNode)
		targetOAs := rs.collectTargetOAs(g, uaIDs, "read")
		rs.resolveLeafOAs(g, targetOAs)
	}
}

// BenchmarkScopeResolution_RegionalMgr: single region, 50 depts
func BenchmarkScopeResolution_RegionalMgr(b *testing.B) {
	g := buildLargeHierarchy(20, 50)
	rs := &ReadServer{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		userNode := g.GetNode("user_mgr_1")
		uaIDs := rs.collectUAAncestors(g, "user_mgr_1", userNode)
		targetOAs := rs.collectTargetOAs(g, uaIDs, "read")
		rs.resolveLeafOAs(g, targetOAs)
	}
}
