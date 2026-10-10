package domain_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"ngac-platform/ngac"
	policypb "ngac-platform/proto/policy"
	"ngac-platform/services/workspace/internal/domain"
	"ngac-platform/services/workspace/internal/store"
)

// ---------------------------------------------------------------------------
// Fixture
//
// Two workspaces, each with its own PC and Mgmt OA. The fake PDP answers
// CheckAccess from an explicit grant table — anything not listed is DENY, which
// mirrors the real system's default. Subjects:
//
//	owner      — every op on ws-1's Mgmt and Documents OAs
//	member     — read/write/upload on ws-1 Documents; nothing on Mgmt
//	inviter    — only invite on ws-1 Mgmt
//	manager    — only manage on ws-1 Mgmt
//	delegator  — manage on ws-1 Mgmt, but only read on ws-1 Documents
//	otherOwner — every op on ws-2's Mgmt; nothing in ws-1 (different PC)
//	outsider   — nothing anywhere
// ---------------------------------------------------------------------------

const (
	ws1, ws2 = "ws-1", "ws-2"
	pc1, pc2 = "pc-1", "pc-2"

	mgmt1, mgmt2 = "oa-mgmt-1", "oa-mgmt-2"
	docs1        = "oa-docs-1"
	folder1      = "oa-folder-1"
	folder2      = "oa-folder-2"
	owners1      = "ua-owners-1"
	members1     = "ua-members-1"
	role1        = "ua-role-1"
	owners2      = "ua-owners-2"

	owner      = "u-owner"
	member     = "u-member"
	inviter    = "u-inviter"
	manager    = "u-manager"
	delegator  = "u-delegator"
	otherOwner = "u-other-owner"
	outsider   = "u-outsider"
	target     = "u-target"
	leaver     = "u-leaver"

	assets1  = "oa-assets-1"
	chat1    = "oa-channels-1"
	roleRead = "ua-role-read"
	roleEdit = "ua-role-edit"
)

type grant struct{ user, obj, op string }

type fakePolicyRead struct {
	policypb.PolicyReadServiceClient
	grants      map[grant]bool
	children    map[string][]*policypb.NGACNode
	descendants map[string][]*policypb.NGACNode
	ancestors   map[string][]*policypb.NGACNode
	nodes       map[string]*policypb.NGACNode
	parents     map[string][]*policypb.NGACNode
	assocs      map[string][]*policypb.Association
	assocErr    error
	checkErr    error
	ancErr      error
	checks      []*policypb.CheckAccessRequest
}

func (f *fakePolicyRead) CheckAccess(_ context.Context, req *policypb.CheckAccessRequest, _ ...grpc.CallOption) (*policypb.AccessDecision, error) {
	f.checks = append(f.checks, req)
	if f.checkErr != nil {
		return nil, f.checkErr
	}
	if f.grants[grant{req.UserNodeId, req.ObjectNodeId, req.Operation}] {
		return &policypb.AccessDecision{Decision: ngac.DecisionAllow}, nil
	}
	return &policypb.AccessDecision{Decision: ngac.DecisionDeny}, nil
}

func (f *fakePolicyRead) GetChildren(_ context.Context, req *policypb.GetChildrenRequest, _ ...grpc.CallOption) (*policypb.NodeList, error) {
	return &policypb.NodeList{Nodes: f.children[req.NodeId]}, nil
}

func (f *fakePolicyRead) GetDescendants(_ context.Context, req *policypb.GetDescendantsRequest, _ ...grpc.CallOption) (*policypb.NodeList, error) {
	return &policypb.NodeList{Nodes: f.descendants[req.NodeId]}, nil
}

func (f *fakePolicyRead) GetAncestors(_ context.Context, req *policypb.GetAncestorsRequest, _ ...grpc.CallOption) (*policypb.NodeList, error) {
	if f.ancErr != nil {
		return nil, f.ancErr
	}
	return &policypb.NodeList{Nodes: f.ancestors[req.NodeId]}, nil
}

func (f *fakePolicyRead) GetNode(_ context.Context, req *policypb.GetNodeRequest, _ ...grpc.CallOption) (*policypb.NGACNode, error) {
	if n, ok := f.nodes[req.NodeId]; ok {
		return n, nil
	}
	return nil, status.Error(codes.NotFound, "node not found")
}

