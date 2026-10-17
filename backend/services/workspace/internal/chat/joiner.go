// Package chat is the workspace service's client of the messaging service.
package chat

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"ngac-platform/pkg/grpcauth"
	messagingpb "ngac-platform/proto/messaging"
)

// callTimeout bounds the call: a messaging service that never answers must not hold
// an invitation being accepted.
const callTimeout = 5 * time.Second

// Joiner puts people into a workspace's #general by calling the messaging service.
type Joiner struct {
	client messagingpb.MessagingServiceClient
}

// NewJoiner returns a Joiner over a connection to the messaging service.
func NewJoiner(c messagingpb.MessagingServiceClient) *Joiner { return &Joiner{client: c} }

// JoinWorkspaceChannel asks messaging to put the member into the workspace's
// workspace-wide channel. The call goes out on this service's own identity, not the
// signed-in person's on ctx: the person being admitted is not the one deciding, and
// messaging refuses a person's identity on this method. A messaging service that
// predates the method is not an error: the member is joined when they next list
// their channels.
func (j *Joiner) JoinWorkspaceChannel(ctx context.Context, workspaceID, userID, userNodeID string) error {
	ctx, cancel := context.WithTimeout(grpcauth.WithCaller(ctx, grpcauth.Caller{}), callTimeout)
	defer cancel()
	_, err := j.client.JoinWorkspaceChannel(ctx, &messagingpb.JoinWorkspaceChannelRequest{
		WorkspaceId: workspaceID, UserNgacNodeId: userNodeID, UserId: userID,
	})
	if status.Code(err) == codes.Unimplemented {
		slog.Warn("messaging predates JoinWorkspaceChannel; the member joins #general when they next list channels", "workspace", workspaceID)
		return nil
	}
	if err != nil {
		return fmt.Errorf("join workspace channel: %w", err)
	}
	return nil
}
