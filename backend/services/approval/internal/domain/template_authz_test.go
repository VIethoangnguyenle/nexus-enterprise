package domain

import (
	"context"
	"errors"
	"testing"
	"time"
)

const tenant = "tenant-1"

var oneStep = []StepInput{{StepOrder: 1, Name: "Duyệt", ApproverType: ApproverSpecificUser, ApproverValue: "approver1"}}

// admin may manage the tenant's management attribute ("mgmt-oa"); member may not.
func authzFixture() (*Service, *mockStore, *mockPolicy) {
	ms := newMockStore()
	mp := &mockPolicy{allowedBy: map[string]bool{"admin|mgmt-oa|manage": true}}
	return NewService(ms, mp), ms, mp
}

func TestCreateTemplate_NeedsManageOnTheTenant(t *testing.T) {
	svc, ms, _ := authzFixture()
	ctx := context.Background()
	in := CreateTemplateInput{TenantID: tenant, Name: "T", EntityType: "expense", Steps: oneStep}

	for name, caller := range map[string]string{"a member": "member", "nobody": ""} {
		if _, err := svc.CreateTemplate(ctx, caller, in); !errors.Is(err, ErrAccessDenied) {
			t.Errorf("%s: err = %v, want ErrAccessDenied", name, err)
		}
	}
	// A tenant with no management attribute denies everyone, administrators included.
	other := in
	other.TenantID = "tenant-2"
	if _, err := svc.CreateTemplate(ctx, "admin", other); !errors.Is(err, ErrAccessDenied) {
		t.Errorf("tenant without a management attribute: err = %v, want ErrAccessDenied", err)
	}
	if len(ms.templates) != 0 {
		t.Fatalf("a denied caller stored %d template(s)", len(ms.templates))
	}
	if _, err := svc.CreateTemplate(ctx, "admin", in); err != nil {
		t.Fatalf("administrator: %v", err)
	}
}

func TestUpdateTemplate_NeedsManageOnTheTenant(t *testing.T) {
	svc, ms, _ := authzFixture()
	ctx := context.Background()
	at := time.Now().Truncate(time.Microsecond)
	ms.templates = append(ms.templates, &Template{ID: "t1", Name: "Old", EntityType: "x", IsActive: true, UpdatedAt: at,
		Steps: []*Step{{StepOrder: 1, ApproverType: ApproverSpecificUser, ApproverValue: "approver1", RequiredCount: 1}}})

	// The attack this closes: a member rewriting the chain to name themself.
	evil := UpdateTemplateInput{TenantID: tenant, Name: "Old", IsActive: true, ExpectedUpdatedAt: at,
		Steps: []StepInput{{StepOrder: 1, Name: "Me", ApproverType: ApproverSpecificUser, ApproverValue: "member"}}}
	if _, err := svc.UpdateTemplate(ctx, "member", "t1", evil); !errors.Is(err, ErrAccessDenied) {
		t.Fatalf("member: err = %v, want ErrAccessDenied", err)
	}
	if got := ms.templates[0].Steps[0].ApproverValue; got != "approver1" {
		t.Fatalf("a denied update changed the chain: approver = %q", got)
	}
	if _, err := svc.UpdateTemplate(ctx, "", "t1", evil); !errors.Is(err, ErrAccessDenied) {
		t.Errorf("anonymous: err = %v, want ErrAccessDenied", err)
	}
	if _, err := svc.UpdateTemplate(ctx, "admin", "t1", evil); err != nil {
		t.Fatalf("administrator: %v", err)
	}
}

func TestCanManageTemplates_IsAnAnswerNotAnError(t *testing.T) {
	svc, _, _ := authzFixture()
	ctx := context.Background()
	if ok, err := svc.CanManageTemplates(ctx, "admin", tenant); !ok || err != nil {
		t.Errorf("admin: %v %v", ok, err)
	}
	if ok, err := svc.CanManageTemplates(ctx, "member", tenant); ok || err != nil {
		t.Errorf("member: %v %v, want false and no error", ok, err)
	}
}

func TestTemplatePolicyFailureIsNotAGrant(t *testing.T) {
	ms := newMockStore()
	svc := NewService(ms, &failingCheck{})
	if _, err := svc.CreateTemplate(context.Background(), "admin", CreateTemplateInput{TenantID: tenant, Name: "T", EntityType: "x", Steps: oneStep}); err == nil || len(ms.templates) != 0 {
		t.Fatalf("err = %v templates=%d, want a refusal", err, len(ms.templates))
	}
}

