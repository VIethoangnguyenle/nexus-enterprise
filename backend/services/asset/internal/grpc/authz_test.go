package grpc_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"ngac-platform/ngac"
	pb "ngac-platform/proto/asset"
	policypb "ngac-platform/proto/policy"
	"ngac-platform/services/asset/internal/caller"
	agrpc "ngac-platform/services/asset/internal/grpc"
	"ngac-platform/services/asset/internal/store"
)

// ---------------------------------------------------------------------------
// Policy fakes
// ---------------------------------------------------------------------------

// fakePolicyRead answers access checks from an explicit grant set, so each test
// pins both the operation and the object a guard checks. Anything not granted
// is denied. Names resolve only if registered, so a test controls whether a
// workspace's Assets OA exists.
type fakePolicyRead struct {
	policypb.PolicyReadServiceClient
	mu      sync.Mutex
	grants  map[[3]string]bool // {user, object, op}
	nodes   map[string]string  // name -> node id
	failErr error              // access checks error
	findErr error              // FindNodeByName errors (other than not-found)
	checks  [][3]string
}

func newFakePolicy() *fakePolicyRead {
	return &fakePolicyRead{grants: map[[3]string]bool{}, nodes: map[string]string{}}
}

func (f *fakePolicyRead) grant(user, object, op string) { f.grants[[3]string{user, object, op}] = true }

func (f *fakePolicyRead) record(user, object, op string) bool {
	key := [3]string{user, object, op}
	f.checks = append(f.checks, key)
	return f.grants[key]
}

func (f *fakePolicyRead) CheckAccess(_ context.Context, req *policypb.CheckAccessRequest, _ ...grpc.CallOption) (*policypb.AccessDecision, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	allowed := f.record(req.UserNodeId, req.ObjectNodeId, req.Operation)
	if f.failErr != nil {
		return nil, f.failErr
	}
	if allowed {
		return &policypb.AccessDecision{Decision: ngac.DecisionAllow}, nil
	}
	return &policypb.AccessDecision{Decision: ngac.DecisionDeny}, nil
}

func (f *fakePolicyRead) BatchCheckAccess(_ context.Context, req *policypb.BatchCheckAccessRequest, _ ...grpc.CallOption) (*policypb.BatchAccessResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failErr != nil {
		return nil, f.failErr
	}
	results := map[string]*policypb.ObjectPermissions{}
	for _, obj := range req.ObjectIds {
		perms := map[string]bool{}
		for _, op := range req.Operations {
			perms[op] = f.record(req.UserNodeId, obj, op)
		}
		results[obj] = &policypb.ObjectPermissions{Permissions: perms}
	}
	return &policypb.BatchAccessResult{Results: results}, nil
}

func (f *fakePolicyRead) FindNodeByName(_ context.Context, req *policypb.FindNodeByNameRequest, _ ...grpc.CallOption) (*policypb.NGACNode, error) {
	if f.findErr != nil {
		return nil, f.findErr
	}
	if id, ok := f.nodes[req.Name]; ok {
		return &policypb.NGACNode{Id: id, Name: req.Name, NodeType: req.NodeType}, nil
	}
	return nil, status.Errorf(codes.NotFound, "node %s not found", req.Name)
}

func (f *fakePolicyRead) GetChildren(_ context.Context, _ *policypb.GetChildrenRequest, _ ...grpc.CallOption) (*policypb.NodeList, error) {
	return &policypb.NodeList{}, nil
}

// fakePolicyWrite records graph writes. CreateNode hands back a real OA node
// ID from the test DB, because asset_types.ngac_oa_id is a foreign key.
type fakePolicyWrite struct {
	policypb.PolicyWriteServiceClient
	mu      sync.Mutex
	nodeID  string
	created []string
}

