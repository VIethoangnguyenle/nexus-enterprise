package domain

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

// --- mock store ---

type mockStore struct {
	templates   []*Template
	requests    map[string]*Request
	assignments map[string]*AssignmentRecord // keyed by "requestID:userNodeID"
	assignList  []*AssignmentRecord
	auditLog    []*AuditEntry
	approved    int // count approved for step
}

func newMockStore() *mockStore {
	return &mockStore{
		requests:    make(map[string]*Request),
		assignments: make(map[string]*AssignmentRecord),
	}
}

func (m *mockStore) InsertTemplate(_ context.Context, t *Template) error {
	m.templates = append(m.templates, t)
	return nil
}
func (m *mockStore) GetTemplate(_ context.Context, id string) (*Template, error) {
	for _, t := range m.templates {
		if t.ID == id {
			return t, nil
		}
	}
	return nil, ErrNotFound
}
func (m *mockStore) ListTemplates(_ context.Context, entityType string, activeOnly bool) ([]*Template, error) {
	var result []*Template
	for _, t := range m.templates {
		if t.EntityType == entityType && (!activeOnly || t.IsActive) {
			result = append(result, t)
		}
	}
	return result, nil
}
func (m *mockStore) UpdateTemplate(_ context.Context, t *Template) error { return nil }

func (m *mockStore) InsertRequest(_ context.Context, r *Request) error {
	m.requests[r.ID] = r
	return nil
}
func (m *mockStore) GetRequest(_ context.Context, id string) (*Request, error) {
	r, ok := m.requests[id]
	if !ok {
		return nil, ErrNotFound
	}
	return r, nil
}
func (m *mockStore) InsertAssignments(_ context.Context, assignments []*AssignmentRecord) error {
	for _, a := range assignments {
		key := a.RequestID + ":" + a.UserNodeID
		m.assignments[key] = a
		m.assignList = append(m.assignList, a)
	}
	return nil
}
func (m *mockStore) GetAssignment(_ context.Context, requestID, userNodeID string) (*AssignmentRecord, error) {
	key := requestID + ":" + userNodeID
	a, ok := m.assignments[key]
	if !ok {
		return nil, ErrNotFound
	}
	return a, nil
}
func (m *mockStore) UpdateAssignmentStatus(_ context.Context, id, status, comment string) error {
	for _, a := range m.assignList {
		if a.ID == id {
			a.Status = status
			a.Comment = comment
			return nil
		}
	}
	return ErrNotFound
}
func (m *mockStore) CountApprovedForStep(_ context.Context, requestID string, stepOrder int) (int, error) {
	return m.approved, nil
}
func (m *mockStore) SkipRemainingAssignments(_ context.Context, requestID string, stepOrder int) error {
	return nil
}
func (m *mockStore) SkipAllPendingAssignments(_ context.Context, requestID string) error {
	return nil
}
// AdvanceStep mirrors the store's compare-and-swap: it only moves the request
// when it is still sitting on fromStep, so a second caller racing on the same
// quorum is told it lost.
func (m *mockStore) AdvanceStep(_ context.Context, requestID string, fromStep, nextStep int) (bool, error) {
	r, ok := m.requests[requestID]
	if !ok || r.CurrentStep != fromStep || r.Status != "pending" {
		return false, nil
	}
	r.CurrentStep = nextStep
	return true, nil
}
func (m *mockStore) CompleteRequest(_ context.Context, requestID, status string) (bool, error) {
	r, ok := m.requests[requestID]
	if !ok || r.Status != "pending" {
		return false, nil
	}
	r.Status = status
	return true, nil
}
func (m *mockStore) ListPending(_ context.Context, _ string) ([]*RequestWithAssignment, error) {
	return nil, nil
}
func (m *mockStore) ListHistory(_ context.Context, _, _ string, _ int) ([]*RequestWithAssignment, string, error) {
	return nil, "", nil
}
func (m *mockStore) ListMyRequests(_ context.Context, _, _ string, _ int) ([]*Request, string, error) {
	return nil, "", nil
}
func (m *mockStore) ListByScopes(_ context.Context, _ []string, _ string, _ int) ([]*Request, string, error) {
	return nil, "", nil
}
func (m *mockStore) InsertAuditEntry(_ context.Context, e *AuditEntry) error {
	m.auditLog = append(m.auditLog, e)
	return nil
}
func (m *mockStore) ListAuditEntries(_ context.Context, requestID string) ([]*AuditEntry, error) {
	var result []*AuditEntry
	for _, e := range m.auditLog {
		if e.RequestID == requestID {
			result = append(result, e)
		}
	}
	return result, nil
}

