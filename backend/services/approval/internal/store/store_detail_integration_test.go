//go:build integration
// +build integration

package store_test

import (
	"errors"
	"testing"
	"time"

	"ngac-platform/services/approval/internal/domain"
	"ngac-platform/services/approval/internal/store"
)

func twoStepTemplate(name string) *domain.Template {
	now := time.Now()
	return &domain.Template{
		ID: newID(), Name: name, EntityType: "expense", IsActive: true, Priority: 3, CreatedBy: "admin",
		CreatedAt: now, UpdatedAt: now,
		Conditions: []*domain.Condition{{ID: newID(), Field: "amount", Operator: "gt", Value: "100"}},
		Steps: []*domain.Step{
			{ID: newID(), StepOrder: 1, Name: "Trưởng phòng", ApproverType: "specific_user", ApproverValue: "mgr", RequiredCount: 1},
			{ID: newID(), StepOrder: 2, Name: "Giám đốc", ApproverType: "specific_user", ApproverValue: "dir", RequiredCount: 1},
		},
	}
}

// Editing a template used to save its name, flag and form fields and silently
// drop the new steps and conditions, so the builder showed a chain the
// database never got.
func TestUpdateTemplate_ReplacesStepsAndConditions(t *testing.T) {
	s := store.NewStore(testDB)
	ctx := tenantCtx(tenantAID)
	tmpl := twoStepTemplate("Replace chain")
	if err := s.InsertTemplate(ctx, tmpl); err != nil {
		t.Fatalf("insert: %v", err)
	}

	tmpl.Name = "Replaced chain"
	tmpl.Steps = []*domain.Step{
		{ID: newID(), StepOrder: 1, Name: "Kế toán trưởng", ApproverType: "specific_user", ApproverValue: "chief", RequiredCount: 2},
	}
	tmpl.Conditions = nil
	if _, err := s.UpdateTemplate(ctx, tmpl, tmpl.UpdatedAt); err != nil {
		t.Fatalf("update: %v", err)
	}

	got, err := s.GetTemplate(ctx, tmpl.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Name != "Replaced chain" {
		t.Errorf("name = %q", got.Name)
	}
	if len(got.Steps) != 1 || got.Steps[0].Name != "Kế toán trưởng" || got.Steps[0].RequiredCount != 2 {
		t.Errorf("steps = %+v, want the one new step", got.Steps)
	}
	if len(got.Conditions) != 0 {
		t.Errorf("conditions = %+v, want none", got.Conditions)
	}
}

func TestUpdateTemplate_KeepsStepsWhenNotChanged(t *testing.T) {
	s := store.NewStore(testDB)
	ctx := tenantCtx(tenantAID)
	tmpl := twoStepTemplate("Keep chain")
	if err := s.InsertTemplate(ctx, tmpl); err != nil {
		t.Fatalf("insert: %v", err)
	}
	loaded, _ := s.GetTemplate(ctx, tmpl.ID)
	loaded.IsActive = false // what the domain does for a rename or a switch-off
	if _, err := s.UpdateTemplate(ctx, loaded, loaded.UpdatedAt); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, _ := s.GetTemplate(ctx, tmpl.ID)
	if got.IsActive || len(got.Steps) != 2 || len(got.Conditions) != 1 {
		t.Errorf("active=%v steps=%d conditions=%d, want off/2/1", got.IsActive, len(got.Steps), len(got.Conditions))
	}
}

func TestGetTemplate_UnknownIsNotFound(t *testing.T) {
	s := store.NewStore(testDB)
	if _, err := s.GetTemplate(tenantCtx(tenantAID), newID()); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestListAssignments_EveryStepInOrder(t *testing.T) {
	s := store.NewStore(testDB)
	ctx := tenantCtx(tenantAID)
	tmpl := twoStepTemplate("Assignments")
	if err := s.InsertTemplate(ctx, tmpl); err != nil {
		t.Fatalf("insert template: %v", err)
	}
	reqID := newID()
	if err := s.InsertRequest(ctx, &domain.Request{
		ID: reqID, EntityType: "expense", EntityID: newID(), TemplateID: tmpl.ID, TemplateName: tmpl.Name,
		TemplateSnapshot: `{}`, CurrentStep: 1, Status: "pending", ScopeOAID: "s", DepartmentID: "d", CreatedBy: "u1",
	}); err != nil {
		t.Fatalf("insert request: %v", err)
	}
	// Inserted out of order on purpose.
	if err := s.InsertAssignments(ctx, []*domain.AssignmentRecord{
		{ID: newID(), RequestID: reqID, StepOrder: 2, UserNodeID: "dir", GrantSource: "direct", Status: "pending"},
		{ID: newID(), RequestID: reqID, StepOrder: 1, UserNodeID: "mgr", GrantSource: "direct", Status: "pending"},
	}); err != nil {
		t.Fatalf("insert assignments: %v", err)
	}

	got, err := s.ListAssignments(ctx, reqID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 2 || got[0].UserNodeID != "mgr" || got[1].UserNodeID != "dir" {
		t.Errorf("assignments = %+v, want step 1 then step 2", got)
	}
	if other, _ := s.ListAssignments(ctx, newID()); len(other) != 0 {
		t.Errorf("another request's assignments leaked: %+v", other)
	}
}

// The rows of "my requests" and the department list show the amount, which
// lives in the submitted form, so both queries must carry it.
func TestRequestLists_CarryTheSubmittedForm(t *testing.T) {
	s := store.NewStore(testDB)
	ctx := tenantCtx(tenantAID)
	tmpl := twoStepTemplate("Form carry")
	if err := s.InsertTemplate(ctx, tmpl); err != nil {
		t.Fatalf("insert template: %v", err)
	}
	reqID := newID()
	scope := "scope-form-" + reqID[:8]
	creator := "creator-form-" + reqID[:8]
	if err := s.InsertRequest(ctx, &domain.Request{
		ID: reqID, EntityType: "expense", EntityID: newID(), TemplateID: tmpl.ID, TemplateName: tmpl.Name,
		TemplateSnapshot: `{}`, FormDataJSON: `{"Số tiền":"12450000"}`, CurrentStep: 1, Status: "pending",
		ScopeOAID: scope, DepartmentID: "d", CreatedBy: creator,
	}); err != nil {
		t.Fatalf("insert request: %v", err)
	}

	mine, _, err := s.ListMyRequests(ctx, creator, "", 10)
	if err != nil || len(mine) != 1 {
		t.Fatalf("my requests: %v %d", err, len(mine))
	}
	if mine[0].FormDataJSON != `{"Số tiền": "12450000"}` && mine[0].FormDataJSON != `{"Số tiền":"12450000"}` {
		t.Errorf("my-requests form = %q", mine[0].FormDataJSON)
	}
	dept, _, err := s.ListByScopes(ctx, []string{scope}, "", 10)
	if err != nil || len(dept) != 1 || dept[0].FormDataJSON == "" {
		t.Errorf("department form = %+v err=%v", dept, err)
	}
}

// --- groups, concurrency, atomic create ---

func newRequest(t *testing.T, s *store.Store, tmplID string, as []*domain.AssignmentRecord) string {
	t.Helper()
	ctx := tenantCtx(tenantAID)
	reqID := newID()
	for _, a := range as {
		a.RequestID = reqID
	}
	if err := s.InsertRequestWithAssignments(ctx, &domain.Request{
		ID: reqID, EntityType: "expense", EntityID: newID(), TemplateID: tmplID, TemplateName: "G",
		TemplateSnapshot: `{}`, CurrentStep: 1, Status: "pending", ScopeOAID: "s", DepartmentID: "d", CreatedBy: "u1",
	}, as); err != nil {
		t.Fatalf("insert request: %v", err)
	}
	return reqID
}

func groupRow(ua string) *domain.AssignmentRecord {
	return &domain.AssignmentRecord{ID: newID(), StepOrder: 1, UserNodeID: ua, GrantSource: "role:" + ua, Status: "pending"}
}

func TestGroupRows_FindActAndNeverTwice(t *testing.T) {
	s := store.NewStore(testDB)
	ctx := tenantCtx(tenantAID)
	tmpl := twoStepTemplate("Group rows")
	if err := s.InsertTemplate(ctx, tmpl); err != nil {
		t.Fatal(err)
	}
	ua := "ua-" + newID()
	reqID := newRequest(t, s, tmpl.ID, []*domain.AssignmentRecord{groupRow(ua)})

	if _, err := s.FindGroupAssignment(ctx, reqID, 1, []string{"not-the-ua"}); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("another group: err = %v, want ErrNotFound", err)
	}
	if _, err := s.FindGroupAssignment(ctx, reqID, 2, []string{ua}); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("another step: err = %v, want ErrNotFound", err)
	}
	g, err := s.FindGroupAssignment(ctx, reqID, 1, []string{"x", ua})
	if err != nil || g.UserNodeID != ua {
		t.Fatalf("group row: %v %+v", err, g)
	}

	person := &domain.AssignmentRecord{ID: newID(), RequestID: reqID, StepOrder: 1, UserNodeID: "alice", GrantSource: "role:" + ua, Status: "approved"}
	if err := s.InsertActedAssignment(ctx, person); err != nil {
		t.Fatalf("first decision: %v", err)
	}
	again := *person
	again.ID = newID()
	if err := s.InsertActedAssignment(ctx, &again); !errors.Is(err, domain.ErrAlreadyExists) {
		t.Fatalf("second decision by the same person: err = %v, want ErrAlreadyExists", err)
	}
	if n, _ := s.CountApprovedForStep(ctx, reqID, 1); n != 1 {
		t.Errorf("quorum counts people: %d, want 1", n)
	}
	// A person's row is not a group row, so it is never offered to someone else.
	if _, err := s.FindGroupAssignment(ctx, reqID, 1, []string{"alice"}); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("a person's own row found as a group's: %v", err)
	}
}