func (f *fakePolicyRead) GetParents(_ context.Context, req *policypb.GetParentsRequest, _ ...grpc.CallOption) (*policypb.NodeList, error) {
	return &policypb.NodeList{Nodes: f.parents[req.NodeId]}, nil
}

func (f *fakePolicyRead) GetAssociations(_ context.Context, req *policypb.GetAssociationsRequest, _ ...grpc.CallOption) (*policypb.AssociationList, error) {
	if f.assocErr != nil {
		return nil, f.assocErr
	}
	return &policypb.AssociationList{Associations: f.assocs[req.UaId]}, nil
}

type fakePolicyWrite struct {
	policypb.PolicyWriteServiceClient
	// src answers GetAssociations as the writer: by default what the read side has.
	src          *fakePolicyRead
	writerAssocs map[string][]*policypb.Association
	// writer views that differ from the (stale) read side
	writerChildren, writerDescendants, writerAncestors map[string][]*policypb.NGACNode
	mutations                                          []string
	assocs                                             []*policypb.CreateAssociationRequest
	n                                                  int
}

func (f *fakePolicyWrite) GetAssociations(_ context.Context, req *policypb.GetAssociationsRequest, _ ...grpc.CallOption) (*policypb.AssociationList, error) {
	if as, ok := f.writerAssocs[req.UaId]; ok {
		return &policypb.AssociationList{Associations: as}, nil
	}
	if f.src == nil {
		return &policypb.AssociationList{}, nil
	}
	return f.src.GetAssociations(context.Background(), req)
}

// Graph reads answer as the writer: by default what the read side has, unless a
// test gives the writer its own fresher view.
func (f *fakePolicyWrite) GetChildren(ctx context.Context, req *policypb.GetChildrenRequest, _ ...grpc.CallOption) (*policypb.NodeList, error) {
	if ns, ok := f.writerChildren[req.NodeId]; ok {
		return &policypb.NodeList{Nodes: ns}, nil
	}
	return f.src.GetChildren(ctx, req)
}

func (f *fakePolicyWrite) GetDescendants(ctx context.Context, req *policypb.GetDescendantsRequest, _ ...grpc.CallOption) (*policypb.NodeList, error) {
	if ns, ok := f.writerDescendants[req.NodeId]; ok {
		return &policypb.NodeList{Nodes: ns}, nil
	}
	return f.src.GetDescendants(ctx, req)
}

func (f *fakePolicyWrite) GetAncestors(ctx context.Context, req *policypb.GetAncestorsRequest, _ ...grpc.CallOption) (*policypb.NodeList, error) {
	if ns, ok := f.writerAncestors[req.NodeId]; ok {
		return &policypb.NodeList{Nodes: ns}, nil
	}
	return f.src.GetAncestors(ctx, req)
}

func (f *fakePolicyWrite) GetParents(ctx context.Context, req *policypb.GetParentsRequest, _ ...grpc.CallOption) (*policypb.NodeList, error) {
	return f.src.GetParents(ctx, req)
}

func (f *fakePolicyWrite) GetNode(ctx context.Context, req *policypb.GetNodeRequest, _ ...grpc.CallOption) (*policypb.NGACNode, error) {
	return f.src.GetNode(ctx, req)
}

func (f *fakePolicyWrite) record(s string) { f.mutations = append(f.mutations, s) }

func (f *fakePolicyWrite) CreateNode(_ context.Context, req *policypb.CreateNodeRequest, _ ...grpc.CallOption) (*policypb.NGACNode, error) {
	f.record("CreateNode " + req.Name)
	f.n++
	return &policypb.NGACNode{Id: fmt.Sprintf("new-%d", f.n), Name: req.Name, NodeType: req.NodeType}, nil
}

func (f *fakePolicyWrite) DeleteNode(_ context.Context, req *policypb.DeleteNodeRequest, _ ...grpc.CallOption) (*policypb.Empty, error) {
	f.record("DeleteNode " + req.NodeId)
	return &policypb.Empty{}, nil
}

func (f *fakePolicyWrite) CreateAssignment(_ context.Context, req *policypb.CreateAssignmentRequest, _ ...grpc.CallOption) (*policypb.Assignment, error) {
	f.record("CreateAssignment " + req.ChildId + "->" + req.ParentId)
	return &policypb.Assignment{Id: "asg"}, nil
}

