package domain_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"google.golang.org/grpc"

	workspacepb "ngac-platform/proto/workspace"
	"ngac-platform/services/auth/internal/domain"
	"ngac-platform/services/auth/internal/store"
	"ngac-platform/testutil"
)

// verifiedUser is an account whose address has been proved, the only kind that
// may create a workspace.
func verifiedUser(t *testing.T, w *fakeWorld, email string) *store.User {
	t.Helper()
	u := w.addUser(email)
	if _, err := w.MarkEmailVerified(context.Background(), u.ID); err != nil {
		t.Fatal(err)
	}
	return u
}

func TestListMyWorkspaces_OnlyMyMembershipsWithRoleAndCount(t *testing.T) {
	w := newFakeWorld()
	svc := w.service(t)
	me := w.addUser("me@example.test")
	other := w.addUser("other@example.test")

	mine := w.addTenant("Khối Vận hành", "")
	theirs := w.addTenant("Nhóm khác", "")
	_ = w.InsertTenantUser(context.Background(), mine, me.ID, "owner", "active", me.NGACNodeID)
	_ = w.InsertTenantUser(context.Background(), mine, other.ID, "member", "active", other.NGACNodeID)
	_ = w.InsertTenantUser(context.Background(), theirs, other.ID, "owner", "active", other.NGACNodeID)

	got, err := svc.ListMyWorkspaces(context.Background(), me.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d workspaces, want only the one I belong to: %+v", len(got), got)
	}
	if got[0].ID != mine || got[0].Name != "Khối Vận hành" || got[0].Role != "owner" || got[0].MemberCount != 2 {
		t.Errorf("summary = %+v", got[0])
	}
}

func TestListMyWorkspaces_NoUserIsInvalid(t *testing.T) {
	if _, err := newFakeWorld().service(t).ListMyWorkspaces(context.Background(), ""); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
}

func TestCreateMyWorkspace_ProvisionsLikeSignup(t *testing.T) {
	w := newFakeWorld()
	svc := w.service(t)
	u := verifiedUser(t, w, "maker@example.test")

	ws, err := svc.CreateMyWorkspace(context.Background(), u.ID, u.NGACNodeID, "  Tổ Đối soát  ")
	if err != nil {
		t.Fatal(err)
	}
	if ws.Name != "Tổ Đối soát" || ws.Role != "owner" || ws.MemberCount != 1 || ws.ID == "" {
		t.Errorf("created = %+v", ws)
	}
	if m := w.membership(ws.ID, u.ID); m == nil || m.Role != "owner" {
		t.Errorf("creator must be listed as owner in tenant_users, got %+v", m)
	}
	if len(w.channels) != 1 || w.channels[0] != ws.ID {
		t.Errorf("#general must be provisioned, channels = %v", w.channels)
	}
	if c := w.callers["CreateWorkspace"]; c.UserID != u.ID || c.NGACNodeID != u.NGACNodeID {
		t.Errorf("the workspace is created on behalf of the caller, got %+v", c)
	}
}

func TestCreateMyWorkspace_RefusesBadNamesAndCreatesNothing(t *testing.T) {
	w := newFakeWorld()
	svc := w.service(t)
	u := verifiedUser(t, w, "bad@example.test")

	for name, in := range map[string]string{
		"empty":       "",
		"blank":       "   ",
		"tabs":        "\t\t",
		"81 runes":    strings.Repeat("a", 81),
		"newline":     "Tổ\nĐối soát",
		"NUL":         "Tổ\x00",
		"bidi":        "Tổ\u202eĐối soát", // right-to-left override: spoofs how a name reads
		"zero width":  "\u200b",
		"only spaces": "  ",
	} {
		before := w.workspaceCount()
		if _, err := svc.CreateMyWorkspace(context.Background(), u.ID, u.NGACNodeID, in); !errors.Is(err, domain.ErrInvalidInput) {
			t.Errorf("%s: err = %v, want ErrInvalidInput", name, err)
		}
		if w.workspaceCount() != before {
			t.Errorf("%s: a workspace was created for a refused name", name)
		}
	}
}

func TestCreateMyWorkspace_NeedsACaller(t *testing.T) {
	svc := newFakeWorld().service(t)
	if _, err := svc.CreateMyWorkspace(context.Background(), "", "", "Ok"); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("err = %v, want ErrInvalidInput", err)
	}
}

