// Package store handles all database operations for the messaging service.
// It owns the SQL queries and row scanning; no business logic lives here.
package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Store provides database access for channels, messages, and threads.
type Store struct {
	db *pgxpool.Pool
}

// NewStore creates a Store backed by the given connection pool.
func NewStore(db *pgxpool.Pool) *Store {
	return &Store{db: db}
}

// --- Channels ---

// InsertChannel persists a new channel row.
func (s *Store) InsertChannel(ctx context.Context, ch *Channel) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO channels (id, name, channel_type, workspace_id, ngac_oa_id, ngac_ua_id, created_by)
		 VALUES ($1, $2, $3, NULLIF($4, ''), $5, $6, $7)`,
		ch.ID, ch.Name, ch.ChannelType, ch.WorkspaceID,
		ch.NGACOaID, ch.NGACUaID, ch.CreatedBy)
	if err != nil {
		return fmt.Errorf("insert channel: %w", err)
	}
	return nil
}

// scanChannel scans a channel row from the standard column set.
func scanChannel(row pgx.Row) (*Channel, error) {
	ch := &Channel{}
	var ca time.Time
	err := row.Scan(&ch.ID, &ch.Name, &ch.ChannelType, &ch.WorkspaceID,
		&ch.NGACOaID, &ch.NGACUaID, &ch.CreatedBy, &ca)
	if err != nil {
		return nil, err
	}
	ch.CreatedAt = ca
	return ch, nil
}

const channelCols = `id, name, channel_type, COALESCE(workspace_id,''),
	COALESCE(ngac_oa_id,''), COALESCE(ngac_ua_id,''), COALESCE(created_by,''), created_at`

// GetChannel retrieves a single channel by ID.
func (s *Store) GetChannel(ctx context.Context, id string) (*Channel, error) {
	row := s.db.QueryRow(ctx,
		`SELECT `+channelCols+` FROM channels WHERE id = $1`, id)
	ch, err := scanChannel(row)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return ch, err
}

// ListChannelsByWorkspace returns non-DM channels for a workspace.
func (s *Store) ListChannelsByWorkspace(ctx context.Context, workspaceID string) ([]*Channel, error) {
	rows, err := s.db.Query(ctx,
		`SELECT `+channelCols+` FROM channels
		 WHERE workspace_id = $1 AND channel_type != 'dm'`, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list channels: %w", err)
	}
	defer rows.Close()
	return collectChannels(rows)
}

// ListAllDMs returns all DM channels (for filtering by membership).
func (s *Store) ListAllDMs(ctx context.Context) ([]*Channel, error) {
	rows, err := s.db.Query(ctx,
		`SELECT `+channelCols+` FROM channels WHERE channel_type = 'dm'`)
	if err != nil {
		return nil, fmt.Errorf("list DMs: %w", err)
	}
	defer rows.Close()
	return collectChannels(rows)
}

// FindDMByMembers finds an existing DM between two users using a DB-level join.
// This replaces the O(N) full-table-scan + N gRPC calls approach.
func (s *Store) FindDMByMembers(ctx context.Context, userNodeID, targetNodeID string) (*Channel, error) {
	row := s.db.QueryRow(ctx,
		`SELECT `+channelCols+`
		 FROM channels
		 WHERE channel_type = 'dm'
		   AND id IN (
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
	_, err := s.db.Exec(ctx,
		`INSERT INTO channel_members (channel_id, ngac_node_id) VALUES ($1, $2)
		 ON CONFLICT DO NOTHING`, channelID, ngacNodeID)
	return err
}

// GetWorkspaceName returns workspace name and PC node ID.
func (s *Store) GetWorkspaceName(ctx context.Context, workspaceID string) (name, pcNodeID string, err error) {
	err = s.db.QueryRow(ctx,
		`SELECT name, ngac_pc_id FROM workspaces WHERE id = $1`, workspaceID).
		Scan(&name, &pcNodeID)
	if err == pgx.ErrNoRows {
		return "", "", nil
	}
	return name, pcNodeID, err
}

func collectChannels(rows pgx.Rows) ([]*Channel, error) {
	var channels []*Channel
	for rows.Next() {
		ch := &Channel{}
		var ca time.Time
		if err := rows.Scan(&ch.ID, &ch.Name, &ch.ChannelType, &ch.WorkspaceID,
			&ch.NGACOaID, &ch.NGACUaID, &ch.CreatedBy, &ca); err != nil {
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
	COALESCE(m.linked_entity_type,''), COALESCE(m.linked_entity_id,''), m.reply_count`

// InsertMessage persists a new message row.
func (s *Store) InsertMessage(ctx context.Context, msg *Message) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO messages (id, channel_id, sender_id, content, message_type,
		 parent_message_id, linked_entity_type, linked_entity_id, created_at)
		 VALUES ($1, $2, $3, $4, $5, NULLIF($6, ''), NULLIF($7, ''), NULLIF($8, ''), $9)`,
		msg.ID, msg.ChannelID, msg.SenderID, msg.Content, msg.MessageType,
		msg.ParentMessageID, msg.LinkedEntityType, msg.LinkedEntityID, msg.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert message: %w", err)
	}
	return nil
}

// IncrementReplyCount bumps the reply counter on a parent message.
func (s *Store) IncrementReplyCount(ctx context.Context, parentID string) {
	s.db.Exec(ctx,
		`UPDATE messages SET reply_count = reply_count + 1 WHERE id = $1`, parentID)
}

// TrackThreadParticipant records a user as a participant in a thread.
func (s *Store) TrackThreadParticipant(ctx context.Context, parentID, userID string) {
	s.db.Exec(ctx,
		`INSERT INTO thread_participants (message_id, user_id) VALUES ($1, $2)
		 ON CONFLICT DO NOTHING`, parentID, userID)
}

// ListMessages returns paginated messages for a channel, excluding thread replies.
func (s *Store) ListMessages(ctx context.Context, channelID string, before *time.Time, limit int) ([]*Message, bool, error) {
	query := `SELECT ` + messageCols + `
		FROM messages m LEFT JOIN users u ON m.sender_id = u.id
		WHERE m.channel_id = $1 AND m.parent_message_id IS NULL`
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
		 WHERE m.id = $1 OR m.parent_message_id = $1
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
		 WHERE m.linked_entity_type = $1 AND m.linked_entity_id = $2
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
			&m.LinkedEntityType, &m.LinkedEntityID, &m.ReplyCount); err != nil {
			return nil, err
		}
		m.CreatedAt = ca
		msgs = append(msgs, m)
	}
	return msgs, nil
}
