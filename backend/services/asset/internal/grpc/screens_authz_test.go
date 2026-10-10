package grpc_test

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"

	"ngac-platform/ngac"
	pb "ngac-platform/proto/asset"
	agrpc "ngac-platform/services/asset/internal/grpc"
)

// What the asset screens ask of the server, and what the server refuses. Every
// operation is decided on the OA of the asset's type (or of the request's
// type); each test pins the grant that allows it and the grants one step off
// that must not.

// roster makes both fixture users active members of the fixture workspace, with
// names, and returns the ids of two extra assets of type A: one available, one
// in maintenance.
func (f *fixture) roster(t *testing.T) (available, maintenance string) {
	t.Helper()
	ctx := context.Background()
	exec := func(q string, args ...any) {
		_, err := f.pool.Exec(ctx, q, args...)
		require.NoError(t, err, q)
	}
	exec(`UPDATE users SET display_name = 'Lê Thị Hoa' WHERE id = $1`, f.userX)
	exec(`UPDATE users SET display_name = 'Phạm Hải Yến' WHERE id = $1`, f.userY)
	exec(`INSERT INTO tenant_users (tenant_id, user_id) VALUES ($1, $2), ($1, $3)`, f.wsID, f.userX, f.userY)
	available, maintenance = "screens-av-"+uuid.NewString()[:8], "screens-mt-"+uuid.NewString()[:8]
	exec(`INSERT INTO assets (id, name, type_id, workspace_id, state, created_by) VALUES
	      ($1, 'Laptop số 4', $3, $4, 'available', $5), ($2, 'Laptop đang sửa', $3, $4, 'maintenance', $5)`,
		available, maintenance, f.typeA, f.wsID, f.userX)
	return available, maintenance
}

func (f *fixture) assetRow(t *testing.T, id string) (state string, holder *string) {
	t.Helper()
	require.NoError(t, f.pool.QueryRow(context.Background(),
		`SELECT state, assigned_to FROM assets WHERE id = $1`, id).Scan(&state, &holder))
	return state, holder
}

func (f *fixture) requestRow(t *testing.T, id string) (status string, assetID *string) {
	t.Helper()
	require.NoError(t, f.pool.QueryRow(context.Background(),
		`SELECT status, assigned_asset_id FROM asset_requests WHERE id = $1`, id).Scan(&status, &assetID))
	return status, assetID
}

func (f *fixture) reqSrv(p *fakePolicyRead) *agrpc.AssetRequestServer {
	return agrpc.NewAssetRequestServer(f.st, p, &fakePolicyWrite{}, nil)
}

// ---------------------------------------------------------------------------
// Hand over: manage on the asset's type OA
// ---------------------------------------------------------------------------

func TestHandOverAsset_ManageOnTheTypeOAHandsItToAMember(t *testing.T) {
	f := newFixture(t)
	available, _ := f.roster(t)
	p := f.policy()
	p.grant("n-manager", f.oaA, ngac.OpManage)
	p.grant("n-manager", f.oaA, ngac.OpRead)

	a, err := f.srv(p).HandOverAsset(asCaller(f.userX, "n-manager"), &pb.HandOverRequest{AssetId: available, AssigneeId: f.userY, Comment: "Thay máy cũ"})

	require.NoError(t, err)
	assert.Equal(t, "assigned", a.State)
	assert.Equal(t, f.userY, a.AssignedToUserId)
	assert.Equal(t, "Phạm Hải Yến", a.AssignedToName)
	assert.Contains(t, p.checks, [3]string{"n-manager", f.oaA, ngac.OpManage}, "the check lands on the type OA")

	hist, err := f.srv(p).GetAssetHistory(asCaller(f.userX, "n-manager"), &pb.GetHistoryRequest{AssetId: available})
	require.NoError(t, err)
	require.Len(t, hist.Records, 1)
	assert.Equal(t, "Lê Thị Hoa", hist.Records[0].ActorName)
	assert.Equal(t, "Phạm Hải Yến", hist.Records[0].SubjectName)
	assert.Equal(t, "Thay máy cũ", hist.Records[0].Comment)
}

func TestHandOverAsset_DeniedOffTheTypeOAChangesNothing(t *testing.T) {
	cases := map[string]func(p *fakePolicyRead, f *fixture){
		"manage on another type's OA": func(p *fakePolicyRead, f *fixture) { p.grant("n-x", f.oaB, ngac.OpManage) },
		"write, not manage":           func(p *fakePolicyRead, f *fixture) { p.grant("n-x", f.oaA, ngac.OpWrite) },
		"read, not manage":            func(p *fakePolicyRead, f *fixture) { p.grant("n-x", f.oaA, ngac.OpRead) },
		"approve, not manage":         func(p *fakePolicyRead, f *fixture) { p.grant("n-x", f.oaA, ngac.OpApprove) },
		"nothing":                     func(*fakePolicyRead, *fixture) {},
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			available, _ := f.roster(t)
			p := f.policy()
			setup(p, f)

			_, err := f.srv(p).HandOverAsset(asCaller(f.userX, "n-x"), &pb.HandOverRequest{AssetId: available, AssigneeId: f.userY})

			asserted(t, err, codes.PermissionDenied)
			state, holder := f.assetRow(t, available)
			assert.Equal(t, "available", state)
			assert.Nil(t, holder)
		})
	}
}

func TestHandOverAsset_DeniedWithoutCallerOrWhenPolicyFails(t *testing.T) {
	f := newFixture(t)
	available, _ := f.roster(t)
	p := f.policy()
	p.grant("n-manager", f.oaA, ngac.OpManage)

	_, err := f.srv(p).HandOverAsset(context.Background(), &pb.HandOverRequest{AssetId: available, AssigneeId: f.userY})
	asserted(t, err, codes.PermissionDenied)

	p.failErr = errPolicyDown
	_, err = f.srv(p).HandOverAsset(asCaller(f.userX, "n-manager"), &pb.HandOverRequest{AssetId: available, AssigneeId: f.userY})
	asserted(t, err, codes.PermissionDenied)
	state, _ := f.assetRow(t, available)
	assert.Equal(t, "available", state)
}

