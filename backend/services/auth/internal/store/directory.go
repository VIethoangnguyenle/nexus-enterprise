package store

import (
	"context"
	"fmt"
	"strings"
)

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
