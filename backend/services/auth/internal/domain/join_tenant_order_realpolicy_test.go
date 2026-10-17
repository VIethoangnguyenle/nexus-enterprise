//go:build realpolicy

package domain_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"ngac-platform/ngac"
	"ngac-platform/pkg/grpcauth"
	"ngac-platform/pkg/policyclient"
	policypb "ngac-platform/proto/policy"
	workspacepb "ngac-platform/proto/workspace"
	"ngac-platform/services/auth/internal/auth"
	"ngac-platform/services/auth/internal/domain"
	"ngac-platform/services/auth/internal/store"
	"ngac-platform/testutil"
)

// These tests run the auth service's tenant joins and sign-in against the real policy
// service. The graph is the truth: a join writes it first (under the workspace's own
// Members/Owners UA, which every workspace has, whether or not it has TenantMember
// UAs) and lists the person after. Each deny has its allow on the same fixture.

// flakyWrite is the policy writer with switches: down makes every graph call fail as an
// unreachable service would, and failAssignTo makes assigning anyone under that UA fail.
type flakyWrite struct {
	policypb.PolicyWriteServiceClient
	down         atomic.Bool
	failAssignTo atomic.Value // string
}

var errPolicyDown = errors.New("policy: connection refused")

func (f *flakyWrite) gate() error {
	if f.down.Load() {
		return errPolicyDown
	}
	return nil
}

func (f *flakyWrite) FindNodeByName(ctx context.Context, in *policypb.FindNodeByNameRequest, o ...grpc.CallOption) (*policypb.NGACNode, error) {
	if err := f.gate(); err != nil {
		return nil, err
	}
	return f.PolicyWriteServiceClient.FindNodeByName(ctx, in, o...)
}

func (f *flakyWrite) IsAssigned(ctx context.Context, in *policypb.IsAssignedRequest, o ...grpc.CallOption) (*policypb.BoolResponse, error) {
	if err := f.gate(); err != nil {
		return nil, err
	}
	return f.PolicyWriteServiceClient.IsAssigned(ctx, in, o...)
}

func (f *flakyWrite) GetNode(ctx context.Context, in *policypb.GetNodeRequest, o ...grpc.CallOption) (*policypb.NGACNode, error) {
	if err := f.gate(); err != nil {
		return nil, err
	}
	return f.PolicyWriteServiceClient.GetNode(ctx, in, o...)
}

func (f *flakyWrite) GetAncestors(ctx context.Context, in *policypb.GetAncestorsRequest, o ...grpc.CallOption) (*policypb.NodeList, error) {
	if err := f.gate(); err != nil {
		return nil, err
	}
	return f.PolicyWriteServiceClient.GetAncestors(ctx, in, o...)
}

func (f *flakyWrite) CreateAssignment(ctx context.Context, in *policypb.CreateAssignmentRequest, o ...grpc.CallOption) (*policypb.Assignment, error) {
	if err := f.gate(); err != nil {
		return nil, err
	}
	if to, _ := f.failAssignTo.Load().(string); to != "" && in.ParentId == to {
		return nil, errors.New("policy: injected failure")
	}
	return f.PolicyWriteServiceClient.CreateAssignment(ctx, in, o...)
}

func (f *flakyWrite) RemoveAssignment(ctx context.Context, in *policypb.RemoveAssignmentRequest, o ...grpc.CallOption) (*policypb.Empty, error) {
	if err := f.gate(); err != nil {
		return nil, err
	}
	return f.PolicyWriteServiceClient.RemoveAssignment(ctx, in, o...)
}

// graphWorkspace is the workspace service as the real one behaves towards the graph:
// CreateWorkspace builds the workspace's topology in the policy service and puts the
// creator under Owners. It builds no TenantMember or TenantOwner UA.
type graphWorkspace struct {
	*fakeWorkspace
	t     *testing.T
	write policypb.PolicyWriteServiceClient
}