func TestHandOverAsset_RefusesWhatTheAssetOrThePersonCannotTake(t *testing.T) {
	f := newFixture(t)
	available, maintenance := f.roster(t)
	stranger := "screens-stranger-" + uuid.NewString()[:8]
	_, err := f.pool.Exec(context.Background(), `INSERT INTO users (id, username, password) VALUES ($1, $1, '')`, stranger)
	require.NoError(t, err)
	t.Cleanup(func() { f.pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, stranger) })
	p := f.policy()
	p.grant("n-manager", f.oaA, ngac.OpManage)
	srv := f.srv(p)
	ctx := asCaller(f.userX, "n-manager")

	_, err = srv.HandOverAsset(ctx, &pb.HandOverRequest{AssetId: maintenance, AssigneeId: f.userY})
	asserted(t, err, codes.FailedPrecondition)

	_, err = srv.HandOverAsset(ctx, &pb.HandOverRequest{AssetId: available, AssigneeId: stranger})
	asserted(t, err, codes.InvalidArgument)

	_, err = srv.HandOverAsset(ctx, &pb.HandOverRequest{AssetId: available, AssigneeId: ""})
	asserted(t, err, codes.InvalidArgument)

	_, err = srv.HandOverAsset(ctx, &pb.HandOverRequest{AssetId: "missing-" + uuid.NewString(), AssigneeId: f.userY})
	asserted(t, err, codes.NotFound)

	state, holder := f.assetRow(t, available)
	assert.Equal(t, "available", state)
	assert.Nil(t, holder)
}

// ---------------------------------------------------------------------------
// The generic transition does not hand an asset to nobody
// ---------------------------------------------------------------------------

func TestTransitionAsset_AssignNeedsAPersonSoItGoesThroughHandOver(t *testing.T) {
	f := newFixture(t)
	available, _ := f.roster(t)
	f.withLifecycleOnA(t)
	p := f.policy()
	p.grant("n-manager", f.oaA, ngac.OpManage)
	srv := f.srv(p)

	_, err := srv.TransitionAsset(asCaller(f.userX, "n-manager"), &pb.TransitionRequest{AssetId: available, Action: "assign"})
	asserted(t, err, codes.InvalidArgument)
	state, holder := f.assetRow(t, available)
	assert.Equal(t, "available", state, "no asset ends up 'assigned' with no holder")
	assert.Nil(t, holder)

	// Without manage the answer is a denial, whatever the action.
	_, err = srv.TransitionAsset(asCaller(f.userX, "n-nobody"), &pb.TransitionRequest{AssetId: available, Action: "assign"})
	asserted(t, err, codes.PermissionDenied)
}

func TestTransitionAsset_ReturningClearsTheHolder(t *testing.T) {
	f := newFixture(t)
	available, _ := f.roster(t)
	f.withLifecycleOnA(t)
	p := f.policy()
	p.grant("n-manager", f.oaA, ngac.OpManage)
	srv := f.srv(p)
	_, err := srv.HandOverAsset(asCaller(f.userX, "n-manager"), &pb.HandOverRequest{AssetId: available, AssigneeId: f.userY})
	require.NoError(t, err)

	_, err = srv.TransitionAsset(asCaller(f.userX, "n-manager"), &pb.TransitionRequest{AssetId: available, Action: "return"})

	require.NoError(t, err)
	state, holder := f.assetRow(t, available)
	assert.Equal(t, "available", state)
	assert.Nil(t, holder)
}

func TestGetAvailableTransitions_SaysWhetherTheCallerMayHandTheAssetOver(t *testing.T) {
	f := newFixture(t)
	available, maintenance := f.roster(t)
	f.withLifecycleOnA(t)
	p := f.policy()
	p.grant("n-manager", f.oaA, ngac.OpManage)
	p.grant("n-manager", f.oaA, ngac.OpRead)
	p.grant("n-reader", f.oaA, ngac.OpRead)
	p.grant("n-elsewhere", f.oaB, ngac.OpManage)
	srv := f.srv(p)

	got, err := srv.GetAvailableTransitions(asCaller(f.userX, "n-manager"), &pb.GetTransitionsRequest{AssetId: available})
	require.NoError(t, err)
	assert.True(t, got.CanAssign)

	got, err = srv.GetAvailableTransitions(asCaller(f.userX, "n-manager"), &pb.GetTransitionsRequest{AssetId: maintenance})
	require.NoError(t, err)
	assert.False(t, got.CanAssign, "an asset in repair cannot be handed over")

	got, err = srv.GetAvailableTransitions(asCaller(f.userX, "n-reader"), &pb.GetTransitionsRequest{AssetId: available})
	require.NoError(t, err)
	assert.False(t, got.CanAssign)
	for _, caller := range []string{"n-elsewhere", "n-nobody"} {
		_, err = srv.GetAvailableTransitions(asCaller(f.userX, caller), &pb.GetTransitionsRequest{AssetId: available})
		asserted(t, err, codes.PermissionDenied)
	}
}

// ---------------------------------------------------------------------------
// Summary and activity: only what the caller may read
// ---------------------------------------------------------------------------

