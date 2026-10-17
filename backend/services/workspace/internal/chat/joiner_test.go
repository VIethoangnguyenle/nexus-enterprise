package chat_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"ngac-platform/pkg/grpcauth"
	messagingpb "ngac-platform/proto/messaging"
	"ngac-platform/services/workspace/internal/chat"
	"ngac-platform/testutil"
)

// spyMessaging records the identity each JoinWorkspaceChannel arrived on.
type spyMessaging struct {
	messagingpb.UnimplementedMessagingServiceServer
	service string
	caller  grpcauth.Caller
	req     *messagingpb.JoinWorkspaceChannelRequest
	left    time.Duration // time left on the request's deadline; 0: none
	err     error
}

func (s *spyMessaging) JoinWorkspaceChannel(ctx context.Context, r *messagingpb.JoinWorkspaceChannelRequest) (*messagingpb.Empty, error) {
	s.service, s.caller, s.req = grpcauth.ServiceFrom(ctx), grpcauth.CallerFrom(ctx), r
	if d, ok := ctx.Deadline(); ok {
		s.left = time.Until(d)
	}
	if s.err != nil {
		return nil, s.err
	}
	return &messagingpb.Empty{}, nil
}

func policy() grpcauth.ServerPolicy {
	return grpcauth.ServerPolicy{ServiceOK: map[string]grpcauth.ServiceRule{
		messagingpb.MessagingService_JoinWorkspaceChannel_FullMethodName: grpcauth.ServiceOnly("test", "workspace"),
	}}
}

// The join is made as the workspace service even when the request that led to it
// carries the person accepting an invitation: the policy admits only a service
// there, and a person's token would be refused.
func TestJoiner_GoesOutAsTheServiceEvenWithAPersonOnTheContext(t *testing.T) {
	spy := &spyMessaging{}
	conn := testutil.ServeGRPCAs(t, "workspace", policy(), func(s *grpc.Server) { messagingpb.RegisterMessagingServiceServer(s, spy) })
	j := chat.NewJoiner(messagingpb.NewMessagingServiceClient(conn))
	withPerson := grpcauth.WithCaller(context.Background(), grpcauth.Caller{UserID: "u1", NGACNodeID: "n1"})

	require.NoError(t, j.JoinWorkspaceChannel(withPerson, "ws-1", "user-9", "node-9"))

	assert.Equal(t, "workspace", spy.service)
	assert.Equal(t, grpcauth.Caller{}, spy.caller)
	assert.Equal(t, "ws-1", spy.req.WorkspaceId)
	assert.Equal(t, "node-9", spy.req.UserNgacNodeId)
	assert.Equal(t, "user-9", spy.req.UserId)
	assert.True(t, spy.left > 0 && spy.left <= 10*time.Second, "the call is bounded in time, had %v", spy.left)
}

// A messaging service that predates the method (a rolling deploy) is not a failure of
// the admission: the join is skipped with a warning and happens when the member next
// lists their channels. Any other failure is reported to the caller, which logs it.
func TestJoiner_AnOldMessagingServiceIsNotAnError(t *testing.T) {
	spy := &spyMessaging{err: status.Error(codes.Unimplemented, "unknown method")}
	conn := testutil.ServeGRPCAs(t, "workspace", policy(), func(s *grpc.Server) { messagingpb.RegisterMessagingServiceServer(s, spy) })
	j := chat.NewJoiner(messagingpb.NewMessagingServiceClient(conn))
	require.NoError(t, j.JoinWorkspaceChannel(context.Background(), "ws-1", "user-9", "node-9"))

	spy.err = status.Error(codes.Unavailable, "down")
	require.Error(t, j.JoinWorkspaceChannel(context.Background(), "ws-1", "user-9", "node-9"))
}

// Another service's identity is not admitted by the same policy, so the join is only
// as good as the identity it carries.
func TestJoiner_AServiceNotInThePolicyIsRefused(t *testing.T) {
	spy := &spyMessaging{}
	conn := testutil.ServeGRPCAs(t, "drive", policy(), func(s *grpc.Server) { messagingpb.RegisterMessagingServiceServer(s, spy) })
	j := chat.NewJoiner(messagingpb.NewMessagingServiceClient(conn))

	err := j.JoinWorkspaceChannel(context.Background(), "ws-1", "user-9", "node-9")

	require.Error(t, err)
	assert.Nil(t, spy.req)
}