// --- mock policy ---

type mockPolicy struct {
	scopes  []string
	allowed bool
}

func (m *mockPolicy) ResolveAccessibleScopes(_ context.Context, _, _ string) ([]string, error) {
	return m.scopes, nil
}
func (m *mockPolicy) CheckAccess(_ context.Context, _, _, _ string) (bool, error) {
	return m.allowed, nil
}

// --- helper: create a service with a template already registered ---

func setupServiceWithTemplate() (*Service, *mockStore, *mockPolicy) {
	ms := newMockStore()
	mp := &mockPolicy{scopes: []string{"user1"}, allowed: true}
	svc := NewService(ms, mp)

	tmpl := &Template{
		ID:         "tmpl-1",
		Name:       "High Value Transfer",
		EntityType: "transfer",
		IsActive:   true,
		Priority:   10,
		Steps: []*Step{
			{StepOrder: 1, Name: "Manager", ApproverType: "specific_user", ApproverValue: "approver1", RequiredCount: 1},
			{StepOrder: 2, Name: "Director", ApproverType: "specific_user", ApproverValue: "approver2", RequiredCount: 1},
		},
	}
	ms.templates = append(ms.templates, tmpl)
	return svc, ms, mp
}

// --- Tests ---

func TestCreateApprovalRequest_MatchesTemplate(t *testing.T) {
	svc, ms, _ := setupServiceWithTemplate()

	req, err := svc.CreateApprovalRequest(context.Background(), CreateRequestInput{
		EntityType:   "transfer",
		EntityID:     "txn-001",
		EntityFields: EntityFields{},
		ScopeOAID:    "oa_dept1",
		DepartmentID: "dept1",
		CreatedBy:    "user1",
	})
	if err != nil {
		t.Fatalf("CreateApprovalRequest: %v", err)
	}
	if req.Status != "pending" {
		t.Errorf("status = %q, want pending", req.Status)
	}
	if req.CurrentStep != 1 {
		t.Errorf("current_step = %d, want 1", req.CurrentStep)
	}
	if req.TemplateName != "High Value Transfer" {
		t.Errorf("template_name = %q, want High Value Transfer", req.TemplateName)
	}

	// Should have created assignment for step 1 approver
	key := req.ID + ":approver1"
	a, ok := ms.assignments[key]
	if !ok {
		t.Fatal("assignment for approver1 not created")
	}
	if a.Status != "pending" {
		t.Errorf("assignment status = %q, want pending", a.Status)
	}
	if a.StepOrder != 1 {
		t.Errorf("assignment step_order = %d, want 1", a.StepOrder)
	}

	// Audit log should contain "created" and "assigned"
	actions := auditActions(ms.auditLog)
	if !contains(actions, "created") {
		t.Error("audit log missing 'created' action")
	}
	if !contains(actions, "assigned") {
		t.Error("audit log missing 'assigned' action")
	}
}

func TestCreateApprovalRequest_NoTemplate(t *testing.T) {
	ms := newMockStore()
	mp := &mockPolicy{allowed: true}
	svc := NewService(ms, mp)

	_, err := svc.CreateApprovalRequest(context.Background(), CreateRequestInput{
		EntityType:   "unknown_type",
		EntityID:     "txn-001",
		EntityFields: EntityFields{},
		ScopeOAID:    "oa_dept1",
		DepartmentID: "dept1",
		CreatedBy:    "user1",
	})
	if !errors.Is(err, ErrNoMatchingTemplate) {
		t.Errorf("err = %v, want ErrNoMatchingTemplate", err)
	}
}

