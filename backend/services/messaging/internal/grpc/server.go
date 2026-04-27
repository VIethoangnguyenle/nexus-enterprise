// Package grpc provides thin gRPC handlers for the messaging service.
// Each handler parses the request, delegates to the domain layer, and returns the response.
// No SQL, no business logic, no direct policy calls.
package grpc

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "ngac-platform/proto/messaging"
	"ngac-platform/services/messaging/internal/domain"
	"ngac-platform/services/messaging/internal/events"
)

// MessagingServer implements the MessagingService gRPC interface.
type MessagingServer struct {
	pb.UnimplementedMessagingServiceServer
	svc      *domain.Service
	hub      *Hub
	producer *events.Producer
}

// NewMessagingServer creates a thin gRPC handler backed by the domain service.
func NewMessagingServer(svc *domain.Service, hub *Hub, producer *events.Producer) *MessagingServer {
	return &MessagingServer{svc: svc, hub: hub, producer: producer}
}

// CreateChannel delegates to domain.Service.CreateChannel.
func (s *MessagingServer) CreateChannel(ctx context.Context, req *pb.CreateChannelRequest) (*pb.Channel, error) {
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "channel name required")
	}
	chType := req.ChannelType
	if chType == "" {
		chType = "workspace"
	}
	ch, err := s.svc.CreateChannel(ctx, domain.CreateChannelInput{
		Name:        req.Name,
		WorkspaceID: req.WorkspaceId,
		UserID:      req.UserId,
		UserNodeID:  req.UserNgacNodeId,
		ChannelType: chType,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "create channel: %v", err)
	}
	return ch, nil
}

// ListChannels delegates to domain.Service.ListChannels.
func (s *MessagingServer) ListChannels(ctx context.Context, req *pb.ListChannelsRequest) (*pb.ChannelList, error) {
	channels, err := s.svc.ListChannels(ctx, req.WorkspaceId, req.UserNgacNodeId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list channels: %v", err)
	}
	return &pb.ChannelList{Channels: channels}, nil
}

// GetChannel delegates to domain.Service.GetChannel.
func (s *MessagingServer) GetChannel(ctx context.Context, req *pb.GetChannelRequest) (*pb.Channel, error) {
	ch, err := s.svc.GetChannel(ctx, req.ChannelId)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "channel not found")
	}
	return ch, nil
}

// ListDMs delegates to domain.Service.ListDMs.
func (s *MessagingServer) ListDMs(ctx context.Context, req *pb.ListDMsRequest) (*pb.ChannelList, error) {
	channels, err := s.svc.ListDMs(ctx, req.UserNgacNodeId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list DMs: %v", err)
	}
	return &pb.ChannelList{Channels: channels}, nil
}

// CreateDM delegates to domain.Service.FindOrCreateDM.
func (s *MessagingServer) CreateDM(ctx context.Context, req *pb.CreateDMRequest) (*pb.Channel, error) {
	ch, err := s.svc.FindOrCreateDM(ctx, req.UserId, req.UserNgacNodeId, req.TargetUserId, req.TargetNgacNodeId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "create DM: %v", err)
	}
	return ch, nil
}

// SendMessage delegates to domain.Service.SendMessage and broadcasts via WebSocket.
func (s *MessagingServer) SendMessage(ctx context.Context, req *pb.SendMessageRequest) (*pb.Message, error) {
	msg, err := s.svc.SendMessage(ctx, domain.SendMessageInput{
		ChannelID:        req.ChannelId,
		SenderID:         req.SenderId,
		SenderNodeID:     req.SenderNgacNodeId,
		Content:          req.Content,
		MessageType:      req.MessageType,
		ParentMessageID:  req.ParentMessageId,
		LinkedEntityType: req.LinkedEntityType,
		LinkedEntityID:   req.LinkedEntityId,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "send message: %v", err)
	}

	// Broadcast via WebSocket hub (fire-and-forget).
	s.hub.BroadcastToChannel(req.ChannelId, msg)

	// Publish to Kafka for async processing (fire-and-forget).
	if s.producer != nil {
		s.producer.PublishMessageSent(req.ChannelId, req.SenderId)
	}

	return msg, nil
}

// GetMessages delegates to domain.Service.GetMessages.
func (s *MessagingServer) GetMessages(ctx context.Context, req *pb.GetMessagesRequest) (*pb.MessageList, error) {
	list, err := s.svc.GetMessages(ctx, req.ChannelId, req.UserNgacNodeId, req.Before, int(req.Limit))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get messages: %v", err)
	}
	return list, nil
}

// GetThread delegates to domain.Service.GetThread.
func (s *MessagingServer) GetThread(ctx context.Context, req *pb.GetThreadRequest) (*pb.MessageList, error) {
	list, err := s.svc.GetThread(ctx, req.MessageId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get thread: %v", err)
	}
	return list, nil
}

// FindThreadsByEntity delegates to domain.Service.FindThreadsByEntity.
func (s *MessagingServer) FindThreadsByEntity(ctx context.Context, req *pb.FindThreadsByEntityRequest) (*pb.MessageList, error) {
	list, err := s.svc.FindThreadsByEntity(ctx, req.EntityType, req.EntityId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "find threads: %v", err)
	}
	return list, nil
}

// AddChannelMember delegates to domain.Service.AddMember.
func (s *MessagingServer) AddChannelMember(ctx context.Context, req *pb.AddChannelMemberRequest) (*pb.Empty, error) {
	if err := s.svc.AddMember(ctx, req.ChannelId, req.RequesterNgacNodeId, req.TargetNgacNodeId); err != nil {
		return nil, status.Errorf(codes.Internal, "add member: %v", err)
	}
	return &pb.Empty{}, nil
}

// RemoveChannelMember delegates to domain.Service.RemoveMember.
func (s *MessagingServer) RemoveChannelMember(ctx context.Context, req *pb.RemoveChannelMemberRequest) (*pb.Empty, error) {
	if err := s.svc.RemoveMember(ctx, req.ChannelId, req.TargetNgacNodeId); err != nil {
		return nil, status.Errorf(codes.Internal, "remove member: %v", err)
	}
	return &pb.Empty{}, nil
}

// ListChannelMembers delegates to domain.Service.ListMembers.
func (s *MessagingServer) ListChannelMembers(ctx context.Context, req *pb.ListChannelMembersRequest) (*pb.ChannelMemberList, error) {
	members, err := s.svc.ListMembers(ctx, req.ChannelId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list members: %v", err)
	}
	return &pb.ChannelMemberList{Members: members}, nil
}
