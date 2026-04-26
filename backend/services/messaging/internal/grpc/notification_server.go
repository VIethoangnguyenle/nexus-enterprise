package grpc

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "ngac-platform/proto/messaging"
)

// NotificationServer handles gRPC calls for the notification system.
type NotificationServer struct {
	pb.UnimplementedNotificationServiceServer
	db  *pgxpool.Pool
	hub *Hub
}

// NewNotificationServer creates the notification gRPC handler.
func NewNotificationServer(db *pgxpool.Pool, hub *Hub) *NotificationServer {
	return &NotificationServer{db: db, hub: hub}
}

func (s *NotificationServer) ListNotifications(ctx context.Context, req *pb.ListNotificationsRequest) (*pb.NotificationList, error) {
	limit := int(req.Limit)
	if limit <= 0 || limit > 50 {
		limit = 25
	}

	// Count total and unread
	var total, unread int32
	s.db.QueryRow(ctx,
		"SELECT COUNT(*), COUNT(*) FILTER (WHERE read = FALSE) FROM notifications WHERE user_id = $1",
		req.UserId,
	).Scan(&total, &unread)

	rows, err := s.db.Query(ctx,
		`SELECT id, user_id, type, title, body, COALESCE(entity_type,''), COALESCE(entity_id,''), read, created_at
		 FROM notifications WHERE user_id = $1
		 ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
		req.UserId, limit, req.Offset,
	)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list notifications: %v", err)
	}
	defer rows.Close()

	var notifs []*pb.Notification
	for rows.Next() {
		var n pb.Notification
		var ca time.Time
		if err := rows.Scan(&n.Id, &n.UserId, &n.Type, &n.Title, &n.Body,
			&n.EntityType, &n.EntityId, &n.Read, &ca,
		); err != nil {
			return nil, err
		}
		n.CreatedAt = timestamppb.New(ca)
		notifs = append(notifs, &n)
	}

	return &pb.NotificationList{
		Notifications: notifs,
		Total:         total,
		UnreadCount:   unread,
	}, nil
}

func (s *NotificationServer) MarkRead(ctx context.Context, req *pb.MarkReadRequest) (*pb.Empty, error) {
	_, err := s.db.Exec(ctx,
		"UPDATE notifications SET read = TRUE WHERE id = $1 AND user_id = $2",
		req.NotificationId, req.UserId,
	)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "mark read: %v", err)
	}
	return &pb.Empty{}, nil
}

func (s *NotificationServer) MarkAllRead(ctx context.Context, req *pb.MarkAllReadRequest) (*pb.Empty, error) {
	_, err := s.db.Exec(ctx,
		"UPDATE notifications SET read = TRUE WHERE user_id = $1 AND read = FALSE",
		req.UserId,
	)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "mark all read: %v", err)
	}
	return &pb.Empty{}, nil
}

func (s *NotificationServer) GetUnreadCount(ctx context.Context, req *pb.GetUnreadCountRequest) (*pb.UnreadCountResponse, error) {
	var count int32
	err := s.db.QueryRow(ctx,
		"SELECT COUNT(*) FROM notifications WHERE user_id = $1 AND read = FALSE",
		req.UserId,
	).Scan(&count)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get unread count: %v", err)
	}
	return &pb.UnreadCountResponse{Count: count}, nil
}

// CreateNotification inserts a notification and pushes to connected WebSocket.
// This is called internally (not via gRPC) by the Kafka consumer.
func (s *NotificationServer) CreateNotification(ctx context.Context, userID, notifType, title, body, entityType, entityID string) error {
	id := uuid.New().String()
	now := time.Now()
	_, err := s.db.Exec(ctx,
		`INSERT INTO notifications (id, user_id, type, title, body, entity_type, entity_id, created_at)
		 VALUES ($1, $2, $3, $4, $5, NULLIF($6, ''), NULLIF($7, ''), $8)`,
		id, userID, notifType, title, body, entityType, entityID, now,
	)
	if err != nil {
		return err
	}

	// Push to WebSocket if user is online
	if s.hub != nil {
		s.hub.SendNotification(userID, &pb.Notification{
			Id:         id,
			UserId:     userID,
			Type:       notifType,
			Title:      title,
			Body:       body,
			EntityType: entityType,
			EntityId:   entityID,
			Read:       false,
			CreatedAt:  timestamppb.New(now),
		})
	}
	return nil
}