func (w *fakePolicyWrite) CreateNode(_ context.Context, req *policypb.CreateNodeRequest, _ ...grpc.CallOption) (*policypb.NGACNode, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.created = append(w.created, req.Name)
	return &policypb.NGACNode{Id: w.nodeID, Name: req.Name, NodeType: req.NodeType}, nil
}

func (w *fakePolicyWrite) CreateAssignment(_ context.Context, _ *policypb.CreateAssignmentRequest, _ ...grpc.CallOption) (*policypb.Assignment, error) {
	return &policypb.Assignment{Id: "assign"}, nil
}

func (w *fakePolicyWrite) CreateAssociation(_ context.Context, _ *policypb.CreateAssociationRequest, _ ...grpc.CallOption) (*policypb.Association, error) {
	return &policypb.Association{Id: "assoc"}, nil
}

// ---------------------------------------------------------------------------
// Fixture: an isolated workspace with two asset types on two different OAs,
// one asset of each, and one request against each.
// ---------------------------------------------------------------------------

type fixture struct {
	pool               *pgxpool.Pool
	st                 *store.Store
	wsID               string
	oaA, oaB           string // type OAs
	typeA, typeB       string
	assetA, assetB     string
	userX, userY       string // users.id
	reqByXonA, reqByYB string
}

func testDBURL() string {
	if url := os.Getenv("TEST_DATABASE_URL"); url != "" {
		return url
	}
	return "postgres://ngac:ngac_secret@localhost:5432/ngac?sslmode=disable"
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, testDBURL())
	if err != nil {
		t.Fatalf("connect to test DB: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		t.Skipf("test DB not available: %v", err)
	}
	t.Cleanup(pool.Close)

	rows, err := pool.Query(ctx, "SELECT id FROM ngac_nodes WHERE node_type = 'OA' ORDER BY id LIMIT 2")
	require.NoError(t, err)
	var oas []string
	for rows.Next() {
		var id string
		require.NoError(t, rows.Scan(&id))
		oas = append(oas, id)
	}
	rows.Close()
	if len(oas) < 2 {
		t.Skip("need two OA nodes in the test DB")
	}

	sfx := uuid.New().String()[:8]
	f := &fixture{pool: pool, st: store.New(pool), oaA: oas[0], oaB: oas[1]}
	f.userX = "authz-x-" + sfx
	f.userY = "authz-y-" + sfx
	f.wsID = "authz-ws-" + sfx
	f.typeA = "authz-ta-" + sfx
	f.typeB = "authz-tb-" + sfx
	f.assetA = "authz-aa-" + sfx
	f.assetB = "authz-ab-" + sfx
	f.reqByXonA = "authz-ra-" + sfx
	f.reqByYB = "authz-rb-" + sfx

	exec := func(q string, args ...any) {
		_, err := pool.Exec(ctx, q, args...)
		require.NoError(t, err, q)
	}
	exec(`INSERT INTO users (id, username, password) VALUES ($1, $1, ''), ($2, $2, '')`, f.userX, f.userY)
	exec(`INSERT INTO workspaces (id, name, owner_id) VALUES ($1, $1, $2)`, f.wsID, f.userX)
	exec(`INSERT INTO asset_types (id, name, category, workspace_id, ngac_oa_id) VALUES
	      ($1, $1, 'hardware', $3, $4), ($2, $2, 'hardware', $3, $5)`, f.typeA, f.typeB, f.wsID, f.oaA, f.oaB)
	exec(`INSERT INTO assets (id, name, type_id, workspace_id, created_by) VALUES
	      ($1, $1, $3, $5, $6), ($2, $2, $4, $5, $6)`, f.assetA, f.assetB, f.typeA, f.typeB, f.wsID, f.userX)
	exec(`INSERT INTO asset_requests (id, type_id, workspace_id, requester_id, justification) VALUES
	      ($1, $3, $5, $6, 'x'), ($2, $4, $5, $7, 'y')`, f.reqByXonA, f.reqByYB, f.typeA, f.typeB, f.wsID, f.userX, f.userY)

	t.Cleanup(func() {
		c := context.Background()
		pool.Exec(c, `DELETE FROM asset_requests WHERE workspace_id = $1`, f.wsID)
		pool.Exec(c, `DELETE FROM assets WHERE workspace_id = $1`, f.wsID)
		pool.Exec(c, `DELETE FROM asset_types WHERE workspace_id = $1`, f.wsID)
		pool.Exec(c, `DELETE FROM workspaces WHERE id = $1`, f.wsID)
		pool.Exec(c, `DELETE FROM users WHERE id IN ($1, $2)`, f.userX, f.userY)
	})
	return f
}

