package domain_test

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	"google.golang.org/grpc"

	"ngac-platform/pkg/grpcauth"
	messagingpb "ngac-platform/proto/messaging"
	policypb "ngac-platform/proto/policy"
	workspacepb "ngac-platform/proto/workspace"
	"ngac-platform/services/auth/internal/auth"
	"ngac-platform/services/auth/internal/domain"
	"ngac-platform/services/auth/internal/store"
)

// fakeWorld is an in-memory stand-in for Postgres plus the workspace,
// messaging and policy services, so the sign-in rules can be exercised
// without any of them running.
// recordCaller notes who a downstream RPC would be attributed to. Callers hold mu.
func (w *fakeWorld) recordCaller(rpc string, ctx context.Context) {
	if w.callers == nil {
		w.callers = map[string]grpcauth.Caller{}
	}
	w.callers[rpc] = grpcauth.CallerFrom(ctx)
}

type fakeWorld struct {
	mu         sync.Mutex
	users      map[string]*store.User // by id
	identities map[string]string      // provider|subject -> user id
	workspaces map[string]*store.Tenant
	members    map[string]*store.TenantMembership // tenant|user -> membership
	channels   []string                           // workspace ids that got #general
	callers    map[string]grpcauth.Caller         // downstream RPC -> caller on its context
	seq        int
	verified   map[string]bool // user id -> email proven
	deleted    []string        // workspaces removed through DeleteWorkspace
	deleteErr  error           // DeleteWorkspace fails with this
	channelErr error           // CreateChannel fails with this
}

func newFakeWorld() *fakeWorld {
	return &fakeWorld{
		users:      map[string]*store.User{},
		identities: map[string]string{},
		workspaces: map[string]*store.Tenant{},
		members:    map[string]*store.TenantMembership{},
		verified:   map[string]bool{},
	}
}

func (w *fakeWorld) next(prefix string) string {
	w.seq++
	return fmt.Sprintf("%s-%d", prefix, w.seq)
}

// service builds a domain.Service wired entirely to this fake world.
func (w *fakeWorld) service(t *testing.T) *domain.Service {
	t.Helper()
	auth.SetJWTSecret("test-secret-key-for-testing-only")
	return domain.NewService(w, nil, &fakePolicyRead{}, &fakePolicyWrite{w: w}, &fakeWorkspace{w: w}, &fakeMessaging{w: w})
}

// addTenant seeds a tenant (workspace) with an optional domain.
func (w *fakeWorld) addTenant(name, domainName string) string {
	w.mu.Lock()
	defer w.mu.Unlock()
	id := w.next("ws")
	w.workspaces[id] = &store.Tenant{ID: id, Name: name, Domain: domainName}
	return id
}

// addUser seeds an existing account.
func (w *fakeWorld) addUser(email string) *store.User {
	w.mu.Lock()
	defer w.mu.Unlock()
	id := w.next("user")
	u := &store.User{ID: id, Username: "existing." + id, NGACNodeID: "ngac-" + id, Email: email, DisplayName: "Existing"}
	w.users[id] = u
	return u
}

func (w *fakeWorld) membership(tenantID, userID string) *store.TenantMembership {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.members[tenantID+"|"+userID]
}

func (w *fakeWorld) userCount() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return len(w.users)
}

func (w *fakeWorld) workspaceCount() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return len(w.workspaces)
}

// --- domain.AuthStore ---

func (w *fakeWorld) CreateUser(_ context.Context, id, username, password, ngacNodeID, email, unionID, displayName, phone string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, u := range w.users {
		if email != "" && strings.EqualFold(u.Email, email) {
			return fmt.Errorf("duplicate email")
		}
		if u.Username == username {
			return fmt.Errorf("duplicate username")
		}
	}
	w.users[id] = &store.User{ID: id, Username: username, Password: password, NGACNodeID: ngacNodeID,
		Email: email, UnionID: unionID, DisplayName: displayName, Phone: phone}
	return nil
}

func (w *fakeWorld) CreateUserWithVerifiedEmail(ctx context.Context, id, username, password, ngacNodeID, email, unionID, displayName, phone string) error {
	if err := w.CreateUser(ctx, id, username, password, ngacNodeID, email, unionID, displayName, phone); err != nil {
		return err
	}
	if email != "" {
		w.mu.Lock()
		w.verified[id] = true
		w.mu.Unlock()
	}
	return nil
}

func (w *fakeWorld) find(match func(*store.User) bool) *store.User {
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, u := range w.users {
		if match(u) {
			cp := *u
			cp.EmailVerified = w.verified[u.ID]
			return &cp
		}
	}
	return nil
}

