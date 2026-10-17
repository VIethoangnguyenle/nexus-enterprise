package domain

import (
	"context"
	"fmt"
	"slices"

	"ngac-platform/ngac"
	"ngac-platform/pkg/policyclient"
	"ngac-platform/services/auth/internal/store"
)

// requireActiveMember reports whether the person may hold a token for the tenant:
// ErrAccessDenied when they may not, ErrUnavailable when it could not be told.
//
// A person is a member when the graph says so (they sit under `<ws>_Members`) and
// their row does not say otherwise (`removed`, `disabled` and `invited` are not
// membership). The graph is asked through the policy writer, not the read replica,
// because the answer has to include a removal made a moment ago. A graph member
// with no row is let in: rows are written after the graph, and a row missing for
// an old member is a gap migration 063 fills, not a reason to lock them out.
//
// A tenant that no longer exists, or has no Members UA, holds nobody.
func (s *Service) requireActiveMember(ctx context.Context, tenantID, userID, nodeID string) error {
	if s.policyWrite == nil {
		return fmt.Errorf("%w: no policy connection", ErrUnavailable)
	}
	row, err := s.store.GetTenantUser(ctx, tenantID, userID)
	if err != nil {
		return fmt.Errorf("%w: read membership row: %v", ErrUnavailable, err)
	}
	if row != nil && row.Status != "active" {
		return ErrAccessDenied
	}
	if nodeID == "" {
		return ErrAccessDenied
	}
	ok, err := policyclient.IsWorkspaceMember(actingAs(ctx, userID, nodeID), s.policyWrite, nodeID, ngac.WorkspaceID(tenantID))
	if err != nil {
		// No Members UA to be under (the workspace is gone, or its node is only a
		// lookalike): nobody is a member, and asking again will not change that.
		if policyclient.IsNotFound(err) || policyclient.IsNotMembersUA(err) {
			return ErrAccessDenied
		}
		return fmt.Errorf("%w: ask the policy about membership: %v", ErrUnavailable, err)
	}
	if !ok {
		return ErrAccessDenied
	}
	return nil
}

// firstTenantWhereMember is the person's default tenant among those they still
// belong to, in sign-in order (a workspace they own first, then the earliest
// joined), skipping the tenant named in skip. It returns "" when there is none.
//
// The graph is asked once, for every workspace the person sits in; the rows (active
// ones only) decide the order. A workspace whose Members UA is only a lookalike is left
// out of that answer by the helper, so one odd tenant cannot fail the whole refresh.
func (s *Service) firstTenantWhereMember(ctx context.Context, userID, nodeID, skip string) (string, error) {
	tenants, err := s.store.ListTenantsByUser(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("%w: list tenants: %v", ErrUnavailable, err)
	}
	if len(tenants) == 0 || nodeID == "" {
		return "", nil
	}
	inGraph, err := policyclient.MemberWorkspaces(actingAs(ctx, userID, nodeID), s.policyWrite, nodeID)
	if err != nil {
		return "", fmt.Errorf("%w: ask the policy for the person's workspaces: %v", ErrUnavailable, err)
	}
	for _, t := range inDefaultOrder(tenants) {
		if t.TenantID != skip && slices.Contains(inGraph, ngac.WorkspaceID(t.TenantID)) {
			return t.TenantID, nil
		}
	}
	return "", nil
}

// inDefaultOrder lists the tenants as selectDefaultTenant would pick from them:
// owned ones first, the rest as given.
func inDefaultOrder(tenants []store.TenantMembership) []store.TenantMembership {
	out := make([]store.TenantMembership, 0, len(tenants))
	for _, t := range tenants {
		if t.Role == "owner" {
			out = append(out, t)
		}
	}
	for _, t := range tenants {
		if t.Role != "owner" {
			out = append(out, t)
		}
	}
	return out
}