func (g *graphWorkspace) CreateWorkspace(ctx context.Context, req *workspacepb.CreateWorkspaceRequest, _ ...grpc.CallOption) (*workspacepb.Workspace, error) {
	topo := testutil.SeedWorkspaceGraph(g.t, g.write)
	g.w.mu.Lock()
	g.w.workspaces[topo.WorkspaceID] = &store.Tenant{ID: topo.WorkspaceID, Name: req.Name, Type: req.Type}
	g.w.mu.Unlock()
	if node := callerNode(ctx); node != "" {
		testutil.Assign(g.t, g.write, node, topo.Owners)
	}
	return &workspacepb.Workspace{Id: topo.WorkspaceID, Name: req.Name, Type: req.Type,
		PcNodeId: topo.PC, OwnersUaId: topo.Owners, MembersUaId: topo.Members}, nil
}

type joinWorld struct {
	t     *testing.T
	w     *fakeWorld
	svc   *domain.Service
	write *flakyWrite
	read  policypb.PolicyReadServiceClient
	raw   policypb.PolicyWriteServiceClient
}

func newJoinWorld(t *testing.T) *joinWorld {
	t.Helper()
	auth.SetJWTSecret("test-secret-key-for-testing-only")
	read, raw := testutil.StartPolicy(t)
	w := newFakeWorld()
	flaky := &flakyWrite{PolicyWriteServiceClient: raw}
	ws := &graphWorkspace{fakeWorkspace: &fakeWorkspace{w: w}, t: t, write: raw}
	svc := domain.NewService(w, testRedisClient(t), read, flaky, ws, &fakeMessaging{w: w})
	return &joinWorld{t: t, w: w, svc: svc, write: flaky, read: read, raw: raw}
}

// workspace seeds a workspace in the graph and the table. withTenantUAs adds the
// TenantMember UA under Members, as an old organization has.
func (j *joinWorld) workspace(name, domainName string, withTenantUAs bool) testutil.WorkspaceGraph {
	j.t.Helper()
	g := testutil.SeedWorkspaceGraph(j.t, j.raw)
	if withTenantUAs {
		testutil.NewNode(j.t, j.raw, ngac.TenantMemberUAName(ngac.WorkspaceID(g.WorkspaceID)), ngac.TypeUA, g.Members)
	}
	j.w.mu.Lock()
	j.w.workspaces[g.WorkspaceID] = &store.Tenant{ID: g.WorkspaceID, Name: name, Domain: domainName}
	j.w.mu.Unlock()
	return g
}

// user makes an account whose graph node sits under the given UAs.
func (j *joinWorld) user(parents ...string) *store.User {
	j.t.Helper()
	u := j.w.addUser(uniqueEmail())
	// A fresh id on every run: the per-person counters in Redis (workspace creation)
	// outlive the process, and the fake's sequential ids would meet them again.
	j.w.mu.Lock()
	delete(j.w.users, u.ID)
	u.ID = uuid.NewString()
	j.w.users[u.ID] = u
	j.w.mu.Unlock()
	u.NGACNodeID = testutil.NewUser(j.t, j.raw, parents...)
	return u
}

func (j *joinWorld) under(u *store.User, parent string) bool {
	j.t.Helper()
	r, err := j.raw.IsAssigned(testutil.PolicyCaller(context.Background(), u.NGACNodeID),
		&policypb.IsAssignedRequest{ChildId: u.NGACNodeID, ParentId: parent})
	require.NoError(j.t, err)
	return r.GetValue()
}

func (j *joinWorld) isMember(u *store.User, ws string) bool {
	j.t.Helper()
	ok, err := policyclient.IsWorkspaceMember(testutil.PolicyCaller(context.Background(), u.NGACNodeID), j.raw, u.NGACNodeID, ngac.WorkspaceID(ws))
	require.NoError(j.t, err)
	return ok
}

func (j *joinWorld) tenantMemberUA(ws string) string {
	j.t.Helper()
	n, err := j.raw.FindNodeByName(testutil.PolicyAdmin(context.Background()), &policypb.FindNodeByNameRequest{
		Name: ngac.TenantMemberUAName(ngac.WorkspaceID(ws)), NodeType: ngac.TypeUA})
	if err != nil {
		return ""
	}
	return n.GetId()
}

