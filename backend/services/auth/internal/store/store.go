package store

import (
	"context"
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
}

// Tenant represents a workspace used as a tenant.
type Tenant struct {
	ID     string
	Name   string
	Domain string
}

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

// GetUserByUsername looks up a user by username.
func (s *Store) GetUserByUsername(ctx context.Context, username string) (*User, error) {
	return s.scanUser(s.db.QueryRow(ctx,
		`SELECT id, username, COALESCE(password,''), COALESCE(ngac_node,''), COALESCE(email,''), COALESCE(union_id,''), COALESCE(display_name,''), COALESCE(phone,'')
		 FROM users WHERE username = $1`, username))
}

// GetUserByEmail looks up a user by email address.
func (s *Store) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	return s.scanUser(s.db.QueryRow(ctx,
		`SELECT id, username, COALESCE(password,''), COALESCE(ngac_node,''), COALESCE(email,''), COALESCE(union_id,''), COALESCE(display_name,''), COALESCE(phone,'')
		 FROM users WHERE email = $1`, email))
}

// GetUserByPhone looks up a user by phone number.
func (s *Store) GetUserByPhone(ctx context.Context, phone string) (*User, error) {
	return s.scanUser(s.db.QueryRow(ctx,
		`SELECT id, username, COALESCE(password,''), COALESCE(ngac_node,''), COALESCE(email,''), COALESCE(union_id,''), COALESCE(display_name,''), COALESCE(phone,'')
		 FROM users WHERE phone = $1`, phone))
}

// GetUserByID looks up a user by primary key.
func (s *Store) GetUserByID(ctx context.Context, userID string) (*User, error) {
	return s.scanUser(s.db.QueryRow(ctx,
		`SELECT id, username, COALESCE(password,''), COALESCE(ngac_node,''), COALESCE(email,''), COALESCE(union_id,''), COALESCE(display_name,''), COALESCE(phone,'')
		 FROM users WHERE id = $1`, userID))
}

// GetUserByNGACNodeID looks up a user by their NGAC graph node ID.
func (s *Store) GetUserByNGACNodeID(ctx context.Context, ngacNodeID string) (*User, error) {
	return s.scanUser(s.db.QueryRow(ctx,
		`SELECT id, username, COALESCE(password,''), COALESCE(ngac_node,''), COALESCE(email,''), COALESCE(union_id,''), COALESCE(display_name,''), COALESCE(phone,'')
		 FROM users WHERE ngac_node = $1`, ngacNodeID))
}

// ListUsers returns all users without passwords.
func (s *Store) ListUsers(ctx context.Context) ([]User, error) {
	rows, err := s.db.Query(ctx,
		`SELECT id, username, COALESCE(ngac_node,''), COALESCE(email,''), COALESCE(union_id,''), COALESCE(display_name,''), COALESCE(phone,'')
		 FROM users ORDER BY username`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Username, &u.NGACNodeID, &u.Email, &u.UnionID, &u.DisplayName, &u.Phone); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, nil
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

// GetTenantUser retrieves a specific tenant membership.
func (s *Store) GetTenantUser(ctx context.Context, tenantID, userID string) (*TenantMembership, error) {
	var m TenantMembership
	err := s.db.QueryRow(ctx,
		`SELECT tu.tenant_id, w.name, tu.user_id, tu.role, tu.status, tu.open_id, COALESCE(tu.ngac_node_id,'')
		 FROM tenant_users tu
		 JOIN workspaces w ON w.id = tu.tenant_id
		 WHERE tu.tenant_id = $1 AND tu.user_id = $2`,
		tenantID, userID).Scan(&m.TenantID, &m.TenantName, &m.UserID, &m.Role, &m.Status, &m.OpenID, &m.NGACNodeID)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get tenant user: %w", err)
	}
	return &m, nil
}

// FindTenantByDomain finds a workspace/tenant by email domain.
func (s *Store) FindTenantByDomain(ctx context.Context, domain string) (*Tenant, error) {
	var t Tenant
	err := s.db.QueryRow(ctx,
		`SELECT id, name, COALESCE(domain,'') FROM workspaces WHERE domain = $1 LIMIT 1`,
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
	err := row.Scan(&u.ID, &u.Username, &u.Password, &u.NGACNodeID, &u.Email, &u.UnionID, &u.DisplayName, &u.Phone)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan user: %w", err)
	}
	return &u, nil
}

// UpdateProfile updates user profile fields (title, department, location, avatar_url, display_name).
func (s *Store) UpdateProfile(ctx context.Context, userID, displayName, title, department, location, avatarURL string) error {
	_, err := s.db.Exec(ctx,
		`UPDATE users SET display_name = $2, title = $3, department = $4, location = $5, avatar_url = $6
		 WHERE id = $1`,
		userID, displayName, title, department, location, avatarURL)
	if err != nil {
		return fmt.Errorf("update profile: %w", err)
	}
	return nil
}

// ListContactsByWorkspace returns enriched user profiles for workspace members.
func (s *Store) ListContactsByWorkspace(ctx context.Context, workspaceID, departmentFilter, locationFilter string) ([]User, error) {
	query := `SELECT u.id, u.username, COALESCE(u.ngac_node,''), COALESCE(u.email,''),
			COALESCE(u.union_id,''), COALESCE(u.display_name,''), COALESCE(u.phone,''),
			COALESCE(u.title,''), COALESCE(u.department,''), COALESCE(u.location,''), COALESCE(u.avatar_url,'')
		 FROM users u
		 JOIN tenant_users tu ON tu.user_id = u.id
		 WHERE tu.tenant_id = $1 AND tu.status = 'active'`

	args := []any{workspaceID}
	argIdx := 2

	if departmentFilter != "" {
		query += fmt.Sprintf(" AND u.department = $%d", argIdx)
		args = append(args, departmentFilter)
		argIdx++
	}
	if locationFilter != "" {
		query += fmt.Sprintf(" AND u.location = $%d", argIdx)
		args = append(args, locationFilter)
	}

	query += " ORDER BY u.display_name, u.username LIMIT 200"

	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list contacts: %w", err)
	}
	defer rows.Close()

	var contacts []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Username, &u.NGACNodeID, &u.Email,
			&u.UnionID, &u.DisplayName, &u.Phone,
			&u.Title, &u.Department, &u.Location, &u.AvatarURL); err != nil {
			return nil, fmt.Errorf("scan contact: %w", err)
		}
		contacts = append(contacts, u)
	}
	return contacts, nil
}
