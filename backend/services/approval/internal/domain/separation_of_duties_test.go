package domain

import (
	"context"
	"errors"
	"testing"

	"ngac-platform/ngac"
	"ngac-platform/pkg/grpcauth"
)

// The person who files a request never decides it, whatever route would
// otherwise make them an approver: a role or department they sit in, a
// department placeholder they could point at themselves, or a scope they pick.

// tenantCtx is a request made inside the workspace "tenant-1", which is where
// the server looks up the creator's department and checks a scope.
func tenantCtx() context.Context {
	return grpcauth.WithCaller(context.Background(), grpcauth.Caller{UserID: "u", NGACNodeID: "requester", TenantID: "tenant-1"})
}

// sodFixture: the requester and Alice both sit in the role "ua-role", which
// approves step 1. The requester belongs to department "d-own" (UA "ua-own");
// "d-other" (UA "ua-other") is a department they are not in.
func sodFixture(t *testing.T) (*Service, *mockStore, *mockPolicy, *Request) {
	t.Helper()
	ms := newMockStore()
	ms.departments = map[string]string{"d-own": "ua-own", "d-other": "ua-other"}
	ms.creatorDepts = map[string][]string{"requester": {"d-own"}}
	mp := &mockPolicy{ancestors: map[string][]string{
		"requester": {"ua-role", "ua-own", "pc"}, "alice": {"ua-role", "pc"}, "dora": {"ua-own", "pc"},
	}}
	ms.templates = append(ms.templates, &Template{
		ID: "tmpl-sod", Name: "SoD", EntityType: "expense", IsActive: true,
		Steps: []*Step{
			{StepOrder: 1, ApproverType: ApproverRole, ApproverValue: "ua-role", RequiredCount: 1},
			{StepOrder: 2, ApproverType: ApproverDepartment, ApproverValue: "{creator_dept}", RequiredCount: 1},
		},
	})
	svc := NewService(ms, mp)
	req, err := svc.CreateApprovalRequest(tenantCtx(), CreateRequestInput{TemplateID: "tmpl-sod", CreatedBy: "requester"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	return svc, ms, mp, req
}

func countActed(ms *mockStore, requestID, who string) int {
	n := 0
	for _, a := range ms.assignList {
		if a.RequestID == requestID && a.UserNodeID == who {
			n++
		}
	}
	return n
}

func TestSelfApproval_CreatorInTheApprovingRoleIsRefused(t *testing.T) {
	svc, ms, _, req := sodFixture(t)

	err := svc.Approve(context.Background(), ApproveInput{RequestID: req.ID, UserNodeID: "requester"})
	if !errors.Is(err, ErrAccessDenied) {
		t.Fatalf("self approve err = %v, want ErrAccessDenied", err)
	}
	err = svc.Reject(context.Background(), RejectInput{RequestID: req.ID, UserNodeID: "requester"})
	if !errors.Is(err, ErrAccessDenied) {
		t.Fatalf("self reject err = %v, want ErrAccessDenied", err)
	}
	if n := countActed(ms, req.ID, "requester"); n != 0 {
		t.Errorf("the requester left %d decision rows", n)
	}
	if got := ms.requests[req.ID]; got.Status != "pending" || got.CurrentStep != 1 {
		t.Errorf("request = %s step %d, want pending on step 1", got.Status, got.CurrentStep)
	}

	approved, err := svc.BatchApprove(context.Background(), BatchApproveInput{RequestIDs: []string{req.ID}, UserNodeID: "requester"})
	if err != nil || len(approved) != 0 {
		t.Errorf("batch self approve = %v, %v; want nothing approved", approved, err)
	}

	// Another member of the role still approves.
	if err := svc.Approve(context.Background(), ApproveInput{RequestID: req.ID, UserNodeID: "alice"}); err != nil {
		t.Fatalf("alice approve: %v", err)
	}
	if ms.requests[req.ID].CurrentStep != 2 {
		t.Fatalf("step = %d, want 2", ms.requests[req.ID].CurrentStep)
	}
}

func TestSelfApproval_TheRequestIsNotWaitingOnItsCreator(t *testing.T) {
	svc, _, _, req := sodFixture(t)

	pending, err := svc.GetPending(context.Background(), "requester")
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range pending {
		if p.Request.ID == req.ID {
			t.Errorf("the requester's own request is in their pending list")
		}
	}
	d, err := svc.GetRequestDetail(context.Background(), "requester", req.ID)
	if err != nil {
		t.Fatal(err)
	}
	if d.CanAct {
		t.Error("detail says the requester can act on their own request")
	}
	// Alice can.
	if d, err := svc.GetRequestDetail(context.Background(), "alice", req.ID); err != nil || !d.CanAct {
		t.Errorf("alice detail = %+v, %v; want can_act", d, err)
	}
}

// The placeholder is the creator's department as the graph holds it; the
// creator also sits in that department and still cannot take the step.
func TestCreatorDept_ResolvedOnTheServerAndNotByTheCreator(t *testing.T) {
	svc, ms, _, req := sodFixture(t)
	if req.DepartmentID != "d-own" {
		t.Errorf("department = %q, want the creator's own d-own", req.DepartmentID)
	}
	if err := svc.Approve(context.Background(), ApproveInput{RequestID: req.ID, UserNodeID: "alice"}); err != nil {
		t.Fatal(err)
	}
	var group *AssignmentRecord
	for _, a := range ms.assignList {
		if a.RequestID == req.ID && a.StepOrder == 2 {
			group = a
		}
	}
	if group == nil || group.UserNodeID != "ua-own" || group.GrantSource != "department:ua-own" {
		t.Fatalf("step 2 row = %+v, want the department ua-own", group)
	}
	if err := svc.Approve(context.Background(), ApproveInput{RequestID: req.ID, UserNodeID: "requester"}); !errors.Is(err, ErrAccessDenied) {
		t.Fatalf("self approve through own department err = %v, want ErrAccessDenied", err)
	}
	if err := svc.Approve(context.Background(), ApproveInput{RequestID: req.ID, UserNodeID: "dora"}); err != nil {
		t.Fatalf("department colleague approve: %v", err)
	}
	if ms.requests[req.ID].Status != "approved" {
		t.Errorf("status = %s, want approved", ms.requests[req.ID].Status)
	}
}

// A template saved before the placeholder was limited to department steps may
// still name it on a person step; the request is refused rather than resolving
// it to anything.
func TestCreatorDept_OnAPersonStepIsRefused(t *testing.T) {
	ms := newMockStore()
	ms.departments = map[string]string{"d-own": "ua-own"}
	ms.creatorDepts = map[string][]string{"requester": {"d-own"}}
	for _, typ := range []string{ApproverSpecificUser, ApproverRole} {
		ms.templates = []*Template{{
			ID: "t", EntityType: "x", IsActive: true,
			Steps: []*Step{{StepOrder: 1, ApproverType: typ, ApproverValue: "{creator_dept}", RequiredCount: 1}},
		}}
		svc := NewService(ms, &mockPolicy{})
		if _, err := svc.CreateApprovalRequest(tenantCtx(), CreateRequestInput{TemplateID: "t", CreatedBy: "requester"}); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("%s placeholder: err = %v, want ErrInvalidInput", typ, err)
		}
	}
	if len(ms.requests) != 0 {
		t.Errorf("%d requests stored", len(ms.requests))
	}
}

// A creator in no department, or in more than one, has no single department
// to stand for the placeholder.
func TestCreatorDept_NoSingleDepartmentIsRefused(t *testing.T) {
	for name, depts := range map[string][]string{"none": nil, "two": {"d-own", "d-other"}} {
		ms := newMockStore()
		ms.departments = map[string]string{"d-own": "ua-own", "d-other": "ua-other"}
		ms.creatorDepts = map[string][]string{"requester": depts}
		ms.templates = []*Template{{
			ID: "t", EntityType: "x", IsActive: true,
			Steps: []*Step{{StepOrder: 1, ApproverType: ApproverDepartment, ApproverValue: "{creator_dept}", RequiredCount: 1}},
		}}
		svc := NewService(ms, &mockPolicy{})
		if _, err := svc.CreateApprovalRequest(tenantCtx(), CreateRequestInput{TemplateID: "t", CreatedBy: "requester"}); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("%s: err = %v, want ErrInvalidInput", name, err)
		}
	}
}

