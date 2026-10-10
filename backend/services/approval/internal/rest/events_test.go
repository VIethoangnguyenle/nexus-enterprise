package rest

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/labstack/echo/v4"

	"ngac-platform/pkg/httputil"
	"ngac-platform/services/approval/internal/domain"
	"ngac-platform/services/approval/internal/events"
)

// ---------------------------------------------------------------------------
// In-memory store: just enough of domain.Store for the lifecycle endpoints.
// Unused methods fall through to the embedded nil interface and would panic,
// which is the point — a test that reaches one is testing something else.
// ---------------------------------------------------------------------------

type memStore struct {
	domain.Store

	mu          sync.Mutex
	templates   []*domain.Template
	requests    map[string]*domain.Request
	assignments []*domain.AssignmentRecord
}

func newMemStore() *memStore {
	return &memStore{requests: map[string]*domain.Request{}}
}

func (m *memStore) ListTemplates(_ context.Context, entityType string, activeOnly bool) ([]*domain.Template, error) {
	var out []*domain.Template
	for _, t := range m.templates {
		if t.EntityType == entityType && (!activeOnly || t.IsActive) {
			out = append(out, t)
		}
	}
	return out, nil
}

func (m *memStore) GetTemplate(_ context.Context, id string) (*domain.Template, error) {
	for _, t := range m.templates {
		if t.ID == id {
			return t, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (m *memStore) InsertRequestWithAssignments(ctx context.Context, r *domain.Request, as []*domain.AssignmentRecord) error {
	if err := m.InsertRequest(ctx, r); err != nil {
		return err
	}
	return m.InsertAssignments(ctx, as)
}

func (m *memStore) InsertRequest(_ context.Context, r *domain.Request) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := *r
	m.requests[r.ID] = &cp
	return nil
}

func (m *memStore) GetRequest(_ context.Context, id string) (*domain.Request, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.requests[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	cp := *r
	return &cp, nil
}

func (m *memStore) InsertAssignments(_ context.Context, as []*domain.AssignmentRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.assignments = append(m.assignments, as...)
	return nil
}

func (m *memStore) GetAssignment(_ context.Context, requestID, userNodeID string) (*domain.AssignmentRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, a := range m.assignments {
		if a.RequestID == requestID && a.UserNodeID == userNodeID && a.Status == "pending" {
			return a, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (m *memStore) FindGroupAssignment(context.Context, string, int, []string) (*domain.AssignmentRecord, error) {
	return nil, domain.ErrNotFound
}

func (m *memStore) UpdateAssignmentStatus(_ context.Context, id, status, comment string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, a := range m.assignments {
		if a.ID == id {
			a.Status, a.Comment = status, comment
		}
	}
	return nil
}

func (m *memStore) CountApprovedForStep(_ context.Context, requestID string, step int) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	for _, a := range m.assignments {
		if a.RequestID == requestID && a.StepOrder == step && a.Status == "approved" {
			n++
		}
	}
	return n, nil
}

func (m *memStore) setPendingTo(requestID string, step int, to string) {
	for _, a := range m.assignments {
		if a.RequestID == requestID && (step == 0 || a.StepOrder == step) && a.Status == "pending" {
			a.Status = to
		}
	}
}

func (m *memStore) SkipRemainingAssignments(_ context.Context, requestID string, step int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.setPendingTo(requestID, step, "skipped")
	return nil
}

func (m *memStore) SkipAllPendingAssignments(_ context.Context, requestID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.setPendingTo(requestID, 0, "skipped")
	return nil
}

func (m *memStore) AdvanceStep(_ context.Context, requestID string, from, next int) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r := m.requests[requestID]
	if r == nil || r.CurrentStep != from || r.Status != "pending" {
		return false, nil
	}
	r.CurrentStep = next
	return true, nil
}

func (m *memStore) CompleteRequest(_ context.Context, requestID, status string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r := m.requests[requestID]
	if r == nil || r.Status != "pending" {
		return false, nil
	}
	r.Status = status
	return true, nil
}

func (m *memStore) ListPendingAssignees(_ context.Context, requestID string, step int) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []string
	for _, a := range m.assignments {
		if a.RequestID == requestID && a.StepOrder == step && a.Status == "pending" {
			out = append(out, a.UserNodeID)
		}
	}
	return out, nil
}

func (m *memStore) ListAssignments(_ context.Context, requestID string) ([]*domain.AssignmentRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []*domain.AssignmentRecord
	for _, a := range m.assignments {
		if a.RequestID == requestID {
			out = append(out, a)
		}
	}
	return out, nil
}

func (m *memStore) InsertAuditEntry(context.Context, *domain.AuditEntry) error { return nil }

type allowPolicy struct{}

func (allowPolicy) ResolveAccessibleScopes(context.Context, string, string) ([]string, error) {
	return nil, nil
}
func (allowPolicy) CheckAccess(context.Context, string, string, string) (bool, error) {
	return true, nil
}
func (allowPolicy) GetAncestors(context.Context, string) ([]string, error) { return nil, nil }
func (allowPolicy) GetMembers(context.Context, string) ([]string, error)   { return nil, nil }

type capturePublisher struct {
	mu  sync.Mutex
	got []events.ApprovalEventPayload
}

func (p *capturePublisher) Publish(_ context.Context, evt events.ApprovalEventPayload) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.got = append(p.got, evt)
}

// ---------------------------------------------------------------------------
// Fixture: a two-step template, requester "n-req", approvers "n-ap1" → "n-ap2".
// ---------------------------------------------------------------------------

type lifecycle struct {
	h     *Handler
	store *memStore
	pub   *capturePublisher
}

func newLifecycle(t *testing.T) *lifecycle {
	t.Helper()
	st := newMemStore()
	st.templates = []*domain.Template{{
		ID: "tmpl-1", Name: "Leave", EntityType: "leave", IsActive: true,
		Steps: []*domain.Step{
			{StepOrder: 1, Name: "Manager", ApproverType: "specific_user", ApproverValue: "n-ap1", RequiredCount: 1},
			{StepOrder: 2, Name: "Director", ApproverType: "specific_user", ApproverValue: "n-ap2", RequiredCount: 1},
		},
	}}
	pub := &capturePublisher{}
	return &lifecycle{
		h:     NewHandler(domain.NewService(st, allowPolicy{}), nil, pub),
		store: st,
		pub:   pub,
	}
}

func (l *lifecycle) call(t *testing.T, handler echo.HandlerFunc, nodeID, tenantID, body string) error {
	t.Helper()
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	c := e.NewContext(req, httptest.NewRecorder())
	httputil.SetClaims(c, &httputil.Claims{UserID: "u-" + nodeID, NGACNodeID: nodeID, TenantID: tenantID})
	return handler(c)
}

func (l *lifecycle) create(t *testing.T) string {
	t.Helper()
	if err := l.call(t, l.h.CreateRequest, "n-req", "tenant-a",
		`{"entity_type":"leave","entity_id":"e-1"}`); err != nil {
		t.Fatalf("create: %v", err)
	}
	for id := range l.store.requests {
		return id
	}
	t.Fatal("no request stored")
	return ""
}

func (l *lifecycle) last(t *testing.T) events.ApprovalEventPayload {
	t.Helper()
	l.pub.mu.Lock()
	defer l.pub.mu.Unlock()
	if len(l.pub.got) == 0 {
		t.Fatal("no event published")
	}
	return l.pub.got[len(l.pub.got)-1]
}

func body(fields map[string]any) string {
	b, _ := json.Marshal(fields)
	return string(b)
}

func sorted(s []string) []string {
	out := append([]string(nil), s...)
	sort.Strings(out)
	return out
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestCreateRequest_EventNamesTenantRequesterAndFirstApprover(t *testing.T) {
	l := newLifecycle(t)
	id := l.create(t)

	evt := l.last(t)
	if evt.RequestID != id || evt.Action != "created" {
		t.Fatalf("event = %+v", evt)
	}
	if evt.TenantID != "tenant-a" || evt.WorkspaceID != "tenant-a" {
		t.Errorf("tenant_id = %q workspace_id = %q, want tenant-a for both", evt.TenantID, evt.WorkspaceID)
	}
	if evt.CreatedBy != "n-req" {
		t.Errorf("created_by = %q, want n-req", evt.CreatedBy)
	}
	if got := sorted(evt.AssigneeNodeIDs); len(got) != 1 || got[0] != "n-ap1" {
		t.Errorf("assignee_node_ids = %v, want [n-ap1]", got)
	}
}

func TestApprove_EventNamesRequesterAndNextApprover(t *testing.T) {
	l := newLifecycle(t)
	id := l.create(t)

	if err := l.call(t, l.h.ApproveAction, "n-ap1", "tenant-a", body(map[string]any{"request_id": id})); err != nil {
		t.Fatalf("approve: %v", err)
	}

	evt := l.last(t)
	if evt.Action != "approved" || evt.ActorNodeID != "n-ap1" {
		t.Fatalf("event = %+v", evt)
	}
	if evt.TenantID != "tenant-a" {
		t.Errorf("tenant_id = %q, want tenant-a", evt.TenantID)
	}
	if evt.CreatedBy != "n-req" {
		t.Errorf("created_by = %q, want n-req — the requester must hear about the approval", evt.CreatedBy)
	}
	if got := sorted(evt.AssigneeNodeIDs); len(got) != 1 || got[0] != "n-ap2" {
		t.Errorf("assignee_node_ids = %v, want [n-ap2] — the next approver", got)
	}
	if evt.TemplateName != "Leave" {
		t.Errorf("template_name = %q, want Leave", evt.TemplateName)
	}
}

func TestReject_EventNamesRequesterAndNoApprover(t *testing.T) {
	l := newLifecycle(t)
	id := l.create(t)

	if err := l.call(t, l.h.RejectAction, "n-ap1", "tenant-a",
		body(map[string]any{"request_id": id, "comment": "no"})); err != nil {
		t.Fatalf("reject: %v", err)
	}

	evt := l.last(t)
	if evt.Action != "rejected" || evt.Status != "rejected" || evt.Comment != "no" {
		t.Fatalf("event = %+v", evt)
	}
	if evt.TenantID != "tenant-a" || evt.CreatedBy != "n-req" {
		t.Errorf("tenant_id = %q created_by = %q, want tenant-a / n-req", evt.TenantID, evt.CreatedBy)
	}
	if len(evt.AssigneeNodeIDs) != 0 {
		t.Errorf("assignee_node_ids = %v, want none on a terminal request", evt.AssigneeNodeIDs)
	}
}

func TestBatchApprove_EventsNameRequesterAndTenant(t *testing.T) {
	l := newLifecycle(t)
	id := l.create(t)

	if err := l.call(t, l.h.BatchApproveAction, "n-ap1", "tenant-a",
		body(map[string]any{"request_ids": []string{id}})); err != nil {
		t.Fatalf("batch approve: %v", err)
	}

	evt := l.last(t)
	if evt.Action != "approved" || evt.RequestID != id {
		t.Fatalf("event = %+v", evt)
	}
	if evt.TenantID != "tenant-a" || evt.CreatedBy != "n-req" {
		t.Errorf("tenant_id = %q created_by = %q, want tenant-a / n-req", evt.TenantID, evt.CreatedBy)
	}
	if got := sorted(evt.AssigneeNodeIDs); len(got) != 1 || got[0] != "n-ap2" {
		t.Errorf("assignee_node_ids = %v, want [n-ap2]", got)
	}
}

// A refused action announces nothing: an event goes out only for something
// that actually happened.
func TestApprove_RefusedPublishesNothing(t *testing.T) {
	l := newLifecycle(t)
	id := l.create(t)
	before := len(l.pub.got)

	err := l.call(t, l.h.ApproveAction, "n-stranger", "tenant-a", body(map[string]any{"request_id": id}))

	if err == nil {
		t.Fatal("a user with no assignment must not be able to approve")
	}
	if len(l.pub.got) != before {
		t.Errorf("a refused approval published %d event(s)", len(l.pub.got)-before)
	}
}

func TestEveryPublishedEventCarriesTenantAndWorkspace(t *testing.T) {
	l := newLifecycle(t)
	id := l.create(t)
	_ = l.call(t, l.h.ApproveAction, "n-ap1", "tenant-a", body(map[string]any{"request_id": id}))
	_ = l.call(t, l.h.RejectAction, "n-ap2", "tenant-a", body(map[string]any{"request_id": id, "comment": "no"}))

	if len(l.pub.got) != 3 {
		t.Fatalf("published %d events, want 3", len(l.pub.got))
	}
	for _, evt := range l.pub.got {
		if evt.TenantID != "tenant-a" || evt.WorkspaceID != "tenant-a" {
			t.Errorf("%s event: tenant_id = %q workspace_id = %q", evt.Action, evt.TenantID, evt.WorkspaceID)
		}
	}
}

// failingInsertStore fails the transaction that writes a new request, the way a
// rolled-back commit does: nothing is stored.
type failingInsertStore struct{ *memStore }

func (failingInsertStore) InsertRequestWithAssignments(context.Context, *domain.Request, []*domain.AssignmentRecord) error {
	return errors.New("commit failed")
}

func TestCreateRequest_RolledBackPublishesNothing(t *testing.T) {
	l := newLifecycle(t)
	l.h = NewHandler(domain.NewService(failingInsertStore{l.store}, allowPolicy{}), nil, l.pub)

	err := l.call(t, l.h.CreateRequest, "n-req", "tenant-a", `{"entity_type":"leave","entity_id":"e-1"}`)

	if err == nil {
		t.Fatal("a failed commit must surface as an error")
	}
	if len(l.pub.got) != 0 {
		t.Errorf("a rolled-back request published %d event(s)", len(l.pub.got))
	}
}

func TestReject_RefusedPublishesNothing(t *testing.T) {
	l := newLifecycle(t)
	id := l.create(t)
	before := len(l.pub.got)

	err := l.call(t, l.h.RejectAction, "n-stranger", "tenant-a", body(map[string]any{"request_id": id, "comment": "x"}))

	if err == nil {
		t.Fatal("a user with no assignment must not be able to reject")
	}
	if len(l.pub.got) != before {
		t.Errorf("a refused rejection published %d event(s)", len(l.pub.got)-before)
	}
}
