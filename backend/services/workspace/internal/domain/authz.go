package domain

import (
	"context"
	"fmt"

	"ngac-platform/ngac"
	"ngac-platform/pkg/policyclient"
	policypb "ngac-platform/proto/policy"
)

// Authorization for workspace administration.
//
// The workspace service is a PEP: every mutating operation asks the policy
// service whether the caller holds the required operation on the workspace's
// management OA (ngac.MgmtOAName). Objects are not graph nodes, and neither is
// "the workspace" as such — the Mgmt OA is the attribute that stands for its
// administration, so that is where the check lands.
//
// Every helper here is fail-closed: an empty caller, a missing Mgmt OA, a
// policy-service error, or any decision other than ALLOW is a denial.

// checkAccess asks the PDP for one decision. Routed through ngac.Allowed so a
// transport error or an unrecognised decision denies.
func (s *Service) checkAccess(ctx context.Context, userNodeID, objectNodeID, operation string) error {
	if userNodeID == "" || objectNodeID == "" {
		return fmt.Errorf("%w: %s requires an authenticated caller", ErrAccessDenied, operation)
	}
	if ok, _ := policyclient.New(s.policyRead).Check(ctx, userNodeID, objectNodeID, operation); !ok {
		return fmt.Errorf("%w: %s", ErrAccessDenied, operation)
	}
	return nil
}

// mgmtOAID resolves the workspace's management OA among the direct children of
// its own PC. Looking it up under the PC — rather than by name globally — keeps
// a node that merely shares the name, created elsewhere, from standing in for it.
func (s *Service) mgmtOAID(ctx context.Context, ws *WorkspaceResult) (string, error) {
	children, err := s.policyWrite.GetChildren(ctx, &policypb.GetChildrenRequest{NodeId: ws.PcNodeID})
	if err != nil {
		return "", fmt.Errorf("%w: cannot resolve workspace management attribute", ErrAccessDenied)
	}
	want := ngac.MgmtOAName(ngac.WorkspaceID(ws.ID))
	for _, n := range children.GetNodes() {
		if n.NodeType == ngac.TypeOA && n.Name == want {
			return n.Id, nil
		}
	}
	return "", fmt.Errorf("%w: workspace has no management attribute", ErrAccessDenied)
}

// authorizeAdmin loads the workspace and requires op on its Mgmt OA.
func (s *Service) authorizeAdmin(ctx context.Context, callerNodeID, wsID, op string) (*WorkspaceResult, error) {
	if callerNodeID == "" {
		return nil, fmt.Errorf("%w: %s requires an authenticated caller", ErrAccessDenied, op)
	}
	ws, err := s.GetWorkspace(ctx, wsID)
	if err != nil {
		return nil, err
	}
	mgmtID, err := s.mgmtOAID(ctx, ws)
	if err != nil {
		return nil, err
	}
	if err := s.checkAccess(ctx, callerNodeID, mgmtID, op); err != nil {
		return nil, err
	}
	return ws, nil
}

// authorizeMember loads the workspace and requires the caller to belong to it:
// the caller must be a user under the workspace's Members UA. Reaching the policy
// class is not enough — a group's UA and an event's guest UA sit beneath it too,
// and being in one of them is not membership. ListAccessibleWorkspaces uses the
// same test, so anyone who can see a workspace in their list can read it.
func (s *Service) authorizeMember(ctx context.Context, callerNodeID, wsID string) (*WorkspaceResult, error) {
	if callerNodeID == "" {
		return nil, fmt.Errorf("%w: requires an authenticated caller", ErrAccessDenied)
	}
	ws, err := s.GetWorkspace(ctx, wsID)
	if err != nil {
		return nil, err
	}
	member, err := s.isMember(ctx, ws, callerNodeID)
	if err != nil {
		return nil, fmt.Errorf("%w: cannot resolve membership", ErrAccessDenied)
	}
	if !member {
		return nil, fmt.Errorf("%w: not a member of this workspace", ErrAccessDenied)
	}
	return ws, nil
}

// isMember reports whether the node is a user under the workspace's Members UA
// (see policyclient.IsWorkspaceMember). The writer answers: it sees a removal made
// a moment ago, which a read replica may not. An error means the answer is unknown
// and the caller denies.
func (s *Service) isMember(ctx context.Context, ws *WorkspaceResult, nodeID string) (bool, error) {
	if nodeID == "" {
		return false, nil
	}
	return policyclient.IsWorkspaceMember(ctx, s.policyWrite, nodeID, ngac.WorkspaceID(ws.ID))
}

// memberUsers lists the user nodes under the workspace's Members UA, each once.
func (s *Service) memberUsers(ctx context.Context, ws *WorkspaceResult) ([]*policypb.NGACNode, error) {
	membersID, err := policyclient.MembersUA(ctx, s.policyWrite, ngac.WorkspaceID(ws.ID))
	if err != nil {
		return nil, err
	}
	desc, err := s.policyWrite.GetDescendants(ctx, &policypb.GetDescendantsRequest{NodeId: membersID})
	if err != nil {
		return nil, fmt.Errorf("get descendants: %w", err)
	}
	return uniqueNodes(userNodes(desc.GetNodes())), nil
}

// requireInWorkspace confirms nodeID is a node of the given type under the
// workspace's PC. Admin routes accept node IDs from the client; without this,
// holding manage on one workspace would let a caller delete or attach to
// nodes of another by passing their IDs through its own route.
func (s *Service) requireInWorkspace(ctx context.Context, ws *WorkspaceResult, nodeID, nodeType string) error {
	_, err := s.nodeInWorkspace(ctx, ws, nodeID, nodeType)
	return err
}

func (s *Service) nodeInWorkspace(ctx context.Context, ws *WorkspaceResult, nodeID, nodeType string) (*policypb.NGACNode, error) {
	if nodeID == "" {
		return nil, fmt.Errorf("%w: node id required", ErrInvalidInput)
	}
	desc, err := s.policyWrite.GetDescendants(ctx, &policypb.GetDescendantsRequest{NodeId: ws.PcNodeID})
	if err != nil {
		return nil, fmt.Errorf("resolve workspace nodes: %w", err)
	}
	for _, n := range desc.GetNodes() {
		if n.Id == nodeID && n.NodeType == nodeType {
			return n, nil
		}
	}
	return nil, fmt.Errorf("%w: %s node not in this workspace", ErrNotFound, nodeType)
}

// isRole reports whether a UA is a role an administrator created, as opposed to
// a UA the platform builds (Owners, Members, tenant, department, channel,
// personal). Roles carry type=role (CreateRole; migration 023 for older ones).
func isRole(n *policypb.NGACNode) bool {
	return n.GetNodeType() == ngac.TypeUA && n.GetProperties()[ngac.PropType] == ngac.PropTypeRole
}

// requireRole confirms roleID is a role of this workspace. The roles API takes
// node IDs from the client, and a platform UA is a UA too: without this check,
// DeleteRole could delete the workspace's Owners UA and a role assignment could
// assign a member to it.
func (s *Service) requireRole(ctx context.Context, ws *WorkspaceResult, roleID string) error {
	n, err := s.nodeInWorkspace(ctx, ws, roleID, ngac.TypeUA)
	if err != nil {
		return err
	}
	if !isRole(n) {
		return fmt.Errorf("%w: role not in this workspace", ErrNotFound)
	}
	return nil
}
