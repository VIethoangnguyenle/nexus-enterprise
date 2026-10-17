package domain_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"ngac-platform/services/auth/internal/domain"
)

// A token scoped to a tenant is minted only for somebody the graph holds there.
// The row says who was listed; the graph says who belongs, and the two can differ
// (a row written by an older release, a removal that did not reach the table). Each
// test has the drifted tenant and a healthy one for the same person, and an allow
// on the same fixture, so a gate that refused everything would not pass.

type driftWorld struct {
	w       *fakeWorld
	svc     *domain.Service
	user    userRef
	drifted string // active row, owner, but the graph does not hold the person
	healthy string // active row and the graph holds the person
}

type userRef struct{ id, node, username, email string }

func newDriftWorld(t *testing.T) *driftWorld {
	t.Helper()
	w := newFakeWorld()
	w.rowsLeaveGraphAlone = true
	svc := w.service(t)
	ctx := context.Background()
	u := w.addUser("drift-" + uuid.NewString()[:8] + "@example.test")
	d := &driftWorld{w: w, svc: svc, user: userRef{id: u.ID, node: u.NGACNodeID, username: u.Username, email: u.Email}}
	d.drifted = w.addTenant("Drifted", "")
	d.healthy = w.addTenant("Healthy", "")
	// Owned first, so it is the default whenever the graph does not say otherwise.
	if err := w.InsertTenantUser(ctx, d.drifted, u.ID, "owner", "active", u.NGACNodeID); err != nil {
		t.Fatal(err)
	}
	if err := w.InsertTenantUser(ctx, d.healthy, u.ID, "member", "active", u.NGACNodeID); err != nil {
		t.Fatal(err)
	}
	w.attach(u.NGACNodeID, d.healthy)
	if err := w.LinkIdentity(ctx, domain.ProviderGoogle, "sub-"+u.ID, u.ID, u.Email); err != nil {
		t.Fatal(err)
	}
	return d
}

func (d *driftWorld) google(t *testing.T) *domain.SigninResult {
	t.Helper()
	res, err := d.svc.SignInWithGoogle(context.Background(), googleIdentity("sub-"+d.user.id, d.user.email, ""))
	if err != nil {
		t.Fatalf("sign in must not fail on a tenant question: %v", err)
	}
	return res
}

func tenantIDs(ts []domain.TenantInfo) []string {
	var out []string
	for _, t := range ts {
		out = append(out, t.ID)
	}
	return out
}

func TestSignIn_Google_ATenantTheGraphDoesNotHoldIsNeitherDefaultNorListed(t *testing.T) {
	d := newDriftWorld(t)

	res := d.google(t)
	if res.DefaultTenantID != d.healthy {
		t.Errorf("default tenant = %q, want %q: the person is not in %q", res.DefaultTenantID, d.healthy, d.drifted)
	}
	if got := tenantIDs(res.Tenants); len(got) != 1 || got[0] != d.healthy {
		t.Errorf("tenants = %v, want only %q", got, d.healthy)
	}
	if got := tenantOf(t, res.Token); got != d.healthy {
		t.Errorf("token tenant = %q, want %q", got, d.healthy)
	}

	// Allow, same person: once the graph holds them in the owned tenant it is the default again.
	d.w.attach(d.user.node, d.drifted)
	res = d.google(t)
	if res.DefaultTenantID != d.drifted || len(res.Tenants) != 2 {
		t.Errorf("default = %q with %v, want the owned tenant %q and both listed", res.DefaultTenantID, tenantIDs(res.Tenants), d.drifted)
	}
}

