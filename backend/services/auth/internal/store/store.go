package store

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
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

// ProfileChanges names the profile fields a person may write. A nil field is
// left as it is; a pointer to "" clears it. The department is not one of them:
// it is assigned by an administrator, per workspace. Nor is the avatar, until
// there is an image store a profile could point at.
type ProfileChanges struct {
	DisplayName *string
	Title       *string
	Location    *string
}

// UpdateProfile writes only the fields in ch and marks the profile as
// completed the first time anything is saved. It returns ErrNoSuchUser when
// the account does not exist.
func (s *Store) UpdateProfile(ctx context.Context, userID string, ch ProfileChanges) error {
	tag, err := s.db.Exec(ctx,
		`UPDATE users SET
		     display_name = COALESCE($2, display_name),
		     title        = COALESCE($3, title),
		     location     = COALESCE($4, location),
		     profile_completed_at = COALESCE(profile_completed_at, NOW())
		 WHERE id = $1`,
		userID, ch.DisplayName, ch.Title, ch.Location)
	if err != nil {
		return fmt.Errorf("update profile: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNoSuchUser
	}
	return nil
}

// WorkspaceSummary is one workspace a person belongs to, as the workspace
// picker shows it.
type WorkspaceSummary struct {
	ID          string
	Name        string
	Role        string
	MemberCount int
	// Domain is the company domain the workspace claimed (acme.com), when it
	// is a company's; empty for a personal or unclaimed one.
	Domain string
}

// ListWorkspaceSummaries returns the workspaces userID is an active member of,
// with that person's role and the number of active members. Oldest first.
func (s *Store) ListWorkspaceSummaries(ctx context.Context, userID string) ([]WorkspaceSummary, error) {
	rows, err := s.db.Query(ctx,
		`SELECT w.id, w.name, mine.role,
		        (SELECT count(*) FROM tenant_users m WHERE m.tenant_id = w.id AND m.status = 'active'),
		        COALESCE(w.domain, '')
		   FROM tenant_users mine
		   JOIN workspaces w ON w.id = mine.tenant_id
		  WHERE mine.user_id = $1 AND mine.status = 'active'
		  ORDER BY mine.joined_at, w.id`, userID)
	if err != nil {
		return nil, fmt.Errorf("list workspace summaries: %w", err)
	}
	defer rows.Close()
	var out []WorkspaceSummary
	for rows.Next() {
		var w WorkspaceSummary
		if err := rows.Scan(&w.ID, &w.Name, &w.Role, &w.MemberCount, &w.Domain); err != nil {
			return nil, fmt.Errorf("scan workspace summary: %w", err)
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

// ContactCursor is where the next page of the directory starts: the sort key
// and ID of the last person on the page before. Keyset paging, so a person who
// joins or leaves between two requests moves nothing that was already read.
type ContactCursor struct {
	Key string `json:"k"`
	ID  string `json:"i"`
}

// ContactFilter narrows and pages the workspace directory. Zero values mean
// "no filter"; Limit is required.
type ContactFilter struct {
	Department string // an administrator-assigned department name, exact
	Location   string
	Search     string // case-insensitive substring of name, title, department or email
	Limit      int
	After      *ContactCursor // nil for the first page
}

// likeEscaper makes a person's text literal inside ILIKE.
var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

// ListContactsByWorkspace returns one page of the workspace's active members,
// the true number of people the filter matches (not the page's length), and the
// cursor of the next page, nil on the last.
//
// The department is the one an administrator assigned in this workspace
// (tenant_users.department_id), never what the person wrote about themselves.
// A display name is sent only once the person has been through the profile step:
// before it, the stored name is the handle derived from their address, and a
// handle is not a name.
func (s *Store) ListContactsByWorkspace(ctx context.Context, workspaceID string, f ContactFilter) ([]User, int, *ContactCursor, error) {
	from := ` FROM users u
		 JOIN tenant_users tu ON tu.user_id = u.id AND tu.tenant_id = $1 AND tu.status = 'active'
		 LEFT JOIN departments d ON d.id = tu.department_id AND d.workspace_id = tu.tenant_id`
	name := `CASE WHEN u.profile_completed_at IS NOT NULL THEN COALESCE(u.display_name,'') ELSE '' END`
	where := ""
	args := []any{workspaceID}
	add := func(cond string, vs ...any) {
		for _, v := range vs {
			args = append(args, v)
			cond = strings.Replace(cond, "?", fmt.Sprintf("$%d", len(args)), 1)
		}
		where += " AND " + cond
	}
	if f.Department != "" {
		add("d.name = ?", f.Department)
	}
	if f.Location != "" {
		add("u.location = ?", f.Location)
	}
	if f.Search != "" {
		like := "%" + likeEscaper.Replace(f.Search) + "%"
		add(`(`+name+` ILIKE ? ESCAPE '\' OR COALESCE(u.title,'') ILIKE ? ESCAPE '\'
			OR COALESCE(d.name,'') ILIKE ? ESCAPE '\' OR COALESCE(u.email,'') ILIKE ? ESCAPE '\')`, like, like, like, like)
	}

	// The total is of everyone the filter matches, wherever the page starts.
	var total int
	if err := s.db.QueryRow(ctx, "SELECT count(*)"+from+" WHERE true"+where, args...).Scan(&total); err != nil {
		return nil, 0, nil, fmt.Errorf("count contacts: %w", err)
	}

	if f.After != nil {
		add("(lower("+name+"), u.id) > (?, ?)", f.After.Key, f.After.ID)
	}
	limit := f.Limit
	if limit <= 0 {
		limit = 50
	}
	args = append(args, limit+1)
	query := `SELECT u.id, u.username, COALESCE(u.ngac_node,''), COALESCE(u.email,''),
			COALESCE(u.union_id,''), ` + name + `, COALESCE(u.phone,''),
			COALESCE(u.title,''), COALESCE(d.name,''), COALESCE(u.location,''), COALESCE(u.avatar_url,''),
			lower(` + name + `)` +
		from + " WHERE true" + where +
		fmt.Sprintf(" ORDER BY lower(%s), u.id LIMIT $%d", name, len(args))
	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, nil, fmt.Errorf("list contacts: %w", err)
	}
	defer rows.Close()

	var contacts []User
	var keys []string
	for rows.Next() {
		var u User
		var key string
		if err := rows.Scan(&u.ID, &u.Username, &u.NGACNodeID, &u.Email,
			&u.UnionID, &u.DisplayName, &u.Phone,
			&u.Title, &u.Department, &u.Location, &u.AvatarURL, &key); err != nil {
			return nil, 0, nil, fmt.Errorf("scan contact: %w", err)
		}
		contacts = append(contacts, u)
		keys = append(keys, key)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, nil, err
	}
	var next *ContactCursor
	if len(contacts) > limit {
		contacts = contacts[:limit]
		next = &ContactCursor{Key: keys[limit-1], ID: contacts[limit-1].ID}
	}
	return contacts, total, next, nil
}

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
