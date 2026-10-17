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
	// req.ChannelType is not read: a group made by a user is private, and the
	// workspace-wide channel has its own method.
	ch, err := s.svc.CreateChannel(ctx, domain.CreateChannelInput{
		Name:        req.Name,
		WorkspaceID: req.WorkspaceId,
		UserID:      grpcauth.CallerFrom(ctx).UserID,
		UserNodeID:  grpcauth.CallerFrom(ctx).NGACNodeID,
	})
	if err != nil {
		return nil, domainError("create channel", err)
	}
	return ch, nil
}

// ProvisionWorkspaceChannel makes the workspace's workspace-wide channel. Only the
// auth service may call it (see ServerPolicy); a person's identity is refused even
// though the interceptor would admit it.
func (s *MessagingServer) ProvisionWorkspaceChannel(ctx context.Context, req *pb.ProvisionWorkspaceChannelRequest) (*pb.Channel, error) {
	if err := requireService(ctx, "auth"); err != nil {
		return nil, err
	}
	ch, err := s.svc.ProvisionWorkspaceChannel(ctx, domain.ProvisionWorkspaceChannelInput{
		WorkspaceID:   req.WorkspaceId,
		Name:          req.Name,
		CreatorUserID: req.CreatorUserId,
		CreatorNodeID: req.CreatorNgacNodeId,
	})
	if err != nil {
		return nil, domainError("provision workspace channel", err)
	}
	return ch, nil
}

