package grpc

import (
	"testing"

	"ngac-platform/services/policy/internal/ngac"
)

// buildRegionalHierarchy creates:
//
//	PC1 (Policy Class)
//	├── HQ_OA (Hội sở)
//	│   ├── MienNam_OA (Miền Nam)
//	│   │   ├── KeToan_MN_OA (Kế Toán MN)
//	│   │   └── KinhDoanh_MN_OA (Kinh Doanh MN)
//	│   └── MienBac_OA (Miền Bắc)
//	│       └── KeToan_MB_OA (Kế Toán MB)
//	├── CEO_UA → assoc to HQ_OA [read, approve]
//	├── ManagerMN_UA → assoc to MienNam_OA [read, approve]
//	├── ManagerMB_UA → assoc to MienBac_OA [read, approve]
//	├── NV_KeToan_MN_UA → assoc to KeToan_MN_OA [read]
//	├── user_ceo (U) → CEO_UA
//	├── user_mgr_mn (U) → ManagerMN_UA
//	├── user_mgr_mb (U) → ManagerMB_UA
//	└── user_nv_mn (U) → NV_KeToan_MN_UA
func buildRegionalHierarchy() *ngac.Graph {
	g := ngac.NewGraph()

	// Nodes
	nodes := []*ngac.NGACNode{
		{ID: "pc1", Name: "PC1", NodeType: "PC"},
		{ID: "hq_oa", Name: "HQ", NodeType: "OA"},
		{ID: "mn_oa", Name: "MienNam", NodeType: "OA"},
		{ID: "mb_oa", Name: "MienBac", NodeType: "OA"},
		{ID: "kt_mn_oa", Name: "KeToan_MN", NodeType: "OA"},
		{ID: "kd_mn_oa", Name: "KinhDoanh_MN", NodeType: "OA"},
		{ID: "kt_mb_oa", Name: "KeToan_MB", NodeType: "OA"},
		{ID: "ceo_ua", Name: "CEO", NodeType: "UA"},
		{ID: "mgr_mn_ua", Name: "Manager_MN", NodeType: "UA"},
		{ID: "mgr_mb_ua", Name: "Manager_MB", NodeType: "UA"},
		{ID: "nv_kt_mn_ua", Name: "NV_KeToan_MN", NodeType: "UA"},
		{ID: "user_ceo", Name: "CEO User", NodeType: "U"},
		{ID: "user_mgr_mn", Name: "Manager MN User", NodeType: "U"},
		{ID: "user_mgr_mb", Name: "Manager MB User", NodeType: "U"},
		{ID: "user_nv_mn", Name: "NV KeToan MN User", NodeType: "U"},
	}
	for _, n := range nodes {
		g.AddNode(n)
	}

	// OA hierarchy: HQ → MienNam → KeToan_MN, KinhDoanh_MN
	//               HQ → MienBac → KeToan_MB
	assignments := []*ngac.Assignment{
		{ID: "a1", ChildID: "hq_oa", ParentID: "pc1"},
		{ID: "a2", ChildID: "mn_oa", ParentID: "hq_oa"},
		{ID: "a3", ChildID: "mb_oa", ParentID: "hq_oa"},
		{ID: "a4", ChildID: "kt_mn_oa", ParentID: "mn_oa"},
		{ID: "a5", ChildID: "kd_mn_oa", ParentID: "mn_oa"},
		{ID: "a6", ChildID: "kt_mb_oa", ParentID: "mb_oa"},
		// UA hierarchy
		{ID: "a7", ChildID: "ceo_ua", ParentID: "pc1"},
		{ID: "a8", ChildID: "mgr_mn_ua", ParentID: "pc1"},
		{ID: "a9", ChildID: "mgr_mb_ua", ParentID: "pc1"},
		{ID: "a10", ChildID: "nv_kt_mn_ua", ParentID: "pc1"},
		// User → UA
		{ID: "a11", ChildID: "user_ceo", ParentID: "ceo_ua"},
		{ID: "a12", ChildID: "user_mgr_mn", ParentID: "mgr_mn_ua"},
		{ID: "a13", ChildID: "user_mgr_mb", ParentID: "mgr_mb_ua"},
		{ID: "a14", ChildID: "user_nv_mn", ParentID: "nv_kt_mn_ua"},
	}
	for _, a := range assignments {
		g.AddAssignment(a)
	}

	// Associations: who has access to which OAs
	associations := []*ngac.Association{
		{ID: "assoc1", UAID: "ceo_ua", OAID: "hq_oa", Operations: []string{"read", "approve"}},
		{ID: "assoc2", UAID: "mgr_mn_ua", OAID: "mn_oa", Operations: []string{"read", "approve"}},
		{ID: "assoc3", UAID: "mgr_mb_ua", OAID: "mb_oa", Operations: []string{"read", "approve"}},
		{ID: "assoc4", UAID: "nv_kt_mn_ua", OAID: "kt_mn_oa", Operations: []string{"read"}},
	}
	for _, a := range associations {
		g.AddAssociation(a)
	}

	return g
}

