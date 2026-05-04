package domain

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"ngac-platform/ngac"
	policypb "ngac-platform/proto/policy"
	"ngac-platform/services/workspace/internal/store"
)

// DepartmentStore defines persistence operations for departments.
type DepartmentStore interface {
	InsertDepartment(ctx context.Context, d *store.Department) error
	ListDepartmentsByWorkspace(ctx context.Context, wsID string) ([]*store.Department, error)
	GetDepartment(ctx context.Context, id string) (*store.Department, error)
	UpdateDepartmentName(ctx context.Context, id, name string) error
	MoveDepartment(ctx context.Context, id string, newParentID *string) error
	DeleteDepartment(ctx context.Context, id string) error
	UpdateUserDepartment(ctx context.Context, userID string, deptID *string) error
	CountMembersByDepartment(ctx context.Context, deptID string) (int, error)
	ReassignDepartmentChildren(ctx context.Context, oldParentID string, newParentID *string) error
	ReassignDepartmentUsers(ctx context.Context, oldDeptID string, newDeptID *string) error
}

// DepartmentResult is the domain output for department operations.
type DepartmentResult struct {
	ID          string
	Name        string
	ParentID    string
	MemberCount int
	NGACUaID    string
}

// CreateDepartmentInput holds parameters for creating a department.
type CreateDepartmentInput struct {
	WorkspaceID string
	Name        string
	ParentID    string // empty = root department
}

