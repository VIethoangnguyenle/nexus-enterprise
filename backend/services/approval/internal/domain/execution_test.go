package domain

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"ngac-platform/ngac"
)

// --- mock store ---

type mockStore struct {
	templates   []*Template
	requests    map[string]*Request
	assignments map[string]*AssignmentRecord // keyed by "requestID:userNodeID"
	assignList  []*AssignmentRecord
	auditLog    []*AuditEntry
	approved    int // when non-zero, the count reported for any step
	// departments maps a department id to its UA; approvers, when non-nil, is the
	// set of approver keys that exist in the tenant.
	departments map[string]string
	approvers   map[string]bool
}

func newMockStore() *mockStore {
	return &mockStore{
		requests:    make(map[string]*Request),
		assignments: make(map[string]*AssignmentRecord),
	}
}

// InTx gives the mock the all-or-nothing behaviour of the real store: when fn
// fails, everything it wrote through the mock is put back.
func (m *mockStore) InTx(ctx context.Context, fn func(ctx context.Context) error) error {
	reqs := make(map[string]*Request, len(m.requests))
	for k, v := range m.requests {
		cp := *v
		reqs[k] = &cp
	}
	assigns := make(map[string]*AssignmentRecord, len(m.assignments))
	for k, v := range m.assignments {
		cp := *v
		assigns[k] = &cp
	}
	list := make([]*AssignmentRecord, len(m.assignList))
	for i, v := range m.assignList {
		cp := *v
		list[i] = &cp
	}
	audit := append([]*AuditEntry(nil), m.auditLog...)

	if err := fn(ctx); err != nil {
		m.requests, m.assignments, m.assignList, m.auditLog = reqs, assigns, list, audit
		return err
	}
	return nil
}

