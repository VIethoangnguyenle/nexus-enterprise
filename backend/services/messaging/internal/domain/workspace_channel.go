package domain

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"ngac-platform/ngac"
	"ngac-platform/pkg/grpcauth"
	"ngac-platform/pkg/policyclient"
	"ngac-platform/pkg/realtime"
	pb "ngac-platform/proto/messaging"
	policypb "ngac-platform/proto/policy"
)

// The workspace-wide channel (#general) has no graph shape of its own: it is a
// channel whose Members UA holds every member of the workspace as a direct
// U edge, with a channel_members cache row beside each. Direct edges, not a UA
// assigned under the channel's UA, because the roster (GetChildren of the UA),
// push recipients and member_count all read people at that level.
//
// Nothing makes workspace membership reach it by itself, so every admission
// writes the edge (JoinWorkspaceChannel), and leaving the workspace removes it
// with the rest of the person's UAs.

// ProvisionWorkspaceChannelInput is the workspace-wide channel to create and the
// person creating it.
type ProvisionWorkspaceChannelInput struct {
	WorkspaceID   string
	Name          string
	CreatorUserID string
	CreatorNodeID string
}

// actingAs returns ctx with the person the service is acting for as the caller,
// which is what the policy service answers for. The messaging gRPC methods below
// are reached on a service identity (auth, workspace), which carries no person,
// and a policy call made on one is refused.
func actingAs(ctx context.Context, userID, nodeID, workspaceID string) context.Context {
	return grpcauth.WithCaller(ctx, grpcauth.Caller{UserID: userID, NGACNodeID: nodeID, TenantID: workspaceID})
}

// ProvisionWorkspaceChannel creates the workspace's one workspace-wide channel,
// with its creator (the workspace's owner) as the first member. The creator needs
// create_channel on the Channels OA like anyone starting a group. A workspace
// that has one already gets ErrAlreadyExists before anything is written; the
// unique index closes the race between two such calls.
//
// Only the auth service may reach this (the gRPC layer enforces it): a person
// never makes a workspace-wide channel.
func (s *Service) ProvisionWorkspaceChannel(ctx context.Context, in ProvisionWorkspaceChannelInput) (*pb.Channel, error) {
	if in.WorkspaceID == "" || in.Name == "" || in.CreatorUserID == "" || in.CreatorNodeID == "" {
		return nil, fmt.Errorf("%w: workspace, name and creator are required", ErrInvalidInput)
	}
	exists, err := s.store.HasWorkspaceTypeChannel(ctx, in.WorkspaceID)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, fmt.Errorf("%w: the workspace already has its workspace-wide channel", ErrAlreadyExists)
	}
	return s.createInWorkspace(actingAs(ctx, in.CreatorUserID, in.CreatorNodeID, in.WorkspaceID), CreateChannelInput{
		Name: in.Name, WorkspaceID: in.WorkspaceID, UserID: in.CreatorUserID,
		UserNodeID: in.CreatorNodeID, ChannelType: ChannelTypeWorkspace,
	})
}

