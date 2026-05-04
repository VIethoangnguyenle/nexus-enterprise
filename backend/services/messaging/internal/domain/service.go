// Package domain contains the business logic for the messaging service.
// It orchestrates between the store (database), NGAC policy (access control),
// and external services (auth, drive). No SQL or protobuf lives here.
package domain

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"

	"ngac-platform/ngac"
	authpb "ngac-platform/proto/auth"
	drivepb "ngac-platform/proto/drive"
	pb "ngac-platform/proto/messaging"
	policypb "ngac-platform/proto/policy"
	"ngac-platform/services/messaging/internal/store"
)

// Service orchestrates messaging business logic.
type Service struct {
	store       *store.Store
	policyRead  policypb.PolicyReadServiceClient
	policyWrite policypb.PolicyWriteServiceClient
	authClient  authpb.AuthServiceClient
	driveClient drivepb.DriveServiceClient
}

// NewService creates a messaging domain service.
func NewService(
	st *store.Store,
	pr policypb.PolicyReadServiceClient,
	pw policypb.PolicyWriteServiceClient,
	ac authpb.AuthServiceClient,
	dc drivepb.DriveServiceClient,
) *Service {
	return &Service{
		store:       st,
		policyRead:  pr,
		policyWrite: pw,
		authClient:  ac,
		driveClient: dc,
	}
}

// --- Channel operations ---

// CreateChannelInput holds validated parameters for channel creation.
type CreateChannelInput struct {
	Name        string
	WorkspaceID string
	UserID      string
	UserNodeID  string
	ChannelType string
}

// CreateChannel creates a channel with NGAC nodes, permissions, and optional drive.
func (s *Service) CreateChannel(ctx context.Context, in CreateChannelInput) (*pb.Channel, error) {
	// Normalize channel type: "group" maps to "workspace" for DB constraint.
	if in.ChannelType == "group" {
		in.ChannelType = "workspace"
	}

	chID := uuid.New().String()

	contentOA, membersUA, err := s.createChannelNGACNodes(ctx, chID)
	if err != nil {
		return nil, err
	}

	if err := s.assignChannelToWorkspace(ctx, in.WorkspaceID, contentOA.Id, membersUA.Id); err != nil {
		return nil, err
	}

	s.grantChannelAccess(ctx, membersUA.Id, contentOA.Id, in.UserNodeID)

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
	if err := s.store.InsertChannel(ctx, ch); err != nil {
		return nil, fmt.Errorf("create channel: %w", err)
	}

	// Track creator as channel member for DM lookup optimization.
	s.store.InsertChannelMember(ctx, chID, in.UserNodeID)

	s.createChannelDrive(ctx, in.WorkspaceID, chID, in.Name, contentOA.Id, membersUA.Id)

	return channelToProto(ch), nil
}

