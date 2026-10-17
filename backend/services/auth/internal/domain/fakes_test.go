package domain_test

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc"

	"ngac-platform/ngac"
	"ngac-platform/pkg/grpcauth"
	messagingpb "ngac-platform/proto/messaging"
	policypb "ngac-platform/proto/policy"
	workspacepb "ngac-platform/proto/workspace"
	"ngac-platform/services/auth/internal/auth"
	"ngac-platform/services/auth/internal/domain"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
	mu          sync.Mutex
	users       map[string]*store.User // by id
	identities  map[string]string      // provider|subject -> user id
	workspaces  map[string]*store.Tenant
	members     map[string]*store.TenantMembership // tenant|user -> membership
	channels    []string                           // workspace ids that got #general
	callers     map[string]grpcauth.Caller         // downstream RPC -> caller on its context
	seq         int
	verified    map[string]bool // user id -> email proven
	deleted     []string        // workspaces removed through DeleteWorkspace
	deleteErr   error           // DeleteWorkspace fails with this
	channelErr  error           // ProvisionWorkspaceChannel fails with this
	joinErr     error           // JoinWorkspaceChannel fails with this
	provisioned []*messagingpb.ProvisionWorkspaceChannelRequest
	joins       [][2]string // workspace id, user node: who was put into #general
	joinUsers   []string    // the users.id sent with each join
	// provisionErr makes ProvisionWorkspaceChannel fail with this; joinUnimplemented
	// makes JoinWorkspaceChannel answer as a messaging service that predates it.
	provisionErr      error
	joinUnimplemented bool
	legacyCreates     []*messagingpb.CreateChannelRequest
	rpcDeadlines      map[string]time.Duration // time left on the context of each downstream call
	avatars           map[string]fakeAvatar    // token -> picture
	// graph is who sits under which workspace's Members UA, as the policy writer
	// would answer; policyDown makes every membership read fail with this.
	graph      map[string]map[string]bool // user node -> workspace ids
	policyDown error
	// edges is every assignment written through the policy writer, "child|parent"; the
	// graph above is what membership reads see, edges is what IsAssigned answers.
	edges map[string]bool
	// assignLog is every CreateAssignment / RemoveAssignment, in order ("+child>parent",
	// "-child>parent"), so a test can see what a join created and what it undid.
	assignLog []string
	// assignErr, when set, is asked before every CreateAssignment; a non-nil answer fails it.
	assignErr func(child, parent string) error
	// insertErr makes InsertTenantUser fail with this; beforeInsert sees each insert as it
	// is attempted, which is when a graph-first write must already have its edges.
	insertErr    error
	beforeInsert func(tenantID, nodeID string)
	// rowsLeaveGraphAlone makes InsertTenantUser touch only the table. By default a row
	// stands for a person who is also in the graph (rows are written after the graph).
	rowsLeaveGraphAlone bool
	// afterRemove runs after each RemoveAssignment: a test uses it to be the other
	// call that wrote a row while this one was taking its edge back.
	afterRemove func()
	// nodeTypes is the type of each node made through CreateNode (their ids look like a UA's).
	nodeTypes map[string]string
	// noTenantUAs makes the TenantMember and TenantOwner UAs not exist, as in the
	// workspaces made by autoProvisionWorkspace and the legacy ones.
	noTenantUAs bool
}

type fakeAvatar struct {
	userID string
	img    store.AvatarImages
}

func newFakeWorld() *fakeWorld {
	return &fakeWorld{
		users:        map[string]*store.User{},
		identities:   map[string]string{},
		workspaces:   map[string]*store.Tenant{},
		members:      map[string]*store.TenantMembership{},
		verified:     map[string]bool{},
		rpcDeadlines: map[string]time.Duration{},
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
	hook, insertErr, attach := w.beforeInsert, w.insertErr, !w.rowsLeaveGraphAlone && status == "active"
	w.mu.Unlock()
	if hook != nil {
		hook(tenantID, ngacNodeID)
	}
	if insertErr != nil {
		return insertErr
	}
	if attach {
		w.attach(ngacNodeID, tenantID)
	}
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
		cp.TenantType = w.workspaceType(tenantID)
		return &cp, nil
	}
	return nil, nil
}

// workspaceType is the type the workspace was created with.
func (w *fakeWorld) workspaceType(id string) string {
	w.mu.Lock()
	defer w.mu.Unlock()
	if ws, ok := w.workspaces[id]; ok {
		return ws.Type
	}
	return ""
}

