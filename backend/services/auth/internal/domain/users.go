package domain

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"ngac-platform/ngac"
	workspacepb "ngac-platform/proto/workspace"
	"ngac-platform/services/auth/internal/store"
)

// ContactInfo is a user enriched with profile data for the contacts directory.
type ContactInfo struct {
	UserID      string
	NGACNodeID  string
	Username    string
	DisplayName string
	Email       string
	Title       string
	Department  string
	Location    string
	AvatarURL   string
	// DepartmentID is the department assigned in this workspace ("" for none).
	DepartmentID string
	// Roles are the roles the caller may see this person hold.
	Roles []ContactRole
}

// ContactRole is one role a person holds. The owners group has Kind "owners" and no Name:
// the screen words it; custom roles have Kind "custom" and the name an administrator gave.
type ContactRole struct {
	ID   string
	Name string
	Kind string
}

// ContactPage is one page of the directory as the caller may see it.
type ContactPage struct {
	Contacts   []ContactInfo
	Total      int
	NextCursor string
	// Visibility is "limited" when the caller sees only part of the workspace, else "all".
	Visibility string
}

// ResolvedPerson is a name and a picture, nothing else.
type ResolvedPerson struct {
	UserID      string
	NGACNodeID  string
	DisplayName string
	AvatarURL   string
}

// GetUserByID retrieves a user by their primary key.
func (s *Service) GetUserByID(ctx context.Context, userID string) (*UserInfo, error) {
	user, err := s.store.GetUserByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get user by id: %w", err)
	}
	if user == nil {
		return nil, ErrNotFound
	}
	return userInfo(user), nil
}

// userInfo is the domain view of an account (no password).
func userInfo(u *store.User) *UserInfo {
	return &UserInfo{
		ID: u.ID, Username: u.Username, NGACNodeID: u.NGACNodeID,
		Email: u.Email, UnionID: u.UnionID, DisplayName: u.DisplayName,
		Title: u.Title, Location: u.Location, AvatarURL: u.AvatarURL,
		EmailVerified: u.EmailVerified, NeedsProfile: !u.ProfileCompleted,
	}
}

// GetUserByNGACNodeID retrieves a user by their NGAC graph node.
func (s *Service) GetUserByNGACNodeID(ctx context.Context, nodeID string) (*UserInfo, error) {
	user, err := s.store.GetUserByNGACNodeID(ctx, nodeID)
	if err != nil {
		return nil, fmt.Errorf("get user by ngac node: %w", err)
	}
	if user == nil {
		return nil, ErrNotFound
	}
	return &UserInfo{ID: user.ID, Username: user.Username, NGACNodeID: user.NGACNodeID}, nil
}

// ContactQuery is what a member asks of the workspace directory.
type ContactQuery struct {
	// DepartmentID narrows to one department by id; Department by its name.
	DepartmentID string
	Department   string
	Location     string
	Search       string
	// Cursor is the NextCursor of the page before; empty for the first page.
	Cursor string
	Limit  int
}

// Directory page sizes: the default, and the most one request may ask for.
const (
	DefaultContactsLimit = 50
	MaxContactsLimit     = 500
	maxCursorBytes       = 512
)

// encodeCursor and decodeCursor keep the keyset position opaque to clients: it
// is a place to continue from, not something to build.
func encodeCursor(c *store.ContactCursor) string {
	if c == nil {
		return ""
	}
	raw, _ := json.Marshal(c)
	return base64.RawURLEncoding.EncodeToString(raw)
}

func decodeCursor(s string) (*store.ContactCursor, error) {
	if s == "" {
		return nil, nil
	}
	if len(s) > maxCursorBytes {
		return nil, ErrInvalidInput
	}
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return nil, ErrInvalidInput
	}
	var c store.ContactCursor
	if err := json.Unmarshal(raw, &c); err != nil || c.ID == "" {
		return nil, ErrInvalidInput
	}
	return &c, nil
}

// scopeTimeout bounds the ask of the workspace service: Danh bạ waits for the scope, and
// when it does not come in time the answer is "try again", never the whole directory.
const scopeTimeout = 5 * time.Second

// MaxResolveIDs is the most people one resolve call may name, user ids and node ids together.
const MaxResolveIDs = 200

// directoryScope asks the workspace service what the caller may see, as the caller. The
// workspace makes the caller prove graph membership before it decides anything, so a
// PermissionDenied is a refusal, and every other failure is an outage: a scope that could
// not be had is never guessed.
func (s *Service) directoryScope(ctx context.Context, callerID, callerNode, workspaceID string) (*workspacepb.DirectoryScope, error) {
	if s.wsClient == nil {
		return nil, fmt.Errorf("%w: no workspace connection", ErrUnavailable)
	}
	ctx, cancel := context.WithTimeout(actingAs(ctx, callerID, callerNode), scopeTimeout)
	defer cancel()
	scope, err := s.wsClient.GetDirectoryScope(ctx, &workspacepb.GetDirectoryScopeRequest{WorkspaceId: workspaceID})
	if err != nil {
		switch status.Code(err) {
		case codes.PermissionDenied:
			return nil, ErrAccessDenied
		case codes.Unimplemented:
			// Auth shipped before the workspace service it asks: a deploy-order mistake,
			// named so it is not mistaken for an outage. Still a refusal to guess.
			slog.Error("workspace service too old: it has no GetDirectoryScope; deploy workspace before auth", "workspace", workspaceID, "error", err)
		}
		return nil, fmt.Errorf("%w: ask the workspace for the directory scope: %v", ErrUnavailable, err)
	}
	if scope == nil {
		return nil, fmt.Errorf("%w: the workspace answered with no scope", ErrUnavailable)
	}
	return scope, nil
}

