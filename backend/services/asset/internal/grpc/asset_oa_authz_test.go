package grpc_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"ngac-platform/ngac"
	pb "ngac-platform/proto/asset"
	agrpc "ngac-platform/services/asset/internal/grpc"
)

// The graph holds attributes, not objects, so an asset is authorized on the OA
// of its type: every asset of a type hangs under that one OA, and a grant on
// the Assets or a category OA reaches it through the OA tree. These tests pin
// both sides of that: the right grant on the asset's own type OA allows, and a
// grant that is one step off — another type's OA, another operation, nothing at
// all — denies.

func (f *fixture) srv(p *fakePolicyRead) *agrpc.AssetServer {
	return agrpc.NewAssetServer(f.st, p, nil)
}

// objectNodes counts the O nodes the graph holds for this fixture's workspace.
// An asset is never a node, so the answer is 0 whatever is done to assets.
func (f *fixture) objectNodes(t *testing.T) int {
	t.Helper()
	var n int
	require.NoError(t, f.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM ngac_nodes WHERE node_type = 'O' AND properties->>'workspace_id' = $1`, f.wsID).Scan(&n))
	return n
}

func asserted(t *testing.T, err error, want codes.Code) {
	t.Helper()
	assert.Equal(t, want, status.Code(err), "err = %v", err)
}

func TestGetAsset_AllowedWithReadOnItsTypeOA(t *testing.T) {
	f := newFixture(t)
	p := f.policy()
	p.grant("n-reader", f.oaA, ngac.OpRead)

	a, err := f.srv(p).GetAsset(asCaller("", "n-reader"), &pb.GetAssetRequest{AssetId: f.assetA})

	require.NoError(t, err)
	assert.Equal(t, f.assetA, a.Id)
	assert.Contains(t, p.checks, [3]string{"n-reader", f.oaA, ngac.OpRead}, "the check lands on the type OA")
}

func TestGetAsset_DeniedOffTheTypeOA(t *testing.T) {
	cases := map[string]func(p *fakePolicyRead, f *fixture){
		"read on another type's OA":   func(p *fakePolicyRead, f *fixture) { p.grant("n-x", f.oaB, ngac.OpRead) },
		"write, not read, on its OA":  func(p *fakePolicyRead, f *fixture) { p.grant("n-x", f.oaA, ngac.OpWrite) },
		"manage, not read, on its OA": func(p *fakePolicyRead, f *fixture) { p.grant("n-x", f.oaA, ngac.OpManage) },
		"nothing":                     func(*fakePolicyRead, *fixture) {},
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			p := f.policy()
			setup(p, f)

			_, err := f.srv(p).GetAsset(asCaller("", "n-x"), &pb.GetAssetRequest{AssetId: f.assetA})

			asserted(t, err, codes.PermissionDenied)
		})
	}
}

func TestGetAsset_DeniedWithoutCallerOrWhenPolicyFails(t *testing.T) {
	f := newFixture(t)
	p := f.policy()
	p.grant("n-reader", f.oaA, ngac.OpRead)
	srv := f.srv(p)

	_, err := srv.GetAsset(context.Background(), &pb.GetAssetRequest{AssetId: f.assetA})
	asserted(t, err, codes.PermissionDenied)

	p.failErr = errPolicyDown
	_, err = srv.GetAsset(asCaller("", "n-reader"), &pb.GetAssetRequest{AssetId: f.assetA})
	asserted(t, err, codes.PermissionDenied)
}

// A type recorded without an OA authorizes nothing, whatever else is granted.
func TestGetAsset_DeniedWhenItsTypeHasNoOA(t *testing.T) {
	f := newFixture(t)
	p := f.policy()
	_, err := f.pool.Exec(context.Background(), `UPDATE asset_types SET ngac_oa_id = NULL WHERE id = $1`, f.typeA)
	require.NoError(t, err)
	for _, oa := range []string{f.oaA, f.oaB, f.assetsOA(), ""} {
		p.grant("n-x", oa, ngac.OpRead)
	}

	_, err = f.srv(p).GetAsset(asCaller("", "n-x"), &pb.GetAssetRequest{AssetId: f.assetA})

	asserted(t, err, codes.PermissionDenied)
}

func TestUpdateAsset_WriteOnTheTypeOAAllowsAndReadAloneDenies(t *testing.T) {
	f := newFixture(t)
	p := f.policy()
	p.grant("n-writer", f.oaA, ngac.OpWrite)
	p.grant("n-reader", f.oaA, ngac.OpRead)
	srv := f.srv(p)

	_, err := srv.UpdateAsset(asCaller("", "n-reader"), &pb.UpdateAssetRequest{AssetId: f.assetA, Name: "renamed"})
	asserted(t, err, codes.PermissionDenied)
	_, err = srv.UpdateAsset(asCaller("", "n-writer"), &pb.UpdateAssetRequest{AssetId: f.assetB, Name: "renamed"})
	asserted(t, err, codes.PermissionDenied) // write on A's OA is not write on B's

	a, err := srv.UpdateAsset(asCaller("", "n-writer"), &pb.UpdateAssetRequest{AssetId: f.assetA, Name: "renamed"})
	require.NoError(t, err)
	assert.Equal(t, "renamed", a.Name)
}

func TestDeleteAsset_ManageOnTheTypeOA(t *testing.T) {
	f := newFixture(t)
	p := f.policy()
	p.grant("n-writer", f.oaA, ngac.OpWrite)
	p.grant("n-manager", f.oaA, ngac.OpManage)
	srv := f.srv(p)

	_, err := srv.DeleteAsset(asCaller("", "n-writer"), &pb.DeleteAssetRequest{AssetId: f.assetA})
	asserted(t, err, codes.PermissionDenied)

	_, err = srv.DeleteAsset(asCaller("", "n-manager"), &pb.DeleteAssetRequest{AssetId: f.assetA})
	require.NoError(t, err)
	assert.Zero(t, f.objectNodes(t), "an asset has no node to leave behind")
	var deleted bool
	require.NoError(t, f.pool.QueryRow(context.Background(), `SELECT deleted FROM assets WHERE id = $1`, f.assetA).Scan(&deleted))
	assert.True(t, deleted)
}

func TestGetAssetHistory_ReadOnTheTypeOA(t *testing.T) {
	f := newFixture(t)
	p := f.policy()
	p.grant("n-reader", f.oaA, ngac.OpRead)
	p.grant("n-other", f.oaB, ngac.OpRead)
	srv := f.srv(p)

	_, err := srv.GetAssetHistory(asCaller("", "n-other"), &pb.GetHistoryRequest{AssetId: f.assetA})
	asserted(t, err, codes.PermissionDenied)

	_, err = srv.GetAssetHistory(asCaller("", "n-reader"), &pb.GetHistoryRequest{AssetId: f.assetA})
	require.NoError(t, err)
}

func TestTransitionAsset_NeedsTheTransitionsOperationOnTheTypeOA(t *testing.T) {
	f := newFixture(t)
	f.withLifecycleOnA(t)
	p := f.policy()
	p.grant("n-manager", f.oaA, ngac.OpManage) // the transition from "requested" needs approve
	p.grant("n-approver-b", f.oaB, ngac.OpApprove)
	srv := f.srv(p)

	for _, caller := range []string{"n-manager", "n-approver-b"} {
		_, err := srv.TransitionAsset(asCaller(f.userY, caller), &pb.TransitionRequest{AssetId: f.assetA, Action: "approve"})
		asserted(t, err, codes.PermissionDenied)
	}
}

func TestGetAvailableTransitions_AreFilteredByWhatTheTypeOAGrants(t *testing.T) {
	f := newFixture(t)
	f.withLifecycleOnA(t)
	p := f.policy()
	p.grant("n-approver", f.oaA, ngac.OpApprove)
	p.grant("n-approver", f.oaA, ngac.OpRead)
	p.grant("n-reader", f.oaA, ngac.OpRead)
	p.grant("n-elsewhere", f.oaB, ngac.OpApprove)
	p.grant("n-elsewhere", f.oaB, ngac.OpRead)
	srv := f.srv(p)

	list, err := srv.GetAvailableTransitions(asCaller("", "n-approver"), &pb.GetTransitionsRequest{AssetId: f.assetA})
	require.NoError(t, err)
	require.Len(t, list.Transitions, 1)
	assert.Equal(t, "approve", list.Transitions[0].Action)

	list, err = srv.GetAvailableTransitions(asCaller("", "n-reader"), &pb.GetTransitionsRequest{AssetId: f.assetA})
	require.NoError(t, err)
	assert.Empty(t, list.Transitions, "read alone offers no step")

	// Reading another type's assets does not reveal this asset's state.
	_, err = srv.GetAvailableTransitions(asCaller("", "n-elsewhere"), &pb.GetTransitionsRequest{AssetId: f.assetA})
	asserted(t, err, codes.PermissionDenied)
}

func TestCreateAsset_WriteOnTheTypeOA(t *testing.T) {
	f := newFixture(t)
	f.withLifecycleOnA(t)
	p := f.policy()
	p.grant("n-writer", f.oaA, ngac.OpWrite)
	p.grant("n-elsewhere", f.oaB, ngac.OpWrite)
	srv := f.srv(p)
	req := func() *pb.CreateAssetRequest {
		return &pb.CreateAssetRequest{Name: "Laptop 7", TypeId: f.typeA, WorkspaceId: f.wsID}
	}

	_, err := srv.CreateAsset(asCaller(f.userX, "n-elsewhere"), req())
	asserted(t, err, codes.PermissionDenied)
	var n int
	require.NoError(t, f.pool.QueryRow(context.Background(), `SELECT count(*) FROM assets WHERE name = 'Laptop 7'`).Scan(&n))
	assert.Zero(t, n, "a denied create writes nothing")

	a, err := srv.CreateAsset(asCaller(f.userX, "n-writer"), req())
	require.NoError(t, err)
	t.Cleanup(func() {
		f.pool.Exec(context.Background(), `DELETE FROM asset_transitions WHERE asset_id = $1`, a.Id)
		f.pool.Exec(context.Background(), `DELETE FROM assets WHERE id = $1`, a.Id)
	})
	assert.Zero(t, f.objectNodes(t), "no node is created for an asset: the graph holds attributes, not objects")
	assert.Equal(t, f.oaA, a.NgacNodeId, "the asset reports the OA it is authorized on")

	// And the asset it created is authorized through its type, like any other.
	p.grant("n-reader", f.oaA, ngac.OpRead)
	_, err = srv.GetAsset(asCaller("", "n-reader"), &pb.GetAssetRequest{AssetId: a.Id})
	require.NoError(t, err)
}

func TestAssignAndReturnAsset_ManageOnTheTypeOA(t *testing.T) {
	f := newFixture(t)
	p := f.policy()
	p.grant("n-manager", f.oaA, ngac.OpManage)
	p.grant("n-elsewhere", f.oaB, ngac.OpManage)
	srv := agrpc.NewAssetRequestServer(f.st, p, &fakePolicyWrite{}, nil)
	_, err := f.pool.Exec(context.Background(), `UPDATE assets SET state = 'assigned' WHERE id = $1`, f.assetA)
	require.NoError(t, err)

	_, err = srv.ReturnAsset(asCaller(f.userY, "n-elsewhere"), &pb.ReturnAssetReq{AssetId: f.assetA})
	asserted(t, err, codes.PermissionDenied)

	_, err = srv.ReturnAsset(asCaller(f.userY, "n-manager"), &pb.ReturnAssetReq{AssetId: f.assetA})
	require.NoError(t, err)
}

// An asset belongs to the workspace of its type. A request that names another
// workspace for it would file an asset of one tenant under another's id.
func TestCreateAsset_TypeMustBelongToTheNamedWorkspace(t *testing.T) {
	f := newFixture(t)
	f.withLifecycleOnA(t)
	p := f.policy()
	p.grant("n-writer", f.oaA, ngac.OpWrite)
	srv := f.srv(p)

	_, err := srv.CreateAsset(asCaller(f.userX, "n-writer"), &pb.CreateAssetRequest{
		Name: "Misfiled", TypeId: f.typeA, WorkspaceId: "some-other-workspace",
	})

	asserted(t, err, codes.InvalidArgument)
	var n int
	require.NoError(t, f.pool.QueryRow(context.Background(), `SELECT count(*) FROM assets WHERE name = 'Misfiled'`).Scan(&n))
	assert.Zero(t, n)
}

// Without write on the type's OA the caller learns nothing about workspaces:
// the authorization answer comes first.
func TestCreateAsset_MismatchedWorkspaceWithoutWriteIsPermissionDenied(t *testing.T) {
	f := newFixture(t)
	f.withLifecycleOnA(t)

	_, err := f.srv(f.policy()).CreateAsset(asCaller(f.userX, "n-nobody"), &pb.CreateAssetRequest{
		Name: "Misfiled", TypeId: f.typeA, WorkspaceId: "some-other-workspace",
	})

	asserted(t, err, codes.PermissionDenied)
}