// workspaceName is the name the workspace was created with.
func (w *fakeWorld) workspaceName(id string) string {
	w.mu.Lock()
	defer w.mu.Unlock()
	if ws, ok := w.workspaces[id]; ok {
		return ws.Name
	}
	return ""
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

func (w *fakeWorld) SetAvatar(_ context.Context, userID, token string, img store.AvatarImages, url string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	u := w.users[userID]
	if u == nil {
		return store.ErrNoSuchUser
	}
	if w.avatars == nil {
		w.avatars = map[string]fakeAvatar{}
	}
	w.dropAvatarOf(userID)
	w.avatars[token] = fakeAvatar{userID: userID, img: img}
	u.AvatarURL = url
	return nil
}

func (w *fakeWorld) ClearAvatar(_ context.Context, userID string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	u := w.users[userID]
	if u == nil {
		return store.ErrNoSuchUser
	}
	w.dropAvatarOf(userID)
	u.AvatarURL = ""
	return nil
}

// dropAvatarOf forgets userID's picture; the caller holds w.mu.
func (w *fakeWorld) dropAvatarOf(userID string) {
	for tok, a := range w.avatars {
		if a.userID == userID {
			delete(w.avatars, tok)
		}
	}
}

func (w *fakeWorld) GetAvatar(_ context.Context, token string, size int) ([]byte, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	a, ok := w.avatars[token]
	if !ok {
		return nil, nil
	}
	switch size {
	case 64:
		return a.img.Small, nil
	case 192:
		return a.img.Medium, nil
	case 1024:
		return a.img.Large, nil
	}
	return nil, nil
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
		var typ string
		if ws, ok := w.workspaces[m.TenantID]; ok {
			typ = ws.Type
		}
		out = append(out, store.WorkspaceSummary{ID: m.TenantID, Name: m.TenantName, Role: m.Role, MemberCount: count, Type: typ})
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
	// Like the real service: a workspace's type is stated when it is made.
	if req.Type != domain.WorkspacePersonal && req.Type != domain.WorkspaceOrganization {
		return nil, status.Error(codes.InvalidArgument, "type must be personal or organization")
	}
	id := f.w.next("ws")
	f.w.workspaces[id] = &store.Tenant{ID: id, Name: req.Name, Type: req.Type}
	// Like the real service, the creator is put under the workspace's Owners UA.
	if node := grpcauth.CallerFrom(ctx).NGACNodeID; node != "" {
		f.w.attachLocked(node, id)
		f.w.recordEdgeLocked(node, "node-"+ngac.OwnersUAName(ngac.WorkspaceID(id)))
	}
	return &workspacepb.Workspace{Id: id, Name: req.Name, Type: req.Type, PcNodeId: "pc-" + id,
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

// ProvisionWorkspaceChannel is the only way auth makes #general. The fake has no
// CreateChannel on purpose: a call to it would panic on the embedded nil client.
func (f *fakeMessaging) ProvisionWorkspaceChannel(ctx context.Context, req *messagingpb.ProvisionWorkspaceChannelRequest, _ ...grpc.CallOption) (*messagingpb.Channel, error) {
	f.w.mu.Lock()
	defer f.w.mu.Unlock()
	f.w.recordCaller("ProvisionWorkspaceChannel", ctx)
	f.w.provisioned = append(f.w.provisioned, req)
	f.w.rpcDeadlines["ProvisionWorkspaceChannel"] = deadlineIn(ctx)
	if f.w.provisionErr != nil {
		return nil, f.w.provisionErr
	}
	if f.w.channelErr != nil {
		return nil, f.w.channelErr
	}
	f.w.channels = append(f.w.channels, req.WorkspaceId)
	return &messagingpb.Channel{Id: "ch-" + req.WorkspaceId, Name: req.Name, ChannelType: "workspace"}, nil
}

// JoinWorkspaceChannel records who auth put into a workspace's #general.
func (f *fakeMessaging) JoinWorkspaceChannel(ctx context.Context, req *messagingpb.JoinWorkspaceChannelRequest, _ ...grpc.CallOption) (*messagingpb.Empty, error) {
	f.w.mu.Lock()
	defer f.w.mu.Unlock()
	f.w.recordCaller("JoinWorkspaceChannel", ctx)
	f.w.joins = append(f.w.joins, [2]string{req.WorkspaceId, req.UserNgacNodeId})
	f.w.joinUsers = append(f.w.joinUsers, req.UserId)
	f.w.rpcDeadlines["JoinWorkspaceChannel"] = deadlineIn(ctx)
	if f.w.joinUnimplemented {
		return nil, status.Error(codes.Unimplemented, "unknown method JoinWorkspaceChannel")
	}
	return &messagingpb.Empty{}, f.w.joinErr
}

// CreateChannel is what a messaging service from before ProvisionWorkspaceChannel
// answers #general with; auth falls back to it only when the new method is unknown.
func (f *fakeMessaging) CreateChannel(ctx context.Context, req *messagingpb.CreateChannelRequest, _ ...grpc.CallOption) (*messagingpb.Channel, error) {
	f.w.mu.Lock()
	defer f.w.mu.Unlock()
	f.w.recordCaller("CreateChannel", ctx)
	f.w.legacyCreates = append(f.w.legacyCreates, req)
	f.w.channels = append(f.w.channels, req.WorkspaceId)
	return &messagingpb.Channel{Id: "ch-" + req.WorkspaceId, Name: req.Name}, nil
}

// deadlineIn reports how long the context has left, or 0 when it has no deadline.
func deadlineIn(ctx context.Context) time.Duration {
	if d, ok := ctx.Deadline(); ok {
		return time.Until(d)
	}
	return 0
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
	id := f.w.next("node")
	if f.w.nodeTypes == nil {
		f.w.nodeTypes = map[string]string{}
	}
	f.w.nodeTypes[id] = req.NodeType
	return &policypb.NGACNode{Id: id, Name: req.Name, NodeType: req.NodeType}, nil
}

func (f *fakePolicyWrite) CreateAssignment(_ context.Context, req *policypb.CreateAssignmentRequest, _ ...grpc.CallOption) (*policypb.Assignment, error) {
	f.w.mu.Lock()
	failOn := f.w.assignErr
	f.w.mu.Unlock()
	if failOn != nil {
		if err := failOn(req.ChildId, req.ParentId); err != nil {
			return nil, err
		}
	}
	f.w.mu.Lock()
	f.w.assignLog = append(f.w.assignLog, "+"+req.ChildId+">"+req.ParentId)
	f.w.recordEdgeLocked(req.ChildId, req.ParentId)
	f.w.mu.Unlock()
	if ws, ok := workspaceUnder(req.ParentId); ok {
		f.w.attach(req.ChildId, ws)
	}
	return &policypb.Assignment{Id: "asg-" + req.ChildId + "-" + req.ParentId}, nil
}

func (f *fakePolicyWrite) RemoveAssignment(_ context.Context, req *policypb.RemoveAssignmentRequest, _ ...grpc.CallOption) (*policypb.Empty, error) {
	f.w.mu.Lock()
	f.w.assignLog = append(f.w.assignLog, "-"+req.ChildId+">"+req.ParentId)
	delete(f.w.edges, req.ChildId+"|"+req.ParentId)
	hook := f.w.afterRemove
	f.w.mu.Unlock()
	if ws, ok := workspaceUnder(req.ParentId); ok && !f.w.stillUnder(req.ChildId, ws) {
		f.w.detach(req.ChildId, ws)
	}
	if hook != nil {
		hook()
	}
	return &policypb.Empty{}, nil
}

// workspaceUnder is the workspace whose Members, Owners, TenantMember or TenantOwner UA
// the node id names (the fake's UA ids are "node-" plus the UA's name). Each of them
// puts whoever sits under it under the workspace's Members UA.
func workspaceUnder(parentID string) (string, bool) {
	name, ok := strings.CutPrefix(parentID, "node-")
	if !ok {
		return "", false
	}
	if ws, ok := ngac.WorkspaceIDFromMembersUAName(name); ok {
		return string(ws), true
	}
	for _, suffix := range []string{ngac.OwnersUAName("")} {
		if ws, ok := strings.CutSuffix(name, suffix); ok {
			return ws, true
		}
	}
	for _, prefix := range []string{ngac.TenantMemberUAName(""), ngac.TenantOwnerUAName("")} {
		if ws, ok := strings.CutPrefix(name, prefix); ok {
			return ws, true
		}
	}
	return "", false
}

// stillUnder reports whether some edge of the node still puts it under the workspace
// (it may hold a Members edge and a TenantMember edge at once).
func (w *fakeWorld) stillUnder(node, ws string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	for e := range w.edges {
		child, parent, _ := strings.Cut(e, "|")
		if got, ok := workspaceUnder(parent); child == node && ok && got == ws {
			return true
		}
	}
	return false
}

// recordEdgeLocked notes an assignment; the caller holds w.mu.
func (w *fakeWorld) recordEdgeLocked(child, parent string) {
	if w.edges == nil {
		w.edges = map[string]bool{}
	}
	w.edges[child+"|"+parent] = true
}

// attachLocked is attach for a caller that holds w.mu.
func (w *fakeWorld) attachLocked(node, ws string) {
	if w.graph == nil {
		w.graph = map[string]map[string]bool{}
	}
	if w.graph[node] == nil {
		w.graph[node] = map[string]bool{}
	}
	w.graph[node][ws] = true
}

// attach puts the user node under the workspace's Members UA in the fake graph.
func (w *fakeWorld) attach(node, ws string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.attachLocked(node, ws)
}

// detach takes the user node out from under the workspace's Members UA.
func (w *fakeWorld) detach(node, ws string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	delete(w.graph[node], ws)
}

// assignments is the log of CreateAssignment ("+child>parent") and RemoveAssignment
// ("-child>parent") calls so far.
func (w *fakeWorld) assignments() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return slices.Clone(w.assignLog)
}

// assigned reports whether the edge was written and not removed.
func (w *fakeWorld) assigned(child, parent string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.edges[child+"|"+parent]
}

func (f *fakePolicyWrite) down() error {
	f.w.mu.Lock()
	defer f.w.mu.Unlock()
	return f.w.policyDown
}

func (f *fakePolicyWrite) FindNodeByName(_ context.Context, req *policypb.FindNodeByNameRequest, _ ...grpc.CallOption) (*policypb.NGACNode, error) {
	if err := f.down(); err != nil {
		return nil, err
	}
	f.w.mu.Lock()
	missing := f.w.noTenantUAs && (strings.HasPrefix(req.Name, ngac.TenantMemberUAName("")) || strings.HasPrefix(req.Name, ngac.TenantOwnerUAName("")))
	f.w.mu.Unlock()
	if missing {
		return nil, status.Error(codes.NotFound, "node "+req.Name+" not found")
	}
	return &policypb.NGACNode{Id: "node-" + req.Name, Name: req.Name, NodeType: req.NodeType}, nil
}

// IsAssigned answers from the edges written through this fake. A workspace's Members UA
// is under its policy class by construction, which is all MembersUA asks of it.
func (f *fakePolicyWrite) IsAssigned(_ context.Context, req *policypb.IsAssignedRequest, _ ...grpc.CallOption) (*policypb.BoolResponse, error) {
	if err := f.down(); err != nil {
		return nil, err
	}
	// The workspace's own UAs are chained by construction: each is under its policy class,
	// and Owners, TenantMember and TenantOwner are under Members/Owners.
	childWS, childIsWorkspaceUA := workspaceUnder(req.ChildId)
	if childIsWorkspaceUA && strings.HasPrefix(req.ParentId, "node-"+ngac.PCName("")) {
		return &policypb.BoolResponse{Value: true}, nil
	}
	if parentWS, parentIsWorkspaceUA := workspaceUnder(req.ParentId); childIsWorkspaceUA && parentIsWorkspaceUA && childWS == parentWS && req.ChildId != req.ParentId {
		return &policypb.BoolResponse{Value: true}, nil
	}
	return &policypb.BoolResponse{Value: f.w.assigned(req.ChildId, req.ParentId)}, nil
}

func (f *fakePolicyWrite) GetNode(_ context.Context, req *policypb.GetNodeRequest, _ ...grpc.CallOption) (*policypb.NGACNode, error) {
	if err := f.down(); err != nil {
		return nil, err
	}
	nodeType := ngac.TypeU
	f.w.mu.Lock()
	created, made := f.w.nodeTypes[req.NodeId]
	f.w.mu.Unlock()
	switch {
	case made:
		nodeType = created
	case strings.HasPrefix(req.NodeId, "node-"):
		nodeType = ngac.TypeUA
	}
	return &policypb.NGACNode{Id: req.NodeId, NodeType: nodeType}, nil
}

func (f *fakePolicyWrite) GetAncestors(_ context.Context, req *policypb.GetAncestorsRequest, _ ...grpc.CallOption) (*policypb.NodeList, error) {
	if err := f.down(); err != nil {
		return nil, err
	}
	f.w.mu.Lock()
	defer f.w.mu.Unlock()
	var out []*policypb.NGACNode
	for ws := range f.w.graph[req.NodeId] {
		name := ngac.MembersUAName(ngac.WorkspaceID(ws))
		out = append(out, &policypb.NGACNode{Id: "node-" + name, Name: name, NodeType: ngac.TypeUA})
		if owners := ngac.OwnersUAName(ngac.WorkspaceID(ws)); f.w.edges[req.NodeId+"|node-"+owners] {
			out = append(out, &policypb.NGACNode{Id: "node-" + owners, Name: owners, NodeType: ngac.TypeUA})
		}
	}
	return &policypb.NodeList{Nodes: out}, nil
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

// storeRow is an active member row as another call would have written it.
func storeRow(tenantID, userID, nodeID string) *store.TenantMembership {
	return &store.TenantMembership{TenantID: tenantID, UserID: userID, Role: "member", Status: "active", NGACNodeID: nodeID}
}