// The policy being down is not a reason to refuse a sign-in, nor to guess: the person
// gets a session with no tenant, and regains one at the next sign-in or by choosing a workspace.
func TestSignIn_Google_PolicyDownGivesATenantlessSessionNotAnError(t *testing.T) {
	d := newDriftWorld(t)

	d.w.policyDown = errors.New("policy: connection refused")
	res := d.google(t)
	if res.DefaultTenantID != "" || tenantOf(t, res.Token) != "" {
		t.Errorf("default = %q, token tenant = %q, want none while the graph cannot be asked", res.DefaultTenantID, tenantOf(t, res.Token))
	}
	if len(res.Tenants) != 0 {
		t.Errorf("tenants = %v, want none: the unfiltered list must not stand in for the answer", tenantIDs(res.Tenants))
	}
	if res.Token == "" {
		t.Error("no session was issued")
	}

	d.w.policyDown = nil
	if res = d.google(t); res.DefaultTenantID != d.healthy {
		t.Errorf("with the policy back, default = %q, want %q", res.DefaultTenantID, d.healthy)
	}
}

func TestSignIn_OTP_ATenantTheGraphDoesNotHoldIsNotTheDefault(t *testing.T) {
	svc, w, _ := otpService(t, nil)
	w.rowsLeaveGraphAlone = true
	ctx := context.Background()
	u := w.addUser(uniqueEmail())
	drifted, healthy := w.addTenant("Drifted", ""), w.addTenant("Healthy", "")
	for tenant, role := range map[string]string{drifted: "owner", healthy: "member"} {
		if err := w.InsertTenantUser(ctx, tenant, u.ID, role, "active", u.NGACNodeID); err != nil {
			t.Fatal(err)
		}
	}
	w.attach(u.NGACNodeID, healthy)

	verify := func() *domain.OTPResult {
		t.Helper()
		sid, err := svc.RequestOTP(ctx, u.Email, "email")
		if err != nil {
			t.Fatal(err)
		}
		res, err := svc.VerifyOTP(ctx, sid, "999999")
		if err != nil {
			t.Fatalf("sign in must not fail on a tenant question: %v", err)
		}
		return res
	}

	if res := verify(); res.TenantID != healthy || tenantOf(t, res.Token) != healthy {
		t.Errorf("tenant = %q, want %q: the person is not in %q", res.TenantID, healthy, drifted)
	}

	w.policyDown = errors.New("policy: connection refused")
	if res := verify(); res.TenantID != "" || tenantOf(t, res.Token) != "" || res.Token == "" {
		t.Errorf("tenant = %q with the policy down, want a session with none", res.TenantID)
	}

	w.policyDown = nil
	w.attach(u.NGACNodeID, drifted)
	if res := verify(); res.TenantID != drifted {
		t.Errorf("tenant = %q once the graph holds them in the owned one, want %q", res.TenantID, drifted)
	}
}

