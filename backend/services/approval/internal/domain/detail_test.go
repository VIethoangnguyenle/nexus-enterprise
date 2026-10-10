package domain

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

// detailFixture holds req1: created by "requester", scoped to oa-dept, with a
// two-step template snapshot and an assignment on each step.
func detailFixture(policy *mockPolicy) *Service {
	snap, _ := json.Marshal(&Template{
		Name: "Tạm ứng",
		Steps: []*Step{
			{StepOrder: 1, Name: "Trưởng phòng", ApproverType: "specific_user", ApproverValue: "approver"},
			{StepOrder: 2, Name: "Giám đốc", ApproverType: "specific_user", ApproverValue: "boss"},
		},
		FormFields: []*FormField{{Label: "Số tiền", FieldType: "currency", FieldOrder: 1}},
	})
	ms := newMockStore()
	ms.requests[req1ID] = &Request{
		ID: req1ID, CreatedBy: "requester", ScopeOAID: "oa-dept", TemplateName: "Tạm ứng",
		TemplateSnapshot: string(snap),
	}
	ms.requests[req2ID] = &Request{ID: req2ID, CreatedBy: "someone-else", ScopeOAID: "oa-other"}
	ms.assignList = append(ms.assignList,
		&AssignmentRecord{ID: "as1", RequestID: req1ID, StepOrder: 1, UserNodeID: "approver", Status: "approved"},
		&AssignmentRecord{ID: "as2", RequestID: req1ID, StepOrder: 2, UserNodeID: "boss", Status: "pending"},
	)
	return NewService(ms, policy)
}

func TestGetRequestDetail_CarriesStepsFormFieldsAndEveryAssignment(t *testing.T) {
	svc := detailFixture(&mockPolicy{})
	d, err := svc.GetRequestDetail(context.Background(), "requester", req1ID)
	if err != nil {
		t.Fatalf("GetRequestDetail: %v", err)
	}
	if d.Request.ID != req1ID || d.Request.TemplateSnapshot != "" {
		t.Errorf("request = %+v; the frozen snapshot is unpacked, not repeated", d.Request)
	}
	if len(d.Steps) != 2 || d.Steps[0].Name != "Trưởng phòng" || d.Steps[1].Name != "Giám đốc" {
		t.Errorf("steps = %+v, want the two snapshot steps in order", d.Steps)
	}
	if len(d.FormFields) != 1 || d.FormFields[0].Label != "Số tiền" {
		t.Errorf("form fields = %+v", d.FormFields)
	}
	if len(d.Assignments) != 2 {
		t.Errorf("assignments = %d, want both steps' approvers, not only the caller's", len(d.Assignments))
	}
}

func TestGetRequestDetail_AllowedForEachPathToTheRequest(t *testing.T) {
	for _, tc := range []struct {
		name   string
		caller string
		policy *mockPolicy
	}{
		{"requester", "requester", &mockPolicy{}},
		{"assignee of a later step", "boss", &mockPolicy{}},
		{"department scope covers it", "dept-head", &mockPolicy{scopes: []string{"oa-dept"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := detailFixture(tc.policy).GetRequestDetail(context.Background(), tc.caller, req1ID); err != nil {
				t.Fatalf("GetRequestDetail: %v", err)
			}
		})
	}
}

func TestGetRequestDetail_DeniedMissingAndMalformed(t *testing.T) {
	svc := detailFixture(&mockPolicy{scopes: []string{"oa-other"}})
	ctx := context.Background()

	if d, err := svc.GetRequestDetail(ctx, "stranger", req1ID); !errors.Is(err, ErrAccessDenied) || d != nil {
		t.Errorf("stranger: detail=%v err=%v, want ErrAccessDenied and nothing", d, err)
	}
	// Being an approver on req1 grants nothing on req2.
	if _, err := detailFixture(&mockPolicy{}).GetRequestDetail(ctx, "approver", req2ID); !errors.Is(err, ErrAccessDenied) {
		t.Errorf("approver reading req2: err = %v, want ErrAccessDenied", err)
	}
	// A missing request answers like a hidden one.
	if _, err := svc.GetRequestDetail(ctx, "requester", "3f2c1d0e-0000-4000-8000-000000000000"); !errors.Is(err, ErrAccessDenied) {
		t.Errorf("missing: err = %v, want ErrAccessDenied", err)
	}
	if _, err := svc.GetRequestDetail(ctx, "requester", "not-a-uuid"); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("malformed: err = %v, want ErrInvalidInput", err)
	}
	if _, err := svc.GetRequestDetail(ctx, "", req1ID); !errors.Is(err, ErrAccessDenied) {
		t.Errorf("anonymous: err = %v, want ErrAccessDenied", err)
	}
}