// A workspace without a TenantMember UA still takes a member: the edge goes under its
// Members UA. Where the UA exists the member is under it as well.
func TestRealPolicy_JoinTenantWorksWithoutATenantMemberUA(t *testing.T) {
	j := newJoinWorld(t)
	ctx := context.Background()
	bare := j.workspace("Bare", "", false)
	old := j.workspace("Old", "", true)
	u := j.user()

	require.NoError(t, j.svc.JoinTenantForTest(ctx, bare.WorkspaceID, u.ID, u.NGACNodeID, "member"))
	assert.True(t, j.under(u, bare.Members), "assigned under the workspace's Members UA")
	assert.True(t, j.isMember(u, bare.WorkspaceID))
	row := j.w.membership(bare.WorkspaceID, u.ID)
	require.NotNil(t, row)
	assert.Equal(t, "active", row.Status)

	// Allow: the workspace that has the UA gets the member under both.
	require.NoError(t, j.svc.JoinTenantForTest(ctx, old.WorkspaceID, u.ID, u.NGACNodeID, "member"))
	assert.True(t, j.under(u, old.Members))
	assert.True(t, j.under(u, j.tenantMemberUA(old.WorkspaceID)), "and under TenantMember where it exists")
}

func TestRealPolicy_JoinTenantAnOwnerGoesUnderOwners(t *testing.T) {
	j := newJoinWorld(t)
	g := j.workspace("Bare", "", false)
	u := j.user()

	require.NoError(t, j.svc.JoinTenantForTest(context.Background(), g.WorkspaceID, u.ID, u.NGACNodeID, "owner"))
	assert.True(t, j.under(u, g.Owners))
	assert.True(t, j.isMember(u, g.WorkspaceID), "Owners sits under Members, so an owner is a member")
	row := j.w.membership(g.WorkspaceID, u.ID)
	require.NotNil(t, row)
	assert.Equal(t, "owner", row.Role)
}

// The row is written after the edge: when it is attempted, the person is already
// a member in the graph.
func TestRealPolicy_JoinTenantWritesTheEdgeBeforeTheRow(t *testing.T) {
	j := newJoinWorld(t)
	g := j.workspace("Bare", "", false)
	u := j.user()
	var memberAtInsert bool
	j.w.beforeInsert = func(tenantID, nodeID string) {
		ok, err := policyclient.IsWorkspaceMember(testutil.PolicyCaller(context.Background(), nodeID), j.raw, nodeID, ngac.WorkspaceID(tenantID))
		memberAtInsert = err == nil && ok
	}

	require.NoError(t, j.svc.JoinTenantForTest(context.Background(), g.WorkspaceID, u.ID, u.NGACNodeID, "member"))
	assert.True(t, memberAtInsert, "the row was written before the graph agreed")
}

// A graph write that fails leaves no row, and the join says so; the same fixture with
// a healthy graph joins.
func TestRealPolicy_JoinTenantAFailedGraphWriteLeavesNoRow(t *testing.T) {
	j := newJoinWorld(t)
	g := j.workspace("Bare", "", false)
	denied, fine := j.user(), j.user()

	j.write.failAssignTo.Store(g.Members)
	err := j.svc.JoinTenantForTest(context.Background(), g.WorkspaceID, denied.ID, denied.NGACNodeID, "member")
	require.Error(t, err)
	assert.Nil(t, j.w.membership(g.WorkspaceID, denied.ID), "no row for somebody the graph does not hold")
	assert.False(t, j.isMember(denied, g.WorkspaceID))

	j.write.failAssignTo.Store("")
	require.NoError(t, j.svc.JoinTenantForTest(context.Background(), g.WorkspaceID, fine.ID, fine.NGACNodeID, "member"))
	assert.True(t, j.isMember(fine, g.WorkspaceID))
}

