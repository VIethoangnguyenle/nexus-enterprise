package grpc

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	authpb "ngac-platform/proto/auth"
	pb "ngac-platform/proto/messaging"
	policypb "ngac-platform/proto/policy"
)

type MessagingServer struct {
	pb.UnimplementedMessagingServiceServer
	db           *pgxpool.Pool
	policyClient policypb.PolicyServiceClient
	authClient   authpb.AuthServiceClient
	hub          *Hub
}

func NewMessagingServer(db *pgxpool.Pool, pc policypb.PolicyServiceClient, ac authpb.AuthServiceClient, hub *Hub) *MessagingServer {
	return &MessagingServer{db: db, policyClient: pc, authClient: ac, hub: hub}
}

func (s *MessagingServer) CreateChannel(ctx context.Context, req *pb.CreateChannelRequest) (*pb.Channel, error) {
	// Create channel content OA
	contentOA, err := s.policyClient.CreateNode(ctx, &policypb.CreateNodeRequest{
		Name: fmt.Sprintf("Ch_%s_Content", req.Name), NodeType: "OA",
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "create channel OA: %v", err)
	}
	// Create channel members UA
	membersUA, err := s.policyClient.CreateNode(ctx, &policypb.CreateNodeRequest{
		Name: fmt.Sprintf("Ch_%s_Members", req.Name), NodeType: "UA",
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "create channel UA: %v", err)
	}
	// Find workspace's Channels OA by looking up workspace in DB.
	// Workspace creation names OAs as {WorkspaceName}_Channels.
	var channelsOAID, pcNodeID string
	if req.WorkspaceId != "" {
		var wsName string
		s.db.QueryRow(ctx, "SELECT name, ngac_pc_id FROM workspaces WHERE id = $1", req.WorkspaceId).
			Scan(&wsName, &pcNodeID)
		if pcNodeID != "" {
			children, _ := s.policyClient.GetChildren(ctx, &policypb.GetChildrenRequest{NodeId: pcNodeID})
			if children != nil {
				for _, n := range children.Nodes {
					if n.NodeType == "OA" && n.Name == fmt.Sprintf("%s_Channels", wsName) {
						channelsOAID = n.Id
						break
					}
				}
			}
		}
	}
	// Assign channel content OA under workspace's Channels OA
	if channelsOAID != "" {
		s.policyClient.CreateAssignment(ctx, &policypb.CreateAssignmentRequest{
			ChildId: contentOA.Id, ParentId: channelsOAID,
		})
	}
	// Assign channel members UA to workspace PC
	if pcNodeID != "" {
		s.policyClient.CreateAssignment(ctx, &policypb.CreateAssignmentRequest{
			ChildId: membersUA.Id, ParentId: pcNodeID,
		})
	}
	// Association: members can read+write on channel content
	s.policyClient.CreateAssociation(ctx, &policypb.CreateAssociationRequest{
		UaId: membersUA.Id, OaId: contentOA.Id,
		Operations: []string{"read", "write"},
	})
	// Assign creator to members
	s.policyClient.CreateAssignment(ctx, &policypb.CreateAssignmentRequest{
		ChildId: req.UserNgacNodeId, ParentId: membersUA.Id,
	})

	chID := uuid.New().String()
	_, err = s.db.Exec(ctx,
		"INSERT INTO channels (id, name, channel_type, workspace_id, ngac_oa_id, ngac_ua_id, created_by) VALUES ($1, $2, $3, $4, $5, $6, $7)",
		chID, req.Name, req.ChannelType, req.WorkspaceId, contentOA.Id, membersUA.Id, req.UserId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "insert channel: %v", err)
	}

	return &pb.Channel{
		Id: chID, Name: req.Name, ChannelType: req.ChannelType,
		WorkspaceId: req.WorkspaceId, NgacOaId: contentOA.Id,
		NgacUaId: membersUA.Id, CreatedBy: req.UserId,
		CreatedAt: timestamppb.Now(),
	}, nil
}

