package domain

import (
	"context"
	"fmt"
	"log/slog"
	"slices"

	"github.com/google/uuid"

	"ngac-platform/ngac"
	"ngac-platform/pkg/provision"
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
	UpdateUserDepartment(ctx context.Context, tenantID, nodeID string, deptID *string) error
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

// CreateDepartment provisions a new department: NGAC UA node + DB row. The
// caller must hold manage on the workspace's Mgmt OA; a parent, if given, must
// be a department of the same workspace.
func (s *Service) CreateDepartment(ctx context.Context, callerNodeID string, in CreateDepartmentInput) (*DepartmentResult, error) {
	if in.Name == "" {
		return nil, fmt.Errorf("%w: department name required", ErrInvalidInput)
	}

	ws, err := s.authorizeAdmin(ctx, callerNodeID, in.WorkspaceID, ngac.OpManage)
	if err != nil {
		return nil, err
	}

	var parentDept *store.Department
	if in.ParentID != "" {
		parentDept, err = s.departmentInWorkspace(ctx, in.WorkspaceID, in.ParentID)
		if err != nil {
			return nil, err
		}
	}

	// The department's ID is chosen first so the node can be named by it: a
	// name taken from input would be shared by two tenants who both have a
	// "Sales", and the graph resolves nodes by exact name. The display name rides
	// along as a property.
	deptID := uuid.New().String()
	prov := provision.NewCreator(s.policyWrite)
	node, err := prov.Node(ctx, &policypb.CreateNodeRequest{
		Name:     ngac.DeptUAName(ngac.DeptID(deptID)),
		NodeType: ngac.TypeUA,
		Properties: map[string]string{
			"workspace_id":       in.WorkspaceID,
			"dept_name":          in.Name,
			ngac.PropDisplayName: in.Name,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("create dept UA: %w", err)
	}

	// Assign dept UA to parent dept or workspace PC
	parentNGACID := ws.PcNodeID
	if parentDept != nil {
		parentNGACID = parentDept.NGACUaID
	}
	if err := prov.Assign(ctx, node.Id, parentNGACID); err != nil {
		return nil, prov.Fail(ctx, fmt.Errorf("assign dept UA: %w", err))
	}

	// Persist to DB
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
		return nil, prov.Fail(ctx, err)
	}
	prov.Done()

	slog.Info("department created", "dept_id", deptID, "name", in.Name, "workspace", in.WorkspaceID)

	return &DepartmentResult{
		ID:       deptID,
		Name:     in.Name,
		ParentID: in.ParentID,
		NGACUaID: node.Id,
	}, nil
}

// ListDepartments returns all departments for a workspace with member counts.
// The caller must belong to the workspace.
func (s *Service) ListDepartments(ctx context.Context, callerNodeID, wsID string) ([]*DepartmentResult, error) {
	if _, err := s.authorizeMember(ctx, callerNodeID, wsID); err != nil {
		return nil, err
	}
	deps, err := s.deptStore.ListDepartmentsByWorkspace(ctx, wsID)
	if err != nil {
		return nil, err
	}

	results := make([]*DepartmentResult, 0, len(deps))
	for _, d := range deps {
		// The people of a department are who is assigned to its UA in the graph,
		// the same source as the people table; a cached column would drift from it.
		kids, err := s.policyWrite.GetChildren(ctx, &policypb.GetChildrenRequest{NodeId: d.NGACUaID})
		if err != nil {
			return nil, fmt.Errorf("count department members: %w", err)
		}
		count := len(uniqueNodes(userNodes(kids.GetNodes())))
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

// UpdateDepartment renames a department. The caller must hold manage on the
// workspace's Mgmt OA, and the department must belong to that workspace.
func (s *Service) UpdateDepartment(ctx context.Context, callerNodeID, wsID, deptID, newName string) (*DepartmentResult, error) {
	if newName == "" {
		return nil, fmt.Errorf("%w: department name required", ErrInvalidInput)
	}

	if _, err := s.authorizeAdmin(ctx, callerNodeID, wsID, ngac.OpManage); err != nil {
		return nil, err
	}
	dept, err := s.departmentInWorkspace(ctx, wsID, deptID)
	if err != nil {
		return nil, err
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
	WorkspaceID string
	DeptID      string
	NewParentID string // empty = move to root
}

// MoveDepartment changes a department's parent. Prevents circular references.
// The caller must hold manage on the workspace's Mgmt OA, and both the
// department and its new parent must belong to that workspace.
func (s *Service) MoveDepartment(ctx context.Context, callerNodeID string, in MoveDepartmentInput) (*DepartmentResult, error) {
	ws, err := s.authorizeAdmin(ctx, callerNodeID, in.WorkspaceID, ngac.OpManage)
	if err != nil {
		return nil, err
	}
	dept, err := s.departmentInWorkspace(ctx, in.WorkspaceID, in.DeptID)
	if err != nil {
		return nil, err
	}
	var newParent *store.Department
	if in.NewParentID != "" {
		newParent, err = s.departmentInWorkspace(ctx, in.WorkspaceID, in.NewParentID)
		if err != nil {
			return nil, err
		}
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

	// Moving a department under a parent gives everyone in it, and under it,
	// everything the parent and its ancestors grant: a delegation, so the caller
	// must hold all of it. Moving to the root grants nothing.
	if newParent != nil {
		if err := s.guardDelegation(ctx, callerNodeID, newParent.NGACUaID); err != nil {
			return nil, err
		}
	}

	// Update NGAC assignments
	// Remove old assignment
	oldParentNGACID := ws.PcNodeID
	if dept.ParentID != nil && *dept.ParentID != "" {
		if oldParent, err := s.deptStore.GetDepartment(ctx, *dept.ParentID); err == nil {
			oldParentNGACID = oldParent.NGACUaID
		}
	}
	// If the old edge survives, the department hangs under both parents and
	// inherits from both — a move would silently become an addition.
	if _, err := s.policyWrite.RemoveAssignment(ctx, &policypb.RemoveAssignmentRequest{
		ChildId: dept.NGACUaID, ParentId: oldParentNGACID,
	}); err != nil {
		return nil, fmt.Errorf("detach department from old parent: %w", err)
	}

	// Create new assignment
	newParentNGACID := ws.PcNodeID
	if newParent != nil {
		newParentNGACID = newParent.NGACUaID
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

// DeleteDepartment removes a department, reassigning children and users to
// parent. The caller must hold manage on the workspace's Mgmt OA, and the
// department must belong to that workspace.
//
// The graph is moved first: a child department's UA hangs under this one and a
// person may be assigned to it, and deleting the node takes those edges with it.
// Without re-parenting them, a child department would lose its way to the
// workspace's PC and the people here would silently have no department while the
// table said otherwise. Moving them up grants nothing new: they already reached
// the parent's grants through this department.
func (s *Service) DeleteDepartment(ctx context.Context, callerNodeID, wsID, deptID string) error {
	ws, err := s.authorizeAdmin(ctx, callerNodeID, wsID, ngac.OpManage)
	if err != nil {
		return err
	}
	dept, err := s.departmentInWorkspace(ctx, wsID, deptID)
	if err != nil {
		return err
	}

	parentUA := ws.PcNodeID
	if dept.ParentID != nil {
		parent, err := s.departmentInWorkspace(ctx, wsID, *dept.ParentID)
		if err != nil {
			return err
		}
		parentUA = parent.NGACUaID
	}
	all, err := s.deptStore.ListDepartmentsByWorkspace(ctx, wsID)
	if err != nil {
		return err
	}
	for _, d := range all {
		if d.ParentID != nil && *d.ParentID == deptID {
			if _, err := s.policyWrite.CreateAssignment(ctx, &policypb.CreateAssignmentRequest{ChildId: d.NGACUaID, ParentId: parentUA}); err != nil {
				return fmt.Errorf("move sub-department up: %w", err)
			}
		}
	}
	if dept.ParentID != nil {
		kids, err := s.policyWrite.GetChildren(ctx, &policypb.GetChildrenRequest{NodeId: dept.NGACUaID})
		if err != nil {
			return fmt.Errorf("get department members: %w", err)
		}
		for _, u := range userNodes(kids.GetNodes()) {
			if _, err := s.policyWrite.CreateAssignment(ctx, &policypb.CreateAssignmentRequest{ChildId: u.Id, ParentId: parentUA}); err != nil {
				return fmt.Errorf("move member up: %w", err)
			}
		}
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

// UpdateMemberDepartment puts a member in a department, taking them out of the
// one they were in, or, with an empty deptID, out of any. The caller must hold
// manage on the workspace's Mgmt OA, the department must belong to that
// workspace, the person must already belong to it, and the caller must hold
// whatever the department confers.
func (s *Service) UpdateMemberDepartment(ctx context.Context, callerNodeID, wsID, userNGACNodeID, deptID string) error {
	ws, err := s.authorizeAdmin(ctx, callerNodeID, wsID, ngac.OpManage)
	if err != nil {
		return err
	}
	var dept *store.Department
	if deptID != "" {
		if dept, err = s.departmentInWorkspace(ctx, wsID, deptID); err != nil {
			return err
		}
	}
	if err := s.requireMemberUser(ctx, ws, userNGACNodeID); err != nil {
		return err
	}
	if dept != nil {
		if err := s.guardDelegation(ctx, callerNodeID, dept.NGACUaID); err != nil {
			return err
		}
	}

	return s.applyDepartment(ctx, ws, userNGACNodeID, dept)
}

// applyDepartment puts a person in a department (nil: out of any), taking them
// out of the one they were in. It performs no authorization.
func (s *Service) applyDepartment(ctx context.Context, ws *WorkspaceResult, userNGACNodeID string, dept *store.Department) error {
	wsID := ws.ID
	deptID := ""
	if dept != nil {
		deptID = dept.ID
	}
	// Which department UAs the person sits directly under now.
	all, err := s.deptStore.ListDepartmentsByWorkspace(ctx, wsID)
	if err != nil {
		return err
	}
	isDept := make(map[string]bool, len(all))
	for _, d := range all {
		isDept[d.NGACUaID] = true
	}
	parents, err := s.policyWrite.GetParents(ctx, &policypb.GetParentsRequest{NodeId: userNGACNodeID})
	if err != nil {
		return fmt.Errorf("get current department: %w", err)
	}
	var current []string
	for _, p := range parents.GetNodes() {
		if isDept[p.Id] {
			current = append(current, p.Id)
		}
	}

	// New edge first, then the old ones: if leaving fails the person is not
	// left in no department, and the new edge is taken back.
	joined := false
	if dept != nil && !slices.Contains(current, dept.NGACUaID) {
		if _, err := s.policyWrite.CreateAssignment(ctx, &policypb.CreateAssignmentRequest{
			ChildId: userNGACNodeID, ParentId: dept.NGACUaID,
		}); err != nil {
			return fmt.Errorf("assign user to dept: %w", err)
		}
		joined = true
	}
	for _, old := range current {
		if dept != nil && old == dept.NGACUaID {
			continue
		}
		if _, err := s.policyWrite.RemoveAssignment(ctx, &policypb.RemoveAssignmentRequest{
			ChildId: userNGACNodeID, ParentId: old,
		}); err != nil {
			if joined {
				if _, undo := s.policyWrite.RemoveAssignment(ctx, &policypb.RemoveAssignmentRequest{
					ChildId: userNGACNodeID, ParentId: dept.NGACUaID,
				}); undo != nil {
					slog.Error("could not undo a department move", "workspace", wsID, "error", undo)
				}
			}
			return fmt.Errorf("leave previous department: %w", err)
		}
	}

	// The row is a cache of the graph; it follows the graph.
	var deptPtr *string
	if dept != nil {
		deptPtr = &deptID
	}
	return s.deptStore.UpdateUserDepartment(ctx, wsID, userNGACNodeID, deptPtr)
}

// departmentInWorkspace loads a department and confirms it belongs to wsID.
// A department of another workspace is reported as not found, so a caller who
// administers one workspace cannot act on another's departments by ID.
func (s *Service) departmentInWorkspace(ctx context.Context, wsID, deptID string) (*store.Department, error) {
	dept, err := s.deptStore.GetDepartment(ctx, deptID)
	if err != nil || dept == nil || dept.WorkspaceID != wsID {
		return nil, fmt.Errorf("%w: department %s", ErrNotFound, deptID)
	}
	return dept, nil
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