// JoinWorkspaceChannel puts a member of the workspace into its workspace-wide
// channel: a direct edge from the person's node to the channel's Members UA and
// the cache row beside it. It is safe to repeat, and does nothing in a workspace
// that has no such channel.
//
// The person must be a member of the workspace (a node under its Members UA);
// anyone else is refused, so this cannot be used to put a stranger into
// #general. Messaging's policy clients both dial the primary policy service, so
// an admission made a moment ago is seen.
//
// Called by the workspace service when it admits someone and by the auth service
// when it joins a tenant; both reach it on a service identity. Listing channels
// calls it for a member found without the edge.
func (s *Service) JoinWorkspaceChannel(ctx context.Context, workspaceID, userID, userNodeID string) error {
	if workspaceID == "" || userID == "" || userNodeID == "" {
		return fmt.Errorf("%w: workspace, user and user node are required", ErrInvalidInput)
	}
	// The policy answers on behalf of the person being joined; the announcement of
	// it has the workspace as its boundary and no person as its cause (the service
	// admits them, nobody acts), so it must not carry the stand-in identity.
	announceCtx := grpcauth.WithCaller(ctx, grpcauth.Caller{TenantID: workspaceID})
	ctx = actingAs(ctx, userID, userNodeID, workspaceID)
	ch, err := s.store.WorkspaceWideChannel(ctx, workspaceID)
	if err != nil {
		return err
	}
	if ch == nil {
		// Not an error (a workspace made before #general existed has none), but
		// worth seeing: a workspace that should have one and does not looks like this.
		slog.Warn("workspace has no workspace-wide channel to join", "workspace", workspaceID)
		return nil
	}
	member, err := policyclient.IsWorkspaceMember(ctx, s.policyRead, userNodeID, ngac.WorkspaceID(workspaceID))
	if err != nil {
		return fmt.Errorf("check workspace membership: %w", err)
	}
	if !member {
		return fmt.Errorf("%w: not a member of the workspace", ErrAccessDenied)
	}

	// CreateAssignment is idempotent, so ask first: only an edge this call makes
	// may be withdrawn if the cache row then fails, never one that was there.
	res, err := s.policyRead.IsAssigned(ctx, &policypb.IsAssignedRequest{ChildId: userNodeID, ParentId: ch.NGACUaID})
	if err != nil {
		return fmt.Errorf("check channel membership: %w", err)
	}
	existed := res.GetValue()
	if !existed {
		if _, err := s.policyWrite.CreateAssignment(ctx, &policypb.CreateAssignmentRequest{
			ChildId: userNodeID, ParentId: ch.NGACUaID,
		}); err != nil {
			return fmt.Errorf("join workspace channel: %w", err)
		}
	}
	if err := s.members.InsertChannelMember(ctx, ch.ID, userNodeID); err != nil {
		if !existed {
			if _, rerr := s.policyWrite.RemoveAssignment(ctx, &policypb.RemoveAssignmentRequest{
				ChildId: userNodeID, ParentId: ch.NGACUaID,
			}); rerr != nil {
				slog.Error("member joined the workspace channel in the graph but not in the cache, and the edge could not be withdrawn",
					"channel", ch.ID, "member", userNodeID, "record_error", err, "withdraw_error", rerr)
			}
		}
		return fmt.Errorf("join workspace channel: %w", err)
	}
	if !existed {
		s.announceMembership(announceCtx, realtime.KindMemberAdded, ch, userNodeID)
	}
	s.hasWideEdge.Store(wideEdgeKey(workspaceID, userNodeID), true)
	return nil
}

// wideEdgeKey keys hasWideEdge.
func wideEdgeKey(workspaceID, userNodeID string) string { return workspaceID + "|" + userNodeID }

// ensureWorkspaceChannelMembership repairs a missing #general edge for the person
// listing the workspace's channels: the join at admission is best-effort (the other
// service may have been down), and nothing else would ever retry it. It never
// fails the listing and never joins anyone who is not a member of the workspace
// (JoinWorkspaceChannel checks), and a person found with the edge is remembered so
// the policy is not asked again on every list.
func (s *Service) ensureWorkspaceChannelMembership(ctx context.Context, workspaceID, userNodeID string) {
	who := grpcauth.CallerFrom(ctx)
	if workspaceID == "" || who.UserID == "" || who.NGACNodeID == "" || who.NGACNodeID != userNodeID {
		return
	}
	if _, ok := s.hasWideEdge.Load(wideEdgeKey(workspaceID, userNodeID)); ok {
		return
	}
	ch, err := s.store.WorkspaceWideChannel(ctx, workspaceID)
	if err != nil || ch == nil {
		return
	}
	has, err := s.policyRead.IsAssigned(ctx, &policypb.IsAssignedRequest{ChildId: userNodeID, ParentId: ch.NGACUaID})
	if err != nil {
		return
	}
	if has.GetValue() {
		s.hasWideEdge.Store(wideEdgeKey(workspaceID, userNodeID), true)
		return
	}
	if err := s.JoinWorkspaceChannel(ctx, workspaceID, who.UserID, userNodeID); err != nil && !errors.Is(err, ErrAccessDenied) {
		slog.Warn("workspace channel not repaired for a member", "workspace", workspaceID, "error", err)
	}
}

// ForgetWorkspaceMember drops the member-cache rows of someone who left the
// workspace, for every channel of it. The graph side needs no call: leaving
// strips every UA under the workspace's policy class, the channels' Members UAs
// included.
func (s *Service) ForgetWorkspaceMember(ctx context.Context, workspaceID, userNodeID string) error {
	if workspaceID == "" || userNodeID == "" {
		return fmt.Errorf("%w: workspace and user are required", ErrInvalidInput)
	}
	s.hasWideEdge.Delete(wideEdgeKey(workspaceID, userNodeID))
	_, err := s.store.DeleteChannelMembersInWorkspace(ctx, workspaceID, userNodeID)
	return err
}
