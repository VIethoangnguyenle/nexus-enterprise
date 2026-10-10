package store_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"ngac-platform/pkg/httputil"
	"ngac-platform/services/approval/internal/domain"
	"ngac-platform/services/approval/internal/store"
	"ngac-platform/testutil"
)

// scratchTenant provisions a tenant schema of its own (the first 8 characters of
// the id name it) and returns a context scoped to it. The schema is dropped when
// the test ends.
func scratchTenant(t *testing.T) (context.Context, *store.Store, *pgxpool.Pool) {
	t.Helper()
	pool := testutil.SetupTestDB(t)
	owner, _ := testutil.CreateUser(t, pool)
	bg := context.Background()
	id := "z" + strings.ReplaceAll(uuid.NewString(), "-", "")[:7]
	if _, err := pool.Exec(bg, `INSERT INTO workspaces (id, name, owner_id) VALUES ($1, 'scratch', $2)`, id, owner); err != nil {
		t.Fatalf("workspace: %v", err)
	}
	var schema string
	if err := pool.QueryRow(bg, `SELECT provision_tenant_schema($1)`, id).Scan(&schema); err != nil {
		t.Fatalf("provision schema: %v", err)
	}
	t.Cleanup(func() {
		pool.Exec(bg, fmt.Sprintf(`DROP SCHEMA IF EXISTS %q CASCADE`, schema))
		pool.Exec(bg, `DELETE FROM tenant_schemas WHERE tenant_id = $1`, id)
		pool.Exec(bg, `DELETE FROM workspaces WHERE id = $1`, id)
	})
	return httputil.WithTenantSchema(bg, schema), store.NewStore(pool), pool
}

func newRequest(tmplID string) (*domain.Request, *domain.AssignmentRecord) {
	req := &domain.Request{
		ID: uuid.NewString(), EntityType: "expense", EntityID: uuid.NewString(), TemplateID: tmplID, TemplateName: "T",
		TemplateSnapshot: `{"steps":[]}`, CurrentStep: 1, Status: "pending", ScopeOAID: "oa", DepartmentID: "d", CreatedBy: "u", CreatedAt: time.Now(),
	}
	a := &domain.AssignmentRecord{ID: uuid.NewString(), RequestID: req.ID, StepOrder: 1, UserNodeID: "approver", GrantSource: "direct", Status: "pending"}
	return req, a
}

func insertTemplate(t *testing.T, ctx context.Context, st *store.Store) string {
	t.Helper()
	tmpl := &domain.Template{ID: uuid.NewString(), Name: "T", EntityType: "expense", IsActive: true, CreatedBy: "u", CreatedAt: time.Now(), UpdatedAt: time.Now(),
		Steps: []*domain.Step{{ID: uuid.NewString(), StepOrder: 1, Name: "S", ApproverType: "specific_user", ApproverValue: "approver", RequiredCount: 1}}}
	if err := st.InsertTemplate(ctx, tmpl); err != nil {
		t.Fatalf("insert template: %v", err)
	}
	return tmpl.ID
}

// Everything written inside InTx stands or falls together: when fn fails, the
// request, its assignment and the audit entry written before the failure are gone.
func TestInTx_RollsBackEverythingWhenTheFunctionFails(t *testing.T) {
	ctx, st, _ := scratchTenant(t)
	req, a := newRequest(insertTemplate(t, ctx, st))
	boom := errors.New("later step failed")

	err := st.InTx(ctx, func(ctx context.Context) error {
		if err := st.InsertRequestWithAssignments(ctx, req, []*domain.AssignmentRecord{a}); err != nil {
			return err
		}
		if err := st.InsertAuditEntry(ctx, &domain.AuditEntry{ID: uuid.NewString(), RequestID: req.ID, Action: "created", CreatedAt: time.Now()}); err != nil {
			return err
		}
		return boom
	})

	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want the function's error", err)
	}
	if _, err := st.GetRequest(ctx, req.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("the request survived the rollback: %v", err)
	}
	if rows, _ := st.ListAssignments(ctx, req.ID); len(rows) != 0 {
		t.Errorf("%d assignments survived", len(rows))
	}
	if entries, _ := st.ListAuditEntries(ctx, req.ID); len(entries) != 0 {
		t.Errorf("%d audit entries survived", len(entries))
	}
}

func TestInTx_CommitsEverythingWhenTheFunctionSucceeds(t *testing.T) {
	ctx, st, _ := scratchTenant(t)
	req, a := newRequest(insertTemplate(t, ctx, st))

	err := st.InTx(ctx, func(ctx context.Context) error {
		if err := st.InsertRequestWithAssignments(ctx, req, []*domain.AssignmentRecord{a}); err != nil {
			return err
		}
		return st.InsertAuditEntry(ctx, &domain.AuditEntry{ID: uuid.NewString(), RequestID: req.ID, Action: "created", CreatedAt: time.Now()})
	})

	if err != nil {
		t.Fatal(err)
	}
	if got, err := st.GetRequest(ctx, req.ID); err != nil || got.Status != "pending" {
		t.Errorf("request: %v %v", got, err)
	}
	if entries, _ := st.ListAuditEntries(ctx, req.ID); len(entries) != 1 {
		t.Errorf("%d audit entries, want 1", len(entries))
	}
}

// A failed statement inside the change is not papered over by what follows it:
// the whole change is refused, including what was written before the failure.
func TestInTx_AFailingStatementRollsBackTheEarlierOnes(t *testing.T) {
	ctx, st, _ := scratchTenant(t)
	req, a := newRequest(insertTemplate(t, ctx, st))
	twin := *a
	twin.ID = uuid.NewString() // same request, step and person: the unique index refuses it

	err := st.InTx(ctx, func(ctx context.Context) error {
		if err := st.InsertRequestWithAssignments(ctx, req, []*domain.AssignmentRecord{a}); err != nil {
			return err
		}
		acted := twin
		acted.Status = "approved"
		if err := st.InsertActedAssignment(ctx, &acted); err != nil {
			return err
		}
		return st.InsertActedAssignment(ctx, &acted) // the duplicate
	})

	if !errors.Is(err, domain.ErrAlreadyExists) {
		t.Fatalf("err = %v, want ErrAlreadyExists", err)
	}
	if _, err := st.GetRequest(ctx, req.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("the request survived: %v", err)
	}
}

// Called inside a change, InTx joins it rather than starting a second one.
func TestInTx_NestedCallJoinsTheOuterChange(t *testing.T) {
	ctx, st, _ := scratchTenant(t)
	req, a := newRequest(insertTemplate(t, ctx, st))
	boom := errors.New("outer failed")

	err := st.InTx(ctx, func(ctx context.Context) error {
		if err := st.InTx(ctx, func(ctx context.Context) error {
			return st.InsertRequestWithAssignments(ctx, req, []*domain.AssignmentRecord{a})
		}); err != nil {
			return err
		}
		return boom
	})

	if !errors.Is(err, boom) {
		t.Fatalf("err = %v", err)
	}
	if _, err := st.GetRequest(ctx, req.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("the inner write outlived the outer failure: %v", err)
	}
}

func TestInTx_NeedsATenant(t *testing.T) {
	st := store.NewStore(testutil.SetupTestDB(t))
	if err := st.InTx(context.Background(), func(context.Context) error { return nil }); err == nil {
		t.Error("a change with no tenant schema must be refused")
	}
}
