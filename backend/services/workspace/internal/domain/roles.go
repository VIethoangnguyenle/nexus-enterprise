// Package domain provides business logic orchestration for the workspace service.
// It delegates to store for persistence and policy clients for NGAC graph operations.
package domain

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"ngac-platform/ngac"
	"ngac-platform/pkg/provision"
	"ngac-platform/pkg/realtime"
	policypb "ngac-platform/proto/policy"
)

// Role represents an NGAC role (UA) in a workspace.
type Role struct {
	ID         string
	Name       string
	NGACNodeID string
}

// CreateRole provisions a new UA role under the workspace PC. The caller must
// hold manage on the workspace's Mgmt OA.
//
// The node is named by a generated ID; the name the administrator typed is kept
// as its display_name property. Node names are matched exactly, so a name taken
// from input would let a role pose as a node the platform builds itself.
func (s *Service) CreateRole(ctx context.Context, callerNodeID, wsID, roleName string) (*Role, error) {
	ws, err := s.authorizeAdmin(ctx, callerNodeID, wsID, ngac.OpManage)
	if err != nil {
		return nil, err
	}
	if err := ngac.ValidateRoleName(roleName); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}
	prov := provision.NewCreator(s.policyWrite)
	node, err := prov.Node(ctx, &policypb.CreateNodeRequest{
		Name:     ngac.RoleUAName(ngac.RoleID(uuid.New().String())),
		NodeType: ngac.TypeUA,
		Properties: map[string]string{
			ngac.PropType:        ngac.PropTypeRole,
			ngac.PropDisplayName: roleName,
			"workspace_id":       wsID,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("create role: %w", err)
	}
	if err := prov.Assign(ctx, node.Id, ws.PcNodeID); err != nil {
		return nil, prov.Fail(ctx, fmt.Errorf("assign role: %w", err))
	}
	prov.Done()
	s.announce(ctx, realtime.KindRoleChanged, ws.ID, node.Id)
	return &Role{ID: node.Id, Name: roleName, NGACNodeID: node.Id}, nil
}

// ListRoles returns all UA roles under the workspace PC. The caller must belong
// to the workspace.
func (s *Service) ListRoles(ctx context.Context, callerNodeID, wsID string) ([]*Role, error) {
	ws, err := s.authorizeMember(ctx, callerNodeID, wsID)
	if err != nil {
		return nil, err
	}
	children, err := s.policyWrite.GetChildren(ctx, &policypb.GetChildrenRequest{NodeId: ws.PcNodeID})
	if err != nil {
		return nil, fmt.Errorf("get children: %w", err)
	}
	var roles []*Role
	for _, n := range children.Nodes {
		if isRole(n) {
			roles = append(roles, &Role{ID: n.Id, Name: ngac.DisplayName(n.Name, n.Properties), NGACNodeID: n.Id})
		}
	}
	return roles, nil
}

// DeleteRole removes a role (NGAC UA node). The caller must hold manage on the
// workspace's Mgmt OA, and the role must be a UA of this workspace.
func (s *Service) DeleteRole(ctx context.Context, callerNodeID, wsID, roleID string) error {
	ws, err := s.authorizeAdmin(ctx, callerNodeID, wsID, ngac.OpManage)
	if err != nil {
		return err
	}
	if err := s.requireRole(ctx, ws, roleID); err != nil {
		return err
	}
	if _, err := s.policyWrite.DeleteNode(ctx, &policypb.DeleteNodeRequest{NodeId: roleID}); err != nil {
		return fmt.Errorf("delete role: %w", err)
	}
	s.announce(ctx, realtime.KindRoleChanged, ws.ID, roleID)
	return nil
}