func TestCreateMyWorkspace_IsRateLimitedPerPerson(t *testing.T) {
	svc, w, rdb := otpService(t, nil)
	u := verifiedUser(t, w, uniqueEmail())
	other := verifiedUser(t, w, uniqueEmail())
	rdb.Del(context.Background(), "ws_create_rl:"+u.ID, "ws_create_rl:"+other.ID)
	t.Cleanup(func() { rdb.Del(context.Background(), "ws_create_rl:"+u.ID, "ws_create_rl:"+other.ID) })

	for i := 0; i < domain.WorkspaceCreateLimit; i++ {
		if _, err := svc.CreateMyWorkspace(context.Background(), u.ID, u.NGACNodeID, "Nhóm"); err != nil {
			t.Fatalf("creation %d: %v", i+1, err)
		}
	}
	before := w.workspaceCount()
	_, err := svc.CreateMyWorkspace(context.Background(), u.ID, u.NGACNodeID, "Một nhóm nữa")
	if !errors.Is(err, domain.ErrRateLimited) {
		t.Fatalf("err = %v, want ErrRateLimited", err)
	}
	if d, ok := domain.RetryAfter(err); !ok || d <= 0 {
		t.Errorf("a refusal says when to come back, got %v %v", d, ok)
	}
	if w.workspaceCount() != before {
		t.Error("a refused creation must not create a workspace")
	}
	if _, err := svc.CreateMyWorkspace(context.Background(), other.ID, other.NGACNodeID, "Nhóm của người khác"); err != nil {
		t.Errorf("another person's budget is separate: %v", err)
	}
}

