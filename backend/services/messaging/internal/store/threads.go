// Package store — thread follow, per-thread read position and thread listing.
package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// ThreadState is one thread as one user sees it.
type ThreadState struct {
	Following bool
	// Unread counts replies by someone else after the user's read position;
	// always 0 for a thread the user does not follow.
	Unread int32
	// Participants are the three most recent distinct repliers, newest first.
	Participants []string
}

// Thread filters accepted by ListThreads.
const (
	ThreadFilterAll       = "all"
	ThreadFilterFollowing = "following"
	ThreadFilterUnread    = "unread"
)

// The three expressions below read a root message `m`, the user's follow row
// `tf` (LEFT JOINed, may be NULL) and the user's id as parameter $1.
//
// With no follow row a user follows a thread they started or were mentioned
// in, and has read it up to the root message.
const (
	threadFollowingExpr = `COALESCE(tf.following, m.sender_id = $1 OR $1 = ANY(COALESCE(m.mentions, '{}')))`

	threadUnreadExpr = `CASE WHEN ` + threadFollowingExpr + ` THEN
		(SELECT COUNT(*)::int FROM messages r
		 WHERE r.parent_message_id = m.id AND r.sender_id <> $1 AND r.deleted_at IS NULL
		   AND r.created_at > COALESCE(tf.last_read_at, m.created_at))
		ELSE 0 END`

	threadParticipantsExpr = `ARRAY(SELECT p.sender_id FROM messages p
		WHERE p.parent_message_id = m.id AND p.deleted_at IS NULL
		GROUP BY p.sender_id ORDER BY MAX(p.created_at) DESC LIMIT 3)`

	threadFollowJoin = `LEFT JOIN thread_follows tf ON tf.root_message_id = m.id AND tf.user_id = $1`
)

// ThreadStates returns the caller's state of each given root message, in one
// query however many roots a page holds. Ids that are not messages are absent.
func (s *Store) ThreadStates(ctx context.Context, userID string, rootIDs []string) (map[string]ThreadState, error) {
	if len(rootIDs) == 0 {
		return map[string]ThreadState{}, nil
	}
	rows, err := s.db.Query(ctx,
		`SELECT m.id, `+threadFollowingExpr+`, `+threadUnreadExpr+`, `+threadParticipantsExpr+`
		 FROM messages m `+threadFollowJoin+`
		 WHERE m.id = ANY($2)`, userID, rootIDs)
	if err != nil {
		return nil, fmt.Errorf("thread states: %w", err)
	}
	defer rows.Close()

	states := make(map[string]ThreadState, len(rootIDs))
	for rows.Next() {
		var id string
		var st ThreadState
		if err := rows.Scan(&id, &st.Following, &st.Unread, &st.Participants); err != nil {
			return nil, err
		}
		states[id] = st
	}
	return states, rows.Err()
}

// SetThreadFollow follows or unfollows a thread for a user. Following a thread
// for the first time starts with nothing unread; an existing read position is kept.
func (s *Store) SetThreadFollow(ctx context.Context, userID, rootID string, following bool) error {
	if _, err := s.db.Exec(ctx,
		`INSERT INTO thread_follows (user_id, root_message_id, following, last_read_at, updated_at)
		 VALUES ($1, $2, $3, NOW(), NOW())
		 ON CONFLICT (user_id, root_message_id) DO UPDATE SET following = $3, updated_at = NOW()`,
		userID, rootID, following); err != nil {
		return fmt.Errorf("set thread follow: %w", err)
	}
	return nil
}

// MarkThreadRead moves the user's read position in a thread to now. Reading
// does not change whether the user follows the thread.
func (s *Store) MarkThreadRead(ctx context.Context, userID, rootID string) error {
	if _, err := s.db.Exec(ctx,
		`INSERT INTO thread_follows (user_id, root_message_id, following, last_read_at, updated_at)
		 SELECT $1, m.id, `+threadFollowingExpr+`, NOW(), NOW()
		 FROM messages m LEFT JOIN thread_follows tf ON FALSE
		 WHERE m.id = $2
		 ON CONFLICT (user_id, root_message_id) DO UPDATE SET last_read_at = NOW(), updated_at = NOW()`,
		userID, rootID); err != nil {
		return fmt.Errorf("mark thread read: %w", err)
	}
	return nil
}

// ThreadFollower is one user who follows a thread, with the NGAC node their
// sessions authenticate as and their own unread count.
type ThreadFollower struct {
	UserID string
	NodeID string
	Unread int32
}

