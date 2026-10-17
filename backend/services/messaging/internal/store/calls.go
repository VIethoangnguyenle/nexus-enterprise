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
	ID string
	// ChannelID is the conversation the call is in; empty for the call of a
	// calendar event, which has CalendarEventID instead (exactly one is set).
	ChannelID       string
	CalendarEventID string
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

const callCols = `id, COALESCE(channel_id, ''), COALESCE(calendar_event_id, ''), kind, status, started_by, COALESCE(message_id, ''),
	created_at, started_at, ended_at, ring_expires_at`

func scanCall(row pgx.Row) (*Call, error) {
	var c Call
	err := row.Scan(&c.ID, &c.ChannelID, &c.CalendarEventID, &c.Kind, &c.Status, &c.StartedBy, &c.MessageID,
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

// ErrEventCallOpen: the event already has a call ringing or running.
var ErrEventCallOpen = errors.New("a call is already open for this event")

// EventCallRef is what the call of a calendar event needs to know of the event.
type EventCallRef struct {
	// ID is the series (or single event) the guests and the call belong to.
	ID          string
	WorkspaceID string
	Title       string
	HasCall     bool
	// OAID is the node a person must read to be in the call: the event's own,
	// else the one of the calendar it is on.
	OAID string
}

// EventCallRef reads the event a call belongs to, nil when there is no such
// event. An exception of a repeating event stands for its series.
func (s *Store) EventCallRef(ctx context.Context, eventID string) (*EventCallRef, error) {
	var r EventCallRef
	err := s.db.QueryRow(ctx,
		`SELECT m.id, m.workspace_id, m.title, m.has_call, COALESCE(m.ngac_oa_id, c.ngac_oa_id, '')
		 FROM calendar_events e
		 JOIN calendar_events m ON m.id = COALESCE(e.recurring_event_id, e.id)
		 JOIN calendars c ON c.id = m.calendar_id
		 WHERE e.id = $1`, eventID).Scan(&r.ID, &r.WorkspaceID, &r.Title, &r.HasCall, &r.OAID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("event call ref: %w", err)
	}
	return &r, nil
}

// OpenEventCall returns the event's ringing or running call, nil when none.
func (s *Store) OpenEventCall(ctx context.Context, eventID string) (*Call, error) {
	c, err := scanCall(s.db.QueryRow(ctx,
		`SELECT `+callCols+` FROM calls WHERE calendar_event_id = $1 AND status IN ('ringing', 'active')`, eventID))
	if err != nil {
		return nil, fmt.Errorf("open event call: %w", err)
	}
	return c, nil
}

// InsertEventCall stores the call of a calendar event. A second open call for
// the same event is ErrEventCallOpen.
func (s *Store) InsertEventCall(ctx context.Context, c *Call) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO calls (id, calendar_event_id, kind, status, started_by, created_at, started_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		c.ID, c.CalendarEventID, c.Kind, c.Status, c.StartedBy, c.CreatedAt, c.StartedAt)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "idx_calls_one_open_per_event" {
		return ErrEventCallOpen
	}
	if err != nil {
		return fmt.Errorf("create event call: %w", err)
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
// OpenCallsIn returns the ringing or running calls of a conversation; of a
// workspace — its conversations, and the direct calls someone took a ticket
// into from it (a DM belongs to no workspace); or, naming neither, all of them.
func (s *Store) OpenCallsIn(ctx context.Context, channelID, workspaceID string) ([]Call, error) {
	calls, err := collectCalls(s.db.Query(ctx,
		`SELECT `+callCols+` FROM calls
		 WHERE status IN ('ringing', 'active')
		   AND ($1 = '' OR channel_id = $1)
		   AND ($2 = '' OR channel_id IN (SELECT id FROM channels WHERE workspace_id = $2)
		                OR id IN (SELECT call_id FROM call_access WHERE tenant_id = $2))`,
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
// JoinCall records that a person entered a call through one connection (its
// LiveKit participant sid). The same connection reported twice records nothing
// new; with no sid, being in the call already records nothing new.
func (s *Store) JoinCall(ctx context.Context, callID, userID, sid string) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO call_participants (call_id, user_id, participant_sid)
		 SELECT $1, $2, $3
		 WHERE NOT EXISTS (SELECT 1 FROM call_participants
		                   WHERE call_id = $1 AND user_id = $2 AND left_at IS NULL AND ($3 = '' OR participant_sid = $3))`,
		callID, userID, sid)
	if err != nil {
		return fmt.Errorf("join call: %w", err)
	}
	return nil
}

// LeaveCall records that a person left a call.
// LeaveCall records that a connection left a call; with no sid, every stay of
// the person ends (they were put out).
func (s *Store) LeaveCall(ctx context.Context, callID, userID, sid string) error {
	if _, err := s.db.Exec(ctx,
		`UPDATE call_participants SET left_at = NOW()
		 WHERE call_id = $1 AND user_id = $2 AND left_at IS NULL AND ($3 = '' OR participant_sid = $3)`,
		callID, userID, sid); err != nil {
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

// CallLastSeen is when anyone was last seen in a call: the latest arrival or
// departure, or its creation when nobody ever came.
func (s *Store) CallLastSeen(ctx context.Context, callID string) (time.Time, error) {
	var t time.Time
	if err := s.db.QueryRow(ctx,
		`SELECT GREATEST(c.created_at, COALESCE(MAX(p.joined_at), c.created_at), COALESCE(MAX(p.left_at), c.created_at))
		 FROM calls c LEFT JOIN call_participants p ON p.call_id = c.id
		 WHERE c.id = $1 GROUP BY c.created_at`, callID).Scan(&t); err != nil {
		return time.Time{}, fmt.Errorf("call last seen: %w", err)
	}
	return t, nil
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

// WorkspaceNodes maps user ids to the NGAC node each acts as in a workspace:
// their workspace node (tenant_users), else their own. Unknown ids are left out.
func (s *Store) WorkspaceNodes(ctx context.Context, workspaceID string, userIDs []string) (map[string]string, error) {
	rows, err := s.db.Query(ctx,
		`SELECT u.id, COALESCE(tu.ngac_node_id, u.ngac_node, '')
		 FROM users u LEFT JOIN tenant_users tu ON tu.user_id = u.id AND tu.tenant_id = $1
		 WHERE u.id = ANY($2)`, workspaceID, userIDs)
	if err != nil {
		return nil, fmt.Errorf("workspace nodes: %w", err)
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var id, node string
		if err := rows.Scan(&id, &node); err != nil {
			return nil, fmt.Errorf("workspace nodes: %w", err)
		}
		out[id] = node
	}
	return out, rows.Err()
}
