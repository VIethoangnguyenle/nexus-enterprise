package domain_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"ngac-platform/pkg/grpcauth"
	"ngac-platform/services/auth/internal/domain"
)

// #general is made on the auth service's own identity even when the request that
// led to it carries a signed-in person: messaging refuses a person's identity on
// that method, and the person is named in the request instead.
func TestCreateMyWorkspace_ProvisionsGeneralAsTheServiceNamingTheCreator(t *testing.T) {
	w := newFakeWorld()
	svc := w.service(t)
	u := verifiedUser(t, w, "service-id@example.test")
	ctx := grpcauth.WithCaller(context.Background(), grpcauth.Caller{UserID: u.ID, NGACNodeID: u.NGACNodeID})

	ws, err := svc.CreateMyWorkspace(ctx, u.ID, u.NGACNodeID, "Tổ Đối soát", domain.WorkspaceOrganization)
	if err != nil {
		t.Fatal(err)
	}

	w.mu.Lock()
	defer w.mu.Unlock()
	if c := w.callers["ProvisionWorkspaceChannel"]; c != (grpcauth.Caller{}) {
		t.Errorf("ProvisionWorkspaceChannel carried %+v, want no caller (the service identity)", c)
	}
	if len(w.provisioned) != 1 {
		t.Fatalf("provisioned = %+v, want one #general", w.provisioned)
	}
	got := w.provisioned[0]
	if got.WorkspaceId != ws.ID || got.Name != "general" || got.CreatorUserId != u.ID || got.CreatorNgacNodeId != u.NGACNodeID {
		t.Errorf("request = %+v", got)
	}
	// The workspace itself is still created on the person's behalf.
	if c := w.callers["CreateWorkspace"]; c.UserID != u.ID {
		t.Errorf("CreateWorkspace caller = %+v, want the person", c)
	}
}

