package grpc

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/pkg/realtime"
	pb "ngac-platform/proto/policy"
	"ngac-platform/services/policy/internal/ngac"
)

func newRealtimeWriteServer(t *testing.T) (*WriteServer, *tenantFixture, *realtime.Recorder) {
	t.Helper()
	store, pool := setupWriteTestStore(t)
	f := newTenantFixture(t, store, pool)
	coord := ngac.NewInvalidationCoordinator(nil, nil, nil)
	ws := NewWriteServer(store, nil, coord, nil, ngac.NewProhibitionStore(pool, store.GetGraph()), false)
	rec := &realtime.Recorder{}
	ws.SetRealtime(rec)
	t.Cleanup(func() { pool.Exec(context.Background(), "DELETE FROM ngac_prohibitions WHERE name LIKE 'rt-%'") })
	return ws, &f, rec
}

func onlyPermissionEvent(t *testing.T, rec *realtime.Recorder, f *tenantFixture, users ...string) {
	t.Helper()
	evts := rec.Events()
	require.Len(t, evts, 1)
	e := evts[0]
	assert.Equal(t, realtime.DomainPermission, e.Domain)
	assert.Equal(t, realtime.KindChanged, e.Kind)
	assert.Equal(t, f.ws, e.WorkspaceID)
	assert.Equal(t, f.ws, e.TenantID, "the workspace is the tenant")
	assert.ElementsMatch(t, users, e.UserNodeIDs)
	assert.Equal(t, realtime.LevelUser, e.Level())
}

func TestPermissionEvent_AssignmentCreateAndRemove(t *testing.T) {
	ws, f, rec := newRealtimeWriteServer(t)
	ctx := context.Background()
	other, err := ws.store.CreateNode(ctx, "UA_rt_"+uuid.NewString()[:8], ngac.NodeTypeUserAttribute, nil)
	require.NoError(t, err)
	t.Cleanup(func() { ws.store.DeleteNode(ctx, other.ID) })
	_, err = ws.store.CreateAssignment(ctx, other.ID, f.pc)
	require.NoError(t, err)

	_, err = ws.CreateAssignment(ctx, &pb.CreateAssignmentRequest{ChildId: f.user, ParentId: other.ID})
	require.NoError(t, err)
	onlyPermissionEvent(t, rec, f, f.user)

	rec2 := &realtime.Recorder{}
	ws.SetRealtime(rec2)
	_, err = ws.RemoveAssignment(ctx, &pb.RemoveAssignmentRequest{ChildId: f.user, ParentId: other.ID})
	require.NoError(t, err)
	onlyPermissionEvent(t, rec2, f, f.user)
}

func TestPermissionEvent_AssignmentOfAGroupNamesItsUsers(t *testing.T) {
	ws, f, rec := newRealtimeWriteServer(t)
	ctx := context.Background()
	group, err := ws.store.CreateNode(ctx, "UA_rtg_"+uuid.NewString()[:8], ngac.NodeTypeUserAttribute, nil)
	require.NoError(t, err)
	t.Cleanup(func() { ws.store.DeleteNode(ctx, group.ID) })
	_, err = ws.store.CreateAssignment(ctx, group.ID, f.pc)
	require.NoError(t, err)

	// The UA f.ua (holding f.user) is placed under the new group.
	_, err = ws.CreateAssignment(ctx, &pb.CreateAssignmentRequest{ChildId: f.ua, ParentId: group.ID})
	require.NoError(t, err)
	onlyPermissionEvent(t, rec, f, f.user)
}

func TestPermissionEvent_AssociationCreateAndRemove(t *testing.T) {
	ws, f, rec := newRealtimeWriteServer(t)
	ctx := context.Background()

	_, err := ws.CreateAssociation(ctx, &pb.CreateAssociationRequest{UaId: f.ua, OaId: f.oa, Operations: []string{"write"}})
	require.NoError(t, err)
	onlyPermissionEvent(t, rec, f, f.user)

	rec2 := &realtime.Recorder{}
	ws.SetRealtime(rec2)
	_, err = ws.RemoveAssociation(ctx, &pb.RemoveAssociationRequest{UaId: f.ua, OaId: f.oa})
	require.NoError(t, err)
	onlyPermissionEvent(t, rec2, f, f.user)
}

func TestPermissionEvent_ProhibitionCreateAndRemove(t *testing.T) {
	ws, f, rec := newRealtimeWriteServer(t)
	ctx := context.Background()
	name := "rt-" + uuid.NewString()[:8]

	_, err := ws.CreateProhibition(ctx, &pb.CreateProhibitionRequest{
		Name: name, SubjectId: f.ua, Operations: []string{"read"}, TargetOaIds: []string{f.oa},
	})
	require.NoError(t, err)
	onlyPermissionEvent(t, rec, f, f.user)

	rec2 := &realtime.Recorder{}
	ws.SetRealtime(rec2)
	_, err = ws.RemoveProhibition(ctx, &pb.RemoveProhibitionRequest{Name: name})
	require.NoError(t, err)
	onlyPermissionEvent(t, rec2, f, f.user)
}

