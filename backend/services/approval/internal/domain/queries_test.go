package domain

import (
	"context"
	"errors"
	"testing"
)

// --- Query Tab Tests ---

func TestGetPending_EmptyInput(t *testing.T) {
	ms := newMockStore()
	mp := &mockPolicy{allowed: true}
	svc := NewService(ms, mp)

	_, err := svc.GetPending(context.Background(), "")
	if !errors.Is(err, ErrInvalidInput) {
		t.Errorf("err = %v, want ErrInvalidInput", err)
	}
}

func TestGetPending_ReturnsList(t *testing.T) {
	ms := newMockStore()
	mp := &mockPolicy{allowed: true}
	svc := NewService(ms, mp)

	items, err := svc.GetPending(context.Background(), "user1")
	if err != nil {
		t.Fatalf("GetPending: %v", err)
	}
	// Mock returns nil — no items
	if items != nil {
		t.Errorf("expected nil items from empty store, got %d", len(items))
	}
}

func TestGetHistory_DefaultLimit(t *testing.T) {
	ms := newMockStore()
	mp := &mockPolicy{allowed: true}
	svc := NewService(ms, mp)

	_, _, err := svc.GetHistory(context.Background(), "user1", "", 0)
	if err != nil {
		t.Fatalf("GetHistory: %v", err)
	}
}

func TestGetHistory_EmptyInput(t *testing.T) {
	ms := newMockStore()
	mp := &mockPolicy{allowed: true}
	svc := NewService(ms, mp)

	_, _, err := svc.GetHistory(context.Background(), "", "", 10)
	if !errors.Is(err, ErrInvalidInput) {
		t.Errorf("err = %v, want ErrInvalidInput", err)
	}
}

func TestGetMyRequests_EmptyInput(t *testing.T) {
	ms := newMockStore()
	mp := &mockPolicy{allowed: true}
	svc := NewService(ms, mp)

	_, _, err := svc.GetMyRequests(context.Background(), "", "", 10)
	if !errors.Is(err, ErrInvalidInput) {
		t.Errorf("err = %v, want ErrInvalidInput", err)
	}
}

func TestGetDepartmentRequests_EmptyInput(t *testing.T) {
	ms := newMockStore()
	mp := &mockPolicy{allowed: true}
	svc := NewService(ms, mp)

	_, _, err := svc.GetDepartmentRequests(context.Background(), "", "", 10)
	if !errors.Is(err, ErrInvalidInput) {
		t.Errorf("err = %v, want ErrInvalidInput", err)
	}
}

func TestGetDepartmentRequests_NoScopes(t *testing.T) {
	ms := newMockStore()
	mp := &mockPolicy{scopes: []string{}, allowed: true}
	svc := NewService(ms, mp)

	items, cursor, err := svc.GetDepartmentRequests(context.Background(), "user1", "", 20)
	if err != nil {
		t.Fatalf("GetDepartmentRequests: %v", err)
	}
	if items != nil {
		t.Errorf("expected nil items for no scopes, got %d", len(items))
	}
	if cursor != "" {
		t.Errorf("expected empty cursor, got %q", cursor)
	}
}

func TestGetDepartmentRequests_WithScopes(t *testing.T) {
	ms := newMockStore()
	mp := &mockPolicy{scopes: []string{"oa1", "oa2"}, allowed: true}
	svc := NewService(ms, mp)

	// Mock store returns nil for ListByScopes — just verifies no errors
	items, _, err := svc.GetDepartmentRequests(context.Background(), "user1", "", 20)
	if err != nil {
		t.Fatalf("GetDepartmentRequests: %v", err)
	}
	// Mock returns nil — real test needs DB
	if items != nil {
		t.Errorf("expected nil from mock, got %d", len(items))
	}
}

const (
	req1ID = "11111111-1111-4111-8111-111111111111"
	req2ID = "22222222-2222-4222-8222-222222222222"
)

func TestGetAuditLog_EmptyInput(t *testing.T) {
	svc := NewService(newMockStore(), &mockPolicy{allowed: true})

	if _, err := svc.GetAuditLog(context.Background(), "user1", ""); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("empty request id: err = %v, want ErrInvalidInput", err)
	}
	if _, err := svc.GetAuditLog(context.Background(), "", req1ID); !errors.Is(err, ErrAccessDenied) {
		t.Errorf("empty caller: err = %v, want ErrAccessDenied", err)
	}
}

