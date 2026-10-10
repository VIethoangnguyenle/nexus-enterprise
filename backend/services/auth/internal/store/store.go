package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// User represents a row in the users table.
type User struct {
	ID          string
	Username    string
	Password    string
	NGACNodeID  string
	Email       string
	UnionID     string
	DisplayName string
	Phone       string
	Title       string
	Department  string
	Location    string
	AvatarURL   string
	// EmailVerified: the owner of Email has proved it (Google, or a code a real
	// sender delivered to it). An unverified address is only a claim.
	EmailVerified bool
	// ProfileCompleted: the person has been asked for, and saved, their profile.
	ProfileCompleted bool
}

// TenantMembership represents a user's membership in a tenant.
type TenantMembership struct {
	TenantID   string
	TenantName string
	UserID     string
	Role       string
	Status     string
	OpenID     string
	NGACNodeID string
	// DepartmentName is the department an administrator placed the person in,
	// in this workspace. Empty when there is none. Filled by GetTenantUser only.
	DepartmentName string
}

// Tenant represents a workspace used as a tenant.
type Tenant struct {
	ID     string
	Name   string
	Domain string
}

// userColumns is the column list every single-user lookup selects, in the order
// scanUser reads it. One list, so a lookup cannot forget the account state.
const userColumns = `id, username, COALESCE(password,''), COALESCE(ngac_node,''), COALESCE(email,''), COALESCE(union_id,''),
	COALESCE(display_name,''), COALESCE(phone,''), email_verified_at IS NOT NULL, profile_completed_at IS NOT NULL,
	COALESCE(title,''), COALESCE(location,''), COALESCE(avatar_url,'')`

// ErrNoSuchUser is returned when a write names an account that does not exist.
var ErrNoSuchUser = errors.New("no such user")

// Store handles database operations for the auth service.
type Store struct {
	db *pgxpool.Pool
}

// New creates an auth store backed by PostgreSQL.
func New(db *pgxpool.Pool) *Store {
	return &Store{db: db}
}

// CreateUser inserts a new user with all identity fields.
// Empty email/phone are stored as NULL to avoid unique constraint violations.
func (s *Store) CreateUser(ctx context.Context, id, username, password, ngacNodeID, email, unionID, displayName, phone string) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO users (id, username, password, ngac_node, email, union_id, display_name, phone)
		 VALUES ($1, $2, $3, $4, NULLIF($5,''), $6, $7, NULLIF($8,''))`,
		id, username, password, ngacNodeID, email, unionID, displayName, phone)
	return err
}

// CreateUserWithVerifiedEmail is CreateUser for a person whose address has
// already been proved (a Google sign-in with email_verified): the row is born
// verified, in the same statement, so there is no moment — and no failure
// between two statements — in which the account exists unverified. With no
// address there is nothing to verify and it behaves as CreateUser.
func (s *Store) CreateUserWithVerifiedEmail(ctx context.Context, id, username, password, ngacNodeID, email, unionID, displayName, phone string) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO users (id, username, password, ngac_node, email, union_id, display_name, phone, email_verified_at)
		 VALUES ($1, $2, $3, $4, NULLIF($5,''), $6, $7, NULLIF($8,''), CASE WHEN $5 <> '' THEN NOW() END)`,
		id, username, password, ngacNodeID, email, unionID, displayName, phone)
	return err
}

// GetUserByUsername looks up a user by username.
func (s *Store) GetUserByUsername(ctx context.Context, username string) (*User, error) {
	return s.scanUser(s.db.QueryRow(ctx,
		`SELECT `+userColumns+`
		 FROM users WHERE username = $1`, username))
}

// GetUserByEmail looks up a user by email address.
func (s *Store) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	return s.scanUser(s.db.QueryRow(ctx,
		`SELECT `+userColumns+`
		 FROM users WHERE lower(email) = lower($1)`, email))
}