// CreateDepartment provisions a new department: NGAC UA node + DB row.
func (s *Service) CreateDepartment(ctx context.Context, in CreateDepartmentInput) (*DepartmentResult, error) {
	if in.Name == "" {
		return nil, fmt.Errorf("%w: department name required", ErrInvalidInput)
	}

	ws, err := s.GetWorkspace(ctx, in.WorkspaceID)
	if err != nil {
		return nil, err
	}

	// Create NGAC UA node for the department
	uaName := ngac.DeptUAName(in.Name)
	node, err := s.policyWrite.CreateNode(ctx, &policypb.CreateNodeRequest{
		Name:     uaName,
		NodeType: ngac.TypeUA,
		Properties: map[string]string{
			"workspace_id": in.WorkspaceID,
			"dept_name":    in.Name,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("create dept UA: %w", err)
	}

	// Assign dept UA to parent dept or workspace PC
	parentNGACID := ws.PcNodeID
	if in.ParentID != "" {
		parentDept, err := s.deptStore.GetDepartment(ctx, in.ParentID)
		if err != nil {
			return nil, fmt.Errorf("%w: parent department %s", ErrNotFound, in.ParentID)
		}
		parentNGACID = parentDept.NGACUaID
	}

	if _, err := s.policyWrite.CreateAssignment(ctx, &policypb.CreateAssignmentRequest{
		ChildId: node.Id, ParentId: parentNGACID,
	}); err != nil {
		return nil, fmt.Errorf("assign dept UA: %w", err)
	}

	// Persist to DB
	deptID := uuid.New().String()
	var parentPtr *string
	if in.ParentID != "" {
		parentPtr = &in.ParentID
	}

	if err := s.deptStore.InsertDepartment(ctx, &store.Department{
		ID:          deptID,
		WorkspaceID: in.WorkspaceID,
		Name:        in.Name,
		ParentID:    parentPtr,
		NGACUaID:    node.Id,
	}); err != nil {
		return nil, err
	}

	slog.Info("department created", "dept_id", deptID, "name", in.Name, "workspace", in.WorkspaceID)

	return &DepartmentResult{
		ID:       deptID,
		Name:     in.Name,
		ParentID: in.ParentID,
		NGACUaID: node.Id,
	}, nil
}

// ListDepartments returns all departments for a workspace with member counts.
func (s *Service) ListDepartments(ctx context.Context, wsID string) ([]*DepartmentResult, error) {
	deps, err := s.deptStore.ListDepartmentsByWorkspace(ctx, wsID)
	if err != nil {
		return nil, err
	}

	results := make([]*DepartmentResult, 0, len(deps))
	for _, d := range deps {
		count, _ := s.deptStore.CountMembersByDepartment(ctx, d.ID)
		parentID := ""
		if d.ParentID != nil {
			parentID = *d.ParentID
		}
		results = append(results, &DepartmentResult{
			ID:          d.ID,
			Name:        d.Name,
			ParentID:    parentID,
			MemberCount: count,
			NGACUaID:    d.NGACUaID,
		})
	}
	return results, nil
}

// UpdateDepartment renames a department and updates its NGAC node.
func (s *Service) UpdateDepartment(ctx context.Context, deptID, newName string) (*DepartmentResult, error) {
	if newName == "" {
		return nil, fmt.Errorf("%w: department name required", ErrInvalidInput)
	}

	dept, err := s.deptStore.GetDepartment(ctx, deptID)
	if err != nil {
		return nil, fmt.Errorf("%w: department %s", ErrNotFound, deptID)
	}

	if err := s.deptStore.UpdateDepartmentName(ctx, deptID, newName); err != nil {
		return nil, err
	}

	parentID := ""
	if dept.ParentID != nil {
		parentID = *dept.ParentID
	}

	return &DepartmentResult{
		ID:       deptID,
		Name:     newName,
		ParentID: parentID,
		NGACUaID: dept.NGACUaID,
	}, nil
}

// MoveDepartmentInput holds parameters for moving a department.
type MoveDepartmentInput struct {
	DeptID      string
	NewParentID string // empty = move to root
}

// MoveDepartment changes a department's parent. Prevents circular references.
func (s *Service) MoveDepartment(ctx context.Context, in MoveDepartmentInput) (*DepartmentResult, error) {
	dept, err := s.deptStore.GetDepartment(ctx, in.DeptID)
	if err != nil {
		return nil, fmt.Errorf("%w: department %s", ErrNotFound, in.DeptID)
	}

	// Prevent moving to self
	if in.NewParentID == in.DeptID {
		return nil, fmt.Errorf("%w: cannot move department to itself", ErrInvalidInput)
	}

	// Prevent circular: check that new parent is not a descendant of this dept
	if in.NewParentID != "" {
		allDepts, err := s.deptStore.ListDepartmentsByWorkspace(ctx, dept.WorkspaceID)
		if err != nil {
			return nil, err
		}
		if isDescendant(allDepts, in.DeptID, in.NewParentID) {
			return nil, fmt.Errorf("%w: circular department reference", ErrInvalidInput)
		}
	}

	// Update NGAC assignments
	ws, err := s.GetWorkspace(ctx, dept.WorkspaceID)
	if err != nil {
		return nil, err
	}

	// Remove old assignment
	oldParentNGACID := ws.PcNodeID
	if dept.ParentID != nil && *dept.ParentID != "" {
		if oldParent, err := s.deptStore.GetDepartment(ctx, *dept.ParentID); err == nil {
			oldParentNGACID = oldParent.NGACUaID
		}
	}
	s.policyWrite.RemoveAssignment(ctx, &policypb.RemoveAssignmentRequest{
		ChildId: dept.NGACUaID, ParentId: oldParentNGACID,
	})

	// Create new assignment
	newParentNGACID := ws.PcNodeID
	if in.NewParentID != "" {
		if newParent, err := s.deptStore.GetDepartment(ctx, in.NewParentID); err == nil {
			newParentNGACID = newParent.NGACUaID
		}
	}
	if _, err := s.policyWrite.CreateAssignment(ctx, &policypb.CreateAssignmentRequest{
		ChildId: dept.NGACUaID, ParentId: newParentNGACID,
	}); err != nil {
		return nil, fmt.Errorf("move dept assignment: %w", err)
	}

	// Update DB
	var newParentPtr *string
	if in.NewParentID != "" {
		newParentPtr = &in.NewParentID
	}
	if err := s.deptStore.MoveDepartment(ctx, in.DeptID, newParentPtr); err != nil {
		return nil, err
	}

	return &DepartmentResult{
		ID:       dept.ID,
		Name:     dept.Name,
		ParentID: in.NewParentID,
		NGACUaID: dept.NGACUaID,
	}, nil
}

// DeleteDepartment removes a department, reassigning children and users to parent.
func (s *Service) DeleteDepartment(ctx context.Context, deptID string) error {
	dept, err := s.deptStore.GetDepartment(ctx, deptID)
	if err != nil {
		return fmt.Errorf("%w: department %s", ErrNotFound, deptID)
	}

	// Reassign children to parent
	if err := s.deptStore.ReassignDepartmentChildren(ctx, deptID, dept.ParentID); err != nil {
		return err
	}

	// Reassign users to parent
	if err := s.deptStore.ReassignDepartmentUsers(ctx, deptID, dept.ParentID); err != nil {
		return err
	}

	// Remove NGAC node
	if _, err := s.policyWrite.DeleteNode(ctx, &policypb.DeleteNodeRequest{NodeId: dept.NGACUaID}); err != nil {
		slog.Warn("failed to delete dept NGAC node", "dept_id", deptID, "error", err)
	}

	// Remove DB row
	if err := s.deptStore.DeleteDepartment(ctx, deptID); err != nil {
		return err
	}

	slog.Info("department deleted", "dept_id", deptID, "name", dept.Name)
	return nil
}

// UpdateMemberDepartment assigns a user to a department.
func (s *Service) UpdateMemberDepartment(ctx context.Context, wsID, userNGACNodeID, deptID string) error {
	// If deptID is empty, unassign from department
	var deptPtr *string
	if deptID != "" {
		dept, err := s.deptStore.GetDepartment(ctx, deptID)
		if err != nil {
			return fmt.Errorf("%w: department %s", ErrNotFound, deptID)
		}

		// Create NGAC assignment: user → dept UA
		if _, err := s.policyWrite.CreateAssignment(ctx, &policypb.CreateAssignmentRequest{
			ChildId: userNGACNodeID, ParentId: dept.NGACUaID,
		}); err != nil {
			return fmt.Errorf("assign user to dept: %w", err)
		}
		deptPtr = &deptID
	}

	// Update DB — find user_id from ngac_node_id via the workspace members
	// For now, update by ngac_node_id lookup
	if err := s.deptStore.UpdateUserDepartment(ctx, userNGACNodeID, deptPtr); err != nil {
		return err
	}

	return nil
}

// isDescendant checks if targetID is a descendant of parentID in the department tree.
func isDescendant(depts []*store.Department, parentID, targetID string) bool {
	childMap := make(map[string][]string)
	for _, d := range depts {
		if d.ParentID != nil {
			childMap[*d.ParentID] = append(childMap[*d.ParentID], d.ID)
		}
	}

	// BFS from parentID
	queue := childMap[parentID]
	visited := make(map[string]bool)
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		if current == targetID {
			return true
		}
		if !visited[current] {
			visited[current] = true
			queue = append(queue, childMap[current]...)
		}
	}
	return false
}
