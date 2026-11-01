package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"ngac-platform/ngac"
	"ngac-platform/services/approval/internal/domain"
	"ngac-platform/services/approval/internal/store"
	"ngac-platform/testutil"
)

// A request never waits on the person who made it, even when a role they are
// in is the approver.
func TestListPending_NeverListsTheCallersOwnRequest(t *testing.T) {
	ctx, st, _ := scratchTenant(t)
	tmpl := insertTemplate(t, ctx, st)
	seedRequest(t, ctx, st, tmpl, "alice", "oa", time.Hour, "alice") // her own, assigned to her
	other := seedRequest(t, ctx, st, tmpl, "bob", "oa", time.Hour, "alice")
	viaRole, _ := newRequest(tmpl)
	viaRole.CreatedBy = "alice"
	role := &domain.AssignmentRecord{ID: uuid.NewString(), RequestID: viaRole.ID, StepOrder: 1, UserNodeID: "role-node", GrantSource: "role:role-node", Status: "pending"}
	if err := st.InsertRequestWithAssignments(ctx, viaRole, []*domain.AssignmentRecord{role}); err != nil {
		t.Fatal(err)
	}

	got, err := st.ListPending(ctx, "alice", []string{"role-node"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Request.ID != other.ID {
		t.Fatalf("alice's pending = %+v, want only bob's request", got)
	}
	// Another member of the role still sees alice's request.
	if got, _ := st.ListPending(ctx, "bob", []string{"role-node"}); len(got) != 1 || got[0].Request.ID != viaRole.ID {
		t.Fatalf("bob's pending = %+v, want alice's request through the role", got)
	}
}

// The creator's department comes from the graph edge person → department UA,
// only for departments of the tenant.
func TestCreatorDepartments_FromTheGraphWithinTheTenant(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	s := store.NewStore(pool)
	ctx := context.Background()
	a := seedTenant(t, pool, "A")
	b := seedTenant(t, pool, "B")

	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("fixture %q: %v", sql, err)
		}
	}
	edge := "sod-edge-" + uuid.NewString()
	crossEdge := "sod-cross-" + uuid.NewString()
	exec(`INSERT INTO ngac_nodes (id, name, node_type) VALUES ($1, $1, 'UA'), ($2, $2, 'UA')`, a.deptUA, b.deptUA)
	exec(`INSERT INTO ngac_assignments (id, child_id, parent_id) VALUES ($1, $2, $3), ($4, $2, $5)`, edge, a.person, a.deptUA, crossEdge, b.deptUA)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM ngac_assignments WHERE id = ANY($1)`, []string{edge, crossEdge})
		_, _ = pool.Exec(context.Background(), `DELETE FROM ngac_nodes WHERE id = ANY($1)`, []string{a.deptUA, b.deptUA})
	})

	got, err := s.CreatorDepartments(ctx, a.ws, a.person)
	if err != nil || len(got) != 1 || got[0] != a.dept {
		t.Fatalf("departments in A = %v, %v; want only %s", got, err, a.dept)
	}
	if got, err := s.CreatorDepartments(ctx, a.ws, a.bare); err != nil || len(got) != 0 {
		t.Errorf("a person in no department = %v, %v", got, err)
	}
	if got, err := s.CreatorDepartments(ctx, "", a.person); err != nil || len(got) != 0 {
		t.Errorf("no tenant = %v, %v", got, err)
	}
}

// A scope is an object attribute under the tenant's policy class, nothing else.
func TestScopeInTenant_OnlyObjectAttributesOfTheTenant(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	s := store.NewStore(pool)
	ctx := context.Background()
	a := seedTenant(t, pool, "A")
	b := seedTenant(t, pool, "B")

	var pcA string
	if err := pool.QueryRow(ctx, `SELECT id FROM ngac_nodes WHERE name = $1 AND node_type = 'PC'`, ngac.PCName(ngac.WorkspaceID(a.ws))).Scan(&pcA); err != nil {
		t.Fatal(err)
	}
	oa, child, edge1, edge2 := "sod-oa-"+uuid.NewString(), "sod-oa-child-"+uuid.NewString(), uuid.NewString(), uuid.NewString()
	for _, q := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO ngac_nodes (id, name, node_type) VALUES ($1, $1, 'OA'), ($2, $2, 'OA')`, []any{oa, child}},
		{`INSERT INTO ngac_assignments (id, child_id, parent_id) VALUES ($1, $2, $3), ($4, $5, $2)`, []any{edge1, oa, pcA, edge2, child}},
	} {
		if _, err := pool.Exec(ctx, q.sql, q.args...); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM ngac_assignments WHERE id = ANY($1)`, []string{edge1, edge2})
		_, _ = pool.Exec(context.Background(), `DELETE FROM ngac_nodes WHERE id = ANY($1)`, []string{oa, child})
	})

	check := func(tenant, node string, want bool) {
		t.Helper()
		got, err := s.ScopeInTenant(ctx, tenant, node)
		if err != nil || got != want {
			t.Errorf("ScopeInTenant(%s, %s) = %v, %v; want %v", tenant, node, got, err, want)
		}
	}
	check(a.ws, oa, true)
	check(a.ws, child, true) // nested under an attribute of the tenant
	check(b.ws, oa, false)   // another tenant's attribute
	check(a.ws, a.role, false)
	check(a.ws, a.person, false)
	check(a.ws, "nothing-"+uuid.NewString(), false)
	check("", oa, false)
}

// GroupMembers reads the people at or below a UA from the graph tables.
func TestGroupMembers_PeopleAtOrBelowTheGroup(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	s := store.NewStore(pool)
	ctx := context.Background()
	sfx := uuid.NewString()
	role, sub, p1, p2, out := "gm-role-"+sfx, "gm-sub-"+sfx, "gm-p1-"+sfx, "gm-p2-"+sfx, "gm-out-"+sfx
	e1, e2, e3 := "gm-e1-"+sfx, "gm-e2-"+sfx, "gm-e3-"+sfx
	for _, q := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO ngac_nodes (id, name, node_type) VALUES ($1, $1, 'UA'), ($2, $2, 'UA'), ($3, $3, 'U'), ($4, $4, 'U'), ($5, $5, 'U')`, []any{role, sub, p1, p2, out}},
		{`INSERT INTO ngac_assignments (id, child_id, parent_id) VALUES ($1, $2, $3), ($4, $5, $3), ($6, $7, $5)`, []any{e1, p1, role, e2, sub, e3, p2}},
	} {
		if _, err := pool.Exec(ctx, q.sql, q.args...); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM ngac_assignments WHERE id = ANY($1)`, []string{e1, e2, e3})
		_, _ = pool.Exec(context.Background(), `DELETE FROM ngac_nodes WHERE id = ANY($1)`, []string{role, sub, p1, p2, out})
	})
	got, err := s.GroupMembers(ctx, role)
	if err != nil || len(got) != 2 || !(got[0] == p1 && got[1] == p2 || got[0] == p2 && got[1] == p1) {
		t.Fatalf("members = %v, %v; want %s and %s", got, err, p1, p2)
	}
	if got, err := s.GroupMembers(ctx, "nothing-"+sfx); err != nil || len(got) != 0 {
		t.Errorf("unknown group = %v, %v", got, err)
	}
}

// The start-up sweep, against a real tenant schema: a request waiting on a
// step that names only its requester moves to the next step, with the skip on
// its trail, and a second sweep changes nothing.
func TestReconcile_UnsticksARealTenantsRequest(t *testing.T) {
	ctx, st, _ := scratchTenant(t)
	tmpl := insertTemplate(t, ctx, st)
	req, _ := newRequest(tmpl)
	req.CreatedBy = "creator-" + uuid.NewString()
	req.TemplateSnapshot = `{"steps":[` +
		`{"step_order":1,"approver_type":"specific_user","approver_value":"` + req.CreatedBy + `","required_count":1},` +
		`{"step_order":2,"approver_type":"specific_user","approver_value":"boss","required_count":1}]}`
	stuck := &domain.AssignmentRecord{ID: uuid.NewString(), RequestID: req.ID, StepOrder: 1, UserNodeID: req.CreatedBy, GrantSource: "direct", Status: "pending"}
	if err := st.InsertRequestWithAssignments(ctx, req, []*domain.AssignmentRecord{stuck}); err != nil {
		t.Fatal(err)
	}
	svc := domain.NewService(st, nil)

	// Other tenants in the shared test database may fail their own sweep; this
	// one's outcome is what is checked.
	if _, err := svc.ReconcileSoleApproverSteps(context.Background()); err != nil {
		t.Logf("sweep reported (other tenants): %v", err)
	}
	got, err := st.GetRequest(ctx, req.ID)
	if err != nil || got.CurrentStep != 2 || got.Status != "pending" {
		t.Fatalf("request = %+v, %v; want pending on step 2", got, err)
	}
	if a, err := st.GetAssignment(ctx, req.ID, "boss"); err != nil || a.Status != "pending" || a.StepOrder != 2 {
		t.Fatalf("step 2 row = %+v, %v", a, err)
	}
	countSkips := func() int {
		entries, err := st.ListAuditEntries(ctx, req.ID)
		if err != nil {
			t.Fatal(err)
		}
		n := 0
		for _, e := range entries {
			if e.Action == "step_skipped" && e.ActorNodeID == "" && e.StepOrder == 1 {
				n++
			}
		}
		return n
	}
	if n := countSkips(); n != 1 {
		t.Fatalf("skip entries = %d, want 1", n)
	}
	_, _ = svc.ReconcileSoleApproverSteps(context.Background())
	if n := countSkips(); n != 1 {
		t.Errorf("after a second sweep skip entries = %d, want 1", n)
	}
	ids, err := st.ListPendingRequestIDs(ctx)
	if err != nil || len(ids) != 1 || ids[0] != req.ID {
		t.Errorf("pending ids = %v, %v", ids, err)
	}
}
