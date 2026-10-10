package grpc

import (
	"context"

	"google.golang.org/protobuf/types/known/timestamppb"

	"ngac-platform/pkg/grpcauth"
	pb "ngac-platform/proto/messaging"
	"ngac-platform/services/messaging/internal/domain"
	"ngac-platform/services/messaging/internal/store"
)

// NotificationServer is the gRPC transport of the notification service. It runs
// no SQL: reading, marking and creating notifications are the domain service's.
type NotificationServer struct {
	pb.UnimplementedNotificationServiceServer
	svc *domain.NotificationService
}

// NewNotificationServer wraps the notification service.
func NewNotificationServer(svc *domain.NotificationService) *NotificationServer {
	return &NotificationServer{svc: svc}
}

// ListNotifications returns the caller's notifications, newest first.
func (s *NotificationServer) ListNotifications(ctx context.Context, req *pb.ListNotificationsRequest) (*pb.NotificationList, error) {
	page, err := s.svc.List(ctx, grpcauth.CallerFrom(ctx).UserID, int(req.Limit), int(req.Offset))
	if err != nil {
		return nil, domainError("list notifications", err)
	}
	out := &pb.NotificationList{Total: int32(page.Total), UnreadCount: int32(page.Unread)}
	for _, n := range page.Items {
		out.Notifications = append(out.Notifications, notificationToProto(n))
	}
	return out, nil
}

// MarkRead marks one of the caller's notifications read.
func (s *NotificationServer) MarkRead(ctx context.Context, req *pb.MarkNotificationReadRequest) (*pb.Empty, error) {
	if err := s.svc.MarkRead(ctx, grpcauth.CallerFrom(ctx).UserID, req.NotificationId); err != nil {
		return nil, domainError("mark read", err)
	}
	return &pb.Empty{}, nil
}

// MarkAllRead marks all of the caller's notifications read.
func (s *NotificationServer) MarkAllRead(ctx context.Context, req *pb.MarkAllNotificationsReadRequest) (*pb.Empty, error) {
	if err := s.svc.MarkAllRead(ctx, grpcauth.CallerFrom(ctx).UserID); err != nil {
		return nil, domainError("mark all read", err)
	}
	return &pb.Empty{}, nil
}

// GetUnreadCount returns how many notifications the caller has not read.
func (s *NotificationServer) GetUnreadCount(ctx context.Context, req *pb.GetNotificationUnreadCountRequest) (*pb.NotificationUnreadCountResponse, error) {
	n, err := s.svc.UnreadCount(ctx, grpcauth.CallerFrom(ctx).UserID)
	if err != nil {
		return nil, domainError("get unread count", err)
	}
	return &pb.NotificationUnreadCountResponse{Count: int32(n)}, nil
}

func notificationToProto(n *store.Notification) *pb.Notification {
	return &pb.Notification{
		Id: n.ID, UserId: n.UserID, Type: n.Type, Title: n.Title, Body: n.Body,
		EntityType: n.EntityType, EntityId: n.EntityID, Read: n.Read,
		CreatedAt: timestamppb.New(n.CreatedAt),
	}
}
