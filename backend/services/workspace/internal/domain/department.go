package domain

import (
	"context"
	"fmt"
	"log/slog"
	"slices"

	"github.com/google/uuid"

	"ngac-platform/ngac"
	"ngac-platform/pkg/provision"
	"ngac-platform/pkg/realtime"
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
	// Hidden is set only for a caller who administers the directory: the department
	// has been hidden from Danh bạ (its Dir sits directly under Directory).
	Hidden bool
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
	var out *DepartmentResult
	err := s.fresh(func() (err error) {
		out, err = s.createDepartment(ctx, callerNodeID, in)
		return err
	})
	return out, err
}

func (s *Service) createDepartment(ctx context.Context, callerNodeID string, in CreateDepartmentInput) (*DepartmentResult, error) {
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

	// The directory mirror is resolved before the first write: a lookup that fails
	// stops here with nothing to undo. A workspace without a directory, or a parent
	// without nodes, is skipped (migration 064 builds them).
	dir, hasDir, err := s.directoryNodes(ctx, in.WorkspaceID)
	if err != nil {
		return nil, err
	}
	var parentDir *deptDir
	if hasDir && parentDept != nil {
		var ok bool
		if parentDir, ok, err = s.deptDirNodes(ctx, in.WorkspaceID, parentDept.ID); err != nil {
			return nil, err
		} else if !ok {
			skipDept(in.WorkspaceID, parentDept.ID, "parent of a new department")
			hasDir = false
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
	if hasDir {
		if err := s.createDeptDirectory(ctx, prov, ngac.WorkspaceID(in.WorkspaceID), deptID, ws.PcNodeID, dir, parentDir); err != nil {
			return nil, prov.Fail(ctx, err)
		}
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
	s.announce(ctx, realtime.KindDepartmentChanged, in.WorkspaceID, deptID)

	slog.Info("department created", "dept_id", deptID, "name", in.Name, "workspace", in.WorkspaceID)

	return &DepartmentResult{
		ID:       deptID,
		Name:     in.Name,
		ParentID: in.ParentID,
		NGACUaID: node.Id,
	}, nil
}

// ListDepartments returns the departments of a workspace the caller may see, with member
// counts. The caller must belong to the workspace. A caller who administers the directory sees
// every department and is told which are hidden from Danh bạ; anyone else gets the departments
// their scope allows, and a department whose parent is not among them is a root in their list.
func (s *Service) ListDepartments(ctx context.Context, callerNodeID, wsID string) ([]*DepartmentResult, error) {
	ws, err := s.authorizeMember(ctx, callerNodeID, wsID)
	if err != nil {
		return nil, err
	}
	sc, err := s.scopeFor(ctx, ws, callerNodeID, false, true)
	if err != nil {
		return nil, err
	}

	var visible []*store.Department
	for _, d := range sc.depts {
		if sc.VisibleDeptIDs[d.ID] {
			visible = append(visible, d)
		}
	}
	// The people of a department are who is assigned to its UA in the graph, the same
	// source as the people table; a cached column would drift from it. Read once each.
	if err := s.loadDeptUsers(ctx, sc, visible); err != nil {
		return nil, fmt.Errorf("count department members: %w", err)
	}
	results := make([]*DepartmentResult, 0, len(visible))
	for _, d := range visible {
		parentID := ""
		if d.ParentID != nil && sc.VisibleDeptIDs[*d.ParentID] {
			parentID = *d.ParentID
		}
		results = append(results, &DepartmentResult{
			ID:          d.ID,
			Name:        d.Name,
			ParentID:    parentID,
			MemberCount: len(sc.deptUsers[d.ID]),
			NGACUaID:    d.NGACUaID,
			Hidden:      sc.Admin && sc.HiddenDeptIDs[d.ID],
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
	s.announce(ctx, realtime.KindDepartmentChanged, wsID, deptID)

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
	var out *DepartmentResult
	err := s.fresh(func() (err error) {
		out, err = s.moveDepartment(ctx, callerNodeID, in)
		return err
	})
	return out, err
}

func (s *Service) moveDepartment(ctx context.Context, callerNodeID string, in MoveDepartmentInput) (*DepartmentResult, error) {
	// Hidden or not is read while planning the directory move; a hide in between would be undone.
	defer s.lockDirectory(in.WorkspaceID)()
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

	// Where the department's directory nodes go, resolved before the first write.
	mirror, err := s.planDeptMirror(ctx, in.WorkspaceID, dept, newParent)
	if err != nil {
		return nil, err
	}

	// Update NGAC assignments
	// Remove old assignment
	oldParentNGACID := ws.PcNodeID
	var oldParentDept *store.Department
	if dept.ParentID != nil && *dept.ParentID != "" {
		if oldParent, err := s.deptStore.GetDepartment(ctx, *dept.ParentID); err == nil {
			oldParentNGACID = oldParent.NGACUaID
			oldParentDept = oldParent
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
	if err := s.moveDeptDirectory(ctx, mirror); err != nil {
		// Graph, directory and row stay one thing: the department goes back where it was.
		s.undoDepartmentMove(ctx, in.WorkspaceID, dept, oldParentNGACID, newParentNGACID, mirror, oldParentDept)
		return nil, err
	}

	// Update DB
	var newParentPtr *string
	if in.NewParentID != "" {
		newParentPtr = &in.NewParentID
	}
	if err := s.deptStore.MoveDepartment(ctx, in.DeptID, newParentPtr); err != nil {
		return nil, err
	}
	s.announce(ctx, realtime.KindDepartmentChanged, in.WorkspaceID, in.DeptID)

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
	return s.fresh(func() error { return s.deleteDepartment(ctx, callerNodeID, wsID, deptID) })
}

func (s *Service) deleteDepartment(ctx context.Context, callerNodeID, wsID, deptID string) error {
	ws, err := s.authorizeAdmin(ctx, callerNodeID, wsID, ngac.OpManage)
	if err != nil {
		return err
	}
	dept, err := s.departmentInWorkspace(ctx, wsID, deptID)
	if err != nil {
		return err
	}

	parentUA := ws.PcNodeID
	var parent *store.Department
	if dept.ParentID != nil {
		if parent, err = s.departmentInWorkspace(ctx, wsID, *dept.ParentID); err != nil {
			return err
		}
		parentUA = parent.NGACUaID
	}
	all, err := s.deptStore.ListDepartmentsByWorkspace(ctx, wsID)
	if err != nil {
		return err
	}
	// The directory's side of the same move, resolved before the first write.
	mirror, err := s.planDeptMirror(ctx, wsID, dept, parent)
	if err != nil {
		return err
	}
	// A hidden department is not deleted around the people placed directly in it: they would
	// land in a visible parent, or in none, and be published. They are moved out first.
	if mirror != nil && mirror.hidden {
		kids, err := s.policyWrite.GetChildren(ctx, &policypb.GetChildrenRequest{NodeId: dept.NGACUaID})
		if err != nil {
			return fmt.Errorf("%w: cannot tell who is in the department: %v", ErrUnavailable, err)
		}
		if len(userNodes(kids.GetNodes())) > 0 {
			return fmt.Errorf("%w: Chuyển những người trong phòng này sang phòng khác trước khi xoá", ErrInvalidInput)
		}
	}
	var subs []subDeptMirror
	if mirror != nil {
		if subs, err = s.planSubDepartments(ctx, wsID, deptID, mirror.dir, all); err != nil {
			return err
		}
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
			// They are the parent's own people now: its peers, not this department's.
			if mirror != nil && mirror.parent != nil {
				if _, err := s.policyWrite.CreateAssignment(ctx, &policypb.CreateAssignmentRequest{ChildId: u.Id, ParentId: mirror.parent.Peers}); err != nil {
					return fmt.Errorf("move member's peers edge up: %w", err)
				}
			}
		}
	}
	if mirror != nil {
		if err := s.reparentSubDepartments(ctx, mirror, subs); err != nil {
			return err
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

	// Its directory nodes go before the department's UA: a failure here stops the
	// delete with the department still findable, so it can be run again.
	if mirror != nil {
		if err := s.deleteDeptDirectory(ctx, wsID, deptID, mirror.dept); err != nil {
			return err
		}
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
	s.announce(ctx, realtime.KindDepartmentChanged, ws.ID, deptID)
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

	if err := s.applyDepartment(ctx, ws, userNGACNodeID, dept); err != nil {
		return err
	}
	s.announce(ctx, realtime.KindDepartmentChanged, ws.ID, deptID, userNGACNodeID)
	s.announceAccessChanged(ctx, ws.ID, userNGACNodeID)
	return nil
}

// applyDepartment puts a person in a department (nil: out of any), taking them
// out of the one they were in. It performs no authorization.
func (s *Service) applyDepartment(ctx context.Context, ws *WorkspaceResult, userNGACNodeID string, dept *store.Department) error {
	return s.fresh(func() error { return s.applyDepartmentOnce(ctx, ws, userNGACNodeID, dept) })
}

func (s *Service) applyDepartmentOnce(ctx context.Context, ws *WorkspaceResult, userNGACNodeID string, dept *store.Department) error {
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

	// The peers UA of the new department (empty: none to write) and the peers
	// UAs of any other department the person sits under, which must go with
	// their department edge. Resolved before the first write.
	peers, err := s.peersFor(ctx, wsID, dept)
	if err != nil {
		return err
	}
	stale := stalePeers(parents.GetNodes(), all, peers)
	hasPeers := false
	for _, p := range parents.GetNodes() {
		if p.GetId() == peers {
			hasPeers = true
		}
	}

	// New edges first, then the old ones: if leaving fails the person is not
	// left in no department, and the new edges are taken back.
	undo := func(parent string) {
		if _, err := s.policyWrite.RemoveAssignment(ctx, &policypb.RemoveAssignmentRequest{
			ChildId: userNGACNodeID, ParentId: parent,
		}); err != nil {
			slog.Error("could not undo a department move", "workspace", wsID, "error", err)
		}
	}
	var added []string
	undoAll := func() {
		for _, p := range added {
			undo(p)
		}
	}
	if dept != nil && !slices.Contains(current, dept.NGACUaID) {
		if _, err := s.policyWrite.CreateAssignment(ctx, &policypb.CreateAssignmentRequest{
			ChildId: userNGACNodeID, ParentId: dept.NGACUaID,
		}); err != nil {
			return fmt.Errorf("assign user to dept: %w", err)
		}
		added = append(added, dept.NGACUaID)
	}
	if peers != "" && !hasPeers {
		if _, err := s.policyWrite.CreateAssignment(ctx, &policypb.CreateAssignmentRequest{
			ChildId: userNGACNodeID, ParentId: peers,
		}); err != nil {
			undoAll()
			return fmt.Errorf("assign user to department peers: %w", err)
		}
		added = append(added, peers)
	}
	for _, old := range current {
		if dept != nil && old == dept.NGACUaID {
			continue
		}
		if _, err := s.policyWrite.RemoveAssignment(ctx, &policypb.RemoveAssignmentRequest{
			ChildId: userNGACNodeID, ParentId: old,
		}); err != nil {
			undoAll()
			return fmt.Errorf("leave previous department: %w", err)
		}
	}
	// Whatever peers edge they held in another department goes now. A failure here
	// is reported and not undone: the person is already where they were put, the
	// edge that is left is idempotent to remove, and running the same change again
	// finishes it.
	for _, old := range stale {
		if _, err := s.policyWrite.RemoveAssignment(ctx, &policypb.RemoveAssignmentRequest{
			ChildId: userNGACNodeID, ParentId: old,
		}); err != nil {
			return fmt.Errorf("leave previous department's peers: %w", err)
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

// undoDepartmentMove takes a move back after its directory nodes could not follow: the
// department's edge to the new parent goes, the one to the old parent returns, and the
// directory nodes are put back under the old parent's. It is best effort, and says so when a
// step fails: the move is retried by running it again.
func (s *Service) undoDepartmentMove(ctx context.Context, wsID string, dept *store.Department, oldParentUA, newParentUA string, mirror *deptMirror, oldParent *store.Department) {
	if _, err := s.policyWrite.RemoveAssignment(ctx, &policypb.RemoveAssignmentRequest{ChildId: dept.NGACUaID, ParentId: newParentUA}); err != nil {
		slog.Error("could not take a department move back", "workspace", wsID, "department", dept.ID, "error", err)
	}
	if _, err := s.policyWrite.CreateAssignment(ctx, &policypb.CreateAssignmentRequest{ChildId: dept.NGACUaID, ParentId: oldParentUA}); err != nil {
		slog.Error("could not take a department move back", "workspace", wsID, "department", dept.ID, "error", err)
	}
	if mirror == nil {
		return
	}
	back := *mirror
	back.parent = nil
	if oldParent != nil {
		p, ok, err := s.deptDirNodes(ctx, wsID, oldParent.ID)
		if err != nil || !ok {
			slog.Error("could not put a department's directory nodes back", "workspace", wsID, "department", dept.ID, "error", err)
			return
		}
		back.parent = p
	}
	if err := s.moveDeptDirectory(ctx, &back); err != nil {
		slog.Error("could not put a department's directory nodes back", "workspace", wsID, "department", dept.ID, "error", err)
	}
}