func TestGetSummary_CountsOnlyTheTypesTheCallerMayRead(t *testing.T) {
	f := newFixture(t)
	f.roster(t)
	p := f.policy()
	p.grant("n-reader-a", f.oaA, ngac.OpRead)
	srv := f.srv(p)

	got, err := srv.GetSummary(asCaller(f.userX, "n-reader-a"), &pb.GetSummaryRequest{WorkspaceId: f.wsID})
	require.NoError(t, err)
	assert.Equal(t, int32(3), got.Total, "assetA plus the two added; assetB is on a type the caller cannot read")
	assert.Equal(t, int32(1), got.ByState["available"])
	require.Len(t, got.ByType, 1)
	assert.Equal(t, f.typeA, got.ByType[0].TypeId)

	for name, caller := range map[string]string{"nothing": "n-nobody", "another type": "n-reader-b"} {
		p.grant("n-reader-b", f.oaB, ngac.OpWrite) // not read
		got, err = srv.GetSummary(asCaller(f.userX, caller), &pb.GetSummaryRequest{WorkspaceId: f.wsID})
		require.NoError(t, err, name)
		assert.Zero(t, got.Total, name)
		assert.Empty(t, got.ByType, name)
	}

	_, err = srv.GetSummary(context.Background(), &pb.GetSummaryRequest{WorkspaceId: f.wsID})
	require.NoError(t, err)
	p.failErr = errPolicyDown
	_, err = srv.GetSummary(asCaller(f.userX, "n-reader-a"), &pb.GetSummaryRequest{WorkspaceId: f.wsID})
	require.Error(t, err, "a policy failure is an error, not an empty dashboard")
}

func TestListActivity_ShowsStepsOnReadableTypesWithTheActorNamed(t *testing.T) {
	f := newFixture(t)
	available, _ := f.roster(t)
	p := f.policy()
	p.grant("n-manager", f.oaA, ngac.OpManage)
	p.grant("n-manager", f.oaA, ngac.OpRead)
	srv := f.srv(p)
	_, err := srv.HandOverAsset(asCaller(f.userX, "n-manager"), &pb.HandOverRequest{AssetId: available, AssigneeId: f.userY})
	require.NoError(t, err)

	got, err := srv.ListActivity(asCaller(f.userX, "n-manager"), &pb.ListActivityRequest{WorkspaceId: f.wsID, Limit: 5})
	require.NoError(t, err)
	require.Len(t, got.Entries, 1)
	e := got.Entries[0]
	assert.Equal(t, "Lê Thị Hoa", e.ActorName)
	assert.Equal(t, "Phạm Hải Yến", e.SubjectName)
	assert.Equal(t, "Laptop số 4", e.AssetName)
	assert.Equal(t, "assign", e.Action)

	// Manage without read is not a licence to read the trail.
	p.grant("n-manage-only", f.oaA, ngac.OpManage)
	for _, caller := range []string{"n-nobody", "n-manage-only"} {
		got, err = srv.ListActivity(asCaller(f.userX, caller), &pb.ListActivityRequest{WorkspaceId: f.wsID})
		require.NoError(t, err)
		assert.Empty(t, got.Entries, caller)
	}
	got, err = srv.ListActivity(context.Background(), &pb.ListActivityRequest{WorkspaceId: f.wsID})
	require.NoError(t, err)
	assert.Empty(t, got.Entries)
}

// ---------------------------------------------------------------------------
// Types: what the caller sees and may do
// ---------------------------------------------------------------------------

func TestListTypes_ReturnsTheTypesTheCallerHoldsAnOperationOnWithThoseOperations(t *testing.T) {
	f := newFixture(t)
	f.roster(t)
	p := f.policy()
	p.grant("n-requester", f.oaA, ngac.OpWrite)
	p.grant("n-requester", f.oaA, ngac.OpRead)
	p.grant("n-requester", f.oaB, ngac.OpApprove)
	srv := agrpc.NewAssetTypeServer(f.st, p, &fakePolicyWrite{})

	got, err := srv.ListTypes(asCaller(f.userX, "n-requester"), &pb.ListTypesRequest{WorkspaceId: f.wsID})
	require.NoError(t, err)
	perms := map[string][]string{}
	for _, at := range got.Types {
		perms[at.Id] = at.Permissions
	}
	assert.ElementsMatch(t, []string{ngac.OpRead, ngac.OpWrite}, perms[f.typeA])
	assert.ElementsMatch(t, []string{ngac.OpApprove}, perms[f.typeB])
	assert.False(t, got.CanManage, "no manage on the Assets OA")
	for _, at := range got.Types {
		if at.Id == f.typeA {
			assert.Equal(t, int32(1), at.AvailableCount)
		}
	}
}

func TestListTypes_NothingHeldListsNothingAndManageOnTheAssetsOAMayManage(t *testing.T) {
	f := newFixture(t)
	p := f.policy()
	p.grant("n-admin", f.assetsOA(), ngac.OpManage)
	p.grant("n-admin", f.oaA, ngac.OpRead)
	srv := agrpc.NewAssetTypeServer(f.st, p, &fakePolicyWrite{})

	none, err := srv.ListTypes(asCaller(f.userX, "n-nobody"), &pb.ListTypesRequest{WorkspaceId: f.wsID})
	require.NoError(t, err)
	assert.Empty(t, none.Types)
	assert.False(t, none.CanManage)

	admin, err := srv.ListTypes(asCaller(f.userX, "n-admin"), &pb.ListTypesRequest{WorkspaceId: f.wsID})
	require.NoError(t, err)
	assert.True(t, admin.CanManage)
	assert.Len(t, admin.Types, 1)

	_, err = srv.ListTypes(context.Background(), &pb.ListTypesRequest{WorkspaceId: f.wsID})
	asserted(t, err, codes.PermissionDenied)

	p.failErr = errPolicyDown
	_, err = srv.ListTypes(asCaller(f.userX, "n-admin"), &pb.ListTypesRequest{WorkspaceId: f.wsID})
	require.Error(t, err, "a policy failure must not list types")
}

// ---------------------------------------------------------------------------
// Requests: urgency, workspace, reason
// ---------------------------------------------------------------------------

