package domain

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

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
	Department string
	Location   string
	Search     string
	// Cursor is the NextCursor of the page before; empty for the first page.
	Cursor string
	Limit  int
}

// Directory page sizes: the default, and the most one request may ask for.
const (
	DefaultContactsLimit = 50
	MaxContactsLimit     = 200
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

// ListContacts returns one page of the workspace directory, the true number of
// people matching, and the cursor of the next page ("" on the last). Only an
// active member of that workspace may read it. A person's display name is empty
// until they have saved their profile: the directory never shows a login handle
// as a name.
func (s *Service) ListContacts(ctx context.Context, callerID, workspaceID string, q ContactQuery) ([]ContactInfo, int, string, error) {
	if workspaceID == "" {
		return nil, 0, "", ErrInvalidInput
	}
	membership, err := s.store.GetTenantUser(ctx, workspaceID, callerID)
	if err != nil {
		return nil, 0, "", fmt.Errorf("check membership: %w", err)
	}
	if membership == nil || membership.Status != "active" {
		return nil, 0, "", ErrAccessDenied
	}
	limit := q.Limit
	if limit <= 0 {
		limit = DefaultContactsLimit
	}
	if limit > MaxContactsLimit || len([]rune(q.Search)) > 100 {
		return nil, 0, "", ErrInvalidInput
	}
	after, err := decodeCursor(q.Cursor)
	if err != nil {
		return nil, 0, "", err
	}
	users, total, next, err := s.store.ListContactsByWorkspace(ctx, workspaceID, store.ContactFilter{
		Department: strings.TrimSpace(q.Department), Location: strings.TrimSpace(q.Location),
		Search: strings.TrimSpace(q.Search), Limit: limit, After: after,
	})
	if err != nil {
		return nil, 0, "", fmt.Errorf("list contacts: %w", err)
	}
	contacts := make([]ContactInfo, len(users))
	for i, u := range users {
		contacts[i] = ContactInfo{
			UserID: u.ID, NGACNodeID: u.NGACNodeID, Username: u.Username,
			DisplayName: u.DisplayName, Email: u.Email,
			Title: u.Title, Department: u.Department,
			Location: u.Location, AvatarURL: u.AvatarURL,
		}
	}
	return contacts, total, encodeCursor(next), nil
}
