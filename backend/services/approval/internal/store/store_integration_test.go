//go:build integration
// +build integration

// Package store_test provides integration tests for the approval store
// against a live PostgreSQL database. Run with:
//
//	go test ./internal/store/ -tags=integration -count=1 -v
//
// Prerequisites: PostgreSQL running at localhost:5432 with ngac/ngac_secret creds,
// migration 007 applied.
package store_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"ngac-platform/pkg/httputil"
	"ngac-platform/services/approval/internal/domain"
	"ngac-platform/services/approval/internal/store"
)

var testDB *pgxpool.Pool
var tenantAID, tenantBID string
var schemaA, schemaB string

func TestMain(m *testing.M) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://ngac:ngac_secret@localhost:5432/ngac?sslmode=disable"
	}

	var err error
	testDB, err = pgxpool.New(context.Background(), dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot connect to DB: %v\n", err)
		os.Exit(1)
	}
	defer testDB.Close()

	rows, err := testDB.Query(context.Background(), "SELECT id FROM workspaces LIMIT 2")
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot query workspaces: %v\n", err)
		os.Exit(1)
	}
	var wsIDs []string
	for rows.Next() {
		var id string
		rows.Scan(&id)
		wsIDs = append(wsIDs, id)
	}
	rows.Close()
	if len(wsIDs) < 2 {
		fmt.Fprintf(os.Stderr, "need at least 2 workspaces, got %d\n", len(wsIDs))
		os.Exit(1)
	}
	tenantAID = wsIDs[0]
	tenantBID = wsIDs[1]

	testDB.QueryRow(context.Background(), `SELECT provision_tenant_schema($1)`, tenantAID).Scan(&schemaA)
	testDB.QueryRow(context.Background(), `SELECT provision_tenant_schema($1)`, tenantBID).Scan(&schemaB)

	fmt.Fprintf(os.Stderr, "Tenants: A=%s (%s), B=%s (%s)\n", tenantAID[:8], schemaA, tenantBID[:8], schemaB)
	os.Exit(m.Run())
}

func tenantCtx(tenantID string) context.Context {
	schema := schemaA
	if tenantID == tenantBID {
		schema = schemaB
	}
	return httputil.WithTenantSchema(context.Background(), schema)
}

func newID() string { return uuid.New().String() }

func TestSchemaIsolation(t *testing.T) {
	s := store.NewStore(testDB)
	ctxA := tenantCtx(tenantAID)
	ctxB := tenantCtx(tenantBID)

	tmplID := newID()
	now := time.Now()
	tmpl := &domain.Template{
		ID: tmplID, Name: "Isolation Test", EntityType: "transfer",
		IsActive: true, Priority: 10, CreatedBy: "admin",
		CreatedAt: now, UpdatedAt: now,
		Conditions: []*domain.Condition{},
		Steps:      []*domain.Step{{ID: newID(), StepOrder: 1, Name: "Mgr", ApproverType: "specific_user", ApproverValue: "a1", RequiredCount: 1}},
	}
	if err := s.InsertTemplate(ctxA, tmpl); err != nil {
		t.Fatalf("insert in A: %v", err)
	}

	got, err := s.GetTemplate(ctxA, tmplID)
	if err != nil {
		t.Fatalf("get from A: %v", err)
	}
	if got.Name != tmpl.Name {
		t.Errorf("A name = %q, want %q", got.Name, tmpl.Name)
	}

	gotB, err := s.GetTemplate(ctxB, tmplID)
	if err != nil {
		t.Logf("B correctly cannot see A's template: %v", err)
	} else if gotB != nil {
		t.Errorf("ISOLATION VIOLATION: B sees A's template %q", gotB.Name)
	}
	t.Log("✅ Schema isolation verified")
}

