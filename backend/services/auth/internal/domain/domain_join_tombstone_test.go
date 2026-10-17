package domain_test

import (
	"context"
	"testing"
)

// A person who was removed from a company workspace is not brought back by signing
// in with the company's Google Workspace account: the removal is a decision somebody
// made, and the domain claim is not an invitation. Their row stays a tombstone, the
// graph is not touched, and the workspace is not their default.

func TestSignInWithGoogle_HostedDomainDoesNotReAddARemovedMember(t *testing.T) {
	w := newFakeWorld()
	svc := w.service(t)
	acme := w.addTenant("Acme", "acme.com")
	ctx := context.Background()

	gone := w.addUser("gone@acme.com")
	if err := w.InsertTenantUser(ctx, acme, gone.ID, "member", "removed", gone.NGACNodeID); err != nil {
		t.Fatal(err)
	}
	stays := w.addUser("stays@acme.com")
	if err := w.InsertTenantUser(ctx, acme, stays.ID, "member", "active", stays.NGACNodeID); err != nil {
		t.Fatal(err)
	}
	w.attach(stays.NGACNodeID, acme)

	// Allow: a current member signing in with the company account lands in the company workspace.
	res, err := svc.SignInWithGoogle(ctx, googleIdentity("sub-stays", "stays@acme.com", "acme.com"))
	if err != nil {
		t.Fatalf("sign in: %v", err)
	}
	if res.DefaultTenantID != acme {
		t.Errorf("a member's default tenant = %q, want the company workspace %q", res.DefaultTenantID, acme)
	}

	// Deny: the removed person signs in with the same proof.
	res, err = svc.SignInWithGoogle(ctx, googleIdentity("sub-gone", "gone@acme.com", "acme.com"))
	if err != nil {
		t.Fatalf("sign in: %v", err)
	}
	if m := w.membership(acme, gone.ID); m == nil || m.Status != "removed" {
		t.Errorf("membership = %+v, want the tombstone left as it was", m)
	}
	if w.graph[gone.NGACNodeID][acme] {
		t.Error("the removed person was put back under the workspace's Members UA")
	}
	if res.DefaultTenantID == acme {
		t.Error("the workspace they were removed from is their default tenant")
	}
	for _, tn := range res.Tenants {
		if tn.ID == acme {
			t.Errorf("the removed person is offered %q in their workspace list", tn.ID)
		}
	}
}

// A disabled membership is left alone too (signing in must not re-activate it), and
// it is no more a default tenant than a removed one.
func TestSignInWithGoogle_HostedDomainLeavesADisabledMemberDisabled(t *testing.T) {
	w := newFakeWorld()
	svc := w.service(t)
	acme := w.addTenant("Acme", "acme.com")
	ctx := context.Background()
	u := w.addUser("susp@acme.com")
	if err := w.InsertTenantUser(ctx, acme, u.ID, "member", "disabled", u.NGACNodeID); err != nil {
		t.Fatal(err)
	}

	res, err := svc.SignInWithGoogle(ctx, googleIdentity("sub-susp", "susp@acme.com", "acme.com"))
	if err != nil {
		t.Fatalf("sign in: %v", err)
	}
	if m := w.membership(acme, u.ID); m == nil || m.Status != "disabled" {
		t.Errorf("membership = %+v, want still disabled", m)
	}
	if res.DefaultTenantID == acme {
		t.Error("a suspended member's default tenant is the workspace that suspended them")
	}
}