func (f *fixture) assetsOA() string { return "assets-oa:" + f.wsID }
func (f *fixture) mgmtOA() string   { return "mgmt-oa:" + f.wsID }

// policy returns a fake in which this workspace's Assets OA and Mgmt OA exist.
func (f *fixture) policy() *fakePolicyRead {
	p := newFakePolicy()
	p.nodes[ngac.AssetsOAName(f.wsID)] = f.assetsOA()
	p.nodes[ngac.MgmtOAName(f.wsID)] = f.mgmtOA()
	return p
}

func asCaller(userID, nodeID string) context.Context {
	return caller.WithIdentity(context.Background(), caller.Identity{UserID: userID, NGACNodeID: nodeID})
}

var errPolicyDown = errors.New("policy unavailable")

// ---------------------------------------------------------------------------
// ListAssets — filtered by read on each asset's type OA.
// ---------------------------------------------------------------------------

func assetIDs(list *pb.AssetList) []string {
	var ids []string
	for _, a := range list.GetAssets() {
		ids = append(ids, a.Id)
	}
	return ids
}

func TestListAssets_DeniedSeesNothing(t *testing.T) {
	f := newFixture(t)
	p := f.policy()
	// Write and manage on a type OA do not confer read for listing purposes
	// unless read is also held; here only non-read ops are granted.
	p.grant("n-member", f.oaA, ngac.OpWrite)
	srv := agrpc.NewAssetServer(f.st, p, &fakePolicyWrite{}, nil)

	list, err := srv.ListAssets(context.Background(), &pb.ListAssetsRequest{
		WorkspaceId: f.wsID, UserNgacNodeId: "n-member",
	})

	require.NoError(t, err)
	assert.Empty(t, list.Assets)
	assert.Zero(t, list.Total, "total must not count assets the caller cannot read")
}

func TestListAssets_SeesOnlyPermittedSubset(t *testing.T) {
	f := newFixture(t)
	p := f.policy()
	p.grant("n-reader", f.oaA, ngac.OpRead)
	srv := agrpc.NewAssetServer(f.st, p, &fakePolicyWrite{}, nil)

	list, err := srv.ListAssets(context.Background(), &pb.ListAssetsRequest{
		WorkspaceId: f.wsID, UserNgacNodeId: "n-reader",
	})

	require.NoError(t, err)
	assert.Equal(t, []string{f.assetA}, assetIDs(list))
	assert.Equal(t, int32(1), list.Total)
}

func TestListAssets_TypeFilterOnUnreadableTypeIsEmpty(t *testing.T) {
	f := newFixture(t)
	p := f.policy()
	p.grant("n-reader", f.oaA, ngac.OpRead)
	srv := agrpc.NewAssetServer(f.st, p, &fakePolicyWrite{}, nil)

	list, err := srv.ListAssets(context.Background(), &pb.ListAssetsRequest{
		WorkspaceId: f.wsID, UserNgacNodeId: "n-reader", TypeId: f.typeB,
	})

	require.NoError(t, err)
	assert.Empty(t, list.Assets)
	assert.Zero(t, list.Total)
}

