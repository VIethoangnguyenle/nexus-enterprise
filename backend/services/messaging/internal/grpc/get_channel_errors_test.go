package grpc_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "ngac-platform/proto/messaging"
	"ngac-platform/services/messaging/internal/domain"
	grpcserver "ngac-platform/services/messaging/internal/grpc"
	"ngac-platform/services/messaging/internal/store"
)

// GetChannel tells the three failures apart: no such channel, not allowed, and
// the server could not look.
func TestGetChannel_ErrorCodesFollowTheCause(t *testing.T) {
	t.Run("unknown channel is NotFound", func(t *testing.T) {
		srv, _ := setupTestServer(t)
		_, err := srv.GetChannel(asCaller("", "n"), &pb.GetChannelRequest{ChannelId: "no-such-channel"})
		assert.Equal(t, codes.NotFound, status.Code(err))
	})

	t.Run("denied is PermissionDenied, not NotFound", func(t *testing.T) {
		srv, pool := setupTestServerWithPolicy(t, &mockPolicyReadDeny{})
		wsID := getTestWorkspaceID(t, pool)
		chID := insertTestChannel(t, pool, "deny-code", "workspace", wsID)
		t.Cleanup(func() { cleanTestData(t, pool, chID) })

		_, err := srv.GetChannel(asCaller("", "ngac-outsider"), &pb.GetChannelRequest{ChannelId: chID})
		assert.Equal(t, codes.PermissionDenied, status.Code(err))
	})

	t.Run("a database that cannot answer is Internal and says nothing of why", func(t *testing.T) {
		down, err := pgxpool.New(context.Background(), "postgres://x:x@127.0.0.1:1/x?connect_timeout=1")
		require.NoError(t, err)
		t.Cleanup(down.Close)
		svc := domain.NewService(store.NewStore(down), &mockPolicyReadClient{}, nil, &mockAuthClient{}, nil)
		srv := grpcserver.NewMessagingServer(svc, nil, nil)

		_, err = srv.GetChannel(asCaller("", "n"), &pb.GetChannelRequest{ChannelId: "any"})
		assert.Equal(t, codes.Internal, status.Code(err))
		assert.NotContains(t, status.Convert(err).Message(), "127.0.0.1")
	})
}
