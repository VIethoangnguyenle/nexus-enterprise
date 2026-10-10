package domain

import (
	"context"
	"fmt"
	"time"
)

const (
	// WorkspaceCreateLimit is how many workspaces one person may create per
	// workspaceCreateWindow. Creating one provisions a graph, a channel and a
	// bucket, so an unbounded loop is a way to fill the system with them.
	WorkspaceCreateLimit  = 5
	workspaceCreateWindow = time.Hour
	workspaceRateKey      = "ws_create_rl:"
)

// WorkspaceInfo is a workspace as the picker shows it: a name, the person's
// own role in it and how many people belong.
type WorkspaceInfo struct {
	ID          string
	Name        string
	Role        string
	MemberCount int
	// Domain is the company domain the workspace claimed, when it is a company's.
	Domain string
}

// ListMyWorkspaces returns the workspaces userID is an active member of.
func (s *Service) ListMyWorkspaces(ctx context.Context, userID string) ([]WorkspaceInfo, error) {
	if userID == "" {
		return nil, ErrInvalidInput
	}
	rows, err := s.store.ListWorkspaceSummaries(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list my workspaces: %w", err)
	}
	out := make([]WorkspaceInfo, len(rows))
	for i, r := range rows {
		out[i] = WorkspaceInfo{ID: r.ID, Name: r.Name, Role: r.Role, MemberCount: r.MemberCount, Domain: r.Domain}
	}
	return out, nil
}

// CreateMyWorkspace creates a workspace owned by the caller and provisions it
// the way sign-in does (graph, membership row, #general). The name is the only
// input: a workspace is addressed by its ID, never by a name or slug.
//
// The caller's address must be verified: a workspace is the thing other people
// are later invited into, by that address, and an account whose address nobody
// has proved is one anyone could have made with the fixed test code.
//
// Unlike sign-in provisioning, any failure after the workspace exists undoes it
// (through the workspace service) and is returned, so a workspace whose graph
// is half built is never left for someone to find.
func (s *Service) CreateMyWorkspace(ctx context.Context, userID, ngacNodeID, name string) (*WorkspaceInfo, error) {
	if userID == "" || ngacNodeID == "" {
		return nil, ErrInvalidInput
	}
	name, err := cleanText(name, maxWorkspaceNameRunes, true)
	if err != nil {
		return nil, err
	}
	user, err := s.store.GetUserByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	if user == nil {
		return nil, ErrNotFound
	}
	if user.Email == "" || !user.EmailVerified {
		return nil, ErrEmailUnverified
	}
	if err := s.takeWorkspaceCreation(ctx, userID); err != nil {
		return nil, err
	}

	id, wsName, role, err := s.provisionTenant(ctx, name, userID, ngacNodeID, true)
	if err != nil {
		return nil, fmt.Errorf("create workspace: %w", err)
	}
	return &WorkspaceInfo{ID: id, Name: wsName, Role: role, MemberCount: 1}, nil
}

// takeWorkspaceCreation spends one of the caller's creations for this window.
// Without Redis there is nothing to count against and creation is not limited
// (only unit tests run that way: sign-in itself needs Redis for sessions).
func (s *Service) takeWorkspaceCreation(ctx context.Context, userID string) error {
	if s.rdb == nil {
		return nil
	}
	key := workspaceRateKey + userID
	n, err := s.rdb.Incr(ctx, key).Result()
	if err != nil {
		return fmt.Errorf("workspace rate limit: %w", err)
	}
	if n == 1 {
		s.rdb.Expire(ctx, key, workspaceCreateWindow)
	}
	if n <= WorkspaceCreateLimit {
		return nil
	}
	return &rateLimitedError{base: ErrRateLimited, after: s.windowLeft(ctx, key, workspaceCreateWindow)}
}

// windowLeft is how long a counter has to live. A counter that lost its expiry
// (the process died between INCR and EXPIRE) would lock the person out for
// good, so it is given one again.
func (s *Service) windowLeft(ctx context.Context, key string, window time.Duration) time.Duration {
	ttl, err := s.rdb.TTL(ctx, key).Result()
	if err != nil || ttl <= 0 {
		s.rdb.Expire(ctx, key, window)
		return window
	}
	return ttl
}
