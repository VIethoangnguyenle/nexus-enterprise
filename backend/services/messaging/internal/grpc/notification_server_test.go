package grpc_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"ngac-platform/pkg/grpcauth"
	pb "ngac-platform/proto/messaging"
	"ngac-platform/services/messaging/internal/domain"
	mgrpc "ngac-platform/services/messaging/internal/grpc"
	"ngac-platform/services/messaging/internal/store"
	"ngac-platform/testutil"
)

func addMember(t *testing.T, pool *pgxpool.Pool, ws, user, node string) {
	t.Helper()
	_, err := pool.Exec(context.Background(),
		`INSERT INTO tenant_users (tenant_id, user_id, role, status, ngac_node_id) VALUES ($1, $2, 'member', 'active', $3)`, ws, user, node)
	require.NoError(t, err)
}

// The notification server is a transport: it reads the caller and workspace from
// the verified metadata and hands everything to the domain service.
func TestNotificationServer_ServesTheCallersOwnNotificationsInTheirWorkspace(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	alice, aliceNode := testutil.CreateUser(t, pool)
	bob, bobNode := testutil.CreateUser(t, pool)
	wsA, _ := testutil.CreateWorkspace(t, pool, alice)
	wsB, _ := testutil.CreateWorkspace(t, pool, alice)
	for _, ws := range []string{wsA, wsB} {
		addMember(t, pool, ws, alice, aliceNode)
		addMember(t, pool, ws, bob, bobNode)
	}
	svc := domain.NewNotificationService(store.NewStore(pool), nil)
	srv := mgrpc.NewNotificationServer(svc)

	as := func(user, node, ws string) context.Context {
		return grpcauth.WithCaller(context.Background(), grpcauth.Caller{UserID: user, NGACNodeID: node, TenantID: ws})
	}
	ctx := context.Background()
	raise := func(ws, to, name string) {
		require.NoError(t, svc.CreateNotification(ctx, domain.NewNotification{
			WorkspaceID: ws, Type: "asset_assigned", Recipient: to, Actor: bob, TargetType: "asset", TargetID: "a-1", TargetName: name,
		}))
	}
	raise(wsA, alice, "one")
	raise(wsA, alice, "two")
	raise(wsB, alice, "elsewhere")

	list, err := srv.ListNotifications(as(alice, aliceNode, wsA), &pb.ListNotificationsRequest{Limit: 10})
	require.NoError(t, err)
	assert.Equal(t, int32(2), list.Total, "the other workspace's notification is not counted")
	assert.Equal(t, int32(2), list.UnreadCount)
	require.Len(t, list.Notifications, 2)
	assert.Equal(t, alice, list.Notifications[0].UserId)
	assert.Equal(t, bob, list.Notifications[0].ActorUserId)
	assert.Equal(t, wsA, list.Notifications[0].WorkspaceId)

	other, err := srv.ListNotifications(as(bob, bobNode, wsA), &pb.ListNotificationsRequest{})
	require.NoError(t, err)
	assert.Empty(t, other.Notifications, "bob sees none of alice's")

	// Bob cannot mark Alice's notification read.
	_, err = srv.MarkRead(as(bob, bobNode, wsA), &pb.MarkNotificationReadRequest{NotificationId: list.Notifications[0].Id})
	require.NoError(t, err)
	count, err := srv.GetUnreadCount(as(alice, aliceNode, wsA), &pb.GetNotificationUnreadCountRequest{})
	require.NoError(t, err)
	assert.Equal(t, int32(2), count.Count)

	// Nor can Alice mark it from the other workspace.
	_, err = srv.MarkRead(as(alice, aliceNode, wsB), &pb.MarkNotificationReadRequest{NotificationId: list.Notifications[0].Id})
	require.NoError(t, err)
	count, _ = srv.GetUnreadCount(as(alice, aliceNode, wsA), &pb.GetNotificationUnreadCountRequest{})
	assert.Equal(t, int32(2), count.Count, "a notification is marked only from its own workspace")

	_, err = srv.MarkRead(as(alice, aliceNode, wsA), &pb.MarkNotificationReadRequest{NotificationId: list.Notifications[0].Id})
	require.NoError(t, err)
	count, _ = srv.GetUnreadCount(as(alice, aliceNode, wsA), &pb.GetNotificationUnreadCountRequest{})
	assert.Equal(t, int32(1), count.Count)

	_, err = srv.MarkAllRead(as(alice, aliceNode, wsA), &pb.MarkAllNotificationsReadRequest{})
	require.NoError(t, err)
	count, _ = srv.GetUnreadCount(as(alice, aliceNode, wsA), &pb.GetNotificationUnreadCountRequest{})
	assert.Equal(t, int32(0), count.Count)
	count, _ = srv.GetUnreadCount(as(alice, aliceNode, wsB), &pb.GetNotificationUnreadCountRequest{})
	assert.Equal(t, int32(1), count.Count, "mark-all leaves the other workspace unread")
}

func TestNotificationServer_NoCallerIsDeniedNotServedEmpty(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	srv := mgrpc.NewNotificationServer(domain.NewNotificationService(store.NewStore(pool), nil))

	_, err := srv.ListNotifications(context.Background(), &pb.ListNotificationsRequest{})
	assert.Equal(t, codes.PermissionDenied, status.Code(err))
	_, err = srv.GetUnreadCount(context.Background(), &pb.GetNotificationUnreadCountRequest{})
	assert.Equal(t, codes.PermissionDenied, status.Code(err))
}

// A closed database is a failure of ours; the caller learns nothing about it.
func TestNotificationServer_StoreFailureIsAGenericInternal(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	st := store.NewStore(pool)
	pool.Close()
	srv := mgrpc.NewNotificationServer(domain.NewNotificationService(st, nil))

	ctx := grpcauth.WithCaller(context.Background(), grpcauth.Caller{UserID: "u", NGACNodeID: "n", TenantID: "w"})
	_, err := srv.ListNotifications(ctx, &pb.ListNotificationsRequest{})
	assert.Equal(t, codes.Internal, status.Code(err))
	assert.Equal(t, "internal error", status.Convert(err).Message())
}