func TestListAssets_AllowedSeesAll(t *testing.T) {
	f := newFixture(t)
	p := f.policy()
	p.grant("n-owner", f.oaA, ngac.OpRead)
	p.grant("n-owner", f.oaB, ngac.OpRead)
	srv := agrpc.NewAssetServer(f.st, p, &fakePolicyWrite{}, nil)

	list, err := srv.ListAssets(context.Background(), &pb.ListAssetsRequest{
		WorkspaceId: f.wsID, UserNgacNodeId: "n-owner",
	})

	require.NoError(t, err)
	assert.ElementsMatch(t, []string{f.assetA, f.assetB}, assetIDs(list))
	assert.Equal(t, int32(2), list.Total)
}

func TestListAssets_PolicyErrorFailsClosed(t *testing.T) {
	f := newFixture(t)
	p := f.policy()
	p.grant("n-owner", f.oaA, ngac.OpRead)
	p.failErr = errPolicyDown
	srv := agrpc.NewAssetServer(f.st, p, &fakePolicyWrite{}, nil)

	list, err := srv.ListAssets(context.Background(), &pb.ListAssetsRequest{
		WorkspaceId: f.wsID, UserNgacNodeId: "n-owner",
	})

	require.Error(t, err)
	assert.Empty(t, list.GetAssets())
}

// ---------------------------------------------------------------------------
// ListRequests / GetRequest — requester, or approve on the request's type OA.
// ---------------------------------------------------------------------------

func requestIDs(list *pb.AssetRequestList) []string {
	var ids []string
	for _, r := range list.GetRequests() {
		ids = append(ids, r.Id)
	}
	return ids
}

func TestListRequests_UnrelatedCallerSeesNothing(t *testing.T) {
	f := newFixture(t)
	p := f.policy()
	// Read on the type OA (and on the Assets OA) is not approval authority.
	p.grant("n-other", f.oaA, ngac.OpRead)
	p.grant("n-other", f.assetsOA(), ngac.OpRead)
	srv := agrpc.NewAssetRequestServer(f.st, p, &fakePolicyWrite{}, nil)

	list, err := srv.ListRequests(context.Background(), &pb.ListRequestsReq{
		WorkspaceId: f.wsID, UserId: "someone-else", UserNgacNodeId: "n-other",
	})

	require.NoError(t, err)
	assert.Empty(t, list.Requests)
	assert.Zero(t, list.Total)
}

func TestListRequests_RequesterSeesOnlyOwn(t *testing.T) {
	f := newFixture(t)
	p := f.policy()
	srv := agrpc.NewAssetRequestServer(f.st, p, &fakePolicyWrite{}, nil)

	list, err := srv.ListRequests(context.Background(), &pb.ListRequestsReq{
		WorkspaceId: f.wsID, UserId: f.userX, UserNgacNodeId: "n-x",
	})

	require.NoError(t, err)
	assert.Equal(t, []string{f.reqByXonA}, requestIDs(list))
	assert.Equal(t, int32(1), list.Total)
}

func TestListRequests_ApproverSeesRequestsForApprovableTypes(t *testing.T) {
	f := newFixture(t)
	p := f.policy()
	p.grant("n-approver-b", f.oaB, ngac.OpApprove)
	srv := agrpc.NewAssetRequestServer(f.st, p, &fakePolicyWrite{}, nil)

	list, err := srv.ListRequests(context.Background(), &pb.ListRequestsReq{
		WorkspaceId: f.wsID, UserId: "approver-b", UserNgacNodeId: "n-approver-b",
	})

	require.NoError(t, err)
	assert.Equal(t, []string{f.reqByYB}, requestIDs(list))
	assert.Equal(t, int32(1), list.Total)
}

func TestListRequests_PolicyErrorFailsClosed(t *testing.T) {
	f := newFixture(t)
	p := f.policy()
	p.grant("n-approver-b", f.oaB, ngac.OpApprove)
	p.failErr = errPolicyDown
	srv := agrpc.NewAssetRequestServer(f.st, p, &fakePolicyWrite{}, nil)

	list, err := srv.ListRequests(context.Background(), &pb.ListRequestsReq{
		WorkspaceId: f.wsID, UserId: "approver-b", UserNgacNodeId: "n-approver-b",
	})

	require.Error(t, err)
	assert.Empty(t, list.GetRequests())
}