// A row that cannot be written takes back the edges this call made and only those:
// the person who was already under the workspace keeps that edge.
func TestRealPolicy_JoinTenantARowFailureUndoesOnlyItsOwnEdges(t *testing.T) {
	j := newJoinWorld(t)
	ctx := context.Background()
	g := j.workspace("Old", "", true)
	tm := j.tenantMemberUA(g.WorkspaceID)
	fresh := j.user()
	heldMembers := j.user(g.Members)
	heldTenantMember := j.user(tm) // a member through TenantMember only: no direct Members edge
	j.w.insertErr = errors.New("database: down")

	require.Error(t, j.svc.JoinTenantForTest(ctx, g.WorkspaceID, fresh.ID, fresh.NGACNodeID, "member"))
	assert.False(t, j.isMember(fresh, g.WorkspaceID), "every edge this call made is gone")

	require.Error(t, j.svc.JoinTenantForTest(ctx, g.WorkspaceID, heldMembers.ID, heldMembers.NGACNodeID, "member"))
	assert.True(t, j.under(heldMembers, g.Members), "the Members edge they had survives")
	assert.False(t, j.under(heldMembers, tm), "the TenantMember edge this call made is taken back")

	require.Error(t, j.svc.JoinTenantForTest(ctx, g.WorkspaceID, heldTenantMember.ID, heldTenantMember.NGACNodeID, "member"))
	assert.True(t, j.under(heldTenantMember, tm), "the TenantMember edge they had survives")
	assert.False(t, j.under(heldTenantMember, g.Members), "the Members edge this call made is taken back")
	assert.True(t, j.isMember(heldTenantMember, g.WorkspaceID), "and they are still a member through the edge they had")
	for _, u := range []*store.User{fresh, heldMembers, heldTenantMember} {
		assert.Nil(t, j.w.membership(g.WorkspaceID, u.ID), "a failed join leaves no row")
	}

	// Allow, same fixture: with the table back, the same people join.
	j.w.insertErr = nil
	for _, u := range []*store.User{fresh, heldMembers, heldTenantMember} {
		require.NoError(t, j.svc.JoinTenantForTest(ctx, g.WorkspaceID, u.ID, u.NGACNodeID, "member"))
		assert.True(t, j.isMember(u, g.WorkspaceID))
		assert.NotNil(t, j.w.membership(g.WorkspaceID, u.ID))
	}
}

// A person creating a workspace on purpose: the graph is confirmed before the row is
// written, and a failure undoes the whole workspace and leaves no row.
func TestRealPolicy_CreateMyWorkspaceIsGraphFirstAndUndoneAsAWhole(t *testing.T) {
	j := newJoinWorld(t)
	ctx := context.Background()
	u := j.user()
	j.w.verified[u.ID] = true
	var ownerEdgesAtInsert bool
	j.w.beforeInsert = func(tenantID, nodeID string) {
		// The owner is under Owners and under the TenantOwner UA this workspace was
		// given when it was made: the second is the edge the strict path itself writes.
		held := 0
		for _, name := range []string{ngac.OwnersUAName(ngac.WorkspaceID(tenantID)), ngac.TenantOwnerUAName(ngac.WorkspaceID(tenantID))} {
			ua, err := j.raw.FindNodeByName(testutil.PolicyAdmin(ctx), &policypb.FindNodeByNameRequest{Name: name, NodeType: ngac.TypeUA})
			if err != nil {
				return
			}
			if r, err := j.raw.IsAssigned(testutil.PolicyCaller(ctx, nodeID), &policypb.IsAssignedRequest{ChildId: nodeID, ParentId: ua.GetId()}); err == nil && r.GetValue() {
				held++
			}
		}
		ownerEdgesAtInsert = held == 2
	}

	ws, err := j.svc.CreateMyWorkspace(ctx, u.ID, u.NGACNodeID, "Nhóm mới", domain.WorkspaceOrganization)
	require.NoError(t, err)
	assert.True(t, ownerEdgesAtInsert, "the owner's edges were written before the row")
	assert.True(t, j.isMember(u, ws.ID))
	row := j.w.membership(ws.ID, u.ID)
	require.NotNil(t, row)
	assert.Equal(t, "owner", row.Role)
	assert.Empty(t, j.w.deleted, "a workspace that worked is not undone")

	// Deny: the row cannot be written. The workspace is undone and nobody is listed.
	j.w.insertErr = errors.New("database: down")
	_, err = j.svc.CreateMyWorkspace(ctx, u.ID, u.NGACNodeID, "Nhóm hỏng", domain.WorkspaceOrganization)
	require.Error(t, err)
	require.Len(t, j.w.deleted, 1, "the half-made workspace is removed")
	assert.Nil(t, j.w.membership(j.w.deleted[0], u.ID), "and no row names the person in it")
}

