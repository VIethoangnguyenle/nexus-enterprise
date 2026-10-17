package domain_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"ngac-platform/services/auth/internal/domain"
)

// A workspace's type is decided by how it came to be, at the moment it is made:
// the one made for a person on first sign-in is personal; one created on purpose
// or owned by a company domain is an organization.

func TestWorkspaceType_FirstGoogleSignInWithoutACompanyIsPersonal(t *testing.T) {
	w := newFakeWorld()
	res, err := w.service(t).SignInWithGoogle(context.Background(), googleIdentity("sub-1", "someone@gmail.com", ""))
	if err != nil {
		t.Fatal(err)
	}
	if got := w.workspaceType(res.DefaultTenantID); got != domain.WorkspacePersonal {
		t.Errorf("type = %q, want personal", got)
	}
}

// The personal workspace made at sign-up carries the person's name, nothing
// added: a person may have several personal workspaces, and this one is theirs.
func TestPersonalWorkspace_IsNamedAfterThePerson(t *testing.T) {
	w := newFakeWorld()
	res, err := w.service(t).SignInWithGoogle(context.Background(), googleIdentity("sub-1", "alice@gmail.com", ""))
	if err != nil {
		t.Fatal(err)
	}
	if got := w.workspaceName(res.DefaultTenantID); got != "Alice Example" {
		t.Errorf("name = %q, want the person's display name", got)
	}

	svc, ow, _ := otpService(t, nil)
	// A fresh address per run: OTP requests are rate-limited per address in Redis, which outlives the test.
	sid, err := svc.RequestOTP(context.Background(), fmt.Sprintf("lan-%d@example.org", time.Now().UnixNano()), "email")
	if err != nil {
		t.Fatal(err)
	}
	r, err := svc.VerifyOTP(context.Background(), sid, "999999")
	if err != nil {
		t.Fatal(err)
	}
	tenants, _ := ow.ListTenantsByUser(context.Background(), r.UserID)
	if len(tenants) != 1 || ow.workspaceName(tenants[0].TenantID) != r.Username {
		t.Errorf("OTP workspace = %+v, want it named like the account until the person names themselves", tenants)
	}
}

func TestWorkspaceType_CompanyAccountsGetAPersonalWorkspaceToo(t *testing.T) {
	w := newFakeWorld()
	acme := w.addTenant("Acme", "acme.com")
	res, err := w.service(t).SignInWithGoogle(context.Background(), googleIdentity("sub-1", "ceo@acme.com", "acme.com"))
	if err != nil {
		t.Fatal(err)
	}
	tenants, _ := w.ListTenantsByUser(context.Background(), res.UserID)
	if len(tenants) != 2 {
		t.Fatalf("tenants = %+v, want the domain organization and a personal workspace", tenants)
	}
	for _, tm := range tenants {
		if tm.TenantID != acme && w.workspaceType(tm.TenantID) != domain.WorkspacePersonal {
			t.Errorf("own workspace type = %q, want personal", w.workspaceType(tm.TenantID))
		}
	}
}

// "Tạo workspace" makes either kind, as the person chooses; one person may
// have several personal workspaces.
func TestWorkspaceType_CreatedOnPurposeHasTheChosenType(t *testing.T) {
	w := newFakeWorld()
	svc := w.service(t)
	u := verifiedUser(t, w, "maker@example.test")
	for _, typ := range []string{domain.WorkspaceOrganization, domain.WorkspacePersonal, domain.WorkspacePersonal} {
		ws, err := svc.CreateMyWorkspace(context.Background(), u.ID, u.NGACNodeID, "Tổ Đối soát", typ)
		if err != nil {
			t.Fatal(err)
		}
		if ws.Type != typ || w.workspaceType(ws.ID) != typ {
			t.Errorf("returned %q, stored %q; want %q", ws.Type, w.workspaceType(ws.ID), typ)
		}
	}
}

func TestWorkspaceType_CreatingRefusesAnUnknownType(t *testing.T) {
	w := newFakeWorld()
	svc := w.service(t)
	u := verifiedUser(t, w, "maker@example.test")
	before := w.workspaceCount()
	for _, typ := range []string{"", "team"} {
		if _, err := svc.CreateMyWorkspace(context.Background(), u.ID, u.NGACNodeID, "Tổ", typ); !errors.Is(err, domain.ErrInvalidInput) {
			t.Errorf("type %q: err = %v, want invalid input", typ, err)
		}
	}
	if w.workspaceCount() != before {
		t.Error("nothing is created for a type nobody stated")
	}
}

func TestWorkspaceType_FirstOTPSignInIsPersonal(t *testing.T) {
	svc, w, _ := otpService(t, nil)
	sid, err := svc.RequestOTP(context.Background(), uniqueEmail(), "email")
	if err != nil {
		t.Fatal(err)
	}
	res, err := svc.VerifyOTP(context.Background(), sid, "999999")
	if err != nil {
		t.Fatal(err)
	}
	tenants, _ := w.ListTenantsByUser(context.Background(), res.UserID)
	if len(tenants) != 1 || w.workspaceType(tenants[0].TenantID) != domain.WorkspacePersonal {
		t.Fatalf("tenants = %+v, want one personal workspace", tenants)
	}
}

// The type reaches the screens that need it: the picker list and /api/me.
func TestWorkspaceType_IsReportedWithTheWorkspace(t *testing.T) {
	w := newFakeWorld()
	svc := w.service(t)
	res, err := svc.SignInWithGoogle(context.Background(), googleIdentity("sub-1", "someone@gmail.com", ""))
	if err != nil {
		t.Fatal(err)
	}
	list, err := svc.ListMyWorkspaces(context.Background(), res.UserID)
	if err != nil || len(list) != 1 || list[0].Type != domain.WorkspacePersonal {
		t.Fatalf("list = %+v (%v), want one personal", list, err)
	}
	_, tenant, err := svc.GetMe(context.Background(), res.UserID, res.DefaultTenantID)
	if err != nil || tenant == nil || tenant.Type != domain.WorkspacePersonal {
		t.Fatalf("me tenant = %+v (%v), want personal", tenant, err)
	}
}
