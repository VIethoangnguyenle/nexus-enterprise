package domain

import (
	"context"
	"errors"
	"testing"
)

// A step only the requester could decide is skipped by the system, recorded on
// the trail; a step anyone else can decide is left to them; and the requester
// never decides their own request either way.

func skipEntries(ms *mockStore, requestID string) []*AuditEntry {
	var out []*AuditEntry
	for _, e := range ms.auditLog {
		if e.RequestID == requestID && e.Action == "step_skipped" {
			out = append(out, e)
		}
	}
	return out
}

func requireSkipAudit(t *testing.T, ms *mockStore, requestID string, step int) {
	t.Helper()
	for _, e := range skipEntries(ms, requestID) {
		if e.StepOrder == step {
			if e.ActorNodeID != "" {
				t.Errorf("skip of step %d has actor %q, want the system (none)", step, e.ActorNodeID)
			}
			if e.DetailJSON != `{"reason":"`+soleApproverSkipReason+`"}` {
				t.Errorf("skip detail = %s", e.DetailJSON)
			}
			return
		}
	}
	t.Fatalf("no step_skipped entry for step %d", step)
}

func approvedRowsBy(ms *mockStore, requestID, who string) int {
	n := 0
	for _, a := range ms.assignList {
		if a.RequestID == requestID && a.UserNodeID == who && a.Status == "approved" {
			n++
		}
	}
	return n
}