func (w *fakeWorld) GetUserByUsername(_ context.Context, username string) (*store.User, error) {
	return w.find(func(u *store.User) bool { return u.Username == username }), nil
}
func (w *fakeWorld) GetUserByEmail(_ context.Context, email string) (*store.User, error) {
	return w.find(func(u *store.User) bool { return email != "" && strings.EqualFold(u.Email, email) }), nil
}
func (w *fakeWorld) GetUserByPhone(_ context.Context, phone string) (*store.User, error) {
	return w.find(func(u *store.User) bool { return phone != "" && u.Phone == phone }), nil
}
func (w *fakeWorld) GetUserByID(_ context.Context, id string) (*store.User, error) {
	return w.find(func(u *store.User) bool { return u.ID == id }), nil
}
func (w *fakeWorld) GetUserByNGACNodeID(_ context.Context, id string) (*store.User, error) {
	return w.find(func(u *store.User) bool { return u.NGACNodeID == id }), nil
}

func (w *fakeWorld) InsertTenantUser(_ context.Context, tenantID, userID, role, status, ngacNodeID string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	key := tenantID + "|" + userID
	if _, ok := w.members[key]; ok {
		return nil
	}
	name := ""
	if ws := w.workspaces[tenantID]; ws != nil {
		name = ws.Name
	}
	w.members[key] = &store.TenantMembership{TenantID: tenantID, TenantName: name, UserID: userID,
		Role: role, Status: status, OpenID: w.next("open"), NGACNodeID: ngacNodeID}
	return nil
}

func (w *fakeWorld) ListTenantsByUser(_ context.Context, userID string) ([]store.TenantMembership, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	var out []store.TenantMembership
	for _, m := range w.members {
		if m.UserID == userID && m.Status == "active" {
			out = append(out, *m)
		}
	}
	return out, nil
}

func (w *fakeWorld) GetTenantUser(_ context.Context, tenantID, userID string) (*store.TenantMembership, error) {
	if m := w.membership(tenantID, userID); m != nil {
		cp := *m
		return &cp, nil
	}
	return nil, nil
}

func (w *fakeWorld) FindTenantByDomain(_ context.Context, d string) (*store.Tenant, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, ws := range w.workspaces {
		if ws.Domain != "" && strings.EqualFold(ws.Domain, d) {
			cp := *ws
			return &cp, nil
		}
	}
	return nil, nil
}

func (w *fakeWorld) UpdateProfile(_ context.Context, userID string, ch store.ProfileChanges) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	u := w.users[userID]
	if u == nil {
		return store.ErrNoSuchUser
	}
	if ch.DisplayName != nil {
		u.DisplayName = *ch.DisplayName
	}
	if ch.Title != nil {
		u.Title = *ch.Title
	}
	if ch.Location != nil {
		u.Location = *ch.Location
	}
	u.ProfileCompleted = true
	return nil
}

func (w *fakeWorld) ListWorkspaceSummaries(_ context.Context, userID string) ([]store.WorkspaceSummary, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	var out []store.WorkspaceSummary
	for _, m := range w.members {
		if m.UserID != userID || m.Status != "active" {
			continue
		}
		count := 0
		for _, o := range w.members {
			if o.TenantID == m.TenantID && o.Status == "active" {
				count++
			}
		}
		out = append(out, store.WorkspaceSummary{ID: m.TenantID, Name: m.TenantName, Role: m.Role, MemberCount: count})
	}
	return out, nil
}
func (w *fakeWorld) ListContactsByWorkspace(context.Context, string, store.ContactFilter) ([]store.User, int, *store.ContactCursor, error) {
	return nil, 0, nil, nil
}

func (w *fakeWorld) FindUserByIdentity(_ context.Context, provider, subject string) (*store.User, error) {
	w.mu.Lock()
	id, ok := w.identities[provider+"|"+subject]
	w.mu.Unlock()
	if !ok {
		return nil, nil
	}
	return w.find(func(u *store.User) bool { return u.ID == id }), nil
}

func (w *fakeWorld) GetIdentitySubject(_ context.Context, provider, userID string) (string, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	for k, uid := range w.identities {
		if uid == userID && strings.HasPrefix(k, provider+"|") {
			return strings.TrimPrefix(k, provider+"|"), nil
		}
	}
	return "", nil
}

func (w *fakeWorld) LinkIdentity(_ context.Context, provider, subject, userID, _ string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	key := provider + "|" + subject
	if _, ok := w.identities[key]; !ok {
		w.identities[key] = userID
	}
	return nil
}

