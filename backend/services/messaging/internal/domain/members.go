// Package domain contains the business logic for the messaging service.
// It orchestrates between the store (database), NGAC policy (access control),
// and external services (auth, drive). No SQL or protobuf lives here.
package domain

import (
	"context"
	"fmt"
	"log/slog"

	"ngac-platform/ngac"
	"ngac-platform/pkg/realtime"
	authpb "ngac-platform/proto/auth"
	pb "ngac-platform/proto/messaging"
	policypb "ngac-platform/proto/policy"
	"ngac-platform/services/messaging/internal/store"
)

// AddMember adds a user to a channel's NGAC members UA.
//
// Membership is a graph mutation, so it takes OpInvite on the channel — the
// operation the vocabulary reserves for exactly this. Workspace owners hold it
// through the Channels OA; plain channel members hold only read/write and
// therefore cannot pull other people into a channel.
func (s *Service) AddMember(ctx context.Context, channelID, requesterNodeID, targetNodeID string) error {
	ch, err := s.authorizeChannel(ctx, channelID, requesterNodeID, ngac.OpInvite)
	if err != nil {
		return err
	}
	// A DM is a fixed two-party conversation. Its participants hold invite on
	// its content OA like any channel member, so the graph alone would let one
	// of them pull in a third person and silently turn a private exchange into
	// a group. Widening a DM has to be an explicit act, not a side effect of
	// AddMember.
	if ch.ChannelType == "dm" {
		return fmt.Errorf("%w: cannot add members to a direct message", ErrInvalidInput)
	}
	// CreateAssignment is idempotent, so an edge that was already there is
	// indistinguishable afterwards from one this call made. Ask first: only an
	// edge this call creates may be withdrawn again. If the question cannot be
	// answered, assume the edge may be older and never withdraw it.
	existed := true
	if res, err := s.policyRead.IsAssigned(ctx, &policypb.IsAssignedRequest{ChildId: targetNodeID, ParentId: ch.NGACUaID}); err == nil {
		existed = res.GetValue()
	}
	if _, err := s.policyWrite.CreateAssignment(ctx, &policypb.CreateAssignmentRequest{
		ChildId: targetNodeID, ParentId: ch.NGACUaID,
	}); err != nil {
		return fmt.Errorf("add member: %w", err)
	}
	// The graph is the membership; channel_members is the cache DM lookup reads.
	// If the cache cannot be written, an assignment this call made is withdrawn,
	// so the two never disagree about who is in the channel.
	if err := s.members.InsertChannelMember(ctx, channelID, targetNodeID); err != nil {
		if existed {
			return fmt.Errorf("add member: %w", err)
		}
		if _, rerr := s.policyWrite.RemoveAssignment(ctx, &policypb.RemoveAssignmentRequest{
			ChildId: targetNodeID, ParentId: ch.NGACUaID,
		}); rerr != nil {
			slog.Error("member added to the graph but not recorded, and the assignment could not be withdrawn",
				"channel", channelID, "member", targetNodeID, "record_error", err, "withdraw_error", rerr)
			return fmt.Errorf("add member: %w; withdrawing the assignment also failed (%v), so the graph and the cache disagree", err, rerr)
		}
		return fmt.Errorf("add member: %w", err)
	}
	s.announceMembership(ctx, realtime.KindMemberAdded, ch, targetNodeID)
	return nil
}

// announceMembership tells the channel's subscribers that its roster changed,
// and tells the person concerned directly: someone just added is not
// subscribed yet, and someone just removed no longer is.
func (s *Service) announceMembership(ctx context.Context, kind string, ch *store.Channel, targetNodeID string) {
	roster := channelEvent(ctx, kind, ch)
	roster.ChannelID = ch.ID
	s.emit(roster)

	person := channelEvent(ctx, kind, ch)
	person.ChannelID = ch.ID
	person.UserNodeIDs = []string{targetNodeID}
	s.emit(person)
}

// RemoveMember removes a user from a channel's NGAC members UA.
func (s *Service) RemoveMember(ctx context.Context, channelID, requesterNodeID, targetNodeID string) error {
	ch, err := s.authorizeChannel(ctx, channelID, requesterNodeID, ngac.OpInvite)
	if err != nil {
		return err
	}
	if _, err := s.policyWrite.RemoveAssignment(ctx, &policypb.RemoveAssignmentRequest{
		ChildId: targetNodeID, ParentId: ch.NGACUaID,
	}); err != nil {
		return fmt.Errorf("remove member: %w", err)
	}
	// Losing membership must also end the live feed. The subscription was
	// authorized when it was opened; without this the removed user keeps
	// receiving every new message for as long as the socket stays up.
	if s.revoker != nil {
		s.revoker.RevokeChannelSubscriptions(channelID, targetNodeID)
	}
	s.announceMembership(ctx, realtime.KindMemberRemoved, ch, targetNodeID)
	return nil
}

// ListMembers returns the members of a channel via NGAC graph traversal.
func (s *Service) ListMembers(ctx context.Context, channelID, userNodeID string) ([]*pb.ChannelMember, error) {
	ch, err := s.authorizeChannel(ctx, channelID, userNodeID, ngac.OpRead)
	if err != nil {
		return nil, err
	}

	children, err := s.policyRead.GetChildren(ctx, &policypb.GetChildrenRequest{NodeId: ch.NGACUaID})
	if err != nil {
		return nil, fmt.Errorf("list channel members: %w", err)
	}
	if children == nil {
		return nil, nil
	}

	var members []*pb.ChannelMember
	for _, n := range children.Nodes {
		if n.NodeType != ngac.TypeU {
			continue
		}
		username, userID := ngac.DisplayName(n.Name, n.Properties), ""
		user, uerr := s.authClient.GetUserByNGACNodeID(ctx, &authpb.GetUserByNGACNodeIDRequest{NgacNodeId: n.Id})
		if uerr != nil {
			// The member is still listed, under the name the graph holds.
			slog.Warn("member name lookup failed", "channel", channelID, "node", n.Id, "error", uerr)
		}
		if user != nil {
			username = user.Username
			userID = user.Id
		}
		members = append(members, &pb.ChannelMember{
			UserId: userID, Username: username, NgacNodeId: n.Id,
		})
	}
	return members, nil
}