// Step 1 names the requester in person: it is skipped and step 2 starts.
func TestSoleApprover_PersonStepNamingTheRequesterIsSkipped(t *testing.T) {
	svc, ms, _ := setupServiceWithTemplate()
	req, err := svc.CreateApprovalRequest(tenantCtx(), CreateRequestInput{EntityType: "transfer", CreatedBy: "approver1"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if req.CurrentStep != 2 || req.Status != "pending" {
		t.Fatalf("request = %s on step %d, want pending on step 2", req.Status, req.CurrentStep)
	}
	requireSkipAudit(t, ms, req.ID, 1)
	for _, a := range ms.assignList {
		if a.RequestID == req.ID && a.StepOrder == 1 {
			t.Errorf("the skipped step has a row: %+v", a)
		}
	}
	// The requester still cannot act, on the step that is running or at all.
	if err := svc.Approve(context.Background(), ApproveInput{RequestID: req.ID, UserNodeID: "approver1"}); !errors.Is(err, ErrAccessDenied) {
		t.Fatalf("requester approve err = %v, want ErrAccessDenied", err)
	}
	if err := svc.Approve(context.Background(), ApproveInput{RequestID: req.ID, UserNodeID: "approver2"}); err != nil {
		t.Fatal(err)
	}
	if ms.requests[req.ID].Status != "approved" {
		t.Errorf("status = %s, want approved", ms.requests[req.ID].Status)
	}
	if n := approvedRowsBy(ms, req.ID, "approver1"); n != 0 {
		t.Errorf("the skip counted as %d approvals by the requester", n)
	}
}

// The last step names the requester: once step 1 is approved the request
// completes as it would after the final approval.
func TestSoleApprover_SkippingTheLastStepCompletesTheRequest(t *testing.T) {
	svc, ms, _ := setupServiceWithTemplate()
	req, err := svc.CreateApprovalRequest(tenantCtx(), CreateRequestInput{EntityType: "transfer", CreatedBy: "approver2"})
	if err != nil {
		t.Fatal(err)
	}
	if req.CurrentStep != 1 {
		t.Fatalf("step = %d, want 1 (someone else decides it)", req.CurrentStep)
	}
	if err := svc.Approve(tenantCtx(), ApproveInput{RequestID: req.ID, UserNodeID: "approver1"}); err != nil {
		t.Fatal(err)
	}
	got := ms.requests[req.ID]
	if got.Status != "approved" {
		t.Fatalf("status = %s, want approved", got.Status)
	}
	requireSkipAudit(t, ms, req.ID, 2)
	completed := false
	for _, e := range ms.auditLog {
		if e.RequestID == req.ID && e.Action == "completed" && e.DetailJSON == `{"final_status":"approved"}` {
			completed = true
		}
	}
	if !completed {
		t.Error("no completed entry with final_status approved")
	}
	if n := approvedRowsBy(ms, req.ID, "approver2"); n != 0 {
		t.Errorf("the requester has %d approved rows", n)
	}
}

func groupSkipService(members map[string][]string) (*Service, *mockStore) {
	ms := newMockStore()
	ms.groupMembers = members
	ms.templates = []*Template{{
		ID: "t", Name: "Group", EntityType: "x", IsActive: true,
		Steps: []*Step{
			{StepOrder: 1, ApproverType: ApproverRole, ApproverValue: "ua-role", RequiredCount: 1},
			{StepOrder: 2, ApproverType: ApproverSpecificUser, ApproverValue: "boss", RequiredCount: 1},
		},
	}}
	mp := &mockPolicy{ancestors: map[string][]string{"requester": {"ua-role"}, "alice": {"ua-role"}}}
	return NewService(ms, mp), ms
}

func TestSoleApprover_RoleWhoseOnlyMemberIsTheRequesterIsSkipped(t *testing.T) {
	svc, ms := groupSkipService(map[string][]string{"ua-role": {"requester"}})
	req, err := svc.CreateApprovalRequest(tenantCtx(), CreateRequestInput{TemplateID: "t", CreatedBy: "requester"})
	if err != nil {
		t.Fatal(err)
	}
	if req.CurrentStep != 2 {
		t.Fatalf("step = %d, want 2", req.CurrentStep)
	}
	requireSkipAudit(t, ms, req.ID, 1)
	if err := svc.Approve(context.Background(), ApproveInput{RequestID: req.ID, UserNodeID: "requester"}); !errors.Is(err, ErrAccessDenied) {
		t.Fatalf("requester approve err = %v, want ErrAccessDenied", err)
	}
}

func TestSoleApprover_RoleWithAnotherMemberIsNotSkipped(t *testing.T) {
	for name, members := range map[string][]string{
		"requester and alice":              {"requester", "alice"},
		"only alice":                       {"alice"},
		"nobody yet (requester not in it)": nil,
	} {
		svc, ms := groupSkipService(map[string][]string{"ua-role": members})
		req, err := svc.CreateApprovalRequest(tenantCtx(), CreateRequestInput{TemplateID: "t", CreatedBy: "requester"})
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if req.CurrentStep != 1 || len(skipEntries(ms, req.ID)) != 0 {
			t.Errorf("%s: step = %d, skips = %d; want step 1 left to the others", name, req.CurrentStep, len(skipEntries(ms, req.ID)))
		}
		if err := svc.Approve(context.Background(), ApproveInput{RequestID: req.ID, UserNodeID: "requester"}); !errors.Is(err, ErrAccessDenied) {
			t.Errorf("%s: requester approve err = %v, want ErrAccessDenied", name, err)
		}
	}
	svc, ms := groupSkipService(map[string][]string{"ua-role": {"requester", "alice"}})
	req, _ := svc.CreateApprovalRequest(tenantCtx(), CreateRequestInput{TemplateID: "t", CreatedBy: "requester"})
	if err := svc.Approve(context.Background(), ApproveInput{RequestID: req.ID, UserNodeID: "alice"}); err != nil {
		t.Fatal(err)
	}
	if ms.requests[req.ID].CurrentStep != 2 {
		t.Errorf("alice's approval did not advance the step")
	}
}

// Every step only the requester could decide: the request is approved as it is made.
func TestSoleApprover_EveryStepSkippedCompletesOnCreation(t *testing.T) {
	ms := newMockStore()
	ms.groupMembers = map[string][]string{"ua-role": {"requester"}}
	ms.templates = []*Template{{
		ID: "t", EntityType: "x", IsActive: true,
		Steps: []*Step{
			{StepOrder: 1, ApproverType: ApproverRole, ApproverValue: "ua-role", RequiredCount: 1},
			{StepOrder: 2, ApproverType: ApproverSpecificUser, ApproverValue: "requester", RequiredCount: 1},
		},
	}}
	svc := NewService(ms, &mockPolicy{})
	req, err := svc.CreateApprovalRequest(tenantCtx(), CreateRequestInput{TemplateID: "t", CreatedBy: "requester"})
	if err != nil {
		t.Fatal(err)
	}
	if req.Status != "approved" {
		t.Fatalf("status = %s, want approved", req.Status)
	}
	requireSkipAudit(t, ms, req.ID, 1)
	requireSkipAudit(t, ms, req.ID, 2)
	if len(ms.assignList) != 0 {
		t.Errorf("rows = %d, want none: nobody approved", len(ms.assignList))
	}
}

// Membership that cannot be read is never a reason to skip.
func TestSoleApprover_UnreadableMembershipRefusesRatherThanSkips(t *testing.T) {
	svc, ms := groupSkipService(map[string][]string{"ua-role": {"requester"}})
	ms.membersErr = errors.New("db down")
	if _, err := svc.CreateApprovalRequest(tenantCtx(), CreateRequestInput{TemplateID: "t", CreatedBy: "requester"}); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("err = %v, want ErrUnavailable", err)
	}
	if len(ms.requests) != 0 {
		t.Errorf("%d requests stored", len(ms.requests))
	}
}

// Requests stuck before skipping existed are unstuck by the start-up sweep,
// once: running it again changes nothing. A step someone else can decide, or
// on which someone has acted, is left alone.
func TestReconcile_UnsticksRequestsWaitingOnlyOnTheirRequester(t *testing.T) {
	svc, ms, _ := setupServiceWithTemplate()
	ms.tenants = []TenantSchema{{TenantID: "tenant-1", Schema: "tenant_1"}}
	stuck := seedStuck(t, svc, ms, "approver1", nil)
	fine := seedStuck(t, svc, ms, "someone", nil)
	acted := seedStuck(t, svc, ms, "approver1", &AssignmentRecord{StepOrder: 1, UserNodeID: "x", GrantSource: "direct", Status: "approved"})

	n, err := svc.ReconcileSoleApproverSteps(context.Background())
	if err != nil || n != 1 {
		t.Fatalf("reconcile = %d, %v; want 1", n, err)
	}
	if got := ms.requests[stuck.ID]; got.CurrentStep != 2 || got.Status != "pending" {
		t.Errorf("stuck request = %s on step %d, want pending on step 2", got.Status, got.CurrentStep)
	}
	requireSkipAudit(t, ms, stuck.ID, 1)
	if a := ms.assignments[stuck.ID+":approver2"]; a == nil || a.Status != "pending" {
		t.Errorf("step 2 not assigned: %+v", a)
	}
	for _, id := range []string{fine.ID, acted.ID} {
		if ms.requests[id].CurrentStep != 1 || len(skipEntries(ms, id)) != 0 {
			t.Errorf("request %s was touched", id)
		}
	}
	if n, err := svc.ReconcileSoleApproverSteps(context.Background()); err != nil || n != 0 {
		t.Errorf("second run = %d, %v; want 0", n, err)
	}
	if len(skipEntries(ms, stuck.ID)) != 1 {
		t.Errorf("skip recorded %d times", len(skipEntries(ms, stuck.ID)))
	}
}

// seedStuck stores a request as one made before skipping existed: on step 1,
// with step 1's row naming approver1, created by creator. extra, if set, is
// another row on step 1.
func seedStuck(t *testing.T, svc *Service, ms *mockStore, creator string, extra *AssignmentRecord) *Request {
	t.Helper()
	req, err := svc.CreateApprovalRequest(tenantCtx(), CreateRequestInput{EntityType: "transfer", CreatedBy: "seed-" + creator})
	if err != nil {
		t.Fatal(err)
	}
	req.CreatedBy = creator
	if extra != nil {
		extra.ID, extra.RequestID = "extra-"+req.ID, req.ID
		ms.assignList = append(ms.assignList, extra)
	}
	return req
}