// Joining a company workspace by its Google domain puts the new member into the
// workspace's #general, as the service; signing in without joining anything, or
// again after joining, puts nobody anywhere.
func TestSignInWithGoogle_DomainJoinPutsTheMemberInGeneralOnce(t *testing.T) {
	w := newFakeWorld()
	svc := w.service(t)
	acme := w.addTenant("Acme", "acme.com")

	// A consumer account joins no workspace and so no #general.
	if _, err := svc.SignInWithGoogle(context.Background(), googleIdentity("sub-consumer", "bob@gmail.com", "")); err != nil {
		t.Fatal(err)
	}
	w.mu.Lock()
	for _, j := range w.joins {
		if j[0] == acme {
			t.Errorf("a consumer account was put into Acme's channel: %v", w.joins)
		}
	}
	w.mu.Unlock()

	res, err := svc.SignInWithGoogle(context.Background(), googleIdentity("sub-acme", "alice@acme.com", "acme.com"))
	if err != nil {
		t.Fatal(err)
	}
	w.mu.Lock()
	var joined int
	for _, j := range w.joins {
		if j == [2]string{acme, res.NGACNodeID} {
			joined++
		}
	}
	caller := w.callers["JoinWorkspaceChannel"]
	w.mu.Unlock()
	if joined != 1 {
		t.Fatalf("joins = %v, want the new member put into Acme's channel once", w.joins)
	}
	if caller != (grpcauth.Caller{}) {
		t.Errorf("JoinWorkspaceChannel carried %+v, want the service identity", caller)
	}

	// Signing in again changes nothing: the membership exists, so there is no join.
	if _, err := svc.SignInWithGoogle(context.Background(), googleIdentity("sub-acme", "alice@acme.com", "acme.com")); err != nil {
		t.Fatal(err)
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	joined = 0
	for _, j := range w.joins {
		if j == [2]string{acme, res.NGACNodeID} {
			joined++
		}
	}
	if joined != 1 {
		t.Errorf("a second sign-in joined again: %v", w.joins)
	}
}

// The channel is a convenience of joining, not a condition of it: if messaging
// cannot be reached, the person is still a member and signs in.
func TestSignInWithGoogle_DomainJoinSurvivesAFailingChannelJoin(t *testing.T) {
	w := newFakeWorld()
	svc := w.service(t)
	acme := w.addTenant("Acme", "acme.com")
	w.joinErr = context.DeadlineExceeded

	res, err := svc.SignInWithGoogle(context.Background(), googleIdentity("sub-acme", "carol@acme.com", "acme.com"))
	if err != nil {
		t.Fatalf("sign-in must not fail when #general cannot be joined: %v", err)
	}
	if m := w.membership(acme, res.UserID); m == nil || m.Status != "active" {
		t.Errorf("the person must still be an active member, got %+v", m)
	}
}

// A messaging service that predates ProvisionWorkspaceChannel (the window of a rolling
// deploy) answers "unimplemented": the workspace still gets its #general, the old way.
// Any other failure is not that, and is reported.
func TestCreateMyWorkspace_FallsBackToTheOldChannelCallOnlyWhenTheNewOneIsUnknown(t *testing.T) {
	w := newFakeWorld()
	svc := w.service(t)
	u := verifiedUser(t, w, "old-messaging@example.test")

	w.provisionErr = errors.New("messaging is down")
	_, err := svc.CreateMyWorkspace(context.Background(), u.ID, u.NGACNodeID, "Tổ A", domain.WorkspaceOrganization)
	if err == nil {
		t.Fatal("a failure that is not 'unimplemented' must still fail the strict path")
	}
	w.mu.Lock()
	if len(w.legacyCreates) != 0 {
		t.Errorf("fell back on an ordinary failure: %+v", w.legacyCreates)
	}
	w.mu.Unlock()

	w2 := newFakeWorld()
	svc2 := w2.service(t)
	u2 := verifiedUser(t, w2, "old-messaging2@example.test")
	w2.provisionErr = status.Error(codes.Unimplemented, "unknown method ProvisionWorkspaceChannel")
	ws, err := svc2.CreateMyWorkspace(context.Background(), u2.ID, u2.NGACNodeID, "Tổ B", domain.WorkspaceOrganization)
	if err != nil {
		t.Fatalf("the old call must have made the channel: %v", err)
	}
	w2.mu.Lock()
	defer w2.mu.Unlock()
	if len(w2.legacyCreates) != 1 || w2.legacyCreates[0].WorkspaceId != ws.ID || w2.legacyCreates[0].Name != "general" || w2.legacyCreates[0].ChannelType != "workspace" {
		t.Fatalf("legacy creates = %+v, want one workspace-typed general", w2.legacyCreates)
	}
	if c := w2.callers["CreateChannel"]; c.UserID != u2.ID || c.NGACNodeID != u2.NGACNodeID {
		t.Errorf("the old call is made as the owner, got %+v", c)
	}
}

// An old messaging service has no JoinWorkspaceChannel either; the sign-in goes on, and
// a join that was refused for any other reason does not stop it.
func TestSignInWithGoogle_DomainJoinGoesOnWhenTheChannelJoinIsUnimplemented(t *testing.T) {
	w := newFakeWorld()
	svc := w.service(t)
	acme := w.addTenant("Acme", "acme.com")
	w.joinUnimplemented = true

	res, err := svc.SignInWithGoogle(context.Background(), googleIdentity("sub-old", "dan@acme.com", "acme.com"))
	if err != nil {
		t.Fatalf("sign-in must go on: %v", err)
	}
	if m := w.membership(acme, res.UserID); m == nil || m.Status != "active" {
		t.Errorf("the person must still be an active member, got %+v", m)
	}
}

// The two calls are bounded: a messaging service that never answers must not hold a
// sign-in or a workspace creation for as long as the request lives.
func TestChannelCallsAreBoundedInTime(t *testing.T) {
	w := newFakeWorld()
	svc := w.service(t)
	acme := w.addTenant("Acme", "acme.com")
	u := verifiedUser(t, w, "bounded@example.test")

	if _, err := svc.SignInWithGoogle(context.Background(), googleIdentity("sub-b", "eve@acme.com", "acme.com")); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CreateMyWorkspace(context.Background(), u.ID, u.NGACNodeID, "Tổ C", domain.WorkspaceOrganization); err != nil {
		t.Fatal(err)
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, rpc := range []string{"JoinWorkspaceChannel", "ProvisionWorkspaceChannel"} {
		if left := w.rpcDeadlines[rpc]; left <= 0 || left > 10*time.Second {
			t.Errorf("%s had %v left on its context, want a deadline within 10s", rpc, left)
		}
	}
	// The join names the person by user id too: the policy answers on their behalf.
	if len(w.joinUsers) != 1 || w.joinUsers[0] == "" {
		t.Errorf("join user ids = %v, want the new member's id", w.joinUsers)
	}
	_ = acme
}
