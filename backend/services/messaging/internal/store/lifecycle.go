// Package store — editing, deleting, quoting and un-reading messages.
package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// ErrParentGone: a reply's parent was deleted before the reply was stored.
var ErrParentGone = errors.New("parent message deleted")

// ErrMessageGone: the message was deleted by the time it was edited or deleted.
var ErrMessageGone = errors.New("message deleted")

// redactIfDeleted removes everything a deleted message said. Its row stays only
// so a thread keeps its root; nothing of its body leaves the store.
func (m *Message) redactIfDeleted() {
	if !m.Deleted {
		return
	}
	m.Content = ""
	m.ContentText = ""
	m.Mentions = nil
	m.Attachments = nil
	m.LinkedEntityType = ""
	m.LinkedEntityID = ""
	m.QuotedMessageID = ""
}

// MessageFacts is what deciding on an edit or a delete needs to know.
type MessageFacts struct {
	ChannelID        string
	SenderID         string
	ParentMessageID  string
	LinkedEntityType string
	ReplyCount       int32
	CreatedAt        time.Time
	Deleted          bool
}

// GetMessageFacts returns a message's facts; nil when there is no such message.
func (s *Store) GetMessageFacts(ctx context.Context, id string) (*MessageFacts, error) {
	var f MessageFacts
	err := s.db.QueryRow(ctx,
		`SELECT channel_id, sender_id, COALESCE(parent_message_id, ''), COALESCE(linked_entity_type, ''),
		        reply_count, created_at, deleted_at IS NOT NULL
		 FROM messages WHERE id = $1`, id).
		Scan(&f.ChannelID, &f.SenderID, &f.ParentMessageID, &f.LinkedEntityType, &f.ReplyCount, &f.CreatedAt, &f.Deleted)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("message facts: %w", err)
	}
	return &f, nil
}

// UpdateMessageContent replaces a live message's text. A non-nil mentions
// replaces its mention list (nil leaves it as it is) and the users it adds —
// mentioned now, not before — are returned. When the message is a reply, the
// added users follow its thread, as for a reply sent with them.
func (s *Store) UpdateMessageContent(ctx context.Context, id, content, text, format string, mentions []string) ([]string, error) {
	var added []string
	err := pgx.BeginFunc(ctx, s.db, func(tx pgx.Tx) error {
		var prev []string
		var parent, sender string
		err := tx.QueryRow(ctx,
			`SELECT COALESCE(mentions, '{}'), COALESCE(parent_message_id, ''), sender_id
			 FROM messages WHERE id = $1 AND deleted_at IS NULL FOR UPDATE`, id).Scan(&prev, &parent, &sender)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrMessageGone
		}
		if err != nil {
			return fmt.Errorf("edit message: %w", err)
		}
		if _, err := tx.Exec(ctx,
			`UPDATE messages SET content = $2, content_text = $3, content_format = $4, edited_at = NOW(),
			        mentions = COALESCE($5::text[], mentions)
			 WHERE id = $1`, id, content, text, format, mentions); err != nil {
			return fmt.Errorf("edit message: %w", err)
		}
		if mentions == nil {
			return nil
		}
		added = Difference(mentions, prev)
		if parent == "" || len(added) == 0 {
			return nil
		}
		return followMentioned(ctx, tx, added, parent, sender)
	})
	if err != nil {
		return nil, err
	}
	return added, nil
}

// Difference returns the ids of a that are not in b, once each, in a's order.
func Difference(a, b []string) []string {
	have := make(map[string]struct{}, len(b))
	for _, id := range b {
		have[id] = struct{}{}
	}
	var out []string
	for _, id := range a {
		if _, ok := have[id]; !ok {
			have[id] = struct{}{}
			out = append(out, id)
		}
	}
	return out
}

// followMentioned makes the users a reply mentions follow the thread of its
// root. Ids that are not users are dropped rather than failing the write.
func followMentioned(ctx context.Context, tx pgx.Tx, users []string, rootID, senderID string) error {
	if _, err := tx.Exec(ctx,
		`INSERT INTO thread_follows (user_id, root_message_id, following, last_read_at, updated_at)
		 SELECT u.id, root.id, TRUE, root.created_at, NOW()
		 FROM users u, messages root
		 WHERE u.id = ANY($1) AND u.id <> $3 AND root.id = $2
		 ON CONFLICT (user_id, root_message_id) DO UPDATE SET following = TRUE, updated_at = NOW()`,
		users, rootID, senderID); err != nil {
		return fmt.Errorf("follow mentioned users: %w", err)
	}
	return nil
}

// DeletedMessage is the outcome of a delete, for the live views.
type DeletedMessage struct {
	ChannelID       string
	ParentMessageID string
	// KeepPlaceholder: a root message with replies stays, without its body.
	KeepPlaceholder bool
	// ParentReplyCount is the thread's count after a reply was deleted.
	ParentReplyCount int32
}

