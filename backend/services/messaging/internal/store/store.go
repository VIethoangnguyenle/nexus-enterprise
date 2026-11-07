// Package store handles all database operations for the messaging service.
// It owns the SQL queries and row scanning; no business logic lives here.
package store

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Store provides database access for channels, messages, and threads.
type Store struct {
	db *pgxpool.Pool
	// wideIndex remembers whether channels_one_workspace_wide exists; see
	// workspaceChannelsIndexed.
	wideIndex atomic.Int32
}

// NewStore creates a Store backed by the given connection pool.
func NewStore(db *pgxpool.Pool) *Store {
	return &Store{db: db}
}

// --- Channels ---

// InsertChannelWithMembers persists a new channel and records its first members
// in the channel_members cache in one transaction: a channel row without its
// members would make a DM unfindable by its pair and be created twice.
func (s *Store) InsertChannelWithMembers(ctx context.Context, ch *Channel, memberNodeIDs ...string) error {
	err := pgx.BeginFunc(ctx, s.db, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx,
			`INSERT INTO channels (id, name, channel_type, workspace_id, ngac_oa_id, ngac_ua_id, created_by)
			 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			ch.ID, ch.Name, ch.ChannelType, ch.WorkspaceID,
			ch.NGACOaID, ch.NGACUaID, ch.CreatedBy); err != nil {
			return fmt.Errorf("insert channel: %w", err)
		}
		for _, node := range memberNodeIDs {
			if _, err := tx.Exec(ctx,
				`INSERT INTO channel_members (channel_id, ngac_node_id) VALUES ($1, $2)
				 ON CONFLICT DO NOTHING`, ch.ID, node); err != nil {
				return fmt.Errorf("record channel member: %w", err)
			}
		}
		return nil
	})
	return asWorkspaceChannelExists(err)
}

// UpdateChannelName renames a channel.
func (s *Store) UpdateChannelName(ctx context.Context, id, name string) error {
	tag, err := s.db.Exec(ctx,
		`UPDATE channels SET name = $1 WHERE id = $2`, name, id)
	if err != nil {
		return fmt.Errorf("update channel name: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("channel not found")
	}
	return nil
}

// scanChannel scans a channel row from the standard column set.
func scanChannel(row pgx.Row) (*Channel, error) {
	ch := &Channel{}
	var ca time.Time
	err := row.Scan(&ch.ID, &ch.Name, &ch.ChannelType, &ch.WorkspaceID,
		&ch.NGACOaID, &ch.NGACUaID, &ch.CreatedBy, &ca, &ch.MemberCount, &ch.AvatarURL,
		&ch.PostPolicy, &ch.InvitePolicy, &ch.MentionAllPolicy)
	if err != nil {
		return nil, err
	}
	ch.CreatedAt = ca
	return ch, nil
}

const channelCols = `c.id, c.name, c.channel_type, c.workspace_id,
	COALESCE(c.ngac_oa_id,''), COALESCE(c.ngac_ua_id,''), COALESCE(c.created_by,''), c.created_at,
	COALESCE((SELECT COUNT(*)::int FROM channel_members cm WHERE cm.channel_id = c.id), 0),
	c.avatar_url,
	COALESCE(c.post_policy, 'everyone'), COALESCE(c.invite_policy, 'everyone'), COALESCE(c.mention_all_policy, 'everyone')`

// GetChannel retrieves a single channel by ID.
func (s *Store) GetChannel(ctx context.Context, id string) (*Channel, error) {
	row := s.db.QueryRow(ctx,
		`SELECT `+channelCols+` FROM channels c WHERE c.id = $1`, id)
	ch, err := scanChannel(row)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return ch, err
}

// ListChannelsByWorkspace returns non-DM channels for a workspace.
func (s *Store) ListChannelsByWorkspace(ctx context.Context, workspaceID string) ([]*Channel, error) {
	rows, err := s.db.Query(ctx,
		`SELECT `+channelCols+` FROM channels c
		 WHERE c.workspace_id = $1 AND c.channel_type != 'dm'`, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list channels: %w", err)
	}
	defer rows.Close()
	return collectChannels(rows)
}

// ListDMsOf returns the DM channels of the workspace that the node is recorded
// in. A DM belongs to exactly one workspace (the schema requires it), and an
// empty workspaceID names no workspace. The member cache only narrows the candidates
// a caller puts to the policy service: the access decision itself stays with
// NGAC.
func (s *Store) ListDMsOf(ctx context.Context, workspaceID, userNodeID string) ([]*Channel, error) {
	rows, err := s.db.Query(ctx,
		`SELECT `+channelCols+` FROM channels c
		 WHERE c.channel_type = 'dm' AND c.workspace_id = NULLIF($1, '')
		   AND c.id IN (SELECT channel_id FROM channel_members WHERE ngac_node_id = $2)`, workspaceID, userNodeID)
	if err != nil {
		return nil, fmt.Errorf("list DMs of member: %w", err)
	}
	defer rows.Close()
	return collectChannels(rows)
}

// FindDMByMembers finds the DM two users have in a workspace, using a DB-level
// join. The same two people have one DM per workspace they share, so the
// workspace is part of the question; an empty workspaceID names no workspace
// and finds nothing.
func (s *Store) FindDMByMembers(ctx context.Context, workspaceID, userNodeID, targetNodeID string) (*Channel, error) {
	row := s.db.QueryRow(ctx,
		`SELECT `+channelCols+`
		 FROM channels c
		 WHERE c.channel_type = 'dm' AND c.workspace_id = NULLIF($1, '')
		   AND c.id IN (
		       SELECT channel_id FROM channel_members WHERE ngac_node_id = $2
		       INTERSECT
		       SELECT channel_id FROM channel_members WHERE ngac_node_id = $3
		   )
		 ORDER BY c.created_at, c.id
		 LIMIT 1`, workspaceID, userNodeID, targetNodeID)
	ch, err := scanChannel(row)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return ch, err
}

// InsertChannelMember records a member in the channel_members table.
// This is used for DM lookup optimization.
func (s *Store) InsertChannelMember(ctx context.Context, channelID, ngacNodeID string) error {
	if _, err := s.db.Exec(ctx,
		`INSERT INTO channel_members (channel_id, ngac_node_id) VALUES ($1, $2)
		 ON CONFLICT DO NOTHING`, channelID, ngacNodeID); err != nil {
		return fmt.Errorf("record channel member: %w", err)
	}
	return nil
}

// DeleteChannelMember drops a member's row from the channel_members table. It is
// idempotent: a row already gone is not an error.
func (s *Store) DeleteChannelMember(ctx context.Context, channelID, ngacNodeID string) error {
	if _, err := s.db.Exec(ctx,
		`DELETE FROM channel_members WHERE channel_id = $1 AND ngac_node_id = $2`, channelID, ngacNodeID); err != nil {
		return fmt.Errorf("forget channel member: %w", err)
	}
	return nil
}

// Workspace holds workspace metadata needed for channel assignment.
type Workspace struct {
	ID       string
	Name     string
	PCNodeID string
}

// GetWorkspaceByID returns workspace metadata by ID.
func (s *Store) GetWorkspaceByID(ctx context.Context, workspaceID string) (*Workspace, error) {
	var ws Workspace
	err := s.db.QueryRow(ctx,
		`SELECT id, name, ngac_pc_id FROM workspaces WHERE id = $1`, workspaceID).
		Scan(&ws.ID, &ws.Name, &ws.PCNodeID)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get workspace: %w", err)
	}
	return &ws, nil
}

func collectChannels(rows pgx.Rows) ([]*Channel, error) {
	var channels []*Channel
	for rows.Next() {
		ch := &Channel{}
		var ca time.Time
		if err := rows.Scan(&ch.ID, &ch.Name, &ch.ChannelType, &ch.WorkspaceID,
			&ch.NGACOaID, &ch.NGACUaID, &ch.CreatedBy, &ca, &ch.MemberCount, &ch.AvatarURL,
			&ch.PostPolicy, &ch.InvitePolicy, &ch.MentionAllPolicy); err != nil {
			return nil, err
		}
		ch.CreatedAt = ca
		channels = append(channels, ch)
	}
	return channels, nil
}

// --- Messages ---

const messageCols = `m.id, m.channel_id, m.sender_id, COALESCE(u.username,''), m.content, m.created_at,
	COALESCE(m.message_type,'user'), COALESCE(m.parent_message_id,''),
	COALESCE(m.linked_entity_type,''), COALESCE(m.linked_entity_id,''), m.reply_count,
	COALESCE(m.content_format,'markdown'), COALESCE(m.mentions, '{}'), m.last_reply_at,
	m.edited_at, m.deleted_at IS NOT NULL, COALESCE(m.quoted_message_id, ''),
	COALESCE(m.seq, 0), COALESCE(m.reply_seq, 0)`

// scanTargets are where a row of messageCols is read into, in its order.
func (m *Message) scanTargets() []any {
	return []any{&m.ID, &m.ChannelID, &m.SenderID, &m.SenderName,
		&m.Content, &m.CreatedAt, &m.MessageType, &m.ParentMessageID,
		&m.LinkedEntityType, &m.LinkedEntityID, &m.ReplyCount,
		&m.ContentFormat, &m.Mentions, &m.LastReplyAt,
		&m.EditedAt, &m.Deleted, &m.QuotedMessageID,
		&m.Seq, &m.ReplySeq}
}

// execer is what a statement runs on: the pool, or a transaction on it.
type execer interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// insertMessage writes one message row.
// insertMessage writes one message row, in the caller's transaction, and puts
// on msg the number the insert trigger gave it (data/migrations/072).
func insertMessage(ctx context.Context, tx pgx.Tx, msg *Message) error {
	contentFormat := msg.ContentFormat
	if contentFormat == "" {
		contentFormat = "markdown"
	}
	text := msg.ContentText
	if text == "" {
		text = msg.Content
	}
	err := tx.QueryRow(ctx,
		`INSERT INTO messages (id, channel_id, sender_id, content, message_type,
		 parent_message_id, linked_entity_type, linked_entity_id,
		 content_format, mentions, created_at, content_text, quoted_message_id)
		 VALUES ($1, $2, $3, $4, $5, NULLIF($6, ''), NULLIF($7, ''), NULLIF($8, ''), $9, $10, $11, $12, NULLIF($13, ''))
		 RETURNING COALESCE(seq, 0), COALESCE(reply_seq, 0)`,
		msg.ID, msg.ChannelID, msg.SenderID, msg.Content, msg.MessageType,
		msg.ParentMessageID, msg.LinkedEntityType, msg.LinkedEntityID,
		contentFormat, msg.Mentions, msg.CreatedAt, text, msg.QuotedMessageID).Scan(&msg.Seq, &msg.ReplySeq)
	if err != nil {
		return fmt.Errorf("insert message: %w", err)
	}
	return nil
}

// InsertMessageOnce stores a main-stream message whose id the caller derived from
// what it announces, so that the same announcement delivered twice stores one
// message. It says whether this call stored it.
//
// The duplicate is found before the insert rather than by ON CONFLICT: the
// insert trigger has already taken the channel's next number by the time a
// conflict is seen, and a skipped number would read as an unread message for
// ever. Holding the channel row makes two deliveries of one announcement take
// turns, so the second sees the first.
func (s *Store) InsertMessageOnce(ctx context.Context, msg *Message) (bool, error) {
	stored := false
	err := pgx.BeginFunc(ctx, s.db, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT 1 FROM channels WHERE id = $1 FOR UPDATE`, msg.ChannelID); err != nil {
			return fmt.Errorf("hold channel: %w", err)
		}
		var exists bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM messages WHERE id = $1)`, msg.ID).Scan(&exists); err != nil {
			return fmt.Errorf("find message: %w", err)
		}
		if exists {
			return nil
		}
		if err := insertMessage(ctx, tx, msg); err != nil {
			return err
		}
		stored = true
		return nil
	})
	return stored, err
}

// InsertMessage persists a new message. A reply is counted on its parent, moves
// the thread's last reply time, and makes its sender and the users it mentions
// follow the thread, all in one transaction.
func (s *Store) InsertMessage(ctx context.Context, msg *Message) error {
	return pgx.BeginFunc(ctx, s.db, func(tx pgx.Tx) error {
		if err := insertMessage(ctx, tx, msg); err != nil {
			return err
		}
		if err := insertAttachments(ctx, tx, msg); err != nil {
			return err
		}
		return recordReply(ctx, tx, msg)
	})
}

// recordReply does the thread bookkeeping of a stored message that is a reply
// (a root message needs none). A reply that was stored but not counted would
// show a thread with the wrong number of replies for ever, so the caller runs
// this in the transaction that stored the message.
func recordReply(ctx context.Context, tx pgx.Tx, msg *Message) error {
	if msg.ParentMessageID == "" {
		// The sender and anyone mentioned follow a new thread implicitly
		// (see threadFollowingExpr), so a root message writes no follow row.
		return nil
	}
	tag, err := tx.Exec(ctx,
		`UPDATE messages SET reply_count = reply_count + 1,
		        last_reply_at = GREATEST(COALESCE(last_reply_at, $2), $2)
		 WHERE id = $1 AND deleted_at IS NULL`, msg.ParentMessageID, msg.CreatedAt)
	if err != nil {
		return fmt.Errorf("count reply: %w", err)
	}
	if tag.RowsAffected() == 0 {
		// The parent was deleted between the check and this write.
		return ErrParentGone
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO thread_participants (message_id, user_id) VALUES ($1, $2)
		 ON CONFLICT DO NOTHING`, msg.ParentMessageID, msg.SenderID); err != nil {
		return fmt.Errorf("track thread participant: %w", err)
	}
	// Replying follows the thread again even after an unfollow, and the
	// replier has read everything up to their own reply.
	if _, err := tx.Exec(ctx,
		`INSERT INTO thread_follows (user_id, root_message_id, following, last_read_at, updated_at)
		 VALUES ($1, $2, TRUE, $3, NOW())
		 ON CONFLICT (user_id, root_message_id) DO UPDATE SET
		   following = TRUE,
		   last_read_at = GREATEST(thread_follows.last_read_at, EXCLUDED.last_read_at),
		   updated_at = NOW()`, msg.SenderID, msg.ParentMessageID, msg.CreatedAt); err != nil {
		return fmt.Errorf("follow thread: %w", err)
	}
	// A mention in a reply makes the mentioned user follow the thread.
	if len(msg.Mentions) > 0 {
		return followMentioned(ctx, tx, msg.Mentions, msg.ParentMessageID, msg.SenderID)
	}
	return nil
}

