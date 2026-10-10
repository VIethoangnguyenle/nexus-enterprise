package store_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"ngac-platform/services/approval/internal/domain"
	"ngac-platform/services/approval/internal/store"
	"ngac-platform/testutil"
)

// seedRequest stores a pending request created by creator in scope with one
// pending assignment per approver, created `ago` in the past.
func seedRequest(t *testing.T, ctx context.Context, st *store.Store, tmplID, creator, scope string, ago time.Duration, approvers ...string) *domain.Request {
	t.Helper()
	req := &domain.Request{
		ID: uuid.NewString(), EntityType: "expense", EntityID: uuid.NewString(), TemplateID: tmplID, TemplateName: "T",
		TemplateSnapshot: `{"steps":[]}`, CurrentStep: 1, Status: "pending", ScopeOAID: scope, DepartmentID: "d",
		CreatedBy: creator, CreatedAt: time.Now().Add(-ago),
	}
	var as []*domain.AssignmentRecord
	for _, a := range approvers {
		as = append(as, &domain.AssignmentRecord{ID: uuid.NewString(), RequestID: req.ID, StepOrder: 1, UserNodeID: a, GrantSource: "direct", Status: "pending"})
	}
	if err := st.InsertRequestWithAssignments(ctx, req, as); err != nil {
		t.Fatalf("seed request: %v", err)
	}
	return req
}

func ids(rs []*domain.Request) map[string]bool {
	m := map[string]bool{}
	for _, r := range rs {
		m[r.ID] = true
	}
	return m
}

