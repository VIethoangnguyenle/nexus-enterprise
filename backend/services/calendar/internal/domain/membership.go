package domain

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"ngac-platform/pkg/grpcauth"
	policypb "ngac-platform/proto/policy"
	"ngac-platform/services/calendar/internal/store"
)

// Workspace types, as workspaces.type holds them.
const workspaceTypePersonal = "personal"

// requireMember admits the caller to a workspace. The workspace is the one the
// verified token is scoped to, never one a request names for itself: a token for
// another workspace gets nothing here. The caller must also belong to it, which
// is the workspace's own test for membership — its policy class is among the
// caller's ancestors. Every failure denies, including a policy service that
// cannot answer.
func (s *Service) requireMember(ctx context.Context, wsID string) (grpcauth.Caller, *store.Workspace, error) {
	who := grpcauth.CallerFrom(ctx)
	if !who.Authenticated() || wsID == "" || who.TenantID != wsID {
		return who, nil, denied("not available in this workspace")
	}
	ws, err := s.store.Workspace(ctx, wsID)
	switch {
	case errors.Is(err, store.ErrNotFound):
		return who, nil, denied("not available in this workspace")
	case err != nil:
		return who, nil, internal("load workspace", err)
	case ws.PCNodeID == "":
		return who, nil, internal("load workspace", errors.New("workspace has no policy class"))
	}
	in, err := s.inWorkspace(ctx, ws, who.NGACNodeID)
	if err != nil {
		slog.Warn("cannot resolve workspace membership; denying", "workspace", wsID, "error", err)
		return who, nil, denied("not a member of this workspace")
	}
	if !in {
		return who, nil, denied("not a member of this workspace")
	}
	return who, ws, nil
}

// inWorkspace reports whether the workspace's policy class is among the
// ancestors of the user node.
func (s *Service) inWorkspace(ctx context.Context, ws *store.Workspace, userNodeID string) (bool, error) {
	if userNodeID == "" {
		return false, nil
	}
	anc, err := s.policyRead.GetAncestors(ctx, &policypb.GetAncestorsRequest{NodeId: userNodeID})
	if err != nil {
		return false, fmt.Errorf("ancestors of %s: %w", userNodeID, err)
	}
	for _, n := range anc.GetNodes() {
		if n.GetId() == ws.PCNodeID {
			return true, nil
		}
	}
	return false, nil
}

// requirePerson admits a person the caller names (a guest, a grantee, someone to
// subscribe to): an active member of the workspace by the database and by the
// graph. Anyone else is refused, and the answer does not say which test failed.
func (s *Service) requirePerson(ctx context.Context, ws *store.Workspace, nodeID string) (*store.Person, error) {
	if nodeID == "" {
		return nil, invalid("a person is required")
	}
	p, err := s.store.PersonByNode(ctx, ws.ID, nodeID)
	switch {
	case errors.Is(err, store.ErrNotFound):
		return nil, denied("not a member of this workspace")
	case err != nil:
		return nil, internal("load person", err)
	}
	in, err := s.inWorkspace(ctx, ws, nodeID)
	if err != nil {
		slog.Warn("cannot resolve membership of a named person; denying", "workspace", ws.ID, "error", err)
		return nil, denied("not a member of this workspace")
	}
	if !in {
		return nil, denied("not a member of this workspace")
	}
	return p, nil
}

// loadCalendar reads a calendar of the workspace. One of another workspace is
// not found from here: whether it exists is not the caller's to learn.
func (s *Service) loadCalendar(ctx context.Context, wsID, calID string) (*store.Calendar, error) {
	cal, err := s.store.CalendarByID(ctx, calID)
	switch {
	case errors.Is(err, store.ErrNotFound):
		return nil, notFound("calendar not found")
	case err != nil:
		return nil, internal("load calendar", err)
	case cal.WorkspaceID != wsID:
		return nil, notFound("calendar not found")
	}
	return cal, nil
}
