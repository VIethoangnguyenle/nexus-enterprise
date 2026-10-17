package domain_test

import (
	"context"
	"sync"
	"testing"

	"ngac-platform/pkg/realtime"
	"ngac-platform/services/auth/internal/domain"
)

type sink struct {
	mu  sync.Mutex
	got []realtime.Event
}

func (s *sink) Emit(e realtime.Event) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.got = append(s.got, e)
}

func (s *sink) events() []realtime.Event {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]realtime.Event(nil), s.got...)
}

func nameOnly(name string) domain.ProfileUpdateInput {
	return domain.ProfileUpdateInput{DisplayName: &name}
}

// A profile change is announced once to every workspace the person belongs to,
// by ids only, and never to a workspace they do not.
func TestProfileChange_IsAnnouncedToEveryWorkspaceOfThePerson(t *testing.T) {
	w := newFakeWorld()
	out := &sink{}
	svc := w.service(t).WithEmitter(out)
	ctx := context.Background()
	u := w.addUser("rt@example.test")
	other := w.addUser("rt2@example.test")
	a, b, elsewhere := w.addTenant("A", ""), w.addTenant("B", ""), w.addTenant("C", "")
	_ = w.InsertTenantUser(ctx, a, u.ID, "member", "active", "node-a")
	_ = w.InsertTenantUser(ctx, b, u.ID, "member", "active", "node-b")
	_ = w.InsertTenantUser(ctx, elsewhere, other.ID, "member", "active", "node-c")

	if err := svc.UpdateProfile(ctx, u.ID, nameOnly("Tên Mới")); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SetAvatar(ctx, u.ID, encodePNG(t, quadrants(64, 64))); err != nil {
		t.Fatal(err)
	}
	if err := svc.ClearAvatar(ctx, u.ID); err != nil {
		t.Fatal(err)
	}

	got := out.events()
	if len(got) != 6 {
		t.Fatalf("want 3 changes x 2 workspaces = 6 events, got %d: %+v", len(got), got)
	}
	perTenant := map[string]int{}
	for _, e := range got {
		perTenant[e.TenantID]++
		if e.Domain != realtime.DomainWorkspace || e.Kind != realtime.KindProfileUpdated {
			t.Errorf("event = %s/%s, want workspace/profile_updated", e.Domain, e.Kind)
		}
		if e.WorkspaceID != e.TenantID || e.ActorUserID != u.ID {
			t.Errorf("boundary or actor wrong: %+v", e)
		}
		wantNode := map[string]string{a: "node-a", b: "node-b"}[e.TenantID]
		if len(e.IDs) != 1 || e.IDs[0] != wantNode {
			t.Errorf("ids = %v, want the person's node in that workspace (%s)", e.IDs, wantNode)
		}
		if err := e.Validate(); err != nil {
			t.Errorf("event would be dropped by the producer: %v", err)
		}
	}
	if perTenant[a] != 3 || perTenant[b] != 3 || perTenant[elsewhere] != 0 {
		t.Errorf("per tenant = %v; the person's workspaces get all, another tenant none", perTenant)
	}
}

func TestProfileChange_RefusedOrWithoutWorkspacesAnnouncesNothing(t *testing.T) {
	w := newFakeWorld()
	out := &sink{}
	svc := w.service(t).WithEmitter(out)
	ctx := context.Background()
	u := w.addUser("rt3@example.test")
	_ = w.InsertTenantUser(ctx, w.addTenant("A", ""), u.ID, "member", "active", "node-a")
	lone := w.addUser("rt4@example.test") // belongs to no workspace

	if err := svc.UpdateProfile(ctx, u.ID, nameOnly("   ")); err == nil {
		t.Fatal("a blank name must be refused")
	}
	if _, err := svc.SetAvatar(ctx, u.ID, []byte("not a picture")); err == nil {
		t.Fatal("a non-picture must be refused")
	}
	if err := svc.UpdateProfile(ctx, "no-such-user", nameOnly("Ai Đó")); err == nil {
		t.Fatal("an unknown user must be refused")
	}
	if err := svc.UpdateProfile(ctx, lone.ID, nameOnly("Một Mình")); err != nil {
		t.Fatal(err)
	}
	if got := out.events(); len(got) != 0 {
		t.Fatalf("nothing committed for anyone's workspace, got %+v", got)
	}
}

func TestProfileChange_WithoutAnEmitterStillWorks(t *testing.T) {
	w := newFakeWorld()
	svc := w.service(t)
	u := w.addUser("rt5@example.test")
	if err := svc.UpdateProfile(context.Background(), u.ID, nameOnly("Không Phát")); err != nil {
		t.Fatal(err)
	}
}
