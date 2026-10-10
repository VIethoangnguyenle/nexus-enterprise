package domain

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"

	"ngac-platform/ngac"
	"ngac-platform/pkg/realtime"
	policypb "ngac-platform/proto/policy"
)

// RoleKind tells a workspace's two built-in roles from the ones an
// administrator made. Screens name the built-in ones themselves from the kind,
// so the server never sends an English label for them.
type RoleKind string

const (
	RoleOwners  RoleKind = "owners"
	RoleMembers RoleKind = "members"
	RoleCustom  RoleKind = "custom"
)

// RoleSummary is a role as the list shows it. Name is the display name an
// administrator typed (empty for the built-in roles).
type RoleSummary struct {
	ID          string
	Name        string
	Kind        RoleKind
	MemberCount int
}

// PersonRef is a person as an avatar and a name.
type PersonRef struct {
	NodeID      string
	UserID      string
	DisplayName string
	AvatarURL   string
}

// RoleDetail is one role with who holds it and what it confers.
type RoleDetail struct {
	RoleSummary
	// Members is a sample for the avatars; MemberCount is the whole.
	Members []*PersonRef
	Grants  []AreaGrant
}

// roleSampleSize is how many holders of a role a detail lists by name.
const roleSampleSize = 50

// classifyRole reads a node of the workspace as a role: an administrator's
// (type=role) or one of the two the platform builds. Anything else is not one.
func classifyRole(n *policypb.NGACNode, wsID string) (RoleKind, bool) {
	if n.GetNodeType() != ngac.TypeUA {
		return "", false
	}
	switch {
	case isRole(n):
		return RoleCustom, true
	case n.GetName() == ngac.OwnersUAName(ngac.WorkspaceID(wsID)):
		return RoleOwners, true
	case n.GetName() == ngac.MembersUAName(ngac.WorkspaceID(wsID)):
		return RoleMembers, true
	}
	return "", false
}

// userNodes keeps the user nodes of a list.
func userNodes(nodes []*policypb.NGACNode) []*policypb.NGACNode {
	var out []*policypb.NGACNode
	for _, n := range nodes {
		if n.GetNodeType() == ngac.TypeU {
			out = append(out, n)
		}
	}
	return out
}

// roleHolders returns the people a role directly holds. For Members it is
// everyone beneath the attribute, since Owners sits under Members.
func (s *Service) roleHolders(ctx context.Context, kind RoleKind, uaID string) ([]*policypb.NGACNode, error) {
	if kind == RoleMembers {
		desc, err := s.policyWrite.GetDescendants(ctx, &policypb.GetDescendantsRequest{NodeId: uaID})
		if err != nil {
			return nil, fmt.Errorf("get role members: %w", err)
		}
		return uniqueNodes(userNodes(desc.GetNodes())), nil
	}
	kids, err := s.policyWrite.GetChildren(ctx, &policypb.GetChildrenRequest{NodeId: uaID})
	if err != nil {
		return nil, fmt.Errorf("get role members: %w", err)
	}
	return uniqueNodes(userNodes(kids.GetNodes())), nil
}

func uniqueNodes(nodes []*policypb.NGACNode) []*policypb.NGACNode {
	seen := make(map[string]bool, len(nodes))
	out := make([]*policypb.NGACNode, 0, len(nodes))
	for _, n := range nodes {
		if !seen[n.Id] {
			seen[n.Id] = true
			out = append(out, n)
		}
	}
	return out
}

// ListRoleSummaries returns the built-in roles first, then the administrator's
// by name, each with how many people hold it. The caller must belong to the
// workspace.
func (s *Service) ListRoleSummaries(ctx context.Context, callerNodeID, wsID string) ([]*RoleSummary, error) {
	ws, err := s.authorizeMember(ctx, callerNodeID, wsID)
	if err != nil {
		return nil, err
	}
	children, err := s.policyWrite.GetChildren(ctx, &policypb.GetChildrenRequest{NodeId: ws.PcNodeID})
	if err != nil {
		return nil, fmt.Errorf("get children: %w", err)
	}
	var owners, members *RoleSummary
	var custom []*RoleSummary
	for _, n := range children.GetNodes() {
		kind, ok := classifyRole(n, ws.ID)
		if !ok {
			continue
		}
		holders, err := s.roleHolders(ctx, kind, n.Id)
		if err != nil {
			return nil, err
		}
		r := &RoleSummary{ID: n.Id, Kind: kind, MemberCount: len(holders)}
		switch kind {
		case RoleOwners:
			owners = r
		case RoleMembers:
			members = r
		default:
			r.Name = ngac.DisplayName(n.Name, n.Properties)
			custom = append(custom, r)
		}
	}
	sort.SliceStable(custom, func(i, j int) bool { return strings.ToLower(custom[i].Name) < strings.ToLower(custom[j].Name) })
	var out []*RoleSummary
	for _, r := range []*RoleSummary{owners, members} {
		if r != nil {
			out = append(out, r)
		}
	}
	return append(out, custom...), nil
}

