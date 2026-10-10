// Package grpc provides thin gRPC handlers for the messaging service.
// Each handler parses the request, delegates to the domain layer, and returns the response.
// No SQL, no business logic, no direct policy calls.
package grpc

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"ngac-platform/pkg/grpcauth"
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
		UserID:      grpcauth.CallerFrom(ctx).UserID,
		UserNodeID:  grpcauth.CallerFrom(ctx).NGACNodeID,
		ChannelType: chType,
	})
	if err != nil {
		return nil, domainError("create channel", err)
	}
	return ch, nil
}

// ListChannels delegates to domain.Service.ListChannels.
func (s *MessagingServer) ListChannels(ctx context.Context, req *pb.ListChannelsRequest) (*pb.ChannelList, error) {
	channels, err := s.svc.ListChannels(ctx, req.WorkspaceId, grpcauth.CallerFrom(ctx).NGACNodeID)
	if err != nil {
		return nil, domainError("list channels", err)
	}
	return &pb.ChannelList{Channels: channels}, nil
}

// GetChannel delegates to domain.Service.GetChannel.
func (s *MessagingServer) GetChannel(ctx context.Context, req *pb.GetChannelRequest) (*pb.Channel, error) {
	ch, err := s.svc.GetChannel(ctx, req.ChannelId, grpcauth.CallerFrom(ctx).NGACNodeID)
	if err != nil {
		return nil, domainError("get channel", err)
	}
	return ch, nil
}

// ListDMs delegates to domain.Service.ListDMs.
func (s *MessagingServer) ListDMs(ctx context.Context, req *pb.ListDMsRequest) (*pb.ChannelList, error) {
	channels, err := s.svc.ListDMs(ctx, grpcauth.CallerFrom(ctx).NGACNodeID)
	if err != nil {
		return nil, domainError("list DMs", err)
	}
	return &pb.ChannelList{Channels: channels}, nil
}

// CreateDM delegates to domain.Service.FindOrCreateDM.
func (s *MessagingServer) CreateDM(ctx context.Context, req *pb.CreateDMRequest) (*pb.Channel, error) {
	ch, err := s.svc.FindOrCreateDM(ctx, grpcauth.CallerFrom(ctx).UserID, grpcauth.CallerFrom(ctx).NGACNodeID, req.TargetUserId, req.TargetNgacNodeId)
	if err != nil {
		return nil, domainError("create DM", err)
	}
	return ch, nil
}

// SendMessage delegates to domain.Service.SendMessage and broadcasts via WebSocket.
func (s *MessagingServer) SendMessage(ctx context.Context, req *pb.SendMessageRequest) (*pb.Message, error) {
	msg, err := s.svc.SendMessage(ctx, domain.SendMessageInput{
		ChannelID:        req.ChannelId,
		SenderID:         grpcauth.CallerFrom(ctx).UserID,
		SenderNodeID:     grpcauth.CallerFrom(ctx).NGACNodeID,
		Content:          req.Content,
		MessageType:      req.MessageType,
		ParentMessageID:  req.ParentMessageId,
		LinkedEntityType: req.LinkedEntityType,
		LinkedEntityID:   req.LinkedEntityId,
	})
	if err != nil {
		return nil, domainError("send message", err)
	}

	// Broadcast via WebSocket hub (fire-and-forget).
	if s.hub != nil {
		s.hub.BroadcastToChannel(req.ChannelId, msg)
	}

	// Publish to Kafka for async processing (fire-and-forget).
	if s.producer != nil {
		s.producer.PublishMessageSent(req.ChannelId, grpcauth.CallerFrom(ctx).UserID)
	}

	return msg, nil
}

// GetMessages delegates to domain.Service.GetMessages.
func (s *MessagingServer) GetMessages(ctx context.Context, req *pb.GetMessagesRequest) (*pb.MessageList, error) {
	list, err := s.svc.GetMessages(ctx, req.ChannelId, grpcauth.CallerFrom(ctx).NGACNodeID, req.Before, int(req.Limit))
	if err != nil {
		return nil, domainError("get messages", err)
	}
	return list, nil
}

// GetThread delegates to domain.Service.GetThread.
func (s *MessagingServer) GetThread(ctx context.Context, req *pb.GetThreadRequest) (*pb.MessageList, error) {
	list, err := s.svc.GetThread(ctx, req.MessageId, grpcauth.CallerFrom(ctx).NGACNodeID)
	if err != nil {
		return nil, domainError("get thread", err)
	}
	return list, nil
}

// FindThreadsByEntity delegates to domain.Service.FindThreadsByEntity.
func (s *MessagingServer) FindThreadsByEntity(ctx context.Context, req *pb.FindThreadsByEntityRequest) (*pb.MessageList, error) {
	list, err := s.svc.FindThreadsByEntity(ctx, req.EntityType, req.EntityId, grpcauth.CallerFrom(ctx).NGACNodeID)
	if err != nil {
		return nil, domainError("find threads", err)
	}
	return list, nil
}

// AddChannelMember delegates to domain.Service.AddMember.
func (s *MessagingServer) AddChannelMember(ctx context.Context, req *pb.AddChannelMemberRequest) (*pb.Empty, error) {
	if err := s.svc.AddMember(ctx, req.ChannelId, grpcauth.CallerFrom(ctx).NGACNodeID, req.TargetNgacNodeId); err != nil {
		return nil, domainError("add member", err)
	}
	return &pb.Empty{}, nil
}

// RemoveChannelMember delegates to domain.Service.RemoveMember.
func (s *MessagingServer) RemoveChannelMember(ctx context.Context, req *pb.RemoveChannelMemberRequest) (*pb.Empty, error) {
	if err := s.svc.RemoveMember(ctx, req.ChannelId, grpcauth.CallerFrom(ctx).NGACNodeID, req.TargetNgacNodeId); err != nil {
		return nil, domainError("remove member", err)
	}
	return &pb.Empty{}, nil
}

// ListChannelMembers delegates to domain.Service.ListMembers.
func (s *MessagingServer) ListChannelMembers(ctx context.Context, req *pb.ListChannelMembersRequest) (*pb.ChannelMemberList, error) {
	members, err := s.svc.ListMembers(ctx, req.ChannelId, grpcauth.CallerFrom(ctx).NGACNodeID)
	if err != nil {
		return nil, domainError("list members", err)
	}
	return &pb.ChannelMemberList{Members: members}, nil
}