func (f *fakePolicyWrite) RemoveAssignment(_ context.Context, req *policypb.RemoveAssignmentRequest, _ ...grpc.CallOption) (*policypb.Empty, error) {
	f.record("RemoveAssignment " + req.ChildId + "->" + req.ParentId)
	return &policypb.Empty{}, nil
}

func (f *fakePolicyWrite) CreateAssociation(_ context.Context, req *policypb.CreateAssociationRequest, _ ...grpc.CallOption) (*policypb.Association, error) {
	f.record("CreateAssociation " + req.UaId + "->" + req.OaId)
	f.assocs = append(f.assocs, req)
	return &policypb.Association{Id: "assoc"}, nil
}

func (f *fakePolicyWrite) RemoveAssociation(_ context.Context, req *policypb.RemoveAssociationRequest, _ ...grpc.CallOption) (*policypb.Empty, error) {
	f.record("RemoveAssociation " + req.UaId + "->" + req.OaId)
	return &policypb.Empty{}, nil
}

type fakeWSStore struct {
	ws        map[string]*store.Workspace
	mu        sync.Mutex
	mutations []string
}

func (f *fakeWSStore) UpdateDetails(_ context.Context, id string, name, desc *string) (string, string, error) {
	f.mutations = append(f.mutations, "details "+id)
	if name != nil {
		f.ws[id].Name = *name
	}
	if desc != nil {
		f.ws[id].Desc = *desc
	}
	return f.ws[id].Name, f.ws[id].Desc, nil
}

// WithOwnerLock serialises owner changes per process, as the real store does per workspace.
func (f *fakeWSStore) WithOwnerLock(ctx context.Context, _ string, fn func(context.Context) error) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return fn(ctx)
}

func (f *fakeWSStore) Insert(_ context.Context, w *store.Workspace) error { f.ws[w.ID] = w; return nil }
func (f *fakeWSStore) GetByID(_ context.Context, id string) (*store.Workspace, error) {
	if w, ok := f.ws[id]; ok {
		return w, nil
	}
	return nil, errors.New("no rows")
}
func (f *fakeWSStore) ListAll(_ context.Context) ([]*store.Workspace, error) {
	var out []*store.Workspace
	for _, w := range f.ws {
		out = append(out, w)
	}
	return out, nil
}

type fakeDeptStore struct {
	depts     map[string]*store.Department
	mutations []string
}

func (f *fakeDeptStore) InsertDepartment(_ context.Context, d *store.Department) error {
	f.mutations = append(f.mutations, "insert")
	f.depts[d.ID] = d
	return nil
}
func (f *fakeDeptStore) ListDepartmentsByWorkspace(_ context.Context, wsID string) ([]*store.Department, error) {
	var out []*store.Department
	for _, d := range f.depts {
		if d.WorkspaceID == wsID {
			out = append(out, d)
		}
	}
	return out, nil
}
func (f *fakeDeptStore) GetDepartment(_ context.Context, id string) (*store.Department, error) {
	if d, ok := f.depts[id]; ok {
		return d, nil
	}
	return nil, errors.New("no rows")
}
func (f *fakeDeptStore) UpdateDepartmentName(_ context.Context, id, _ string) error {
	f.mutations = append(f.mutations, "rename "+id)
	return nil
}
func (f *fakeDeptStore) MoveDepartment(_ context.Context, id string, _ *string) error {
	f.mutations = append(f.mutations, "move "+id)
	return nil
}
func (f *fakeDeptStore) DeleteDepartment(_ context.Context, id string) error {
	f.mutations = append(f.mutations, "delete "+id)
	return nil
}
func (f *fakeDeptStore) UpdateUserDepartment(_ context.Context, tenant, u string, d *string) error {
	dept := ""
	if d != nil {
		dept = *d
	}
	f.mutations = append(f.mutations, "user-dept "+tenant+" "+u+" "+dept)
	return nil
}
func (f *fakeDeptStore) CountMembersByDepartment(_ context.Context, _ string) (int, error) {
	return 0, nil
}
func (f *fakeDeptStore) ReassignDepartmentChildren(_ context.Context, id string, _ *string) error {
	f.mutations = append(f.mutations, "reassign-children "+id)
	return nil
}
func (f *fakeDeptStore) ReassignDepartmentUsers(_ context.Context, id string, _ *string) error {
	f.mutations = append(f.mutations, "reassign-users "+id)
	return nil
}