func TestCreateRequest_KeepsTheUrgencyAndRefusesAnUnknownOne(t *testing.T) {
	f := newFixture(t)
	f.roster(t)
	p := f.policy()
	p.grant("n-writer", f.oaA, ngac.OpWrite)
	srv := f.reqSrv(p)

	r, err := srv.CreateRequest(asCaller(f.userY, "n-writer"), &pb.CreateAssetRequestReq{TypeId: f.typeA, WorkspaceId: f.wsID, Justification: "Cần gấp", Urgency: "urgent"})
	require.NoError(t, err)
	assert.Equal(t, "urgent", r.Urgency)
	assert.Equal(t, "Phạm Hải Yến", r.RequesterName)

	r, err = srv.CreateRequest(asCaller(f.userY, "n-writer"), &pb.CreateAssetRequestReq{TypeId: f.typeA, WorkspaceId: f.wsID, Justification: "Không nói mức độ"})
	require.NoError(t, err)
	assert.Equal(t, "normal", r.Urgency)

	_, err = srv.CreateRequest(asCaller(f.userY, "n-writer"), &pb.CreateAssetRequestReq{TypeId: f.typeA, WorkspaceId: f.wsID, Justification: "x", Urgency: "asap"})
	asserted(t, err, codes.InvalidArgument)
}

func TestCreateRequest_TypeMustBelongToTheNamedWorkspace(t *testing.T) {
	f := newFixture(t)
	p := f.policy()
	p.grant("n-writer", f.oaA, ngac.OpWrite)
	srv := f.reqSrv(p)

	_, err := srv.CreateRequest(asCaller(f.userY, "n-writer"), &pb.CreateAssetRequestReq{TypeId: f.typeA, WorkspaceId: "some-other-workspace", Justification: "x"})
	asserted(t, err, codes.InvalidArgument)

	// Without write the answer is a denial and says nothing of workspaces.
	_, err = srv.CreateRequest(asCaller(f.userY, "n-nobody"), &pb.CreateAssetRequestReq{TypeId: f.typeA, WorkspaceId: "some-other-workspace", Justification: "x"})
	asserted(t, err, codes.PermissionDenied)
}

func TestRejectRequest_NeedsAReason(t *testing.T) {
	f := newFixture(t)
	p := f.policy()
	p.grant("n-approver", f.oaA, ngac.OpApprove)
	srv := f.reqSrv(p)
	ctx := asCaller(f.userY, "n-approver")

	for _, reason := range []string{"", "   "} {
		_, err := srv.RejectRequest(ctx, &pb.RejectRequestReq{RequestId: f.reqByXonA, Reason: reason})
		asserted(t, err, codes.InvalidArgument)
	}
	status, _ := f.requestRow(t, f.reqByXonA)
	assert.Equal(t, "pending", status)

	r, err := srv.RejectRequest(ctx, &pb.RejectRequestReq{RequestId: f.reqByXonA, Reason: "Kho còn máy đời cũ"})
	require.NoError(t, err)
	assert.Equal(t, "rejected", r.Status)
	assert.Equal(t, "Kho còn máy đời cũ", r.ApproverComment)
}

// ---------------------------------------------------------------------------
// Approve and hand over an asset, in one step
// ---------------------------------------------------------------------------

func TestApproveRequest_WithAnAsset_NeedsApproveAndManageOnTheTypeOA(t *testing.T) {
	f := newFixture(t)
	available, _ := f.roster(t)
	p := f.policy()
	p.grant("n-both", f.oaA, ngac.OpApprove)
	p.grant("n-both", f.oaA, ngac.OpManage)
	srv := f.reqSrv(p)

	r, err := srv.ApproveRequest(asCaller(f.userY, "n-both"), &pb.ApproveRequestReq{RequestId: f.reqByXonA, AssetId: available, Comment: "Máy mới nhất"})

	require.NoError(t, err)
	assert.Equal(t, "fulfilled", r.Status)
	assert.Equal(t, available, r.AssignedAssetId)
	assert.Equal(t, "Laptop số 4", r.AssignedAssetName)
	assert.Equal(t, "Phạm Hải Yến", r.ApproverName)
	state, holder := f.assetRow(t, available)
	assert.Equal(t, "assigned", state)
	require.NotNil(t, holder)
	assert.Equal(t, f.userX, *holder, "the asset goes to whoever asked")
}

func TestApproveRequest_WithAnAsset_DeniedWithoutBothOperationsChangesNothing(t *testing.T) {
	cases := map[string]func(p *fakePolicyRead, f *fixture){
		"approve, not manage": func(p *fakePolicyRead, f *fixture) { p.grant("n-x", f.oaA, ngac.OpApprove) },
		"manage, not approve": func(p *fakePolicyRead, f *fixture) { p.grant("n-x", f.oaA, ngac.OpManage) },
		"both, on another type's OA": func(p *fakePolicyRead, f *fixture) {
			p.grant("n-x", f.oaB, ngac.OpApprove)
			p.grant("n-x", f.oaB, ngac.OpManage)
		},
		"read and write only": func(p *fakePolicyRead, f *fixture) {
			p.grant("n-x", f.oaA, ngac.OpRead)
			p.grant("n-x", f.oaA, ngac.OpWrite)
		},
		"nothing": func(*fakePolicyRead, *fixture) {},
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			available, _ := f.roster(t)
			p := f.policy()
			setup(p, f)

			_, err := f.reqSrv(p).ApproveRequest(asCaller(f.userY, "n-x"), &pb.ApproveRequestReq{RequestId: f.reqByXonA, AssetId: available})

			asserted(t, err, codes.PermissionDenied)
			status, given := f.requestRow(t, f.reqByXonA)
			assert.Equal(t, "pending", status, "an approval that is refused leaves the request open")
			assert.Nil(t, given)
			state, holder := f.assetRow(t, available)
			assert.Equal(t, "available", state)
			assert.Nil(t, holder)
		})
	}
}

