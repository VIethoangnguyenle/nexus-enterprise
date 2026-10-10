package grpc_test

import (
	"context"
	"testing"

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

// The notification server is a transport: it reads the caller from the verified
// metadata and hands everything to the domain service, whose store runs the SQL.
func TestNotificationServer_ServesTheCallersOwnNotifications(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	alice, aliceNode := testutil.CreateUser(t, pool)
	bob, bobNode := testutil.CreateUser(t, pool)
	svc := domain.NewNotificationService(store.NewStore(pool), nil)
	srv := mgrpc.NewNotificationServer(svc)

	as := func(user, node string) context.Context {
		return grpcauth.WithCaller(context.Background(), grpcauth.Caller{UserID: user, NGACNodeID: node})
	}
	ctx := context.Background()
	require.NoError(t, svc.CreateNotification(ctx, alice, "mention", "one", "", "asset", "a-1"))
	require.NoError(t, svc.CreateNotification(ctx, alice, "mention", "two", "", "", ""))

	list, err := srv.ListNotifications(as(alice, aliceNode), &pb.ListNotificationsRequest{Limit: 10})
	require.NoError(t, err)
	assert.Equal(t, int32(2), list.Total)
	assert.Equal(t, int32(2), list.UnreadCount)
	require.Len(t, list.Notifications, 2)
	assert.Equal(t, alice, list.Notifications[0].UserId)

	other, err := srv.ListNotifications(as(bob, bobNode), &pb.ListNotificationsRequest{})
	require.NoError(t, err)
	assert.Empty(t, other.Notifications, "bob sees none of alice's")

	// Bob cannot mark Alice's notification read.
	_, err = srv.MarkRead(as(bob, bobNode), &pb.MarkNotificationReadRequest{NotificationId: list.Notifications[0].Id})
	require.NoError(t, err)
	count, err := srv.GetUnreadCount(as(alice, aliceNode), &pb.GetNotificationUnreadCountRequest{})
	require.NoError(t, err)
	assert.Equal(t, int32(2), count.Count)

	_, err = srv.MarkRead(as(alice, aliceNode), &pb.MarkNotificationReadRequest{NotificationId: list.Notifications[0].Id})
	require.NoError(t, err)
	count, _ = srv.GetUnreadCount(as(alice, aliceNode), &pb.GetNotificationUnreadCountRequest{})
	assert.Equal(t, int32(1), count.Count)

	_, err = srv.MarkAllRead(as(alice, aliceNode), &pb.MarkAllNotificationsReadRequest{})
	require.NoError(t, err)
	count, _ = srv.GetUnreadCount(as(alice, aliceNode), &pb.GetNotificationUnreadCountRequest{})
	assert.Equal(t, int32(0), count.Count)
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

	ctx := grpcauth.WithCaller(context.Background(), grpcauth.Caller{UserID: "u", NGACNodeID: "n"})
	_, err := srv.ListNotifications(ctx, &pb.ListNotificationsRequest{})
	assert.Equal(t, codes.Internal, status.Code(err))
	assert.Equal(t, "internal error", status.Convert(err).Message())
}
