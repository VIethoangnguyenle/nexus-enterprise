package events

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/twmb/franz-go/pkg/kgo"

	"ngac-platform/services/approval/internal/domain"
)

// mockReconcStore implements ReconciliationStore for testing.
type mockReconcStore struct {
	pendingBySource map[string][]*domain.AssignmentRecord
	pendingByUser   map[string][]*domain.AssignmentRecord
	inserted        []*domain.AssignmentRecord
	statusUpdates   map[string]string // id → status
	auditEntries    []*domain.AuditEntry
}

func newMockReconcStore() *mockReconcStore {
	return &mockReconcStore{
		pendingBySource: make(map[string][]*domain.AssignmentRecord),
		pendingByUser:   make(map[string][]*domain.AssignmentRecord),
		statusUpdates:   make(map[string]string),
	}
}

func (m *mockReconcStore) FindPendingByGrantSource(_ context.Context, pattern string) ([]*domain.AssignmentRecord, error) {
	recs, ok := m.pendingBySource[pattern]
	if !ok {
		return nil, nil
	}
	return recs, nil
}

func (m *mockReconcStore) FindPendingByUserAndSource(_ context.Context, userNodeID, pattern string) ([]*domain.AssignmentRecord, error) {
	key := userNodeID + ":" + pattern
	recs, ok := m.pendingByUser[key]
	if !ok {
		return nil, errors.New("not found")
	}
	return recs, nil
}

func (m *mockReconcStore) InsertAssignments(_ context.Context, assignments []*domain.AssignmentRecord) error {
	m.inserted = append(m.inserted, assignments...)
	return nil
}

func (m *mockReconcStore) UpdateAssignmentStatus(_ context.Context, id, status, _ string) error {
	m.statusUpdates[id] = status
	return nil
}

func (m *mockReconcStore) InsertAuditEntry(_ context.Context, entry *domain.AuditEntry) error {
	m.auditEntries = append(m.auditEntries, entry)
	return nil
}

// --- Tests ---

func TestHandleRecord_UserAddedToUA_CreatesNewAssignments(t *testing.T) {
	store := newMockReconcStore()
	consumer := &ReconciliationConsumer{store: store}

	// Existing pending assignment granted by role:Managers_UA
	store.pendingBySource["role:Managers_UA"] = []*domain.AssignmentRecord{
		{ID: "asgn-1", RequestID: "req-1", StepOrder: 1, UserNodeID: "old_user", GrantSource: "role:Managers_UA", Status: "pending"},
		{ID: "asgn-2", RequestID: "req-2", StepOrder: 1, UserNodeID: "old_user", GrantSource: "role:Managers_UA", Status: "pending"},
	}

	// Create the event: new_user added to Managers_UA
	evt := GraphMutatedEvent{
		MutationType: "create_assignment",
		NodeIDs:      []string{"new_user", "Managers_UA"},
		ChildType:    "U",
		ParentType:   "UA",
		Timestamp:    1000,
	}
	data, _ := json.Marshal(evt)
	record := &kgo.Record{Value: data}

	consumer.handleRecord(context.Background(), record)

	// Should create 2 new assignments for new_user
	if len(store.inserted) != 2 {
		t.Fatalf("inserted %d, want 2", len(store.inserted))
	}
	for _, a := range store.inserted {
		if a.UserNodeID != "new_user" {
			t.Errorf("assignment user = %q, want new_user", a.UserNodeID)
		}
		if a.Status != "pending" {
			t.Errorf("status = %q, want pending", a.Status)
		}
		if a.GrantSource != "role:Managers_UA" {
			t.Errorf("grant_source = %q, want role:Managers_UA", a.GrantSource)
		}
	}

	// Should have 2 audit entries
	if len(store.auditEntries) != 2 {
		t.Errorf("audit entries = %d, want 2", len(store.auditEntries))
	}
	for _, e := range store.auditEntries {
		if e.Action != "reassigned_policy_change" {
			t.Errorf("audit action = %q, want reassigned_policy_change", e.Action)
		}
	}
	t.Log("✅ User added to UA → 2 new assignments created + 2 audit entries")
}