type fixture struct {
	svc     *domain.Service
	read    *fakePolicyRead
	write   *fakePolicyWrite
	depts   *fakeDeptStore
	wsStore *fakeWSStore
}

func node(id, name, typ string) *policypb.NGACNode {
	return &policypb.NGACNode{Id: id, Name: name, NodeType: typ}
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	grants := map[grant]bool{}
	allow := func(user, obj string, ops ...string) {
		for _, op := range ops {
			grants[grant{user, obj, op}] = true
		}
	}
	allow(owner, mgmt1, ngac.AllOwnerOps()...)
	allow(owner, docs1, ngac.AllOwnerOps()...)
	allow(member, docs1, ngac.MemberDocumentOps()...)
	allow(inviter, mgmt1, ngac.OpInvite)
	allow(manager, mgmt1, ngac.OpManage)
	allow(delegator, mgmt1, ngac.OpManage)
	allow(delegator, docs1, ngac.OpRead)
	allow(otherOwner, mgmt2, ngac.AllOwnerOps()...)

	pc1Children := []*policypb.NGACNode{
		node(owners1, ngac.OwnersUAName(ws1), ngac.TypeUA),
		node(members1, ngac.MembersUAName(ws1), ngac.TypeUA),
		{Id: role1, Name: "Editor", NodeType: ngac.TypeUA, Properties: map[string]string{ngac.PropType: ngac.PropTypeRole}},
		node(mgmt1, ngac.MgmtOAName(ws1), ngac.TypeOA),
		node(docs1, ngac.DocumentsOAName(ws1), ngac.TypeOA),
		node(chat1, ngac.ChannelsOAName(ws1), ngac.TypeOA),
		node(assets1, ngac.AssetsOAName(ws1), ngac.TypeOA),
		node(folder1, "Engineering", ngac.TypeOA),
		{Id: roleRead, Name: ngac.RoleUAName("r-read"), NodeType: ngac.TypeUA, Properties: map[string]string{ngac.PropType: ngac.PropTypeRole, ngac.PropDisplayName: "Người đọc"}},
		{Id: roleEdit, Name: ngac.RoleUAName("r-edit"), NodeType: ngac.TypeUA, Properties: map[string]string{ngac.PropType: ngac.PropTypeRole, ngac.PropDisplayName: "Biên tập"}},
	}
	pc1Desc := append([]*policypb.NGACNode{}, pc1Children...)
	pc1Desc = append(pc1Desc,
		node(owner, "owner", ngac.TypeU),
		node(member, "member", ngac.TypeU),
		node(target, "target", ngac.TypeU),
		node(leaver, "leaver", ngac.TypeU),
	)
	pc2Children := []*policypb.NGACNode{
		node(owners2, ngac.OwnersUAName(ws2), ngac.TypeUA),
		node(mgmt2, ngac.MgmtOAName(ws2), ngac.TypeOA),
		node(folder2, "Finance", ngac.TypeOA),
	}

	inPC1 := []*policypb.NGACNode{node(members1, ngac.MembersUAName(ws1), ngac.TypeUA), node(pc1, ngac.PCName(ws1), ngac.TypePC)}
	inPC2 := []*policypb.NGACNode{node(owners2, ngac.OwnersUAName(ws2), ngac.TypeUA), node(pc2, ngac.PCName(ws2), ngac.TypePC)}

	read := &fakePolicyRead{
		grants:      grants,
		children:    map[string][]*policypb.NGACNode{pc1: pc1Children, pc2: pc2Children, owners1: {node(owner, "owner", ngac.TypeU), node(target, "target", ngac.TypeU)}},
		descendants: map[string][]*policypb.NGACNode{pc1: pc1Desc, pc2: pc2Children},
		ancestors: map[string][]*policypb.NGACNode{
			owner: inPC1, member: inPC1, inviter: inPC1, manager: inPC1, delegator: inPC1,
			target: inPC1, leaver: inPC1,
			otherOwner: inPC2,
		},
		// Every person the screens act on is a user node of ws-1, except the
		// foreign owner, who belongs to ws-2.
		nodes: map[string]*policypb.NGACNode{
			owner: node(owner, "owner", ngac.TypeU), member: node(member, "member", ngac.TypeU),
			target: node(target, "target", ngac.TypeU), leaver: node(leaver, "leaver", ngac.TypeU),
			inviter: node(inviter, "inviter", ngac.TypeU), manager: node(manager, "manager", ngac.TypeU), delegator: node(delegator, "delegator", ngac.TypeU),
			otherOwner: node(otherOwner, "other", ngac.TypeU), outsider: node(outsider, "outsider", ngac.TypeU),
			role1: node(role1, "Editor", ngac.TypeUA), docs1: node(docs1, "docs", ngac.TypeOA),
		},
		parents: map[string][]*policypb.NGACNode{},
		assocs: map[string][]*policypb.Association{
			// What the roles confer: reading, or reading and writing, Documents.
			roleRead: {{Id: "as-1", UaId: roleRead, OaId: docs1, Operations: []string{ngac.OpRead}}},
			roleEdit: {{Id: "as-2", UaId: roleEdit, OaId: docs1, Operations: []string{ngac.OpRead, ngac.OpWrite}}},
		},
	}
	write := &fakePolicyWrite{src: read}
	wsStore := &fakeWSStore{ws: map[string]*store.Workspace{
		ws1: {ID: ws1, Name: "Acme", NGACPcID: pc1},
		ws2: {ID: ws2, Name: "Globex", NGACPcID: pc2},
	}}
	root1 := "dept-root-1"
	depts := &fakeDeptStore{depts: map[string]*store.Department{
		root1:          {ID: root1, WorkspaceID: ws1, Name: "Root", NGACUaID: "ua-dept-root-1"},
		"dept-child-1": {ID: "dept-child-1", WorkspaceID: ws1, Name: "Child", ParentID: &root1, NGACUaID: "ua-dept-child-1"},
		"dept-other-1": {ID: "dept-other-1", WorkspaceID: ws1, Name: "Other", NGACUaID: "ua-dept-other-1"},
		"dept-ws2":     {ID: "dept-ws2", WorkspaceID: ws2, Name: "Foreign", NGACUaID: "ua-dept-ws2"},
	}}
	svc := domain.NewService(wsStore, depts, read, write, nil, nil)
	return &fixture{svc: svc, read: read, write: write, depts: depts, wsStore: wsStore}
}