// ListMessages returns a page of a channel's main messages, newest first, and
// whether older ones remain. The page ends below beforeSeq when it is set (a
// number cursor never skips or repeats a message); otherwise below before when
// that is set, the time cursor of clients older than the numbers. A deleted
// message is kept while it has replies, as its thread's placeholder.
func (s *Store) ListMessages(ctx context.Context, channelID string, beforeSeq int64, before *time.Time, limit int) ([]*Message, bool, error) {
	query := `SELECT ` + messageCols + `
		FROM messages m LEFT JOIN users u ON m.sender_id = u.id
		WHERE m.channel_id = $1 AND m.parent_message_id IS NULL
		  AND (m.deleted_at IS NULL OR m.reply_count > 0)`
	args := []any{channelID}
	switch {
	case beforeSeq > 0:
		args = append(args, beforeSeq)
		query += ` AND m.seq < $2`
	case before != nil:
		args = append(args, *before)
		query += ` AND m.created_at < $2`
	}
	args = append(args, limit+1)
	query += fmt.Sprintf(` ORDER BY m.seq DESC LIMIT $%d`, len(args))

	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, false, fmt.Errorf("list messages: %w", err)
	}
	defer rows.Close()

	msgs, err := scanMessages(rows)
	if err != nil {
		return nil, false, err
	}
	hasMore := len(msgs) > limit
	if hasMore {
		msgs = msgs[:limit]
	}
	return msgs, hasMore, nil
}