func (s *MessagingServer) ListChannels(ctx context.Context, req *pb.ListChannelsRequest) (*pb.ChannelList, error) {
	rows, err := s.db.Query(ctx,
		"SELECT id, name, channel_type, workspace_id, ngac_oa_id, ngac_ua_id, created_by, created_at FROM channels WHERE workspace_id = $1 AND channel_type != 'dm'",
		req.WorkspaceId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list channels: %v", err)
	}
	defer rows.Close()

	var channels []*pb.Channel
	for rows.Next() {
		var ch pb.Channel
		var ca time.Time
		if err := rows.Scan(&ch.Id, &ch.Name, &ch.ChannelType, &ch.WorkspaceId, &ch.NgacOaId, &ch.NgacUaId, &ch.CreatedBy, &ca); err != nil {
			return nil, err
		}
		ch.CreatedAt = timestamppb.New(ca)
		// Filter by access
		resp, _ := s.policyClient.CheckAccess(ctx, &policypb.CheckAccessRequest{
			UserNodeId: req.UserNgacNodeId, ObjectNodeId: ch.NgacOaId, Operation: "read",
		})
		if resp != nil && resp.Decision == "ALLOW" {
			channels = append(channels, &ch)
		}
	}
	return &pb.ChannelList{Channels: channels}, nil
}

func (s *MessagingServer) GetChannel(ctx context.Context, req *pb.GetChannelRequest) (*pb.Channel, error) {
	var ch pb.Channel
	var ca time.Time
	err := s.db.QueryRow(ctx,
		"SELECT id, name, channel_type, workspace_id, ngac_oa_id, ngac_ua_id, created_by, created_at FROM channels WHERE id = $1",
		req.ChannelId).Scan(&ch.Id, &ch.Name, &ch.ChannelType, &ch.WorkspaceId, &ch.NgacOaId, &ch.NgacUaId, &ch.CreatedBy, &ca)
	if err == pgx.ErrNoRows {
		return nil, status.Errorf(codes.NotFound, "channel not found")
	}
	ch.CreatedAt = timestamppb.New(ca)
	return &ch, err
}

func (s *MessagingServer) AddChannelMember(ctx context.Context, req *pb.AddChannelMemberRequest) (*pb.Empty, error) {
	ch, err := s.GetChannel(ctx, &pb.GetChannelRequest{ChannelId: req.ChannelId})
	if err != nil {
		return nil, err
	}
	_, err = s.policyClient.CreateAssignment(ctx, &policypb.CreateAssignmentRequest{
		ChildId: req.TargetNgacNodeId, ParentId: ch.NgacUaId,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "add member: %v", err)
	}
	return &pb.Empty{}, nil
}

func (s *MessagingServer) RemoveChannelMember(ctx context.Context, req *pb.RemoveChannelMemberRequest) (*pb.Empty, error) {
	ch, err := s.GetChannel(ctx, &pb.GetChannelRequest{ChannelId: req.ChannelId})
	if err != nil {
		return nil, err
	}
	_, err = s.policyClient.RemoveAssignment(ctx, &policypb.RemoveAssignmentRequest{
		ChildId: req.TargetNgacNodeId, ParentId: ch.NgacUaId,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "remove member: %v", err)
	}
	return &pb.Empty{}, nil
}

func (s *MessagingServer) ListChannelMembers(ctx context.Context, req *pb.ListChannelMembersRequest) (*pb.ChannelMemberList, error) {
	ch, err := s.GetChannel(ctx, &pb.GetChannelRequest{ChannelId: req.ChannelId})
	if err != nil {
		return nil, err
	}
	children, _ := s.policyClient.GetChildren(ctx, &policypb.GetChildrenRequest{NodeId: ch.NgacUaId})
	var members []*pb.ChannelMember
	if children != nil {
		for _, n := range children.Nodes {
			if n.NodeType == "U" {
				user, _ := s.authClient.GetUserByNGACNodeID(ctx, &authpb.GetUserByNGACNodeIDRequest{NgacNodeId: n.Id})
				username := n.Name
				userID := ""
				if user != nil {
					username = user.Username
					userID = user.Id
				}
				members = append(members, &pb.ChannelMember{
					UserId: userID, Username: username, NgacNodeId: n.Id,
				})
			}
		}
	}
	return &pb.ChannelMemberList{Members: members}, nil
}

