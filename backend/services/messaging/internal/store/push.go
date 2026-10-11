// Package store — push preferences and the people a message concerns.
package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// PushPreferences is what a user wants pushed to their devices.
type PushPreferences struct {
	Direct, Mention, ThreadReply bool
	QuietEnabled                 bool
	QuietStart, QuietEnd         int // minutes after midnight
	TimeZone                     string
}

// DefaultPushPreferences: everything on, no quiet hours.
var DefaultPushPreferences = PushPreferences{
	Direct: true, Mention: true, ThreadReply: true, QuietStart: 22 * 60, QuietEnd: 7 * 60, TimeZone: "Asia/Ho_Chi_Minh",
}

// GetPushPreferences returns a user's preferences, or the defaults.
func (s *Store) GetPushPreferences(ctx context.Context, userID string) (PushPreferences, error) {
	p := DefaultPushPreferences
	err := s.db.QueryRow(ctx,
		`SELECT direct, mention, thread_reply, quiet_enabled, quiet_start, quiet_end, time_zone
		 FROM push_preferences WHERE user_id = $1`, userID).
		Scan(&p.Direct, &p.Mention, &p.ThreadReply, &p.QuietEnabled, &p.QuietStart, &p.QuietEnd, &p.TimeZone)
	if errors.Is(err, pgx.ErrNoRows) {
		return DefaultPushPreferences, nil
	}
	if err != nil {
		return p, fmt.Errorf("push preferences: %w", err)
	}
	return p, nil
}

// SavePushPreferences stores a user's preferences.
func (s *Store) SavePushPreferences(ctx context.Context, userID string, p PushPreferences) error {
	if _, err := s.db.Exec(ctx,
		`INSERT INTO push_preferences (user_id, direct, mention, thread_reply, quiet_enabled, quiet_start, quiet_end, time_zone, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW())
		 ON CONFLICT (user_id) DO UPDATE SET direct = $2, mention = $3, thread_reply = $4, quiet_enabled = $5,
		   quiet_start = $6, quiet_end = $7, time_zone = $8, updated_at = NOW()`,
		userID, p.Direct, p.Mention, p.ThreadReply, p.QuietEnabled, p.QuietStart, p.QuietEnd, p.TimeZone); err != nil {
		return fmt.Errorf("save push preferences: %w", err)
	}
	return nil
}

// Person is a user with the NGAC node their sessions act as.
type Person struct {
	UserID string
	NodeID string
	Name   string
}

// PeopleByID returns the users among the ids, with their node and display name.
func (s *Store) PeopleByID(ctx context.Context, ids []string) ([]Person, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	rows, err := s.db.Query(ctx,
		`SELECT id, COALESCE(ngac_node, ''), COALESCE(NULLIF(display_name, ''), username)
		 FROM users WHERE id = ANY($1)`, ids)
	if err != nil {
		return nil, fmt.Errorf("people: %w", err)
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

// ChannelPeople returns the users recorded as members of a channel.
func (s *Store) ChannelPeople(ctx context.Context, channelID string) ([]Person, error) {
	rows, err := s.db.Query(ctx,
		`SELECT u.id, COALESCE(u.ngac_node, ''), COALESCE(NULLIF(u.display_name, ''), u.username)
		 FROM channel_members cm JOIN users u ON u.ngac_node = cm.ngac_node_id
		 WHERE cm.channel_id = $1`, channelID)
	if err != nil {
		return nil, fmt.Errorf("channel people: %w", err)
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