func (f *fixture) mutated() bool {
	return len(f.write.mutations) > 0 || len(f.depts.mutations) > 0 || len(f.wsStore.mutations) > 0
}

// ---------------------------------------------------------------------------
// Admin operations, table-driven: every guarded call is exercised for DENY and
// ALLOW. The deny subjects are chosen to sit one step short of the grant:
// a plain member, a holder of the *other* admin op, and an owner of a
// different workspace (different PC).
// ---------------------------------------------------------------------------

type adminCase struct {
	name string
	op   string // operation that must be checked on ws-1's Mgmt OA
	call func(ctx context.Context, s *domain.Service, caller string) error
}

func adminCases() []adminCase {
	return []adminCase{
		{"RemoveMember", ngac.OpInvite, func(ctx context.Context, s *domain.Service, c string) error {
			return s.RemoveMember(ctx, c, ws1, leaver)
		}},
		{"CreateRole", ngac.OpManage, func(ctx context.Context, s *domain.Service, c string) error {
			_, err := s.CreateRole(ctx, c, ws1, "Reviewer")
			return err
		}},
		{"DeleteRole", ngac.OpManage, func(ctx context.Context, s *domain.Service, c string) error {
			return s.DeleteRole(ctx, c, ws1, role1)
		}},
		{"CreateFolder", ngac.OpManage, func(ctx context.Context, s *domain.Service, c string) error {
			_, err := s.CreateFolder(ctx, c, ws1, "Legal", "")
			return err
		}},
		{"DeleteFolder", ngac.OpManage, func(ctx context.Context, s *domain.Service, c string) error {
			return s.DeleteFolder(ctx, c, ws1, folder1)
		}},
		{"TransferOwnership", ngac.OpManage, func(ctx context.Context, s *domain.Service, c string) error {
			return s.TransferOwnership(ctx, c, ws1, target)
		}},
		{"RemoveOwner", ngac.OpManage, func(ctx context.Context, s *domain.Service, c string) error {
			return s.RemoveOwner(ctx, c, ws1, target)
		}},
		{"CreateDepartment", ngac.OpManage, func(ctx context.Context, s *domain.Service, c string) error {
			_, err := s.CreateDepartment(ctx, c, domain.CreateDepartmentInput{WorkspaceID: ws1, Name: "Sales"})
			return err
		}},
		{"UpdateDepartment", ngac.OpManage, func(ctx context.Context, s *domain.Service, c string) error {
			_, err := s.UpdateDepartment(ctx, c, ws1, "dept-root-1", "Renamed")
			return err
		}},
		{"DeleteDepartment", ngac.OpManage, func(ctx context.Context, s *domain.Service, c string) error {
			return s.DeleteDepartment(ctx, c, ws1, "dept-child-1")
		}},
		{"MoveDepartment", ngac.OpManage, func(ctx context.Context, s *domain.Service, c string) error {
			_, err := s.MoveDepartment(ctx, c, domain.MoveDepartmentInput{WorkspaceID: ws1, DeptID: "dept-child-1", NewParentID: "dept-other-1"})
			return err
		}},
		{"UpdateMemberDepartment", ngac.OpManage, func(ctx context.Context, s *domain.Service, c string) error {
			return s.UpdateMemberDepartment(ctx, c, ws1, target, "dept-root-1")
		}},
		{"UpdateWorkspaceDetails", ngac.OpManage, func(ctx context.Context, s *domain.Service, c string) error {
			name := "Renamed"
			_, err := s.UpdateWorkspaceDetails(ctx, c, ws1, &name, nil)
			return err
		}},
	}
}

