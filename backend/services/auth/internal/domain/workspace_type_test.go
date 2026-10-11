package domain_test

import (
	"context"
	"testing"

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

func TestWorkspaceType_CompanyDomainTenantIsAnOrganization(t *testing.T) {
	w := newFakeWorld()
	res, err := w.service(t).SignInWithGoogle(context.Background(), googleIdentity("sub-1", "ceo@newco.io", "newco.io"))
	if err != nil {
		t.Fatal(err)
	}
	if got := w.workspaceType(res.DefaultTenantID); got != domain.WorkspaceOrganization {
		t.Errorf("type = %q, want organization", got)
	}
}

func TestWorkspaceType_CreatedOnPurposeIsAnOrganization(t *testing.T) {
	w := newFakeWorld()
	svc := w.service(t)
	u := verifiedUser(t, w, "maker@example.test")
	ws, err := svc.CreateMyWorkspace(context.Background(), u.ID, u.NGACNodeID, "Tổ Đối soát")
	if err != nil {
		t.Fatal(err)
	}
	if ws.Type != domain.WorkspaceOrganization || w.workspaceType(ws.ID) != domain.WorkspaceOrganization {
		t.Errorf("returned %q, stored %q; want organization", ws.Type, w.workspaceType(ws.ID))
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