type failingCheck struct{ mockPolicy }

func (failingCheck) CheckAccess(context.Context, string, string, string) (bool, error) {
	return false, errors.New("policy down")
}

// --- what a saved chain may contain ---

func TestTemplateSteps_AreValidatedOnSave(t *testing.T) {
	svc, ms, _ := authzFixture()
	ctx := context.Background()
	step := func(order int, typ, val string) StepInput {
		return StepInput{StepOrder: order, Name: "S", ApproverType: typ, ApproverValue: val}
	}
	for name, steps := range map[string][]StepInput{
		"none":             nil,
		"starts at 2":      {step(2, ApproverSpecificUser, "approver1")},
		"gap":              {step(1, ApproverSpecificUser, "approver1"), step(3, ApproverSpecificUser, "approver1")},
		"duplicate order":  {step(1, ApproverSpecificUser, "approver1"), step(1, ApproverSpecificUser, "approver1")},
		"unsupported kind": {step(1, "creator_manager", "approver1")},
		"empty approver":   {step(1, ApproverSpecificUser, "")},
	} {
		if _, err := svc.CreateTemplate(ctx, "admin", CreateTemplateInput{TenantID: tenant, Name: "T", EntityType: "x", Steps: steps}); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("%s: err = %v, want ErrInvalidInput", name, err)
		}
	}
	if len(ms.templates) != 0 {
		t.Fatalf("an invalid chain was stored")
	}
	// Orders may arrive shuffled; they are stored 1..n.
	tmpl, err := svc.CreateTemplate(ctx, "admin", CreateTemplateInput{TenantID: tenant, Name: "T", EntityType: "x",
		Steps: []StepInput{step(2, ApproverSpecificUser, "approver1"), step(1, ApproverSpecificUser, "approver1")}})
	if err != nil || tmpl.Steps[0].StepOrder != 1 || tmpl.Steps[1].StepOrder != 2 {
		t.Fatalf("shuffled steps: %v %+v", err, tmpl)
	}
}

func TestTemplateApprovers_MustExistInTheTenant(t *testing.T) {
	svc, ms, _ := authzFixture()
	ms.approvers = map[string]bool{"known-ua": true, "known-user": true}
	ms.departments = map[string]string{"dept-1": "dept-ua"}
	ctx := context.Background()
	mk := func(typ, val string) CreateTemplateInput {
		return CreateTemplateInput{TenantID: tenant, Name: "T", EntityType: "x",
			Steps: []StepInput{{StepOrder: 1, Name: "S", ApproverType: typ, ApproverValue: val}}}
	}
	if _, err := svc.CreateTemplate(ctx, "admin", mk(ApproverRole, "ua-of-another-tenant")); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("foreign role: err = %v, want ErrInvalidInput", err)
	}
	if _, err := svc.CreateTemplate(ctx, "admin", mk(ApproverSpecificUser, "someone-else")); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("foreign user: err = %v, want ErrInvalidInput", err)
	}
	tmpl, err := svc.CreateTemplate(ctx, "admin", mk(ApproverDepartment, "dept-1"))
	if err != nil || tmpl.Steps[0].ApproverValue != "dept-ua" {
		t.Fatalf("a department id is stored as its UA: %v %+v", err, tmpl)
	}
}

// --- editing ---

func editFixture() (*Service, *mockStore, time.Time) {
	svc, ms, _ := authzFixture()
	at := time.Now().Truncate(time.Microsecond)
	ms.templates = append(ms.templates, &Template{
		ID: "t1", Name: "Old", EntityType: "expense", IsActive: true, UpdatedAt: at,
		Conditions: []*Condition{{ID: "c", Field: "amount", Operator: "lt", Value: "10000000"}},
		FormFields: []*FormField{{Label: "Số tiền", FieldType: "currency", FieldOrder: 1}},
		Steps:      []*Step{{StepOrder: 1, ApproverType: ApproverSpecificUser, ApproverValue: "approver1", RequiredCount: 1}},
	})
	return svc, ms, at
}