func TestApproveRequest_WithAnAsset_TheAssetMustBeAvailableAndOfTheRequestedType(t *testing.T) {
	f := newFixture(t)
	available, maintenance := f.roster(t)
	p := f.policy()
	for _, oa := range []string{f.oaA, f.oaB} {
		p.grant("n-both", oa, ngac.OpApprove)
		p.grant("n-both", oa, ngac.OpManage)
	}
	srv := f.reqSrv(p)
	ctx := asCaller(f.userY, "n-both")

	// A free asset, but of the other type.
	otherType := "screens-ot-" + uuid.NewString()[:8]
	_, err := f.pool.Exec(context.Background(), `INSERT INTO assets (id, name, type_id, workspace_id, state, created_by) VALUES ($1, 'Màn hình', $2, $3, 'available', $4)`, otherType, f.typeB, f.wsID, f.userX)
	require.NoError(t, err)

	_, err = srv.ApproveRequest(ctx, &pb.ApproveRequestReq{RequestId: f.reqByXonA, AssetId: otherType})
	asserted(t, err, codes.InvalidArgument)

	_, err = srv.ApproveRequest(ctx, &pb.ApproveRequestReq{RequestId: f.reqByXonA, AssetId: maintenance})
	asserted(t, err, codes.FailedPrecondition)

	_, err = srv.ApproveRequest(ctx, &pb.ApproveRequestReq{RequestId: f.reqByXonA, AssetId: "missing-" + uuid.NewString()})
	asserted(t, err, codes.NotFound)

	status, given := f.requestRow(t, f.reqByXonA)
	assert.Equal(t, "pending", status)
	assert.Nil(t, given)

	// The same asset cannot be given twice: after one request takes it, the next is refused.
	other := "screens-req-" + uuid.NewString()[:8]
	_, err = f.pool.Exec(context.Background(), `INSERT INTO asset_requests (id, type_id, workspace_id, requester_id, justification) VALUES ($1, $2, $3, $4, 'again')`, other, f.typeA, f.wsID, f.userY)
	require.NoError(t, err)
	_, err = srv.ApproveRequest(asCaller(f.userX, "n-both"), &pb.ApproveRequestReq{RequestId: other, AssetId: available})
	require.NoError(t, err)
	_, err = srv.ApproveRequest(ctx, &pb.ApproveRequestReq{RequestId: f.reqByXonA, AssetId: available})
	asserted(t, err, codes.FailedPrecondition)
	status, _ = f.requestRow(t, f.reqByXonA)
	assert.Equal(t, "pending", status)
}

func TestApproveRequest_WithAnAsset_NobodyApprovesTheirOwnRequest(t *testing.T) {
	f := newFixture(t)
	available, _ := f.roster(t)
	p := f.policy()
	p.grant("n-both", f.oaA, ngac.OpApprove)
	p.grant("n-both", f.oaA, ngac.OpManage)

	// userX made reqByXonA.
	_, err := f.reqSrv(p).ApproveRequest(asCaller(f.userX, "n-both"), &pb.ApproveRequestReq{RequestId: f.reqByXonA, AssetId: available})

	asserted(t, err, codes.PermissionDenied)
	status, _ := f.requestRow(t, f.reqByXonA)
	assert.Equal(t, "pending", status)
}

func TestApproveRequest_WithoutAnAssetStillOnlyNeedsApprove(t *testing.T) {
	f := newFixture(t)
	f.roster(t)
	p := f.policy()
	p.grant("n-approver", f.oaA, ngac.OpApprove)

	r, err := f.reqSrv(p).ApproveRequest(asCaller(f.userY, "n-approver"), &pb.ApproveRequestReq{RequestId: f.reqByXonA})

	require.NoError(t, err)
	assert.Equal(t, "approved", r.Status)
	assert.Empty(t, r.AssignedAssetId)
}

// ---------------------------------------------------------------------------
// Give an asset to a request that was approved earlier
// ---------------------------------------------------------------------------

func TestAssignAsset_ManageOnTheRequestsTypeOAFulfilsAnApprovedRequest(t *testing.T) {
	f := newFixture(t)
	available, _ := f.roster(t)
	p := f.policy()
	p.grant("n-approver", f.oaA, ngac.OpApprove)
	p.grant("n-manager", f.oaA, ngac.OpManage)
	srv := f.reqSrv(p)

	// Not approved yet.
	_, err := srv.AssignAsset(asCaller(f.userY, "n-manager"), &pb.AssignAssetReq{RequestId: f.reqByXonA, AssetId: available})
	asserted(t, err, codes.FailedPrecondition)

	_, err = srv.ApproveRequest(asCaller(f.userY, "n-approver"), &pb.ApproveRequestReq{RequestId: f.reqByXonA})
	require.NoError(t, err)

	// Approve is not manage.
	_, err = srv.AssignAsset(asCaller(f.userY, "n-approver"), &pb.AssignAssetReq{RequestId: f.reqByXonA, AssetId: available})
	asserted(t, err, codes.PermissionDenied)
	state, _ := f.assetRow(t, available)
	assert.Equal(t, "available", state)

	r, err := srv.AssignAsset(asCaller(f.userY, "n-manager"), &pb.AssignAssetReq{RequestId: f.reqByXonA, AssetId: available})
	require.NoError(t, err)
	assert.Equal(t, "fulfilled", r.Status)
	assert.Equal(t, "Laptop số 4", r.AssignedAssetName)
	state, holder := f.assetRow(t, available)
	assert.Equal(t, "assigned", state)
	assert.Equal(t, f.userX, *holder)
}

// ---------------------------------------------------------------------------
// Return: the asset must be out
// ---------------------------------------------------------------------------