// GetRoleDetail returns one role with some of its holders and its grants by
// area. The caller must hold manage on the workspace's Mgmt OA: what a role
// confers is administration, not something every member reads.
func (s *Service) GetRoleDetail(ctx context.Context, callerNodeID, wsID, roleID string) (*RoleDetail, error) {
	ws, err := s.authorizeAdmin(ctx, callerNodeID, wsID, ngac.OpManage)
	if err != nil {
		return nil, err
	}
	n, err := s.nodeInWorkspace(ctx, ws, roleID, ngac.TypeUA)
	if err != nil {
		return nil, err
	}
	kind, ok := classifyRole(n, ws.ID)
	if !ok {
		return nil, fmt.Errorf("%w: role not in this workspace", ErrNotFound)
	}
	holders, err := s.roleHolders(ctx, kind, n.Id)
	if err != nil {
		return nil, err
	}
	detail := &RoleDetail{RoleSummary: RoleSummary{ID: n.Id, Kind: kind, MemberCount: len(holders)}}
	if kind == RoleCustom {
		detail.Name = ngac.DisplayName(n.Name, n.Properties)
	}

	sample := holders
	if len(sample) > roleSampleSize {
		sample = sample[:roleSampleSize]
	}
	detail.Members = s.peopleFor(ctx, ws.ID, sample)

	assocs, err := s.policyWrite.GetAssociations(ctx, &policypb.GetAssociationsRequest{UaId: n.Id})
	if err != nil {
		return nil, fmt.Errorf("get role grants: %w", err)
	}
	oas, err := s.areaOAs(ctx, ws)
	if err != nil {
		return nil, err
	}
	detail.Grants = grantsByArea(assocs.GetAssociations(), oas)
	return detail, nil
}

// SetRolePermissions makes ops exactly what a custom role holds on one area; an
// empty list takes the area away from the role.
//
// The caller needs manage on the Mgmt OA, and — as for any delegation — must
// hold every operation they add on the area's OA. Keeping or removing what the
// role already has needs nothing more. The OA is resolved from the area inside
// this workspace, never taken from the request, and only the administrator's
// own roles can be changed: Owners and Members are not editable.
func (s *Service) SetRolePermissions(ctx context.Context, callerNodeID, wsID, roleID string, area ngac.Area, ops []string) (*AreaGrant, error) {
	ws, err := s.authorizeAdmin(ctx, callerNodeID, wsID, ngac.OpManage)
	if err != nil {
		return nil, err
	}
	if !ngac.IsArea(area) {
		return nil, fmt.Errorf("%w: unknown area", ErrInvalidInput)
	}
	want, err := normalizeAreaOps(area, ops)
	if err != nil {
		return nil, err
	}
	if err := s.requireRole(ctx, ws, roleID); err != nil {
		return nil, err
	}
	oas, err := s.areaOAs(ctx, ws)
	if err != nil {
		return nil, err
	}
	oaID, ok := oas[area]
	if !ok {
		return nil, fmt.Errorf("%w: this workspace has no %s area", ErrNotFound, area)
	}

	// Read from the writer: what the role holds now decides which operations are
	// being added, and a replica that is behind would say an operation is kept
	// when it is not.
	assocs, err := s.policyWrite.GetAssociations(ctx, &policypb.GetAssociationsRequest{UaId: roleID})
	if err != nil {
		return nil, fmt.Errorf("read role grants: %w", err)
	}
	var have []string
	for _, a := range assocs.GetAssociations() {
		if a.OaId == oaID {
			have = append(have, a.Operations...)
		}
	}
	for _, op := range want {
		if slices.Contains(have, op) {
			continue
		}
		if err := s.checkAccess(ctx, callerNodeID, oaID, op); err != nil {
			return nil, fmt.Errorf("cannot grant an operation you do not hold: %w", err)
		}
	}

	grant := &AreaGrant{Area: area, Operations: want}
	haveSet := filterAreaOps(area, have)
	switch {
	case slices.Equal(haveSet, want):
		// Nothing to change: no write, no invalidation.
	case len(want) == 0:
		if _, err := s.policyWrite.RemoveAssociation(ctx, &policypb.RemoveAssociationRequest{UaId: roleID, OaId: oaID}); err != nil {
			return nil, fmt.Errorf("remove permission: %w", err)
		}
	default:
		if _, err := s.policyWrite.CreateAssociation(ctx, &policypb.CreateAssociationRequest{UaId: roleID, OaId: oaID, Operations: want}); err != nil {
			return nil, fmt.Errorf("set permission: %w", err)
		}
	}
	if !slices.Equal(haveSet, want) {
		s.announce(ctx, realtime.KindRoleChanged, ws.ID, roleID)
	}
	return grant, nil
}

