// Package domain contains the business logic for the messaging service.
// It orchestrates between the store (database), NGAC policy (access control),
// and external services (auth, drive). No SQL or protobuf lives here.
package domain

import (
	"context"
	"fmt"
	"log/slog"

	"ngac-platform/ngac"
	"ngac-platform/pkg/policyclient"
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
func (s *Service) FindOrCreateDM(ctx context.Context, userID, userNodeID, targetUserID, targetNodeID string) (*pb.Channel, error) {
	existing, err := s.store.FindDMByMembers(ctx, userNodeID, targetNodeID)
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
		Name:        ngac.DMChannelName(s.lookupUsername(ctx, userID), s.lookupUsername(ctx, targetUserID)),
		ChannelType: "dm",
		UserID:      userID,
		UserNodeID:  userNodeID,
	}, "", "", targetNodeID)
	if err != nil {
		return nil, fmt.Errorf("create DM channel: %w", err)
	}

	return ch, nil
}
