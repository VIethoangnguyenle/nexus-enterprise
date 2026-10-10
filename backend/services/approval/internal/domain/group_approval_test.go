package domain

import (
	"context"
	"errors"
	"testing"
)

// groupFixture: one template with a single step approved by the role "ua-role",
// quorum `need`. Alice and Bob belong to the role; Carol belongs to another
// department; Dave belongs to nothing. "requester" made the request.
func groupFixture(t *testing.T, need int) (*Service, *mockStore, *mockPolicy, *Request) {
	t.Helper()
	ms := newMockStore()
	mp := &mockPolicy{
		ancestors: map[string][]string{
			"alice": {"ua-role", "pc"}, "bob": {"ua-role", "pc"}, "carol": {"ua-other", "pc"}, "dave": nil,
		},
		members: map[string][]string{"ua-role": {"alice", "bob"}},
	}
	ms.templates = append(ms.templates, &Template{
		ID: "tmpl-g", Name: "Group", EntityType: "expense", IsActive: true,
		Steps: []*Step{
			{StepOrder: 1, Name: "Role", ApproverType: ApproverRole, ApproverValue: "ua-role", RequiredCount: need},
			{StepOrder: 2, Name: "Last", ApproverType: ApproverSpecificUser, ApproverValue: "boss", RequiredCount: 1},
		},
	})
	svc := NewService(ms, mp)
	req, err := svc.CreateApprovalRequest(context.Background(), CreateRequestInput{
		TemplateID: "tmpl-g", CreatedBy: "requester",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	return svc, ms, mp, req
}

func approveAs(svc *Service, req *Request, who string) error {
	return svc.Approve(context.Background(), ApproveInput{RequestID: req.ID, UserNodeID: who})
}

func TestGroupStep_StoresOneRowForTheRole(t *testing.T) {
	_, ms, _, req := groupFixture(t, 1)
	rows, _ := ms.ListAssignments(context.Background(), req.ID)
	if len(rows) != 1 || rows[0].UserNodeID != "ua-role" || rows[0].GrantSource != "role:ua-role" || rows[0].Status != "pending" {
		t.Fatalf("rows = %+v, want one pending group row for the role", rows)
	}
}

func TestGroupStep_DepartmentGrantSource(t *testing.T) {
	rows, err := buildAssignments("r1", &Step{StepOrder: 1, ApproverType: ApproverDepartment, ApproverValue: "ua-dept"}, "")
	if err != nil || len(rows) != 1 || rows[0].GrantSource != "department:ua-dept" || rows[0].UserNodeID != "ua-dept" {
		t.Fatalf("rows=%+v err=%v", rows, err)
	}
}

func TestGroupApproval_MemberApprovesAndTheRequestMovesOn(t *testing.T) {
	svc, ms, _, req := groupFixture(t, 1)
	if err := approveAs(svc, req, "alice"); err != nil {
		t.Fatalf("member approve: %v", err)
	}
	if ms.requests[req.ID].CurrentStep != 2 {
		t.Errorf("step = %d, want 2", ms.requests[req.ID].CurrentStep)
	}
	rows, _ := ms.ListAssignments(context.Background(), req.ID)
	var mine *AssignmentRecord
	for _, r := range rows {
		if r.UserNodeID == "alice" {
			mine = r
		}
	}
	if mine == nil || mine.Status != "approved" || mine.GrantSource != "role:ua-role" || mine.ActedAt == nil {
		t.Errorf("the member's own row = %+v, want approved under the same grant with a time", mine)
	}
}

func TestGroupApproval_DeniesWhoIsNotInTheRole(t *testing.T) {
	svc, ms, _, req := groupFixture(t, 1)
	for _, who := range []string{"dave", "carol", "stranger"} { // nobody / other department / unknown
		if err := approveAs(svc, req, who); !errors.Is(err, ErrAccessDenied) {
			t.Errorf("%s: err = %v, want ErrAccessDenied", who, err)
		}
	}
	if err := svc.Reject(context.Background(), RejectInput{RequestID: req.ID, UserNodeID: "carol", Comment: "no"}); !errors.Is(err, ErrAccessDenied) {
		t.Errorf("reject by another department: err = %v, want ErrAccessDenied", err)
	}
	rows, _ := ms.ListAssignments(context.Background(), req.ID)
	if len(rows) != 1 || ms.requests[req.ID].Status != "pending" {
		t.Errorf("a denied caller changed something: rows=%d status=%s", len(rows), ms.requests[req.ID].Status)
	}
}

func TestGroupApproval_DeniesAMemberRemovedSinceTheRequestWasMade(t *testing.T) {
	svc, ms, mp, req := groupFixture(t, 1)
	mp.ancestors["alice"] = []string{"pc"} // removed from the role
	if err := approveAs(svc, req, "alice"); !errors.Is(err, ErrAccessDenied) {
		t.Fatalf("err = %v, want ErrAccessDenied", err)
	}
	if rows, _ := ms.ListAssignments(context.Background(), req.ID); len(rows) != 1 {
		t.Errorf("a removed member left a row: %+v", rows)
	}
}

func TestGroupApproval_OnePersonCannotActTwice(t *testing.T) {
	svc, ms, _, req := groupFixture(t, 2)
	if err := approveAs(svc, req, "alice"); err != nil {
		t.Fatalf("first: %v", err)
	}
	if err := approveAs(svc, req, "alice"); !errors.Is(err, ErrAlreadyExists) {
		t.Fatalf("second approval: err = %v, want ErrAlreadyExists", err)
	}
	// Nor can she switch to a rejection.
	if err := svc.Reject(context.Background(), RejectInput{RequestID: req.ID, UserNodeID: "alice", Comment: "x"}); !errors.Is(err, ErrAlreadyExists) {
		t.Fatalf("reject after approve: err = %v, want ErrAlreadyExists", err)
	}
	if got := ms.requests[req.ID]; got.CurrentStep != 1 || got.Status != "pending" {
		t.Errorf("one person met a quorum of two: step=%d status=%s", got.CurrentStep, got.Status)
	}
	if err := approveAs(svc, req, "bob"); err != nil {
		t.Fatalf("second member: %v", err)
	}
	if ms.requests[req.ID].CurrentStep != 2 {
		t.Errorf("two different members should meet the quorum, step=%d", ms.requests[req.ID].CurrentStep)
	}
}

func TestGroupApproval_RejectByAMemberEndsTheRequest(t *testing.T) {
	svc, ms, _, req := groupFixture(t, 1)
	if err := svc.Reject(context.Background(), RejectInput{RequestID: req.ID, UserNodeID: "bob", Comment: "Thiếu hoá đơn"}); err != nil {
		t.Fatalf("reject: %v", err)
	}
	if ms.requests[req.ID].Status != "rejected" {
		t.Errorf("status = %s", ms.requests[req.ID].Status)
	}
	rows, _ := ms.ListAssignments(context.Background(), req.ID)
	found := false
	for _, r := range rows {
		if r.UserNodeID == "bob" && r.Status == "rejected" && r.Comment == "Thiếu hoá đơn" {
			found = true
		}
	}
	if !found {
		t.Errorf("the rejecting member has no row of their own: %+v", rows)
	}
}

// A named approver still acts through their own row, and a non-direct row of
// their own is only honoured while they belong to its group.
func TestDirectRowFromAGroupNeedsMembershipNow(t *testing.T) {
	svc, ms, mp, req := groupFixture(t, 1)
	now := ms.assignList[0]
	ms.assignList = append(ms.assignList, &AssignmentRecord{
		ID: "own", RequestID: req.ID, StepOrder: 1, UserNodeID: "bob", GrantSource: "role:ua-role", Status: "pending",
	})
	ms.assignments[req.ID+":bob"] = ms.assignList[1]
	_ = now
	mp.ancestors["bob"] = nil
	if err := approveAs(svc, req, "bob"); !errors.Is(err, ErrAccessDenied) {
		t.Fatalf("err = %v, want ErrAccessDenied", err)
	}
}

func TestGroupVisibility_PendingListAuditAndDetail(t *testing.T) {
	svc, _, _, req := groupFixture(t, 2)
	ctx := context.Background()

	items, err := svc.GetPending(ctx, "alice")
	if err != nil || len(items) != 1 || items[0].Request.ID != req.ID {
		t.Fatalf("member pending = %+v err=%v", items, err)
	}
	for _, who := range []string{"carol", "dave"} {
		if items, _ := svc.GetPending(ctx, who); len(items) != 0 {
			t.Errorf("%s sees %d pending requests of a role they are not in", who, len(items))
		}
	}

	if _, err := svc.GetAuditLog(ctx, "alice", req.ID); err != nil {
		t.Errorf("member may read the trail: %v", err)
	}
	if _, err := svc.GetAuditLog(ctx, "carol", req.ID); !errors.Is(err, ErrAccessDenied) {
		t.Errorf("other department reading the trail: err = %v, want ErrAccessDenied", err)
	}

	d, err := svc.GetRequestDetail(ctx, "alice", req.ID)
	if err != nil || !d.CanAct {
		t.Fatalf("member detail can_act = %v err=%v", d != nil && d.CanAct, err)
	}
	if err := approveAs(svc, req, "alice"); err != nil {
		t.Fatal(err)
	}
	// Having acted, she can still read it but it is no longer her turn, and it
	// leaves her pending list although the group row stays open for Bob.
	d, _ = svc.GetRequestDetail(ctx, "alice", req.ID)
	if d.CanAct {
		t.Error("can_act stays true after the member has acted")
	}
	if items, _ := svc.GetPending(ctx, "alice"); len(items) != 0 {
		t.Errorf("acted request still pending for her: %d", len(items))
	}
	if items, _ := svc.GetPending(ctx, "bob"); len(items) != 1 {
		t.Errorf("the other member lost it: %d", len(items))
	}
	if d, err := svc.GetRequestDetail(ctx, "carol", req.ID); !errors.Is(err, ErrAccessDenied) || d != nil {
		t.Errorf("other department opening it: detail=%v err=%v", d, err)
	}
	if d, _ := svc.GetRequestDetail(ctx, "requester", req.ID); d.CanAct {
		t.Error("the requester is not an approver")
	}
}

func TestGroupVisibility_PendingFallsBackToDirectRowsWhenMembershipIsUnreadable(t *testing.T) {
	svc, _, mp, req := groupFixture(t, 1)
	mp.ancestorsErr = errors.New("policy down")
	items, err := svc.GetPending(context.Background(), "alice")
	if err != nil || len(items) != 0 {
		t.Fatalf("items=%d err=%v: a shorter list, never an error or a wider one", len(items), err)
	}
	_ = req
}

func TestGroupVisibility_PolicyFailureIsNotAGrant(t *testing.T) {
	svc, _, mp, req := groupFixture(t, 1)
	mp.ancestorsErr = errors.New("policy down")
	if err := approveAs(svc, req, "alice"); err == nil || errors.Is(err, ErrAlreadyExists) {
		t.Fatalf("err = %v, want a failure when membership cannot be read", err)
	}
	if _, err := svc.GetAuditLog(context.Background(), "alice", req.ID); err == nil {
		t.Error("the trail opened while membership was unreadable")
	}
}

func TestEventAudience_NamesThePeopleInAGroupWhoHaveNotActed(t *testing.T) {
	svc, _, _, req := groupFixture(t, 2)
	aud, err := svc.EventAudience(context.Background(), req.ID)
	if err != nil || len(aud.AssigneeNodeIDs) != 2 {
		t.Fatalf("audience = %+v err=%v, want both members", aud, err)
	}
	if err := approveAs(svc, req, "alice"); err != nil {
		t.Fatal(err)
	}
	aud, _ = svc.EventAudience(context.Background(), req.ID)
	if len(aud.AssigneeNodeIDs) != 1 || aud.AssigneeNodeIDs[0] != "bob" {
		t.Errorf("audience = %v, want only bob", aud.AssigneeNodeIDs)
	}
}

func TestCreate_NoApproverMeansNoRequest(t *testing.T) {
	ms := newMockStore()
	ms.templates = append(ms.templates, &Template{
		ID: "t", EntityType: "x", IsActive: true,
		Steps: []*Step{{StepOrder: 1, ApproverType: ApproverRole, ApproverValue: "", RequiredCount: 1}},
	})
	svc := NewService(ms, &mockPolicy{})
	if _, err := svc.CreateApprovalRequest(context.Background(), CreateRequestInput{TemplateID: "t", CreatedBy: "u"}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
	if len(ms.requests) != 0 {
		t.Errorf("a request with nobody to decide it was left behind: %d", len(ms.requests))
	}
	// A department placeholder with no department resolves to nobody too.
	ms.templates[0].Steps[0] = &Step{StepOrder: 1, ApproverType: ApproverDepartment, ApproverValue: "{creator_dept}", RequiredCount: 1}
	if _, err := svc.CreateApprovalRequest(context.Background(), CreateRequestInput{TemplateID: "t", CreatedBy: "u"}); !errors.Is(err, ErrInvalidInput) || len(ms.requests) != 0 {
		t.Fatalf("placeholder with no department: err=%v requests=%d", err, len(ms.requests))
	}
}