func TestGetRequestDetail_ScopeFailureIsNotAGrant(t *testing.T) {
	svc := detailFixture(&mockPolicy{scopes: []string{"oa-dept"}, scopesErr: errors.New("policy down")})
	if d, err := svc.GetRequestDetail(context.Background(), "dept-head", req1ID); err == nil || d != nil {
		t.Errorf("detail=%v err=%v, want an error and nothing", d, err)
	}
}

// A request with a damaged snapshot still opens: the chain falls back to the
// assignments, which are what the caller needs to act.
func TestGetRequestDetail_UnreadableSnapshotStillOpens(t *testing.T) {
	svc := detailFixture(&mockPolicy{})
	svc.store.(*mockStore).requests[req1ID].TemplateSnapshot = "{not json"
	d, err := svc.GetRequestDetail(context.Background(), "requester", req1ID)
	if err != nil {
		t.Fatalf("GetRequestDetail: %v", err)
	}
	if len(d.Steps) != 0 || len(d.Assignments) != 2 {
		t.Errorf("steps=%d assignments=%d", len(d.Steps), len(d.Assignments))
	}
}

// --- Create from a chosen template ---

func TestCreateApprovalRequest_ChosenTemplateNeedsNoEntityIdentity(t *testing.T) {
	svc, ms, _ := setupServiceWithTemplate()
	req, err := svc.CreateApprovalRequest(context.Background(), CreateRequestInput{
		TemplateID: "tmpl-1", CreatedBy: "user1",
	})
	if err != nil {
		t.Fatalf("CreateApprovalRequest: %v", err)
	}
	if req.TemplateID != "tmpl-1" || req.EntityType != "transfer" {
		t.Errorf("template=%q entity_type=%q, want the chosen template's", req.TemplateID, req.EntityType)
	}
	if req.EntityID == "" {
		t.Error("a request with no outside entity still gets its own entity id")
	}
	if _, ok := ms.assignments[req.ID+":approver1"]; !ok {
		t.Error("step 1 approver not assigned")
	}
}

func TestCreateApprovalRequest_ChosenTemplateMustBeUsable(t *testing.T) {
	svc, ms, _ := setupServiceWithTemplate()
	ms.templates[0].IsActive = false
	ctx := context.Background()

	if _, err := svc.CreateApprovalRequest(ctx, CreateRequestInput{TemplateID: "tmpl-1", CreatedBy: "u"}); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("inactive template: err = %v, want ErrInvalidInput", err)
	}
	if _, err := svc.CreateApprovalRequest(ctx, CreateRequestInput{TemplateID: "nope", CreatedBy: "u"}); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown template: err = %v, want ErrNotFound", err)
	}
}

func TestCreateApprovalRequest_EntityTypeStillRequiredWithoutTemplate(t *testing.T) {
	svc, _, _ := setupServiceWithTemplate()
	if _, err := svc.CreateApprovalRequest(context.Background(), CreateRequestInput{CreatedBy: "u"}); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("err = %v, want ErrInvalidInput", err)
	}
}
