package domain_test

import (
	"context"
	"errors"
	"testing"

	"ngac-platform/services/auth/internal/domain"
)

// A person's title belongs to the workspace it was set in: GetMe for one
// workspace shows that workspace's title, another workspace the account default.
func TestWorkspaceTitle_SetInOneWorkspaceShowsOnlyThere(t *testing.T) {
	w := newFakeWorld()
	svc := w.service(t)
	ctx := context.Background()
	u := w.addUser("ttl@example.test")
	a, b := w.addTenant("Khối A", ""), w.addTenant("Khối B", "")
	for _, ws := range []string{a, b} {
		if err := w.InsertTenantUser(ctx, ws, u.ID, "member", "active", u.NGACNodeID); err != nil {
			t.Fatal(err)
		}
	}
	if err := svc.UpdateProfile(ctx, u.ID, domain.ProfileUpdateInput{Title: ptr("Mặc định")}); err != nil {
		t.Fatal(err)
	}

	if err := svc.UpdateProfile(ctx, u.ID, domain.ProfileUpdateInput{Title: ptr("  Kế toán  "), WorkspaceID: a}); err != nil {
		t.Fatal(err)
	}

	title := func(ws string) string {
		t.Helper()
		me, _, err := svc.GetMe(ctx, u.ID, ws)
		if err != nil {
			t.Fatal(err)
		}
		return me.Title
	}
	if got := title(a); got != "Kế toán" {
		t.Errorf("workspace A title = %q, want the one set there, trimmed", got)
	}
	if got := title(b); got != "Mặc định" {
		t.Errorf("workspace B title = %q, want the account default", got)
	}
	if got := title(""); got != "Mặc định" {
		t.Errorf("no workspace: title = %q, want the account default", got)
	}
}

// Naming a workspace the caller is not an active member of is refused with
// ErrAccessDenied, and the title shown elsewhere does not change.
func TestWorkspaceTitle_NonMemberWorkspaceIsDenied(t *testing.T) {
	w := newFakeWorld()
	svc := w.service(t)
	ctx := context.Background()
	u := w.addUser("ttl2@example.test")
	foreign, off := w.addTenant("Khối khác", ""), w.addTenant("Khối cũ", "")
	if err := w.InsertTenantUser(ctx, off, u.ID, "member", "disabled", u.NGACNodeID); err != nil {
		t.Fatal(err)
	}

	for name, ws := range map[string]string{"not a member": foreign, "disabled": off, "no such workspace": "ws-none"} {
		err := svc.UpdateProfile(ctx, u.ID, domain.ProfileUpdateInput{Title: ptr("Giám đốc"), WorkspaceID: ws})
		if !errors.Is(err, domain.ErrAccessDenied) {
			t.Errorf("%s: err = %v, want ErrAccessDenied", name, err)
		}
	}
	me, _, _ := svc.GetMe(ctx, u.ID, "")
	if me.Title != "" {
		t.Errorf("a refused write changed the title to %q", me.Title)
	}
}
