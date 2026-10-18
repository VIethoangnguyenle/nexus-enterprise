// Package domain contains the business logic for the messaging service.
// It orchestrates between the store (database), NGAC policy (access control),
// and external services (auth, drive). No SQL or protobuf lives here.
package domain

import (
	"context"
	"fmt"
	"log/slog"
	"slices"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"ngac-platform/ngac"
	"ngac-platform/pkg/grpcauth"
	"ngac-platform/pkg/policyclient"
	authpb "ngac-platform/proto/auth"
	pb "ngac-platform/proto/messaging"
	"ngac-platform/services/messaging/internal/store"
)

// ListDMs returns the DM channels of the workspace that the user has access to.
// A DM belongs to one workspace, so the DMs of any other are not listed. Only the
// DMs the member cache records the user in are put to the policy service; the
// graph then decides which of them are readable, so a stale cache can hide a DM
// but never show one.
func (s *Service) ListDMs(ctx context.Context, workspaceID, userNodeID string) ([]*pb.Channel, error) {
	channels, err := s.store.ListDMsOf(ctx, workspaceID, userNodeID)
	if err != nil {
		return nil, fmt.Errorf("list DMs: %w", err)
	}
	return s.filterAccessible(ctx, channels, userNodeID), nil
}

// filterAccessible checks NGAC read access for each channel and returns only allowed ones.
func (s *Service) filterAccessible(ctx context.Context, channels []*store.Channel, userNodeID string) []*pb.Channel {
	if len(channels) == 0 {
		return nil
	}

	// One batch call for the whole list. A user in fifty channels otherwise
	// paid fifty sequential policy round-trips every time the sidebar loaded.
	objectIDs := make([]string, 0, len(channels))
	for _, ch := range channels {
		objectIDs = append(objectIDs, ch.NGACOaID)
	}
	batch, err := policyclient.New(s.policyRead).BatchCheck(ctx, userNodeID, objectIDs, []string{ngac.OpRead})
	if err != nil {
		// Fail closed: showing every channel because the policy service is
		// unreachable is the one outcome worse than showing none.
		slog.Error("batch access check failed; listing no channels", "error", err)
		return nil
	}

	var result []*pb.Channel
	for _, ch := range channels {
		if batch.Has(ch.NGACOaID, ngac.OpRead) {
			result = append(result, channelToProto(ch))
		}
	}
	return result
}

// FindOrCreateDM finds the DM the caller and another person have in the
// workspace the caller is signed in to, or creates it. A DM belongs to exactly
// one workspace, so the same two people have one for each workspace they share.
//
// The other person is named by user id alone: their node is read from the auth
// service, never taken from the client, who may send one only to confirm it. The
// node must be a person's, and both people must be in the caller's workspace;
// that is settled before any lookup, so asking for a DM answers nothing about who
// already talks to whom.
func (s *Service) FindOrCreateDM(ctx context.Context, userID, userNodeID, targetUserID, targetNodeID string) (*pb.Channel, error) {
	if targetUserID == "" {
		return nil, fmt.Errorf("%w: a direct message needs a person", ErrInvalidInput)
	}
	if targetUserID == userID {
		return nil, fmt.Errorf("%w: a direct message is with someone else", ErrInvalidInput)
	}
	workspaceID := grpcauth.CallerFrom(ctx).TenantID
	if workspaceID == "" {
		return nil, fmt.Errorf("%w: a direct message is started inside a workspace", ErrAccessDenied)
	}
	target, err := s.authClient.GetUserByID(ctx, &authpb.GetUserByIDRequest{UserId: targetUserID})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, fmt.Errorf("%w: person", ErrNotFound)
		}
		return nil, fmt.Errorf("resolve DM target: %w", err)
	}
	node := target.GetNgacNodeId()
	if node == "" {
		return nil, fmt.Errorf("%w: person", ErrNotFound)
	}
	if targetNodeID != "" && targetNodeID != node {
		return nil, fmt.Errorf("%w: the node does not belong to that person", ErrInvalidInput)
	}
	if node == userNodeID {
		return nil, fmt.Errorf("%w: a direct message is with someone else", ErrInvalidInput)
	}
	// Asked of the writer: a person removed a moment ago must not be reachable
	// because a replica still shows them as a colleague. Only a person (a U node)
	// has workspaces, so a UA or any other node shares none. The session's own
	// workspace must be among those both are in, which also keeps a caller from
	// naming a workspace they are not in.
	shared, err := policyclient.SharedWorkspaces(ctx, s.policyWrite, userNodeID, node)
	if err != nil {
		return nil, fmt.Errorf("check DM relationship: %w", err)
	}
	if !slices.Contains(shared, ngac.WorkspaceID(workspaceID)) {
		return nil, fmt.Errorf("%w: you and that person are not both in this workspace", ErrAccessDenied)
	}

	existing, err := s.store.FindDMByMembers(ctx, workspaceID, userNodeID, node)
	if err == nil && existing != nil {
		return channelToProto(existing), nil
	}

	// The DM's title reaches the screen, so it is built from display names.
	// The previous form spliced two truncated user IDs together, which both
	// put raw identifiers in front of the user and panicked on any ID shorter
	// than eight characters.
	// The row belongs to the workspace, but its graph nodes do not: there is no
	// Channels OA to check create_channel against, so the DM is created directly
	// under PC_Global, exactly as before DMs had a workspace.
	// The other participant joins the Members UA inside the same provisioning,
	// so a failure to add them removes the whole DM rather than leaving a
	// one-person DM behind.
	ch, err := s.createChannel(ctx, CreateChannelInput{
		Name:        ngac.DMChannelName(s.lookupUsername(ctx, userID), target.GetUsername()),
		ChannelType: ChannelTypeDM,
		WorkspaceID: workspaceID,
		UserID:      userID,
		UserNodeID:  userNodeID,
	}, "", "", node)
	if err != nil {
		return nil, fmt.Errorf("create DM channel: %w", err)
	}
	// Both people have a new conversation in this workspace.
	s.BumpChat(workspaceID, userID, targetUserID)

	return ch, nil
}
