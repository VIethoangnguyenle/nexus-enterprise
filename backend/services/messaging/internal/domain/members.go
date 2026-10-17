// Package domain contains the business logic for the messaging service.
// It orchestrates between the store (database), NGAC policy (access control),
// and external services (auth, drive). No SQL or protobuf lives here.
package domain

import (
	"context"
	"fmt"
	"log/slog"

	"ngac-platform/ngac"
	"ngac-platform/pkg/grpcauth"
	"ngac-platform/pkg/policyclient"
	"ngac-platform/pkg/realtime"
	authpb "ngac-platform/proto/auth"
	pb "ngac-platform/proto/messaging"
	policypb "ngac-platform/proto/policy"
	"ngac-platform/services/messaging/internal/store"
)

// AddMember adds a user to a channel's NGAC members UA.
//
// Membership is a graph mutation, so it takes OpInvite on the channel — the
// operation the vocabulary reserves for exactly this. A group is private, so only
// its own members hold it (ChannelMemberOps): a workspace owner who is not in the
// group holds manage there, not invite, and a member of the workspace who is not
// in it holds nothing.
//
// The channel's Members UA sits under the workspace's policy class (the
// intersection rule needs it there), so whoever is put under it reaches that class.
// The target is therefore held to the workspace's own standard: a user node that is
// already under the workspace's Members UA. Without that, any group member could
// make anyone, or a whole UA, a member of the workspace, or undo a removal.
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
	if err := s.requireAddableMember(ctx, ch, targetNodeID); err != nil {
		return err
	}
	// CreateAssignment is idempotent, so an edge that was already there is
	// indistinguishable afterwards from one this call made. Ask first: only an
	// edge this call creates may be withdrawn again. If the question cannot be
	// answered, assume the edge may be older and never withdraw it.
	existed := true
	if res, err := s.policyWrite.IsAssigned(ctx, &policypb.IsAssignedRequest{ChildId: targetNodeID, ParentId: ch.NGACUaID}); err == nil {
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

// requireAddableMember asks the writer, not the read replica: a removal made a
// moment ago must be seen, or the removed person could be put back. It refuses a target who is not a user node, or is a user who
// does not belong to the channel's workspace. A policy service that cannot answer
// denies: the answer is unknown, and a group edge is hard to take back once other
// services have acted on it.
func (s *Service) requireAddableMember(ctx context.Context, ch *store.Channel, targetNodeID string) error {
	if targetNodeID == "" {
		return fmt.Errorf("%w: a person is required", ErrInvalidInput)
	}
	n, err := s.policyWrite.GetNode(ctx, &policypb.GetNodeRequest{NodeId: targetNodeID})
	if err != nil {
		if policyclient.IsNotFound(err) {
			return fmt.Errorf("%w: not a person", ErrInvalidInput)
		}
		return fmt.Errorf("add member: resolve target: %w", err)
	}
	if n.GetNodeType() != ngac.TypeU {
		return fmt.Errorf("%w: not a person", ErrInvalidInput)
	}
	member, err := policyclient.IsWorkspaceMember(ctx, s.policyWrite, targetNodeID, ngac.WorkspaceID(ch.WorkspaceID))
	if err != nil {
		return fmt.Errorf("add member: check workspace membership: %w", err)
	}
	if !member {
		return fmt.Errorf("%w: not a member of this workspace", ErrAccessDenied)
	}
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

// RemoveMember takes a person out of a group's Members UA.
//
// Invite, which every member holds, is what lets people in; it does not let them
// put others out. Removal is for the person themselves (leaving), for the person
// who made the group, and for a holder of manage on it (the workspace's owners).
// A direct message is a fixed pair and #general's people are the workspace's, so
// neither loses anyone this way.
//
// The caller must be inside the conversation (read) or a manager of it before
// anything about it is answered, so a stranger learns nothing of its kind or
// roster. The target must be a person directly in the group, not a UA or a
// node that merely exists. The graph is the membership: it is changed first, and
// the channel_members copy follows; a copy that cannot be deleted is reported after
// the live feed has been cut and the roster announced, and a repeat is harmless.
func (s *Service) RemoveMember(ctx context.Context, channelID, requesterNodeID, targetNodeID string) error {
	if channelID == "" || targetNodeID == "" {
		return ErrInvalidInput
	}
	ch, err := s.loadChannel(ctx, channelID)
	if err != nil {
		return err
	}
	// Inside the conversation, or its manager.
	manages := false
	if err := s.checkAccess(ctx, requesterNodeID, ch.NGACOaID, ngac.OpRead); err != nil {
		if merr := s.checkAccess(ctx, requesterNodeID, ch.NGACOaID, ngac.OpManage); merr != nil {
			return err
		}
		manages = true
	}
	if ch.ChannelType == ChannelTypeDM || ch.ChannelType == ChannelTypeWorkspace {
		return fmt.Errorf("%w: people cannot be removed from a direct message or the workspace channel", ErrInvalidInput)
	}
	if requesterNodeID != targetNodeID && !s.isChannelCreator(ctx, ch, requesterNodeID) && !manages {
		if err := s.checkAccess(ctx, requesterNodeID, ch.NGACOaID, ngac.OpManage); err != nil {
			return fmt.Errorf("%w: only the person themselves, the group's creator or a manager removes someone", ErrAccessDenied)
		}
	}
	if err := s.requireInChannel(ctx, ch, targetNodeID); err != nil {
		return err
	}
	if _, err := s.policyWrite.RemoveAssignment(ctx, &policypb.RemoveAssignmentRequest{
		ChildId: targetNodeID, ParentId: ch.NGACUaID,
	}); err != nil {
		return fmt.Errorf("remove member: %w", err)
	}
	cacheErr := s.members.DeleteChannelMember(ctx, channelID, targetNodeID)
	if cacheErr != nil {
		slog.Error("member removed from the graph but their channel_members row remains; repeating the removal clears it",
			"channel", channelID, "member", targetNodeID, "error", cacheErr)
	}
	// Losing membership must also end the live feed. The subscription was
	// authorized when it was opened; without this the removed user keeps
	// receiving every new message for as long as the socket stays up.
	if s.revoker != nil {
		s.revoker.RevokeChannelSubscriptions(channelID, targetNodeID)
	}
	s.announceMembership(ctx, realtime.KindMemberRemoved, ch, targetNodeID)
	if cacheErr != nil {
		return fmt.Errorf("remove member: %w", cacheErr)
	}
	return nil
}

// isChannelCreator reports whether the requester made the channel. The creator is
// recorded by user id; the verified caller on ctx links that to the node asking.
func (s *Service) isChannelCreator(ctx context.Context, ch *store.Channel, requesterNodeID string) bool {
	who := grpcauth.CallerFrom(ctx)
	return ch.CreatedBy != "" && who.UserID == ch.CreatedBy && who.NGACNodeID == requesterNodeID
}

// requireInChannel confirms the target is a person sitting directly under the
// channel's Members UA, asking the writer so a change made a moment ago is seen.
func (s *Service) requireInChannel(ctx context.Context, ch *store.Channel, targetNodeID string) error {
	notIn := fmt.Errorf("%w: not a member of this channel", ErrNotFound)
	n, err := s.policyWrite.GetNode(ctx, &policypb.GetNodeRequest{NodeId: targetNodeID})
	if err != nil {
		if policyclient.IsNotFound(err) {
			return notIn
		}
		return fmt.Errorf("remove member: resolve target: %w", err)
	}
	if n.GetNodeType() != ngac.TypeU {
		return notIn
	}
	res, err := s.policyWrite.IsAssigned(ctx, &policypb.IsAssignedRequest{ChildId: targetNodeID, ParentId: ch.NGACUaID})
	if err != nil {
		return fmt.Errorf("remove member: check membership: %w", err)
	}
	if !res.GetValue() {
		return notIn
	}
	return nil
}

// Roster is a channel's people as one reader sees them, with what that reader may do.
type Roster struct {
	Members []*pb.ChannelMember
	// CanRemove: the reader may take others out of this channel (RemoveMember would
	// not refuse them for who they are). It is false in a DM and the workspace-wide
	// channel, whose people are not removed this way. The screen shows its remove
	// action by this, not by comparing ids.
	CanRemove bool
}

// MemberRoster is ListMembers plus whether the reader may remove people.
func (s *Service) MemberRoster(ctx context.Context, channelID, userNodeID string) (*Roster, error) {
	members, err := s.ListMembers(ctx, channelID, userNodeID)
	if err != nil {
		return nil, err
	}
	ch, err := s.loadChannel(ctx, channelID)
	if err != nil {
		return nil, err
	}
	canRemove := ch.ChannelType != ChannelTypeDM && ch.ChannelType != ChannelTypeWorkspace &&
		(s.isChannelCreator(ctx, ch, userNodeID) || s.checkAccess(ctx, userNodeID, ch.NGACOaID, ngac.OpManage) == nil)
	return &Roster{Members: members, CanRemove: canRemove}, nil
}

// ListMembers returns the members of a channel via NGAC graph traversal: the user
// nodes directly under its Members UA. That includes the workspace-wide channel,
// whose members are direct edges for exactly this reason.
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
