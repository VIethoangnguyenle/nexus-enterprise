// Package domain provides business logic orchestration for the workspace service.
// It delegates to store for persistence and policy clients for NGAC graph operations.
package domain

import (
	"context"
	"fmt"
	"log/slog"

	"ngac-platform/ngac"
	"ngac-platform/pkg/realtime"
	policypb "ngac-platform/proto/policy"
)

// withdrawOffers revokes what was still open to a removed person's proved
// address in this workspace, so an old invitation cannot bring them back.
// Access is already gone; a failure here is logged, not returned.
func (s *Service) withdrawOffers(ctx context.Context, wsID, nodeID string) {
	if s.invitations == nil {
		return
	}
	email, err := s.invitations.VerifiedEmailByNode(ctx, nodeID)
	if err != nil || email == "" {
		if err != nil {
			slog.Warn("could not look up a removed member's address", "workspace", wsID, "error", err)
		}
		return
	}
	if _, err := s.invitations.RevokePendingForEmail(ctx, wsID, email, s.clock()); err != nil {
		slog.Warn("member removed but their invitations remain", "workspace", wsID, "error", err)
	}
}

// ownersOf returns the people in the workspace's Owners UA and the UA's ID.
func (s *Service) ownersOf(ctx context.Context, ws *WorkspaceResult) (string, []*policypb.NGACNode, error) {
	ownersUAID, err := s.FindUAByName(ctx, ws.ID, ngac.OwnersUAName(ngac.WorkspaceID(ws.ID)))
	if err != nil {
		return "", nil, err
	}
	kids, err := s.policyWrite.GetChildren(ctx, &policypb.GetChildrenRequest{NodeId: ownersUAID})
	if err != nil {
		return "", nil, fmt.Errorf("get owners: %w", err)
	}
	return ownersUAID, userNodes(kids.GetNodes()), nil
}

func containsNode(nodes []*policypb.NGACNode, id string) bool {
	for _, n := range nodes {
		if n.Id == id {
			return true
		}
	}
	return false
}

// requireOwner confirms the caller is in the Owners UA. Holding manage is not
// enough to change who the owners are: a role can carry manage, and a manager
// must not be able to demote or replace the people above them.
func requireOwner(owners []*policypb.NGACNode, callerNodeID string) error {
	if !containsNode(owners, callerNodeID) {
		return fmt.Errorf("%w: only an owner changes the owners", ErrAccessDenied)
	}
	return nil
}

// RemoveMember removes a user from all UAs under the workspace PC. The caller
// must hold invite on the workspace's Mgmt OA.
//
// Removing someone in the Owners UA takes being an Owner and holding manage, and
// the last owner stays. That rule is checked and acted on under the workspace's
// owner lock, so two removals at once cannot both pass it.
func (s *Service) RemoveMember(ctx context.Context, callerNodeID, wsID, targetNGACNodeID string) error {
	ws, err := s.authorizeAdmin(ctx, callerNodeID, wsID, ngac.OpInvite)
	if err != nil {
		return err
	}
	remove := func(ctx context.Context) error {
		_, owners, err := s.ownersOf(ctx, ws)
		if err != nil {
			return err
		}
		if containsNode(owners, targetNGACNodeID) {
			if err := requireOwner(owners, callerNodeID); err != nil {
				return err
			}
			mgmtID, err := s.mgmtOAID(ctx, ws)
			if err != nil {
				return err
			}
			if err := s.checkAccess(ctx, callerNodeID, mgmtID, ngac.OpManage); err != nil {
				return err
			}
			if len(owners) <= 1 {
				return fmt.Errorf("%w: cannot remove last owner", ErrInvalidInput)
			}
		}
		return s.detachMember(ctx, ws, targetNGACNodeID)
	}
	if err := s.store.WithOwnerLock(ctx, ws.ID, remove); err != nil {
		return err
	}
	s.withdrawOffers(ctx, ws.ID, targetNGACNodeID)
	if s.directory != nil {
		// Access is already gone; a listing left behind only keeps the person in
		// contacts, so a failure is reported but does not undo the removal.
		if err := s.directory.RemoveTenantUser(ctx, ws.ID, targetNGACNodeID); err != nil {
			slog.Warn("member removed but their listing remains", "workspace", ws.ID, "error", err)
		}
	}
	s.announce(ctx, realtime.KindMemberRemoved, ws.ID, targetNGACNodeID)
	s.announceAccessChanged(ctx, ws.ID, targetNGACNodeID)
	return nil
}