func TestReturnAsset_OnlyAnAssignedAssetComesBackAndItsHolderIsCleared(t *testing.T) {
	f := newFixture(t)
	available, _ := f.roster(t)
	p := f.policy()
	p.grant("n-manager", f.oaA, ngac.OpManage)
	srv := f.srv(p)
	req := f.reqSrv(p)

	_, err := req.ReturnAsset(asCaller(f.userX, "n-manager"), &pb.ReturnAssetReq{AssetId: available})
	asserted(t, err, codes.FailedPrecondition)

	_, err = srv.HandOverAsset(asCaller(f.userX, "n-manager"), &pb.HandOverRequest{AssetId: available, AssigneeId: f.userY})
	require.NoError(t, err)
	_, err = req.ReturnAsset(asCaller(f.userX, "n-manager"), &pb.ReturnAssetReq{AssetId: available})
	require.NoError(t, err)
	state, holder := f.assetRow(t, available)
	assert.Equal(t, "available", state)
	assert.Nil(t, holder)
}

// ---------------------------------------------------------------------------
// What the caller may do with a request, so a screen offers only that
// ---------------------------------------------------------------------------

func TestListRequests_SaysWhatTheCallerMayDoWithEach(t *testing.T) {
	f := newFixture(t)
	f.roster(t)
	p := f.policy()
	p.grant("n-both", f.oaA, ngac.OpApprove)
	p.grant("n-both", f.oaA, ngac.OpManage)
	p.grant("n-approve-only", f.oaA, ngac.OpApprove)
	srv := f.reqSrv(p)

	flags := func(userID, node string) (decide, assign bool) {
		list, err := srv.ListRequests(asCaller(userID, node), &pb.ListRequestsReq{WorkspaceId: f.wsID})
		require.NoError(t, err)
		for _, r := range list.Requests {
			if r.Id == f.reqByXonA {
				return r.CanDecide, r.CanAssign
			}
		}
		t.Fatalf("request not listed for %s", node)
		return
	}
	decide, assign := flags(f.userY, "n-both")
	assert.True(t, decide)
	assert.True(t, assign)
	decide, assign = flags(f.userY, "n-approve-only")
	assert.True(t, decide)
	assert.False(t, assign)

	// The person who asked sees it, and may do nothing with it.
	decide, assign = flags(f.userX, "n-x")
	assert.False(t, decide)
	assert.False(t, assign)

	// The same through GetRequest.
	got, err := srv.GetRequest(asCaller(f.userY, "n-approve-only"), &pb.GetRequestReq{RequestId: f.reqByXonA})
	require.NoError(t, err)
	assert.True(t, got.CanDecide)
	assert.False(t, got.CanAssign)
	got, err = srv.GetRequest(asCaller(f.userX, "n-both"), &pb.GetRequestReq{RequestId: f.reqByXonA})
	require.NoError(t, err)
	assert.False(t, got.CanDecide, "nobody decides their own request")
}

func TestListRequests_StatusListAndNames(t *testing.T) {
	f := newFixture(t)
	f.roster(t)
	p := f.policy()
	p.grant("n-approver", f.oaA, ngac.OpApprove)
	srv := f.reqSrv(p)
	_, err := srv.RejectRequest(asCaller(f.userY, "n-approver"), &pb.RejectRequestReq{RequestId: f.reqByXonA, Reason: "Không đủ ngân sách"})
	require.NoError(t, err)

	list, err := srv.ListRequests(asCaller(f.userY, "n-approver"), &pb.ListRequestsReq{WorkspaceId: f.wsID, Status: "rejected,fulfilled"})
	require.NoError(t, err)
	require.Len(t, list.Requests, 1)
	assert.Equal(t, "Lê Thị Hoa", list.Requests[0].RequesterName)
	assert.Equal(t, "Phạm Hải Yến", list.Requests[0].ApproverName)
	assert.False(t, list.Requests[0].CanDecide, "a decided request offers no decision")
}

// ---------------------------------------------------------------------------
// Existence is not leaked; limits; people of other tenants; machine reasons
// ---------------------------------------------------------------------------

func TestDecisions_AMissingRequestAndAForbiddenOneAnswerAlike(t *testing.T) {
	f := newFixture(t)
	f.roster(t)
	p := f.policy()
	srv := f.reqSrv(p)
	ctx := asCaller(f.userY, "n-nobody")
	missing := "missing-" + uuid.NewString()

	for name, call := range map[string]func(id string) error{
		"approve": func(id string) error { _, e := srv.ApproveRequest(ctx, &pb.ApproveRequestReq{RequestId: id}); return e },
		"reject": func(id string) error {
			_, e := srv.RejectRequest(ctx, &pb.RejectRequestReq{RequestId: id, Reason: "x"})
			return e
		},
		"assign": func(id string) error {
			_, e := srv.AssignAsset(ctx, &pb.AssignAssetReq{RequestId: id, AssetId: "a"})
			return e
		},
	} {
		real, none := call(f.reqByXonA), call(missing)
		asserted(t, real, codes.PermissionDenied)
		asserted(t, none, codes.PermissionDenied)
		assert.Equal(t, status.Convert(real).Message(), status.Convert(none).Message(), name)
	}
	// Not pending is said only to someone who may decide.
	p.grant("n-approver", f.oaA, ngac.OpApprove)
	_, err := srv.ApproveRequest(asCaller(f.userY, "n-approver"), &pb.ApproveRequestReq{RequestId: f.reqByXonA})
	require.NoError(t, err)
	_, err = srv.ApproveRequest(asCaller(f.userY, "n-approver"), &pb.ApproveRequestReq{RequestId: f.reqByXonA})
	asserted(t, err, codes.FailedPrecondition)
	asserted(t, func() error { _, e := srv.ApproveRequest(ctx, &pb.ApproveRequestReq{RequestId: f.reqByXonA}); return e }(), codes.PermissionDenied)
}

func reasonOf(err error) string {
	for _, d := range status.Convert(err).Details() {
		if info, ok := d.(*errdetails.ErrorInfo); ok {
			return info.Reason
		}
	}
	return ""
}

