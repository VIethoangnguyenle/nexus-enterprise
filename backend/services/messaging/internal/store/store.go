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
			 VALUES ($1, $2, $3, NULLIF($4, ''), $5, $6, $7)`,
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
		&ch.NGACOaID, &ch.NGACUaID, &ch.CreatedBy, &ca, &ch.MemberCount, &ch.AvatarURL)
	if err != nil {
		return nil, err
	}
	ch.CreatedAt = ca
	return ch, nil
}

const channelCols = `c.id, c.name, c.channel_type, COALESCE(c.workspace_id,''),
	COALESCE(c.ngac_oa_id,''), COALESCE(c.ngac_ua_id,''), COALESCE(c.created_by,''), c.created_at,
	COALESCE((SELECT COUNT(*)::int FROM channel_members cm WHERE cm.channel_id = c.id), 0),
	c.avatar_url`

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

// ListAllDMs returns all DM channels (for filtering by membership).
func (s *Store) ListAllDMs(ctx context.Context) ([]*Channel, error) {
	rows, err := s.db.Query(ctx,
		`SELECT `+channelCols+` FROM channels c WHERE c.channel_type = 'dm'`)
	if err != nil {
		return nil, fmt.Errorf("list DMs: %w", err)
	}
	defer rows.Close()
	return collectChannels(rows)
}

// ListDMsOf returns the DM channels the node is recorded in. The member cache
// only narrows the candidates a caller puts to the policy service: the access
// decision itself stays with NGAC.
func (s *Store) ListDMsOf(ctx context.Context, userNodeID string) ([]*Channel, error) {
	rows, err := s.db.Query(ctx,
		`SELECT `+channelCols+` FROM channels c
		 WHERE c.channel_type = 'dm'
		   AND c.id IN (SELECT channel_id FROM channel_members WHERE ngac_node_id = $1)`, userNodeID)
	if err != nil {
		return nil, fmt.Errorf("list DMs of member: %w", err)
	}
	defer rows.Close()
	return collectChannels(rows)
}

// FindDMByMembers finds an existing DM between two users using a DB-level join.
// This replaces the O(N) full-table-scan + N gRPC calls approach.
func (s *Store) FindDMByMembers(ctx context.Context, userNodeID, targetNodeID string) (*Channel, error) {
	row := s.db.QueryRow(ctx,
		`SELECT `+channelCols+`
		 FROM channels c
		 WHERE c.channel_type = 'dm'
		   AND c.id IN (
		       SELECT channel_id FROM channel_members WHERE ngac_node_id = $1
		       INTERSECT
		       SELECT channel_id FROM channel_members WHERE ngac_node_id = $2
		   )
		 LIMIT 1`, userNodeID, targetNodeID)
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
			&ch.NGACOaID, &ch.NGACUaID, &ch.CreatedBy, &ca, &ch.MemberCount, &ch.AvatarURL); err != nil {
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
	m.edited_at, m.deleted_at IS NOT NULL, COALESCE(m.quoted_message_id, '')`

// execer is what a statement runs on: the pool, or a transaction on it.
type execer interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// insertMessage writes one message row.
func insertMessage(ctx context.Context, x execer, msg *Message) error {
	_, err := insertMessageRow(ctx, x, msg, "")
	return err
}

// insertMessageRow writes one message row and says how many rows it wrote;
// onConflict is appended to the statement ("" for none).
func insertMessageRow(ctx context.Context, x execer, msg *Message, onConflict string) (int64, error) {
	contentFormat := msg.ContentFormat
	if contentFormat == "" {
		contentFormat = "markdown"
	}
	text := msg.ContentText
	if text == "" {
		text = msg.Content
	}
	tag, err := x.Exec(ctx,
		`INSERT INTO messages (id, channel_id, sender_id, content, message_type,
		 parent_message_id, linked_entity_type, linked_entity_id,
		 content_format, mentions, created_at, content_text, quoted_message_id)
		 VALUES ($1, $2, $3, $4, $5, NULLIF($6, ''), NULLIF($7, ''), NULLIF($8, ''), $9, $10, $11, $12, NULLIF($13, ''))`+onConflict,
		msg.ID, msg.ChannelID, msg.SenderID, msg.Content, msg.MessageType,
		msg.ParentMessageID, msg.LinkedEntityType, msg.LinkedEntityID,
		contentFormat, msg.Mentions, msg.CreatedAt, text, msg.QuotedMessageID)
	if err != nil {
		return 0, fmt.Errorf("insert message: %w", err)
	}
	return tag.RowsAffected(), nil
}

// InsertMessageOnce stores a main-stream message whose id the caller derived from
// what it announces, so that the same announcement delivered twice stores one
// message. It says whether this call stored it.
func (s *Store) InsertMessageOnce(ctx context.Context, msg *Message) (bool, error) {
	n, err := insertMessageRow(ctx, s.db, msg, " ON CONFLICT (id) DO NOTHING")
	return n == 1, err
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

// ListMessages returns paginated messages for a channel, excluding thread replies.
func (s *Store) ListMessages(ctx context.Context, channelID string, before *time.Time, limit int) ([]*Message, bool, error) {
	query := `SELECT ` + messageCols + `
		FROM messages m LEFT JOIN users u ON m.sender_id = u.id
		WHERE m.channel_id = $1 AND m.parent_message_id IS NULL
		  AND (m.deleted_at IS NULL OR m.reply_count > 0)`
	args := []interface{}{channelID}

	if before != nil {
		query += ` AND m.created_at < $2 ORDER BY m.created_at DESC LIMIT $3`
		args = append(args, *before, limit+1)
	} else {
		query += ` ORDER BY m.created_at DESC LIMIT $2`
		args = append(args, limit+1)
	}

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

// GetThread returns the parent message plus all replies.
func (s *Store) GetThread(ctx context.Context, messageID string) ([]*Message, error) {
	rows, err := s.db.Query(ctx,
		`SELECT `+messageCols+`
		 FROM messages m LEFT JOIN users u ON m.sender_id = u.id
		 WHERE m.id = $1 OR (m.parent_message_id = $1 AND m.deleted_at IS NULL)
		 ORDER BY m.created_at`, messageID)
	if err != nil {
		return nil, fmt.Errorf("get thread: %w", err)
	}
	defer rows.Close()
	return scanMessages(rows)
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
		var ca time.Time
		if err := rows.Scan(&m.ID, &m.ChannelID, &m.SenderID, &m.SenderName,
			&m.Content, &ca, &m.MessageType, &m.ParentMessageID,
			&m.LinkedEntityType, &m.LinkedEntityID, &m.ReplyCount,
			&m.ContentFormat, &m.Mentions, &m.LastReplyAt,
			&m.EditedAt, &m.Deleted, &m.QuotedMessageID); err != nil {
			return nil, err
		}
		m.redactIfDeleted()
		m.CreatedAt = ca
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
