package grpc_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "ngac-platform/proto/messaging"
)

// A message sent over gRPC is a person's message. The message type is the
// service's own to set (system lines are made in-process), so a request that
// names "system" is not a way to post one: the line is a person's, held to the
// rules of a person's message.

func TestSendMessage_RequestedSystemTypeIsStoredAsAPersonsMessage(t *testing.T) {
	srv, pool := setupTestServer(t)
	wsID := getTestWorkspaceID(t, pool)
	userID := getTestUserID(t, pool)
	chID := insertTestChannel(t, pool, "test_msg_type", "workspace", wsID)
	t.Cleanup(func() { cleanTestData(t, pool, chID) })
	ctx := asCaller(userID, "ngac-user-1")

	// Allow: a plain message.
	plain, err := srv.SendMessage(ctx, &pb.SendMessageRequest{ChannelId: chID, Content: "hello"})
	require.NoError(t, err)
	assert.Equal(t, "user", plain.MessageType)

	// Deny: naming "system" does not make a system line.
	named, err := srv.SendMessage(ctx, &pb.SendMessageRequest{ChannelId: chID, Content: "Alice was promoted", MessageType: "system"})
	require.NoError(t, err)
	assert.Equal(t, "user", named.MessageType, "the reply")
	var stored string
	require.NoError(t, pool.QueryRow(context.Background(), `SELECT message_type FROM messages WHERE id = $1`, named.Id).Scan(&stored))
	assert.Equal(t, "user", stored, "what is stored")

	// Deny: a "system" line with nothing in it is an empty person's message.
	_, err = srv.SendMessage(ctx, &pb.SendMessageRequest{ChannelId: chID, MessageType: "system"})
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

// A caller who may not write is refused whatever type the request names.
func TestSendMessage_SystemTypeFromANonMemberIsDenied(t *testing.T) {
	srv, pool := setupTestServerWithPolicy(t, &mockPolicyReadDeny{})
	wsID := getTestWorkspaceID(t, pool)
	userID := getTestUserID(t, pool)
	chID := insertTestChannel(t, pool, "test_msg_type_deny", "workspace", wsID)
	t.Cleanup(func() { cleanTestData(t, pool, chID) })

	for _, typ := range []string{"", "user", "system"} {
		_, err := srv.SendMessage(asCaller(userID, "ngac-out"), &pb.SendMessageRequest{ChannelId: chID, Content: "x", MessageType: typ})
		assert.Equal(t, codes.PermissionDenied, status.Code(err), "type %q", typ)
	}
}