func TestGetRequest_DeniedForUnrelatedCaller(t *testing.T) {
	f := newFixture(t)
	p := f.policy()
	p.grant("n-other", f.oaA, ngac.OpRead)
	p.grant("n-other", f.assetsOA(), ngac.OpRead)
	p.grant("n-other", f.oaB, ngac.OpApprove) // approval on a different type
	srv := agrpc.NewAssetRequestServer(f.st, p, &fakePolicyWrite{}, nil)

	_, err := srv.GetRequest(asCaller("someone-else", "n-other"), &pb.GetRequestReq{RequestId: f.reqByXonA})

	assert.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestGetRequest_DeniedWithoutCaller(t *testing.T) {
	f := newFixture(t)
	srv := agrpc.NewAssetRequestServer(f.st, f.policy(), &fakePolicyWrite{}, nil)

	_, err := srv.GetRequest(context.Background(), &pb.GetRequestReq{RequestId: f.reqByXonA})

	assert.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestGetRequest_DeniedWhenPolicyErrors(t *testing.T) {
	f := newFixture(t)
	p := f.policy()
	p.grant("n-approver-a", f.oaA, ngac.OpApprove)
	p.failErr = errPolicyDown
	srv := agrpc.NewAssetRequestServer(f.st, p, &fakePolicyWrite{}, nil)

	_, err := srv.GetRequest(asCaller("approver-a", "n-approver-a"), &pb.GetRequestReq{RequestId: f.reqByXonA})

	assert.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestGetRequest_AllowedForRequester(t *testing.T) {
	f := newFixture(t)
	srv := agrpc.NewAssetRequestServer(f.st, f.policy(), &fakePolicyWrite{}, nil)

	r, err := srv.GetRequest(asCaller(f.userX, "n-x"), &pb.GetRequestReq{RequestId: f.reqByXonA})

	require.NoError(t, err)
	assert.Equal(t, f.reqByXonA, r.Id)
}

func TestGetRequest_AllowedForApproverOfType(t *testing.T) {
	f := newFixture(t)
	p := f.policy()
	p.grant("n-approver-a", f.oaA, ngac.OpApprove)
	srv := agrpc.NewAssetRequestServer(f.st, p, &fakePolicyWrite{}, nil)

	r, err := srv.GetRequest(asCaller("approver-a", "n-approver-a"), &pb.GetRequestReq{RequestId: f.reqByXonA})

	require.NoError(t, err)
	assert.Equal(t, f.reqByXonA, r.Id)
}

func TestGetRequest_NotFound(t *testing.T) {
	f := newFixture(t)
	srv := agrpc.NewAssetRequestServer(f.st, f.policy(), &fakePolicyWrite{}, nil)

	_, err := srv.GetRequest(asCaller(f.userX, "n-x"), &pb.GetRequestReq{RequestId: "does-not-exist"})

	assert.Equal(t, codes.NotFound, status.Code(err))
}

// ---------------------------------------------------------------------------
// Asset types — manage on the Assets OA to mutate, read on it to read.
// ---------------------------------------------------------------------------

func typeCount(t *testing.T, f *fixture) int {
	t.Helper()
	var n int
	require.NoError(t, f.pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM asset_types WHERE workspace_id = $1`, f.wsID).Scan(&n))
	return n
}

func createTypeReq(f *fixture, node string) *pb.CreateTypeRequest {
	return &pb.CreateTypeRequest{
		Name: fmt.Sprintf("NewType%d", time.Now().UnixNano()), Category: "hardware",
		WorkspaceId: f.wsID, UserNgacNodeId: node,
	}
}

func TestCreateType_DeniedWithoutManageOnAssetsOA(t *testing.T) {
	f := newFixture(t)
	p := f.policy()
	p.grant("n-member", f.assetsOA(), ngac.OpRead)
	p.grant("n-member", f.assetsOA(), ngac.OpWrite)
	// Manage on Mgmt does not count once the Assets OA exists.
	p.grant("n-member", f.mgmtOA(), ngac.OpManage)
	w := &fakePolicyWrite{nodeID: f.oaA}
	srv := agrpc.NewAssetTypeServer(f.st, p, w)
	before := typeCount(t, f)

	_, err := srv.CreateType(context.Background(), createTypeReq(f, "n-member"))

	assert.Equal(t, codes.PermissionDenied, status.Code(err))
	assert.Empty(t, w.created, "a denied create must not write to the graph")
	assert.Equal(t, before, typeCount(t, f))
}

func TestCreateType_DeniedWhenPolicyErrors(t *testing.T) {
	f := newFixture(t)
	p := f.policy()
	p.grant("n-owner", f.assetsOA(), ngac.OpManage)
	p.failErr = errPolicyDown
	w := &fakePolicyWrite{nodeID: f.oaA}
	srv := agrpc.NewAssetTypeServer(f.st, p, w)
	before := typeCount(t, f)

	_, err := srv.CreateType(context.Background(), createTypeReq(f, "n-owner"))

	assert.Equal(t, codes.PermissionDenied, status.Code(err))
	assert.Empty(t, w.created)
	assert.Equal(t, before, typeCount(t, f))
}

func TestCreateType_DeniedWhenAssetsOALookupErrors(t *testing.T) {
	f := newFixture(t)
	p := f.policy()
	p.grant("n-owner", f.mgmtOA(), ngac.OpManage)
	p.findErr = status.Error(codes.Unavailable, "policy unavailable")
	w := &fakePolicyWrite{nodeID: f.oaA}
	srv := agrpc.NewAssetTypeServer(f.st, p, w)

	_, err := srv.CreateType(context.Background(), createTypeReq(f, "n-owner"))

	assert.Equal(t, codes.PermissionDenied, status.Code(err))
	assert.Empty(t, w.created)
}

func TestCreateType_AllowedWithManageOnAssetsOA(t *testing.T) {
	f := newFixture(t)
	p := f.policy()
	p.grant("n-owner", f.assetsOA(), ngac.OpManage)
	w := &fakePolicyWrite{nodeID: f.oaA}
	srv := agrpc.NewAssetTypeServer(f.st, p, w)
	before := typeCount(t, f)

	at, err := srv.CreateType(context.Background(), createTypeReq(f, "n-owner"))

	require.NoError(t, err)
	assert.Contains(t, p.checks, [3]string{"n-owner", f.assetsOA(), ngac.OpManage})
	assert.Equal(t, f.wsID, at.WorkspaceId)
	assert.Equal(t, before+1, typeCount(t, f))
}

// The first type in a workspace creates the Assets OA, so there is nothing to
// check manage on yet. Bootstrapping falls back to manage on the workspace's
// Mgmt OA — held by workspace owners, the same UA the new Assets OA is then
// granted to.
func TestCreateType_FirstTypeDeniedWithoutManageOnMgmtOA(t *testing.T) {
	f := newFixture(t)
	p := f.policy()
	delete(p.nodes, ngac.AssetsOAName(f.wsID))
	p.grant("n-member", f.mgmtOA(), ngac.OpRead)
	w := &fakePolicyWrite{nodeID: f.oaA}
	srv := agrpc.NewAssetTypeServer(f.st, p, w)
	before := typeCount(t, f)

	_, err := srv.CreateType(context.Background(), createTypeReq(f, "n-member"))

	assert.Equal(t, codes.PermissionDenied, status.Code(err))
	assert.Empty(t, w.created)
	assert.Equal(t, before, typeCount(t, f))
}

func TestCreateType_FirstTypeAllowedWithManageOnMgmtOA(t *testing.T) {
	f := newFixture(t)
	p := f.policy()
	delete(p.nodes, ngac.AssetsOAName(f.wsID))
	p.grant("n-owner", f.mgmtOA(), ngac.OpManage)
	w := &fakePolicyWrite{nodeID: f.oaA}
	srv := agrpc.NewAssetTypeServer(f.st, p, w)
	before := typeCount(t, f)

	_, err := srv.CreateType(context.Background(), createTypeReq(f, "n-owner"))

	require.NoError(t, err)
	assert.Contains(t, p.checks, [3]string{"n-owner", f.mgmtOA(), ngac.OpManage})
	assert.Equal(t, before+1, typeCount(t, f))
}

func typeSchema(t *testing.T, f *fixture, typeID string) string {
	t.Helper()
	var s string
	require.NoError(t, f.pool.QueryRow(context.Background(),
		`SELECT fields_schema::text FROM asset_types WHERE id = $1`, typeID).Scan(&s))
	return s
}

const newSchema = `{"type": "object", "properties": {"serial": {"type": "string"}}}`

func TestUpdateTypeSchema_DeniedWithoutManageOnAssetsOA(t *testing.T) {
	f := newFixture(t)
	p := f.policy()
	p.grant("n-member", f.assetsOA(), ngac.OpWrite)
	p.grant("n-member", f.oaA, ngac.OpManage) // the type's own OA is not the gate
	srv := agrpc.NewAssetTypeServer(f.st, p, &fakePolicyWrite{})
	before := typeSchema(t, f, f.typeA)

	_, err := srv.UpdateTypeSchema(context.Background(), &pb.UpdateTypeSchemaRequest{
		TypeId: f.typeA, UserNgacNodeId: "n-member", FieldsSchema: newSchema,
	})

	assert.Equal(t, codes.PermissionDenied, status.Code(err))
	assert.Equal(t, before, typeSchema(t, f, f.typeA))
}

func TestUpdateTypeSchema_DeniedWhenPolicyErrors(t *testing.T) {
	f := newFixture(t)
	p := f.policy()
	p.grant("n-owner", f.assetsOA(), ngac.OpManage)
	p.failErr = errPolicyDown
	srv := agrpc.NewAssetTypeServer(f.st, p, &fakePolicyWrite{})
	before := typeSchema(t, f, f.typeA)

	_, err := srv.UpdateTypeSchema(context.Background(), &pb.UpdateTypeSchemaRequest{
		TypeId: f.typeA, UserNgacNodeId: "n-owner", FieldsSchema: newSchema,
	})

	assert.Equal(t, codes.PermissionDenied, status.Code(err))
	assert.Equal(t, before, typeSchema(t, f, f.typeA))
}

func TestUpdateTypeSchema_AllowedWithManageOnAssetsOA(t *testing.T) {
	f := newFixture(t)
	p := f.policy()
	p.grant("n-owner", f.assetsOA(), ngac.OpManage)
	srv := agrpc.NewAssetTypeServer(f.st, p, &fakePolicyWrite{})

	at, err := srv.UpdateTypeSchema(context.Background(), &pb.UpdateTypeSchemaRequest{
		TypeId: f.typeA, UserNgacNodeId: "n-owner", FieldsSchema: newSchema,
	})

	require.NoError(t, err)
	assert.Equal(t, f.typeA, at.Id)
	assert.JSONEq(t, newSchema, typeSchema(t, f, f.typeA))
}

func TestGetType_DeniedWithoutReadOnAssetsOA(t *testing.T) {
	f := newFixture(t)
	p := f.policy()
	p.grant("n-member", f.oaA, ngac.OpRead) // the gate is the Assets OA
	srv := agrpc.NewAssetTypeServer(f.st, p, &fakePolicyWrite{})

	_, err := srv.GetType(asCaller("member", "n-member"), &pb.GetTypeRequest{TypeId: f.typeA})

	assert.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestGetType_DeniedWithoutCaller(t *testing.T) {
	f := newFixture(t)
	p := f.policy()
	p.grant("", f.assetsOA(), ngac.OpRead)
	srv := agrpc.NewAssetTypeServer(f.st, p, &fakePolicyWrite{})

	_, err := srv.GetType(context.Background(), &pb.GetTypeRequest{TypeId: f.typeA})

	assert.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestGetType_AllowedWithReadOnAssetsOA(t *testing.T) {
	f := newFixture(t)
	p := f.policy()
	p.grant("n-owner", f.assetsOA(), ngac.OpRead)
	srv := agrpc.NewAssetTypeServer(f.st, p, &fakePolicyWrite{})

	at, err := srv.GetType(asCaller("owner", "n-owner"), &pb.GetTypeRequest{TypeId: f.typeA})

	require.NoError(t, err)
	assert.Equal(t, f.typeA, at.Id)
}

func TestListTypes_DeniedWithoutReadOnAssetsOA(t *testing.T) {
	f := newFixture(t)
	p := f.policy()
	p.grant("n-member", f.oaA, ngac.OpRead)
	srv := agrpc.NewAssetTypeServer(f.st, p, &fakePolicyWrite{})

	list, err := srv.ListTypes(asCaller("member", "n-member"), &pb.ListTypesRequest{WorkspaceId: f.wsID})

	assert.Equal(t, codes.PermissionDenied, status.Code(err))
	assert.Empty(t, list.GetTypes())
}

func TestListTypes_DeniedWithoutCaller(t *testing.T) {
	f := newFixture(t)
	srv := agrpc.NewAssetTypeServer(f.st, f.policy(), &fakePolicyWrite{})

	list, err := srv.ListTypes(context.Background(), &pb.ListTypesRequest{WorkspaceId: f.wsID})

	assert.Equal(t, codes.PermissionDenied, status.Code(err))
	assert.Empty(t, list.GetTypes())
}

func TestListTypes_DeniedWhenPolicyErrors(t *testing.T) {
	f := newFixture(t)
	p := f.policy()
	p.grant("n-owner", f.assetsOA(), ngac.OpRead)
	p.failErr = errPolicyDown
	srv := agrpc.NewAssetTypeServer(f.st, p, &fakePolicyWrite{})

	list, err := srv.ListTypes(asCaller("owner", "n-owner"), &pb.ListTypesRequest{WorkspaceId: f.wsID})

	assert.Equal(t, codes.PermissionDenied, status.Code(err))
	assert.Empty(t, list.GetTypes())
}

func TestListTypes_AllowedWithReadOnAssetsOA(t *testing.T) {
	f := newFixture(t)
	p := f.policy()
	p.grant("n-owner", f.assetsOA(), ngac.OpRead)
	srv := agrpc.NewAssetTypeServer(f.st, p, &fakePolicyWrite{})

	list, err := srv.ListTypes(asCaller("owner", "n-owner"), &pb.ListTypesRequest{WorkspaceId: f.wsID})

	require.NoError(t, err)
	var ids []string
	for _, at := range list.Types {
		ids = append(ids, at.Id)
	}
	assert.ElementsMatch(t, []string{f.typeA, f.typeB}, ids)
}

// A workspace with no Assets OA (the state before its first type is created)
// lists nothing rather than refusing. Even if rows exist — as in this fixture —
// nothing can authorize them, so none are returned.
func TestListTypes_NoAssetsOAIsEmpty(t *testing.T) {
	f := newFixture(t)
	p := f.policy()
	delete(p.nodes, ngac.AssetsOAName(f.wsID))
	srv := agrpc.NewAssetTypeServer(f.st, p, &fakePolicyWrite{})

	list, err := srv.ListTypes(asCaller("owner", "n-owner"), &pb.ListTypesRequest{WorkspaceId: f.wsID})

	require.NoError(t, err)
	assert.Empty(t, list.Types)
}
