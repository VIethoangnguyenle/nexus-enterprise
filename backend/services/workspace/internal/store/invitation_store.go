package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// Invitation statuses.
const (
	InvitationPending  = "pending"
	InvitationAccepted = "accepted"
	InvitationDeclined = "declined"
	InvitationRevoked  = "revoked"
)

// Invitation is a standing offer to join a workspace, addressed to an email.
type Invitation struct {
	ID           string
	WorkspaceID  string
	Email        string
	RoleID       string // a role's node ID; empty for none
	DepartmentID string // empty for none
	InvitedBy    string // the inviter's user node
	Status       string
	CreatedAt    time.Time
	ExpiresAt    time.Time
}

const invitationColumns = `id, workspace_id, email, COALESCE(role_id,''), COALESCE(department_id,''),
	invited_by, status, created_at, expires_at`

func scanInvitation(row pgx.Row) (*Invitation, error) {
	var i Invitation
	if err := row.Scan(&i.ID, &i.WorkspaceID, &i.Email, &i.RoleID, &i.DepartmentID, &i.InvitedBy, &i.Status, &i.CreatedAt, &i.ExpiresAt); err != nil {
		return nil, err
	}
	return &i, nil
}

// UpsertInvitation records a pending invitation, or refreshes the one already
// pending for this address in this workspace (new inviter, role, department and
// expiry), so inviting twice is one offer.
func (s *Store) UpsertInvitation(ctx context.Context, inv *Invitation) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO workspace_invitations (workspace_id, email, role_id, department_id, invited_by, status, expires_at)
		 VALUES ($1, $2, NULLIF($3,''), NULLIF($4,''), $5, 'pending', $6)
		 ON CONFLICT (workspace_id, email) WHERE status = 'pending'
		 DO UPDATE SET role_id = EXCLUDED.role_id, department_id = EXCLUDED.department_id,
		               invited_by = EXCLUDED.invited_by, created_at = NOW(), expires_at = EXCLUDED.expires_at`,
		inv.WorkspaceID, inv.Email, inv.RoleID, inv.DepartmentID, inv.InvitedBy, inv.ExpiresAt)
	if err != nil {
		return fmt.Errorf("upsert invitation: %w", err)
	}
	return nil
}

// GetInvitation returns one invitation by ID, or nil.
func (s *Store) GetInvitation(ctx context.Context, id string) (*Invitation, error) {
	i, err := scanInvitation(s.db.QueryRow(ctx, `SELECT `+invitationColumns+` FROM workspace_invitations WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get invitation: %w", err)
	}
	return i, nil
}

func (s *Store) listInvitations(ctx context.Context, where string, arg any, now time.Time) ([]*Invitation, error) {
	rows, err := s.db.Query(ctx,
		`SELECT `+invitationColumns+` FROM workspace_invitations
		  WHERE `+where+` AND status = 'pending' AND expires_at > $2 ORDER BY created_at DESC`, arg, now)
	if err != nil {
		return nil, fmt.Errorf("list invitations: %w", err)
	}
	defer rows.Close()
	var out []*Invitation
	for rows.Next() {
		i, err := scanInvitation(rows)
		if err != nil {
			return nil, fmt.Errorf("scan invitation: %w", err)
		}
		out = append(out, i)
	}
	return out, rows.Err()
}

// ListPendingForWorkspace returns the offers still open in a workspace, newest first.
func (s *Store) ListPendingForWorkspace(ctx context.Context, wsID string, now time.Time) ([]*Invitation, error) {
	return s.listInvitations(ctx, "workspace_id = $1", wsID, now)
}

// ListPendingForEmail returns the offers still open to an address, newest first.
func (s *Store) ListPendingForEmail(ctx context.Context, email string, now time.Time) ([]*Invitation, error) {
	return s.listInvitations(ctx, "email = $1", email, now)
}

// RespondInvitation moves a pending, unexpired invitation addressed to email to
// status (accepted or declined) and returns it; nil when there is no such open
// invitation. Being one conditional update, two answers at once cannot both win.
func (s *Store) RespondInvitation(ctx context.Context, id, email, status string, now time.Time) (*Invitation, error) {
	i, err := scanInvitation(s.db.QueryRow(ctx,
		`UPDATE workspace_invitations SET status = $3, responded_at = $4
		  WHERE id = $1 AND email = $2 AND status = 'pending' AND expires_at > $4
		  RETURNING `+invitationColumns, id, email, status, now))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("respond to invitation: %w", err)
	}
	return i, nil
}

// ReopenInvitation puts an accepted invitation back to pending, when accepting
// it could not be carried out.
func (s *Store) ReopenInvitation(ctx context.Context, id string) error {
	if _, err := s.db.Exec(ctx,
		`UPDATE workspace_invitations SET status = 'pending', responded_at = NULL WHERE id = $1 AND status = 'accepted'`, id); err != nil {
		return fmt.Errorf("reopen invitation: %w", err)
	}
	return nil
}

// VerifiedEmailByNode returns the proved address of the account behind a user
// node, lower-cased; empty if there is none or it is unproved.
func (s *Store) VerifiedEmailByNode(ctx context.Context, nodeID string) (string, error) {
	var email string
	err := s.db.QueryRow(ctx,
		`SELECT CASE WHEN email_verified_at IS NOT NULL THEN lower(COALESCE(email,'')) ELSE '' END FROM users WHERE ngac_node = $1`, nodeID).Scan(&email)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("verified email by node: %w", err)
	}
	return email, nil
}

// RevokePendingForEmail withdraws every pending invitation to an address in a
// workspace (the person has been removed from it, or is no longer wanted), and
// returns how many.
func (s *Store) RevokePendingForEmail(ctx context.Context, wsID, email string, now time.Time) (int, error) {
	tag, err := s.db.Exec(ctx,
		`UPDATE workspace_invitations SET status = 'revoked', responded_at = $3
		  WHERE workspace_id = $1 AND email = $2 AND status = 'pending'`, wsID, email, now)
	if err != nil {
		return 0, fmt.Errorf("revoke pending invitations: %w", err)
	}
	return int(tag.RowsAffected()), nil
}

// RevokeInvitation withdraws a pending invitation of a workspace; false when
// there is no such open invitation in that workspace.
func (s *Store) RevokeInvitation(ctx context.Context, wsID, id string, now time.Time) (bool, error) {
	tag, err := s.db.Exec(ctx,
		`UPDATE workspace_invitations SET status = 'revoked', responded_at = $3
		  WHERE id = $1 AND workspace_id = $2 AND status = 'pending'`, id, wsID, now)
	if err != nil {
		return false, fmt.Errorf("revoke invitation: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

// UserEmail returns the address on a user's account, lower-cased, only if its
// owner has proved it (users.email_verified_at). An address typed at signup is
// a claim anyone can make, so it matches no invitation: empty if none or unproved.
func (s *Store) UserEmail(ctx context.Context, userID string) (string, error) {
	var email string
	err := s.db.QueryRow(ctx, `SELECT CASE WHEN email_verified_at IS NOT NULL THEN lower(COALESCE(email,'')) ELSE '' END FROM users WHERE id = $1`, userID).Scan(&email)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("user email: %w", err)
	}
	return email, nil
}
