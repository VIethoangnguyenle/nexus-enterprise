// Package store — reactions, pins, read receipts queries.
package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// --- Reaction Models ---

// Reaction represents a single emoji reaction row.
type Reaction struct {
	ID        string
	MessageID string
	UserID    string
	Emoji     string
	CreatedAt time.Time
}

// ReactionGroup aggregates reactions by emoji.
type ReactionGroup struct {
	Emoji   string
	Count   int32
	UserIDs []string
}

// --- Pin Models ---

// Pin represents a pinned message row.
type Pin struct {
	ID        string
	ChannelID string
	MessageID string
	PinnedBy  string
	CreatedAt time.Time
}

// --- Read Receipt Models ---

// ReadReceipt represents a user's read position in a channel.
type ReadReceipt struct {
	UserID        string
	ChannelID     string
	LastReadAt    time.Time
	LastMessageID string
}

// ChannelUnread holds unread info for a single channel.
type ChannelUnread struct {
	ChannelID       string
	UnreadCount     int32
	LastReadMsgID   string
}

// --- Reactions Store ---

// InsertReaction adds an emoji reaction. Idempotent via UNIQUE constraint.
func (s *Store) InsertReaction(ctx context.Context, messageID, userID, emoji string) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO message_reactions (message_id, user_id, emoji)
		 VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`,
		messageID, userID, emoji)
	if err != nil {
		return fmt.Errorf("insert reaction: %w", err)
	}
	return nil
}

// DeleteReaction removes a specific emoji reaction by a user.
func (s *Store) DeleteReaction(ctx context.Context, messageID, userID, emoji string) error {
	_, err := s.db.Exec(ctx,
		`DELETE FROM message_reactions WHERE message_id = $1 AND user_id = $2 AND emoji = $3`,
		messageID, userID, emoji)
	if err != nil {
		return fmt.Errorf("delete reaction: %w", err)
	}
	return nil
}

// ListReactionsByMessage returns aggregated reaction groups for a message.
func (s *Store) ListReactionsByMessage(ctx context.Context, messageID string) ([]ReactionGroup, error) {
	rows, err := s.db.Query(ctx,
		`SELECT emoji, COUNT(*), ARRAY_AGG(user_id ORDER BY created_at)
		 FROM message_reactions WHERE message_id = $1
		 GROUP BY emoji ORDER BY MIN(created_at)`,
		messageID)
	if err != nil {
		return nil, fmt.Errorf("list reactions: %w", err)
	}
	defer rows.Close()

	var groups []ReactionGroup
	for rows.Next() {
		var g ReactionGroup
		if err := rows.Scan(&g.Emoji, &g.Count, &g.UserIDs); err != nil {
			return nil, err
		}
		groups = append(groups, g)
	}
	return groups, nil
}

// ListReactionsForMessages batch-loads reaction groups for multiple message IDs.
func (s *Store) ListReactionsForMessages(ctx context.Context, messageIDs []string) (map[string][]ReactionGroup, error) {
	if len(messageIDs) == 0 {
		return nil, nil
	}
	rows, err := s.db.Query(ctx,
		`SELECT message_id, emoji, COUNT(*), ARRAY_AGG(user_id ORDER BY created_at)
		 FROM message_reactions WHERE message_id = ANY($1)
		 GROUP BY message_id, emoji ORDER BY message_id, MIN(created_at)`,
		messageIDs)
	if err != nil {
		return nil, fmt.Errorf("batch list reactions: %w", err)
	}
	defer rows.Close()

	result := make(map[string][]ReactionGroup)
	for rows.Next() {
		var msgID string
		var g ReactionGroup
		if err := rows.Scan(&msgID, &g.Emoji, &g.Count, &g.UserIDs); err != nil {
			return nil, err
		}
		result[msgID] = append(result[msgID], g)
	}
	return result, nil
}

// GetChannelIDForMessage returns the channel_id for a given message.
func (s *Store) GetChannelIDForMessage(ctx context.Context, messageID string) (string, error) {
	var channelID string
	err := s.db.QueryRow(ctx,
		`SELECT channel_id FROM messages WHERE id = $1`, messageID).Scan(&channelID)
	if err == pgx.ErrNoRows {
		return "", nil
	}
	return channelID, err
}

// --- Pins Store ---

// InsertPin pins a message in a channel. Idempotent via UNIQUE constraint.
func (s *Store) InsertPin(ctx context.Context, channelID, messageID, pinnedBy string) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO message_pins (channel_id, message_id, pinned_by)
		 VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`,
		channelID, messageID, pinnedBy)
	if err != nil {
		return fmt.Errorf("insert pin: %w", err)
	}
	return nil
}

