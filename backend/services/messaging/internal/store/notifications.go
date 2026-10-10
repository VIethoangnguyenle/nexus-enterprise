package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// Notification is a row of the notifications table.
//
// It records what happened, not how to say it: the screen words each type from
// these fields. ActorName and TargetName are what the person and the subject were
// called in the workspace at the time; empty means unknown, never an id.
type Notification struct {
	ID          string
	UserID      string
	WorkspaceID string
	Type        string
	ActorUserID string
	ActorName   string
	TargetType  string
	TargetID    string
	TargetName  string
	Params      map[string]string
	Read        bool
	CreatedAt   time.Time
}

// TypeWorkspaceInvitation is the notification that tells someone they were
// invited to a workspace. It is personal: the invitee is not yet a member, so it
// is listed, counted and marked whichever workspace they have open, while every
// other type belongs to one workspace.
const TypeWorkspaceInvitation = "workspace_invitation"

// inScope is the SQL condition for "this user's notifications visible from the
// workspace in $2": that workspace's, plus the personal ones.
const inScope = `user_id = $1 AND (workspace_id = $2 OR type = '` + TypeWorkspaceInvitation + `')`

// Member is a person as one workspace knows them.
type Member struct {
	UserID string
	// Name is the display name, else the login. Never empty for a member.
	Name string
}

// InsertNotification stores a notification. Empty optional fields are stored as
// NULL (or the column default). The workspace is required: a notification that
// belongs to no workspace could never be shown.
func (s *Store) InsertNotification(ctx context.Context, n *Notification) error {
	if n.WorkspaceID == "" {
		return errors.New("insert notification: a workspace is required")
	}
	params := n.Params
	if params == nil {
		params = map[string]string{}
	}
	raw, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("insert notification: %w", err)
	}
	_, err = s.db.Exec(ctx,
		`INSERT INTO notifications
		   (id, user_id, workspace_id, type, actor_user_id, actor_name, target_type, target_id, target_name, params, created_at)
		 VALUES ($1, $2, $3, $4, NULLIF($5, ''), $6, NULLIF($7, ''), NULLIF($8, ''), $9, $10, $11)`,
		n.ID, n.UserID, n.WorkspaceID, n.Type, n.ActorUserID, n.ActorName, n.TargetType, n.TargetID, n.TargetName, raw, n.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert notification: %w", err)
	}
	return nil
}

// ListNotifications returns one user's notifications in one workspace, newest
// first. Rows with no workspace (from before notifications were scoped) match
// no workspace and are never listed.
func (s *Store) ListNotifications(ctx context.Context, userID, workspaceID string, limit, offset int) ([]*Notification, error) {
	rows, err := s.db.Query(ctx,
		`SELECT id, user_id, workspace_id, type, COALESCE(actor_user_id,''), actor_name,
		        COALESCE(target_type,''), COALESCE(target_id,''), target_name, params, read, created_at
		 FROM notifications WHERE `+inScope+`
		 ORDER BY created_at DESC, id DESC LIMIT $3 OFFSET $4`,
		userID, workspaceID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list notifications: %w", err)
	}
	defer rows.Close()

	var out []*Notification
	for rows.Next() {
		var n Notification
		var raw []byte
		if err := rows.Scan(&n.ID, &n.UserID, &n.WorkspaceID, &n.Type, &n.ActorUserID, &n.ActorName,
			&n.TargetType, &n.TargetID, &n.TargetName, &raw, &n.Read, &n.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan notification: %w", err)
		}
		if err := json.Unmarshal(raw, &n.Params); err != nil {
			return nil, fmt.Errorf("decode notification params: %w", err)
		}
		out = append(out, &n)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list notifications: %w", err)
	}
	return out, nil
}

// NotificationCounts returns how many notifications a user has in one workspace
// and how many of them are unread.
func (s *Store) NotificationCounts(ctx context.Context, userID, workspaceID string) (total, unread int, err error) {
	err = s.db.QueryRow(ctx,
		`SELECT COUNT(*), COUNT(*) FILTER (WHERE read = FALSE)
		   FROM notifications WHERE `+inScope,
		userID, workspaceID).Scan(&total, &unread)
	if err != nil {
		return 0, 0, fmt.Errorf("count notifications: %w", err)
	}
	return total, unread, nil
}

