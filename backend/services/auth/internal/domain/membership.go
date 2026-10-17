package domain

import (
	"context"
	"fmt"
	"log/slog"
	"slices"

	"ngac-platform/ngac"
	"ngac-platform/pkg/policyclient"
	policypb "ngac-platform/proto/policy"
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
	_, err := s.activeMemberRow(ctx, tenantID, userID, nodeID)
	return err
}

// activeMemberRow is requireActiveMember that also hands back the row it read (nil for a
// graph member whose row is missing), so a caller that needs the row reads it once.
func (s *Service) activeMemberRow(ctx context.Context, tenantID, userID, nodeID string) (*store.TenantMembership, error) {
	if s.policyWrite == nil {
		return nil, fmt.Errorf("%w: no policy connection", ErrUnavailable)
	}
	row, err := s.store.GetTenantUser(ctx, tenantID, userID)
	if err != nil {
		return nil, fmt.Errorf("%w: read membership row: %v", ErrUnavailable, err)
	}
	if row != nil && row.Status != "active" {
		return nil, ErrAccessDenied
	}
	if nodeID == "" {
		return nil, ErrAccessDenied
	}
	ok, err := policyclient.IsWorkspaceMember(actingAs(ctx, userID, nodeID), s.policyWrite, nodeID, ngac.WorkspaceID(tenantID))
	if err != nil {
		// No Members UA to be under (the workspace is gone, or its node is only a
		// lookalike): nobody is a member, and asking again will not change that.
		if policyclient.IsNotFound(err) || policyclient.IsNotMembersUA(err) {
			return nil, ErrAccessDenied
		}
		return nil, fmt.Errorf("%w: ask the policy about membership: %v", ErrUnavailable, err)
	}
	if !ok {
		return nil, ErrAccessDenied
	}
	return row, nil
}

// workspacesOfPerson is every workspace the graph holds the person in, from one ask of
// the policy writer. ErrUnavailable when it could not be asked. A lookalike "_Members" UA
// is left out by the helper, so one odd tenant cannot fail the whole answer.
func (s *Service) workspacesOfPerson(ctx context.Context, userID, nodeID string) ([]ngac.WorkspaceID, error) {
	if nodeID == "" {
		return nil, nil
	}
	if s.policyWrite == nil {
		return nil, fmt.Errorf("%w: no policy connection", ErrUnavailable)
	}
	inGraph, err := policyclient.MemberWorkspaces(actingAs(ctx, userID, nodeID), s.policyWrite, nodeID)
	if err != nil {
		return nil, fmt.Errorf("%w: ask the policy for the person's workspaces: %v", ErrUnavailable, err)
	}
	return inGraph, nil
}

// graphMemberTenants keeps the tenants the graph holds the person in, in the order
// given. The rows list who was written down; a token is minted only for a tenant the
// graph agrees on. ErrUnavailable when the graph could not be asked.
func (s *Service) graphMemberTenants(ctx context.Context, userID, nodeID string, tenants []store.TenantMembership) ([]store.TenantMembership, error) {
	if len(tenants) == 0 {
		return nil, nil
	}
	inGraph, err := s.workspacesOfPerson(ctx, userID, nodeID)
	if err != nil {
		return nil, err
	}
	return keepHeld(tenants, inGraph), nil
}

func keepHeld(tenants []store.TenantMembership, inGraph []ngac.WorkspaceID) []store.TenantMembership {
	var kept []store.TenantMembership
	for _, t := range tenants {
		if slices.Contains(inGraph, ngac.WorkspaceID(t.TenantID)) {
			kept = append(kept, t)
		}
	}
	return kept
}