// contactScope turns what the workspace granted into the rule of the query. An
// administrator, and a workspace with no directory rules yet (not ready), see everyone.
func contactScope(callerID string, sc *workspacepb.DirectoryScope) store.ContactScope {
	if sc.Admin || !sc.Ready {
		return store.ContactScope{All: true, CallerID: callerID}
	}
	return store.ContactScope{
		CallerID:      callerID,
		DepartmentIDs: sc.VisibleDepartmentIds,
		NoDepartment:  sc.NoDepartmentVisible,
		OwnerNodeIDs:  sc.OwnerNodeIds,
		OwnersVisible: sc.OwnersVisible,
	}
}

// rolesByNode maps a person's NGAC node to the roles the scope shows them holding.
func rolesByNode(sc *workspacepb.DirectoryScope) map[string][]ContactRole {
	out := map[string][]ContactRole{}
	for _, r := range sc.Roles {
		role := ContactRole{ID: r.Id, Kind: r.Kind}
		if r.Kind != ngac.RoleKindOwners {
			role.Name = r.Name
		}
		for _, node := range r.HolderNodeIds {
			out[node] = append(out[node], role)
		}
	}
	return out
}

// ListContacts returns one page of the workspace directory as the caller may see it, the
// true number of people matching, and the cursor of the next page ("" on the last).
//
// Two checks stand before any row is read: the caller's row is an active one, and the
// workspace service, asked as the caller, grants a scope (which includes its own graph
// membership check). Without a scope there is no directory: the answer is a refusal or
// "unavailable", never the unfiltered list. A person's display name is empty until they
// have saved their profile: the directory never shows a login handle as a name.
func (s *Service) ListContacts(ctx context.Context, callerID, callerNode, workspaceID string, q ContactQuery) (*ContactPage, error) {
	if workspaceID == "" {
		return nil, ErrInvalidInput
	}
	membership, err := s.store.GetTenantUser(ctx, workspaceID, callerID)
	if err != nil {
		return nil, fmt.Errorf("check membership: %w", err)
	}
	if membership == nil || membership.Status != "active" {
		return nil, ErrAccessDenied
	}
	limit := q.Limit
	if limit <= 0 {
		limit = DefaultContactsLimit
	}
	if limit > MaxContactsLimit || len([]rune(q.Search)) > 100 {
		return nil, ErrInvalidInput
	}
	after, err := decodeCursor(q.Cursor)
	if err != nil {
		return nil, err
	}
	scope, err := s.directoryScope(ctx, callerID, callerNode, workspaceID)
	if err != nil {
		return nil, err
	}
	users, total, next, err := s.store.ListContactsByWorkspace(ctx, workspaceID, store.ContactFilter{
		Scope:        contactScope(callerID, scope),
		DepartmentID: strings.TrimSpace(q.DepartmentID),
		Department:   strings.TrimSpace(q.Department), Location: strings.TrimSpace(q.Location),
		Search: strings.TrimSpace(q.Search), Limit: limit, After: after,
	})
	if err != nil {
		return nil, fmt.Errorf("list contacts: %w", err)
	}
	roles := rolesByNode(scope)
	contacts := make([]ContactInfo, len(users))
	for i, u := range users {
		contacts[i] = ContactInfo{
			UserID: u.ID, NGACNodeID: u.NGACNodeID, Username: u.Username,
			DisplayName: u.DisplayName, Email: u.Email,
			Title: u.Title, Department: u.Department, DepartmentID: u.DepartmentID,
			Location: u.Location, AvatarURL: u.AvatarURL, Roles: roles[u.NGACNodeID],
		}
	}
	visibility := "all"
	if scope.Ready && scope.Limited && !scope.Admin {
		visibility = "limited"
	}
	return &ContactPage{Contacts: contacts, Total: total, NextCursor: encodeCursor(next), Visibility: visibility}, nil
}

// ResolvePeople gives a name and a picture to the people a member met in content (a message,
// an approval, a document, an invitation), by user id or by NGAC node. Any member of the
// workspace may ask about any person the workspace ever held, whatever the status of their
// row, hidden from the directory or removed: the name stays where the work stays. Nothing
// else is returned (no address, title or department), the ids of another workspace are
// omitted, and nobody can be listed this way.
//
// The caller must be an active member in the row and the graph (requireActiveMember, over
// the policy writer so a removal made a moment ago counts). The workspace service is not
// asked: names keep working when it is down.
func (s *Service) ResolvePeople(ctx context.Context, callerID, callerNode, workspaceID string, userIDs, nodeIDs []string) ([]ResolvedPerson, error) {
	if workspaceID == "" || len(userIDs)+len(nodeIDs) > MaxResolveIDs {
		return nil, ErrInvalidInput
	}
	if err := s.requireActiveMember(ctx, workspaceID, callerID, callerNode); err != nil {
		return nil, err
	}
	if len(userIDs)+len(nodeIDs) == 0 {
		return []ResolvedPerson{}, nil
	}
	found, err := s.store.ResolvePeopleByWorkspace(ctx, workspaceID, userIDs, nodeIDs)
	if err != nil {
		return nil, fmt.Errorf("resolve people: %w", err)
	}
	out := make([]ResolvedPerson, len(found))
	for i, p := range found {
		out[i] = ResolvedPerson{UserID: p.UserID, NGACNodeID: p.NGACNodeID, DisplayName: p.DisplayName, AvatarURL: p.AvatarURL}
	}
	return out, nil
}