func TestTemplate_PlaceholderOnlyOnDepartmentSteps(t *testing.T) {
	ms := newMockStore()
	svc := NewService(ms, &mockPolicy{allowed: true})
	for _, typ := range []string{ApproverSpecificUser, ApproverRole} {
		_, err := svc.CreateTemplate(context.Background(), "admin", CreateTemplateInput{
			TenantID: "tenant-1", Name: "T", EntityType: "x",
			Steps: []StepInput{{StepOrder: 1, ApproverType: typ, ApproverValue: "{creator_dept}"}},
		})
		if !errors.Is(err, ErrInvalidInput) {
			t.Errorf("%s with placeholder: err = %v, want ErrInvalidInput", typ, err)
		}
	}
	if _, err := svc.CreateTemplate(context.Background(), "admin", CreateTemplateInput{
		TenantID: "tenant-1", Name: "T", EntityType: "x",
		Steps: []StepInput{{StepOrder: 1, ApproverType: ApproverDepartment, ApproverValue: "{creator_dept}"}},
	}); err != nil {
		t.Errorf("department placeholder: %v", err)
	}
	// Only the exact placeholder is one; anything containing it is an ordinary
	// value and must name something that exists.
	ms.approvers = map[string]bool{}
	if _, err := svc.CreateTemplate(context.Background(), "admin", CreateTemplateInput{
		TenantID: "tenant-1", Name: "T", EntityType: "x",
		Steps: []StepInput{{StepOrder: 1, ApproverType: ApproverRole, ApproverValue: "managers_of_{creator_dept}"}},
	}); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("embedded placeholder: err = %v, want ErrInvalidInput", err)
	}
}