// createChannelNGACNodes creates the Content OA and Members UA for a channel.
// Uses channel ID for naming to prevent collisions.
func (s *Service) createChannelNGACNodes(ctx context.Context, chID string) (*policypb.NGACNode, *policypb.NGACNode, error) {
	contentOA, err := s.policyWrite.CreateNode(ctx, &policypb.CreateNodeRequest{
		Name: ngac.ChannelContentOAName(chID), NodeType: ngac.TypeOA,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("create channel content OA: %w", err)
	}

	membersUA, err := s.policyWrite.CreateNode(ctx, &policypb.CreateNodeRequest{
		Name: ngac.ChannelMembersUAName(chID), NodeType: ngac.TypeUA,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("create channel members UA: %w", err)
	}

	return contentOA, membersUA, nil
}

// assignChannelToWorkspace links channel nodes into the workspace NGAC tree.
// For DMs (no workspace), assigns under PC_Global.
func (s *Service) assignChannelToWorkspace(ctx context.Context, workspaceID, contentOAID, membersUAID string) error {
	if workspaceID == "" {
		return s.assignToGlobalPC(ctx, contentOAID, membersUAID)
	}

	// Find workspace's Channels OA by workspace ID (not name).
	ws, err := s.store.GetWorkspaceByID(ctx, workspaceID)
	if err != nil || ws == nil {
		return fmt.Errorf("workspace lookup failed: %w", err)
	}

	// Try ID-based naming first (new convention), fallback to name-based (legacy).
	channelsOAID := s.findChildByName(ctx, ws.PCNodeID, ngac.ChannelsOAName(workspaceID), ngac.TypeOA)
	if channelsOAID == "" {
		channelsOAID = s.findChildByName(ctx, ws.PCNodeID, ngac.ChannelsOAName(ws.Name), ngac.TypeOA)
	}
	if channelsOAID != "" {
		s.policyWrite.CreateAssignment(ctx, &policypb.CreateAssignmentRequest{
			ChildId: contentOAID, ParentId: channelsOAID,
		})
	}

	s.policyWrite.CreateAssignment(ctx, &policypb.CreateAssignmentRequest{
		ChildId: membersUAID, ParentId: ws.PCNodeID,
	})

	return nil
}

// assignToGlobalPC assigns orphaned channel nodes (DMs) under PC_Global.
func (s *Service) assignToGlobalPC(ctx context.Context, contentOAID, membersUAID string) error {
	globalPC, err := s.policyRead.FindNodeByName(ctx, &policypb.FindNodeByNameRequest{
		Name: ngac.NodePCGlobal, NodeType: ngac.TypePC,
	})
	if err != nil || globalPC == nil {
		return fmt.Errorf("PC_Global not found: %w", err)
	}

	s.policyWrite.CreateAssignment(ctx, &policypb.CreateAssignmentRequest{
		ChildId: contentOAID, ParentId: globalPC.Id,
	})
	s.policyWrite.CreateAssignment(ctx, &policypb.CreateAssignmentRequest{
		ChildId: membersUAID, ParentId: globalPC.Id,
	})

	return nil
}

// grantChannelAccess creates the association and assigns the creator.
func (s *Service) grantChannelAccess(ctx context.Context, membersUAID, contentOAID, creatorNodeID string) {
	s.policyWrite.CreateAssociation(ctx, &policypb.CreateAssociationRequest{
		UaId: membersUAID, OaId: contentOAID,
		Operations: ngac.ChannelMemberOps(),
	})
	s.policyWrite.CreateAssignment(ctx, &policypb.CreateAssignmentRequest{
		ChildId: creatorNodeID, ParentId: membersUAID,
	})
}

// createChannelDrive creates a drive folder for the channel (non-fatal on error).
func (s *Service) createChannelDrive(ctx context.Context, workspaceID, chID, chName, oaID, uaID string) {
	if s.driveClient == nil || workspaceID == "" {
		return
	}
	s.driveClient.CreateDriveForChannel(ctx, &drivepb.CreateDriveForChannelRequest{
		WorkspaceId:     workspaceID,
		ChannelId:       chID,
		ChannelName:     chName,
		ChannelNgacOaId: oaID,
		ChannelNgacUaId: uaID,
	})
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
func (s *Service) UpdateChannel(ctx context.Context, channelID, name string) (*pb.Channel, error) {
	if name == "" {
		return nil, fmt.Errorf("channel name cannot be empty")
	}
	if err := s.store.UpdateChannelName(ctx, channelID, name); err != nil {
		return nil, fmt.Errorf("update channel: %w", err)
	}
	ch, err := s.store.GetChannel(ctx, channelID)
	if err != nil || ch == nil {
		return nil, fmt.Errorf("get updated channel: %w", err)
	}
	return channelToProto(ch), nil
}

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
	var result []*pb.Channel
	for _, ch := range channels {
		resp, _ := s.policyRead.CheckAccess(ctx, &policypb.CheckAccessRequest{
			UserNodeId: userNodeID, ObjectNodeId: ch.NGACOaID, Operation: ngac.OpRead,
		})
		if resp != nil && resp.Decision == ngac.DecisionAllow {
			result = append(result, channelToProto(ch))
		}
	}
	return result
}

// --- DM operations ---

// FindOrCreateDM finds an existing DM or creates a new one.
func (s *Service) FindOrCreateDM(ctx context.Context, userID, userNodeID, targetUserID, targetNodeID string) (*pb.Channel, error) {
	existing, err := s.store.FindDMByMembers(ctx, userNodeID, targetNodeID)
	if err == nil && existing != nil {
		return channelToProto(existing), nil
	}

	ch, err := s.CreateChannel(ctx, CreateChannelInput{
		Name:        fmt.Sprintf("DM_%s_%s", userID[:8], targetUserID[:8]),
		ChannelType: "dm",
		UserID:      userID,
		UserNodeID:  userNodeID,
	})
	if err != nil {
		return nil, fmt.Errorf("create DM channel: %w", err)
	}

	// Assign target user to channel members UA.
	if _, err := s.policyWrite.CreateAssignment(ctx, &policypb.CreateAssignmentRequest{
		ChildId: targetNodeID, ParentId: ch.NgacUaId,
	}); err != nil {
		return nil, fmt.Errorf("assign target to DM: %w", err)
	}

	// Track target as channel member for future DM lookups.
	s.store.InsertChannelMember(ctx, ch.Id, targetNodeID)

	return ch, nil
}

// --- Message operations ---

// SendMessageInput holds validated parameters for sending a message.
type SendMessageInput struct {
	ChannelID        string
	SenderID         string
	SenderNodeID     string
	Content          string
	ContentFormat    string
	Mentions         []string
	MessageType      string
	ParentMessageID  string
	LinkedEntityType string
	LinkedEntityID   string
}

// SendMessage sends a message after access checking.
func (s *Service) SendMessage(ctx context.Context, in SendMessageInput) (*pb.Message, error) {
	ch, err := s.store.GetChannel(ctx, in.ChannelID)
	if err != nil || ch == nil {
		return nil, fmt.Errorf("channel not found")
	}

	msgType := in.MessageType
	if msgType == "" {
		msgType = "user"
	}

	if msgType == "user" {
		if err := s.checkAccess(ctx, in.SenderNodeID, ch.NGACOaID, ngac.OpWrite); err != nil {
			return nil, err
		}
	}

	now := time.Now()
	contentFormat := in.ContentFormat
	if contentFormat == "" {
		contentFormat = "markdown"
	}
	msg := &store.Message{
		ID:               uuid.New().String(),
		ChannelID:        in.ChannelID,
		SenderID:         in.SenderID,
		Content:          in.Content,
		ContentFormat:    contentFormat,
		Mentions:         in.Mentions,
		MessageType:      msgType,
		ParentMessageID:  in.ParentMessageID,
		LinkedEntityType: in.LinkedEntityType,
		LinkedEntityID:   in.LinkedEntityID,
		CreatedAt:        now,
	}

	if err := s.store.InsertMessage(ctx, msg); err != nil {
		return nil, fmt.Errorf("send message: %w", err)
	}

	if in.ParentMessageID != "" {
		s.store.IncrementReplyCount(ctx, in.ParentMessageID)
		s.store.TrackThreadParticipant(ctx, in.ParentMessageID, in.SenderID)
	}

	msg.SenderName = s.lookupUsername(ctx, in.SenderID)

	return messageToProto(msg), nil
}

// GetMessages returns paginated messages for a channel.
func (s *Service) GetMessages(ctx context.Context, channelID, userNodeID, before string, limit int) (*pb.MessageList, error) {
	ch, err := s.store.GetChannel(ctx, channelID)
	if err != nil || ch == nil {
		return nil, fmt.Errorf("channel not found")
	}

	if err := s.checkAccess(ctx, userNodeID, ch.NGACOaID, ngac.OpRead); err != nil {
		return nil, err
	}

	if limit <= 0 || limit > 50 {
		limit = 50
	}

	var beforeTime *time.Time
	if before != "" {
		t, _ := time.Parse(time.RFC3339Nano, before)
		beforeTime = &t
	}

	msgs, hasMore, err := s.store.ListMessages(ctx, channelID, beforeTime, limit)
	if err != nil {
		return nil, fmt.Errorf("get messages: %w", err)
	}

	s.EnrichMessagesWithMetadata(ctx, msgs, channelID)

	return &pb.MessageList{
		Messages: messagesToProto(msgs),
		HasMore:  hasMore,
	}, nil
}

// GetThread returns the parent message plus all replies.
func (s *Service) GetThread(ctx context.Context, messageID string) (*pb.MessageList, error) {
	msgs, err := s.store.GetThread(ctx, messageID)
	if err != nil {
		return nil, fmt.Errorf("get thread: %w", err)
	}

	// Enrich with reactions and pin status; thread messages share the parent's channel.
	if len(msgs) > 0 {
		s.EnrichMessagesWithMetadata(ctx, msgs, msgs[0].ChannelID)
	}

	return &pb.MessageList{Messages: messagesToProto(msgs)}, nil
}

// FindThreadsByEntity returns messages linked to a specific entity.
func (s *Service) FindThreadsByEntity(ctx context.Context, entityType, entityID string) (*pb.MessageList, error) {
	msgs, err := s.store.FindByEntity(ctx, entityType, entityID)
	if err != nil {
		return nil, fmt.Errorf("find threads: %w", err)
	}
	return &pb.MessageList{Messages: messagesToProto(msgs)}, nil
}

// --- Channel member operations ---

// GetChannel retrieves a single channel.
func (s *Service) GetChannel(ctx context.Context, channelID string) (*pb.Channel, error) {
	ch, err := s.store.GetChannel(ctx, channelID)
	if err != nil || ch == nil {
		return nil, fmt.Errorf("channel not found")
	}
	return channelToProto(ch), nil
}

// AddMember adds a user to a channel's NGAC members UA.
func (s *Service) AddMember(ctx context.Context, channelID, requesterNodeID, targetNodeID string) error {
	ch, err := s.store.GetChannel(ctx, channelID)
	if err != nil || ch == nil {
		return fmt.Errorf("channel not found")
	}
	_, err = s.policyWrite.CreateAssignment(ctx, &policypb.CreateAssignmentRequest{
		ChildId: targetNodeID, ParentId: ch.NGACUaID,
	})
	if err != nil {
		return fmt.Errorf("add member: %w", err)
	}
	s.store.InsertChannelMember(ctx, channelID, targetNodeID)
	return nil
}

// RemoveMember removes a user from a channel's NGAC members UA.
func (s *Service) RemoveMember(ctx context.Context, channelID, targetNodeID string) error {
	ch, err := s.store.GetChannel(ctx, channelID)
	if err != nil || ch == nil {
		return fmt.Errorf("channel not found")
	}
	_, err = s.policyWrite.RemoveAssignment(ctx, &policypb.RemoveAssignmentRequest{
		ChildId: targetNodeID, ParentId: ch.NGACUaID,
	})
	if err != nil {
		return fmt.Errorf("remove member: %w", err)
	}
	return nil
}

// ListMembers returns the members of a channel via NGAC graph traversal.
func (s *Service) ListMembers(ctx context.Context, channelID string) ([]*pb.ChannelMember, error) {
	ch, err := s.store.GetChannel(ctx, channelID)
	if err != nil || ch == nil {
		return nil, fmt.Errorf("channel not found")
	}

	children, _ := s.policyRead.GetChildren(ctx, &policypb.GetChildrenRequest{NodeId: ch.NGACUaID})
	if children == nil {
		return nil, nil
	}

	var members []*pb.ChannelMember
	for _, n := range children.Nodes {
		if n.NodeType != ngac.TypeU {
			continue
		}
		username, userID := n.Name, ""
		user, _ := s.authClient.GetUserByNGACNodeID(ctx, &authpb.GetUserByNGACNodeIDRequest{NgacNodeId: n.Id})
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

// --- Helpers ---

// checkAccess verifies NGAC access and returns an error if denied.
func (s *Service) checkAccess(ctx context.Context, userNodeID, objectNodeID, operation string) error {
	resp, _ := s.policyRead.CheckAccess(ctx, &policypb.CheckAccessRequest{
		UserNodeId: userNodeID, ObjectNodeId: objectNodeID, Operation: operation,
	})
	if resp != nil && resp.Decision == ngac.DecisionDeny {
		return fmt.Errorf("%w: %s on %s", ErrAccessDenied, operation, objectNodeID)
	}
	return nil
}

// lookupUsername resolves a user ID to a username via the auth service.
func (s *Service) lookupUsername(ctx context.Context, userID string) string {
	user, _ := s.authClient.GetUserByID(ctx, &authpb.GetUserByIDRequest{UserId: userID})
	if user != nil {
		return user.Username
	}
	return ""
}

// --- Proto conversions ---

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

func messageToProto(m *store.Message) *pb.Message {
	var reactions []*pb.ReactionGroup
	for _, rg := range m.Reactions {
		reactions = append(reactions, &pb.ReactionGroup{
			Emoji:   rg.Emoji,
			Count:   rg.Count,
			UserIds: rg.UserIDs,
		})
	}
	return &pb.Message{
		Id:               m.ID,
		ChannelId:        m.ChannelID,
		SenderId:         m.SenderID,
		SenderName:       m.SenderName,
		Content:          m.Content,
		MessageType:      m.MessageType,
		ParentMessageId:  m.ParentMessageID,
		LinkedEntityType: m.LinkedEntityType,
		LinkedEntityId:   m.LinkedEntityID,
		ReplyCount:       m.ReplyCount,
		ContentFormat:    m.ContentFormat,
		Mentions:         m.Mentions,
		Reactions:        reactions,
		IsPinned:         m.IsPinned,
		CreatedAt:        timestamppb.New(m.CreatedAt),
	}
}

func messagesToProto(msgs []*store.Message) []*pb.Message {
	result := make([]*pb.Message, 0, len(msgs))
	for _, m := range msgs {
		result = append(result, messageToProto(m))
	}
	return result
}