func TestUpdateTemplate_NilKeepsEmptyClears(t *testing.T) {
	svc, ms, at := editFixture()
	ctx := context.Background()
	in := UpdateTemplateInput{TenantID: tenant, Name: "A", IsActive: true, ExpectedUpdatedAt: at}
	got, err := svc.UpdateTemplate(ctx, "admin", "t1", in)
	if err != nil || len(got.Conditions) != 1 || len(got.FormFields) != 1 || len(got.Steps) != 1 {
		t.Fatalf("nothing sent must keep everything: %v %+v", err, got)
	}
	in.ExpectedUpdatedAt = got.UpdatedAt
	in.Conditions, in.FormFields = []ConditionInput{}, []FormFieldInput{}
	got, err = svc.UpdateTemplate(ctx, "admin", "t1", in)
	if err != nil || len(got.Conditions) != 0 || len(got.FormFields) != 0 {
		t.Fatalf("empty lists must clear: %v conditions=%d fields=%d", err, len(got.Conditions), len(got.FormFields))
	}
	// A template cannot lose its last step.
	in.ExpectedUpdatedAt = got.UpdatedAt
	in.Steps = []StepInput{}
	if _, err := svc.UpdateTemplate(ctx, "admin", "t1", in); !errors.Is(err, ErrInvalidInput) {
		t.Errorf("clearing the steps: err = %v, want ErrInvalidInput", err)
	}
	_ = ms
}

func TestUpdateTemplate_RefusesAStaleOrMissingPrecondition(t *testing.T) {
	svc, ms, at := editFixture()
	ctx := context.Background()
	in := UpdateTemplateInput{TenantID: tenant, Name: "Mine", IsActive: true, ExpectedUpdatedAt: at.Add(-time.Minute)}
	if _, err := svc.UpdateTemplate(ctx, "admin", "t1", in); !errors.Is(err, ErrStale) {
		t.Fatalf("stale: err = %v, want ErrStale", err)
	}
	if ms.templates[0].Name != "Old" {
		t.Error("a stale update overwrote the template")
	}
	in.ExpectedUpdatedAt = time.Time{}
	if _, err := svc.UpdateTemplate(ctx, "admin", "t1", in); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("no precondition: err = %v, want ErrInvalidInput", err)
	}
	// Two edits from the same read: the second loses.
	ok := UpdateTemplateInput{TenantID: tenant, Name: "First", IsActive: true, ExpectedUpdatedAt: at}
	if _, err := svc.UpdateTemplate(ctx, "admin", "t1", ok); err != nil {
		t.Fatal(err)
	}
	ok.Name = "Second"
	if _, err := svc.UpdateTemplate(ctx, "admin", "t1", ok); !errors.Is(err, ErrStale) {
		t.Fatalf("second edit from the same read: err = %v, want ErrStale", err)
	}
}

func TestUpdateTemplate_ValidatesWhatItWrites(t *testing.T) {
	svc, _, at := editFixture()
	ctx := context.Background()
	for name, in := range map[string]UpdateTemplateInput{
		"duplicate step order": {Steps: []StepInput{
			{StepOrder: 1, ApproverType: ApproverSpecificUser, ApproverValue: "a"}, {StepOrder: 1, ApproverType: ApproverSpecificUser, ApproverValue: "a"}}},
		"gap in steps":       {Steps: []StepInput{{StepOrder: 2, ApproverType: ApproverSpecificUser, ApproverValue: "a"}}},
		"unknown kind":       {Steps: []StepInput{{StepOrder: 1, ApproverType: "creator_manager", ApproverValue: "a"}}},
		"bad operator":       {Conditions: []ConditionInput{{Field: "amount", Operator: "near", Value: "1"}}},
		"value not JSON":     {Conditions: []ConditionInput{{Field: "amount", Operator: "gt", Value: "ten"}}},
		"condition no field": {Conditions: []ConditionInput{{Operator: "gt", Value: "1"}}},
	} {
		in.TenantID, in.Name, in.IsActive, in.ExpectedUpdatedAt = tenant, "N", true, at
		if _, err := svc.UpdateTemplate(ctx, "admin", "t1", in); !errors.Is(err, ErrInvalidInput) {
			t.Errorf("%s: err = %v, want ErrInvalidInput (400, not 500)", name, err)
		}
	}
}

// --- choosing a template for a form ---