// GetMessage returns one message, deleted or not, with its sender's name; nil
// when there is none.
func (s *Store) GetMessage(ctx context.Context, id string) (*Message, error) {
	rows, err := s.db.Query(ctx,
		`SELECT `+messageCols+`
		 FROM messages m LEFT JOIN users u ON m.sender_id = u.id
		 WHERE m.id = $1`, id)
	if err != nil {
		return nil, fmt.Errorf("get message: %w", err)
	}
	defer rows.Close()
	msgs, err := scanMessages(rows)
	if err != nil || len(msgs) == 0 {
		return nil, err
	}
	return msgs[0], nil
}

// GetThread returns a thread's root message followed by a page of its live
// replies, oldest first: those numbered above afterReplySeq, at most limit of
// them; and whether later replies remain.
func (s *Store) GetThread(ctx context.Context, rootID string, afterReplySeq int64, limit int) ([]*Message, bool, error) {
	rows, err := s.db.Query(ctx,
		`SELECT `+messageCols+`
		 FROM messages m LEFT JOIN users u ON m.sender_id = u.id
		 WHERE m.id = $1
		    OR (m.parent_message_id = $1 AND m.reply_seq > $2 AND m.deleted_at IS NULL)
		 ORDER BY COALESCE(m.reply_seq, 0) LIMIT $3`, rootID, afterReplySeq, limit+2)
	if err != nil {
		return nil, false, fmt.Errorf("get thread: %w", err)
	}
	defer rows.Close()
	msgs, err := scanMessages(rows)
	if err != nil {
		return nil, false, fmt.Errorf("get thread: %w", err)
	}
	// The root has no reply number, so it sorts first; one row past the page
	// says whether more remain.
	hasMore := len(msgs) > limit+1
	if hasMore {
		msgs = msgs[:limit+1]
	}
	return msgs, hasMore, nil
}