func TestPermissionEvent_DeleteNodeNamesTheUsersItHeld(t *testing.T) {
	ws, f, rec := newRealtimeWriteServer(t)
	_, err := ws.DeleteNode(context.Background(), &pb.DeleteNodeRequest{NodeId: f.ua})
	require.NoError(t, err)
	onlyPermissionEvent(t, rec, f, f.user)
}

// An object attribute reaches people through its own resource events; a folder
// placed under another does not tell the workspace's members their permissions
// changed.
func TestPermissionEvent_ObjectAttributeAssignmentAnnouncesNothing(t *testing.T) {
	ws, f, rec := newRealtimeWriteServer(t)
	ctx := context.Background()
	child, err := ws.store.CreateNode(ctx, "OA_rtc_"+uuid.NewString()[:8], ngac.NodeTypeObjectAttr, nil)
	require.NoError(t, err)
	t.Cleanup(func() { ws.store.DeleteNode(ctx, child.ID) })

	_, err = ws.CreateAssignment(ctx, &pb.CreateAssignmentRequest{ChildId: child.ID, ParentId: f.oa})
	require.NoError(t, err)
	_, err = ws.RemoveAssignment(ctx, &pb.RemoveAssignmentRequest{ChildId: child.ID, ParentId: f.oa})
	require.NoError(t, err)
	assert.Empty(t, rec.Events())

	_, err = ws.DeleteNode(ctx, &pb.DeleteNodeRequest{NodeId: child.ID})
	require.NoError(t, err)
	assert.Empty(t, rec.Events(), "deleting an object attribute is the resource's event, not a permission change")
}

// Putting one person into a group tells that person, not everyone else already
// in the group, and not the people who share the workspace.
func TestPermissionEvent_AddingToAGroupAnnouncesOnlyThePerson(t *testing.T) {
	ws, f, rec := newRealtimeWriteServer(t)
	ctx := context.Background()
	newcomer, err := ws.store.CreateNode(ctx, "U_rtn_"+uuid.NewString()[:8], ngac.NodeTypeUser, nil)
	require.NoError(t, err)
	t.Cleanup(func() { ws.store.DeleteNode(ctx, newcomer.ID) })

	// f.ua already holds f.user, a bystander who must hear nothing.
	_, err = ws.CreateAssignment(ctx, &pb.CreateAssignmentRequest{ChildId: newcomer.ID, ParentId: f.ua})
	require.NoError(t, err)
	onlyPermissionEvent(t, rec, f, newcomer.ID)
	assert.NotContains(t, rec.Events()[0].UserNodeIDs, f.user)
}

// A grant on a group reaches that group's users; the object it names, and the
// other groups with rights on the same object, are not part of it.
func TestPermissionEvent_AssociationLeavesOtherHoldersOfTheObjectAlone(t *testing.T) {
	ws, f, rec := newRealtimeWriteServer(t)
	ctx := context.Background()
	otherUA, err := ws.store.CreateNode(ctx, "UA_rto_"+uuid.NewString()[:8], ngac.NodeTypeUserAttribute, nil)
	require.NoError(t, err)
	bystander, err := ws.store.CreateNode(ctx, "U_rto_"+uuid.NewString()[:8], ngac.NodeTypeUser, nil)
	require.NoError(t, err)
	t.Cleanup(func() { ws.store.DeleteNode(ctx, bystander.ID); ws.store.DeleteNode(ctx, otherUA.ID) })
	_, err = ws.store.CreateAssignment(ctx, otherUA.ID, f.pc)
	require.NoError(t, err)
	_, err = ws.store.CreateAssignment(ctx, bystander.ID, otherUA.ID)
	require.NoError(t, err)
	_, err = ws.store.CreateAssociation(ctx, otherUA.ID, f.oa, []string{"read"})
	require.NoError(t, err)

	_, err = ws.CreateAssociation(ctx, &pb.CreateAssociationRequest{UaId: f.ua, OaId: f.oa, Operations: []string{"write"}})
	require.NoError(t, err)

	onlyPermissionEvent(t, rec, f, f.user)
	assert.NotContains(t, rec.Events()[0].UserNodeIDs, bystander.ID, "another holder of rights on the same object")
}