// SoftDeleteMessage marks a message deleted by a user. A deleted reply no longer
// counts in its thread, whose last reply time moves back to the newest reply
// left; the writes stand or fall together.
func (s *Store) SoftDeleteMessage(ctx context.Context, id, byUserID string) (*DeletedMessage, error) {
	out := &DeletedMessage{}
	err := pgx.BeginFunc(ctx, s.db, func(tx pgx.Tx) error {
		var replyCount int32
		err := tx.QueryRow(ctx,
			`UPDATE messages SET deleted_at = NOW(), deleted_by = $2
			 WHERE id = $1 AND deleted_at IS NULL
			 RETURNING channel_id, COALESCE(parent_message_id, ''), reply_count`, id, byUserID).
			Scan(&out.ChannelID, &out.ParentMessageID, &replyCount)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrMessageGone
		}
		if err != nil {
			return fmt.Errorf("delete message: %w", err)
		}
		if out.ParentMessageID == "" {
			out.KeepPlaceholder = replyCount > 0
			return nil
		}
		return tx.QueryRow(ctx,
			`UPDATE messages root SET reply_count = GREATEST(root.reply_count - 1, 0),
			        last_reply_at = (SELECT MAX(r.created_at) FROM messages r
			                         WHERE r.parent_message_id = root.id AND r.deleted_at IS NULL)
			 WHERE root.id = $1
			 RETURNING root.reply_count`, out.ParentMessageID).Scan(&out.ParentReplyCount)
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Quote is what a quote shows of the message it quotes.
type Quote struct {
	ID         string
	ChannelID  string
	SenderID   string
	SenderName string
	Text       string
	Deleted    bool
}

// quoteChars is how much of the quoted text is shown.
const quoteChars = 200

// QuotesByID loads the quoted messages of a page in one query.
func (s *Store) QuotesByID(ctx context.Context, ids []string) (map[string]Quote, error) {
	out := map[string]Quote{}
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := s.db.Query(ctx,
		`SELECT m.id, m.channel_id, m.sender_id, COALESCE(NULLIF(u.display_name, ''), u.username, ''),
		        CASE WHEN m.deleted_at IS NULL THEN LEFT(COALESCE(m.content_text, m.content), $2) ELSE '' END,
		        m.deleted_at IS NOT NULL
		 FROM messages m LEFT JOIN users u ON u.id = m.sender_id
		 WHERE m.id = ANY($1)`, ids, quoteChars)
	if err != nil {
		return nil, fmt.Errorf("quotes: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var q Quote
		if err := rows.Scan(&q.ID, &q.ChannelID, &q.SenderID, &q.SenderName, &q.Text, &q.Deleted); err != nil {
			return nil, err
		}
		out[q.ID] = q
	}
	return out, rows.Err()
}

// DisplayName returns the name a user is shown by: their display name, else
// their username; empty for an unknown id.
func (s *Store) DisplayName(ctx context.Context, userID string) (string, error) {
	var name string
	err := s.db.QueryRow(ctx,
		`SELECT COALESCE(NULLIF(display_name, ''), username) FROM users WHERE id = $1`, userID).Scan(&name)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("display name: %w", err)
	}
	return name, nil
}

// SetChannelReadBefore moves the user's read position in a channel to just
// before a moment, so what came from then on is unread again.
func (s *Store) SetChannelReadBefore(ctx context.Context, userID, channelID string, at time.Time) error {
	if _, err := s.db.Exec(ctx,
		`INSERT INTO read_receipts (user_id, channel_id, last_read_at, last_message_id)
		 VALUES ($1, $2, $3, NULL)
		 ON CONFLICT (user_id, channel_id) DO UPDATE SET last_read_at = $3, last_message_id = NULL`,
		userID, channelID, at.Add(-time.Microsecond)); err != nil {
		return fmt.Errorf("mark channel unread: %w", err)
	}
	return nil
}

// SetThreadReadBefore moves the user's read position in a thread to just
// before a moment. Whether the user follows the thread does not change.
func (s *Store) SetThreadReadBefore(ctx context.Context, userID, rootID string, at time.Time) error {
	if _, err := s.db.Exec(ctx,
		`INSERT INTO thread_follows (user_id, root_message_id, following, last_read_at, updated_at)
		 SELECT $1, m.id, `+threadFollowingExpr+`, $3, NOW()
		 FROM messages m LEFT JOIN thread_follows tf ON FALSE
		 WHERE m.id = $2
		 ON CONFLICT (user_id, root_message_id) DO UPDATE SET last_read_at = $3, updated_at = NOW()`,
		userID, rootID, at.Add(-time.Microsecond)); err != nil {
		return fmt.Errorf("mark thread unread: %w", err)
	}
	return nil
}