func TestFullApprovalFlow(t *testing.T) {
	s := store.NewStore(testDB)
	ctx := tenantCtx(tenantAID)

	tmplID := newID()
	now := time.Now()
	tmpl := &domain.Template{
		ID: tmplID, Name: "3-Step Flow", EntityType: "payment",
		IsActive: true, Priority: 10, CreatedBy: "admin",
		CreatedAt: now, UpdatedAt: now,
		Conditions: []*domain.Condition{{ID: newID(), Field: "amount", Operator: "gt", Value: "1000000"}},
		Steps: []*domain.Step{
			{ID: newID(), StepOrder: 1, Name: "Manager", ApproverType: "specific_user", ApproverValue: "mgr1", RequiredCount: 1},
			{ID: newID(), StepOrder: 2, Name: "Director", ApproverType: "specific_user", ApproverValue: "dir1", RequiredCount: 1},
			{ID: newID(), StepOrder: 3, Name: "CEO", ApproverType: "specific_user", ApproverValue: "ceo1", RequiredCount: 1},
		},
	}
	if err := s.InsertTemplate(ctx, tmpl); err != nil {
		t.Fatalf("insert template: %v", err)
	}

	reqID := newID()
	entityID := newID()
	req := &domain.Request{
		ID: reqID, EntityType: "payment", EntityID: entityID,
		TemplateID: tmplID, TemplateName: tmpl.Name, TemplateSnapshot: `{}`,
		CurrentStep: 1, Status: "pending",
		ScopeOAID: "dept_oa_1", DepartmentID: "dept1", CreatedBy: "user1",
	}
	if err := s.InsertRequest(ctx, req); err != nil {
		t.Fatalf("insert request: %v", err)
	}

	asgnIDs := [3]string{newID(), newID(), newID()}
	assignments := []*domain.AssignmentRecord{
		{ID: asgnIDs[0], RequestID: reqID, StepOrder: 1, UserNodeID: "mgr1", GrantSource: "template", Status: "pending"},
		{ID: asgnIDs[1], RequestID: reqID, StepOrder: 2, UserNodeID: "dir1", GrantSource: "template", Status: "pending"},
		{ID: asgnIDs[2], RequestID: reqID, StepOrder: 3, UserNodeID: "ceo1", GrantSource: "template", Status: "pending"},
	}
	if err := s.InsertAssignments(ctx, assignments); err != nil {
		t.Fatalf("insert assignments: %v", err)
	}

	// Verify pending
	pending, err := s.ListPending(ctx, "mgr1")
	if err != nil {
		t.Fatalf("list pending: %v", err)
	}
	found := false
	for _, p := range pending {
		if p.Request.ID == reqID {
			found = true
		}
	}
	if !found {
		t.Error("mgr1 pending should contain request")
	}

	// Step 1: Manager approves
	if err := s.UpdateAssignmentStatus(ctx, asgnIDs[0], "approved", "LGTM"); err != nil {
		t.Fatalf("mgr approve: %v", err)
	}
	count, _ := s.CountApprovedForStep(ctx, reqID, 1)
	if count != 1 {
		t.Errorf("step 1 approved count = %d, want 1", count)
	}
	s.AdvanceStep(ctx, reqID, 1, 2)

	// Step 2: Director
	s.UpdateAssignmentStatus(ctx, asgnIDs[1], "approved", "OK")
	s.AdvanceStep(ctx, reqID, 2, 3)

	// Step 3: CEO → complete
	s.UpdateAssignmentStatus(ctx, asgnIDs[2], "approved", "Final")
	s.CompleteRequest(ctx, reqID, "approved")

	final, _ := s.GetRequest(ctx, reqID)
	if final.Status != "approved" {
		t.Errorf("final status = %q, want approved", final.Status)
	}
	t.Log("✅ 3-step approval flow completed")

	// Verify history
	history, _, _ := s.ListHistory(ctx, "mgr1", "", 10)
	found = false
	for _, h := range history {
		if h.Request.ID == reqID {
			found = true
		}
	}
	if !found {
		t.Error("mgr1 history should contain completed request")
	}
	t.Log("✅ History tab verified")

	// Verify audit
	auditID := newID()
	if err := s.InsertAuditEntry(ctx, &domain.AuditEntry{
		ID: auditID, RequestID: reqID, Action: "created", ActorNodeID: "user1",
		DetailJSON: `{}`, CreatedAt: time.Now(),
	}); err != nil {
		t.Logf("insert audit: %v", err)
	}
	entries, _ := s.ListAuditEntries(ctx, reqID)
	if len(entries) == 0 {
		t.Error("audit log should have entries")
	}
	t.Log("✅ Audit trail verified")
}

func TestScopeBasedListing(t *testing.T) {
	s := store.NewStore(testDB)
	ctx := tenantCtx(tenantAID)

	// Need a template first
	tmplID := newID()
	now := time.Now()
	s.InsertTemplate(ctx, &domain.Template{
		ID: tmplID, Name: "Scope Test", EntityType: "transfer",
		IsActive: true, CreatedBy: "admin",
		CreatedAt: now, UpdatedAt: now,
		Conditions: []*domain.Condition{},
		Steps:      []*domain.Step{{ID: newID(), StepOrder: 1, Name: "M", ApproverType: "specific_user", RequiredCount: 1}},
	})

	for i, scope := range []string{"scope_mn", "scope_mn", "scope_mb"} {
		reqID := newID()
		entityID := newID()
		s.InsertRequest(ctx, &domain.Request{
			ID: reqID, EntityType: "transfer", EntityID: entityID,
			TemplateID: tmplID, TemplateName: "Scope Test", TemplateSnapshot: `{}`,
			CurrentStep: 1, Status: "pending",
			ScopeOAID: scope, DepartmentID: fmt.Sprintf("dept%d", i), CreatedBy: "user1",
		})
	}

	mnItems, _, err := s.ListByScopes(ctx, []string{"scope_mn"}, "", 50)
	if err != nil {
		t.Fatalf("list MN: %v", err)
	}
	for _, item := range mnItems {
		if item.ScopeOAID != "scope_mn" {
			t.Errorf("MN query returned wrong scope: %s", item.ScopeOAID)
		}
	}
	t.Logf("✅ MN scope: %d items", len(mnItems))

	mbItems, _, _ := s.ListByScopes(ctx, []string{"scope_mb"}, "", 50)
	for _, item := range mbItems {
		if item.ScopeOAID != "scope_mb" {
			t.Errorf("MB query returned wrong scope: %s", item.ScopeOAID)
		}
	}
	t.Logf("✅ MB scope: %d items", len(mbItems))

	allItems, _, _ := s.ListByScopes(ctx, []string{"scope_mn", "scope_mb"}, "", 50)
	if len(allItems) < len(mnItems)+len(mbItems) {
		t.Logf("Warning: all=%d < mn+mb=%d", len(allItems), len(mnItems)+len(mbItems))
	}
	t.Logf("✅ CEO scope (all): %d items", len(allItems))
}