// auditFixture holds req1 (created by "requester", scoped to oa-dept, with
// "approver" assigned to a past step) and req2, plus an audit trail for each.
func auditFixture(policy *mockPolicy) (*Service, *mockStore) {
	ms := newMockStore()
	ms.requests[req1ID] = &Request{ID: req1ID, CreatedBy: "requester", ScopeOAID: "oa-dept"}
	ms.requests[req2ID] = &Request{ID: req2ID, CreatedBy: "someone-else", ScopeOAID: "oa-other"}
	ms.assignList = append(ms.assignList, &AssignmentRecord{
		ID: "as1", RequestID: req1ID, StepOrder: 1, UserNodeID: "approver", Status: "skipped",
	})
	ms.auditLog = append(ms.auditLog,
		&AuditEntry{ID: "a1", RequestID: req1ID, Action: "created"},
		&AuditEntry{ID: "a2", RequestID: req1ID, Action: "approved"},
		&AuditEntry{ID: "a3", RequestID: req2ID, Action: "created"},
	)
	return NewService(ms, policy), ms
}

func TestGetAuditLog_AllowedForEachPathToTheRequest(t *testing.T) {
	for _, tc := range []struct {
		name   string
		caller string
		policy *mockPolicy
	}{
		{"requester", "requester", &mockPolicy{}},
		{"assignee of a past step", "approver", &mockPolicy{}},
		{"department scope covers it", "dept-head", &mockPolicy{scopes: []string{"oa-x", "oa-dept"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, _ := auditFixture(tc.policy)
			entries, err := svc.GetAuditLog(context.Background(), tc.caller, req1ID)
			if err != nil {
				t.Fatalf("GetAuditLog: %v", err)
			}
			if len(entries) != 2 {
				t.Errorf("got %d entries, want the 2 of req1", len(entries))
			}
		})
	}
}

func TestGetAuditLog_DeniedForAnUnrelatedCaller(t *testing.T) {
	// Scopes cover another request only; the caller neither created nor was
	// assigned req1.
	svc, _ := auditFixture(&mockPolicy{scopes: []string{"oa-other"}})
	entries, err := svc.GetAuditLog(context.Background(), "stranger", req1ID)
	if !errors.Is(err, ErrAccessDenied) {
		t.Fatalf("err = %v, want ErrAccessDenied", err)
	}
	if entries != nil {
		t.Errorf("a denied caller received %d entries", len(entries))
	}

	// Having an assignment on req1 grants nothing on req2.
	svc, _ = auditFixture(&mockPolicy{})
	if _, err := svc.GetAuditLog(context.Background(), "approver", req2ID); !errors.Is(err, ErrAccessDenied) {
		t.Errorf("assignee of req1 reading req2: err = %v, want ErrAccessDenied", err)
	}
}

func TestGetAuditLog_ScopeResolutionFailureIsNotAGrant(t *testing.T) {
	svc, _ := auditFixture(&mockPolicy{scopes: []string{"oa-dept"}, scopesErr: errors.New("policy down")})
	entries, err := svc.GetAuditLog(context.Background(), "dept-head", req1ID)
	if err == nil {
		t.Fatal("want an error when scopes cannot be resolved")
	}
	if entries != nil {
		t.Errorf("entries leaked while the policy was unreachable")
	}
}

func TestGetAuditLog_MissingHiddenAndMalformedIDs(t *testing.T) {
	svc, _ := auditFixture(&mockPolicy{})
	// A missing request is answered exactly like a hidden one.
	missing := "3f2c1d0e-0000-4000-8000-000000000000"
	if _, err := svc.GetAuditLog(context.Background(), "requester", missing); !errors.Is(err, ErrAccessDenied) {
		t.Errorf("missing request: err = %v, want ErrAccessDenied", err)
	}
	if _, err := svc.GetAuditLog(context.Background(), "stranger", req1ID); !errors.Is(err, ErrAccessDenied) {
		t.Errorf("hidden request: err = %v, want ErrAccessDenied", err)
	}
	// A malformed id is a bad request, not a server error.
	for _, bad := range []string{"not-a-uuid", "1", "req1"} {
		if _, err := svc.GetAuditLog(context.Background(), "requester", bad); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("id %q: err = %v, want ErrInvalidInput", bad, err)
		}
	}
}

