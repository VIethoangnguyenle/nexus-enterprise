package store

import (
	"context"
	"fmt"
	"strings"
)

// ProfileChanges names the profile fields a person may write. A nil field is
// left as it is; a pointer to "" clears it. The department is not one of them:
// it is assigned by an administrator, per workspace. Nor is the avatar: it is
// written with the picture itself, by SetAvatar and ClearAvatar.
type ProfileChanges struct {
	DisplayName *string
	Title       *string
	Location    *string
	Language    *string
}

// UpdateProfile writes only the fields in ch and marks the profile as
// completed the first time a profile field (name, title, location) is saved.
// The language alone is a preference, not a profile: saving only it completes
// nothing. It returns ErrNoSuchUser when the account does not exist.
func (s *Store) UpdateProfile(ctx context.Context, userID string, ch ProfileChanges) error {
	// The personal workspaces this person owns that still carry their previous
	// name follow the new one, in the same statement: the one made at sign-up
	// is named after them (first after the handle sign-up gave them). One they
	// renamed, or an organization, keeps its name.
	var updated bool
	err := s.db.QueryRow(ctx,
		`WITH old AS (
		     -- The statement's snapshot: the name before this update. (No FOR UPDATE:
		     -- it would skip the row the UPDATE below changes in the same statement.)
		     SELECT display_name FROM users WHERE id = $1
		 ), upd AS (
		     UPDATE users SET
		         display_name = COALESCE($2, display_name),
		         title        = COALESCE($3, title),
		         location     = COALESCE($4, location),
		         language     = COALESCE($5, language),
		         profile_completed_at = CASE
		             WHEN $2::text IS NULL AND $3::text IS NULL AND $4::text IS NULL THEN profile_completed_at
		             ELSE COALESCE(profile_completed_at, NOW())
		         END
		     WHERE id = $1
		     RETURNING id
		 ), ws AS (
		     UPDATE workspaces w SET name = $2
		       FROM old
		      WHERE $2::text IS NOT NULL AND w.owner_id = $1 AND w.type = 'personal'
		        AND w.name = old.display_name AND old.display_name IS DISTINCT FROM $2
		 )
		 SELECT EXISTS (SELECT 1 FROM upd)`,
		userID, ch.DisplayName, ch.Title, ch.Location, ch.Language).Scan(&updated)
	if err != nil {
		return fmt.Errorf("update profile: %w", err)
	}
	if !updated {
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
	// Type is "personal" or "organization".
	Type string
}

// ListWorkspaceSummaries returns the workspaces userID is an active member of,
// with that person's role and the number of active members. Oldest first.
func (s *Store) ListWorkspaceSummaries(ctx context.Context, userID string) ([]WorkspaceSummary, error) {
	rows, err := s.db.Query(ctx,
		`SELECT w.id, w.name, mine.role,
		        (SELECT count(*) FROM tenant_users m WHERE m.tenant_id = w.id AND m.status = 'active'),
		        COALESCE(w.domain, ''), w.type
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
		if err := rows.Scan(&w.ID, &w.Name, &w.Role, &w.MemberCount, &w.Domain, &w.Type); err != nil {
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

// ContactScope is which people of the workspace the caller may be shown. The zero
// value shows the caller only themselves: a filter nobody scoped must not read as
// "everyone". All lifts the restriction; it is for a caller who may see everyone.
type ContactScope struct {
	All bool
	// CallerID is always visible to themselves.
	CallerID string
	// DepartmentIDs are the departments whose people are visible.
	DepartmentIDs []string
	// NoDepartment: people with no department are visible.
	NoDepartment bool
	// OwnerNodeIDs are the NGAC nodes of the owners; they are visible when OwnersVisible.
	OwnerNodeIDs  []string
	OwnersVisible bool
}

// ContactFilter narrows and pages the workspace directory. Zero values mean
// "no filter" except Scope, whose zero value is the narrowest; Limit is required.
type ContactFilter struct {
	Scope        ContactScope
	DepartmentID string // a department by id (tenant_users.department_id)
	Department   string // an administrator-assigned department name, exact
	Location     string
	Search       string // case-insensitive substring of name, title, department or email
	Limit        int
	After        *ContactCursor // nil for the first page
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
	args := []any{workspaceID}
	// The department is joined, so it is named, filtered by and searched, only where the
	// scope shows it: on an owner's row, or the caller's own, a department outside the
	// scope stays unnamed and cannot be confirmed by a filter or a search.
	deptJoin := ""
	if !f.Scope.All {
		visible := f.Scope.DepartmentIDs
		if visible == nil {
			visible = []string{}
		}
		args = append(args, visible, f.Scope.CallerID)
		deptJoin = ` AND (tu.department_id = ANY($2) OR u.id = $3)`
	}
	from := ` FROM users u
		 JOIN tenant_users tu ON tu.user_id = u.id AND tu.tenant_id = $1 AND tu.status = 'active'
		 LEFT JOIN departments d ON d.id = tu.department_id AND d.workspace_id = tu.tenant_id` + deptJoin
	name := `CASE WHEN u.profile_completed_at IS NOT NULL THEN COALESCE(u.display_name,'') ELSE '' END`
	where := ""
	add := func(cond string, vs ...any) {
		for _, v := range vs {
			args = append(args, v)
			cond = strings.Replace(cond, "?", fmt.Sprintf("$%d", len(args)), 1)
		}
		where += " AND " + cond
	}
	// Who the caller may see comes first and is not optional: every other condition
	// narrows what is left. The zero scope shows the caller themselves and no one else.
	if !f.Scope.All {
		departments := f.Scope.DepartmentIDs
		if departments == nil {
			departments = []string{}
		}
		owners := f.Scope.OwnerNodeIDs
		if owners == nil {
			owners = []string{}
		}
		add(`(tu.department_id = ANY(?) OR (tu.department_id IS NULL AND ?::boolean)
			OR (u.ngac_node = ANY(?) AND ?::boolean) OR u.id = ?)`,
			departments, f.Scope.NoDepartment, owners, f.Scope.OwnersVisible, f.Scope.CallerID)
	}
	if f.DepartmentID != "" {
		add("d.id = ?", f.DepartmentID)
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
			COALESCE(u.title,''), COALESCE(d.name,''), COALESCE(d.id,''), COALESCE(u.location,''), COALESCE(u.avatar_url,''),
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
			&u.Title, &u.Department, &u.DepartmentID, &u.Location, &u.AvatarURL, &key); err != nil {
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

// ResolvedPerson is what the name resolver knows of one person: a name and a picture.
// Never an address, a title or a department.
type ResolvedPerson struct {
	UserID, NGACNodeID, DisplayName, AvatarURL string
}

// ResolvePeopleByWorkspace returns the name and picture of the people asked for by user id
// or by NGAC node, who have a membership row in this workspace.
//
// This is a deliberate exception to "a reader of tenant_users filters status = 'active'":
// whoever the workspace ever held, whatever the status of the row (active, invited,
// disabled or removed), keeps a name where their work stays, in a chat, an approval, a
// document or a calendar. It lists nobody, and is bound to the ids the caller already met.
// A person of another workspace, or no one at all, is simply absent. The display name
// follows the directory's rule: empty until the profile was saved.
func (s *Store) ResolvePeopleByWorkspace(ctx context.Context, workspaceID string, userIDs, nodeIDs []string) ([]ResolvedPerson, error) {
	if len(userIDs) == 0 && len(nodeIDs) == 0 {
		return nil, nil
	}
	if userIDs == nil {
		userIDs = []string{}
	}
	if nodeIDs == nil {
		nodeIDs = []string{}
	}
	rows, err := s.db.Query(ctx,
		`SELECT u.id, COALESCE(u.ngac_node,''),
		        CASE WHEN u.profile_completed_at IS NOT NULL THEN COALESCE(u.display_name,'') ELSE '' END,
		        COALESCE(u.avatar_url,'')
		   FROM users u
		   JOIN tenant_users tu ON tu.user_id = u.id AND tu.tenant_id = $1
		  WHERE u.id = ANY($2) OR u.ngac_node = ANY($3)
		  ORDER BY u.id`, workspaceID, userIDs, nodeIDs)
	if err != nil {
		return nil, fmt.Errorf("resolve people: %w", err)
	}
	defer rows.Close()
	var out []ResolvedPerson
	for rows.Next() {
		var p ResolvedPerson
		if err := rows.Scan(&p.UserID, &p.NGACNodeID, &p.DisplayName, &p.AvatarURL); err != nil {
			return nil, fmt.Errorf("scan resolved person: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