// Two approvals that satisfy the same quorum concurrently both see the count
// met and both call AdvanceStep. Only one may win, or each winner goes on to
// create a full set of assignments for the next step.
func TestAdvanceStep_IsCompareAndSwap(t *testing.T) {
	ctx := context.Background()
	s := store.NewStore(testDB)
	ctx = httputil.WithTenantSchema(ctx, schemaA)

	reqID := insertCASRequest(t, ctx, s, "CAS")

	first, err := s.AdvanceStep(ctx, reqID, 1, 2)
	if err != nil {
		t.Fatalf("first advance: %v", err)
	}
	if !first {
		t.Fatal("first advance should win")
	}

	// The loser still believes current_step is 1.
	second, err := s.AdvanceStep(ctx, reqID, 1, 2)
	if err != nil {
		t.Fatalf("second advance: %v", err)
	}
	if second {
		t.Error("second advance from the same step must lose, got a win")
	}

	got, _ := s.GetRequest(ctx, reqID)
	if got.CurrentStep != 2 {
		t.Errorf("current_step = %d, want 2 (advanced exactly once)", got.CurrentStep)
	}
}

// A terminal status must be reached exactly once, so a late approval cannot
// overwrite a rejection.
func TestCompleteRequest_OnlyFromPending(t *testing.T) {
	ctx := context.Background()
	s := store.NewStore(testDB)
	ctx = httputil.WithTenantSchema(ctx, schemaA)

	reqID := insertCASRequest(t, ctx, s, "CAS2")

	ok, err := s.CompleteRequest(ctx, reqID, "rejected")
	if err != nil || !ok {
		t.Fatalf("first completion should win: ok=%v err=%v", ok, err)
	}

	ok, err = s.CompleteRequest(ctx, reqID, "approved")
	if err != nil {
		t.Fatalf("second completion: %v", err)
	}
	if ok {
		t.Error("a completed request must not be completable again")
	}

	got, _ := s.GetRequest(ctx, reqID)
	if got.Status != "rejected" {
		t.Errorf("status = %q, want rejected (first terminal status wins)", got.Status)
	}
}

// insertCASRequest creates a template plus a pending request bound to it, so the
// request satisfies the approval_requests -> approval_templates foreign key.
func insertCASRequest(t *testing.T, ctx context.Context, s *store.Store, label string) string {
	t.Helper()
	tmplID := newID()
	now := time.Now()
	if err := s.InsertTemplate(ctx, &domain.Template{
		ID: tmplID, Name: label, EntityType: "transfer",
		IsActive: true, CreatedBy: "admin", CreatedAt: now, UpdatedAt: now,
		Conditions: []*domain.Condition{},
		Steps:      []*domain.Step{{ID: newID(), StepOrder: 1, Name: "M", ApproverType: "specific_user", RequiredCount: 1}},
	}); err != nil {
		t.Fatalf("insert template: %v", err)
	}

	reqID := newID()
	if err := s.InsertRequest(ctx, &domain.Request{
		ID: reqID, EntityType: "transfer", EntityID: newID(),
		TemplateID: tmplID, TemplateName: label, TemplateSnapshot: `{}`,
		CurrentStep: 1, Status: "pending",
		ScopeOAID: "scope_cas", DepartmentID: "dept1", CreatedBy: "user1",
	}); err != nil {
		t.Fatalf("insert request: %v", err)
	}

	// Leave the schema as we found it, so repeated runs do not slowly grow the
	// dataset that the performance assertions measure against.
	t.Cleanup(func() {
		bg := context.Background()
		testDB.Exec(bg, fmt.Sprintf("DELETE FROM %s.approval_requests WHERE id = $1", schemaA), reqID)
		testDB.Exec(bg, fmt.Sprintf("DELETE FROM %s.approval_steps WHERE template_id = $1", schemaA), tmplID)
		testDB.Exec(bg, fmt.Sprintf("DELETE FROM %s.approval_templates WHERE id = $1", schemaA), tmplID)
	})
	return reqID
}