// otherAdminOpHolder returns the subject that holds the admin op this case
// does NOT require — proving the check is on the specific op, not on "is
// some kind of admin".
func otherAdminOpHolder(op string) string {
	if op == ngac.OpInvite {
		return manager
	}
	return inviter
}

func TestAdminOps_DenyWhenCallerLacksOp(t *testing.T) {
	for _, tc := range adminCases() {
		for _, caller := range []string{member, outsider, otherOwner, otherAdminOpHolder(tc.op), ""} {
			t.Run(tc.name+"/"+caller, func(t *testing.T) {
				f := newFixture(t)
				err := tc.call(context.Background(), f.svc, caller)
				require.Error(t, err)
				assert.ErrorIs(t, err, domain.ErrAccessDenied)
				assert.False(t, f.mutated(), "denied call must not touch the graph or the DB: %v %v", f.write.mutations, f.depts.mutations)
			})
		}
	}
}

func TestAdminOps_DenyWhenPolicyCallErrors(t *testing.T) {
	for _, tc := range adminCases() {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			f.read.checkErr = errors.New("policy unavailable")
			err := tc.call(context.Background(), f.svc, owner)
			require.Error(t, err)
			assert.ErrorIs(t, err, domain.ErrAccessDenied, "an unreachable PDP must deny, not allow")
			assert.False(t, f.mutated())
		})
	}
}

func TestAdminOps_AllowWhenCallerHoldsOpOnMgmtOA(t *testing.T) {
	for _, tc := range adminCases() {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			err := tc.call(context.Background(), f.svc, owner)
			require.NoError(t, err)
			assert.True(t, f.mutated(), "allowed call should perform its write")
			require.NotEmpty(t, f.read.checks)
			first := f.read.checks[0]
			assert.Equal(t, owner, first.UserNodeId)
			assert.Equal(t, mgmt1, first.ObjectNodeId, "admin ops are checked on the workspace Mgmt OA")
			assert.Equal(t, tc.op, first.Operation)
		})
	}
}

// The holder of exactly the required op (and nothing else) is enough.
func TestAdminOps_AllowWithExactlyTheRequiredOp(t *testing.T) {
	for _, tc := range adminCases() {
		if tc.name == "TransferOwnership" || tc.name == "RemoveOwner" {
			continue // owner changes need to be an Owner, not only to hold manage; see the owner tests
		}
		holder := inviter
		if tc.op == ngac.OpManage {
			holder = manager
		}
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			require.NoError(t, tc.call(context.Background(), f.svc, holder))
		})
	}
}

// ---------------------------------------------------------------------------
// Cross-tenant scoping: holding manage on ws-1 must not reach into ws-2 by
// passing ws-2's node or department IDs through a ws-1 route.
// ---------------------------------------------------------------------------