func TestListPending_ShowsOnlyWhatIsWaitingOnTheCaller(t *testing.T) {
	ctx, st, _ := scratchTenant(t)
	tmpl := insertTemplate(t, ctx, st)
	mine := seedRequest(t, ctx, st, tmpl, "creator", "oa", time.Hour, "alice")
	theirs := seedRequest(t, ctx, st, tmpl, "creator", "oa", time.Hour, "bob")

	got, err := st.ListPending(ctx, "alice", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Request.ID != mine.ID || got[0].Assignment.UserNodeID != "alice" {
		t.Fatalf("alice sees %+v, want only her own request", got)
	}
	for _, g := range got {
		if g.Request.ID == theirs.ID {
			t.Fatal("alice sees bob's request")
		}
	}
	if none, _ := st.ListPending(ctx, "stranger", nil); len(none) != 0 {
		t.Fatalf("a stranger sees %d requests", len(none))
	}

	// A decided assignment is no longer pending.
	if err := st.UpdateAssignmentStatus(ctx, got[0].Assignment.ID, "approved", "ok"); err != nil {
		t.Fatal(err)
	}
	if after, _ := st.ListPending(ctx, "alice", nil); len(after) != 0 {
		t.Fatalf("an approved assignment is still pending: %d", len(after))
	}
}

func TestListPending_ReachesAGroupAssignmentOnlyThroughItsGrantSource(t *testing.T) {
	ctx, st, _ := scratchTenant(t)
	tmpl := insertTemplate(t, ctx, st)
	req, _ := newRequest(tmpl)
	role := &domain.AssignmentRecord{ID: uuid.NewString(), RequestID: req.ID, StepOrder: 1, UserNodeID: "role-node", GrantSource: "role:role-node", Status: "pending"}
	direct := &domain.AssignmentRecord{ID: uuid.NewString(), RequestID: req.ID, StepOrder: 1, UserNodeID: "someone-else", GrantSource: "direct", Status: "pending"}
	if err := st.InsertRequestWithAssignments(ctx, req, []*domain.AssignmentRecord{role, direct}); err != nil {
		t.Fatal(err)
	}

	if got, _ := st.ListPending(ctx, "member", []string{"role-node"}); len(got) != 1 || got[0].Assignment.ID != role.ID {
		t.Fatalf("a member of the role sees %+v, want the role's assignment", got)
	}
	if got, _ := st.ListPending(ctx, "member", []string{"someone-else"}); len(got) != 0 {
		t.Fatalf("a direct assignment must not be reachable as a group: %+v", got)
	}
	if got, _ := st.ListPending(ctx, "member", nil); len(got) != 0 {
		t.Fatalf("no groups, no group assignments: %+v", got)
	}
}

func TestListHistory_HoldsOnlyWhatTheCallerDecided(t *testing.T) {
	ctx, st, _ := scratchTenant(t)
	tmpl := insertTemplate(t, ctx, st)
	req := seedRequest(t, ctx, st, tmpl, "creator", "oa", time.Hour, "alice", "bob")
	a, err := st.GetAssignment(ctx, req.ID, "alice")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.UpdateAssignmentStatus(ctx, a.ID, "approved", "fine"); err != nil {
		t.Fatal(err)
	}

	hist, next, err := st.ListHistory(ctx, "alice", "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(hist) != 1 || hist[0].Request.ID != req.ID || hist[0].Assignment.Status != "approved" || next != "" {
		t.Fatalf("history = %+v next=%q", hist, next)
	}
	if other, _, _ := st.ListHistory(ctx, "bob", "", 10); len(other) != 0 {
		t.Fatalf("bob has decided nothing but sees %d", len(other))
	}
	if _, err := st.GetAssignment(ctx, req.ID, "alice"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("a decided assignment is not pending any more: %v", err)
	}
}

func TestListMyRequests_PagesNewestFirstAndOnlyTheCreators(t *testing.T) {
	ctx, st, _ := scratchTenant(t)
	tmpl := insertTemplate(t, ctx, st)
	oldest := seedRequest(t, ctx, st, tmpl, "me", "oa", 3*time.Hour, "x")
	middle := seedRequest(t, ctx, st, tmpl, "me", "oa", 2*time.Hour, "x")
	newest := seedRequest(t, ctx, st, tmpl, "me", "oa", time.Hour, "x")
	seedRequest(t, ctx, st, tmpl, "not-me", "oa", time.Minute, "x")

	page1, cursor, err := st.ListMyRequests(ctx, "me", "", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(page1) != 2 || page1[0].ID != newest.ID || page1[1].ID != middle.ID || cursor == "" {
		t.Fatalf("page 1 = %v cursor=%q", ids(page1), cursor)
	}
	page2, cursor2, err := st.ListMyRequests(ctx, "me", cursor, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(page2) != 1 || page2[0].ID != oldest.ID || cursor2 != "" {
		t.Fatalf("page 2 = %v cursor=%q", ids(page2), cursor2)
	}
}

func TestListByScopes_StaysInsideTheScopesGiven(t *testing.T) {
	ctx, st, _ := scratchTenant(t)
	tmpl := insertTemplate(t, ctx, st)
	in := seedRequest(t, ctx, st, tmpl, "c", "oa-sales", time.Hour, "x")
	seedRequest(t, ctx, st, tmpl, "c", "oa-finance", time.Hour, "x")

	got, _, err := st.ListByScopes(ctx, []string{"oa-sales"}, "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != in.ID {
		t.Fatalf("scope list = %v", ids(got))
	}
	if none, _, _ := st.ListByScopes(ctx, nil, "", 10); len(none) != 0 {
		t.Fatalf("no scope, no requests: %v", ids(none))
	}
}

func TestAssignmentLookups(t *testing.T) {
	ctx, st, _ := scratchTenant(t)
	tmpl := insertTemplate(t, ctx, st)
	req := seedRequest(t, ctx, st, tmpl, "c", "oa", time.Hour, "alice", "bob")

	if ok, _ := st.HasAssignment(ctx, req.ID, []string{"alice"}); !ok {
		t.Error("alice has an assignment")
	}
	if ok, _ := st.HasAssignment(ctx, req.ID, []string{"carol", "dave"}); ok {
		t.Error("carol and dave have none")
	}
	if ok, _ := st.HasAssignment(ctx, req.ID, nil); ok {
		t.Error("no nodes, no assignment")
	}

	pending, err := st.ListPendingAssignees(ctx, req.ID, 1)
	if err != nil || len(pending) != 2 || pending[0] != "alice" || pending[1] != "bob" {
		t.Fatalf("pending = %v err=%v", pending, err)
	}
	a, _ := st.GetAssignment(ctx, req.ID, "alice")
	if err := st.UpdateAssignmentStatus(ctx, a.ID, "approved", ""); err != nil {
		t.Fatal(err)
	}
	pending, _ = st.ListPendingAssignees(ctx, req.ID, 1)
	if len(pending) != 1 || pending[0] != "bob" {
		t.Fatalf("pending after alice acted = %v", pending)
	}
	if n, _ := st.CountApprovedForStep(ctx, req.ID, 1); n != 1 {
		t.Fatalf("approved = %d", n)
	}
	if pending, _ := st.ListPendingAssignees(ctx, req.ID, 2); len(pending) != 0 {
		t.Fatalf("step 2 has nobody: %v", pending)
	}
}

func TestInsertAssignments_AddsRowsAndRefusesADuplicateID(t *testing.T) {
	ctx, st, _ := scratchTenant(t)
	tmpl := insertTemplate(t, ctx, st)
	req := seedRequest(t, ctx, st, tmpl, "c", "oa", time.Hour, "alice")

	extra := &domain.AssignmentRecord{ID: uuid.NewString(), RequestID: req.ID, StepOrder: 2, UserNodeID: "carol", GrantSource: "direct", Status: "pending"}
	if err := st.InsertAssignments(ctx, []*domain.AssignmentRecord{extra}); err != nil {
		t.Fatal(err)
	}
	if rows, _ := st.ListAssignments(ctx, req.ID); len(rows) != 2 {
		t.Fatalf("rows = %d", len(rows))
	}
	if err := st.InsertAssignments(ctx, []*domain.AssignmentRecord{extra}); err == nil {
		t.Fatal("the same assignment id twice must be refused")
	}
}

func TestFindPendingByGrantSource(t *testing.T) {
	ctx, st, _ := scratchTenant(t)
	tmpl := insertTemplate(t, ctx, st)
	req, _ := newRequest(tmpl)
	src := "role:chief-" + uuid.NewString()[:6]
	as := []*domain.AssignmentRecord{
		{ID: uuid.NewString(), RequestID: req.ID, StepOrder: 1, UserNodeID: "alice", GrantSource: src, Status: "pending"},
		{ID: uuid.NewString(), RequestID: req.ID, StepOrder: 1, UserNodeID: "bob", GrantSource: src, Status: "pending"},
		{ID: uuid.NewString(), RequestID: req.ID, StepOrder: 1, UserNodeID: "carol", GrantSource: "direct", Status: "pending"},
	}
	if err := st.InsertRequestWithAssignments(ctx, req, as); err != nil {
		t.Fatal(err)
	}

	got, err := st.FindPendingByGrantSource(ctx, src)
	if err != nil || len(got) != 2 {
		t.Fatalf("by source = %d err=%v", len(got), err)
	}
	one, err := st.FindPendingByUserAndSource(ctx, "alice", src)
	if err != nil || len(one) != 1 || one[0].UserNodeID != "alice" {
		t.Fatalf("by user+source = %+v err=%v", one, err)
	}
	if none, _ := st.FindPendingByUserAndSource(ctx, "carol", src); len(none) != 0 {
		t.Fatalf("carol's grant is direct: %+v", none)
	}
}

func TestAdvanceStep_IsACompareAndSwapOnAPendingRequest(t *testing.T) {
	ctx, st, _ := scratchTenant(t)
	tmpl := insertTemplate(t, ctx, st)
	req := seedRequest(t, ctx, st, tmpl, "c", "oa", time.Hour, "alice")

	if moved, err := st.AdvanceStep(ctx, req.ID, 2, 3); err != nil || moved {
		t.Fatalf("a step the request is not on must not move: moved=%v err=%v", moved, err)
	}
	if moved, err := st.AdvanceStep(ctx, req.ID, 1, 2); err != nil || !moved {
		t.Fatalf("1 -> 2: moved=%v err=%v", moved, err)
	}
	if moved, _ := st.AdvanceStep(ctx, req.ID, 1, 2); moved {
		t.Fatal("the second caller of the same transition must lose")
	}
	if ok, err := st.CompleteRequest(ctx, req.ID, "approved"); err != nil || !ok {
		t.Fatalf("complete: ok=%v err=%v", ok, err)
	}
	if moved, _ := st.AdvanceStep(ctx, req.ID, 2, 3); moved {
		t.Fatal("a finished request does not advance")
	}
}

func TestInsertRequest_StoresAndReadsBack(t *testing.T) {
	ctx, st, _ := scratchTenant(t)
	tmpl := insertTemplate(t, ctx, st)
	req, _ := newRequest(tmpl)
	req.FormDataJSON = `{"amount":5}`
	if err := st.InsertRequest(ctx, req); err != nil {
		t.Fatal(err)
	}
	got, err := st.GetRequest(ctx, req.ID)
	if err != nil || got.FormDataJSON == "" || got.CreatedBy != req.CreatedBy {
		t.Fatalf("got %+v err=%v", got, err)
	}
	if err := st.InsertRequest(ctx, req); err == nil {
		t.Fatal("the same request id twice must be refused")
	}
}

func TestUpdateTemplate_ReplacesStepsAndRefusesAStaleOrUnknownEdit(t *testing.T) {
	ctx, st, _ := scratchTenant(t)
	id := insertTemplate(t, ctx, st)
	tmpl, err := st.GetTemplate(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	readAt := tmpl.UpdatedAt

	tmpl.Name = "Renamed"
	tmpl.Steps = []*domain.Step{
		{ID: uuid.NewString(), StepOrder: 1, Name: "First", ApproverType: "specific_user", ApproverValue: "a", RequiredCount: 1},
		{ID: uuid.NewString(), StepOrder: 2, Name: "Second", ApproverType: "specific_user", ApproverValue: "b", RequiredCount: 1},
	}
	tmpl.Conditions = []*domain.Condition{{ID: uuid.NewString(), Field: "amount", Operator: "gt", Value: "100"}}
	tmpl.FormFields = []*domain.FormField{{Label: "Why", FieldType: "text", Required: true}}
	newAt, err := st.UpdateTemplate(ctx, tmpl, readAt)
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if !newAt.After(readAt) {
		t.Errorf("updated_at did not move: %v -> %v", readAt, newAt)
	}
	got, _ := st.GetTemplate(ctx, id)
	if got.Name != "Renamed" || len(got.Steps) != 2 || len(got.Conditions) != 1 || len(got.FormFields) != 1 {
		t.Fatalf("saved = %+v", got)
	}

	// A second edit made from the same read is refused and changes nothing.
	stale := *tmpl
	stale.Name = "Lost update"
	if _, err := st.UpdateTemplate(ctx, &stale, readAt); !errors.Is(err, domain.ErrStale) {
		t.Fatalf("stale edit: %v, want ErrStale", err)
	}
	if got, _ := st.GetTemplate(ctx, id); got.Name != "Renamed" {
		t.Fatalf("a refused edit changed the name to %q", got.Name)
	}

	missing := *tmpl
	missing.ID = uuid.NewString()
	if _, err := st.UpdateTemplate(ctx, &missing, readAt); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("unknown template: %v, want ErrNotFound", err)
	}
}

func TestProvisionSchemaAndFindNodeID(t *testing.T) {
	ctx, st, pool := scratchTenant(t)
	bg := context.Background()
	owner, _ := testutil.CreateUser(t, pool)
	id := "y" + uuid.NewString()[:7]
	if _, err := pool.Exec(bg, `INSERT INTO workspaces (id, name, owner_id) VALUES ($1, 'prov', $2)`, id, owner); err != nil {
		t.Fatal(err)
	}
	schema, err := st.ProvisionSchema(bg, id)
	if err != nil || schema == "" {
		t.Fatalf("schema=%q err=%v", schema, err)
	}
	t.Cleanup(func() {
		pool.Exec(bg, `DROP SCHEMA IF EXISTS "`+schema+`" CASCADE`)
		pool.Exec(bg, `DELETE FROM tenant_schemas WHERE tenant_id = $1`, id)
		pool.Exec(bg, `DELETE FROM workspaces WHERE id = $1`, id)
	})
	again, err := st.ProvisionSchema(bg, id)
	if err != nil || again != schema {
		t.Fatalf("provisioning twice must give the same schema: %q vs %q err=%v", again, schema, err)
	}

	nodeName := "findme-" + uuid.NewString()
	nodeID := uuid.NewString()
	if _, err := pool.Exec(bg, `INSERT INTO ngac_nodes (id, name, node_type) VALUES ($1, $2, 'UA')`, nodeID, nodeName); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pool.Exec(bg, `DELETE FROM ngac_nodes WHERE id = $1`, nodeID) })
	if got, err := st.FindNodeID(ctx, nodeName, "UA"); err != nil || got != nodeID {
		t.Fatalf("FindNodeID = %q err=%v", got, err)
	}
	if _, err := st.FindNodeID(ctx, nodeName, "OA"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("the type is part of the key: %v", err)
	}
}

// Walking every page of a list yields every row once: the cursor names the last
// row of a page, so the next page starts right after it and skips nothing.
func TestPaging_VisitsEveryRowExactlyOnce(t *testing.T) {
	ctx, st, _ := scratchTenant(t)
	tmpl := insertTemplate(t, ctx, st)
	var want []string
	for i := 5; i >= 1; i-- {
		r := seedRequest(t, ctx, st, tmpl, "me", "oa-walk", time.Duration(i)*time.Hour, "me")
		want = append([]string{r.ID}, want...) // newest first
		a, err := st.GetAssignment(ctx, r.ID, "me")
		if err != nil {
			t.Fatal(err)
		}
		if err := st.UpdateAssignmentStatus(ctx, a.ID, "approved", ""); err != nil {
			t.Fatal(err)
		}
		time.Sleep(2 * time.Millisecond) // acted_at orders history
	}

	var mine, scoped, history []string
	cursor := ""
	for page := 0; page < 10; page++ {
		rs, next, err := st.ListMyRequests(ctx, "me", cursor, 2)
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range rs {
			mine = append(mine, r.ID)
		}
		if cursor = next; cursor == "" {
			break
		}
	}
	cursor = ""
	for page := 0; page < 10; page++ {
		rs, next, err := st.ListByScopes(ctx, []string{"oa-walk"}, cursor, 2)
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range rs {
			scoped = append(scoped, r.ID)
		}
		if cursor = next; cursor == "" {
			break
		}
	}
	cursor = ""
	for page := 0; page < 10; page++ {
		rs, next, err := st.ListHistory(ctx, "me", cursor, 2)
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range rs {
			history = append(history, r.Request.ID)
		}
		if cursor = next; cursor == "" {
			break
		}
	}

	same := func(name string, got []string) {
		t.Helper()
		if len(got) != len(want) {
			t.Fatalf("%s visited %d rows, want %d: %v", name, len(got), len(want), got)
		}
		seen := map[string]bool{}
		for _, id := range got {
			if seen[id] {
				t.Fatalf("%s visited %s twice", name, id)
			}
			seen[id] = true
		}
		for _, id := range want {
			if !seen[id] {
				t.Fatalf("%s skipped %s", name, id)
			}
		}
	}
	same("my requests", mine)
	same("by scopes", scoped)
	same("history", history)
}

// Rows that share a timestamp must not be skipped or repeated where a page
// ends between them: the cursor is the pair (timestamp, id), and the order is
// the same pair.
func TestPaging_EqualTimestampsAcrossAPageBoundary(t *testing.T) {
	ctx, st, _ := scratchTenant(t)
	tmpl := insertTemplate(t, ctx, st)
	same := time.Now().Add(-time.Hour).Truncate(time.Microsecond)

	var want []string
	for i := 0; i < 7; i++ {
		req, a := newRequest(tmpl)
		req.CreatedAt, req.CreatedBy, req.ScopeOAID = same, "me", "oa-tie"
		a.UserNodeID = "me"
		if err := st.InsertRequestWithAssignments(ctx, req, []*domain.AssignmentRecord{a}); err != nil {
			t.Fatal(err)
		}
		want = append(want, req.ID)
	}
	// All seven are decided in one transaction, so they share an acted_at too.
	if err := st.InTx(ctx, func(ctx context.Context) error {
		for _, id := range want {
			a, err := st.GetAssignment(ctx, id, "me")
			if err != nil {
				return err
			}
			if err := st.UpdateAssignmentStatus(ctx, a.ID, "approved", ""); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	for name, walk := range map[string]func(cursor string) ([]string, string, error){
		"my requests": func(c string) ([]string, string, error) {
			rs, next, err := st.ListMyRequests(ctx, "me", c, 3)
			return idsOf(rs), next, err
		},
		"by scopes": func(c string) ([]string, string, error) {
			rs, next, err := st.ListByScopes(ctx, []string{"oa-tie"}, c, 3)
			return idsOf(rs), next, err
		},
		"history": func(c string) ([]string, string, error) {
			rs, next, err := st.ListHistory(ctx, "me", c, 3)
			var out []string
			for _, r := range rs {
				out = append(out, r.Request.ID)
			}
			return out, next, err
		},
	} {
		var got []string
		cursor := ""
		for page := 0; page < 10; page++ {
			ids, next, err := walk(cursor)
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			got = append(got, ids...)
			if cursor = next; cursor == "" {
				break
			}
		}
		seen := map[string]int{}
		for _, id := range got {
			seen[id]++
		}
		if len(got) != len(want) {
			t.Fatalf("%s visited %d rows, want %d", name, len(got), len(want))
		}
		for _, id := range want {
			if seen[id] != 1 {
				t.Fatalf("%s saw %s %d times", name, id, seen[id])
			}
		}
	}
}

func idsOf(rs []*domain.Request) []string {
	var out []string
	for _, r := range rs {
		out = append(out, r.ID)
	}
	return out
}

// A cursor that is not one of ours is a bad request, not a database error.
func TestPaging_AMalformedCursorIsRefused(t *testing.T) {
	ctx, st, _ := scratchTenant(t)
	for _, bad := range []string{"yesterday", "2026-10-01T00:00:00Z", "x|y", "|"} {
		if _, _, err := st.ListMyRequests(ctx, "me", bad, 3); !errors.Is(err, domain.ErrInvalidInput) {
			t.Errorf("my requests, cursor %q: %v", bad, err)
		}
		if _, _, err := st.ListByScopes(ctx, []string{"oa"}, bad, 3); !errors.Is(err, domain.ErrInvalidInput) {
			t.Errorf("scopes, cursor %q: %v", bad, err)
		}
		if _, _, err := st.ListHistory(ctx, "me", bad, 3); !errors.Is(err, domain.ErrInvalidInput) {
			t.Errorf("history, cursor %q: %v", bad, err)
		}
	}
}