func TestApprove_StepAdvancement(t *testing.T) {
	svc, ms, _ := setupServiceWithTemplate()

	// Create request
	req, err := svc.CreateApprovalRequest(context.Background(), CreateRequestInput{
		EntityType:   "transfer",
		EntityID:     "txn-002",
		EntityFields: EntityFields{},
		ScopeOAID:    "oa_dept1",
		DepartmentID: "dept1",
		CreatedBy:    "user1",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// Simulate step 1 approval count met
	ms.approved = 1

	// Approve step 1
	err = svc.Approve(context.Background(), ApproveInput{
		RequestID:  req.ID,
		UserNodeID: "approver1",
		Comment:    "looks good",
	})
	if err != nil {
		t.Fatalf("approve step 1: %v", err)
	}

	// Request should advance to step 2
	updatedReq := ms.requests[req.ID]
	if updatedReq.CurrentStep != 2 {
		t.Errorf("current_step = %d, want 2 (advanced)", updatedReq.CurrentStep)
	}

	// Should have created assignment for step 2 approver
	key := req.ID + ":approver2"
	if _, ok := ms.assignments[key]; !ok {
		t.Error("assignment for step 2 approver not created after advancement")
	}

	// Audit should have step_advanced
	actions := auditActions(ms.auditLog)
	if !contains(actions, "step_advanced") {
		t.Error("audit log missing 'step_advanced'")
	}
}

func TestApprove_FinalStep_Completes(t *testing.T) {
	svc, ms, _ := setupServiceWithTemplate()

	// Create request
	req, err := svc.CreateApprovalRequest(context.Background(), CreateRequestInput{
		EntityType:   "transfer",
		EntityID:     "txn-003",
		EntityFields: EntityFields{},
		ScopeOAID:    "oa_dept1",
		DepartmentID: "dept1",
		CreatedBy:    "user1",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// Advance to step 2 manually
	ms.requests[req.ID].CurrentStep = 2
	step2Assignment := &AssignmentRecord{
		ID: "a2", RequestID: req.ID, StepOrder: 2,
		UserNodeID: "approver2", Status: "pending",
	}
	ms.assignments[req.ID+":approver2"] = step2Assignment
	ms.assignList = append(ms.assignList, step2Assignment)
	ms.approved = 1

	err = svc.Approve(context.Background(), ApproveInput{
		RequestID:  req.ID,
		UserNodeID: "approver2",
		Comment:    "approved final",
	})
	if err != nil {
		t.Fatalf("approve final: %v", err)
	}

	// Request should be completed
	if ms.requests[req.ID].Status != "approved" {
		t.Errorf("status = %q, want approved", ms.requests[req.ID].Status)
	}

	actions := auditActions(ms.auditLog)
	if !contains(actions, "completed") {
		t.Error("audit log missing 'completed'")
	}
}

func TestReject_Terminal(t *testing.T) {
	svc, ms, _ := setupServiceWithTemplate()

	req, err := svc.CreateApprovalRequest(context.Background(), CreateRequestInput{
		EntityType:   "transfer",
		EntityID:     "txn-004",
		EntityFields: EntityFields{},
		ScopeOAID:    "oa_dept1",
		DepartmentID: "dept1",
		CreatedBy:    "user1",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	err = svc.Reject(context.Background(), RejectInput{
		RequestID:  req.ID,
		UserNodeID: "approver1",
		Comment:    "nope",
	})
	if err != nil {
		t.Fatalf("reject: %v", err)
	}

	// Request should be rejected (terminal)
	if ms.requests[req.ID].Status != "rejected" {
		t.Errorf("status = %q, want rejected", ms.requests[req.ID].Status)
	}

	// Audit should have both "rejected" and "completed"
	actions := auditActions(ms.auditLog)
	if !contains(actions, "rejected") {
		t.Error("audit log missing 'rejected'")
	}
	if !contains(actions, "completed") {
		t.Error("audit log missing 'completed'")
	}
}

func TestReject_AlreadyCompleted(t *testing.T) {
	svc, ms, _ := setupServiceWithTemplate()

	req, err := svc.CreateApprovalRequest(context.Background(), CreateRequestInput{
		EntityType:   "transfer",
		EntityID:     "txn-005",
		EntityFields: EntityFields{},
		ScopeOAID:    "oa_dept1",
		DepartmentID: "dept1",
		CreatedBy:    "user1",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// Mark as already completed
	ms.requests[req.ID].Status = "approved"

	err = svc.Reject(context.Background(), RejectInput{
		RequestID:  req.ID,
		UserNodeID: "approver1",
		Comment:    "too late",
	})
	if !errors.Is(err, ErrRequestCompleted) {
		t.Errorf("err = %v, want ErrRequestCompleted", err)
	}
}

func TestApprove_WrongStep(t *testing.T) {
	svc, ms, _ := setupServiceWithTemplate()

	req, err := svc.CreateApprovalRequest(context.Background(), CreateRequestInput{
		EntityType:   "transfer",
		EntityID:     "txn-006",
		EntityFields: EntityFields{},
		ScopeOAID:    "oa_dept1",
		DepartmentID: "dept1",
		CreatedBy:    "user1",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// Try to approve as step 2 user while step 1 is active
	step2 := &AssignmentRecord{
		ID: "a2", RequestID: req.ID, StepOrder: 2,
		UserNodeID: "approver2", Status: "pending",
	}
	ms.assignments[req.ID+":approver2"] = step2
	ms.assignList = append(ms.assignList, step2)

	err = svc.Approve(context.Background(), ApproveInput{
		RequestID:  req.ID,
		UserNodeID: "approver2",
	})
	if !errors.Is(err, ErrStepNotActive) {
		t.Errorf("err = %v, want ErrStepNotActive", err)
	}
}

func TestApprove_NGACDenied(t *testing.T) {
	svc, ms, mp := setupServiceWithTemplate()
	mp.allowed = false // NGAC will deny

	req, err := svc.CreateApprovalRequest(context.Background(), CreateRequestInput{
		EntityType:   "transfer",
		EntityID:     "txn-007",
		EntityFields: EntityFields{},
		ScopeOAID:    "oa_dept1",
		DepartmentID: "dept1",
		CreatedBy:    "user1",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	_ = ms // suppress unused

	// Change grant source to non-direct so the NGAC double-check is triggered
	if a, ok := ms.assignments[req.ID+":approver1"]; ok {
		a.GrantSource = "role:manager"
	}

	err = svc.Approve(context.Background(), ApproveInput{
		RequestID:  req.ID,
		UserNodeID: "approver1",
		Comment:    "try approve",
	})
	if !errors.Is(err, ErrAccessDenied) {
		t.Errorf("err = %v, want ErrAccessDenied (NGAC double-check)", err)
	}
}

func TestBatchApprove(t *testing.T) {
	svc, ms, _ := setupServiceWithTemplate()

	// Three real, pending requests the caller is genuinely assigned to.
	var ids []string
	for i, entity := range []string{"txn-101", "txn-102", "txn-103"} {
		req, err := svc.CreateApprovalRequest(context.Background(), CreateRequestInput{
			EntityType:   "transfer",
			EntityID:     entity,
			EntityFields: EntityFields{},
			ScopeOAID:    "oa_dept1",
			DepartmentID: "dept1",
			CreatedBy:    "user1",
		})
		if err != nil {
			t.Fatalf("create request %d: %v", i, err)
		}
		ids = append(ids, req.ID)
	}

	approved, err := svc.BatchApprove(context.Background(), BatchApproveInput{
		RequestIDs: ids,
		UserNodeID: "approver1",
		Comment:    "batch ok",
	})
	if err != nil {
		t.Fatalf("batch approve: %v", err)
	}
	if len(approved) != 3 {
		t.Errorf("approved count = %d, want 3", len(approved))
	}
	for _, id := range ids {
		if a := ms.assignments[id+":approver1"]; a == nil || a.Status != "approved" {
			t.Errorf("assignment for %s was not recorded as approved", id)
		}
	}
}

// An unknown request ID must be skipped, not silently reported as approved.
func TestBatchApprove_SkipsUnknownRequests(t *testing.T) {
	svc, _, _ := setupServiceWithTemplate()

	approved, err := svc.BatchApprove(context.Background(), BatchApproveInput{
		RequestIDs: []string{"does-not-exist-1", "does-not-exist-2"},
		UserNodeID: "approver1",
	})
	if err != nil {
		t.Fatalf("batch approve: %v", err)
	}
	if len(approved) != 0 {
		t.Errorf("approved = %v, want none for unknown request IDs", approved)
	}
}

// Batch approval must enforce the same guards as single approval. Otherwise the
// stale-role defence in Approve is decorative: a user whose role was revoked
// just sends the same request IDs to /batch-approve instead.
func TestBatchApprove_EnforcesNGACLikeSingleApprove(t *testing.T) {
	svc, ms, mp := setupServiceWithTemplate()

	req, err := svc.CreateApprovalRequest(context.Background(), CreateRequestInput{
		EntityType:   "transfer",
		EntityID:     "txn-008",
		EntityFields: EntityFields{},
		ScopeOAID:    "oa_dept1",
		DepartmentID: "dept1",
		CreatedBy:    "user1",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// Role-based grant, and the role has since been revoked.
	a, ok := ms.assignments[req.ID+":approver1"]
	if !ok {
		t.Fatal("expected an assignment for approver1")
	}
	a.GrantSource = "role:manager"
	mp.allowed = false

	approved, err := svc.BatchApprove(context.Background(), BatchApproveInput{
		RequestIDs: []string{req.ID},
		UserNodeID: "approver1",
		Comment:    "batch bypass attempt",
	})
	if err == nil && len(approved) > 0 {
		t.Fatalf("batch approved %v despite revoked role — /approve denies this", approved)
	}
	if a.Status == "approved" {
		t.Error("assignment was marked approved despite the NGAC denial")
	}
}

// A request that is no longer pending must not be approvable in a batch either.
func TestBatchApprove_RejectsCompletedRequest(t *testing.T) {
	svc, ms, _ := setupServiceWithTemplate()

	req, err := svc.CreateApprovalRequest(context.Background(), CreateRequestInput{
		EntityType:   "transfer",
		EntityID:     "txn-009",
		EntityFields: EntityFields{},
		ScopeOAID:    "oa_dept1",
		DepartmentID: "dept1",
		CreatedBy:    "user1",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	ms.requests[req.ID].Status = "rejected"

	approved, err := svc.BatchApprove(context.Background(), BatchApproveInput{
		RequestIDs: []string{req.ID},
		UserNodeID: "approver1",
	})
	if err == nil && len(approved) > 0 {
		t.Fatalf("batch approved %v on a rejected request", approved)
	}
}

func TestBatchApprove_EmptyInput(t *testing.T) {
	svc, _, _ := setupServiceWithTemplate()

	_, err := svc.BatchApprove(context.Background(), BatchApproveInput{
		RequestIDs: []string{},
		UserNodeID: "approver1",
	})
	if !errors.Is(err, ErrInvalidInput) {
		t.Errorf("err = %v, want ErrInvalidInput for empty batch", err)
	}
}

func TestTemplateSnapshot_Preserved(t *testing.T) {
	svc, ms, _ := setupServiceWithTemplate()

	req, err := svc.CreateApprovalRequest(context.Background(), CreateRequestInput{
		EntityType:   "transfer",
		EntityID:     "txn-snap",
		EntityFields: EntityFields{},
		ScopeOAID:    "oa_dept1",
		DepartmentID: "dept1",
		CreatedBy:    "user1",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// Verify snapshot is valid JSON
	var tmpl Template
	err = json.Unmarshal([]byte(ms.requests[req.ID].TemplateSnapshot), &tmpl)
	if err != nil {
		t.Fatalf("snapshot JSON invalid: %v", err)
	}
	if tmpl.Name != "High Value Transfer" {
		t.Errorf("snapshot name = %q, want High Value Transfer", tmpl.Name)
	}
	if len(tmpl.Steps) != 2 {
		t.Errorf("snapshot steps = %d, want 2", len(tmpl.Steps))
	}
}

// --- helpers ---

func auditActions(entries []*AuditEntry) []string {
	var actions []string
	for _, e := range entries {
		actions = append(actions, e.Action)
	}
	return actions
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
