package ngac_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"ngac-platform/services/policy/internal/ngac"
)

// The batch path is an evaluation-order optimisation. The only thing that makes
// it safe is that it agrees with the single-object path on every case — most of
// all on the denials, since a batch that is merely *faster* at saying ALLOW
// would be a silent authorization widening across every list view in the app.
//
// This asserts agreement object-by-object over a graph that contains all the
// shapes the PDP distinguishes: an allowed path, an unrelated object, an object
// in a different policy class, an object with no PC at all, and IDs that are not
// in the graph.
func TestCheckAccessBatch_AgreesWithSingleObjectPath(t *testing.T) {
	g := ngac.NewGraph()

	for _, n := range []*ngac.NGACNode{
		{ID: "pcA", Name: "PC_A", NodeType: "PC"},
		{ID: "pcB", Name: "PC_B", NodeType: "PC"},
		{ID: "uaMember", Name: "Members", NodeType: "UA"},
		{ID: "uaOwner", Name: "Owners", NodeType: "UA"},
		{ID: "uaOther", Name: "OtherTenantMembers", NodeType: "UA"},
		{ID: "user", Name: "User", NodeType: "U"},
		{ID: "stranger", Name: "Stranger", NodeType: "U"},
		{ID: "docs", Name: "Docs", NodeType: "OA"},
		{ID: "secret", Name: "Secret", NodeType: "OA"},
		{ID: "otherTenant", Name: "OtherTenantDocs", NodeType: "OA"},
		{ID: "orphan", Name: "OrphanNoPC", NodeType: "OA"},
	} {
		g.AddNode(n)
	}

	for i, a := range []struct{ child, parent string }{
		{"uaMember", "pcA"},
		{"uaOwner", "pcA"},
		{"uaOwner", "uaMember"},
		{"user", "uaMember"},
		{"uaOther", "pcB"},
		{"stranger", "uaOther"},
		{"docs", "pcA"},
		{"secret", "pcA"},
		{"otherTenant", "pcB"},
		// orphan is deliberately left unassigned — it reaches no PC.
	} {
		require.NoError(t, g.AddAssignment(&ngac.Assignment{
			ID: fmt.Sprintf("as%d", i), ChildID: a.child, ParentID: a.parent,
		}))
	}

	require.NoError(t, g.AddAssociation(&ngac.Association{
		ID: "m-docs", UAID: "uaMember", OAID: "docs", Operations: []string{"read"},
	}))
	require.NoError(t, g.AddAssociation(&ngac.Association{
		ID: "o-secret", UAID: "uaOwner", OAID: "secret", Operations: []string{"read", "write"},
	}))
	require.NoError(t, g.AddAssociation(&ngac.Association{
		ID: "b-other", UAID: "uaMember", OAID: "otherTenant", Operations: []string{"read"},
	}))

	objects := []string{"docs", "secret", "otherTenant", "orphan", "does-not-exist"}
	operations := []string{"read", "write", "manage"}

	for _, subject := range []string{"user", "stranger", "ghost-user"} {
		t.Run("subject="+subject, func(t *testing.T) {
			batch := g.CheckAccessBatch(subject, objects, operations)

			require.Len(t, batch, len(objects),
				"every requested object must appear in the result, present in the graph or not")

			for _, obj := range objects {
				for _, op := range operations {
					want := g.CheckAccess(subject, obj, op).Decision == "ALLOW"
					got := batch[obj][op]
					require.Equalf(t, want, got,
						"batch and single-object path disagree for %s/%s/%s", subject, obj, op)
				}
			}
		})
	}
}

// Sanity on the fixture itself: if every case above were DENY the agreement
// test would pass against a batch that always denies.
func TestCheckAccessBatch_FixtureExercisesBothOutcomes(t *testing.T) {
	g := ngac.NewGraph()
	for _, n := range []*ngac.NGACNode{
		{ID: "pc", Name: "PC", NodeType: "PC"},
		{ID: "ua", Name: "UA", NodeType: "UA"},
		{ID: "u", Name: "U", NodeType: "U"},
		{ID: "allowed", Name: "Allowed", NodeType: "OA"},
		{ID: "denied", Name: "Denied", NodeType: "OA"},
	} {
		g.AddNode(n)
	}
	for i, a := range []struct{ child, parent string }{
		{"ua", "pc"}, {"u", "ua"}, {"allowed", "pc"}, {"denied", "pc"},
	} {
		require.NoError(t, g.AddAssignment(&ngac.Assignment{
			ID: fmt.Sprintf("a%d", i), ChildID: a.child, ParentID: a.parent,
		}))
	}
	require.NoError(t, g.AddAssociation(&ngac.Association{
		ID: "assoc", UAID: "ua", OAID: "allowed", Operations: []string{"read"},
	}))

	batch := g.CheckAccessBatch("u", []string{"allowed", "denied"}, []string{"read"})
	require.True(t, batch["allowed"]["read"], "fixture must contain a genuine ALLOW")
	require.False(t, batch["denied"]["read"], "fixture must contain a genuine DENY")
}

// Duplicate object IDs must not multiply work or produce inconsistent entries.
func TestCheckAccessBatch_HandlesDuplicateIDs(t *testing.T) {
	g, objects := buildDeepUserGraph(3, 2)
	dupes := append(append([]string{}, objects...), objects...)

	batch := g.CheckAccessBatch("u", dupes, []string{"read"})

	require.Len(t, batch, len(objects))
	for _, obj := range objects {
		require.True(t, batch[obj]["read"])
	}
}

func TestCheckAccessBatch_EmptyInputs(t *testing.T) {
	g, objects := buildDeepUserGraph(3, 2)

	require.Empty(t, g.CheckAccessBatch("u", nil, []string{"read"}))

	noOps := g.CheckAccessBatch("u", objects, nil)
	require.Len(t, noOps, len(objects), "objects are still reported when no operation is asked about")
	for _, obj := range objects {
		require.Empty(t, noOps[obj])
	}
}