// A prohibition names its subject's users, never the objects it targets.
func TestPermissionEvent_ProhibitionDoesNotExpandItsTargets(t *testing.T) {
	ws, f, rec := newRealtimeWriteServer(t)
	ctx := context.Background()
	otherUA, err := ws.store.CreateNode(ctx, "UA_rtp_"+uuid.NewString()[:8], ngac.NodeTypeUserAttribute, nil)
	require.NoError(t, err)
	bystander, err := ws.store.CreateNode(ctx, "U_rtp_"+uuid.NewString()[:8], ngac.NodeTypeUser, nil)
	require.NoError(t, err)
	t.Cleanup(func() {
		ws.store.DeleteNode(ctx, bystander.ID)
		ws.store.DeleteNode(ctx, otherUA.ID)
	})
	_, err = ws.store.CreateAssignment(ctx, otherUA.ID, f.pc)
	require.NoError(t, err)
	_, err = ws.store.CreateAssignment(ctx, bystander.ID, otherUA.ID)
	require.NoError(t, err)
	_, err = ws.store.CreateAssociation(ctx, otherUA.ID, f.oa, []string{"read"})
	require.NoError(t, err)
	name := "rt-" + uuid.NewString()[:8]

	_, err = ws.CreateProhibition(ctx, &pb.CreateProhibitionRequest{
		Name: name, SubjectId: f.ua, Operations: []string{"read"}, TargetOaIds: []string{f.oa},
	})
	require.NoError(t, err)
	t.Cleanup(func() { ws.RemoveProhibition(ctx, &pb.RemoveProhibitionRequest{Name: name}) })

	onlyPermissionEvent(t, rec, f, f.user)
}

// A rejected or failed write changes nothing, so it announces nothing.
func TestPermissionEvent_NoneWhenTheWriteFails(t *testing.T) {
	ws, f, rec := newRealtimeWriteServer(t)
	ctx := context.Background()

	_, err := ws.CreateAssignment(ctx, &pb.CreateAssignmentRequest{ChildId: "no-such-node", ParentId: f.pc})
	assert.Error(t, err)
	_, err = ws.CreateAssignment(ctx, &pb.CreateAssignmentRequest{ChildId: f.pc, ParentId: f.user}) // invalid direction
	assert.Error(t, err)
	_, err = ws.CreateAssociation(ctx, &pb.CreateAssociationRequest{UaId: f.oa, OaId: f.oa, Operations: []string{"read"}}) // source is not a UA
	assert.Error(t, err)
	_, err = ws.RemoveProhibition(ctx, &pb.RemoveProhibitionRequest{Name: "rt-missing-" + uuid.NewString()})
	assert.Error(t, err)
	_, err = ws.CreateProhibition(ctx, &pb.CreateProhibitionRequest{Name: "rt-bad", SubjectId: f.ua}) // no operations
	assert.Error(t, err)

	assert.Empty(t, rec.Events())
}

func TestPermissionEvent_NoneForReadsAndNodeCreation(t *testing.T) {
	ws, f, rec := newRealtimeWriteServer(t)
	ctx := context.Background()
	_, err := ws.GetNode(ctx, &pb.GetNodeRequest{NodeId: f.ua})
	require.NoError(t, err)
	_, err = ws.GetAssociations(ctx, &pb.GetAssociationsRequest{UaId: f.ua})
	require.NoError(t, err)
	n, err := ws.CreateNode(ctx, &pb.CreateNodeRequest{Name: "UA_rtn_" + uuid.NewString()[:8], NodeType: ngac.NodeTypeUserAttribute})
	require.NoError(t, err)
	t.Cleanup(func() { ws.store.DeleteNode(ctx, n.Id) })
	assert.Empty(t, rec.Events(), "an unattached node changes nobody's permissions")
}

func TestPermissionEvent_NoneWhenWorkspaceCannotBeResolved(t *testing.T) {
	ws, _, rec := newRealtimeWriteServer(t)
	ctx := context.Background()
	a, err := ws.store.CreateNode(ctx, "U_rtl_"+uuid.NewString()[:8], ngac.NodeTypeUser, nil)
	require.NoError(t, err)
	b, err := ws.store.CreateNode(ctx, "UA_rtl_"+uuid.NewString()[:8], ngac.NodeTypeUserAttribute, nil)
	require.NoError(t, err)
	t.Cleanup(func() { ws.store.DeleteNode(ctx, a.ID); ws.store.DeleteNode(ctx, b.ID) })

	_, err = ws.CreateAssignment(ctx, &pb.CreateAssignmentRequest{ChildId: a.ID, ParentId: b.ID})
	require.NoError(t, err)
	assert.Empty(t, rec.Events(), "no policy class, no workspace: nothing is invented")
}

func TestBatchUsers(t *testing.T) {
	ids := make([]string, 450)
	for i := range ids {
		ids[i] = fmt.Sprintf("u%d", i)
	}
	batches := batchUsers(ids, 200)
	require.Len(t, batches, 3)
	assert.Len(t, batches[0], 200)
	assert.Len(t, batches[2], 50)
	assert.Empty(t, batchUsers(nil, 200))
}
