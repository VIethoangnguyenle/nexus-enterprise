// Package store — a person's own stars on conversations and messages, and the
// feed of messages that mention them.
package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// SetConversationStar stars or unstars a conversation for a user. Both are
// idempotent.
func (s *Store) SetConversationStar(ctx context.Context, userID, channelID string, starred bool) error {
	q := `DELETE FROM starred_conversations WHERE user_id = $1 AND channel_id = $2`
	if starred {
		q = `INSERT INTO starred_conversations (user_id, channel_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`
	}
	if _, err := s.db.Exec(ctx, q, userID, channelID); err != nil {
		return fmt.Errorf("set conversation star: %w", err)
	}
	return nil
}

// SetMessageStar stars or unstars a message for a user. Both are idempotent;
// a repeated star keeps its original time.
func (s *Store) SetMessageStar(ctx context.Context, userID, messageID string, starred bool) error {
	q := `DELETE FROM starred_messages WHERE user_id = $1 AND message_id = $2`
	if starred {
		q = `INSERT INTO starred_messages (user_id, message_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`
	}
	if _, err := s.db.Exec(ctx, q, userID, messageID); err != nil {
		return fmt.Errorf("set message star: %w", err)
	}
	return nil
}

// StarredConversations returns which of the given channels the user starred.
func (s *Store) StarredConversations(ctx context.Context, userID string, channelIDs []string) (map[string]bool, error) {
	out := map[string]bool{}
	if len(channelIDs) == 0 {
		return out, nil
	}
	rows, err := s.db.Query(ctx,
		`SELECT channel_id FROM starred_conversations WHERE user_id = $1 AND channel_id = ANY($2::text[])`,
		userID, channelIDs)
	if err != nil {
		return nil, fmt.Errorf("starred conversations: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, rows.Err()
}

// scanFeedMessages reads rows of messageCols followed by one extra column
// scanned into extra(m).
func scanFeedMessages(rows pgx.Rows, extra func(m *Message) any) ([]*Message, error) {
	var msgs []*Message
	for rows.Next() {
		m := &Message{}
		if err := rows.Scan(&m.ID, &m.ChannelID, &m.SenderID, &m.SenderName,
			&m.Content, &m.CreatedAt, &m.MessageType, &m.ParentMessageID,
			&m.LinkedEntityType, &m.LinkedEntityID, &m.ReplyCount,
			&m.ContentFormat, &m.Mentions, &m.LastReplyAt,
			&m.EditedAt, &m.Deleted, &m.QuotedMessageID, extra(m)); err != nil {
			return nil, err
		}
		msgs = append(msgs, m)
	}
	return msgs, rows.Err()
}

// StarredMessages lists the messages the user starred in the given channels,
// newest star first, leaving out deleted ones. before pages by star time and
// beforeID — the id of the last message already shown — breaks a tie between
// stars set at the same instant.
func (s *Store) StarredMessages(ctx context.Context, userID string, channelIDs []string, before *time.Time, beforeID string, limit int) ([]*Message, bool, error) {
	if len(channelIDs) == 0 {
		return nil, false, nil
	}
	rows, err := s.db.Query(ctx,
		`SELECT `+messageCols+`, st.created_at
		 FROM starred_messages st
		 JOIN messages m ON m.id = st.message_id
		 LEFT JOIN users u ON m.sender_id = u.id
		 WHERE st.user_id = $1 AND m.channel_id = ANY($2::text[]) AND m.deleted_at IS NULL
		   AND ($3::timestamptz IS NULL OR st.created_at < $3
		        OR ($5 <> '' AND st.created_at = $3 AND m.id < $5))
		 ORDER BY st.created_at DESC, m.id DESC
		 LIMIT $4`, userID, channelIDs, before, limit+1, beforeID)
	if err != nil {
		return nil, false, fmt.Errorf("starred messages: %w", err)
	}
	defer rows.Close()
	msgs, err := scanFeedMessages(rows, func(m *Message) any { return &m.StarredAt })
	if err != nil {
		return nil, false, err
	}
	return trimPage(msgs, limit)
}

// The mentions feed and its unread count share one definition of "a mention
// of $1 in the channels $2" and of "unread", so the badge always equals the
// unread items of the list.
const (
	mentionsFrom = `FROM messages m
		 LEFT JOIN read_receipts rr ON rr.user_id = $1 AND rr.channel_id = m.channel_id
		 LEFT JOIN messages root ON root.id = m.parent_message_id
		 LEFT JOIN message_mention_times mt ON mt.message_id = m.id AND mt.user_id = $1
		 LEFT JOIN thread_follows tf ON tf.user_id = $1 AND tf.root_message_id = m.parent_message_id`

	mentionsWhere = `m.channel_id = ANY($2::text[]) AND m.mentions @> ARRAY[$1]::text[]
		   AND m.sender_id <> $1 AND m.deleted_at IS NULL`

	// A mention dates from the message, or from the edit that added the user to
	// it. A main message is unread after the channel read position; a reply
	// after its thread's (the root's creation when never read).
	mentionUnreadExpr = `CASE WHEN m.parent_message_id IS NULL
		             THEN GREATEST(m.created_at, mt.mentioned_at) > COALESCE(rr.last_read_at, '-infinity'::timestamptz)
		             ELSE GREATEST(m.created_at, mt.mentioned_at) > COALESCE(tf.last_read_at, root.created_at, '-infinity'::timestamptz)
		        END`
)

// CountUnreadMentions counts, in one query, the unread messages that mention
// the user in the given channels: the unread items of MentionsOf.
func (s *Store) CountUnreadMentions(ctx context.Context, userID string, channelIDs []string) (int, error) {
	if len(channelIDs) == 0 {
		return 0, nil
	}
	var n int
	err := s.db.QueryRow(ctx,
		`SELECT COUNT(*)::int `+mentionsFrom+`
		 WHERE `+mentionsWhere+` AND `+mentionUnreadExpr, userID, channelIDs).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("unread mentions: %w", err)
	}
	return n, nil
}

// MentionsOf lists the messages by someone else that mention the user in the
// given channels, newest first, leaving out deleted ones. Unread is set per
// message: a main message is unread after the user's channel read position, a
// reply after the user's read position in its thread (the root's creation when
// they never read it). The user's notification level does not matter: a mention
// always counts.
func (s *Store) MentionsOf(ctx context.Context, userID string, channelIDs []string, before *time.Time, beforeID string, limit int) ([]*Message, bool, error) {
	if len(channelIDs) == 0 {
		return nil, false, nil
	}
	rows, err := s.db.Query(ctx,
		`SELECT `+messageCols+`, `+mentionUnreadExpr+`
		 `+mentionsFrom+`
		 LEFT JOIN users u ON m.sender_id = u.id
		 WHERE `+mentionsWhere+`
		   AND ($3::timestamptz IS NULL OR m.created_at < $3
		        OR ($5 <> '' AND m.created_at = $3 AND m.id < $5))
		 ORDER BY m.created_at DESC, m.id DESC
		 LIMIT $4`, userID, channelIDs, before, limit+1, beforeID)
	if err != nil {
		return nil, false, fmt.Errorf("mentions: %w", err)
	}
	defer rows.Close()
	msgs, err := scanFeedMessages(rows, func(m *Message) any { return &m.Unread })
	if err != nil {
		return nil, false, err
	}
	return trimPage(msgs, limit)
}

// trimPage cuts a page fetched with one row to spare and says whether that
// spare row existed.
func trimPage(msgs []*Message, limit int) ([]*Message, bool, error) {
	if len(msgs) > limit {
		return msgs[:limit], true, nil
	}
	return msgs, false, nil
}