func (m *mockStore) InsertTemplate(_ context.Context, t *Template) error {
	m.templates = append(m.templates, t)
	return nil
}
func (m *mockStore) GetTemplate(_ context.Context, id string) (*Template, error) {
	for _, t := range m.templates {
		if t.ID == id {
			cp := *t // a read hands out a copy, as the database does
			return &cp, nil
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
func (m *mockStore) UpdateTemplate(_ context.Context, t *Template, expected time.Time) (time.Time, error) {
	for i, cur := range m.templates {
		if cur.ID != t.ID {
			continue
		}
		if !cur.UpdatedAt.Equal(expected) {
			return time.Time{}, ErrStale
		}
		t.UpdatedAt = expected.Add(time.Second)
		m.templates[i] = t
		return t.UpdatedAt, nil
	}
	return time.Time{}, ErrNotFound
}

// FindNodeID answers for the management OA of the tenant "tenant-1" only.
func (m *mockStore) FindNodeID(_ context.Context, name, nodeType string) (string, error) {
	if nodeType == ngac.TypeOA && name == ngac.MgmtOAName("tenant-1") {
		return "mgmt-oa", nil
	}
	return "", ErrNotFound
}

// CanonicalApprover accepts the values in knownApprovers and turns a department
// id into its UA; everything else is not in the tenant.
func (m *mockStore) CanonicalApprover(_ context.Context, _, _, value string) (string, error) {
	if ua, ok := m.departments[value]; ok {
		return ua, nil
	}
	if m.approvers == nil || m.approvers[value] {
		return value, nil
	}
	return "", ErrInvalidInput
}

func (m *mockStore) InsertRequestWithAssignments(ctx context.Context, r *Request, as []*AssignmentRecord) error {
	if err := m.InsertRequest(ctx, r); err != nil {
		return err
	}
	return m.InsertAssignments(ctx, as)
}
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
func (m *mockStore) LockRequest(_ context.Context, id string) (string, int, error) {
	r, ok := m.requests[id]
	if !ok {
		return "", 0, ErrNotFound
	}
	return r.Status, r.CurrentStep, nil
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
func (m *mockStore) HasAssignment(_ context.Context, requestID string, nodeIDs []string) (bool, error) {
	for _, a := range m.assignList {
		for _, id := range nodeIDs {
			if a.RequestID == requestID && a.UserNodeID == id {
				return true, nil
			}
		}
	}
	return false, nil
}

func (m *mockStore) FindGroupAssignment(_ context.Context, requestID string, step int, groups []string) (*AssignmentRecord, error) {
	for _, a := range m.assignList {
		if a.RequestID != requestID || a.StepOrder != step || a.Status != "pending" || !isGroupRow(a) {
			continue
		}
		for _, g := range groups {
			if a.UserNodeID == g {
				return a, nil
			}
		}
	}
	return nil, ErrNotFound
}

// InsertActedAssignment enforces the unique (request, step, person) index.
func (m *mockStore) InsertActedAssignment(_ context.Context, a *AssignmentRecord) error {
	for _, x := range m.assignList {
		if x.RequestID == a.RequestID && x.StepOrder == a.StepOrder && x.UserNodeID == a.UserNodeID {
			return ErrAlreadyExists
		}
	}
	m.assignList = append(m.assignList, a)
	return nil
}
func (m *mockStore) ListAssignments(_ context.Context, requestID string) ([]*AssignmentRecord, error) {
	var out []*AssignmentRecord
	for _, a := range m.assignList {
		if a.RequestID == requestID {
			out = append(out, a)
		}
	}
	return out, nil
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
func (m *mockStore) ListPendingAssignees(_ context.Context, requestID string, stepOrder int) ([]string, error) {
	var out []string
	for _, a := range m.assignList {
		if a.RequestID == requestID && a.StepOrder == stepOrder && a.Status == "pending" {
			out = append(out, a.UserNodeID)
		}
	}
	return out, nil
}
func (m *mockStore) CountApprovedForStep(_ context.Context, requestID string, stepOrder int) (int, error) {
	if m.approved != 0 {
		return m.approved, nil
	}
	n := 0
	for _, a := range m.assignList {
		if a.RequestID == requestID && a.StepOrder == stepOrder && a.Status == "approved" {
			n++
		}
	}
	return n, nil
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

// ListPending applies the store's rule: the user's own pending rows and the group
// rows of the groups given, on a request's current step, minus steps the user
// has already acted on.
func (m *mockStore) ListPending(_ context.Context, user string, groups []string) ([]*RequestWithAssignment, error) {
	var out []*RequestWithAssignment
	for _, a := range m.assignList {
		r := m.requests[a.RequestID]
		if r == nil || r.Status != "pending" || a.Status != "pending" || a.StepOrder != r.CurrentStep {
			continue
		}
		mine := a.UserNodeID == user
		for _, g := range groups {
			if a.UserNodeID == g && isGroupRow(a) {
				mine = true
			}
		}
		acted := false
		for _, x := range m.assignList {
			if x != a && x.RequestID == a.RequestID && x.StepOrder == a.StepOrder && x.UserNodeID == user {
				acted = true
			}
		}
		if mine && !acted {
			out = append(out, &RequestWithAssignment{Request: r, Assignment: a})
		}
	}
	return out, nil
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
	scopes    []string
	allowed   bool
	scopesErr error
	// ancestors is what each node reaches upward (roles and departments of a
	// person); members is what sits under a group.
	ancestors    map[string][]string
	members      map[string][]string
	ancestorsErr error
	// allowedBy, when set, decides CheckAccess per "user|object|op".
	allowedBy map[string]bool
}

func (m *mockPolicy) ResolveAccessibleScopes(_ context.Context, _, _ string) ([]string, error) {
	return m.scopes, m.scopesErr
}
func (m *mockPolicy) CheckAccess(_ context.Context, user, object, op string) (bool, error) {
	if m.allowedBy != nil {
		return m.allowedBy[user+"|"+object+"|"+op], nil
	}
	return m.allowed, nil
}
func (m *mockPolicy) GetAncestors(_ context.Context, node string) ([]string, error) {
	return m.ancestors[node], m.ancestorsErr
}
func (m *mockPolicy) GetMembers(_ context.Context, node string) ([]string, error) {
	return m.members[node], nil
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
		UserNodeID: "approver2", GrantSource: "direct", Status: "pending",
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
		UserNodeID: "approver2", GrantSource: "direct", Status: "pending",
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