func (s *MessagingServer) SendMessage(ctx context.Context, req *pb.SendMessageRequest) (*pb.Message, error) {
	ch, err := s.GetChannel(ctx, &pb.GetChannelRequest{ChannelId: req.ChannelId})
	if err != nil {
		return nil, err
	}
	// Check write access
	resp, _ := s.policyClient.CheckAccess(ctx, &policypb.CheckAccessRequest{
		UserNodeId: req.SenderNgacNodeId, ObjectNodeId: ch.NgacOaId, Operation: "write",
	})
	if resp != nil && resp.Decision == "DENY" {
		return nil, status.Errorf(codes.PermissionDenied, "no write access to channel")
	}

	msgID := uuid.New().String()
	now := time.Now()
	_, err = s.db.Exec(ctx,
		"INSERT INTO messages (id, channel_id, sender_id, content, created_at) VALUES ($1, $2, $3, $4, $5)",
		msgID, req.ChannelId, req.SenderId, req.Content, now)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "insert message: %v", err)
	}

	// Get sender name
	senderName := ""
	user, _ := s.authClient.GetUserByID(ctx, &authpb.GetUserByIDRequest{UserId: req.SenderId})
	if user != nil {
		senderName = user.Username
	}

	msg := &pb.Message{
		Id: msgID, ChannelId: req.ChannelId, SenderId: req.SenderId,
		SenderName: senderName, Content: req.Content,
		CreatedAt: timestamppb.New(now),
	}

	// Broadcast via WebSocket hub
	if s.hub != nil {
		s.hub.BroadcastToChannel(req.ChannelId, msg)
	}

	return msg, nil
}

func (s *MessagingServer) GetMessages(ctx context.Context, req *pb.GetMessagesRequest) (*pb.MessageList, error) {
	ch, err := s.GetChannel(ctx, &pb.GetChannelRequest{ChannelId: req.ChannelId})
	if err != nil {
		return nil, err
	}
	// Check read access
	resp, _ := s.policyClient.CheckAccess(ctx, &policypb.CheckAccessRequest{
		UserNodeId: req.UserNgacNodeId, ObjectNodeId: ch.NgacOaId, Operation: "read",
	})
	if resp != nil && resp.Decision == "DENY" {
		return nil, status.Errorf(codes.PermissionDenied, "no read access to channel")
	}

	limit := int(req.Limit)
	if limit <= 0 || limit > 50 {
		limit = 50
	}

	query := "SELECT m.id, m.channel_id, m.sender_id, COALESCE(u.username,''), m.content, m.created_at FROM messages m LEFT JOIN users u ON m.sender_id = u.id WHERE m.channel_id = $1"
	args := []interface{}{req.ChannelId}

	if req.Before != "" {
		query += " AND m.created_at < $2 ORDER BY m.created_at DESC LIMIT $3"
		t, _ := time.Parse(time.RFC3339Nano, req.Before)
		args = append(args, t, limit+1)
	} else {
		query += " ORDER BY m.created_at DESC LIMIT $2"
		args = append(args, limit+1)
	}

	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get messages: %v", err)
	}
	defer rows.Close()

	var msgs []*pb.Message
	for rows.Next() {
		var m pb.Message
		var ca time.Time
		if err := rows.Scan(&m.Id, &m.ChannelId, &m.SenderId, &m.SenderName, &m.Content, &ca); err != nil {
			return nil, err
		}
		m.CreatedAt = timestamppb.New(ca)
		msgs = append(msgs, &m)
	}

	hasMore := len(msgs) > limit
	if hasMore {
		msgs = msgs[:limit]
	}

	return &pb.MessageList{Messages: msgs, HasMore: hasMore}, nil
}