func TestHandleRecord_UserRemovedFromUA_RevokesAssignments(t *testing.T) {
	store := newMockReconcStore()
	consumer := &ReconciliationConsumer{store: store}

	// User has pending assignments granted by role:Accountant_UA
	store.pendingByUser["removed_user:role:Accountant_UA"] = []*domain.AssignmentRecord{
		{ID: "asgn-10", RequestID: "req-10", StepOrder: 1, UserNodeID: "removed_user", GrantSource: "role:Accountant_UA", Status: "pending"},
		{ID: "asgn-11", RequestID: "req-11", StepOrder: 2, UserNodeID: "removed_user", GrantSource: "role:Accountant_UA", Status: "pending"},
	}

	evt := GraphMutatedEvent{
		MutationType: "remove_assignment",
		NodeIDs:      []string{"removed_user", "Accountant_UA"},
		ChildType:    "U",
		ParentType:   "UA",
		Timestamp:    2000,
	}
	data, _ := json.Marshal(evt)
	record := &kgo.Record{Value: data}

	consumer.handleRecord(context.Background(), record)

	// Both assignments should be revoked
	if len(store.statusUpdates) != 2 {
		t.Fatalf("status updates = %d, want 2", len(store.statusUpdates))
	}
	if store.statusUpdates["asgn-10"] != "revoked" {
		t.Errorf("asgn-10 status = %q, want revoked", store.statusUpdates["asgn-10"])
	}
	if store.statusUpdates["asgn-11"] != "revoked" {
		t.Errorf("asgn-11 status = %q, want revoked", store.statusUpdates["asgn-11"])
	}

	// Audit entries
	if len(store.auditEntries) != 2 {
		t.Errorf("audit entries = %d, want 2", len(store.auditEntries))
	}
	for _, e := range store.auditEntries {
		if e.Action != "revoked_policy_change" {
			t.Errorf("audit action = %q, want revoked_policy_change", e.Action)
		}
	}
	t.Log("✅ User removed from UA → 2 assignments revoked + 2 audit entries")
}

func TestHandleRecord_IgnoresNonUserUAEvents(t *testing.T) {
	store := newMockReconcStore()
	consumer := &ReconciliationConsumer{store: store}

	// OA→PC assignment (not user→UA) — should be ignored
	evt := GraphMutatedEvent{
		MutationType: "create_assignment",
		NodeIDs:      []string{"oa1", "pc1"},
		ChildType:    "OA",
		ParentType:   "PC",
	}
	data, _ := json.Marshal(evt)
	record := &kgo.Record{Value: data}

	consumer.handleRecord(context.Background(), record)

	if len(store.inserted) != 0 {
		t.Errorf("should not insert for non-U→UA event, got %d", len(store.inserted))
	}
	t.Log("✅ Non-user-UA events correctly ignored")
}

func TestHandleRecord_SkipsDuplicateRequestStep(t *testing.T) {
	store := newMockReconcStore()
	consumer := &ReconciliationConsumer{store: store}

	// Two assignments for the same request+step (different approvers)
	store.pendingBySource["role:Managers_UA"] = []*domain.AssignmentRecord{
		{ID: "a1", RequestID: "req-dup", StepOrder: 1, UserNodeID: "user_a", GrantSource: "role:Managers_UA"},
		{ID: "a2", RequestID: "req-dup", StepOrder: 1, UserNodeID: "user_b", GrantSource: "role:Managers_UA"},
	}

	evt := GraphMutatedEvent{
		MutationType: "create_assignment",
		NodeIDs:      []string{"new_user", "Managers_UA"},
		ChildType:    "U",
		ParentType:   "UA",
	}
	data, _ := json.Marshal(evt)
	consumer.handleRecord(context.Background(), &kgo.Record{Value: data})

	// Should only create 1 assignment (deduped by request_id+step_order)
	if len(store.inserted) != 1 {
		t.Errorf("inserted %d, want 1 (deduplicated)", len(store.inserted))
	}
	t.Log("✅ Duplicate request+step correctly deduplicated")
}