// JoinWorkspaceChannel puts a member of the workspace into its workspace-wide
// channel. Services only.
func (s *MessagingServer) JoinWorkspaceChannel(ctx context.Context, req *pb.JoinWorkspaceChannelRequest) (*pb.Empty, error) {
	if err := requireService(ctx, "auth", "workspace"); err != nil {
		return nil, err
	}
	if err := s.svc.JoinWorkspaceChannel(ctx, req.WorkspaceId, req.UserId, req.UserNgacNodeId); err != nil {
		return nil, domainError("join workspace channel", err)
	}
	return &pb.Empty{}, nil
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
	attachments := make([]domain.AttachmentInput, 0, len(req.Attachments))
	for _, a := range req.Attachments {
		attachments = append(attachments, domain.AttachmentInput{DriveFileID: a.DriveFileId, Width: a.Width, Height: a.Height})
	}
	msg, err := s.svc.SendMessage(ctx, domain.SendMessageInput{
		Attachments:      attachments,
		QuotedMessageID:  req.QuotedMessageId,
		ContentFormat:    req.ContentFormat,
		Mentions:         req.Mentions,
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

	s.svc.PushLater(ctx, msg)

	// Broadcast via WebSocket hub (fire-and-forget): a root message as it is,
	// a reply as its thread's summary plus each follower's own state.
	if s.hub != nil {
		if msg.ParentMessageId == "" {
			s.hub.BroadcastToChannel(req.ChannelId, msg)
		} else {
			s.svc.AnnounceReply(ctx, msg, grpcauth.CallerFrom(ctx).TenantID, s.hub)
		}
	}

	// Publish to Kafka for async processing (fire-and-forget).
	if s.producer != nil {
		s.producer.PublishMessageSent(req.ChannelId, grpcauth.CallerFrom(ctx).UserID)
	}

	return msg, nil
}

// GetMessages delegates to domain.Service.GetMessages.
func (s *MessagingServer) GetMessages(ctx context.Context, req *pb.GetMessagesRequest) (*pb.MessageList, error) {
	caller := grpcauth.CallerFrom(ctx)
	list, err := s.svc.GetMessages(ctx, req.ChannelId, caller.UserID, caller.NGACNodeID, req.Before, int(req.Limit))
	if err != nil {
		return nil, domainError("get messages", err)
	}
	return list, nil
}

// GetThread delegates to domain.Service.GetThread.
func (s *MessagingServer) GetThread(ctx context.Context, req *pb.GetThreadRequest) (*pb.MessageList, error) {
	caller := grpcauth.CallerFrom(ctx)
	list, err := s.svc.GetThread(ctx, req.MessageId, caller.UserID, caller.NGACNodeID)
	if err != nil {
		return nil, domainError("get thread", err)
	}
	return list, nil
}

// threadStateAnswer sends the caller's new thread state to their sessions.
func (s *MessagingServer) threadStateAnswer(ctx context.Context, what string, st *pb.ThreadStateEvent, err error) (*pb.Empty, error) {
	if err != nil {
		return nil, domainError(what, err)
	}
	if s.hub != nil {
		caller := grpcauth.CallerFrom(ctx)
		s.hub.SendThreadStates(caller.TenantID, map[string]*pb.ThreadStateEvent{caller.NGACNodeID: st})
	}
	return &pb.Empty{}, nil
}

// FollowThread delegates to domain.Service.FollowThread.
func (s *MessagingServer) FollowThread(ctx context.Context, req *pb.ThreadRequest) (*pb.Empty, error) {
	caller := grpcauth.CallerFrom(ctx)
	st, err := s.svc.FollowThread(ctx, req.MessageId, caller.UserID, caller.NGACNodeID)
	return s.threadStateAnswer(ctx, "follow thread", st, err)
}

// UnfollowThread delegates to domain.Service.UnfollowThread.
func (s *MessagingServer) UnfollowThread(ctx context.Context, req *pb.ThreadRequest) (*pb.Empty, error) {
	caller := grpcauth.CallerFrom(ctx)
	st, err := s.svc.UnfollowThread(ctx, req.MessageId, caller.UserID, caller.NGACNodeID)
	return s.threadStateAnswer(ctx, "unfollow thread", st, err)
}

// MarkThreadRead delegates to domain.Service.MarkThreadRead.
func (s *MessagingServer) MarkThreadRead(ctx context.Context, req *pb.ThreadRequest) (*pb.Empty, error) {
	caller := grpcauth.CallerFrom(ctx)
	st, err := s.svc.MarkThreadRead(ctx, req.MessageId, caller.UserID, caller.NGACNodeID)
	return s.threadStateAnswer(ctx, "mark thread read", st, err)
}

// ListThreads delegates to domain.Service.ListThreads.
func (s *MessagingServer) ListThreads(ctx context.Context, req *pb.ListThreadsRequest) (*pb.MessageList, error) {
	caller := grpcauth.CallerFrom(ctx)
	list, err := s.svc.ListThreads(ctx, req.ChannelId, caller.UserID, caller.NGACNodeID, req.Filter, req.Before, req.BeforeId, int(req.Limit))
	if err != nil {
		return nil, domainError("list threads", err)
	}
	return list, nil
}

// EditMessage delegates to domain.Service.EditMessage and tells the channel.
func (s *MessagingServer) EditMessage(ctx context.Context, req *pb.EditMessageRequest) (*pb.Message, error) {
	caller := grpcauth.CallerFrom(ctx)
	msg, err := s.svc.EditMessage(ctx, req.MessageId, caller.UserID, caller.NGACNodeID, req.Content, req.ContentFormat, nil)
	if err != nil {
		return nil, domainError("edit message", err)
	}
	if s.hub != nil {
		s.hub.BroadcastMessageEdited(msg.ChannelId, msg)
	}
	return msg, nil
}

// DeleteMessage delegates to domain.Service.DeleteMessage and tells the channel.
func (s *MessagingServer) DeleteMessage(ctx context.Context, req *pb.MessageRef) (*pb.Empty, error) {
	caller := grpcauth.CallerFrom(ctx)
	out, err := s.svc.DeleteMessage(ctx, req.MessageId, caller.UserID, caller.NGACNodeID)
	if err != nil {
		return nil, domainError("delete message", err)
	}
	if s.hub != nil {
		s.hub.BroadcastMessageDeleted(out.Event.ChannelId, out.Event)
		if out.Notice != nil {
			s.hub.BroadcastToChannel(out.Notice.ChannelId, out.Notice)
		}
	}
	return &pb.Empty{}, nil
}

// MarkMessageUnread delegates to domain.Service.MarkMessageUnread.
func (s *MessagingServer) MarkMessageUnread(ctx context.Context, req *pb.MessageRef) (*pb.Empty, error) {
	caller := grpcauth.CallerFrom(ctx)
	if err := s.svc.MarkMessageUnread(ctx, req.MessageId, caller.UserID, caller.NGACNodeID); err != nil {
		return nil, domainError("mark message unread", err)
	}
	return &pb.Empty{}, nil
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