func (s *MessagingServer) CreateDM(ctx context.Context, req *pb.CreateDMRequest) (*pb.Channel, error) {
	// Check if DM already exists between these two users.
	// Query all DM channels and check if both users are members via NGAC graph.
	existing, err := s.findExistingDM(ctx, req.UserNgacNodeId, req.TargetNgacNodeId)
	if err == nil && existing != nil {
		return existing, nil
	}

	// Create DM channel — creator is auto-assigned by CreateChannel
	ch, err := s.CreateChannel(ctx, &pb.CreateChannelRequest{
		Name:           fmt.Sprintf("DM_%s_%s", req.UserId[:8], req.TargetUserId[:8]),
		WorkspaceId:    "",
		UserId:         req.UserId,
		UserNgacNodeId: req.UserNgacNodeId,
		ChannelType:    "dm",
	})
	if err != nil {
		return nil, err
	}

	// Assign target user to channel members UA so both have access
	if _, err := s.policyClient.CreateAssignment(ctx, &policypb.CreateAssignmentRequest{
		ChildId: req.TargetNgacNodeId, ParentId: ch.NgacUaId,
	}); err != nil {
		return nil, status.Errorf(codes.Internal, "assign target to DM: %v", err)
	}

	return ch, nil
}

// findExistingDM searches for an existing DM channel where both users are members.
func (s *MessagingServer) findExistingDM(ctx context.Context, userNodeID, targetNodeID string) (*pb.Channel, error) {
	rows, err := s.db.Query(ctx,
		"SELECT id, name, channel_type, COALESCE(workspace_id,''), ngac_oa_id, ngac_ua_id, created_by, created_at FROM channels WHERE channel_type = 'dm'")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var ch pb.Channel
		var ca time.Time
		if err := rows.Scan(&ch.Id, &ch.Name, &ch.ChannelType, &ch.WorkspaceId, &ch.NgacOaId, &ch.NgacUaId, &ch.CreatedBy, &ca); err != nil {
			continue
		}
		ch.CreatedAt = timestamppb.New(ca)

		// Check if both users are assigned to the channel's members UA
		children, err := s.policyClient.GetChildren(ctx, &policypb.GetChildrenRequest{NodeId: ch.NgacUaId})
		if err != nil || children == nil {
			continue
		}
		hasUser, hasTarget := false, false
		for _, n := range children.Nodes {
			if n.Id == userNodeID {
				hasUser = true
			}
			if n.Id == targetNodeID {
				hasTarget = true
			}
		}
		if hasUser && hasTarget {
			return &ch, nil
		}
	}
	return nil, fmt.Errorf("no existing DM found")
}

func (s *MessagingServer) ListDMs(ctx context.Context, req *pb.ListDMsRequest) (*pb.ChannelList, error) {
	rows, err := s.db.Query(ctx,
		"SELECT id, name, channel_type, COALESCE(workspace_id,''), ngac_oa_id, ngac_ua_id, created_by, created_at FROM channels WHERE channel_type = 'dm'")
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list DMs: %v", err)
	}
	defer rows.Close()

	var channels []*pb.Channel
	for rows.Next() {
		var ch pb.Channel
		var ca time.Time
		if err := rows.Scan(&ch.Id, &ch.Name, &ch.ChannelType, &ch.WorkspaceId, &ch.NgacOaId, &ch.NgacUaId, &ch.CreatedBy, &ca); err != nil {
			return nil, err
		}
		ch.CreatedAt = timestamppb.New(ca)
		// Filter: user must be a member
		resp, _ := s.policyClient.CheckAccess(ctx, &policypb.CheckAccessRequest{
			UserNodeId: req.UserNgacNodeId, ObjectNodeId: ch.NgacOaId, Operation: "read",
		})
		if resp != nil && resp.Decision == "ALLOW" {
			channels = append(channels, &ch)
		}
	}
	return &pb.ChannelList{Channels: channels}, nil
}
