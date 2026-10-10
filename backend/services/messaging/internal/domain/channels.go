// Package domain contains the business logic for the messaging service.
// It orchestrates between the store (database), NGAC policy (access control),
// and external services (auth, drive). No SQL or protobuf lives here.
package domain

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"

	"ngac-platform/ngac"
	"ngac-platform/pkg/provision"
	"ngac-platform/pkg/realtime"
	drivepb "ngac-platform/proto/drive"
	pb "ngac-platform/proto/messaging"
	policypb "ngac-platform/proto/policy"
	"ngac-platform/services/messaging/internal/store"
)

// CreateChannelInput holds validated parameters for channel creation.
type CreateChannelInput struct {
	Name        string
	WorkspaceID string
	UserID      string
	UserNodeID  string
	ChannelType string
}

// CreateChannel creates a workspace channel with NGAC nodes, permissions, and
// optional drive.
//
// The caller must hold create_channel on the workspace's Channels OA. The check
// runs before anything is written, so a denied request leaves no orphan nodes
// in the graph. A channel with no workspace has no Channels OA to authorize
// against, so it is refused here — direct messages go through FindOrCreateDM.
func (s *Service) CreateChannel(ctx context.Context, in CreateChannelInput) (*pb.Channel, error) {
	if in.WorkspaceID == "" {
		return nil, fmt.Errorf("%w: a channel must belong to a workspace", ErrInvalidInput)
	}
	ws, err := s.store.GetWorkspaceByID(ctx, in.WorkspaceID)
	if err != nil {
		return nil, fmt.Errorf("workspace lookup: %w", err)
	}
	if ws == nil {
		return nil, fmt.Errorf("%w: workspace", ErrNotFound)
	}
	channelsOAID := s.findChildByName(ctx, ws.PCNodeID, ngac.ChannelsOAName(ngac.WorkspaceID(ws.ID)), ngac.TypeOA)
	if channelsOAID == "" {
		// Fail closed. The legacy name-keyed node is deliberately not consulted:
		// two workspaces may share a display name, and authorizing against a
		// node they share is how one tenant's grant reaches another.
		return nil, fmt.Errorf("%w: %s on workspace channels", ErrAccessDenied, ngac.OpCreateChannel)
	}
	if err := s.checkAccess(ctx, in.UserNodeID, channelsOAID, ngac.OpCreateChannel); err != nil {
		return nil, err
	}
	return s.createChannel(ctx, in, ws.PCNodeID, channelsOAID)
}

// createChannel performs the writes for a channel whose creation has already
// been authorized. pcID and channelsOAID are empty for a DM, which hangs under
// PC_Global instead of a workspace. extraMemberNodeIDs are users assigned to the
// channel's Members UA alongside its creator (a DM's other participant).
//
// The graph writes and the row insert are separate steps. A failure at any of
// them removes the nodes already created, newest first, so a channel that could
// not be completed leaves nothing in the graph.
func (s *Service) createChannel(ctx context.Context, in CreateChannelInput, pcID, channelsOAID string, extraMemberNodeIDs ...string) (*pb.Channel, error) {
	// Normalize channel type: "group" maps to "workspace" for DB constraint.
	if in.ChannelType == "group" {
		in.ChannelType = "workspace"
	}

	chID := uuid.New().String()
	prov := provision.NewCreator(s.policyWrite)

	contentOA, membersUA, err := s.createChannelNGACNodes(ctx, prov, chID)
	if err != nil {
		return nil, prov.Fail(ctx, err)
	}

	if err := s.assignChannelNodes(ctx, prov, pcID, channelsOAID, contentOA.Id, membersUA.Id); err != nil {
		return nil, prov.Fail(ctx, err)
	}

	if err := s.grantChannelAccess(ctx, prov, membersUA.Id, contentOA.Id, in.UserNodeID); err != nil {
		return nil, prov.Fail(ctx, err)
	}
	for _, nodeID := range extraMemberNodeIDs {
		if err := prov.Assign(ctx, nodeID, membersUA.Id); err != nil {
			return nil, prov.Fail(ctx, fmt.Errorf("assign member to channel members UA: %w", err))
		}
	}

	ch := &store.Channel{
		ID:          chID,
		Name:        in.Name,
		ChannelType: in.ChannelType,
		WorkspaceID: in.WorkspaceID,
		NGACOaID:    contentOA.Id,
		NGACUaID:    membersUA.Id,
		CreatedBy:   in.UserID,
		CreatedAt:   time.Now(),
	}
	// The row and the members cache (what DM lookup reads) are written together:
	// a channel without them is a DM nobody can find, created again on the next try.
	members := append([]string{in.UserNodeID}, extraMemberNodeIDs...)
	if err := s.store.InsertChannelWithMembers(ctx, ch, members...); err != nil {
		return nil, prov.Fail(ctx, fmt.Errorf("create channel: %w", err))
	}
	prov.Done()

	s.createChannelDrive(ctx, in.WorkspaceID, chID, in.Name, contentOA.Id, membersUA.Id)

	// A channel is readable by its Members UA, which holds the creator and any
	// extra participants (a DM's other person) when it is born: they are the
	// audience, not the workspace. Others learn of it through member_added.
	created := channelEvent(ctx, realtime.KindCreated, ch)
	created.UserNodeIDs = append([]string{in.UserNodeID}, extraMemberNodeIDs...)
	s.emit(created)

	return channelToProto(ch), nil
}

