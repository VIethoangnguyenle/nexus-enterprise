package domain

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "ngac-platform/proto/messaging"
	"ngac-platform/services/messaging/internal/store"
)

// NotificationStore is the persistence the notification service needs.
// *store.Store implements it.
type NotificationStore interface {
	InsertNotification(ctx context.Context, n *store.Notification) error
	ListNotifications(ctx context.Context, userID string, limit, offset int) ([]*store.Notification, error)
	NotificationCounts(ctx context.Context, userID string) (total, unread int, err error)
	MarkNotificationRead(ctx context.Context, id, userID string) error
	MarkAllNotificationsRead(ctx context.Context, userID string) error
}

// NotificationPusher delivers a new notification to the user's open connections.
// The WebSocket hub implements it.
type NotificationPusher interface {
	SendNotification(userID string, n *pb.Notification)
}

// Notification page bounds.
const (
	DefaultNotificationPage = 25
	MaxNotificationPage     = 50
)

// NotificationService owns a user's notifications: listing, reading, creating.
// Both the gRPC and the REST edge call it; neither runs SQL.
type NotificationService struct {
	store NotificationStore
	push  NotificationPusher
	now   func() time.Time
}

// NewNotificationService returns the service. push may be nil: notifications are
// then stored without being pushed live.
func NewNotificationService(st NotificationStore, push NotificationPusher) *NotificationService {
	return &NotificationService{store: st, push: push, now: time.Now}
}

// NotificationPage is one page of a user's notifications and their totals.
type NotificationPage struct {
	Items  []*store.Notification
	Total  int
	Unread int
}

// List returns one page of the user's notifications, newest first. A limit
// outside 1..MaxNotificationPage becomes DefaultNotificationPage.
func (s *NotificationService) List(ctx context.Context, userID string, limit, offset int) (*NotificationPage, error) {
	if userID == "" {
		return nil, fmt.Errorf("%w: a signed-in user is required", ErrAccessDenied)
	}
	if limit <= 0 || limit > MaxNotificationPage {
		limit = DefaultNotificationPage
	}
	if offset < 0 {
		offset = 0
	}
	total, unread, err := s.store.NotificationCounts(ctx, userID)
	if err != nil {
		return nil, err
	}
	items, err := s.store.ListNotifications(ctx, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	return &NotificationPage{Items: items, Total: total, Unread: unread}, nil
}

// UnreadCount returns how many notifications the user has not read.
func (s *NotificationService) UnreadCount(ctx context.Context, userID string) (int, error) {
	if userID == "" {
		return 0, fmt.Errorf("%w: a signed-in user is required", ErrAccessDenied)
	}
	_, unread, err := s.store.NotificationCounts(ctx, userID)
	return unread, err
}

// MarkRead marks one of the user's notifications read. A notification that is
// not theirs is left alone.
func (s *NotificationService) MarkRead(ctx context.Context, userID, notificationID string) error {
	if userID == "" {
		return fmt.Errorf("%w: a signed-in user is required", ErrAccessDenied)
	}
	return s.store.MarkNotificationRead(ctx, notificationID, userID)
}

// MarkAllRead marks all of the user's notifications read.
func (s *NotificationService) MarkAllRead(ctx context.Context, userID string) error {
	if userID == "" {
		return fmt.Errorf("%w: a signed-in user is required", ErrAccessDenied)
	}
	return s.store.MarkAllNotificationsRead(ctx, userID)
}

// CreateNotification stores a notification for userID and pushes it to their
// open connections. The Kafka consumer calls it; there is no caller to
// authorize. The push happens only after the row is stored.
func (s *NotificationService) CreateNotification(ctx context.Context, userID, notifType, title, body, entityType, entityID string) error {
	n := &store.Notification{
		ID: uuid.New().String(), UserID: userID, Type: notifType, Title: title, Body: body,
		EntityType: entityType, EntityID: entityID, CreatedAt: s.now(),
	}
	if err := s.store.InsertNotification(ctx, n); err != nil {
		return err
	}
	if s.push != nil {
		s.push.SendNotification(userID, &pb.Notification{
			Id: n.ID, UserId: userID, Type: notifType, Title: title, Body: body,
			EntityType: entityType, EntityId: entityID, Read: false, CreatedAt: timestamppb.New(n.CreatedAt),
		})
	}
	return nil
}
