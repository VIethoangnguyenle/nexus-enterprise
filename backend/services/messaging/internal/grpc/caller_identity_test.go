package grpc_test

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"ngac-platform/pkg/grpcauth"
	pb "ngac-platform/proto/messaging"
	policypb "ngac-platform/proto/policy"
	"ngac-platform/testutil"
)

// userGatedPolicy allows an access check only for one user node and records
// whom it was asked about.
type userGatedPolicy struct {
	mockPolicyReadClient
	allowed string
	mu      sync.Mutex
	asked   []string
}

func (g *userGatedPolicy) CheckAccess(_ context.Context, req *policypb.CheckAccessRequest, _ ...grpc.CallOption) (*policypb.AccessDecision, error) {
	g.mu.Lock()
	g.asked = append(g.asked, req.UserNodeId)
	g.mu.Unlock()
	if req.UserNodeId == g.allowed {
		return &policypb.AccessDecision{Decision: "ALLOW"}, nil
	}
	return &policypb.AccessDecision{Decision: "DENY"}, nil
}

func serveMessaging(t *testing.T, policy policypb.PolicyReadServiceClient) (pb.MessagingServiceClient, string) {
	t.Helper()
	srv, pool := setupTestServerWithPolicy(t, policy)
	wsID := getTestWorkspaceID(t, pool)
	chID := insertTestChannel(t, pool, "wireid", "workspace", wsID)
	t.Cleanup(func() { cleanTestData(t, pool, chID) })
	conn := testutil.ServeGRPC(t, grpcauth.ServerPolicy{}, func(s *grpc.Server) { pb.RegisterMessagingServiceServer(s, srv) })
	return pb.NewMessagingServiceClient(conn), chID
}

func TestOverTheWire_MissingCallerIsUnauthenticated(t *testing.T) {
	c, chID := serveMessaging(t, &mockPolicyReadClient{})

	_, err := c.GetChannel(context.Background(), &pb.GetChannelRequest{ChannelId: chID})
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
	_, err = c.ListDMs(context.Background(), &pb.ListDMsRequest{})
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
	_, err = c.SendMessage(context.Background(), &pb.SendMessageRequest{ChannelId: chID, Content: "x"})
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}

// The body claims to be the member; the metadata is an outsider. Reads are
// decided for the outsider.
func TestOverTheWire_BodyCallerIsIgnored_Deny(t *testing.T) {
	policy := &userGatedPolicy{allowed: "ngac-member"}
	c, chID := serveMessaging(t, policy)

	_, err := c.ListChannelMembers(asCaller("u-out", "ngac-outsider"), &pb.ListChannelMembersRequest{
		ChannelId:      chID,
		UserNgacNodeId: "ngac-member", //lint:ignore SA1019 proves the deprecated body field is not trusted
	})

	require.Error(t, err)
	assert.NotContains(t, policy.asked, "ngac-member", "the body's user must never reach the policy check")
	assert.Contains(t, policy.asked, "ngac-outsider")
}

func TestOverTheWire_MetadataCallerDecides_Allow(t *testing.T) {
	policy := &userGatedPolicy{allowed: "ngac-member"}
	c, chID := serveMessaging(t, policy)

	_, err := c.ListChannelMembers(asCaller("u-member", "ngac-member"), &pb.ListChannelMembersRequest{
		ChannelId:      chID,
		UserNgacNodeId: "ngac-outsider", //lint:ignore SA1019 proves the deprecated body field is not trusted
	})

	require.NoError(t, err)
	assert.NotContains(t, policy.asked, "ngac-outsider")
}

// A channel member cannot be added on a requester the body names.
func TestOverTheWire_AddMemberRequesterIsMetadata(t *testing.T) {
	policy := &userGatedPolicy{allowed: "ngac-member"}
	c, chID := serveMessaging(t, policy)

	_, err := c.AddChannelMember(asCaller("u-out", "ngac-outsider"), &pb.AddChannelMemberRequest{
		ChannelId:           chID,
		RequesterNgacNodeId: "ngac-member", //lint:ignore SA1019 proves the deprecated body field is not trusted
		TargetNgacNodeId:    "ngac-victim",
	})

	require.Error(t, err)
	assert.NotContains(t, policy.asked, "ngac-member")
}
