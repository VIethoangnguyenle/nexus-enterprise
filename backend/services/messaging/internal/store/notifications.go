package store

import (
	"context"
	"fmt"
	"time"
)

// Notification is a row of the notifications table.
type Notification struct {
	ID         string
	UserID     string
	Type       string
	Title      string
	Body       string
	EntityType string
	EntityID   string
	Read       bool
	CreatedAt  time.Time
}

// InsertNotification stores a notification. An empty EntityType or EntityID is
// stored as NULL.
func (s *Store) InsertNotification(ctx context.Context, n *Notification) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO notifications (id, user_id, type, title, body, entity_type, entity_id, created_at)
		 VALUES ($1, $2, $3, $4, $5, NULLIF($6, ''), NULLIF($7, ''), $8)`,
		n.ID, n.UserID, n.Type, n.Title, n.Body, n.EntityType, n.EntityID, n.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert notification: %w", err)
	}
	return nil
}

// ListNotifications returns one user's notifications, newest first.
func (s *Store) ListNotifications(ctx context.Context, userID string, limit, offset int) ([]*Notification, error) {
	rows, err := s.db.Query(ctx,
		`SELECT id, user_id, type, title, body, COALESCE(entity_type,''), COALESCE(entity_id,''), read, created_at
		 FROM notifications WHERE user_id = $1
		 ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
		userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list notifications: %w", err)
	}
	defer rows.Close()

	var out []*Notification
	for rows.Next() {
		var n Notification
		if err := rows.Scan(&n.ID, &n.UserID, &n.Type, &n.Title, &n.Body, &n.EntityType, &n.EntityID, &n.Read, &n.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan notification: %w", err)
		}
		out = append(out, &n)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list notifications: %w", err)
	}
	return out, nil
}

// NotificationCounts returns how many notifications a user has and how many of
// them are unread.
func (s *Store) NotificationCounts(ctx context.Context, userID string) (total, unread int, err error) {
	err = s.db.QueryRow(ctx,
		`SELECT COUNT(*), COUNT(*) FILTER (WHERE read = FALSE) FROM notifications WHERE user_id = $1`,
		userID).Scan(&total, &unread)
	if err != nil {
		return 0, 0, fmt.Errorf("count notifications: %w", err)
	}
	return total, unread, nil
}

// MarkNotificationRead marks one of the user's notifications read. Another
// user's notification is not theirs to mark: it matches no row and nothing
// changes.
func (s *Store) MarkNotificationRead(ctx context.Context, id, userID string) error {
	if _, err := s.db.Exec(ctx,
		`UPDATE notifications SET read = TRUE WHERE id = $1 AND user_id = $2`, id, userID); err != nil {
		return fmt.Errorf("mark notification read: %w", err)
	}
	return nil
}

// MarkAllNotificationsRead marks every unread notification of the user read.
func (s *Store) MarkAllNotificationsRead(ctx context.Context, userID string) error {
	if _, err := s.db.Exec(ctx,
		`UPDATE notifications SET read = TRUE WHERE user_id = $1 AND read = FALSE`, userID); err != nil {
		return fmt.Errorf("mark all notifications read: %w", err)
	}
	return nil
}