// A new account on a workspace without TenantMember/TenantOwner UAs (the workspace
// made at sign-up never had them) is signed into it and can keep refreshing.
func TestSignIn_OTP_ANewAccountIsSignedIntoItsWorkspaceWithoutTenantMemberUAs(t *testing.T) {
	svc, w, _ := otpService(t, nil)
	w.noTenantUAs = true
	ctx := context.Background()

	sid, err := svc.RequestOTP(ctx, uniqueEmail(), "email")
	if err != nil {
		t.Fatal(err)
	}
	res, err := svc.VerifyOTP(ctx, sid, "999999")
	if err != nil {
		t.Fatal(err)
	}
	if res.TenantID == "" || tenantOf(t, res.Token) != res.TenantID {
		t.Fatalf("tenant = %q, token tenant = %q, want the new workspace", res.TenantID, tenantOf(t, res.Token))
	}
	if m := w.membership(res.TenantID, res.UserID); m == nil || m.Status != "active" || m.Role != "owner" {
		t.Errorf("row = %+v, want an active owner", m)
	}
	if !w.assigned(res.NGACNodeID, ownersOf(res.TenantID)) {
		t.Errorf("the owner is not under the Owners UA; assignments: %v", w.assignments())
	}
	token, err := svc.AttachRefreshToken(ctx, domain.RefreshIdentity{
		UserID: res.UserID, Username: res.Username, NGACNodeID: res.NGACNodeID, TenantID: res.TenantID, SessionID: res.SessionID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, tenant, err := svc.RefreshSession(ctx, token); err != nil || tenant != res.TenantID {
		t.Errorf("refresh = %q, %v; want the workspace to hold", tenant, err)
	}
}

func TestSwitchTenant_RefusesATenantTheGraphDoesNotHold(t *testing.T) {
	d := newDriftWorld(t)
	ctx := context.Background()

	_, _, _, err := d.svc.SwitchTenant(ctx, d.user.id, d.user.node, d.user.username, d.drifted)
	if !errors.Is(err, domain.ErrAccessDenied) {
		t.Errorf("switching to a tenant the graph does not hold: err = %v, want ErrAccessDenied", err)
	}
	token, _, info, err := d.svc.SwitchTenant(ctx, d.user.id, d.user.node, d.user.username, d.healthy)
	if err != nil {
		t.Fatalf("switching to a tenant the graph holds: %v", err)
	}
	if tenantOf(t, token) != d.healthy || info.ID != d.healthy {
		t.Errorf("token tenant = %q, info = %+v", tenantOf(t, token), info)
	}
}

// An outage is "try again", not "no": the caller is told the service is unavailable.
func TestSwitchTenant_ThePolicyBeingDownIsUnavailableNotDenied(t *testing.T) {
	d := newDriftWorld(t)
	d.w.policyDown = errors.New("policy: connection refused")

	token, _, _, err := d.svc.SwitchTenant(context.Background(), d.user.id, d.user.node, d.user.username, d.healthy)
	if !errors.Is(err, domain.ErrUnavailable) || errors.Is(err, domain.ErrAccessDenied) {
		t.Errorf("err = %v, want ErrUnavailable and not ErrAccessDenied", err)
	}
	if token != "" {
		t.Error("a token was issued without an answer from the graph")
	}
}

func workspaceIDs(ws []domain.WorkspaceInfo) []string {
	var out []string
	for _, w := range ws {
		out = append(out, w.ID)
	}
	return out
}

func TestListWorkspaces_TheSwitcherListsOnlyTenantsTheGraphHolds(t *testing.T) {
	d := newDriftWorld(t)
	ctx := context.Background()

	got, err := d.svc.ListMyWorkspacesAt(ctx, d.user.id, d.user.node, d.healthy)
	if err != nil {
		t.Fatal(err)
	}
	if ids := workspaceIDs(got); len(ids) != 1 || ids[0] != d.healthy {
		t.Errorf("switcher = %v, want only %q", ids, d.healthy)
	}

	d.w.attach(d.user.node, d.drifted)
	got, err = d.svc.ListMyWorkspacesAt(ctx, d.user.id, d.user.node, d.healthy)
	if err != nil || len(got) != 2 {
		t.Errorf("switcher = %v, %v; want both once the graph holds the person in both", workspaceIDs(got), err)
	}
}

// With the policy down the switcher shows the workspace the caller is already in (their
// token vouches for it) and never the unfiltered list.
func TestListWorkspaces_ThePolicyBeingDownShowsOnlyTheCurrentWorkspace(t *testing.T) {
	d := newDriftWorld(t)
	d.w.policyDown = errors.New("policy: connection refused")

	got, err := d.svc.ListMyWorkspacesAt(context.Background(), d.user.id, d.user.node, d.healthy)
	if err != nil {
		t.Fatalf("an outage must not fail the list: %v", err)
	}
	if ids := workspaceIDs(got); len(ids) != 1 || ids[0] != d.healthy {
		t.Errorf("switcher = %v, want only the current workspace %q", ids, d.healthy)
	}

	// With no workspace in the token there is nothing to show and nothing to guess:
	// the caller is told to try again.
	_, err = d.svc.ListMyWorkspacesAt(context.Background(), d.user.id, d.user.node, "")
	if !errors.Is(err, domain.ErrUnavailable) {
		t.Errorf("with no current workspace: err = %v, want ErrUnavailable", err)
	}
}
