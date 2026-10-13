package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// ErrCallOpen: the conversation already has a call ringing or running.
var ErrCallOpen = errors.New("a call is already open in this conversation")

// Call is a row of the calls table.
type Call struct {
	ID            string
	ChannelID     string
	Kind          string // audio | video
	Status        string // ringing | active | ended | missed | declined | cancelled
	StartedBy     string
	MessageID     string
	CreatedAt     time.Time
	StartedAt     *time.Time
	EndedAt       *time.Time
	RingExpiresAt *time.Time
}

// CallPerson is someone in a call, named for display.
type CallPerson struct {
	UserID      string
	DisplayName string
	AvatarURL   string
	JoinedAt    time.Time
}

const callCols = `id, channel_id, kind, status, started_by, COALESCE(message_id, ''),
	created_at, started_at, ended_at, ring_expires_at`

func scanCall(row pgx.Row) (*Call, error) {
	var c Call
	err := row.Scan(&c.ID, &c.ChannelID, &c.Kind, &c.Status, &c.StartedBy, &c.MessageID,
		&c.CreatedAt, &c.StartedAt, &c.EndedAt, &c.RingExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func collectCalls(rows pgx.Rows, err error) ([]Call, error) {
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Call
	for rows.Next() {
		c, err := scanCall(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *c)
	}
	return out, rows.Err()
}

// InsertCallWithMessage stores a call and the card that announces it in one
// transaction. A second open call in the same conversation is ErrCallOpen.
func (s *Store) InsertCallWithMessage(ctx context.Context, msg *Message, c *Call) error {
	err := pgx.BeginFunc(ctx, s.db, func(tx pgx.Tx) error {
		if err := insertMessage(ctx, tx, msg); err != nil {
			return fmt.Errorf("create call card: %w", err)
		}
		_, err := tx.Exec(ctx,
			`INSERT INTO calls (id, channel_id, kind, status, started_by, message_id, created_at, started_at, ring_expires_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
			c.ID, c.ChannelID, c.Kind, c.Status, c.StartedBy, c.MessageID, c.CreatedAt, c.StartedAt, c.RingExpiresAt)
		return err
	})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "idx_calls_one_open_per_channel" {
		return ErrCallOpen
	}
	if err != nil {
		return fmt.Errorf("create call: %w", err)
	}
	return nil
}

// GetCall returns a call, nil when there is none.
func (s *Store) GetCall(ctx context.Context, id string) (*Call, error) {
	c, err := scanCall(s.db.QueryRow(ctx, `SELECT `+callCols+` FROM calls WHERE id = $1`, id))
	if err != nil {
		return nil, fmt.Errorf("get call: %w", err)
	}
	return c, nil
}

// OpenCall returns the conversation's ringing or running call, nil when none.
func (s *Store) OpenCall(ctx context.Context, channelID string) (*Call, error) {
	c, err := scanCall(s.db.QueryRow(ctx,
		`SELECT `+callCols+` FROM calls WHERE channel_id = $1 AND status IN ('ringing', 'active')`, channelID))
	if err != nil {
		return nil, fmt.Errorf("open call: %w", err)
	}
	return c, nil
}

// OpenCallsIn returns the ringing or running calls of the given conversation, or
// of every conversation of a workspace when channelID is empty.
func (s *Store) OpenCallsIn(ctx context.Context, channelID, workspaceID string) ([]Call, error) {
	calls, err := collectCalls(s.db.Query(ctx,
		`SELECT `+callCols+` FROM calls
		 WHERE status IN ('ringing', 'active')
		   AND ($1 = '' OR channel_id = $1)
		   AND ($2 = '' OR channel_id IN (SELECT id FROM channels WHERE workspace_id = $2))`,
		channelID, workspaceID))
	if err != nil {
		return nil, fmt.Errorf("open calls: %w", err)
	}
	return calls, nil
}

// TransitionCall moves a call to status when it is in one of from, stamping
// started_at (for active, once) or ended_at (for a closing status). It reports
// whether the call moved: false means another transition got there first.
func (s *Store) TransitionCall(ctx context.Context, id string, from []string, status string) (*Call, error) {
	c, err := scanCall(s.db.QueryRow(ctx,
		`UPDATE calls SET status = $3,
		        started_at = CASE WHEN $3 = 'active' THEN COALESCE(started_at, NOW()) ELSE started_at END,
		        ended_at   = CASE WHEN $3 IN ('ended', 'missed', 'declined', 'cancelled') THEN NOW() ELSE ended_at END
		 WHERE id = $1 AND status = ANY($2)
		 RETURNING `+callCols, id, from, status))
	if err != nil {
		return nil, fmt.Errorf("update call: %w", err)
	}
	return c, nil
}

// ExpireRinging marks every call that rang past its time as missed and returns
// them. Each call is returned by exactly one caller, however many instances
// sweep at once.
func (s *Store) ExpireRinging(ctx context.Context, now time.Time) ([]Call, error) {
	calls, err := collectCalls(s.db.Query(ctx,
		`UPDATE calls SET status = 'missed', ended_at = $1
		 WHERE status = 'ringing' AND ring_expires_at < $1
		 RETURNING `+callCols, now))
	if err != nil {
		return nil, fmt.Errorf("expire ringing calls: %w", err)
	}
	return calls, nil
}

// JoinCall records that a person entered a call. Being in it already (a second
// device took the first one's place) records nothing new.
func (s *Store) JoinCall(ctx context.Context, callID, userID string) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO call_participants (call_id, user_id)
		 SELECT $1, $2
		 WHERE NOT EXISTS (SELECT 1 FROM call_participants WHERE call_id = $1 AND user_id = $2 AND left_at IS NULL)`,
		callID, userID)
	if err != nil {
		return fmt.Errorf("join call: %w", err)
	}
	return nil
}

// LeaveCall records that a person left a call.
func (s *Store) LeaveCall(ctx context.Context, callID, userID string) error {
	if _, err := s.db.Exec(ctx,
		`UPDATE call_participants SET left_at = NOW() WHERE call_id = $1 AND user_id = $2 AND left_at IS NULL`,
		callID, userID); err != nil {
		return fmt.Errorf("leave call: %w", err)
	}
	return nil
}

// CloseCallStays ends every stay still open in a call (the room closed).
func (s *Store) CloseCallStays(ctx context.Context, callID string) error {
	if _, err := s.db.Exec(ctx,
		`UPDATE call_participants SET left_at = NOW() WHERE call_id = $1 AND left_at IS NULL`, callID); err != nil {
		return fmt.Errorf("close call stays: %w", err)
	}
	return nil
}

// CallPresent lists who is in a call now, by when they joined.
func (s *Store) CallPresent(ctx context.Context, callID string) ([]CallPerson, error) {
	rows, err := s.db.Query(ctx,
		`SELECT p.user_id, COALESCE(NULLIF(u.display_name, ''), u.username), COALESCE(u.avatar_url, ''), MIN(p.joined_at)
		 FROM call_participants p JOIN users u ON u.id = p.user_id
		 WHERE p.call_id = $1 AND p.left_at IS NULL
		 GROUP BY p.user_id, u.display_name, u.username, u.avatar_url
		 ORDER BY MIN(p.joined_at)`, callID)
	if err != nil {
		return nil, fmt.Errorf("call participants: %w", err)
	}
	defer rows.Close()
	var out []CallPerson
	for rows.Next() {
		var p CallPerson
		if err := rows.Scan(&p.UserID, &p.DisplayName, &p.AvatarURL, &p.JoinedAt); err != nil {
			return nil, fmt.Errorf("call participants: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// CallTakers counts the distinct people who were ever in a call.
func (s *Store) CallTakers(ctx context.Context, callID string) (int32, error) {
	var n int32
	if err := s.db.QueryRow(ctx,
		`SELECT count(DISTINCT user_id) FROM call_participants WHERE call_id = $1`, callID).Scan(&n); err != nil {
		return 0, fmt.Errorf("count call participants: %w", err)
	}
	return n, nil
}

// CallPeople names users for display, in the order asked; unknown ids are left out.
func (s *Store) CallPeople(ctx context.Context, userIDs []string) ([]CallPerson, error) {
	rows, err := s.db.Query(ctx,
		`SELECT u.id, COALESCE(NULLIF(u.display_name, ''), u.username), COALESCE(u.avatar_url, '')
		 FROM unnest($1::text[]) WITH ORDINALITY AS q(id, n) JOIN users u ON u.id = q.id
		 ORDER BY q.n`, userIDs)
	if err != nil {
		return nil, fmt.Errorf("call people: %w", err)
	}
	defer rows.Close()
	var out []CallPerson
	for rows.Next() {
		var p CallPerson
		if err := rows.Scan(&p.UserID, &p.DisplayName, &p.AvatarURL); err != nil {
			return nil, fmt.Errorf("call people: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// SetCardText rewrites the text of a call's card, which the conversation
// previews and search read. It is the call's state, not an edit by a person, so
// it leaves edited_at alone.
func (s *Store) SetCardText(ctx context.Context, messageID, text string) error {
	if _, err := s.db.Exec(ctx,
		`UPDATE messages SET content = $2, content_text = $2 WHERE id = $1 AND deleted_at IS NULL`,
		messageID, text); err != nil {
		return fmt.Errorf("update call card: %w", err)
	}
	return nil
}

// UserNodes maps user ids to their NGAC user nodes; unknown ids are left out.
// GrantCallAccess records the node a person was checked as when handed a
// ticket into a call, replacing an earlier one.
func (s *Store) GrantCallAccess(ctx context.Context, callID, userID, nodeID, tenantID string) error {
	if _, err := s.db.Exec(ctx,
		`INSERT INTO call_access (call_id, user_id, ngac_node_id, tenant_id) VALUES ($1, $2, $3, $4)
		 ON CONFLICT (call_id, user_id) DO UPDATE
		 SET ngac_node_id = EXCLUDED.ngac_node_id, tenant_id = EXCLUDED.tenant_id, granted_at = NOW()`,
		callID, userID, nodeID, tenantID); err != nil {
		return fmt.Errorf("record call access: %w", err)
	}
	return nil
}

// CallAccessNode returns the node and workspace a person was last handed a
// ticket into a call as; empty when they never were.
func (s *Store) CallAccessNode(ctx context.Context, callID, userID string) (node, tenantID string, err error) {
	err = s.db.QueryRow(ctx,
		`SELECT ngac_node_id, tenant_id FROM call_access WHERE call_id = $1 AND user_id = $2`, callID, userID).Scan(&node, &tenantID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", nil
	}
	if err != nil {
		return "", "", fmt.Errorf("call access: %w", err)
	}
	return node, tenantID, nil
}

// DMOthers returns the user ids of a DM's members other than userID, from the
// channel_members cache. A member is recorded by NGAC node, which is a
// workspace node (tenant_users) or a person's own (users).
func (s *Store) DMOthers(ctx context.Context, channelID, userID string) ([]string, error) {
	rows, err := s.db.Query(ctx,
		`SELECT id FROM (
		   SELECT tu.user_id AS id FROM channel_members m JOIN tenant_users tu ON tu.ngac_node_id = m.ngac_node_id
		   WHERE m.channel_id = $1
		   UNION
		   SELECT u.id FROM channel_members m JOIN users u ON u.ngac_node = m.ngac_node_id
		   WHERE m.channel_id = $1
		 ) members WHERE id <> $2`, channelID, userID)
	if err != nil {
		return nil, fmt.Errorf("dm members: %w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("dm members: %w", err)
		}
		out = append(out, id)
	}
	return out, rows.Err()
}