func TestCollectUAAncestors(t *testing.T) {
	g := buildRegionalHierarchy()
	rs := &ReadServer{}

	tests := []struct {
		name       string
		userNodeID string
		wantUAs    []string
	}{
		{
			name:       "CEO user has CEO_UA ancestor",
			userNodeID: "user_ceo",
			wantUAs:    []string{"ceo_ua"},
		},
		{
			name:       "Manager MN user has ManagerMN_UA ancestor",
			userNodeID: "user_mgr_mn",
			wantUAs:    []string{"mgr_mn_ua"},
		},
		{
			name:       "NV KeToan MN has NV_KeToan_MN_UA ancestor",
			userNodeID: "user_nv_mn",
			wantUAs:    []string{"nv_kt_mn_ua"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			userNode := g.GetNode(tc.userNodeID)
			if userNode == nil {
				t.Fatalf("user node %s not found", tc.userNodeID)
			}
			uaIDs := rs.collectUAAncestors(g, tc.userNodeID, userNode)
			if len(uaIDs) != len(tc.wantUAs) {
				t.Errorf("got %d UA ancestors, want %d: %v", len(uaIDs), len(tc.wantUAs), uaIDs)
				return
			}
			for _, want := range tc.wantUAs {
				found := false
				for _, got := range uaIDs {
					if got == want {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected UA %q not found in %v", want, uaIDs)
				}
			}
		})
	}
}

func TestCollectTargetOAs(t *testing.T) {
	g := buildRegionalHierarchy()
	rs := &ReadServer{}

	tests := []struct {
		name      string
		uaIDs     []string
		operation string
		wantOAs   []string
	}{
		{
			name:      "CEO with read sees HQ",
			uaIDs:     []string{"ceo_ua"},
			operation: "read",
			wantOAs:   []string{"hq_oa"},
		},
		{
			name:      "Manager MN with approve sees MienNam",
			uaIDs:     []string{"mgr_mn_ua"},
			operation: "approve",
			wantOAs:   []string{"mn_oa"},
		},
		{
			name:      "NV KeToan MN with read sees KeToan_MN",
			uaIDs:     []string{"nv_kt_mn_ua"},
			operation: "read",
			wantOAs:   []string{"kt_mn_oa"},
		},
		{
			name:      "NV KeToan MN with approve sees nothing (no approve assoc)",
			uaIDs:     []string{"nv_kt_mn_ua"},
			operation: "approve",
			wantOAs:   []string{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			targetOAs := rs.collectTargetOAs(g, tc.uaIDs, tc.operation)
			if len(targetOAs) != len(tc.wantOAs) {
				t.Errorf("got %d target OAs, want %d: %v", len(targetOAs), len(tc.wantOAs), keys(targetOAs))
				return
			}
			for _, want := range tc.wantOAs {
				if !targetOAs[want] {
					t.Errorf("expected OA %q not found in %v", want, keys(targetOAs))
				}
			}
		})
	}
}

func TestResolveLeafOAs_CEO(t *testing.T) {
	g := buildRegionalHierarchy()
	rs := &ReadServer{}

	// CEO has access to HQ_OA which has descendants:
	// MienNam_OA → KeToan_MN_OA, KinhDoanh_MN_OA
	// MienBac_OA → KeToan_MB_OA
	// Leaves: KeToan_MN_OA, KinhDoanh_MN_OA, KeToan_MB_OA
	targetOAs := map[string]bool{"hq_oa": true}
	leafOAs := rs.resolveLeafOAs(g, targetOAs)

	expected := map[string]bool{
		"kt_mn_oa": true,
		"kd_mn_oa": true,
		"kt_mb_oa": true,
	}

	if len(leafOAs) != len(expected) {
		t.Fatalf("CEO got %d leaf OAs, want %d: %v", len(leafOAs), len(expected), leafOAs)
	}
	for _, id := range leafOAs {
		if !expected[id] {
			t.Errorf("unexpected leaf OA: %s", id)
		}
	}
}

func TestResolveLeafOAs_ManagerMN(t *testing.T) {
	g := buildRegionalHierarchy()
	rs := &ReadServer{}

	// Manager MN has access to MienNam_OA → KeToan_MN_OA, KinhDoanh_MN_OA
	targetOAs := map[string]bool{"mn_oa": true}
	leafOAs := rs.resolveLeafOAs(g, targetOAs)

	expected := map[string]bool{
		"kt_mn_oa": true,
		"kd_mn_oa": true,
	}

	if len(leafOAs) != len(expected) {
		t.Fatalf("ManagerMN got %d leaf OAs, want %d: %v", len(leafOAs), len(expected), leafOAs)
	}
	for _, id := range leafOAs {
		if !expected[id] {
			t.Errorf("unexpected leaf OA: %s", id)
		}
	}
}

func TestResolveLeafOAs_NV(t *testing.T) {
	g := buildRegionalHierarchy()
	rs := &ReadServer{}

	// NV KeToan MN has access to KeToan_MN_OA (already a leaf, no OA children)
	targetOAs := map[string]bool{"kt_mn_oa": true}
	leafOAs := rs.resolveLeafOAs(g, targetOAs)

	if len(leafOAs) != 1 {
		t.Fatalf("NV got %d leaf OAs, want 1: %v", len(leafOAs), leafOAs)
	}
	if leafOAs[0] != "kt_mn_oa" {
		t.Errorf("got %s, want kt_mn_oa", leafOAs[0])
	}
}

func TestResolveLeafOAs_ManagerMB(t *testing.T) {
	g := buildRegionalHierarchy()
	rs := &ReadServer{}

	// Manager MB has access to MienBac_OA → KeToan_MB_OA
	targetOAs := map[string]bool{"mb_oa": true}
	leafOAs := rs.resolveLeafOAs(g, targetOAs)

	if len(leafOAs) != 1 {
		t.Fatalf("ManagerMB got %d leaf OAs, want 1: %v", len(leafOAs), leafOAs)
	}
	if leafOAs[0] != "kt_mb_oa" {
		t.Errorf("got %s, want kt_mb_oa", leafOAs[0])
	}
}

func TestFullScopeResolution_EndToEnd(t *testing.T) {
	g := buildRegionalHierarchy()
	rs := &ReadServer{}

	// Simulate full flow: user → UA ancestors → target OAs → leaf OAs
	tests := []struct {
		name       string
		userNodeID string
		operation  string
		wantLeaves []string
	}{
		{
			name:       "CEO read: sees all 3 leaf departments",
			userNodeID: "user_ceo",
			operation:  "read",
			wantLeaves: []string{"kt_mn_oa", "kd_mn_oa", "kt_mb_oa"},
		},
		{
			name:       "CEO approve: sees all 3 leaf departments",
			userNodeID: "user_ceo",
			operation:  "approve",
			wantLeaves: []string{"kt_mn_oa", "kd_mn_oa", "kt_mb_oa"},
		},
		{
			name:       "Manager MN read: sees 2 MN departments",
			userNodeID: "user_mgr_mn",
			operation:  "read",
			wantLeaves: []string{"kt_mn_oa", "kd_mn_oa"},
		},
		{
			name:       "Manager MB read: sees 1 MB department",
			userNodeID: "user_mgr_mb",
			operation:  "read",
			wantLeaves: []string{"kt_mb_oa"},
		},
		{
			name:       "NV KeToan MN read: sees only own department",
			userNodeID: "user_nv_mn",
			operation:  "read",
			wantLeaves: []string{"kt_mn_oa"},
		},
		{
			name:       "NV KeToan MN approve: sees nothing (no approve access)",
			userNodeID: "user_nv_mn",
			operation:  "approve",
			wantLeaves: []string{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			userNode := g.GetNode(tc.userNodeID)
			if userNode == nil {
				t.Fatalf("user node %s not found", tc.userNodeID)
			}

			uaIDs := rs.collectUAAncestors(g, tc.userNodeID, userNode)
			targetOAs := rs.collectTargetOAs(g, uaIDs, tc.operation)
			leafOAs := rs.resolveLeafOAs(g, targetOAs)

			if len(leafOAs) != len(tc.wantLeaves) {
				t.Fatalf("got %d leaves, want %d: %v", len(leafOAs), len(tc.wantLeaves), leafOAs)
			}

			leafSet := make(map[string]bool, len(leafOAs))
			for _, id := range leafOAs {
				leafSet[id] = true
			}
			for _, want := range tc.wantLeaves {
				if !leafSet[want] {
					t.Errorf("expected leaf %q not found in %v", want, leafOAs)
				}
			}
		})
	}
}

// keys extracts map keys as a slice for error messages.
func keys(m map[string]bool) []string {
	result := make([]string, 0, len(m))
	for k := range m {
		result = append(result, k)
	}
	return result
}