// MarkEmailVerified records that the owner of the account's address has proved
// it (a Google sign-in with email_verified, or a code delivered to the address).
// It reports whether this call set it: false when it was already set or the
// account has no address.
func (s *Store) MarkEmailVerified(ctx context.Context, userID string) (bool, error) {
	tag, err := s.db.Exec(ctx,
		`UPDATE users SET email_verified_at = NOW()
		  WHERE id = $1 AND email IS NOT NULL AND email_verified_at IS NULL`, userID)
	if err != nil {
		return false, fmt.Errorf("mark email verified: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

// GetUserByPhone looks up a user by phone number.
func (s *Store) GetUserByPhone(ctx context.Context, phone string) (*User, error) {
	return s.scanUser(s.db.QueryRow(ctx,
		`SELECT `+userColumns+`
		 FROM users WHERE phone = $1`, phone))
}

// GetUserByID looks up a user by primary key.
func (s *Store) GetUserByID(ctx context.Context, userID string) (*User, error) {
	return s.scanUser(s.db.QueryRow(ctx,
		`SELECT `+userColumns+`
		 FROM users WHERE id = $1`, userID))
}

// GetUserByNGACNodeID looks up a user by their NGAC graph node ID.
func (s *Store) GetUserByNGACNodeID(ctx context.Context, ngacNodeID string) (*User, error) {
	return s.scanUser(s.db.QueryRow(ctx,
		`SELECT `+userColumns+`
		 FROM users WHERE ngac_node = $1`, ngacNodeID))
}

// InsertTenantUser creates a tenant membership record.
func (s *Store) InsertTenantUser(ctx context.Context, tenantID, userID, role, status, ngacNodeID string) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO tenant_users (tenant_id, user_id, role, status, ngac_node_id)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (tenant_id, user_id) DO NOTHING`,
		tenantID, userID, role, status, ngacNodeID)
	return err
}

// ListTenantsByUser returns all tenants a user belongs to.
func (s *Store) ListTenantsByUser(ctx context.Context, userID string) ([]TenantMembership, error) {
	rows, err := s.db.Query(ctx,
		`SELECT tu.tenant_id, w.name, tu.user_id, tu.role, tu.status, tu.open_id, COALESCE(tu.ngac_node_id,'')
		 FROM tenant_users tu
		 JOIN workspaces w ON w.id = tu.tenant_id
		 WHERE tu.user_id = $1 AND tu.status = 'active'
		 ORDER BY tu.joined_at`, userID)
	if err != nil {
		return nil, fmt.Errorf("list tenants by user: %w", err)
	}
	defer rows.Close()
	var memberships []TenantMembership
	for rows.Next() {
		var m TenantMembership
		if err := rows.Scan(&m.TenantID, &m.TenantName, &m.UserID, &m.Role, &m.Status, &m.OpenID, &m.NGACNodeID); err != nil {
			return nil, fmt.Errorf("scan tenant membership: %w", err)
		}
		memberships = append(memberships, m)
	}
	return memberships, nil
}

// GetTenantUser retrieves a specific tenant membership, with the name of the
// department an administrator assigned (workspace-scoped; never the person's own
// claim).
func (s *Store) GetTenantUser(ctx context.Context, tenantID, userID string) (*TenantMembership, error) {
	var m TenantMembership
	err := s.db.QueryRow(ctx,
		`SELECT tu.tenant_id, w.name, tu.user_id, tu.role, tu.status, tu.open_id, COALESCE(tu.ngac_node_id,''), COALESCE(d.name,'')
		 FROM tenant_users tu
		 JOIN workspaces w ON w.id = tu.tenant_id
		 LEFT JOIN departments d ON d.id = tu.department_id AND d.workspace_id = tu.tenant_id
		 WHERE tu.tenant_id = $1 AND tu.user_id = $2`,
		tenantID, userID).Scan(&m.TenantID, &m.TenantName, &m.UserID, &m.Role, &m.Status, &m.OpenID, &m.NGACNodeID, &m.DepartmentName)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get tenant user: %w", err)
	}
	return &m, nil
}

// FindTenantByDomain finds a workspace/tenant by email domain.
// Domains are compared case-insensitively (DNS names are).
func (s *Store) FindTenantByDomain(ctx context.Context, domain string) (*Tenant, error) {
	var t Tenant
	err := s.db.QueryRow(ctx,
		`SELECT id, name, COALESCE(domain,'') FROM workspaces
		 WHERE domain IS NOT NULL AND lower(domain) = lower($1)
		 ORDER BY created_at LIMIT 1`,
		domain).Scan(&t.ID, &t.Name, &t.Domain)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find tenant by domain: %w", err)
	}
	return &t, nil
}

// scanUser is a shared row scanner for user queries.
func (s *Store) scanUser(row pgx.Row) (*User, error) {
	var u User
	err := row.Scan(&u.ID, &u.Username, &u.Password, &u.NGACNodeID, &u.Email, &u.UnionID, &u.DisplayName, &u.Phone,
		&u.EmailVerified, &u.ProfileCompleted, &u.Title, &u.Location, &u.AvatarURL)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan user: %w", err)
	}
	return &u, nil
}
