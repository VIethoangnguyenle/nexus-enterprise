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
	ChannelID     string
	UnreadCount   int32
	LastReadMsgID string
	// ThreadUnread counts unread replies in the threads of the channel the user
	// follows — every thread's, when the level is "all".
	ThreadUnread int32
	// Level is the user's notification level for the channel ("main" unless set).
	Level string
	// MentionCount counts unread main messages that mention the user, at every level.
	MentionCount int32
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

// PinnedMessageIDsIn returns the set of pinned message ids of the channels, in one query.
func (s *Store) PinnedMessageIDsIn(ctx context.Context, channelIDs []string) (map[string]bool, error) {
	result := make(map[string]bool)
	if len(channelIDs) == 0 {
		return result, nil
	}
	rows, err := s.db.Query(ctx,
		`SELECT message_id FROM message_pins WHERE channel_id = ANY($1)`, channelIDs)
	if err != nil {
		return nil, fmt.Errorf("pinned ids: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		result[id] = true
	}
	return result, rows.Err()
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

// GetUnreadCounts returns the user's unread message and thread-reply counts
// for the given channels.
func (s *Store) GetUnreadCounts(ctx context.Context, userID string, channelIDs []string) ([]ChannelUnread, error) {
	if len(channelIDs) == 0 {
		return nil, nil
	}
	// The caller decides which channels to count (the ones the user may read);
	// the read position and thread follows are keyed by the user.
	//
	// Thread unread is counted from the threads the user follows — explicit
	// follow rows, plus the threads they started or were mentioned in that have
	// no row (the same rule as threadFollowingExpr) — rather than from every
	// thread of every channel: the followed set is small, and this query runs
	// each time a message arrives.
	rows, err := s.db.Query(ctx,
		`WITH followed AS (
		   SELECT tf.root_message_id AS id, tf.last_read_at
		   FROM thread_follows tf WHERE tf.user_id = $1 AND tf.following
		   UNION ALL
		   SELECT m.id, m.created_at FROM messages m
		   WHERE m.parent_message_id IS NULL AND m.reply_count > 0
		     AND (m.sender_id = $1 OR m.mentions @> ARRAY[$1]::text[])
		     AND NOT EXISTS (SELECT 1 FROM thread_follows tf WHERE tf.user_id = $1 AND tf.root_message_id = m.id)
		 ), thread_unread AS (
		   SELECT root.channel_id, COUNT(r.id)::int AS n
		   FROM followed f
		   JOIN messages root ON root.id = f.id
		   JOIN messages r ON r.parent_message_id = f.id AND r.sender_id <> $1 AND r.deleted_at IS NULL
		     AND r.created_at > f.last_read_at
		   GROUP BY root.channel_id
		 ), mention_unread AS (
		   -- Main messages that mention the user and are unread for them: after
		   -- the channel read position from when the message was sent, or from
		   -- the edit that added the user to it.
		   SELECT m.channel_id, COUNT(*)::int AS n
		   FROM messages m
		   LEFT JOIN read_receipts rr3 ON rr3.user_id = $1 AND rr3.channel_id = m.channel_id
		   LEFT JOIN message_mention_times mt ON mt.message_id = m.id AND mt.user_id = $1
		   WHERE m.channel_id = ANY($2::text[]) AND m.parent_message_id IS NULL AND m.deleted_at IS NULL
		     AND m.mentions @> ARRAY[$1]::text[] AND m.sender_id <> $1
		     AND GREATEST(m.created_at, mt.mentioned_at) > COALESCE(rr3.last_read_at, '-infinity'::timestamptz)
		   GROUP BY m.channel_id
		 ), levels AS (
		   SELECT channel_id, level FROM channel_notification_levels
		   WHERE user_id = $1 AND channel_id = ANY($2::text[])
		 ), all_thread_unread AS (
		   -- "all": every reply by someone else counts — after the thread's own
		   -- read position when the user follows it, else after the channel's.
		   SELECT r.channel_id, COUNT(r.id)::int AS n
		   FROM messages r
		   JOIN levels l ON l.channel_id = r.channel_id AND l.level = 'all'
		   LEFT JOIN read_receipts rr2 ON rr2.user_id = $1 AND rr2.channel_id = r.channel_id
		   LEFT JOIN followed f ON f.id = r.parent_message_id
		   WHERE r.parent_message_id IS NOT NULL AND r.sender_id <> $1 AND r.deleted_at IS NULL
		     AND r.created_at > COALESCE(f.last_read_at, rr2.last_read_at, '-infinity'::timestamptz)
		   GROUP BY r.channel_id
		 )
		 SELECT cm.channel_id,
		        CASE WHEN l.level = 'off' THEN 0 ELSE COUNT(m.id) END AS unread_count,
		        COALESCE(rr.last_message_id, '') AS last_read_msg,
		        CASE WHEN l.level = 'off' THEN 0
		             WHEN l.level = 'all' THEN COALESCE(MAX(atu.n), 0)
		             ELSE COALESCE(MAX(tu.n), 0) END AS thread_unread,
		        COALESCE(l.level, 'main') AS level,
		        COALESCE(MAX(mu.n), 0) AS mention_count
		 FROM (SELECT DISTINCT unnest($2::text[]) AS channel_id) cm
		 LEFT JOIN read_receipts rr ON rr.user_id = $1 AND rr.channel_id = cm.channel_id
		 LEFT JOIN messages m ON m.channel_id = cm.channel_id
		   AND m.parent_message_id IS NULL AND m.deleted_at IS NULL
		   AND (rr.last_read_at IS NULL OR m.created_at > rr.last_read_at)
		 LEFT JOIN thread_unread tu ON tu.channel_id = cm.channel_id
		 LEFT JOIN all_thread_unread atu ON atu.channel_id = cm.channel_id
		 LEFT JOIN mention_unread mu ON mu.channel_id = cm.channel_id
		 LEFT JOIN levels l ON l.channel_id = cm.channel_id
		 GROUP BY cm.channel_id, rr.last_message_id, l.level`,
		userID, channelIDs)
	if err != nil {
		return nil, fmt.Errorf("unread counts: %w", err)
	}
	defer rows.Close()

	var results []ChannelUnread
	for rows.Next() {
		var u ChannelUnread
		if err := rows.Scan(&u.ChannelID, &u.UnreadCount, &u.LastReadMsgID, &u.ThreadUnread, &u.Level, &u.MentionCount); err != nil {
			return nil, err
		}
		results = append(results, u)
	}
	return results, rows.Err()
}

// --- Helper to create store with external pool reference ---

// This file extends Store with reaction/pin/read-receipt/search methods.
