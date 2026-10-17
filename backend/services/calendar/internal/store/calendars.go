package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// Kinds of calendar.
const (
	KindPersonal  = "personal"
	KindWorkspace = "workspace"
	KindChannel   = "channel"
)

// Calendar is one calendar with the name a screen shows for it.
type Calendar struct {
	ID          string
	WorkspaceID string
	Kind        string
	// OwnerNodeID is the owner's user node (personal calendars).
	OwnerNodeID string
	// ChannelID is the conversation a group calendar belongs to.
	ChannelID string
	// Name is the stored name, else the owner's name (personal), the workspace's
	// (workspace) or the conversation's (group).
	Name string
	// Color is the calendar's own palette index.
	Color int16
	// NgacOAID is the OA the calendar is authorized on.
	NgacOAID  string
	CreatedAt time.Time
	// Owner is the person a personal calendar belongs to, when they are still a
	// member of the workspace.
	Owner *Person
}

const calendarColumns = `c.id, c.workspace_id, c.kind, COALESCE(c.owner_node_id, ''), COALESCE(c.channel_id, ''),
		COALESCE(NULLIF(c.name, ''), CASE c.kind
			WHEN 'personal' THEN COALESCE(NULLIF(ou.display_name, ''), ou.username, '')
			WHEN 'workspace' THEN w.name
			ELSE COALESCE(ch.name, '') END),
		c.color, c.ngac_oa_id, c.created_at,
		COALESCE(otu.ngac_node_id, ''), COALESCE(NULLIF(ou.display_name, ''), ou.username, ''), COALESCE(ou.avatar_url, ''), COALESCE(otu.user_id, '')`

const calendarFrom = `
	FROM calendars c
	JOIN workspaces w ON w.id = c.workspace_id
	LEFT JOIN channels ch ON ch.id = c.channel_id
	LEFT JOIN tenant_users otu ON c.kind = 'personal' AND otu.tenant_id = c.workspace_id
		AND otu.ngac_node_id = c.owner_node_id AND otu.status = 'active'
	LEFT JOIN users ou ON ou.id = otu.user_id`

const calendarSelect = `SELECT ` + calendarColumns + calendarFrom

func scanCalendar(row interface{ Scan(...any) error }) (*Calendar, error) {
	c := &Calendar{}
	var p Person
	if err := row.Scan(&c.ID, &c.WorkspaceID, &c.Kind, &c.OwnerNodeID, &c.ChannelID,
		&c.Name, &c.Color, &c.NgacOAID, &c.CreatedAt,
		&p.NodeID, &p.DisplayName, &p.AvatarURL, &p.UserID); err != nil {
		return nil, err
	}
	if p.NodeID != "" {
		c.Owner = &p
	}
	return c, nil
}

