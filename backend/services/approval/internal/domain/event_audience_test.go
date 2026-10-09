package domain

import (
	"context"
	"reflect"
	"testing"
)

// The audience of an approval event is who needs to hear about it live: the
// requester, and whoever must act next. These tests pin both halves.

func TestEventAudience_NamesRequesterAndCurrentApprovers(t *testing.T) {
	svc, ms, _ := setupServiceWithTemplate()
	ctx := context.Background()
	req, err := svc.CreateApprovalRequest(ctx, CreateRequestInput{
		EntityType: "transfer", EntityID: "txn-aud-1", EntityFields: EntityFields{},
		ScopeOAID: "oa_dept1", DepartmentID: "dept1", CreatedBy: "requester",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	aud, err := svc.EventAudience(ctx, req.ID)
	if err != nil {
		t.Fatalf("audience after create: %v", err)
	}
	if aud.Request.CreatedBy != "requester" {
		t.Errorf("created_by = %q, want requester", aud.Request.CreatedBy)
	}
	if !reflect.DeepEqual(aud.AssigneeNodeIDs, []string{"approver1"}) {
		t.Errorf("assignees after create = %v, want [approver1]", aud.AssigneeNodeIDs)
	}

	// Step 1 approved: the audience moves on to the step-2 approver.
	ms.approved = 1
	if err := svc.Approve(ctx, ApproveInput{RequestID: req.ID, UserNodeID: "approver1"}); err != nil {
		t.Fatalf("approve: %v", err)
	}
	aud, err = svc.EventAudience(ctx, req.ID)
	if err != nil {
		t.Fatalf("audience after approve: %v", err)
	}
	if !reflect.DeepEqual(aud.AssigneeNodeIDs, []string{"approver2"}) {
		t.Errorf("assignees after step 1 = %v, want [approver2] (the next approver)", aud.AssigneeNodeIDs)
	}
	if aud.Request.Status != "pending" {
		t.Errorf("status = %q, want pending", aud.Request.Status)
	}
}

// Once a request is terminal nobody is waiting to act on it, so no approver
// is named — only the requester (and the actor, added by the caller).
func TestEventAudience_TerminalRequestNamesNoApprovers(t *testing.T) {
	svc, ms, _ := setupServiceWithTemplate()
	ctx := context.Background()
	req, err := svc.CreateApprovalRequest(ctx, CreateRequestInput{
		EntityType: "transfer", EntityID: "txn-aud-2", EntityFields: EntityFields{},
		ScopeOAID: "oa_dept1", DepartmentID: "dept1", CreatedBy: "requester",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	// Leave a pending row behind (the mock does not skip on reject) to prove
	// the terminal status, not the row state, decides.
	ms.assignList = append(ms.assignList, &AssignmentRecord{
		ID: "stale", RequestID: req.ID, StepOrder: 1, UserNodeID: "stale-approver", Status: "pending",
	})
	if err := svc.Reject(ctx, RejectInput{RequestID: req.ID, UserNodeID: "approver1", Comment: "no"}); err != nil {
		t.Fatalf("reject: %v", err)
	}

	aud, err := svc.EventAudience(ctx, req.ID)
	if err != nil {
		t.Fatalf("audience: %v", err)
	}
	if aud.Request.Status != "rejected" {
		t.Errorf("status = %q, want rejected", aud.Request.Status)
	}
	if aud.Request.CreatedBy != "requester" {
		t.Errorf("created_by = %q, want requester", aud.Request.CreatedBy)
	}
	if len(aud.AssigneeNodeIDs) != 0 {
		t.Errorf("assignees on a rejected request = %v, want none", aud.AssigneeNodeIDs)
	}
}

func TestEventAudience_UnknownRequest(t *testing.T) {
	svc, _, _ := setupServiceWithTemplate()
	if _, err := svc.EventAudience(context.Background(), "no-such-request"); err == nil {
		t.Fatal("an unknown request must not yield an audience")
	}
	if _, err := svc.EventAudience(context.Background(), ""); err == nil {
		t.Fatal("an empty request id must not yield an audience")
	}
}