func TestRefusals_CarryAMachineReadableReason(t *testing.T) {
	f := newFixture(t)
	available, maintenance := f.roster(t)
	p := f.policy()
	p.grant("n-both", f.oaA, ngac.OpApprove)
	p.grant("n-both", f.oaA, ngac.OpManage)
	srv := f.reqSrv(p)
	ctx := asCaller(f.userY, "n-both")

	_, err := srv.ApproveRequest(ctx, &pb.ApproveRequestReq{RequestId: f.reqByXonA, AssetId: maintenance})
	assert.Equal(t, agrpc.ReasonAssetUnavailable, reasonOf(err))

	_, err = srv.ApproveRequest(ctx, &pb.ApproveRequestReq{RequestId: f.reqByXonA, AssetId: available})
	require.NoError(t, err)
	_, err = srv.ApproveRequest(ctx, &pb.ApproveRequestReq{RequestId: f.reqByXonA})
	assert.Equal(t, agrpc.ReasonRequestNotOpen, reasonOf(err), "a request already decided is not the same refusal as an asset just given")

	_, err = f.srv(p).HandOverAsset(asCaller(f.userX, "n-both"), &pb.HandOverRequest{AssetId: maintenance, AssigneeId: f.userY})
	assert.Equal(t, agrpc.ReasonAssetUnavailable, reasonOf(err))
	stranger := "screens-stranger-" + uuid.NewString()[:8]
	_, e := f.pool.Exec(context.Background(), `INSERT INTO users (id, username, password) VALUES ($1, $1, '')`, stranger)
	require.NoError(t, e)
	t.Cleanup(func() { f.pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, stranger) })
	_, err = f.srv(p).HandOverAsset(asCaller(f.userX, "n-both"), &pb.HandOverRequest{AssetId: "x-" + available, AssigneeId: stranger})
	asserted(t, err, codes.NotFound)
}

func TestStaleDecision_IsAConflictNotAnInternalError(t *testing.T) {
	f := newFixture(t)
	available, _ := f.roster(t)
	f.withLifecycleOnA(t)
	p := f.policy()
	p.grant("n-manager", f.oaA, ngac.OpManage)
	p.grant("n-manager", f.oaA, ngac.OpRead)
	srv := f.srv(p)
	ctx := asCaller(f.userX, "n-manager")

	// The asset is handed out between the moment a screen read "available" and the step.
	require.NoError(t, f.st.HandOver(context.Background(), available, f.userY, f.userX, ""))
	_, err := srv.TransitionAsset(ctx, &pb.TransitionRequest{AssetId: available, Action: "retire"})
	asserted(t, err, codes.FailedPrecondition)
	st, _ := f.assetRow(t, available)
	assert.Equal(t, "assigned", st)
}

func TestReturnAsset_RefusedWhenTheHolderChangedUnderIt(t *testing.T) {
	f := newFixture(t)
	available, _ := f.roster(t)
	p := f.policy()
	p.grant("n-manager", f.oaA, ngac.OpManage)
	req := f.reqSrv(p)
	require.NoError(t, f.st.HandOver(context.Background(), available, f.userY, f.userX, ""))

	// Interleaved: return reads Yến; a hand-over to Hoa lands first; the return must not take it back from Hoa.
	var wg sync.WaitGroup
	errs := make([]error, 2)
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, errs[0] = req.ReturnAsset(asCaller(f.userX, "n-manager"), &pb.ReturnAssetReq{AssetId: available})
	}()
	go func() {
		defer wg.Done()
		errs[1] = f.st.HandOver(context.Background(), available, f.userX, f.userX, "")
	}()
	wg.Wait()
	state, holder := f.assetRow(t, available)
	if state == "assigned" {
		require.NotNil(t, holder)
	} else {
		assert.Nil(t, holder)
	}
}

func TestLimits_NameReasonAndRejectionReason(t *testing.T) {
	f := newFixture(t)
	f.roster(t)
	f.withLifecycleOnA(t)
	p := f.policy()
	p.grant("n-writer", f.oaA, ngac.OpWrite)
	p.grant("n-approver", f.oaA, ngac.OpApprove)
	long := func(n int) string { return strings.Repeat("đ", n) }

	_, err := f.srv(p).CreateAsset(asCaller(f.userX, "n-writer"), &pb.CreateAssetRequest{Name: long(121), TypeId: f.typeA, WorkspaceId: f.wsID})
	asserted(t, err, codes.InvalidArgument)
	_, err = f.srv(p).CreateAsset(asCaller(f.userX, "n-writer"), &pb.CreateAssetRequest{Name: "   ", TypeId: f.typeA, WorkspaceId: f.wsID})
	asserted(t, err, codes.InvalidArgument)
	_, err = f.srv(p).UpdateAsset(asCaller(f.userX, "n-writer"), &pb.UpdateAssetRequest{AssetId: f.assetA, Name: long(121)})
	asserted(t, err, codes.InvalidArgument)

	_, err = f.reqSrv(p).CreateRequest(asCaller(f.userY, "n-writer"), &pb.CreateAssetRequestReq{TypeId: f.typeA, WorkspaceId: f.wsID, Justification: long(1001)})
	asserted(t, err, codes.InvalidArgument)
	_, err = f.reqSrv(p).RejectRequest(asCaller(f.userY, "n-approver"), &pb.RejectRequestReq{RequestId: f.reqByXonA, Reason: long(1001)})
	asserted(t, err, codes.InvalidArgument)
	status, _ := f.requestRow(t, f.reqByXonA)
	assert.Equal(t, "pending", status)
}

