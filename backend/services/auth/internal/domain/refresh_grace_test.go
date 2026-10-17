package domain_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"ngac-platform/services/auth/internal/domain"
)

// Two tabs of one browser share a cookie, so both can present the same refresh token at
// once (the hub closes every tab's socket with the same code and each asks for a new
// token). Treating the second as a replay would revoke the whole session. A token spent a
// moment ago therefore answers with the successor it already produced; one spent long ago
// is a replay and still ends the family.

func graceSession(t *testing.T, grace time.Duration) (*domain.Service, *fakeWorld, string, string, string) {
	t.Helper()
	svc, w, _ := otpService(t, nil)
	svc.RefreshStoreForTest().SetGraceForTest(grace)
	ws := w.addTenant("Acme", "")
	u := w.addUser(uniqueEmail())
	seedMember(t, w, ws, u.ID, u.NGACNodeID)
	token, err := svc.AttachRefreshToken(context.Background(), domain.RefreshIdentity{
		UserID: u.ID, Username: u.Username, NGACNodeID: u.NGACNodeID, TenantID: ws, SessionID: "s-" + uuid.NewString(),
	})
	if err != nil {
		t.Fatal(err)
	}
	return svc, w, ws, u.ID, token
}

func TestRefreshSession_TwoTabsAtOnceBothSucceedWithTheSameSuccessor(t *testing.T) {
	svc, _, ws, _, token := graceSession(t, 10*time.Second)

	var wg sync.WaitGroup
	results := make([]domain.RefreshOutcome, 4)
	errs := make([]error, 4)
	for i := range results {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i], errs[i] = svc.RefreshSessionFull(context.Background(), token)
		}()
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
		if results[i].TenantID != ws || results[i].Refresh == "" {
			t.Errorf("call %d: %+v", i, results[i])
		}
		if results[i].Refresh != results[0].Refresh {
			t.Errorf("call %d got a different successor: the family now has two live tokens", i)
		}
	}
	// The family is alive: the successor refreshes.
	if _, _, _, err := svc.RefreshSession(context.Background(), results[0].Refresh); err != nil {
		t.Fatalf("the successor must work: %v", err)
	}
}

// The paired deny: a replay once the window has passed is theft, and ends the family.
func TestRefreshSession_AReplayAfterTheGraceWindowRevokesTheFamily(t *testing.T) {
	svc, _, _, _, token := graceSession(t, 150*time.Millisecond)
	ctx := context.Background()
	_, next, _, err := svc.RefreshSession(ctx, token)
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(300 * time.Millisecond)
	if _, _, _, err := svc.RefreshSession(ctx, token); !errors.Is(err, domain.ErrRefreshRejected) {
		t.Fatalf("a late replay: err = %v, want ErrRefreshRejected", err)
	}
	if _, _, _, err := svc.RefreshSession(ctx, next); !errors.Is(err, domain.ErrRefreshRejected) {
		t.Fatalf("the family must be revoked after a late replay, the successor still works (err = %v)", err)
	}
}

// A spent token is not a reason to ask the policy anything: it is a replay or a duplicate,
// decided by the refresh store alone. With the policy down it must still be rejected, not 503.
func TestRefreshSession_ASpentTokenIsJudgedWithoutAskingThePolicy(t *testing.T) {
	svc, w, _, _, token := graceSession(t, 100*time.Millisecond)
	ctx := context.Background()
	if _, _, _, err := svc.RefreshSession(ctx, token); err != nil {
		t.Fatal(err)
	}
	time.Sleep(250 * time.Millisecond)

	w.policyDown = errors.New("policy: connection refused")
	_, _, _, err := svc.RefreshSession(ctx, token)

	if !errors.Is(err, domain.ErrRefreshRejected) {
		t.Fatalf("err = %v, want ErrRefreshRejected (a replay), not an outage", err)
	}
}

// The explicit flag is set only when the refresh moved the person because they no longer
// belong: an ordinary refresh, and its duplicate, say nothing of the kind.
func TestRefreshSession_MovedIsSetOnlyWhenMembershipMovedThem(t *testing.T) {
	svc, w, ws, userID, token := graceSession(t, 10*time.Second)
	ctx := context.Background()

	out, err := svc.RefreshSessionFull(ctx, token)
	if err != nil || out.Moved || out.TenantID != ws {
		t.Fatalf("an ordinary refresh: %+v, %v", out, err)
	}

	// Removed: the row says so and the graph no longer holds them.
	u, _ := w.GetUserByID(ctx, userID)
	w.mu.Lock()
	w.members[ws+"|"+userID].Status = "removed"
	w.mu.Unlock()
	w.detach(u.NGACNodeID, ws)

	var wg sync.WaitGroup
	moved := make([]domain.RefreshOutcome, 2)
	for i := range moved {
		wg.Add(1)
		go func() {
			defer wg.Done()
			moved[i], _ = svc.RefreshSessionFull(ctx, out.Refresh)
		}()
	}
	wg.Wait()
	for i, m := range moved {
		if !m.Moved || m.TenantID != "" {
			t.Errorf("call %d: %+v, want moved to no tenant (both duplicates agree)", i, m)
		}
	}
}