func TestScoping_RejectsForeignNodesAndDepartments(t *testing.T) {
	cases := []struct {
		name string
		call func(ctx context.Context, s *domain.Service) error
	}{
		{"DeleteRole foreign UA", func(ctx context.Context, s *domain.Service) error {
			return s.DeleteRole(ctx, owner, ws1, owners2)
		}},
		{"DeleteFolder foreign OA", func(ctx context.Context, s *domain.Service) error {
			return s.DeleteFolder(ctx, owner, ws1, folder2)
		}},
		{"CreateFolder under foreign OA", func(ctx context.Context, s *domain.Service) error {
			_, err := s.CreateFolder(ctx, owner, ws1, "x", folder2)
			return err
		}},
		{"CreateDepartment under foreign parent", func(ctx context.Context, s *domain.Service) error {
			_, err := s.CreateDepartment(ctx, owner, domain.CreateDepartmentInput{WorkspaceID: ws1, Name: "x", ParentID: "dept-ws2"})
			return err
		}},
		{"UpdateDepartment foreign", func(ctx context.Context, s *domain.Service) error {
			_, err := s.UpdateDepartment(ctx, owner, ws1, "dept-ws2", "pwned")
			return err
		}},
		{"DeleteDepartment foreign", func(ctx context.Context, s *domain.Service) error {
			return s.DeleteDepartment(ctx, owner, ws1, "dept-ws2")
		}},
		{"MoveDepartment foreign dept", func(ctx context.Context, s *domain.Service) error {
			_, err := s.MoveDepartment(ctx, owner, domain.MoveDepartmentInput{WorkspaceID: ws1, DeptID: "dept-ws2"})
			return err
		}},
		{"MoveDepartment under foreign parent", func(ctx context.Context, s *domain.Service) error {
			_, err := s.MoveDepartment(ctx, owner, domain.MoveDepartmentInput{WorkspaceID: ws1, DeptID: "dept-child-1", NewParentID: "dept-ws2"})
			return err
		}},
		{"UpdateMemberDepartment foreign dept", func(ctx context.Context, s *domain.Service) error {
			return s.UpdateMemberDepartment(ctx, owner, ws1, target, "dept-ws2")
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			err := tc.call(context.Background(), f.svc)
			require.Error(t, err)
			assert.ErrorIs(t, err, domain.ErrNotFound)
			assert.False(t, f.mutated(), "%v %v", f.write.mutations, f.depts.mutations)
		})
	}
}

// ---------------------------------------------------------------------------
// Read operations: membership (the caller reaches the workspace PC).
// ---------------------------------------------------------------------------

type readCase struct {
	name string
	call func(ctx context.Context, s *domain.Service, caller string) error
}

func readCases() []readCase {
	return []readCase{
		{"ViewWorkspace", func(ctx context.Context, s *domain.Service, c string) error {
			_, err := s.ViewWorkspace(ctx, c, ws1)
			return err
		}},
		{"ListMembers", func(ctx context.Context, s *domain.Service, c string) error {
			_, err := s.ListMembers(ctx, c, ws1)
			return err
		}},
		{"ListRoles", func(ctx context.Context, s *domain.Service, c string) error {
			_, err := s.ListRoles(ctx, c, ws1)
			return err
		}},
		{"ListFolders", func(ctx context.Context, s *domain.Service, c string) error {
			_, err := s.ListFolders(ctx, c, ws1)
			return err
		}},
		{"ListDepartments", func(ctx context.Context, s *domain.Service, c string) error {
			_, err := s.ListDepartments(ctx, c, ws1)
			return err
		}},
	}
}

func TestReads_AllowMembers(t *testing.T) {
	for _, tc := range readCases() {
		for _, caller := range []string{owner, member} {
			t.Run(tc.name+"/"+caller, func(t *testing.T) {
				f := newFixture(t)
				require.NoError(t, tc.call(context.Background(), f.svc, caller))
			})
		}
	}
}

func TestReads_DenyNonMembers(t *testing.T) {
	for _, tc := range readCases() {
		for _, caller := range []string{outsider, otherOwner, ""} {
			t.Run(tc.name+"/"+caller, func(t *testing.T) {
				f := newFixture(t)
				err := tc.call(context.Background(), f.svc, caller)
				require.Error(t, err)
				assert.ErrorIs(t, err, domain.ErrAccessDenied)
			})
		}
	}
}

func TestReads_DenyWhenPolicyCallErrors(t *testing.T) {
	for _, tc := range readCases() {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			f.read.ancErr = errors.New("policy unavailable")
			err := tc.call(context.Background(), f.svc, member)
			require.Error(t, err)
			assert.ErrorIs(t, err, domain.ErrAccessDenied)
		})
	}
}