// CalendarByID reads one calendar. One that does not exist is ErrNotFound.
func (s *Store) CalendarByID(ctx context.Context, id string) (*Calendar, error) {
	c, err := scanCalendar(s.pool.QueryRow(ctx, calendarSelect+` WHERE c.id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("calendar: %w", ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("calendar: %w", err)
	}
	return c, nil
}

// CalendarFor finds the one calendar of a kind: a person's personal calendar
// (ownerNodeID), the workspace's, or a conversation's (channelID). Not there yet
// is ErrNotFound.
func (s *Store) CalendarFor(ctx context.Context, workspaceID, kind, ownerNodeID, channelID string) (*Calendar, error) {
	var where string
	var args []any
	switch kind {
	case KindPersonal:
		where, args = `c.kind = 'personal' AND c.workspace_id = $1 AND c.owner_node_id = $2`, []any{workspaceID, ownerNodeID}
	case KindWorkspace:
		where, args = `c.kind = 'workspace' AND c.workspace_id = $1`, []any{workspaceID}
	case KindChannel:
		where, args = `c.kind = 'channel' AND c.workspace_id = $1 AND c.channel_id = $2`, []any{workspaceID, channelID}
	default:
		return nil, fmt.Errorf("calendar of kind %q", kind)
	}
	c, err := scanCalendar(s.pool.QueryRow(ctx, calendarSelect+` WHERE `+where, args...))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("calendar: %w", ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("calendar: %w", err)
	}
	return c, nil
}

// InsertCalendar stores a calendar. A unique index that already holds one of the
// same kind and owner answers ErrAlreadyExists: the creation was raced, and the
// caller reads the winner.
func (s *Store) InsertCalendar(ctx context.Context, c *Calendar) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO calendars (id, workspace_id, kind, owner_node_id, channel_id, name, color, ngac_oa_id)
		 VALUES ($1, $2, $3, NULLIF($4, ''), NULLIF($5, ''), $6, $7, $8)`,
		c.ID, c.WorkspaceID, c.Kind, c.OwnerNodeID, c.ChannelID, c.Name, c.Color, c.NgacOAID)
	if isUniqueViolation(err) {
		return fmt.Errorf("insert calendar: %w", ErrAlreadyExists)
	}
	if err != nil {
		return fmt.Errorf("insert calendar: %w", err)
	}
	return nil
}

// MemberChannelCalendars lists the group calendars that exist for the
// conversations the person is recorded as a member of. The membership table is a
// cache: the caller still checks each calendar with the policy service.
func (s *Store) MemberChannelCalendars(ctx context.Context, workspaceID, userNodeID string) ([]*Calendar, error) {
	rows, err := s.pool.Query(ctx,
		calendarSelect+` JOIN channel_members cm ON cm.channel_id = c.channel_id
		 WHERE c.kind = 'channel' AND c.workspace_id = $1 AND cm.ngac_node_id = $2 AND ch.channel_type <> 'dm'
		 ORDER BY c.name, c.id`, workspaceID, userNodeID)
	if err != nil {
		return nil, fmt.Errorf("member channel calendars: %w", err)
	}
	return collectCalendars(rows)
}

func collectCalendars(rows pgx.Rows) ([]*Calendar, error) {
	defer rows.Close()
	var out []*Calendar
	for rows.Next() {
		c, err := scanCalendar(rows)
		if err != nil {
			return nil, fmt.Errorf("scan calendar: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// Subscription is a person's own settings for a calendar on their list.
type Subscription struct {
	Calendar *Calendar
	// Color overrides the calendar's own when set.
	Color    *int16
	Visible  bool
	Position int
}

// Subscriptions lists the calendars the person has settings for in the workspace.
func (s *Store) Subscriptions(ctx context.Context, workspaceID, userNodeID string) ([]*Subscription, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT sub.color, sub.visible, sub.position, `+calendarColumns+calendarFrom+`
		 JOIN calendar_subscriptions sub ON sub.calendar_id = c.id
		 WHERE c.workspace_id = $1 AND sub.user_node_id = $2
		 ORDER BY sub.position, c.name, c.id`, workspaceID, userNodeID)
	if err != nil {
		return nil, fmt.Errorf("subscriptions: %w", err)
	}
	defer rows.Close()
	var out []*Subscription
	for rows.Next() {
		sub := &Subscription{Calendar: &Calendar{}}
		var p Person
		c := sub.Calendar
		if err := rows.Scan(&sub.Color, &sub.Visible, &sub.Position,
			&c.ID, &c.WorkspaceID, &c.Kind, &c.OwnerNodeID, &c.ChannelID,
			&c.Name, &c.Color, &c.NgacOAID, &c.CreatedAt,
			&p.NodeID, &p.DisplayName, &p.AvatarURL, &p.UserID); err != nil {
			return nil, fmt.Errorf("scan subscription: %w", err)
		}
		if p.NodeID != "" {
			c.Owner = &p
		}
		out = append(out, sub)
	}
	return out, rows.Err()
}

// SubscriptionOf reads one person's settings for one calendar; none is ErrNotFound.
func (s *Store) SubscriptionOf(ctx context.Context, userNodeID, calendarID string) (*Subscription, error) {
	sub := &Subscription{}
	err := s.pool.QueryRow(ctx,
		`SELECT color, visible, position FROM calendar_subscriptions WHERE user_node_id = $1 AND calendar_id = $2`,
		userNodeID, calendarID).Scan(&sub.Color, &sub.Visible, &sub.Position)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("subscription: %w", ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("subscription: %w", err)
	}
	return sub, nil
}

// SaveSubscription creates or changes the person's settings for a calendar. A nil
// field keeps what is there (a new row starts visible, with the calendar's own
// colour).
func (s *Store) SaveSubscription(ctx context.Context, userNodeID, calendarID string, color *int16, visible *bool) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO calendar_subscriptions (user_node_id, calendar_id, color, visible)
		 VALUES ($1, $2, $3::smallint, COALESCE($4::boolean, TRUE))
		 ON CONFLICT (user_node_id, calendar_id) DO UPDATE
		   SET color = COALESCE($3::smallint, calendar_subscriptions.color),
		       visible = COALESCE($4::boolean, calendar_subscriptions.visible)`,
		userNodeID, calendarID, color, visible)
	if err != nil {
		return fmt.Errorf("save subscription: %w", err)
	}
	return nil
}

// DeleteSubscription removes the person's settings for a calendar and reports
// whether there were any.
func (s *Store) DeleteSubscription(ctx context.Context, userNodeID, calendarID string) (bool, error) {
	tag, err := s.pool.Exec(ctx,
		`DELETE FROM calendar_subscriptions WHERE user_node_id = $1 AND calendar_id = $2`, userNodeID, calendarID)
	if err != nil {
		return false, fmt.Errorf("delete subscription: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

// Share is the display record of one share of a personal calendar. The grant
// itself is the NGAC association; this row only lets a screen list it.
type Share struct {
	CalendarID    string
	GranteeNodeID string
	Level         string
	GrantedBy     string
	CreatedAt     time.Time
}

// Shares lists a calendar's shares, oldest first.
func (s *Store) Shares(ctx context.Context, calendarID string) ([]*Share, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT calendar_id, grantee_node_id, level, granted_by, created_at
		   FROM calendar_shares WHERE calendar_id = $1 ORDER BY created_at, grantee_node_id`, calendarID)
	if err != nil {
		return nil, fmt.Errorf("shares: %w", err)
	}
	defer rows.Close()
	var out []*Share
	for rows.Next() {
		sh := &Share{}
		if err := rows.Scan(&sh.CalendarID, &sh.GranteeNodeID, &sh.Level, &sh.GrantedBy, &sh.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan share: %w", err)
		}
		out = append(out, sh)
	}
	return out, rows.Err()
}

// ShareOf reads one share; none is ErrNotFound.
func (s *Store) ShareOf(ctx context.Context, calendarID, granteeNodeID string) (*Share, error) {
	sh := &Share{}
	err := s.pool.QueryRow(ctx,
		`SELECT calendar_id, grantee_node_id, level, granted_by, created_at
		   FROM calendar_shares WHERE calendar_id = $1 AND grantee_node_id = $2`, calendarID, granteeNodeID,
	).Scan(&sh.CalendarID, &sh.GranteeNodeID, &sh.Level, &sh.GrantedBy, &sh.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("share: %w", ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("share: %w", err)
	}
	return sh, nil
}

// SaveShare records a share, replacing the level of one that exists.
func (s *Store) SaveShare(ctx context.Context, sh *Share) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO calendar_shares (calendar_id, grantee_node_id, level, granted_by)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (calendar_id, grantee_node_id) DO UPDATE SET level = EXCLUDED.level, granted_by = EXCLUDED.granted_by`,
		sh.CalendarID, sh.GranteeNodeID, sh.Level, sh.GrantedBy)
	if err != nil {
		return fmt.Errorf("save share: %w", err)
	}
	return nil
}

// WithShareLock runs fn while holding a lock on one share (calendar and grantee)
// that every other WithShareLock of the same share waits for, in this process or
// another. A share lives twice, as an association in the graph and as a record
// here; changing both under the lock keeps two concurrent changes from leaving
// the graph at one level and the record at the other.
func (s *Store) WithShareLock(ctx context.Context, calendarID, granteeNodeID string, fn func(context.Context) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("lock share: %w", err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, calendarID+"|"+granteeNodeID); err != nil {
		return fmt.Errorf("lock share: %w", err)
	}
	if err := fn(ctx); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("lock share: %w", err)
	}
	return nil
}

// DeleteShare removes a share record and reports whether there was one.
func (s *Store) DeleteShare(ctx context.Context, calendarID, granteeNodeID string) (bool, error) {
	tag, err := s.pool.Exec(ctx,
		`DELETE FROM calendar_shares WHERE calendar_id = $1 AND grantee_node_id = $2`, calendarID, granteeNodeID)
	if err != nil {
		return false, fmt.Errorf("delete share: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}
