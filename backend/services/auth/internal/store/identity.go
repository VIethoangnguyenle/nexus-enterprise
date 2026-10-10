package store

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// FindUserByIdentity resolves an external identity (provider + subject) to its
// linked user, or nil if the subject has never signed in. It also records the
// sign-in time on the identity.
func (s *Store) FindUserByIdentity(ctx context.Context, provider, subject string) (*User, error) {
	user, err := s.scanUser(s.db.QueryRow(ctx,
		`UPDATE user_identities ui SET last_login_at = NOW()
		 FROM users u
		 WHERE ui.provider = $1 AND ui.subject = $2 AND u.id = ui.user_id
		 RETURNING u.id, u.username, COALESCE(u.password,''), COALESCE(u.ngac_node,''), COALESCE(u.email,''),
		           COALESCE(u.union_id,''), COALESCE(u.display_name,''), COALESCE(u.phone,''),
		           u.email_verified_at IS NOT NULL, u.profile_completed_at IS NOT NULL,
		           COALESCE(u.title,''), COALESCE(u.location,''), COALESCE(u.avatar_url,'')`,
		provider, subject))
	if err != nil {
		return nil, fmt.Errorf("find user by identity: %w", err)
	}
	return user, nil
}

// GetIdentitySubject returns the subject a user is linked to at provider, or
// "" if the user has no identity there.
func (s *Store) GetIdentitySubject(ctx context.Context, provider, userID string) (string, error) {
	var subject string
	err := s.db.QueryRow(ctx,
		`SELECT subject FROM user_identities WHERE provider = $1 AND user_id = $2`,
		provider, userID).Scan(&subject)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("get identity subject: %w", err)
	}
	return subject, nil
}

// LinkIdentity binds provider+subject to a user. An existing binding for the
// subject is left untouched: a subject never moves between accounts.
func (s *Store) LinkIdentity(ctx context.Context, provider, subject, userID, email string) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO user_identities (provider, subject, user_id, email)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (provider, subject) DO NOTHING`,
		provider, subject, userID, email)
	if err != nil {
		return fmt.Errorf("link identity: %w", err)
	}
	return nil
}

// ClaimTenantDomain gives a tenant that has no domain yet ownership of domain,
// provided no other tenant already owns it, and marks it an organization.
// It reports whether the claim took effect.
func (s *Store) ClaimTenantDomain(ctx context.Context, tenantID, domain string) (bool, error) {
	domain = strings.ToLower(strings.TrimSpace(domain))
	if domain == "" {
		return false, nil
	}
	tag, err := s.db.Exec(ctx,
		`UPDATE workspaces SET domain = $2, type = 'organization'
		 WHERE id = $1 AND domain IS NULL
		   AND NOT EXISTS (SELECT 1 FROM workspaces o WHERE o.domain IS NOT NULL AND lower(o.domain) = $2)`,
		tenantID, domain)
	if err != nil {
		// Lost a race with a concurrent claim: the unique index decided.
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return false, nil
		}
		return false, fmt.Errorf("claim tenant domain: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

// ClearPassword removes a user's password, so password sign-in stops working
// for that account until a new one is set.
func (s *Store) ClearPassword(ctx context.Context, userID string) error {
	if _, err := s.db.Exec(ctx, `UPDATE users SET password = '' WHERE id = $1`, userID); err != nil {
		return fmt.Errorf("clear password: %w", err)
	}
	return nil
}