func TestListPending_GroupRowsForMembersOnly_AndActedStepsDropOut(t *testing.T) {
	s := store.NewStore(testDB)
	ctx := tenantCtx(tenantAID)
	tmpl := twoStepTemplate("Pending groups")
	_ = s.InsertTemplate(ctx, tmpl)
	ua := "ua-" + newID()
	reqID := newRequest(t, s, tmpl.ID, []*domain.AssignmentRecord{groupRow(ua)})
	has := func(user string, groups []string) bool {
		items, err := s.ListPending(ctx, user, groups)
		if err != nil {
			t.Fatal(err)
		}
		for _, it := range items {
			if it.Request.ID == reqID {
				return true
			}
		}
		return false
	}
	if !has("alice", []string{ua}) {
		t.Error("a member does not see the group's request")
	}
	if has("carol", []string{"ua-other"}) || has("dave", nil) {
		t.Error("someone outside the group sees it")
	}
	// A row that merely names the group in its grant source, on a person, is not a group row.
	if err := s.InsertActedAssignment(ctx, &domain.AssignmentRecord{ID: newID(), RequestID: reqID, StepOrder: 1, UserNodeID: "alice", GrantSource: "role:" + ua, Status: "approved"}); err != nil {
		t.Fatal(err)
	}
	if has("alice", []string{ua}) {
		t.Error("a member who has acted still sees it as waiting on them")
	}
	if !has("bob", []string{ua}) {
		t.Error("the other member lost it")
	}
}

