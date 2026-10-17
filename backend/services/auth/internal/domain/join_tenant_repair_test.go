package domain_test

import (
	"context"
	"errors"
	"testing"

	"ngac-platform/services/auth/internal/domain"
)

// A workspace whose creation reached the graph but not the table is listed by the next
// sign-in: the graph holds the person, so the missing row is written. A row that exists
// in any other state (removed, disabled) is a decision and is never overwritten.

func TestSignIn_OTP_AWorkspaceWhoseRowWriteFailedIsListedAtTheNextSignIn(t *testing.T) {
	svc, w, _ := otpService(t, nil)
	ctx := context.Background()
	email := uniqueEmail()

	w.insertErr = errors.New("database: down")
	sid, err := svc.RequestOTP(ctx, email, "email")
	if err != nil {
		t.Fatal(err)
	}
	first, err := svc.VerifyOTP(ctx, sid, "999999")
	if err != nil {
		t.Fatalf("sign-up must not fail on a row: %v", err)
	}
	if first.TenantID != "" {
		t.Fatalf("tenant = %q while no row could be written", first.TenantID)
	}

	w.insertErr = nil
	sid, err = svc.RequestOTP(ctx, email, "email")
	if err != nil {
		t.Fatal(err)
	}
	next, err := svc.VerifyOTP(ctx, sid, "999999")
	if err != nil {
		t.Fatal(err)
	}
	if next.TenantID == "" || tenantOf(t, next.Token) != next.TenantID {
		t.Fatalf("tenant = %q after the database came back, want the workspace the graph holds them in", next.TenantID)
	}
	row := w.membership(next.TenantID, next.UserID)
	if row == nil || row.Status != "active" || row.Role != "owner" {
		t.Errorf("row = %+v, want an active owner written by the sign-in", row)
	}
}

func TestSignIn_Google_AGraphMemberWithARemovedOrDisabledRowIsNotListedAgain(t *testing.T) {
	for _, status := range []string{"removed", "disabled"} {
		t.Run(status, func(t *testing.T) {
			w := newFakeWorld()
			w.rowsLeaveGraphAlone = true
			svc := w.service(t)
			ctx := context.Background()
			u := w.addUser(uniqueEmail())
			held, kept := w.addTenant("Held", ""), w.addTenant("Kept", "")
			if err := w.InsertTenantUser(ctx, held, u.ID, "member", status, u.NGACNodeID); err != nil {
				t.Fatal(err)
			}
			if err := w.InsertTenantUser(ctx, kept, u.ID, "member", "active", u.NGACNodeID); err != nil {
				t.Fatal(err)
			}
			w.attach(u.NGACNodeID, held) // the graph still holds them, as after a failed removal
			w.attach(u.NGACNodeID, kept)
			if err := w.LinkIdentity(ctx, domain.ProviderGoogle, "sub-"+u.ID, u.ID, u.Email); err != nil {
				t.Fatal(err)
			}

			res, err := svc.SignInWithGoogle(ctx, googleIdentity("sub-"+u.ID, u.Email, ""))
			if err != nil {
				t.Fatal(err)
			}
			if m := w.membership(held, u.ID); m == nil || m.Status != status {
				t.Errorf("row = %+v, want it left as %s", m, status)
			}
			if got := tenantIDs(res.Tenants); len(got) != 1 || got[0] != kept {
				t.Errorf("tenants = %v, want only %q", got, kept)
			}
		})
	}
}

// Two joins racing: the one whose row failed must not take back an edge the other
// one's active row depends on.
func TestJoinTenant_ARowFailureDoesNotLeaveAnActiveRowWithoutItsEdge(t *testing.T) {
	w := newFakeWorld()
	w.rowsLeaveGraphAlone = true
	svc := w.service(t)
	ctx := context.Background()
	ws := w.addTenant("Acme", "")
	u := w.addUser("race1@example.test")
	w.insertErr = errors.New("database: down")
	// While this call takes its edge back, the other call's row lands.
	w.afterRemove = func() {
		w.mu.Lock()
		w.members[ws+"|"+u.ID] = storeRow(ws, u.ID, u.NGACNodeID)
		w.mu.Unlock()
	}

	if err := svc.JoinTenantForTest(ctx, ws, u.ID, u.NGACNodeID, "member"); err == nil {
		t.Error("the call whose row failed reported success")
	}
	if !w.assigned(u.NGACNodeID, membersOf(ws)) {
		t.Errorf("an active row exists without its Members edge; assignments: %v", w.assignments())
	}

	// Allow: with no row from anyone the edge stays taken back.
	w2 := newFakeWorld()
	w2.rowsLeaveGraphAlone = true
	svc2 := w2.service(t)
	ws2 := w2.addTenant("Acme", "")
	u2 := w2.addUser("race2@example.test")
	w2.insertErr = errors.New("database: down")
	_ = svc2.JoinTenantForTest(ctx, ws2, u2.ID, u2.NGACNodeID, "member")
	if w2.assigned(u2.NGACNodeID, membersOf(ws2)) {
		t.Error("the edge of a join that left no row was kept")
	}
}

// A cancelled or timed-out insert may have committed: its edge is not taken back.
func TestJoinTenant_ACancelledRowWriteDoesNotUndoTheEdge(t *testing.T) {
	for _, cause := range []error{context.Canceled, context.DeadlineExceeded} {
		w := newFakeWorld()
		w.rowsLeaveGraphAlone = true
		svc := w.service(t)
		ws := w.addTenant("Acme", "")
		u := w.addUser("cancel@example.test")
		w.insertErr = cause

		if err := svc.JoinTenantForTest(context.Background(), ws, u.ID, u.NGACNodeID, "member"); err == nil {
			t.Errorf("%v: reported success", cause)
		}
		if !w.assigned(u.NGACNodeID, membersOf(ws)) {
			t.Errorf("%v: the edge was taken back though the row may have been written", cause)
		}
	}
}