// ThreadFollowers lists everyone who follows a thread — with a follow row or
// implicitly as its starter or a user its root mentions — in one query.
func (s *Store) ThreadFollowers(ctx context.Context, rootID string) ([]ThreadFollower, error) {
	rows, err := s.db.Query(ctx,
		`WITH candidates AS (
		   SELECT tf.user_id FROM thread_follows tf WHERE tf.root_message_id = $1
		   UNION
		   SELECT m.sender_id FROM messages m WHERE m.id = $1
		   UNION
		   SELECT unnest(COALESCE(m.mentions, '{}')) FROM messages m WHERE m.id = $1
		 )
		 SELECT u.id, COALESCE(u.ngac_node, ''),
		        (SELECT COUNT(*)::int FROM messages r
		         WHERE r.parent_message_id = m.id AND r.sender_id <> u.id AND r.deleted_at IS NULL
		           AND r.created_at > COALESCE(tf.last_read_at, m.created_at))
		 FROM candidates c
		 JOIN users u ON u.id = c.user_id
		 JOIN messages m ON m.id = $1
		 LEFT JOIN thread_follows tf ON tf.root_message_id = m.id AND tf.user_id = u.id
		 WHERE COALESCE(tf.following, m.sender_id = u.id OR u.id = ANY(COALESCE(m.mentions, '{}')))`,
		rootID)
	if err != nil {
		return nil, fmt.Errorf("thread followers: %w", err)
	}
	defer rows.Close()
	var out []ThreadFollower
	for rows.Next() {
		var f ThreadFollower
		if err := rows.Scan(&f.UserID, &f.NodeID, &f.Unread); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// ThreadSummaryRow is what every viewer of a channel sees of one thread.
type ThreadSummaryRow struct {
	ChannelID    string
	ReplyCount   int32
	LastReplyAt  *time.Time
	Participants []string
}

// ThreadSummary returns the channel-wide summary of a thread; nil when the
// root message does not exist.
func (s *Store) ThreadSummary(ctx context.Context, rootID string) (*ThreadSummaryRow, error) {
	var r ThreadSummaryRow
	err := s.db.QueryRow(ctx,
		`SELECT m.channel_id, m.reply_count, m.last_reply_at, `+threadParticipantsExpr+`
		 FROM messages m WHERE m.id = $1`, rootID).
		Scan(&r.ChannelID, &r.ReplyCount, &r.LastReplyAt, &r.Participants)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("thread summary: %w", err)
	}
	return &r, nil
}

// ListThreads returns the threads of a channel — root messages with at least
// one reply — most recently active first, each with the caller's thread state.
// before pages by last reply time, and beforeID — the root id of the last
// thread already shown — breaks a tie between threads that moved at the same
// instant; filter is one of the ThreadFilter values.
func (s *Store) ListThreads(ctx context.Context, channelID, userID, filter string, before *time.Time, beforeID string, limit int) ([]*Message, bool, error) {
	where := ""
	switch filter {
	case ThreadFilterFollowing:
		where = ` AND t.following`
	case ThreadFilterUnread:
		where = ` AND t.unread > 0`
	}
	rows, err := s.db.Query(ctx,
		`WITH t AS (
		   SELECT m.id, `+threadFollowingExpr+` AS following, `+threadUnreadExpr+` AS unread,
		          `+threadParticipantsExpr+` AS participants
		   FROM messages m `+threadFollowJoin+`
		   WHERE m.channel_id = $2 AND m.parent_message_id IS NULL AND m.reply_count > 0
		     AND ($3::timestamptz IS NULL OR m.last_reply_at < $3
		          OR ($5 <> '' AND m.last_reply_at = $3 AND m.id < $5))
		 )
		 SELECT `+messageCols+`, t.following, t.unread, t.participants
		 FROM t JOIN messages m ON m.id = t.id
		 LEFT JOIN users u ON m.sender_id = u.id
		 WHERE TRUE`+where+`
		 ORDER BY m.last_reply_at DESC, m.id DESC
		 LIMIT $4`, userID, channelID, before, limit+1, beforeID)
	if err != nil {
		return nil, false, fmt.Errorf("list threads: %w", err)
	}
	defer rows.Close()

	var msgs []*Message
	for rows.Next() {
		m := &Message{}
		if err := rows.Scan(&m.ID, &m.ChannelID, &m.SenderID, &m.SenderName,
			&m.Content, &m.CreatedAt, &m.MessageType, &m.ParentMessageID,
			&m.LinkedEntityType, &m.LinkedEntityID, &m.ReplyCount,
			&m.ContentFormat, &m.Mentions, &m.LastReplyAt,
			&m.EditedAt, &m.Deleted, &m.QuotedMessageID,
			&m.Following, &m.ThreadUnread, &m.ReplyParticipantIDs); err != nil {
			return nil, false, err
		}
		m.redactIfDeleted()
		msgs = append(msgs, m)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	hasMore := len(msgs) > limit
	if hasMore {
		msgs = msgs[:limit]
	}
	return msgs, hasMore, nil
}

// MessageThreadInfo returns the channel and parent of a message; found is false
// when there is no such message.
func (s *Store) MessageThreadInfo(ctx context.Context, messageID string) (channelID, parentID string, found bool, err error) {
	err = s.db.QueryRow(ctx,
		`SELECT channel_id, COALESCE(parent_message_id, '') FROM messages WHERE id = $1`, messageID).
		Scan(&channelID, &parentID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", "", false, nil
		}
		return "", "", false, fmt.Errorf("message thread info: %w", err)
	}
	return channelID, parentID, true, nil
}