// Signing up by code makes a personal workspace that has no TenantMember UA (the
// workspace service never builds one). The new person is its owner in the graph and in
// the table, signs in to it, and keeps refreshing into it.
func TestRealPolicy_OTPSignUpWithoutTenantMemberUAWorks(t *testing.T) {
	j := newJoinWorld(t)
	ctx := context.Background()

	sid, err := j.svc.RequestOTP(ctx, uniqueEmail(), "email")
	require.NoError(t, err)
	res, err := j.svc.VerifyOTP(ctx, sid, "999999")
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = j.raw.DeleteNode(ctx, &policypb.DeleteNodeRequest{NodeId: res.NGACNodeID})
	})

	require.NotEmpty(t, res.TenantID, "the new account is signed into its workspace")
	assert.Equal(t, res.TenantID, tenantOf(t, res.Token))
	row := j.w.membership(res.TenantID, res.UserID)
	require.NotNil(t, row)
	assert.Equal(t, "owner", row.Role)
	assert.Equal(t, "active", row.Status)
	assert.Empty(t, j.tenantMemberUA(res.TenantID), "the workspace has no TenantMember UA, and does not need one")
	u := &store.User{ID: res.UserID, NGACNodeID: res.NGACNodeID}
	assert.True(t, j.isMember(u, res.TenantID))

	refresh, err := j.svc.AttachRefreshToken(ctx, domain.RefreshIdentity{
		UserID: res.UserID, Username: res.Username, NGACNodeID: res.NGACNodeID, TenantID: res.TenantID, SessionID: res.SessionID,
	})
	require.NoError(t, err)
	next, tenant, err := refreshTenant(j.svc, refresh)
	require.NoError(t, err)
	assert.Equal(t, res.TenantID, tenant)
	_, tenant, err = refreshTenant(j.svc, next)
	require.NoError(t, err, "the rotated token keeps working")
	assert.Equal(t, res.TenantID, tenant)
}

func refreshTenant(svc *domain.Service, token string) (next, tenant string, err error) {
	_, next, tenant, err = svc.RefreshSession(context.Background(), token)
	return next, tenant, err
}

// A Google Workspace account of a company whose workspace has no TenantMember UA joins
// it and signs in to it. An account of another domain does not join.
func TestRealPolicy_GoogleHostedDomainJoinsAnOrgWithoutTenantMemberUA(t *testing.T) {
	j := newJoinWorld(t)
	ctx := context.Background()
	domainName := "org-" + uuid.NewString()[:8] + ".test"
	org := j.workspace("Org", domainName, false)
	member, outsider := j.user(), j.user()
	for _, u := range []*store.User{member, outsider} {
		require.NoError(t, j.w.LinkIdentity(ctx, domain.ProviderGoogle, "sub-"+u.ID, u.ID, u.Email))
	}

	res, err := j.svc.SignInWithGoogle(ctx, googleIdentity("sub-"+member.ID, member.Email, domainName))
	require.NoError(t, err)
	assert.Equal(t, org.WorkspaceID, res.DefaultTenantID, "joined and signed into the company workspace")
	assert.Equal(t, org.WorkspaceID, tenantOf(t, res.Token))
	assert.True(t, j.under(member, org.Members))
	row := j.w.membership(org.WorkspaceID, member.ID)
	require.NotNil(t, row)
	assert.Equal(t, "member", row.Role)

	res, err = j.svc.SignInWithGoogle(ctx, googleIdentity("sub-"+outsider.ID, outsider.Email, "other-"+domainName))
	require.NoError(t, err)
	assert.Empty(t, res.DefaultTenantID)
	assert.False(t, j.isMember(outsider, org.WorkspaceID))
	assert.Nil(t, j.w.membership(org.WorkspaceID, outsider.ID))
}

func callerNode(ctx context.Context) string { return grpcauth.CallerFrom(ctx).NGACNodeID }
