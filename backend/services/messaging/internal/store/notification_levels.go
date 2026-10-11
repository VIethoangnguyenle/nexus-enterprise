package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// NotificationLevel returns the user's level for a channel, "main" when they
// never chose one.
func (s *Store) NotificationLevel(ctx context.Context, userID, channelID string) (string, error) {
	var level string
	err := s.db.QueryRow(ctx,
		`SELECT level FROM channel_notification_levels WHERE user_id = $1 AND channel_id = $2`,
		userID, channelID).Scan(&level)
	if errors.Is(err, pgx.ErrNoRows) {
		return "main", nil
	}
	if err != nil {
		return "", fmt.Errorf("notification level: %w", err)
	}
	return level, nil
}

// SetNotificationLevel stores the user's level for a channel.
func (s *Store) SetNotificationLevel(ctx context.Context, userID, channelID, level string) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO channel_notification_levels (user_id, channel_id, level) VALUES ($1, $2, $3)
		 ON CONFLICT (user_id, channel_id) DO UPDATE SET level = EXCLUDED.level, updated_at = NOW()`,
		userID, channelID, level)
	if err != nil {
		return fmt.Errorf("set notification level: %w", err)
	}
	return nil
}

// NotificationLevels returns the levels the given users chose for a channel;
// a user who never chose is absent (the default, "main").
func (s *Store) NotificationLevels(ctx context.Context, channelID string, userIDs []string) (map[string]string, error) {
	out := map[string]string{}
	if len(userIDs) == 0 {
		return out, nil
	}
	rows, err := s.db.Query(ctx,
		`SELECT user_id, level FROM channel_notification_levels WHERE channel_id = $1 AND user_id = ANY($2::text[])`,
		channelID, userIDs)
	if err != nil {
		return nil, fmt.Errorf("notification levels: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id, level string
		if err := rows.Scan(&id, &level); err != nil {
			return nil, err
		}
		out[id] = level
	}
	return out, rows.Err()
}

// PeopleWithLevel returns the users who chose level for a channel. Whether
// they still read it is the caller's to check.
func (s *Store) PeopleWithLevel(ctx context.Context, channelID, level string) ([]Person, error) {
	rows, err := s.db.Query(ctx,
		`SELECT u.id, COALESCE(u.ngac_node, ''), COALESCE(NULLIF(u.display_name, ''), u.username)
		 FROM channel_notification_levels l JOIN users u ON u.id = l.user_id
		 WHERE l.channel_id = $1 AND l.level = $2`, channelID, level)
	if err != nil {
		return nil, fmt.Errorf("people with level: %w", err)
	}
	defer rows.Close()
	var out []Person
	for rows.Next() {
		var p Person
		if err := rows.Scan(&p.UserID, &p.NodeID, &p.Name); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