// FindByEntity returns messages linked to a specific entity.
func (s *Store) FindByEntity(ctx context.Context, entityType, entityID string) ([]*Message, error) {
	rows, err := s.db.Query(ctx,
		`SELECT `+messageCols+`
		 FROM messages m LEFT JOIN users u ON m.sender_id = u.id
		 WHERE m.linked_entity_type = $1 AND m.linked_entity_id = $2 AND m.deleted_at IS NULL
		 ORDER BY m.created_at DESC`, entityType, entityID)
	if err != nil {
		return nil, fmt.Errorf("find by entity: %w", err)
	}
	defer rows.Close()
	return scanMessages(rows)
}

func scanMessages(rows pgx.Rows) ([]*Message, error) {
	var msgs []*Message
	for rows.Next() {
		m := &Message{}
		if err := rows.Scan(m.scanTargets()...); err != nil {
			return nil, err
		}
		m.redactIfDeleted()
		msgs = append(msgs, m)
	}
	return msgs, nil
}

// WorkspaceDocumentsOA returns the ID of the workspace's Documents OA, the
// workspace-level attribute a "read the workspace" check lands on. Empty when
// the workspace does not exist or has none.
func (s *Store) WorkspaceDocumentsOA(ctx context.Context, workspaceID string) (string, error) {
	var oaID string
	err := s.db.QueryRow(ctx,
		`SELECT COALESCE(documents_oa_id, '') FROM workspaces WHERE id = $1`, workspaceID).Scan(&oaID)
	if err == pgx.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("workspace documents oa: %w", err)
	}
	return oaID, nil
}