// createChannelNGACNodes creates the Content OA and Members UA for a channel.
// Uses channel ID for naming to prevent collisions.
func (s *Service) createChannelNGACNodes(ctx context.Context, prov *provision.Creator, chID string) (*policypb.NGACNode, *policypb.NGACNode, error) {
	id := ngac.ChannelID(chID)
	contentOA, err := prov.Node(ctx, &policypb.CreateNodeRequest{
		Name: ngac.ChannelContentOAName(id), NodeType: ngac.TypeOA,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("create channel content OA: %w", err)
	}

	membersUA, err := prov.Node(ctx, &policypb.CreateNodeRequest{
		Name: ngac.ChannelMembersUAName(id), NodeType: ngac.TypeUA,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("create channel members UA: %w", err)
	}

	return contentOA, membersUA, nil
}

// assignChannelNodes links channel nodes into the workspace NGAC tree: content
// under the workspace's Channels OA, members under its PC. With no workspace
// (a DM) both go under PC_Global.
func (s *Service) assignChannelNodes(ctx context.Context, prov *provision.Creator, pcID, channelsOAID, contentOAID, membersUAID string) error {
	if pcID == "" {
		return s.assignToGlobalPC(ctx, prov, contentOAID, membersUAID)
	}

	if err := prov.Assign(ctx, contentOAID, channelsOAID); err != nil {
		return fmt.Errorf("assign channel content under Channels OA: %w", err)
	}

	if err := prov.Assign(ctx, membersUAID, pcID); err != nil {
		return fmt.Errorf("assign channel members UA under workspace PC: %w", err)
	}

	return nil
}

// assignToGlobalPC assigns orphaned channel nodes (DMs) under PC_Global.
func (s *Service) assignToGlobalPC(ctx context.Context, prov *provision.Creator, contentOAID, membersUAID string) error {
	globalPC, err := s.policyRead.FindNodeByName(ctx, &policypb.FindNodeByNameRequest{
		Name: ngac.NodePCGlobal, NodeType: ngac.TypePC,
	})
	if err != nil || globalPC == nil {
		return fmt.Errorf("%s not found: %w", ngac.NodePCGlobal, err)
	}

	if err := prov.Assign(ctx, contentOAID, globalPC.Id); err != nil {
		return fmt.Errorf("assign DM content under %s: %w", ngac.NodePCGlobal, err)
	}
	if err := prov.Assign(ctx, membersUAID, globalPC.Id); err != nil {
		return fmt.Errorf("assign DM members UA under %s: %w", ngac.NodePCGlobal, err)
	}

	return nil
}

// grantChannelAccess creates the association and assigns the creator.
//
// Both writes are load-bearing: without the association the channel grants
// nothing to anyone, and without the assignment its own creator cannot read it.
// A channel that reaches the database in either of those states is unusable and
// looks like a permissions bug rather than a failed write, so the caller has to
// hear about it.
func (s *Service) grantChannelAccess(ctx context.Context, prov *provision.Creator, membersUAID, contentOAID, creatorNodeID string) error {
	if err := prov.Associate(ctx, membersUAID, contentOAID, ngac.ChannelMemberOps()); err != nil {
		return fmt.Errorf("grant channel members access to content: %w", err)
	}
	if err := prov.Assign(ctx, creatorNodeID, membersUAID); err != nil {
		return fmt.Errorf("assign channel creator to members UA: %w", err)
	}
	return nil
}

// createChannelDrive creates a drive folder for the channel (non-fatal on error).
func (s *Service) createChannelDrive(ctx context.Context, workspaceID, chID, chName, oaID, uaID string) {
	if s.driveClient == nil || workspaceID == "" {
		return
	}
	if _, err := s.driveClient.CreateDriveForChannel(ctx, &drivepb.CreateDriveForChannelRequest{
		WorkspaceId:     workspaceID,
		ChannelId:       chID,
		ChannelName:     chName,
		ChannelNgacOaId: oaID,
		ChannelNgacUaId: uaID,
	}); err != nil {
		// The channel exists and works without its drive; the drive is created
		// lazily on first use. The failure is for the log.
		slog.Warn("channel drive not created", "channel", chID, "error", err)
	}
}

// findChildByName searches direct children of a node for a specific name+type.
func (s *Service) findChildByName(ctx context.Context, parentID, name, nodeType string) string {
	children, err := s.policyRead.GetChildren(ctx, &policypb.GetChildrenRequest{NodeId: parentID})
	if err != nil || children == nil {
		return ""
	}
	for _, n := range children.Nodes {
		if n.NodeType == nodeType && n.Name == name {
			return n.Id
		}
	}
	return ""
}

// ListChannels returns channels the user has read access to.
func (s *Service) ListChannels(ctx context.Context, workspaceID, userNodeID string) ([]*pb.Channel, error) {
	channels, err := s.store.ListChannelsByWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list channels: %w", err)
	}
	return s.filterAccessible(ctx, channels, userNodeID), nil
}

// UpdateChannel renames a channel. Returns the updated channel proto.
//
// Renaming changes what every member sees, so it takes manage on the channel's
// content OA — held by workspace owners through the Channels OA, not by plain
// channel members.
func (s *Service) UpdateChannel(ctx context.Context, channelID, userNodeID, name string) (*pb.Channel, error) {
	if name == "" {
		return nil, fmt.Errorf("%w: channel name cannot be empty", ErrInvalidInput)
	}
	if _, err := s.authorizeChannel(ctx, channelID, userNodeID, ngac.OpManage); err != nil {
		return nil, err
	}
	if err := s.store.UpdateChannelName(ctx, channelID, name); err != nil {
		return nil, fmt.Errorf("update channel: %w", err)
	}
	ch, err := s.loadChannel(ctx, channelID)
	if err != nil {
		return nil, fmt.Errorf("get updated channel: %w", err)
	}
	// Only the channel's own subscribers hear about it: a private channel's name
	// is not the rest of the workspace's business.
	renamed := channelEvent(ctx, realtime.KindRenamed, ch)
	renamed.ChannelID = ch.ID
	s.emit(renamed)
	return channelToProto(ch), nil
}

// GetChannel retrieves a single channel.
func (s *Service) GetChannel(ctx context.Context, channelID, userNodeID string) (*pb.Channel, error) {
	ch, err := s.authorizeChannel(ctx, channelID, userNodeID, ngac.OpRead)
	if err != nil {
		return nil, err
	}
	return channelToProto(ch), nil
}

func channelToProto(ch *store.Channel) *pb.Channel {
	return &pb.Channel{
		Id:          ch.ID,
		Name:        ch.Name,
		ChannelType: ch.ChannelType,
		WorkspaceId: ch.WorkspaceID,
		NgacOaId:    ch.NGACOaID,
		NgacUaId:    ch.NGACUaID,
		CreatedBy:   ch.CreatedBy,
		CreatedAt:   timestamppb.New(ch.CreatedAt),
		Topic:       ch.Topic,
		Description: ch.Description,
		MemberCount: ch.MemberCount,
	}
}