func TestHasAssignment_ThroughAGroup(t *testing.T) {
	s := store.NewStore(testDB)
	ctx := tenantCtx(tenantAID)
	tmpl := twoStepTemplate("Has group")
	_ = s.InsertTemplate(ctx, tmpl)
	ua := "ua-" + newID()
	reqID := newRequest(t, s, tmpl.ID, []*domain.AssignmentRecord{groupRow(ua)})
	if ok, _ := s.HasAssignment(ctx, reqID, []string{"alice", ua}); !ok {
		t.Error("the group's row is not found through its UA")
	}
	if ok, _ := s.HasAssignment(ctx, reqID, []string{"alice", "ua-other"}); ok {
		t.Error("an unrelated group matched")
	}
}

func TestGetAssignment_NoRowIsNotFound(t *testing.T) {
	s := store.NewStore(testDB)
	if _, err := s.GetAssignment(tenantCtx(tenantAID), newID(), "nobody"); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound (a 404, not a 500)", err)
	}
}

// A request is stored with its first assignments or not at all.
func TestInsertRequestWithAssignments_AllOrNothing(t *testing.T) {
	s := store.NewStore(testDB)
	ctx := tenantCtx(tenantAID)
	tmpl := twoStepTemplate("Atomic")
	_ = s.InsertTemplate(ctx, tmpl)
	reqID := newID()
	dup := groupRow("ua-dup")
	err := s.InsertRequestWithAssignments(ctx, &domain.Request{
		ID: reqID, EntityType: "expense", EntityID: newID(), TemplateID: tmpl.ID, TemplateName: "A",
		TemplateSnapshot: `{}`, CurrentStep: 1, Status: "pending", ScopeOAID: "s", DepartmentID: "d", CreatedBy: "u1",
	}, []*domain.AssignmentRecord{
		{ID: dup.ID, RequestID: reqID, StepOrder: 1, UserNodeID: "ua-dup", GrantSource: "role:ua-dup", Status: "pending"},
		{ID: dup.ID, RequestID: reqID, StepOrder: 1, UserNodeID: "ua-dup2", GrantSource: "role:ua-dup2", Status: "pending"}, // same primary key
	})
	if err == nil {
		t.Fatal("expected the second assignment to fail")
	}
	if _, err := s.GetRequest(ctx, reqID); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("the request survived a failed assignment: %v", err)
	}
}