func (w *fakeWorld) ClaimTenantDomain(_ context.Context, tenantID, d string) (bool, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, ws := range w.workspaces {
		if strings.EqualFold(ws.Domain, d) {
			return false, nil
		}
	}
	ws := w.workspaces[tenantID]
	if ws == nil || ws.Domain != "" {
		return false, nil
	}
	ws.Domain = d
	return true, nil
}

func (w *fakeWorld) ClearPassword(_ context.Context, userID string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if u := w.users[userID]; u != nil {
		u.Password = ""
	}
	return nil
}

// --- gRPC fakes ---

type fakeWorkspace struct {
	workspacepb.WorkspaceServiceClient
	w *fakeWorld
}

func (f *fakeWorkspace) CreateWorkspace(ctx context.Context, req *workspacepb.CreateWorkspaceRequest, _ ...grpc.CallOption) (*workspacepb.Workspace, error) {
	f.w.mu.Lock()
	defer f.w.mu.Unlock()
	f.w.recordCaller("CreateWorkspace", ctx)
	id := f.w.next("ws")
	f.w.workspaces[id] = &store.Tenant{ID: id, Name: req.Name}
	return &workspacepb.Workspace{Id: id, Name: req.Name, PcNodeId: "pc-" + id,
		OwnersUaId: "owners-" + id, MembersUaId: "members-" + id}, nil
}

// DeleteWorkspace is the compensation: it removes the workspace and every
// membership in it from the fake world and records who asked.
func (f *fakeWorkspace) DeleteWorkspace(ctx context.Context, req *workspacepb.DeleteWorkspaceRequest, _ ...grpc.CallOption) (*workspacepb.Empty, error) {
	f.w.mu.Lock()
	defer f.w.mu.Unlock()
	f.w.recordCaller("DeleteWorkspace", ctx)
	f.w.deleted = append(f.w.deleted, req.WorkspaceId)
	if f.w.deleteErr != nil {
		return nil, f.w.deleteErr
	}
	delete(f.w.workspaces, req.WorkspaceId)
	for k, m := range f.w.members {
		if m.TenantID == req.WorkspaceId {
			delete(f.w.members, k)
		}
	}
	return &workspacepb.Empty{}, nil
}

type fakeMessaging struct {
	messagingpb.MessagingServiceClient
	w *fakeWorld
}

func (f *fakeMessaging) CreateChannel(ctx context.Context, req *messagingpb.CreateChannelRequest, _ ...grpc.CallOption) (*messagingpb.Channel, error) {
	f.w.mu.Lock()
	defer f.w.mu.Unlock()
	f.w.recordCaller("CreateChannel", ctx)
	if f.w.channelErr != nil {
		return nil, f.w.channelErr
	}
	f.w.channels = append(f.w.channels, req.WorkspaceId)
	return &messagingpb.Channel{Id: "ch-" + req.WorkspaceId, Name: req.Name}, nil
}

type fakePolicyRead struct {
	policypb.PolicyReadServiceClient
}

func (f *fakePolicyRead) FindNodeByName(_ context.Context, req *policypb.FindNodeByNameRequest, _ ...grpc.CallOption) (*policypb.NGACNode, error) {
	return &policypb.NGACNode{Id: "node-" + req.Name, Name: req.Name, NodeType: req.NodeType}, nil
}

type fakePolicyWrite struct {
	policypb.PolicyWriteServiceClient
	w *fakeWorld
}

func (f *fakePolicyWrite) CreateNode(_ context.Context, req *policypb.CreateNodeRequest, _ ...grpc.CallOption) (*policypb.NGACNode, error) {
	f.w.mu.Lock()
	defer f.w.mu.Unlock()
	return &policypb.NGACNode{Id: f.w.next("node"), Name: req.Name, NodeType: req.NodeType}, nil
}

func (f *fakePolicyWrite) CreateAssignment(_ context.Context, req *policypb.CreateAssignmentRequest, _ ...grpc.CallOption) (*policypb.Assignment, error) {
	return &policypb.Assignment{Id: "asg-" + req.ChildId + "-" + req.ParentId}, nil
}

// MarkEmailVerified records proof of the address; false when it already was.
func (w *fakeWorld) MarkEmailVerified(_ context.Context, userID string) (bool, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	u, ok := w.users[userID]
	if !ok || u.Email == "" || w.verified[userID] {
		return false, nil
	}
	w.verified[userID] = true
	return true, nil
}

func (w *fakeWorld) emailVerified(userID string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.verified[userID]
}

func (w *fakeWorld) userByEmail(email string) *store.User {
	return w.find(func(u *store.User) bool { return strings.EqualFold(u.Email, email) })
}
