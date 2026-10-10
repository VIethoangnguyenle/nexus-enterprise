package domain

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// flakyStore is the mock store with writes that can be made to fail part way
// through an operation, to show what the operation leaves behind.
type flakyStore struct {
	*mockStore
	failAudit             string // an audit action whose entry cannot be appended
	failInsertAssignments bool
}

func (f flakyStore) InsertAuditEntry(ctx context.Context, e *AuditEntry) error {
	if f.failAudit != "" && e.Action == f.failAudit {
		return errors.New(`ERROR: deadlock detected (SQLSTATE 40P01)`)
	}
	return f.mockStore.InsertAuditEntry(ctx, e)
}

func (f flakyStore) InsertAssignments(ctx context.Context, rows []*AssignmentRecord) error {
	if f.failInsertAssignments {
		return errors.New(`ERROR: connection reset by peer`)
	}
	return f.mockStore.InsertAssignments(ctx, rows)
}

// newRequest creates a request on the healthy store, then swaps in the flaky one,
// so the failure under test is the only thing that goes wrong.
func newRequest(t *testing.T, failAudit string, failAssignments bool) (*Service, *mockStore, *Request) {
	t.Helper()
	svc, ms, _ := setupServiceWithTemplate()
	req, err := svc.CreateApprovalRequest(context.Background(), CreateRequestInput{
		EntityType: "transfer", EntityID: "txn-x", ScopeOAID: "oa_dept1", DepartmentID: "dept1", CreatedBy: "user1",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	svc.store = flakyStore{mockStore: ms, failAudit: failAudit, failInsertAssignments: failAssignments}
	return svc, ms, req
}

func assertUntouched(t *testing.T, ms *mockStore, req *Request, auditBefore int) {
	t.Helper()
	got := ms.requests[req.ID]
	if got.Status != "pending" || got.CurrentStep != 1 {
		t.Errorf("request is %q at step %d, want pending at step 1", got.Status, got.CurrentStep)
	}
	if a := ms.assignments[req.ID+":approver1"]; a.Status != "pending" {
		t.Errorf("the approver's decision stands (%q) although the operation failed", a.Status)
	}
	if _, advanced := ms.assignments[req.ID+":approver2"]; advanced {
		t.Error("the next step has assignments although the step did not advance")
	}
	if len(ms.auditLog) != auditBefore {
		t.Errorf("audit trail grew by %d entries for an operation that failed", len(ms.auditLog)-auditBefore)
	}
}

// An approval that cannot be audited does not happen: nothing of it is kept.
func TestApprove_AFailedAuditLeavesNoDecision(t *testing.T) {
	svc, ms, req := newRequest(t, "approved", false)
	before := len(ms.auditLog)

	err := svc.Approve(context.Background(), ApproveInput{RequestID: req.ID, UserNodeID: "approver1"})

	if err == nil || !strings.Contains(err.Error(), "audit approved") {
		t.Fatalf("err = %v, want the audit failure", err)
	}
	assertUntouched(t, ms, req, before)
}

// The step advances to its approvers or not at all: a request is never left on
// a step that nobody was assigned to.
func TestApprove_AFailedNextStepAssignmentLeavesTheRequestWhereItWas(t *testing.T) {
	svc, ms, req := newRequest(t, "", true)
	ms.approved = 1 // the quorum of step 1 is met by this approval
	before := len(ms.auditLog)

	err := svc.Approve(context.Background(), ApproveInput{RequestID: req.ID, UserNodeID: "approver1"})

	if err == nil {
		t.Fatal("the failure must be reported")
	}
	assertUntouched(t, ms, req, before)
}

func TestApprove_AFailureWhileAdvancingLeavesNoHalfStep(t *testing.T) {
	svc, ms, req := newRequest(t, "step_advanced", false)
	ms.approved = 1
	before := len(ms.auditLog)

	if err := svc.Approve(context.Background(), ApproveInput{RequestID: req.ID, UserNodeID: "approver1"}); err == nil {
		t.Fatal("the failure must be reported")
	}
	assertUntouched(t, ms, req, before)
}

// A rejection that cannot be completed does not close the request halfway.
func TestReject_AFailedAuditLeavesTheRequestOpen(t *testing.T) {
	svc, ms, req := newRequest(t, "completed", false)
	before := len(ms.auditLog)

	err := svc.Reject(context.Background(), RejectInput{RequestID: req.ID, UserNodeID: "approver1", Comment: "no"})

	if err == nil {
		t.Fatal("the failure must be reported")
	}
	assertUntouched(t, ms, req, before)
}

// A request and its audit trail exist together or not at all.
func TestCreateApprovalRequest_AFailedAuditStoresNothing(t *testing.T) {
	svc, ms, _ := setupServiceWithTemplate()
	svc.store = flakyStore{mockStore: ms, failAudit: "assigned"}

	_, err := svc.CreateApprovalRequest(context.Background(), CreateRequestInput{
		EntityType: "transfer", EntityID: "txn-y", ScopeOAID: "oa_dept1", DepartmentID: "dept1", CreatedBy: "user1",
	})

	if err == nil {
		t.Fatal("the failure must be reported")
	}
	if len(ms.requests) != 0 || len(ms.assignments) != 0 || len(ms.auditLog) != 0 {
		t.Errorf("left behind: %d requests, %d assignments, %d audit entries", len(ms.requests), len(ms.assignments), len(ms.auditLog))
	}
}

func TestLogAudit_ReportsAFailedAppend(t *testing.T) {
	svc, _, _ := setupServiceWithTemplate()
	svc.store = flakyStore{mockStore: newMockStore(), failAudit: "approved"}

	err := svc.logAudit(context.Background(), "req-1", "approved", "node-hoa", 2, map[string]string{"comment": "ok"})

	if err == nil || !strings.Contains(err.Error(), "audit approved") {
		t.Fatalf("err = %v", err)
	}
	if err := svc.logAudit(context.Background(), "req-1", "created", "node-hoa", 0, nil); err != nil {
		t.Errorf("an append that works is not an error: %v", err)
	}
}
