// Package store — how Chat behaves for one person in one workspace.
package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// ChatSettings is a person's options for Chat in a workspace. They arrange and
// word the person's own view; none of them grants or hides anything.
type ChatSettings struct {
	EnterToSend bool
	LinkPreview bool
	OpenAt      string // "all" | "sections"
	Sort        string // "activity" | "name"
	DoubleTap   string // the reaction a double tap adds; "" is off
	// DefaultLevel is the notification level a new group starts at for this
	// person; nil is the system default.
	DefaultLevel *string
}

// DefaultChatSettings is what a person who never opened the page has: the
// column defaults of chat_user_settings.
var DefaultChatSettings = ChatSettings{LinkPreview: true, OpenAt: "all", Sort: "activity", DoubleTap: "👍"}

// GetChatSettings returns the person's settings in a workspace, or the defaults.
func (s *Store) GetChatSettings(ctx context.Context, userID, workspaceID string) (ChatSettings, error) {
	var c ChatSettings
	err := s.db.QueryRow(ctx,
		`SELECT enter_to_send, link_preview, open_at, sort, double_tap, default_level
		 FROM chat_user_settings WHERE user_id = $1 AND workspace_id = $2`, userID, workspaceID).
		Scan(&c.EnterToSend, &c.LinkPreview, &c.OpenAt, &c.Sort, &c.DoubleTap, &c.DefaultLevel)
	if errors.Is(err, pgx.ErrNoRows) {
		return DefaultChatSettings, nil
	}
	if err != nil {
		return DefaultChatSettings, fmt.Errorf("chat settings: %w", err)
	}
	return c, nil
}

// SaveChatSettings stores the person's settings in a workspace.
func (s *Store) SaveChatSettings(ctx context.Context, userID, workspaceID string, c ChatSettings) error {
	if _, err := s.db.Exec(ctx,
		`INSERT INTO chat_user_settings (user_id, workspace_id, enter_to_send, link_preview, open_at, sort, double_tap, default_level, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW())
		 ON CONFLICT (user_id, workspace_id) DO UPDATE SET enter_to_send = $3, link_preview = $4, open_at = $5,
		   sort = $6, double_tap = $7, default_level = $8, updated_at = NOW()`,
		userID, workspaceID, c.EnterToSend, c.LinkPreview, c.OpenAt, c.Sort, c.DoubleTap, c.DefaultLevel); err != nil {
		return fmt.Errorf("save chat settings: %w", err)
	}
	return nil
}

// SeedNotificationLevel gives a person joining a group the level they chose as
// their default in the group's workspace. A person with no default, or who
// already chose a level for this group, is left as they are.
func (s *Store) SeedNotificationLevel(ctx context.Context, userID, workspaceID, channelID string) error {
	if _, err := s.db.Exec(ctx,
		`INSERT INTO channel_notification_levels (user_id, channel_id, level)
		 SELECT user_id, $3, default_level FROM chat_user_settings
		 WHERE user_id = $1 AND workspace_id = $2 AND default_level IS NOT NULL
		 ON CONFLICT (user_id, channel_id) DO NOTHING`, userID, workspaceID, channelID); err != nil {
		return fmt.Errorf("seed notification level: %w", err)
	}
	return nil
}

// UserIDByNode returns the user whose NGAC node it is; "" when there is none.
func (s *Store) UserIDByNode(ctx context.Context, nodeID string) (string, error) {
	var id string
	err := s.db.QueryRow(ctx, `SELECT id FROM users WHERE ngac_node = $1`, nodeID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("user by node: %w", err)
	}
	return id, nil
}