func TestHandleRecord_SkipsIfUserAlreadyAssigned(t *testing.T) {
	store := newMockReconcStore()
	consumer := &ReconciliationConsumer{store: store}

	store.pendingBySource["role:Managers_UA"] = []*domain.AssignmentRecord{
		{ID: "a1", RequestID: "req-exists", StepOrder: 1, UserNodeID: "user_a", GrantSource: "role:Managers_UA"},
	}

	// Mark that existing_user already has an assignment for this request
	store.pendingByUser["existing_user:req-exists"] = []*domain.AssignmentRecord{
		{ID: "existing-asgn", RequestID: "req-exists", StepOrder: 1, UserNodeID: "existing_user"},
	}

	evt := GraphMutatedEvent{
		MutationType: "create_assignment",
		NodeIDs:      []string{"existing_user", "Managers_UA"},
		ChildType:    "U",
		ParentType:   "UA",
	}
	data, _ := json.Marshal(evt)
	consumer.handleRecord(context.Background(), &kgo.Record{Value: data})

	// Should not create duplicate assignment
	if len(store.inserted) != 0 {
		t.Errorf("inserted %d, want 0 (user already assigned)", len(store.inserted))
	}
	t.Log("✅ Already-assigned user correctly skipped")
}

func TestHandleRecord_ShortNodeIDs_Ignored(t *testing.T) {
	store := newMockReconcStore()
	consumer := &ReconciliationConsumer{store: store}

	evt := GraphMutatedEvent{
		MutationType: "create_assignment",
		NodeIDs:      []string{"only_one"}, // < 2 IDs
		ChildType:    "U",
		ParentType:   "UA",
	}
	data, _ := json.Marshal(evt)
	consumer.handleRecord(context.Background(), &kgo.Record{Value: data})

	if len(store.inserted) != 0 {
		t.Errorf("should not process events with < 2 node IDs")
	}
	t.Log("✅ Short NodeIDs correctly ignored")
}

func TestHandleRecord_InvalidJSON_NoError(t *testing.T) {
	store := newMockReconcStore()
	consumer := &ReconciliationConsumer{store: store}

	consumer.handleRecord(context.Background(), &kgo.Record{Value: []byte("not json")})

	if len(store.inserted) != 0 || len(store.statusUpdates) != 0 {
		t.Error("invalid JSON should not cause any store operations")
	}
	t.Log("✅ Invalid JSON gracefully handled")
}

func TestHandleRecord_DepartmentPattern(t *testing.T) {
	store := newMockReconcStore()
	consumer := &ReconciliationConsumer{store: store}

	// Assignment granted by department pattern
	store.pendingBySource["department:KeToan_Dept_UA"] = []*domain.AssignmentRecord{
		{ID: "d1", RequestID: "req-dept", StepOrder: 1, UserNodeID: "dept_user", GrantSource: "department:KeToan_Dept_UA"},
	}

	evt := GraphMutatedEvent{
		MutationType: "create_assignment",
		NodeIDs:      []string{"new_dept_user", "KeToan_Dept_UA"},
		ChildType:    "U",
		ParentType:   "UA",
	}
	data, _ := json.Marshal(evt)
	consumer.handleRecord(context.Background(), &kgo.Record{Value: data})

	if len(store.inserted) != 1 {
		t.Fatalf("inserted %d, want 1 for department pattern", len(store.inserted))
	}
	if store.inserted[0].GrantSource != "department:KeToan_Dept_UA" {
		t.Errorf("grant_source = %q, want department:KeToan_Dept_UA", store.inserted[0].GrantSource)
	}
	t.Log("✅ Department pattern reconciliation works")
}