// signinTenants is the tenants a sign-in may scope a session to: those the graph holds
// the person in. The policy being unreachable must not stop anybody signing in, nor be
// answered with the unfiltered list (that would be fail-open): the person gets a
// session with no tenant, and regains one at the next sign-in or by choosing a workspace.
//
// A workspace the graph holds the person in but that has no row for them at all (a
// join whose row write failed after the graph write) is listed here: the row is written
// now, so the person is not locked out of a workspace they own. A row that exists in any
// state (removed, disabled, invited) is a decision and is never touched.
func (s *Service) signinTenants(ctx context.Context, userID, nodeID string, tenants []store.TenantMembership) []store.TenantMembership {
	inGraph, err := s.workspacesOfPerson(ctx, userID, nodeID)
	if err != nil {
		slog.Warn("sign-in without a tenant: the graph could not be asked who the person belongs to",
			"user", userID, "error", err)
		return nil
	}
	listed := map[ngac.WorkspaceID]bool{}
	for _, t := range tenants {
		listed[ngac.WorkspaceID(t.TenantID)] = true
	}
	repaired := false
	for _, ws := range inGraph {
		if !listed[ws] && s.listMissingRow(ctx, userID, nodeID, ws) {
			repaired = true
		}
	}
	if repaired {
		if again, err := s.store.ListTenantsByUser(ctx, userID); err == nil {
			tenants = again
		} else {
			slog.Warn("could not list tenants after repairing a missing row", "user", userID, "error", err)
		}
	}
	return keepHeld(tenants, inGraph)
}

// listMissingRow writes the row of a graph member who has none, as owner when they sit
// under the workspace's Owners UA and as member otherwise. An existing row, whatever its
// status, is left as it is (the insert does nothing on conflict). It reports whether it
// wrote one; a failure is logged and the next sign-in tries again.
func (s *Service) listMissingRow(ctx context.Context, userID, nodeID string, ws ngac.WorkspaceID) bool {
	role, err := s.roleInGraph(ctx, userID, nodeID, ws)
	if err != nil {
		slog.Warn("could not tell the role of a graph member with no row", "user", userID, "workspace", ws, "error", err)
		return false
	}
	if err := s.store.InsertTenantUser(ctx, string(ws), userID, role, "active", nodeID); err != nil {
		slog.Error("could not list a graph member with no row", "user", userID, "workspace", ws, "error", err)
		return false
	}
	return true
}

// roleInGraph is "owner" when the person sits under the workspace's own Owners UA (the
// node that is under its Members UA, not one that is only named like it), else "member".
func (s *Service) roleInGraph(ctx context.Context, userID, nodeID string, ws ngac.WorkspaceID) (string, error) {
	ctx = actingAs(ctx, userID, nodeID)
	ancestors, err := s.policyWrite.GetAncestors(ctx, &policypb.GetAncestorsRequest{NodeId: nodeID})
	if err != nil {
		return "", err
	}
	for _, n := range ancestors.GetNodes() {
		if n.GetNodeType() != ngac.TypeUA || n.GetName() != ngac.OwnersUAName(ws) {
			continue
		}
		membersID, err := policyclient.MembersUA(ctx, s.policyWrite, ws)
		if err != nil {
			return "", err
		}
		under, err := s.policyWrite.IsAssigned(ctx, &policypb.IsAssignedRequest{ChildId: n.GetId(), ParentId: membersID})
		if err != nil {
			return "", err
		}
		if under.GetValue() {
			return "owner", nil
		}
	}
	return "member", nil
}

// firstTenantWhereMember is the person's default tenant among those they still
// belong to, in sign-in order (a workspace they own first, then the earliest
// joined), skipping the tenant named in skip. It returns "" when there is none.
//
// The graph is asked once, for every workspace the person sits in; the rows (active
// ones only) decide the order.
func (s *Service) firstTenantWhereMember(ctx context.Context, userID, nodeID, skip string) (string, error) {
	tenants, err := s.store.ListTenantsByUser(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("%w: list tenants: %v", ErrUnavailable, err)
	}
	held, err := s.graphMemberTenants(ctx, userID, nodeID, tenants)
	if err != nil {
		return "", err
	}
	for _, t := range inDefaultOrder(held) {
		if t.TenantID != skip {
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
