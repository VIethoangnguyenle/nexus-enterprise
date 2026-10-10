package grpc_test

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"

	"ngac-platform/ngac"
	pb "ngac-platform/proto/asset"
	"ngac-platform/services/asset/internal/events"
	agrpc "ngac-platform/services/asset/internal/grpc"
)

// recorder keeps what the servers announce and, at the moment of each
// announcement, what the database already holds for the asset: an event that
// outruns its commit would show the old state here.
type recorder struct {
	mu          sync.Mutex
	assignments []events.AssignmentEvent
	requests    []events.RequestEvent
	lifecycle   []events.LifecycleEvent
	stateAtSend map[string]string
	read        func(assetID string) string
}

func (r *recorder) PublishLifecycle(_ context.Context, e events.LifecycleEvent) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lifecycle = append(r.lifecycle, e)
	r.stateAtSend[e.AssetID] = r.read(e.AssetID)
}

func (r *recorder) PublishRequest(_ context.Context, e events.RequestEvent) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.requests = append(r.requests, e)
}

func (r *recorder) PublishAssignment(_ context.Context, e events.AssignmentEvent) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.assignments = append(r.assignments, e)
	r.stateAtSend[e.AssetID] = r.read(e.AssetID)
}

func (f *fixture) recorder(t *testing.T) *recorder {
	t.Helper()
	return &recorder{stateAtSend: map[string]string{}, read: func(id string) string {
		var state string
		require.NoError(t, f.pool.QueryRow(context.Background(), `SELECT state FROM assets WHERE id = $1`, id).Scan(&state))
		return state
	}}
}

func TestHandOver_AnnouncesOnlyAfterTheChangeIsStored(t *testing.T) {
	f := newFixture(t)
	available, _ := f.roster(t)
	p := f.policy()
	p.grant("n-manager", f.oaA, ngac.OpManage)
	rec := f.recorder(t)
	srv := agrpc.NewAssetServer(f.st, p, rec)

	_, err := srv.HandOverAsset(asCaller(f.userX, "n-manager"), &pb.HandOverRequest{AssetId: available, AssigneeId: f.userY})

	require.NoError(t, err)
	require.Len(t, rec.assignments, 1)
	assert.Equal(t, f.wsID, rec.assignments[0].WorkspaceID)
	assert.Equal(t, "assigned", rec.stateAtSend[available], "the new state was already committed when the event went out")
}

func TestHandOver_DeniedOrRefusedAnnouncesNothing(t *testing.T) {
	f := newFixture(t)
	available, maintenance := f.roster(t)
	p := f.policy()
	p.grant("n-manager", f.oaA, ngac.OpManage)
	rec := f.recorder(t)
	srv := agrpc.NewAssetServer(f.st, p, rec)

	_, err := srv.HandOverAsset(asCaller(f.userX, "n-nobody"), &pb.HandOverRequest{AssetId: available, AssigneeId: f.userY})
	asserted(t, err, codes.PermissionDenied)
	_, err = srv.HandOverAsset(asCaller(f.userX, "n-manager"), &pb.HandOverRequest{AssetId: maintenance, AssigneeId: f.userY})
	assert.Error(t, err, "an asset in maintenance cannot be handed over")
	_, err = srv.HandOverAsset(asCaller(f.userX, "n-manager"), &pb.HandOverRequest{AssetId: available})
	assert.Error(t, err, "no assignee")

	assert.Empty(t, rec.assignments)
	assert.Empty(t, rec.requests)
	assert.Empty(t, rec.lifecycle)
}

func TestRejectRequest_AnnouncesAfterStoreAndNotWhenRefused(t *testing.T) {
	f := newFixture(t)
	p := f.policy()
	p.grant("n-approver", f.oaA, ngac.OpApprove)
	rec := f.recorder(t)
	srv := agrpc.NewAssetRequestServer(f.st, p, &fakePolicyWrite{}, rec)

	created, err := srv.CreateRequest(asCaller(f.userY, "n-requester"), &pb.CreateAssetRequestReq{})
	_ = created
	assert.Error(t, err, "without write on the type the request is refused")
	assert.Empty(t, rec.requests, "a refused request announces nothing")

	p.grant("n-requester", f.oaA, ngac.OpWrite)
	created, err = srv.CreateRequest(asCaller(f.userY, "n-requester"), &pb.CreateAssetRequestReq{TypeId: f.typeA, WorkspaceId: f.wsID, Justification: "need a laptop"})
	require.NoError(t, err)
	require.Len(t, rec.requests, 1)
	assert.Equal(t, "pending", rec.requests[0].Status)
	assert.Equal(t, f.wsID, rec.requests[0].WorkspaceID)

	_, err = srv.RejectRequest(asCaller(f.userX, "n-approver"), &pb.RejectRequestReq{RequestId: created.Id})
	assert.Error(t, err, "a reason is required")
	require.Len(t, rec.requests, 1, "the refused rejection announced nothing")

	_, err = srv.RejectRequest(asCaller(f.userX, "n-approver"), &pb.RejectRequestReq{RequestId: created.Id, Reason: "no budget"})
	require.NoError(t, err)
	require.Len(t, rec.requests, 2)
	assert.Equal(t, "rejected", rec.requests[1].Status)
}