func TestScope_ForgedScopeIsRefused(t *testing.T) {
	newSvc := func() (*Service, *mockStore, *mockPolicy) {
		svc, ms, mp := setupServiceWithTemplate()
		ms.scopesInTenant = map[string]bool{"oa-mine": true, "oa-locked": true}
		mp.allowedBy = map[string]bool{"requester|oa-mine|" + ngac.OpWrite: true}
		return svc, ms, mp
	}
	create := func(svc *Service, ctx context.Context, scope string) (*Request, error) {
		return svc.CreateApprovalRequest(ctx, CreateRequestInput{EntityType: "transfer", ScopeOAID: scope, CreatedBy: "requester"})
	}

	for name, scope := range map[string]string{
		"another tenant's attribute": "oa-elsewhere",
		"an attribute without write": "oa-locked",
	} {
		svc, ms, _ := newSvc()
		if _, err := create(svc, tenantCtx(), scope); !errors.Is(err, ErrAccessDenied) {
			t.Errorf("%s: err = %v, want ErrAccessDenied", name, err)
		}
		if len(ms.requests) != 0 {
			t.Errorf("%s: %d requests stored", name, len(ms.requests))
		}
	}
	// No workspace on the call: nothing to check the scope against.
	svc, _, _ := newSvc()
	if _, err := create(svc, context.Background(), "oa-mine"); !errors.Is(err, ErrAccessDenied) {
		t.Errorf("no tenant: err = %v, want ErrAccessDenied", err)
	}

	svc, _, _ = newSvc()
	req, err := create(svc, tenantCtx(), "oa-mine")
	if err != nil || req.ScopeOAID != "oa-mine" {
		t.Fatalf("own scope = %+v, %v", req, err)
	}
	for _, unset := range []string{"", "default"} {
		svc, _, _ = newSvc()
		if req, err := create(svc, tenantCtx(), unset); err != nil || req.ScopeOAID != "default" {
			t.Errorf("scope %q = %+v, %v; want the default scope", unset, req, err)
		}
	}
}
