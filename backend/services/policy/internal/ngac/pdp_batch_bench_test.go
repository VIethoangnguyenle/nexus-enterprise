package ngac_test

import (
	"fmt"
	"testing"

	"ngac-platform/services/policy/internal/ngac"
)

// buildDeepUserGraph builds a graph where the *user* side is deep: the user sits
// under a chain of nested user attributes, as happens when a person belongs to a
// tenant UA under a workspace Members UA under a department hierarchy.
//
// It is the user side that a batch check repeats: one drive folder listing asks
// about many objects for a single user, so every redundant walk up the user's
// attribute chain is multiplied by the page size.
func buildDeepUserGraph(uaDepth, objectCount int) (*ngac.Graph, []string) {
	g := ngac.NewGraph()

	g.AddNode(&ngac.NGACNode{ID: "pc", Name: "PC", NodeType: "PC"})

	// Chain of nested UAs: ua0 -> ua1 -> ... -> uaN -> pc
	prev := "pc"
	for i := uaDepth - 1; i >= 0; i-- {
		id := fmt.Sprintf("ua%d", i)
		g.AddNode(&ngac.NGACNode{ID: id, Name: id, NodeType: "UA"})
		_ = g.AddAssignment(&ngac.Assignment{ID: "a-" + id, ChildID: id, ParentID: prev})
		prev = id
	}

	g.AddNode(&ngac.NGACNode{ID: "u", Name: "User", NodeType: "U"})
	_ = g.AddAssignment(&ngac.Assignment{ID: "a-u", ChildID: "u", ParentID: "ua0"})

	// A shared parent OA that the association targets, with many child OAs
	// hanging off it — the folder-of-many-files shape.
	g.AddNode(&ngac.NGACNode{ID: "root-oa", Name: "RootOA", NodeType: "OA"})
	_ = g.AddAssignment(&ngac.Assignment{ID: "a-root", ChildID: "root-oa", ParentID: "pc"})
	_ = g.AddAssociation(&ngac.Association{
		ID: "assoc", UAID: fmt.Sprintf("ua%d", uaDepth-1), OAID: "root-oa",
		Operations: []string{"read", "write"},
	})

	objects := make([]string, 0, objectCount)
	for i := range objectCount {
		id := fmt.Sprintf("oa%d", i)
		g.AddNode(&ngac.NGACNode{ID: id, Name: id, NodeType: "OA"})
		_ = g.AddAssignment(&ngac.Assignment{ID: "a-" + id, ChildID: id, ParentID: "root-oa"})
		objects = append(objects, id)
	}
	return g, objects
}

// Per-object CheckAccess — what BatchCheckAccess does today.
func BenchmarkBatch_PerObjectCheckAccess(b *testing.B) {
	g, objects := buildDeepUserGraph(8, 200)
	b.ResetTimer()
	for range b.N {
		for _, obj := range objects {
			if d := g.CheckAccess("u", obj, "read"); d.Decision != "ALLOW" {
				b.Fatalf("expected ALLOW, got %s", d.Decision)
			}
		}
	}
}

// One resolved user side, reused across the page.
func BenchmarkBatch_SharedUserSide(b *testing.B) {
	g, objects := buildDeepUserGraph(8, 200)
	b.ResetTimer()
	for range b.N {
		results := g.CheckAccessBatch("u", objects, []string{"read"})
		if len(results) != len(objects) {
			b.Fatalf("expected %d results, got %d", len(objects), len(results))
		}
	}
}