func conditionedFixture() (*Service, *mockStore) {
	svc, ms, _ := authzFixture()
	fields := []*FormField{{Label: "Số tiền", FieldType: "currency", FieldOrder: 1}}
	step := []*Step{{StepOrder: 1, ApproverType: ApproverSpecificUser, ApproverValue: "approver1", RequiredCount: 1}}
	ms.templates = append(ms.templates,
		&Template{ID: "small", Name: "Dưới 10 triệu", EntityType: "expense", IsActive: true, Priority: 1, FormFields: fields, Steps: step,
			Conditions: []*Condition{{Field: "amount", Operator: "lt", Value: "10000000"}}},
		&Template{ID: "big", Name: "Từ 10 triệu", EntityType: "expense", IsActive: true, Priority: 5, FormFields: fields, Steps: step,
			Conditions: []*Condition{{Field: "amount", Operator: "gte", Value: "10000000"}}},
	)
	return svc, ms
}

func TestChosenTemplate_CannotBypassItsConditions(t *testing.T) {
	svc, ms := conditionedFixture()
	ctx := context.Background()
	// 500 million through the "under 10 million" chain.
	_, err := svc.CreateApprovalRequest(ctx, CreateRequestInput{TemplateID: "small", CreatedBy: "u", FormDataJSON: `{"Số tiền":"500000000"}`})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
	if len(ms.requests) != 0 {
		t.Fatal("a request was created through a template whose conditions do not hold")
	}
	// A form that states no amount does not meet "amount < 10M" either.
	if _, err := svc.CreateApprovalRequest(ctx, CreateRequestInput{TemplateID: "small", CreatedBy: "u", FormDataJSON: `{}`}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("no amount: err = %v, want ErrInvalidInput", err)
	}
	// The same form is accepted where it belongs.
	req, err := svc.CreateApprovalRequest(ctx, CreateRequestInput{TemplateID: "small", CreatedBy: "u", FormDataJSON: `{"Số tiền":"5000000"}`})
	if err != nil || req.TemplateID != "small" {
		t.Fatalf("small amount, small template: %v %+v", err, req)
	}
}

func TestChosenTemplate_CannotDodgeAHigherPriorityOne(t *testing.T) {
	svc, ms, _ := authzFixture()
	step := []*Step{{StepOrder: 1, ApproverType: ApproverSpecificUser, ApproverValue: "approver1", RequiredCount: 1}}
	ms.templates = append(ms.templates,
		&Template{ID: "lax", EntityType: "x", IsActive: true, Priority: 1, Steps: step},
		&Template{ID: "strict", EntityType: "x", IsActive: true, Priority: 9, Steps: step},
	)
	ctx := context.Background()
	if _, err := svc.CreateApprovalRequest(ctx, CreateRequestInput{TemplateID: "lax", CreatedBy: "u"}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput: the stricter template also applies", err)
	}
	// An inactive higher-priority template does not stand in the way.
	ms.templates[1].IsActive = false
	if _, err := svc.CreateApprovalRequest(ctx, CreateRequestInput{TemplateID: "lax", CreatedBy: "u"}); err != nil {
		t.Fatalf("with the stricter one switched off: %v", err)
	}
}

func TestRouting_JudgesTheFormNotTheClientsFigures(t *testing.T) {
	svc, _ := conditionedFixture()
	ctx := context.Background()
	// The client says 1 000 000 in entity_fields, the form says 500 000 000: the form decides.
	req, err := svc.CreateApprovalRequest(ctx, CreateRequestInput{
		EntityType: "expense", CreatedBy: "u", FormDataJSON: `{"Số tiền":"500000000"}`,
	})
	if err != nil || req.TemplateID != "big" {
		t.Fatalf("500M routed to %q (%v), want the big template", req.TemplateID, err)
	}
	req, err = svc.CreateApprovalRequest(ctx, CreateRequestInput{
		EntityType: "expense", CreatedBy: "u", FormDataJSON: `{"Số tiền":"3000000"}`,
	})
	if err != nil || req.TemplateID != "small" {
		t.Fatalf("3M routed to %q (%v), want the small template", req.TemplateID, err)
	}
	if _, err := svc.CreateApprovalRequest(ctx, CreateRequestInput{EntityType: "expense", CreatedBy: "u", FormDataJSON: `{}`}); !errors.Is(err, ErrNoMatchingTemplate) {
		t.Errorf("no amount, every template wants one: err = %v, want ErrNoMatchingTemplate", err)
	}
}
