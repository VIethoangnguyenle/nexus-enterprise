package domain

import (
	"context"
	"errors"
	"testing"

	"ngac-platform/pkg/grpcauth"
)

// What a person may do here is decided on the current graph, not on the tenant
// their token says. A person who was removed from the workspace holds a token for
// it for up to its lifetime, and every approval call is refused for that time.

func inTenant(tenant string) context.Context {
	return grpcauth.WithCaller(context.Background(), grpcauth.Caller{UserID: "u", NGACNodeID: "n", TenantID: tenant})
}

func TestRequireMember_ARemovedPersonIsRefused(t *testing.T) {
	mp := &mockPolicy{notMembers: map[string]bool{"gone": true}}
	svc := NewService(newMockStore(), mp)

	if err := svc.RequireMember(context.Background(), "stays", "ws-1"); err != nil {
		t.Errorf("a member: %v", err)
	}
	if err := svc.RequireMember(context.Background(), "gone", "ws-1"); !errors.Is(err, ErrAccessDenied) {
		t.Errorf("a removed person: err = %v, want ErrAccessDenied", err)
	}
}

// The graph not answering is not a yes.
func TestRequireMember_AnOutageIsNotAnAllow(t *testing.T) {
	mp := &mockPolicy{memberErr: errors.New("policy: unavailable")}
	svc := NewService(newMockStore(), mp)

	err := svc.RequireMember(context.Background(), "anyone", "ws-1")

	if err == nil {
		t.Fatal("allowed with no answer from the policy")
	}
	if errors.Is(err, ErrAccessDenied) {
		t.Error("an outage is an error, not a denial the person could act on")
	}
}

func TestRequireMember_NeedsAPersonAndATenant(t *testing.T) {
	svc := NewService(newMockStore(), &mockPolicy{})
	for _, c := range []struct{ node, tenant string }{{"", "ws-1"}, {"n", ""}} {
		if err := svc.RequireMember(context.Background(), c.node, c.tenant); !errors.Is(err, ErrAccessDenied) {
			t.Errorf("RequireMember(%q, %q) = %v, want ErrAccessDenied", c.node, c.tenant, err)
		}
	}
}

// A named approver is a direct row: nothing re-checks them against a role or
// department. Once removed from the workspace, their row must not let them decide.
func TestApprove_ARemovedNamedApproverCannotDecide(t *testing.T) {
	svc, ms, mp := setupServiceWithTemplate()
	req, err := svc.CreateApprovalRequest(context.Background(), CreateRequestInput{
		EntityType: "transfer", EntityID: "txn-gate", EntityFields: EntityFields{},
		ScopeOAID: "oa_dept1", DepartmentID: "dept1", CreatedBy: "user1",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if a := ms.assignments[req.ID+":approver1"]; a == nil || a.GrantSource != "direct" {
		t.Fatalf("fixture: approver1 should hold a direct row, got %+v", a)
	}

	// Allow: while a member, the same row decides.
	if err := svc.Approve(inTenant("ws-1"), ApproveInput{RequestID: req.ID, UserNodeID: "approver1"}); err != nil {
		t.Fatalf("a member's approval: %v", err)
	}

	// Deny: the second request, same fixture, once approver1 is no longer in the workspace.
	req2, err := svc.CreateApprovalRequest(context.Background(), CreateRequestInput{
		EntityType: "transfer", EntityID: "txn-gate-2", EntityFields: EntityFields{},
		ScopeOAID: "oa_dept1", DepartmentID: "dept1", CreatedBy: "user1",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	mp.notMembers = map[string]bool{"approver1": true}
	before := ms.assignments[req2.ID+":approver1"].Status
	err = svc.Approve(inTenant("ws-1"), ApproveInput{RequestID: req2.ID, UserNodeID: "approver1"})
	if !errors.Is(err, ErrAccessDenied) {
		t.Fatalf("err = %v, want ErrAccessDenied for a removed named approver", err)
	}
	if got := ms.assignments[req2.ID+":approver1"].Status; got != before {
		t.Errorf("assignment moved from %q to %q though the approver was refused", before, got)
	}
	if err := svc.Reject(inTenant("ws-1"), RejectInput{RequestID: req2.ID, UserNodeID: "approver1"}); !errors.Is(err, ErrAccessDenied) {
		t.Errorf("reject: err = %v, want ErrAccessDenied", err)
	}
}
