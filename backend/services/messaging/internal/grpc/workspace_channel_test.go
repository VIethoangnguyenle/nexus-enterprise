package grpc_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"ngac-platform/pkg/grpcauth"
	pb "ngac-platform/proto/messaging"
	"ngac-platform/services/messaging/internal/domain"
	grpcserver "ngac-platform/services/messaging/internal/grpc"
	"ngac-platform/services/messaging/internal/store"
	"ngac-platform/testutil"
)

// wsChannelWire serves the messaging service behind the policy production uses
// and returns a client dialled as service; "" dials as a plain user would be
// admitted (the caller rides on the context instead).
func wsChannelWire(t *testing.T, service string) (pb.MessagingServiceClient, *wsChannelFixture) {
	t.Helper()
	pool := testPool(t)
	ownerID, ownerNode := testutil.CreateUser(t, pool)
	wsID, _ := testutil.CreateWorkspace(t, pool, ownerID)
	svc := domain.NewService(store.NewStore(pool), &mockPolicyReadWithChannelsOA{wsID: wsID},
		&mockPolicyWriteClient{pool: pool}, &mockAuthClient{}, nil)
	srv := grpcserver.NewMessagingServer(svc, nil, nil)
	conn := testutil.ServeGRPCAs(t, service, grpcserver.ServerPolicy(), func(s *grpc.Server) { pb.RegisterMessagingServiceServer(s, srv) })
	fx := &wsChannelFixture{wsID: wsID, ownerID: ownerID, ownerNode: ownerNode, pool: pool}
	// Registered after the workspace, so it runs first: the channels and the
	// nodes the mock policy wrote for them go before the workspace does.
	t.Cleanup(func() {
		ctx := context.Background()
		rows, err := pool.Query(ctx, `SELECT COALESCE(ngac_oa_id,''), COALESCE(ngac_ua_id,'') FROM channels WHERE workspace_id = $1`, wsID)
		require.NoError(t, err)
		var nodes []string
		for rows.Next() {
			var oa, ua string
			require.NoError(t, rows.Scan(&oa, &ua))
			nodes = append(nodes, oa, ua)
		}
		rows.Close()
		pool.Exec(ctx, `DELETE FROM channels WHERE workspace_id = $1`, wsID)
		for _, id := range nodes {
			pool.Exec(ctx, `DELETE FROM ngac_nodes WHERE id = $1`, id)
		}
	})
	return pb.NewMessagingServiceClient(conn), fx
}

type wsChannelFixture struct {
	wsID, ownerID, ownerNode string
	pool                     *pgxpool.Pool
}

