// Package domain contains the business logic for the messaging service.
// It orchestrates between the store (database), NGAC policy (access control),
// and external services (auth, drive). No SQL or protobuf lives here.
package domain

import (
	"context"
	"fmt"
	"log/slog"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"ngac-platform/ngac"
	"ngac-platform/pkg/grpcauth"
	"ngac-platform/pkg/policyclient"
	authpb "ngac-platform/proto/auth"
	pb "ngac-platform/proto/messaging"
	"ngac-platform/services/messaging/internal/store"
)

// ListDMs returns DM channels the user has access to.
func (s *Service) ListDMs(ctx context.Context, userNodeID string) ([]*pb.Channel, error) {
	channels, err := s.store.ListAllDMs(ctx)
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

// FindOrCreateDM finds an existing DM or creates a new one.
//
// The other person is named by user id alone: their node is read from the auth
// service, never taken from the client, who may send one only to confirm it. The
// node must be a person's and share a workspace with the caller, and that is
// settled before any lookup, so asking for a DM answers nothing about who
// already talks to whom.
func (s *Service) FindOrCreateDM(ctx context.Context, userID, userNodeID, targetUserID, targetNodeID string) (*pb.Channel, error) {
	if targetUserID == "" {
		return nil, fmt.Errorf("%w: a direct message needs a person", ErrInvalidInput)
	}
	if targetUserID == userID {
		return nil, fmt.Errorf("%w: a direct message is with someone else", ErrInvalidInput)
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
	// has workspaces, so a UA or any other node shares none.
	shared, err := policyclient.SharedWorkspaces(ctx, s.policyWrite, userNodeID, node)
	if err != nil {
		return nil, fmt.Errorf("check DM relationship: %w", err)
	}
	if len(shared) == 0 {
		return nil, fmt.Errorf("%w: you share no workspace with that person", ErrAccessDenied)
	}

	existing, err := s.store.FindDMByMembers(ctx, userNodeID, node)
	if err == nil && existing != nil {
		return channelToProto(existing), nil
	}

	// The DM's title reaches the screen, so it is built from display names.
	// The previous form spliced two truncated user IDs together, which both
	// put raw identifiers in front of the user and panicked on any ID shorter
	// than eight characters.
	// A DM belongs to no workspace, so there is no Channels OA to check
	// create_channel against; it is created directly under PC_Global.
	// The other participant joins the Members UA inside the same provisioning,
	// so a failure to add them removes the whole DM rather than leaving a
	// one-person DM behind.
	ch, err := s.createChannel(ctx, CreateChannelInput{
		Name:        ngac.DMChannelName(s.lookupUsername(ctx, userID), target.GetUsername()),
		ChannelType: ChannelTypeDM,
		UserID:      userID,
		UserNodeID:  userNodeID,
	}, "", "", node)
	if err != nil {
		return nil, fmt.Errorf("create DM channel: %w", err)
	}
	// Both people have a new conversation, whichever workspace they look from.
	s.BumpChat(grpcauth.CallerFrom(ctx).TenantID, userID, targetUserID)

	return ch, nil
}