// A workspace is where people are invited by address, so only an account whose
// address has been proved may make one.
func TestCreateMyWorkspace_NeedsAVerifiedAddress(t *testing.T) {
	w := newFakeWorld()
	svc := w.service(t)
	unverified := w.addUser("fixed-code@example.test")
	phoneOnly := w.addUser("")
	phoneOnly.Email = ""

	for name, u := range map[string]*store.User{"an unverified address": unverified, "no address at all": phoneOnly} {
		_, err := svc.CreateMyWorkspace(context.Background(), u.ID, u.NGACNodeID, "Tổ Đối soát")
		if !errors.Is(err, domain.ErrEmailUnverified) {
			t.Errorf("%s: err = %v, want ErrEmailUnverified", name, err)
		}
	}
	if w.workspaceCount() != 0 {
		t.Errorf("%d workspaces were created for accounts that may not", w.workspaceCount())
	}

	if _, err := w.MarkEmailVerified(context.Background(), unverified.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CreateMyWorkspace(context.Background(), unverified.ID, unverified.NGACNodeID, "Tổ Đối soát"); err != nil {
		t.Errorf("once the address is proved: %v", err)
	}
}

func TestCreateMyWorkspace_UnknownAccountIsNotFound(t *testing.T) {
	svc := newFakeWorld().service(t)
	if _, err := svc.CreateMyWorkspace(context.Background(), "ghost", "node", "Ok"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

// failingTenantInsert refuses the tenant_users row.
type failingTenantInsert struct{ *fakeWorld }

func (failingTenantInsert) InsertTenantUser(context.Context, string, string, string, string, string) error {
	return errors.New("db down")
}

// Anything that goes wrong after the workspace exists must take the workspace
// with it: removed through the workspace service, on behalf of the person, and
// the error returned. A half-built workspace is worse than none.
func TestCreateMyWorkspace_UndoesTheWorkspaceWhereverProvisioningFails(t *testing.T) {
	cases := []struct {
		name  string
		build func(w *fakeWorld) *domain.Service
	}{
		{"the tenant graph cannot be built", func(w *fakeWorld) *domain.Service {
			write := testutil.NewFakePolicyWrite()
			write.FailAt = 1 // the first attachment under the workspace
			return domain.NewService(w, nil, &fakePolicyRead{}, write, &fakeWorkspace{w: w}, &fakeMessaging{w: w})
		}},
		{"the membership row cannot be written", func(w *fakeWorld) *domain.Service {
			return domain.NewService(failingTenantInsert{w}, nil, &fakePolicyRead{}, &fakePolicyWrite{w: w}, &fakeWorkspace{w: w}, &fakeMessaging{w: w})
		}},
		{"the owner cannot be attached to the tenant", func(w *fakeWorld) *domain.Service {
			write := testutil.NewFakePolicyWrite()
			write.FailAt = 5 // after the four attachments of the tenant graph
			return domain.NewService(w, nil, &fakePolicyRead{}, write, &fakeWorkspace{w: w}, &fakeMessaging{w: w})
		}},
		{"#general cannot be created", func(w *fakeWorld) *domain.Service {
			w.channelErr = errors.New("messaging down")
			return domain.NewService(w, nil, &fakePolicyRead{}, &fakePolicyWrite{w: w}, &fakeWorkspace{w: w}, &fakeMessaging{w: w})
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			auth_ := newFakeWorld()
			u := verifiedUser(t, auth_, "undo@example.test")
			svc := tc.build(auth_)

			ws, err := svc.CreateMyWorkspace(context.Background(), u.ID, u.NGACNodeID, "Tổ Đối soát")

			if err == nil || ws != nil {
				t.Fatalf("got %+v, %v: the failure must be returned", ws, err)
			}
			if errors.Is(err, domain.ErrInvalidInput) || errors.Is(err, domain.ErrAccessDenied) {
				t.Errorf("a server fault must not read as the caller's mistake: %v", err)
			}
			if len(auth_.deleted) != 1 {
				t.Fatalf("DeleteWorkspace calls = %v, want exactly one", auth_.deleted)
			}
			if auth_.workspaceCount() != 0 {
				t.Errorf("%d workspaces remain", auth_.workspaceCount())
			}
			if c := auth_.callers["DeleteWorkspace"]; c.UserID != u.ID || c.NGACNodeID != u.NGACNodeID {
				t.Errorf("the undo is asked on behalf of the creator, got %+v", c)
			}
			if auth_.membership(auth_.deleted[0], u.ID) != nil {
				t.Error("the membership row was left behind")
			}
		})
	}
}

func TestCreateMyWorkspace_ReportsTheFailureEvenWhenTheUndoFails(t *testing.T) {
	w := newFakeWorld()
	w.channelErr = errors.New("messaging down")
	w.deleteErr = errors.New("workspace service down")
	svc := w.service(t)
	u := verifiedUser(t, w, "undo2@example.test")

	_, err := svc.CreateMyWorkspace(context.Background(), u.ID, u.NGACNodeID, "Tổ Đối soát")

	if err == nil || !strings.Contains(err.Error(), "messaging down") {
		t.Fatalf("err = %v, want the original failure", err)
	}
}

func TestCreateMyWorkspace_NothingToUndoWhenTheWorkspaceItselfFails(t *testing.T) {
	w := newFakeWorld()
	u := verifiedUser(t, w, "undo3@example.test")
	svc := domain.NewService(w, nil, &fakePolicyRead{}, &fakePolicyWrite{w: w}, failingCreateWorkspace{&fakeWorkspace{w: w}}, &fakeMessaging{w: w})

	if _, err := svc.CreateMyWorkspace(context.Background(), u.ID, u.NGACNodeID, "Tổ"); err == nil {
		t.Fatal("expected an error")
	}
	if len(w.deleted) != 0 {
		t.Errorf("DeleteWorkspace was called for a workspace that was never made: %v", w.deleted)
	}
}

func TestCreateMyWorkspace_SignInProvisioningStaysLenient(t *testing.T) {
	// A person signing in must still get in when #general cannot be made: that
	// path logs and carries on, and never deletes the workspace.
	w := newFakeWorld()
	w.channelErr = errors.New("messaging down")
	svc := w.service(t)
	res, err := svc.SignInWithGoogle(context.Background(), googleIdentity("sub-lenient", "lenient@example.test", ""))
	if err != nil {
		t.Fatalf("sign in: %v", err)
	}
	if len(w.deleted) != 0 || w.membership(res.DefaultTenantID, res.UserID) == nil {
		t.Errorf("sign-in must keep the new person's workspace (deleted %v)", w.deleted)
	}
}

type failingCreateWorkspace struct{ *fakeWorkspace }

func (failingCreateWorkspace) CreateWorkspace(context.Context, *workspacepb.CreateWorkspaceRequest, ...grpc.CallOption) (*workspacepb.Workspace, error) {
	return nil, errors.New("workspace service down")
}