func TestCustomFields_NoSchemaMeansNoValues_AndRequiredTextIsFilled(t *testing.T) {
	f := newFixture(t)
	f.roster(t)
	f.withLifecycleOnA(t)
	p := f.policy()
	p.grant("n-writer", f.oaA, ngac.OpWrite)
	srv := f.srv(p)
	fields, _ := structpb.NewStruct(map[string]any{"x": "y"})

	_, err := srv.CreateAsset(asCaller(f.userX, "n-writer"), &pb.CreateAssetRequest{Name: "L", TypeId: f.typeA, WorkspaceId: f.wsID, CustomFields: fields})
	asserted(t, err, codes.InvalidArgument)

	_, e := f.pool.Exec(context.Background(), `UPDATE asset_types SET fields_schema = $2 WHERE id = $1`, f.typeA,
		`{"type":"object","properties":{"cfg":{"title":"Cấu hình","type":"string","x-kind":"text"}},"required":["cfg"]}`)
	require.NoError(t, e)
	blank, _ := structpb.NewStruct(map[string]any{"cfg": "  "})
	_, err = srv.CreateAsset(asCaller(f.userX, "n-writer"), &pb.CreateAssetRequest{Name: "L", TypeId: f.typeA, WorkspaceId: f.wsID, CustomFields: blank})
	asserted(t, err, codes.InvalidArgument)
}

func TestCustomFields_APersonMustBeAMemberOfTheTypesWorkspace(t *testing.T) {
	f := newFixture(t)
	f.roster(t)
	f.withLifecycleOnA(t)
	p := f.policy()
	p.grant("n-writer", f.oaA, ngac.OpWrite)
	srv := f.srv(p)
	_, e := f.pool.Exec(context.Background(), `UPDATE asset_types SET fields_schema = $2 WHERE id = $1`, f.typeA,
		`{"type":"object","properties":{"owner":{"title":"Phụ trách","type":"string","x-kind":"person"}}}`)
	require.NoError(t, e)

	// A user who belongs to another tenant only.
	otherWS := "screens-ows-" + uuid.NewString()[:8]
	outsider := "screens-out-" + uuid.NewString()[:8]
	for _, q := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO users (id, username, password) VALUES ($1, $1, '')`, []any{outsider}},
		{`INSERT INTO workspaces (id, name, owner_id) VALUES ($1, $1, $2)`, []any{otherWS, outsider}},
		{`INSERT INTO tenant_users (tenant_id, user_id) VALUES ($1, $2)`, []any{otherWS, outsider}},
	} {
		_, err := f.pool.Exec(context.Background(), q.sql, q.args...)
		require.NoError(t, err)
	}
	t.Cleanup(func() {
		c := context.Background()
		f.pool.Exec(c, `DELETE FROM workspaces WHERE id = $1`, otherWS)
		f.pool.Exec(c, `DELETE FROM users WHERE id = $1`, outsider)
	})

	named := func(id string) *structpb.Struct { s, _ := structpb.NewStruct(map[string]any{"owner": id}); return s }
	_, err := srv.CreateAsset(asCaller(f.userX, "n-writer"), &pb.CreateAssetRequest{Name: "L", TypeId: f.typeA, WorkspaceId: f.wsID, CustomFields: named(outsider)})
	asserted(t, err, codes.InvalidArgument)
	assert.Equal(t, agrpc.ReasonNotAMember, reasonOf(err))
	_, err = srv.UpdateAsset(asCaller(f.userX, "n-writer"), &pb.UpdateAssetRequest{AssetId: f.assetA, CustomFields: named(outsider)})
	asserted(t, err, codes.InvalidArgument)

	a, err := srv.CreateAsset(asCaller(f.userX, "n-writer"), &pb.CreateAssetRequest{Name: "L2", TypeId: f.typeA, WorkspaceId: f.wsID, CustomFields: named(f.userY)})
	require.NoError(t, err, "a member of the workspace is fine")
	f.pool.Exec(context.Background(), `DELETE FROM assets WHERE id = $1`, a.Id)
}

func TestListTypes_WriteAloneListsTheTypeWithoutCounts(t *testing.T) {
	f := newFixture(t)
	f.roster(t)
	p := f.policy()
	p.grant("n-w", f.oaA, ngac.OpWrite)
	p.grant("n-r", f.oaA, ngac.OpRead)
	srv := agrpc.NewAssetTypeServer(f.st, p, &fakePolicyWrite{})

	w, err := srv.ListTypes(asCaller(f.userX, "n-w"), &pb.ListTypesRequest{WorkspaceId: f.wsID})
	require.NoError(t, err)
	require.Len(t, w.Types, 1)
	assert.Zero(t, w.Types[0].AssetCount)
	assert.Zero(t, w.Types[0].AvailableCount)
	r, err := srv.ListTypes(asCaller(f.userX, "n-r"), &pb.ListTypesRequest{WorkspaceId: f.wsID})
	require.NoError(t, err)
	assert.NotZero(t, r.Types[0].AssetCount)
	assert.NotZero(t, r.Types[0].AvailableCount)
}

func TestListActivity_ShowsDecisionsOnRequestsOfReadableTypesOnly(t *testing.T) {
	f := newFixture(t)
	f.roster(t)
	p := f.policy()
	p.grant("n-approver", f.oaA, ngac.OpApprove)
	p.grant("n-reader-a", f.oaA, ngac.OpRead)
	p.grant("n-reader-b", f.oaB, ngac.OpRead)
	_, err := f.reqSrv(p).RejectRequest(asCaller(f.userY, "n-approver"), &pb.RejectRequestReq{RequestId: f.reqByXonA, Reason: "Không đủ ngân sách"})
	require.NoError(t, err)
	srv := f.srv(p)

	got, err := srv.ListActivity(asCaller(f.userX, "n-reader-a"), &pb.ListActivityRequest{WorkspaceId: f.wsID})
	require.NoError(t, err)
	require.Len(t, got.Entries, 1)
	e := got.Entries[0]
	assert.Equal(t, "rejected", e.RequestStatus)
	assert.Equal(t, "Phạm Hải Yến", e.ActorName)
	assert.Equal(t, "Lê Thị Hoa", e.SubjectName)

	other, err := srv.ListActivity(asCaller(f.userX, "n-reader-b"), &pb.ListActivityRequest{WorkspaceId: f.wsID})
	require.NoError(t, err)
	assert.Empty(t, other.Entries, "a type the caller cannot read shows no decisions")
}