// MarkNotificationRead marks one of the user's notifications in this workspace
// read. Another user's, or another workspace's, matches no row and nothing changes.
func (s *Store) MarkNotificationRead(ctx context.Context, id, userID, workspaceID string) error {
	if _, err := s.db.Exec(ctx,
		`UPDATE notifications SET read = TRUE
		  WHERE id = $1 AND user_id = $2 AND (workspace_id = $3 OR type = '`+TypeWorkspaceInvitation+`')`,
		id, userID, workspaceID); err != nil {
		return fmt.Errorf("mark notification read: %w", err)
	}
	return nil
}

// MarkAllNotificationsRead marks every unread notification of the user in this
// workspace read; their notifications in other workspaces stay unread.
func (s *Store) MarkAllNotificationsRead(ctx context.Context, userID, workspaceID string) error {
	if _, err := s.db.Exec(ctx,
		`UPDATE notifications SET read = TRUE WHERE `+inScope+` AND read = FALSE`,
		userID, workspaceID); err != nil {
		return fmt.Errorf("mark all notifications read: %w", err)
	}
	return nil
}

// FindMember resolves key to a member of the workspace. key is a user id or an
// NGAC user node id (an approval names people by node). Someone who is not a
// member of that workspace is not found, so one workspace never reads a name,
// or addresses a person, through another's.
func (s *Store) FindMember(ctx context.Context, workspaceID, key string) (Member, bool, error) {
	if workspaceID == "" || key == "" {
		return Member{}, false, nil
	}
	var m Member
	err := s.db.QueryRow(ctx,
		`SELECT u.id, COALESCE(NULLIF(u.display_name, ''), u.username)
		   FROM tenant_users tu JOIN users u ON u.id = tu.user_id
		  WHERE tu.tenant_id = $1 AND (u.id = $2 OR u.ngac_node = $2 OR tu.ngac_node_id = $2)
		  ORDER BY (u.id = $2) DESC LIMIT 1`,
		workspaceID, key).Scan(&m.UserID, &m.Name)
	if errors.Is(err, pgx.ErrNoRows) {
		return Member{}, false, nil
	}
	if err != nil {
		return Member{}, false, fmt.Errorf("find member: %w", err)
	}
	return m, true, nil
}

// Invitee is an account an open invitation reaches.
type Invitee struct {
	UserID        string
	WorkspaceID   string
	WorkspaceName string
	// InviterNode is the inviting member's NGAC user node.
	InviterNode string
}

// InviteesOf returns the accounts a pending, unexpired invitation reaches: those
// whose address matches it and is verified, the same rule that decides who sees
// it in "my invitations". An address with no account, or only an unverified one,
// reaches nobody and the result is empty.
func (s *Store) InviteesOf(ctx context.Context, invitationID string, now time.Time) ([]Invitee, error) {
	rows, err := s.db.Query(ctx,
		`SELECT u.id, i.workspace_id, w.name, i.invited_by
		   FROM workspace_invitations i
		   JOIN workspaces w ON w.id = i.workspace_id
		   JOIN users u ON lower(u.email) = i.email AND u.email_verified_at IS NOT NULL
		  WHERE i.id = $1 AND i.status = 'pending' AND i.expires_at > $2`,
		invitationID, now)
	if err != nil {
		return nil, fmt.Errorf("find invitees: %w", err)
	}
	defer rows.Close()
	var out []Invitee
	for rows.Next() {
		var v Invitee
		if err := rows.Scan(&v.UserID, &v.WorkspaceID, &v.WorkspaceName, &v.InviterNode); err != nil {
			return nil, fmt.Errorf("scan invitee: %w", err)
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// DeleteNotificationsAbout removes the user's notifications of one type about one
// subject, so a refreshed invitation replaces its earlier notice instead of
// stacking beside it.
func (s *Store) DeleteNotificationsAbout(ctx context.Context, userID, notifType, subjectID string) error {
	if _, err := s.db.Exec(ctx,
		`DELETE FROM notifications WHERE user_id = $1 AND type = $2 AND target_id = $3`,
		userID, notifType, subjectID); err != nil {
		return fmt.Errorf("delete notifications: %w", err)
	}
	return nil
}

// MarkNotificationsAboutRead marks the user's notifications about one subject read
// (those of the workspace, plus the personal ones). It answers an invitation: the
// notice is read once the offer is.
func (s *Store) MarkNotificationsAboutRead(ctx context.Context, userID, workspaceID, subjectType, subjectID string) error {
	if _, err := s.db.Exec(ctx,
		`UPDATE notifications SET read = TRUE
		  WHERE `+inScope+` AND target_type = $3 AND target_id = $4 AND read = FALSE`,
		userID, workspaceID, subjectType, subjectID); err != nil {
		return fmt.Errorf("mark notifications read: %w", err)
	}
	return nil
}