// detachMember removes a user from every UA under the workspace PC. It performs
// no authorization; callers must have authorized already.
func (s *Service) detachMember(ctx context.Context, ws *WorkspaceResult, targetNGACNodeID string) error {
	desc, err := s.policyWrite.GetDescendants(ctx, &policypb.GetDescendantsRequest{NodeId: ws.PcNodeID})
	if err != nil {
		return fmt.Errorf("get descendants: %w", err)
	}
	// Every UA the user is detached from is one revocation. A failure here
	// leaves the user assigned and therefore still authorized, so it has to
	// surface rather than be dropped — reporting "removed" while the graph
	// still grants access is the worst possible outcome.
	var failed []string
	for _, n := range desc.Nodes {
		if n.NodeType != ngac.TypeUA {
			continue
		}
		if _, err := s.policyWrite.RemoveAssignment(ctx, &policypb.RemoveAssignmentRequest{
			ChildId: targetNGACNodeID, ParentId: n.Id,
		}); err != nil {
			slog.Error("failed to detach user from UA during member removal",
				"user_node_id", targetNGACNodeID, "ua_id", n.Id, "error", err)
			failed = append(failed, n.Id)
		}
	}
	if len(failed) > 0 {
		return fmt.Errorf("member not fully removed, still assigned to %d attribute(s): %v", len(failed), failed)
	}
	return nil
}

// Member represents a workspace member.
type Member struct {
	NGACNodeID string
	Username   string
}

// ListMembers returns all unique users under the workspace PC. The caller must
// belong to the workspace.
func (s *Service) ListMembers(ctx context.Context, callerNodeID, wsID string) ([]*Member, error) {
	ws, err := s.authorizeMember(ctx, callerNodeID, wsID)
	if err != nil {
		return nil, err
	}
	desc, err := s.policyWrite.GetDescendants(ctx, &policypb.GetDescendantsRequest{NodeId: ws.PcNodeID})
	if err != nil {
		return nil, fmt.Errorf("get descendants: %w", err)
	}
	seen := make(map[string]bool)
	var members []*Member
	for _, n := range desc.Nodes {
		if n.NodeType == ngac.TypeU && !seen[n.Id] {
			seen[n.Id] = true
			members = append(members, &Member{NGACNodeID: n.Id, Username: ngac.DisplayName(n.Name, n.Properties)})
		}
	}
	return members, nil
}

// TransferOwnership adds a user to the Owners UA of a workspace. The caller
// must hold manage on the workspace's Mgmt OA and be an Owner, and the new owner
// must already belong to the workspace.
func (s *Service) TransferOwnership(ctx context.Context, callerNodeID, wsID, newOwnerNGACNodeID string) error {
	ws, err := s.authorizeAdmin(ctx, callerNodeID, wsID, ngac.OpManage)
	if err != nil {
		return err
	}
	add := func(ctx context.Context) error {
		ownersUAID, owners, err := s.ownersOf(ctx, ws)
		if err != nil {
			return err
		}
		if err := requireOwner(owners, callerNodeID); err != nil {
			return err
		}
		if err := s.requireMemberUser(ctx, ws, newOwnerNGACNodeID); err != nil {
			return err
		}
		if _, err := s.policyWrite.CreateAssignment(ctx, &policypb.CreateAssignmentRequest{
			ChildId: newOwnerNGACNodeID, ParentId: ownersUAID,
		}); err != nil {
			return fmt.Errorf("assign owner: %w", err)
		}
		return nil
	}
	if err := s.store.WithOwnerLock(ctx, ws.ID, add); err != nil {
		return err
	}
	s.announce(ctx, realtime.KindRoleChanged, ws.ID, newOwnerNGACNodeID)
	s.announceAccessChanged(ctx, ws.ID, newOwnerNGACNodeID)
	return nil
}

// RemoveOwner removes a user from the Owners UA, refusing if they are the last
// owner. The caller must hold manage on the workspace's Mgmt OA and be an Owner;
// the count and the removal happen under the workspace's owner lock.
func (s *Service) RemoveOwner(ctx context.Context, callerNodeID, wsID, targetNGACNodeID string) error {
	ws, err := s.authorizeAdmin(ctx, callerNodeID, wsID, ngac.OpManage)
	if err != nil {
		return err
	}
	remove := func(ctx context.Context) error {
		ownersUAID, owners, err := s.ownersOf(ctx, ws)
		if err != nil {
			return err
		}
		if err := requireOwner(owners, callerNodeID); err != nil {
			return err
		}
		if !containsNode(owners, targetNGACNodeID) {
			return fmt.Errorf("%w: not an owner of this workspace", ErrNotFound)
		}
		if len(owners) <= 1 {
			return fmt.Errorf("%w: cannot remove last owner", ErrInvalidInput)
		}
		if _, err := s.policyWrite.RemoveAssignment(ctx, &policypb.RemoveAssignmentRequest{
			ChildId: targetNGACNodeID, ParentId: ownersUAID,
		}); err != nil {
			return fmt.Errorf("remove assignment: %w", err)
		}
		return nil
	}
	if err := s.store.WithOwnerLock(ctx, ws.ID, remove); err != nil {
		return err
	}
	s.announce(ctx, realtime.KindRoleChanged, ws.ID, targetNGACNodeID)
	s.announceAccessChanged(ctx, ws.ID, targetNGACNodeID)
	return nil
}
