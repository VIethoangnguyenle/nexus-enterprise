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

// Adding a member to a role or department creates no copy of pending requests:
// the group row already covers them, and membership is read when they act.
func TestHandleRecord_UserAddedToUA_CreatesNothing(t *testing.T) {
	store := newMockReconcStore()
	consumer := &ReconciliationConsumer{store: store}
	for _, pattern := range []string{"role:Managers_UA", "department:Managers_UA"} {
		store.pendingBySource[pattern] = []*domain.AssignmentRecord{
			{ID: "g", RequestID: "req-1", StepOrder: 1, UserNodeID: "Managers_UA", GrantSource: pattern},
		}
	}
	evt := GraphMutatedEvent{MutationType: "create_assignment", NodeIDs: []string{"new_user", "Managers_UA"}, ChildType: "U", ParentType: "UA"}
	data, _ := json.Marshal(evt)
	consumer.handleRecord(context.Background(), &kgo.Record{Value: data})
	if len(store.inserted) != 0 || len(store.auditEntries) != 0 {
		t.Errorf("inserted %d rows, %d audit entries; want none", len(store.inserted), len(store.auditEntries))
	}
}