// DeletePin unpins a message from a channel.
func (s *Store) DeletePin(ctx context.Context, channelID, messageID string) error {
	_, err := s.db.Exec(ctx,
		`DELETE FROM message_pins WHERE channel_id = $1 AND message_id = $2`,
		channelID, messageID)
	if err != nil {
		return fmt.Errorf("delete pin: %w", err)
	}
	return nil
}

// ListPinsByChannel returns all pinned messages for a channel.
func (s *Store) ListPinsByChannel(ctx context.Context, channelID string) ([]Pin, error) {
	rows, err := s.db.Query(ctx,
		`SELECT id, channel_id, message_id, pinned_by, created_at
		 FROM message_pins WHERE channel_id = $1 ORDER BY created_at DESC`,
		channelID)
	if err != nil {
		return nil, fmt.Errorf("list pins: %w", err)
	}
	defer rows.Close()

	var pins []Pin
	for rows.Next() {
		var p Pin
		var ca time.Time
		if err := rows.Scan(&p.ID, &p.ChannelID, &p.MessageID, &p.PinnedBy, &ca); err != nil {
			return nil, err
		}
		p.CreatedAt = ca
		pins = append(pins, p)
	}
	return pins, nil
}

// IsPinned checks if a specific message is pinned in its channel.
func (s *Store) IsPinned(ctx context.Context, messageID string) (bool, error) {
	var exists bool
	err := s.db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM message_pins WHERE message_id = $1)`,
		messageID).Scan(&exists)
	return exists, err
}

// PinnedMessageIDs returns the set of pinned message IDs for a channel.
func (s *Store) PinnedMessageIDs(ctx context.Context, channelID string) (map[string]bool, error) {
	rows, err := s.db.Query(ctx,
		`SELECT message_id FROM message_pins WHERE channel_id = $1`, channelID)
	if err != nil {
		return nil, fmt.Errorf("pinned ids: %w", err)
	}
	defer rows.Close()

	result := make(map[string]bool)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		result[id] = true
	}
	return result, nil
}

// --- Read Receipts Store ---

// UpsertReadReceipt marks a channel as read up to a specific message.
func (s *Store) UpsertReadReceipt(ctx context.Context, userID, channelID, lastMessageID string) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO read_receipts (user_id, channel_id, last_read_at, last_message_id)
		 VALUES ($1, $2, NOW(), NULLIF($3, ''))
		 ON CONFLICT (user_id, channel_id) DO UPDATE SET
		   last_read_at = NOW(), last_message_id = NULLIF($3, '')`,
		userID, channelID, lastMessageID)
	if err != nil {
		return fmt.Errorf("upsert read receipt: %w", err)
	}
	return nil
}

// GetUnreadCounts returns unread message counts for all channels a user is a member of.
func (s *Store) GetUnreadCounts(ctx context.Context, userID string) ([]ChannelUnread, error) {
	rows, err := s.db.Query(ctx,
		`SELECT cm.channel_id,
		        COUNT(m.id) AS unread_count,
		        COALESCE(rr.last_message_id, '') AS last_read_msg
		 FROM channel_members cm
		 LEFT JOIN read_receipts rr ON rr.user_id = $1 AND rr.channel_id = cm.channel_id
		 LEFT JOIN messages m ON m.channel_id = cm.channel_id
		   AND m.parent_message_id IS NULL
		   AND (rr.last_read_at IS NULL OR m.created_at > rr.last_read_at)
		 WHERE cm.ngac_node_id = (SELECT ngac_node_id FROM users WHERE id = $1)
		 GROUP BY cm.channel_id, rr.last_message_id`,
		userID)
	if err != nil {
		return nil, fmt.Errorf("unread counts: %w", err)
	}
	defer rows.Close()

	var results []ChannelUnread
	for rows.Next() {
		var u ChannelUnread
		if err := rows.Scan(&u.ChannelID, &u.UnreadCount, &u.LastReadMsgID); err != nil {
			return nil, err
		}
		results = append(results, u)
	}
	return results, nil
}

// --- Search Store ---

// SearchMessages performs full-text search on messages in a channel.
func (s *Store) SearchMessages(ctx context.Context, channelID, query string, limit int) ([]*Message, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	rows, err := s.db.Query(ctx,
		`SELECT `+messageCols+`
		 FROM messages m LEFT JOIN users u ON m.sender_id = u.id
		 WHERE m.channel_id = $1 AND m.search_vector @@ plainto_tsquery('english', $2)
		 ORDER BY ts_rank(m.search_vector, plainto_tsquery('english', $2)) DESC
		 LIMIT $3`,
		channelID, query, limit)
	if err != nil {
		return nil, fmt.Errorf("search messages: %w", err)
	}
	defer rows.Close()
	return scanMessages(rows)
}

// --- Helper to create store with external pool reference ---

// This file extends Store with reaction/pin/read-receipt/search methods.