func TestUpdateTemplate_PreconditionIsEnforcedByTheDatabase(t *testing.T) {
	s := store.NewStore(testDB)
	ctx := tenantCtx(tenantAID)
	tmpl := twoStepTemplate("Precondition")
	_ = s.InsertTemplate(ctx, tmpl)
	read, _ := s.GetTemplate(ctx, tmpl.ID)

	read.Name = "First"
	next, err := s.UpdateTemplate(ctx, read, read.UpdatedAt)
	if err != nil || !next.After(read.UpdatedAt) {
		t.Fatalf("first update: %v next=%v", err, next)
	}
	// A second edit made from the same read.
	read.Name = "Second"
	if _, err := s.UpdateTemplate(ctx, read, read.UpdatedAt); !errors.Is(err, domain.ErrStale) {
		t.Fatalf("second edit from the same read: err = %v, want ErrStale", err)
	}
	if got, _ := s.GetTemplate(ctx, tmpl.ID); got.Name != "First" {
		t.Errorf("name = %q, the stale edit must not land", got.Name)
	}
	gone := *read
	gone.ID = newID()
	if _, err := s.UpdateTemplate(ctx, &gone, read.UpdatedAt); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("unknown template: err = %v, want ErrNotFound", err)
	}
}

// Choosing a template for a form judges its conditions, so a list that comes
// without them would let every template match.
func TestListTemplates_CarriesConditions(t *testing.T) {
	s := store.NewStore(testDB)
	ctx := tenantCtx(tenantAID)
	tmpl := twoStepTemplate("With conditions")
	tmpl.EntityType = "cond-" + newID()[:8]
	_ = s.InsertTemplate(ctx, tmpl)
	list, err := s.ListTemplates(ctx, tmpl.EntityType, true)
	if err != nil || len(list) != 1 || len(list[0].Conditions) != 1 || list[0].Conditions[0].Field != "amount" {
		t.Fatalf("list = %+v err=%v", list, err)
	}
}