// count is the number of workspace-wide channels the workspace has.
func (f *wsChannelFixture) count() int {
	var n int
	if err := f.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM channels WHERE workspace_id = $1 AND channel_type = 'workspace'`, f.wsID).Scan(&n); err != nil {
		panic(err)
	}
	return n
}

func (f *wsChannelFixture) provision() *pb.ProvisionWorkspaceChannelRequest {
	return &pb.ProvisionWorkspaceChannelRequest{
		WorkspaceId: f.wsID, Name: "general", CreatorUserId: f.ownerID, CreatorNgacNodeId: f.ownerNode,
	}
}

// Only the auth service may make the workspace-wide channel. Every other way to
// ask is refused, and the one allowed way succeeds on the same fixture.
func TestProvisionWorkspaceChannel_OnlyTheAuthServiceMayMakeIt(t *testing.T) {
	t.Run("auth service is allowed", func(t *testing.T) {
		c, fx := wsChannelWire(t, "auth")
		ch, err := c.ProvisionWorkspaceChannel(context.Background(), fx.provision())
		require.NoError(t, err)
		assert.Equal(t, "workspace", ch.ChannelType)
		assert.Equal(t, 1, fx.count())

		_, err = c.ProvisionWorkspaceChannel(context.Background(), fx.provision())
		assert.Equal(t, codes.AlreadyExists, status.Code(err), "a second one is refused")
	})
	for _, other := range []string{"workspace", "drive", "approval"} {
		t.Run("service "+other+" is refused", func(t *testing.T) {
			c, fx := wsChannelWire(t, other)
			_, err := c.ProvisionWorkspaceChannel(context.Background(), fx.provision())
			assert.Equal(t, codes.Unauthenticated, status.Code(err))
			assert.Equal(t, 0, fx.count())
		})
	}
	t.Run("a user is refused, even the workspace's owner", func(t *testing.T) {
		c, fx := wsChannelWire(t, "auth")
		// A person's identity travels on the context; it is not the service's.
		_, err := c.ProvisionWorkspaceChannel(asCaller(fx.ownerID, fx.ownerNode), fx.provision())
		assert.Equal(t, codes.PermissionDenied, status.Code(err))
		assert.Equal(t, 0, fx.count())
	})
}

// The user-scoped CreateChannel cannot make a workspace-wide channel by asking
// for the type; the same wire makes a private group for the same person.
func TestCreateChannel_OverTheWireNeverMakesAWorkspaceWideChannel(t *testing.T) {
	c, fx := wsChannelWire(t, "auth")
	ch, err := c.CreateChannel(asCaller(fx.ownerID, fx.ownerNode), &pb.CreateChannelRequest{
		Name: "sneaky", WorkspaceId: fx.wsID, ChannelType: "workspace",
	})
	require.NoError(t, err)
	assert.Equal(t, "private", ch.ChannelType)
	assert.Equal(t, 0, fx.count())
}

// JoinWorkspaceChannel is for the auth and workspace services; a user, or any
// other service, cannot put anyone anywhere with it.
func TestJoinWorkspaceChannel_OnlyAuthAndWorkspaceServicesMayCallIt(t *testing.T) {
	for _, svc := range []string{"auth", "workspace"} {
		t.Run("service "+svc+" is allowed", func(t *testing.T) {
			c, fx := wsChannelWire(t, svc)
			// A workspace with no workspace-wide channel has nothing to join.
			_, err := c.JoinWorkspaceChannel(context.Background(), &pb.JoinWorkspaceChannelRequest{WorkspaceId: fx.wsID, UserNgacNodeId: fx.ownerNode, UserId: fx.ownerID})
			require.NoError(t, err)
		})
	}
	for _, svc := range []string{"drive", "calendar"} {
		t.Run("service "+svc+" is refused", func(t *testing.T) {
			c, fx := wsChannelWire(t, svc)
			_, err := c.JoinWorkspaceChannel(context.Background(), &pb.JoinWorkspaceChannelRequest{WorkspaceId: fx.wsID, UserNgacNodeId: fx.ownerNode, UserId: fx.ownerID})
			assert.Equal(t, codes.Unauthenticated, status.Code(err))
		})
	}
	t.Run("a user is refused", func(t *testing.T) {
		c, fx := wsChannelWire(t, "auth")
		_, err := c.JoinWorkspaceChannel(asCaller(fx.ownerID, fx.ownerNode), &pb.JoinWorkspaceChannelRequest{WorkspaceId: fx.wsID, UserNgacNodeId: fx.ownerNode, UserId: fx.ownerID})
		assert.Equal(t, codes.PermissionDenied, status.Code(err))
	})
	// Defence in depth: the interceptor admits the services its policy lists; the handler
	// does not trust that list alone. A policy that (by mistake) let another service in
	// still does not get a join.
	t.Run("a service outside auth and workspace is refused by the handler", func(t *testing.T) {
		pool := testPool(t)
		ownerID, ownerNode := testutil.CreateUser(t, pool)
		wsID, _ := testutil.CreateWorkspace(t, pool, ownerID)
		svc := domain.NewService(store.NewStore(pool), &mockPolicyReadWithChannelsOA{wsID: wsID}, &mockPolicyWriteClient{pool: pool}, &mockAuthClient{}, nil)
		loose := grpcauth.ServerPolicy{ServiceOK: map[string]grpcauth.ServiceRule{
			pb.MessagingService_JoinWorkspaceChannel_FullMethodName: grpcauth.ServiceOnly("a mistake", "calendar"),
		}}
		conn := testutil.ServeGRPCAs(t, "calendar", loose, func(s *grpc.Server) {
			pb.RegisterMessagingServiceServer(s, grpcserver.NewMessagingServer(svc, nil, nil))
		})
		_, err := pb.NewMessagingServiceClient(conn).JoinWorkspaceChannel(context.Background(),
			&pb.JoinWorkspaceChannelRequest{WorkspaceId: wsID, UserNgacNodeId: ownerNode, UserId: ownerID})
		assert.Equal(t, codes.PermissionDenied, status.Code(err))
	})
	t.Run("missing ids are an invalid request", func(t *testing.T) {
		c, _ := wsChannelWire(t, "auth")
		_, err := c.JoinWorkspaceChannel(context.Background(), &pb.JoinWorkspaceChannelRequest{})
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})
}