// filterAreaOps keeps the operations an area offers, once each in canonical
// order, and drops anything else without complaint.
func filterAreaOps(area ngac.Area, ops []string) []string {
	var out []string
	for _, op := range ngac.AreaOps(area) {
		if slices.Contains(ops, op) {
			out = append(out, op)
		}
	}
	return out
}

// normalizeAreaOps checks every operation is one the area offers and returns
// them once each in canonical order. A stored operation the area does not offer
// is refused when asked for.
func normalizeAreaOps(area ngac.Area, ops []string) ([]string, error) {
	valid := ngac.AreaOps(area)
	for _, op := range ops {
		if !slices.Contains(valid, op) {
			return nil, fmt.Errorf("%w: %q does not apply to %s", ErrInvalidInput, op, area)
		}
	}
	return filterAreaOps(area, ops), nil
}

// AssignMemberRole gives a member one more role, leaving everything else they
// belong to alone. The caller needs manage on the Mgmt OA and must hold
// everything the role confers; the person must already belong to the workspace
// (manage does not include inviting).
func (s *Service) AssignMemberRole(ctx context.Context, callerNodeID, wsID, targetNodeID, roleID string) error {
	ws, err := s.authorizeAdmin(ctx, callerNodeID, wsID, ngac.OpManage)
	if err != nil {
		return err
	}
	if err := s.requireRole(ctx, ws, roleID); err != nil {
		return err
	}
	if err := s.requireMemberUser(ctx, ws, targetNodeID); err != nil {
		return err
	}
	if err := s.guardDelegation(ctx, callerNodeID, roleID); err != nil {
		return err
	}
	if _, err := s.policyWrite.CreateAssignment(ctx, &policypb.CreateAssignmentRequest{ChildId: targetNodeID, ParentId: roleID}); err != nil {
		return fmt.Errorf("assign role: %w", err)
	}
	s.announce(ctx, realtime.KindRoleChanged, ws.ID, targetNodeID, roleID)
	s.announceAccessChanged(ctx, ws.ID, targetNodeID)
	return nil
}

// UnassignMemberRole takes one role from a member. Taking away needs no more
// than manage: it hands nothing out.
func (s *Service) UnassignMemberRole(ctx context.Context, callerNodeID, wsID, targetNodeID, roleID string) error {
	ws, err := s.authorizeAdmin(ctx, callerNodeID, wsID, ngac.OpManage)
	if err != nil {
		return err
	}
	if err := s.requireRole(ctx, ws, roleID); err != nil {
		return err
	}
	if err := s.requireMemberUser(ctx, ws, targetNodeID); err != nil {
		return err
	}
	if _, err := s.policyWrite.RemoveAssignment(ctx, &policypb.RemoveAssignmentRequest{ChildId: targetNodeID, ParentId: roleID}); err != nil {
		return fmt.Errorf("unassign role: %w", err)
	}
	s.announce(ctx, realtime.KindRoleChanged, ws.ID, targetNodeID, roleID)
	s.announceAccessChanged(ctx, ws.ID, targetNodeID)
	return nil
}