// --- Multi-step with required_count > 1 ---

func TestApprove_RequiredCountGreaterThanOne(t *testing.T) {
	ms := newMockStore()
	mp := &mockPolicy{scopes: []string{"approver1"}, allowed: true}
	svc := NewService(ms, mp)

	// Template with 1 step requiring 2 approvals
	tmpl := &Template{
		ID:         "tmpl-multi",
		Name:       "Dual Approval",
		EntityType: "transfer",
		IsActive:   true,
		Priority:   10,
		Steps: []*Step{
			{StepOrder: 1, Name: "Dual", ApproverType: "specific_user", ApproverValue: "approver1", RequiredCount: 2},
		},
	}
	ms.templates = append(ms.templates, tmpl)

	req, err := svc.CreateApprovalRequest(context.Background(), CreateRequestInput{
		EntityType:   "transfer",
		EntityID:     "txn-multi",
		EntityFields: EntityFields{},
		ScopeOAID:    "oa1",
		DepartmentID: "dept1",
		CreatedBy:    "user1",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// First approval — count = 1, required = 2 → should NOT complete
	ms.approved = 1
	err = svc.Approve(context.Background(), ApproveInput{
		RequestID:  req.ID,
		UserNodeID: "approver1",
		Comment:    "first approval",
	})
	if err != nil {
		t.Fatalf("approve first: %v", err)
	}

	// Status should still be pending (needs 2 approvals, only got 1)
	if ms.requests[req.ID].Status != "pending" {
		t.Errorf("status = %q, want pending (required_count=2, got 1)", ms.requests[req.ID].Status)
	}
}

// --- Cancel flow ---

func TestApprove_OnCompletedRequest(t *testing.T) {
	svc, ms, _ := setupServiceWithTemplate()

	req, err := svc.CreateApprovalRequest(context.Background(), CreateRequestInput{
		EntityType:   "transfer",
		EntityID:     "txn-completed",
		EntityFields: EntityFields{},
		ScopeOAID:    "oa1",
		DepartmentID: "dept1",
		CreatedBy:    "user1",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// Mark as cancelled
	ms.requests[req.ID].Status = "cancelled"

	err = svc.Approve(context.Background(), ApproveInput{
		RequestID:  req.ID,
		UserNodeID: "approver1",
	})
	if !errors.Is(err, ErrRequestCompleted) {
		t.Errorf("err = %v, want ErrRequestCompleted", err)
	}
}

// --- Validation edge cases ---

func TestCreateApprovalRequest_EmptyEntityType(t *testing.T) {
	ms := newMockStore()
	mp := &mockPolicy{allowed: true}
	svc := NewService(ms, mp)

	_, err := svc.CreateApprovalRequest(context.Background(), CreateRequestInput{
		EntityType:   "",
		EntityID:     "txn-001",
		EntityFields: EntityFields{},
		CreatedBy:    "user1",
	})
	if !errors.Is(err, ErrInvalidInput) {
		t.Errorf("err = %v, want ErrInvalidInput for empty entity_type", err)
	}
}

func TestApprove_EmptyRequestID(t *testing.T) {
	ms := newMockStore()
	mp := &mockPolicy{allowed: true}
	svc := NewService(ms, mp)

	err := svc.Approve(context.Background(), ApproveInput{
		RequestID:  "",
		UserNodeID: "approver1",
	})
	if !errors.Is(err, ErrInvalidInput) {
		t.Errorf("err = %v, want ErrInvalidInput for empty request_id", err)
	}
}

func TestReject_EmptyUserNodeID(t *testing.T) {
	ms := newMockStore()
	mp := &mockPolicy{allowed: true}
	svc := NewService(ms, mp)

	err := svc.Reject(context.Background(), RejectInput{
		RequestID:  "req-1",
		UserNodeID: "",
	})
	if !errors.Is(err, ErrInvalidInput) {
		t.Errorf("err = %v, want ErrInvalidInput for empty user_node_id", err)
	}
}
