package domain

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"sort"
	"strings"
	"sync"

	"ngac-platform/ngac"
	"ngac-platform/pkg/policyclient"
	"ngac-platform/pkg/realtime"
	policypb "ngac-platform/proto/policy"
	"ngac-platform/services/workspace/internal/store"
)

// Who may see whom in Danh bạ (docs/specs/contacts-visibility). The scope is built from
// the membership check and policy decisions alone: the caller must be a member (graph,
// through the writer), then the administrator check and one batch of `read` decisions on the
// directory OAs say what is visible. Nothing here decides anything itself; a decision that
// cannot be had is an outage (ErrUnavailable), never a wider view.

// RoleHolders is a role and the people who hold it directly.
type RoleHolders struct {
	ID            string
	Name          string // empty for the owners role
	Kind          RoleKind
	HolderNodeIDs []string
}

// DirectoryScope is what one member may see of the workspace's people.
type DirectoryScope struct {
	// Ready is false for a workspace with no directory nodes (it predates migration 064):
	// no rule can exist there, so everything below describes "everyone is visible".
	Ready bool
	// Admin: the caller holds manage on the directory and sees everyone.
	Admin bool
	// Limited: the caller is limited to their own department. Display only; the policy decides.
	Limited        bool
	VisibleDeptIDs map[string]bool
	NoDeptVisible  bool
	OwnersVisible  bool
	OwnerNodeIDs   []string
	// HiddenDeptIDs lists the departments hidden from the directory; set for an administrator only.
	HiddenDeptIDs map[string]bool
	// Roles are the owners group and the custom roles with who holds each directly (never
	// Members). Filled by DirectoryScope, not by the reads that only need the visible set.
	Roles []RoleHolders

	depts []*store.Department
	// deptUsers caches who sits directly in each department, read once per request.
	deptUsers map[string][]*policypb.NGACNode
}

// RuleSubject is a department or role named by a directory rule.
type RuleSubject struct {
	ID          string
	Name        string
	ParentID    string // departments only
	MemberCount int
}

// DirectoryRules is what an administrator has set. Ready is false for a workspace that
// cannot carry rules yet (no directory nodes, or a personal workspace).
type DirectoryRules struct {
	Ready              bool
	HiddenDepartments  []RuleSubject
	LimitedRoles       []RuleSubject
	LimitedDepartments []RuleSubject
}

// DirectoryScope computes what the caller may see of the workspace's people. The caller must
// be a member; the answer is for that caller alone.
func (s *Service) DirectoryScope(ctx context.Context, callerNodeID, wsID string) (*DirectoryScope, error) {
	ws, err := s.authorizeMember(ctx, callerNodeID, wsID)
	if err != nil {
		return nil, err
	}
	return s.scopeFor(ctx, ws, callerNodeID, true, true)
}

// scopeFor builds the scope of a caller already known to be a member. withRoles adds the
// roles and their holders; withDepts makes VisibleDeptIDs list the departments even when
// everyone is visible (the reads that only filter people have no use for the list then).
func (s *Service) scopeFor(ctx context.Context, ws *WorkspaceResult, caller string, withRoles, withDepts bool) (*DirectoryScope, error) {
	dir, has, err := s.directoryNodes(ctx, ws.ID)
	if err != nil {
		return nil, err
	}
	sc := &DirectoryScope{Ready: has, VisibleDeptIDs: map[string]bool{}, HiddenDeptIDs: map[string]bool{}}

	loadDepts := func() error {
		if sc.depts != nil {
			return nil
		}
		depts, err := s.deptStore.ListDepartmentsByWorkspace(ctx, ws.ID)
		if err != nil {
			return fmt.Errorf("list departments: %w", err)
		}
		sc.depts = append([]*store.Department{}, depts...)
		return nil
	}
	everyone := func() error {
		sc.NoDeptVisible, sc.OwnersVisible = true, true
		if !withDepts {
			return nil
		}
		if err := loadDepts(); err != nil {
			return err
		}
		for _, d := range sc.depts {
			sc.VisibleDeptIDs[d.ID] = true
		}
		return nil
	}
	if !has {
		if err := everyone(); err != nil {
			return nil, err
		}
	} else {
		admin, err := s.isDirectoryAdmin(ctx, caller, dir)
		if err != nil {
			return nil, err
		}
		if admin {
			sc.Admin = true
			if err := everyone(); err != nil {
				return nil, err
			}
			if withDepts {
				if sc.HiddenDeptIDs, err = s.hiddenDepartments(ctx, ws.ID, dir, sc.depts); err != nil {
					return nil, err
				}
			}
		} else {
			if err := loadDepts(); err != nil {
				return nil, err
			}
			if err := s.decideScope(ctx, ws.ID, caller, dir, sc); err != nil {
				return nil, err
			}
		}
	}

	ownersID, owners, err := s.ownersOf(ctx, ws)
	if err != nil {
		return nil, err
	}
	for _, o := range owners {
		sc.OwnerNodeIDs = append(sc.OwnerNodeIDs, o.Id)
	}
	if withRoles {
		if sc.Roles, err = s.scopeRoles(ctx, ws, ownersID, owners); err != nil {
			return nil, err
		}
		// Who holds a role is told only among the people the caller may see: the one
		// rule, applied here, so no caller has to re-implement it.
		set, err := s.visibleSet(ctx, sc, caller, func() ([]*policypb.NGACNode, error) { return s.memberUsers(ctx, ws) })
		if err != nil {
			return nil, err
		}
		if set != nil {
			for i := range sc.Roles {
				sc.Roles[i].HolderNodeIDs = slices.DeleteFunc(sc.Roles[i].HolderNodeIDs, func(id string) bool { return !set[id] })
			}
		}
	}
	return sc, nil
}

// isDirectoryAdmin asks whether the caller holds manage on the directory root. An answer that
// cannot be had is an outage, not a refusal: the caller would be shown less, but a scope built
// on a guess is not served.
func (s *Service) isDirectoryAdmin(ctx context.Context, caller string, dir *dirNodes) (bool, error) {
	ok, err := policyclient.New(s.policyRead).Check(ctx, caller, dir.Directory, ngac.OpManage)
	if err != nil {
		return false, fmt.Errorf("%w: directory administrator check: %v", ErrUnavailable, err)
	}
	return ok, nil
}

// decideScope fills the scope of a caller who does not administer the directory from one batch
// of `read` decisions on the directory OAs, and marks them limited when they are.
func (s *Service) decideScope(ctx context.Context, wsID, caller string, dir *dirNodes, sc *DirectoryScope) error {
	byName, err := s.directoryTree(ctx, wsID, dir)
	if err != nil {
		return err
	}
	type pair struct{ dir, peer string }
	pairs := make(map[string]pair, len(sc.depts))
	objects := []string{dir.NoDept, dir.Owners}
	for _, d := range sc.depts {
		p := pair{
			dir:  byName[ngac.DeptDirOAName(ngac.DeptID(d.ID))],
			peer: byName[ngac.DeptPeerOAName(ngac.DeptID(d.ID))],
		}
		if p.dir == "" && p.peer == "" {
			// Not shown, not guessed: a member never sees a department the directory does
			// not know. Whatever creates or moves departments builds these nodes.
			slog.Error("department has no directory nodes; it is hidden from members until they exist", "workspace", wsID, "department", d.ID)
			continue
		}
		pairs[d.ID] = p
		for _, id := range []string{p.dir, p.peer} {
			if id != "" {
				objects = append(objects, id)
			}
		}
	}
	perms, err := policyclient.New(s.policyRead).BatchCheck(ctx, caller, objects, []string{ngac.OpRead})
	if err != nil {
		return fmt.Errorf("%w: directory decisions: %v", ErrUnavailable, err)
	}
	for id, p := range pairs {
		if (p.dir != "" && perms.Has(p.dir, ngac.OpRead)) || (p.peer != "" && perms.Has(p.peer, ngac.OpRead)) {
			sc.VisibleDeptIDs[id] = true
		}
	}
	sc.NoDeptVisible = perms.Has(dir.NoDept, ngac.OpRead)
	sc.OwnersVisible = perms.Has(dir.Owners, ngac.OpRead)

	limited, err := s.isLimited(ctx, wsID, caller, dir)
	if err != nil {
		return err
	}
	sc.Limited = limited
	return nil
}

// directoryTree maps the name of every OA beneath Directory to its id. A node is taken only
// with the directory mark of this workspace, as directoryNode does.
func (s *Service) directoryTree(ctx context.Context, wsID string, dir *dirNodes) (map[string]string, error) {
	desc, err := s.policyWrite.GetDescendants(ctx, &policypb.GetDescendantsRequest{NodeId: dir.Directory})
	if err != nil {
		return nil, fmt.Errorf("%w: read the directory: %v", ErrUnavailable, err)
	}
	out := make(map[string]string, len(desc.GetNodes()))
	for _, n := range desc.GetNodes() {
		props := n.GetProperties()
		if n.GetNodeType() == ngac.TypeOA && props[ngac.PropType] == ngac.PropTypeDirectory && props["workspace_id"] == wsID {
			out[n.GetName()] = n.GetId()
		}
	}
	return out, nil
}

// isLimited reports whether the caller sits beneath DirectoryLimited. A limited caller with no
// prohibition behind the limit would be shown everyone, so that is an outage.
func (s *Service) isLimited(ctx context.Context, wsID, caller string, dir *dirNodes) (bool, error) {
	anc, err := s.policyWrite.GetAncestors(ctx, &policypb.GetAncestorsRequest{NodeId: caller})
	if err != nil {
		return false, fmt.Errorf("%w: read what the caller belongs to: %v", ErrUnavailable, err)
	}
	if !slices.ContainsFunc(anc.GetNodes(), func(n *policypb.NGACNode) bool { return n.GetId() == dir.Limited }) {
		return false, nil
	}
	rules, err := s.policyRead.ListProhibitions(ctx, &policypb.ListProhibitionsRequest{SubjectId: dir.Limited})
	if err != nil {
		return false, fmt.Errorf("%w: read the directory rule: %v", ErrUnavailable, err)
	}
	for _, r := range rules.GetProhibitions() {
		if r.GetSubjectId() == dir.Limited && r.GetName() == ngac.DirectoryLimitedProhibitionName(ngac.WorkspaceID(wsID)) {
			return true, nil
		}
	}
	return false, fmt.Errorf("%w: the directory rule behind a limited caller is missing", ErrUnavailable)
}

// hiddenDepartments lists the departments hidden from the directory: those whose Dir sits
// directly under Directory.
func (s *Service) hiddenDepartments(ctx context.Context, wsID string, dir *dirNodes, depts []*store.Department) (map[string]bool, error) {
	kids, err := s.policyWrite.GetChildren(ctx, &policypb.GetChildrenRequest{NodeId: dir.Directory})
	if err != nil {
		return nil, fmt.Errorf("%w: read the directory: %v", ErrUnavailable, err)
	}
	under := make(map[string]bool, len(kids.GetNodes()))
	for _, n := range kids.GetNodes() {
		under[n.GetName()] = true
	}
	out := map[string]bool{}
	for _, d := range depts {
		if under[ngac.DeptDirOAName(ngac.DeptID(d.ID))] {
			out[d.ID] = true
		}
	}
	return out, nil
}

// scopeRoles lists the owners group and the custom roles with their direct holders.
func (s *Service) scopeRoles(ctx context.Context, ws *WorkspaceResult, ownersID string, owners []*policypb.NGACNode) ([]RoleHolders, error) {
	children, err := s.policyWrite.GetChildren(ctx, &policypb.GetChildrenRequest{NodeId: ws.PcNodeID})
	if err != nil {
		return nil, fmt.Errorf("get children: %w", err)
	}
	var custom []RoleHolders
	for _, n := range children.GetNodes() {
		kind, ok := classifyRole(n, ws.ID)
		if !ok || kind == RoleMembers || n.GetId() == ownersID {
			continue
		}
		holders, err := s.roleHolders(ctx, kind, n.GetId())
		if err != nil {
			return nil, err
		}
		custom = append(custom, RoleHolders{ID: n.GetId(), Name: ngac.DisplayName(n.GetName(), n.GetProperties()), Kind: kind, HolderNodeIDs: nodeIDs(holders)})
	}
	sort.SliceStable(custom, func(i, j int) bool { return strings.ToLower(custom[i].Name) < strings.ToLower(custom[j].Name) })
	return append([]RoleHolders{{ID: ownersID, Kind: RoleOwners, HolderNodeIDs: nodeIDs(owners)}}, custom...), nil
}

func nodeIDs(nodes []*policypb.NGACNode) []string {
	out := make([]string, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, n.GetId())
	}
	return out
}

// deptReadParallelism bounds how many department membership reads one request has in flight.
const deptReadParallelism = 8

// loadDeptUsers reads who sits directly in each of the given departments that has not been
// read for this request yet: one read per department, a few at a time, never twice.
func (s *Service) loadDeptUsers(ctx context.Context, sc *DirectoryScope, depts []*store.Department) error {
	if sc.deptUsers == nil {
		sc.deptUsers = map[string][]*policypb.NGACNode{}
	}
	var todo []*store.Department
	for _, d := range depts {
		if _, ok := sc.deptUsers[d.ID]; !ok {
			todo = append(todo, d)
		}
	}
	var (
		mu       sync.Mutex
		wg       sync.WaitGroup
		firstErr error
		slots    = make(chan struct{}, deptReadParallelism)
	)
	for _, d := range todo {
		slots <- struct{}{}
		wg.Add(1)
		go func() {
			defer func() { <-slots; wg.Done() }()
			kids, err := s.policyWrite.GetChildren(ctx, &policypb.GetChildrenRequest{NodeId: d.NGACUaID})
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				if firstErr == nil {
					firstErr = fmt.Errorf("get department members: %w", err)
				}
				return
			}
			sc.deptUsers[d.ID] = uniqueNodes(userNodes(kids.GetNodes()))
		}()
	}
	wg.Wait()
	return firstErr
}

// visibleSet is the set of user nodes among the workspace's people that the caller may see:
// the people of the departments in the scope, those with no department when that is visible,
// the owners when they are, and the caller. nil means everyone (an administrator, or a
// workspace with no rules). people is read only when people with no department are visible.
func (s *Service) visibleSet(ctx context.Context, sc *DirectoryScope, caller string, people func() ([]*policypb.NGACNode, error)) (map[string]bool, error) {
	if sc.Admin || !sc.Ready {
		return nil, nil
	}
	set := map[string]bool{caller: true}
	if sc.OwnersVisible {
		for _, id := range sc.OwnerNodeIDs {
			set[id] = true
		}
	}
	// Those with no department are everyone not in any department, so every department is
	// read then; otherwise only the visible ones.
	want := sc.depts
	if !sc.NoDeptVisible {
		want = nil
		for _, d := range sc.depts {
			if sc.VisibleDeptIDs[d.ID] {
				want = append(want, d)
			}
		}
	}
	if err := s.loadDeptUsers(ctx, sc, want); err != nil {
		return nil, err
	}
	placed := map[string]bool{}
	for _, d := range want {
		for _, u := range sc.deptUsers[d.ID] {
			placed[u.Id] = true
			if sc.VisibleDeptIDs[d.ID] {
				set[u.Id] = true
			}
		}
	}
	if sc.NoDeptVisible {
		all, err := people()
		if err != nil {
			return nil, err
		}
		for _, p := range all {
			if !placed[p.Id] {
				set[p.Id] = true
			}
		}
	}
	return set, nil
}

// onlyVisible keeps the nodes in set; a nil set keeps them all.
func onlyVisible(nodes []*policypb.NGACNode, set map[string]bool) []*policypb.NGACNode {
	if set == nil {
		return nodes
	}
	out := make([]*policypb.NGACNode, 0, len(nodes))
	for _, n := range nodes {
		if set[n.Id] {
			out = append(out, n)
		}
	}
	return out
}

// --- rules ---

// ruleWorkspace authorizes a rule change or read: manage on Mgmt, in an organization
// workspace whose directory exists. ready=false (with no error) is a workspace that cannot
// carry rules; a write on it is refused by the caller.
func (s *Service) ruleWorkspace(ctx context.Context, caller, wsID string) (ws *WorkspaceResult, dir *dirNodes, ready bool, err error) {
	if ws, err = s.authorizeAdmin(ctx, caller, wsID, ngac.OpManage); err != nil {
		return nil, nil, false, err
	}
	if ws.Type != TypeOrganization {
		return ws, nil, false, nil
	}
	dir, has, err := s.directoryNodes(ctx, ws.ID)
	if err != nil || !has {
		return ws, nil, false, err
	}
	return ws, dir, true, nil
}

// writableRules is ruleWorkspace for a write: a workspace that cannot carry rules is refused.
func (s *Service) writableRules(ctx context.Context, caller, wsID string) (*WorkspaceResult, *dirNodes, error) {
	ws, dir, ready, err := s.ruleWorkspace(ctx, caller, wsID)
	switch {
	case err != nil:
		return nil, nil, err
	case ws.Type != TypeOrganization:
		return nil, nil, fmt.Errorf("%w: only an organization has directory rules", ErrInvalidInput)
	case !ready:
		return nil, nil, fmt.Errorf("%w: the workspace's directory is not set up yet", ErrUnavailable)
	}
	return ws, dir, nil
}

// DirectoryRules lists the rules in force. The caller must hold manage on the Mgmt OA.
func (s *Service) DirectoryRules(ctx context.Context, callerNodeID, wsID string) (*DirectoryRules, error) {
	ws, dir, ready, err := s.ruleWorkspace(ctx, callerNodeID, wsID)
	if err != nil {
		return nil, err
	}
	if !ready {
		return &DirectoryRules{}, nil
	}
	depts, err := s.deptStore.ListDepartmentsByWorkspace(ctx, ws.ID)
	if err != nil {
		return nil, fmt.Errorf("list departments: %w", err)
	}
	byUA := make(map[string]*store.Department, len(depts))
	for _, d := range depts {
		byUA[d.NGACUaID] = d
	}
	out := &DirectoryRules{Ready: true}

	hidden, err := s.hiddenDepartments(ctx, ws.ID, dir, depts)
	if err != nil {
		return nil, err
	}
	for _, d := range depts {
		if hidden[d.ID] {
			n, err := s.peopleBeneath(ctx, d.NGACUaID)
			if err != nil {
				return nil, err
			}
			out.HiddenDepartments = append(out.HiddenDepartments, subjectOf(d, n))
		}
	}

	limited, err := s.policyWrite.GetChildren(ctx, &policypb.GetChildrenRequest{NodeId: dir.Limited})
	if err != nil {
		return nil, fmt.Errorf("%w: read who is limited: %v", ErrUnavailable, err)
	}
	for _, n := range limited.GetNodes() {
		switch d := byUA[n.GetId()]; {
		case n.GetNodeType() != ngac.TypeUA:
		case d != nil:
			c, err := s.peopleBeneath(ctx, d.NGACUaID)
			if err != nil {
				return nil, err
			}
			out.LimitedDepartments = append(out.LimitedDepartments, subjectOf(d, c))
		case isRole(n):
			holders, err := s.roleHolders(ctx, RoleCustom, n.GetId())
			if err != nil {
				return nil, err
			}
			out.LimitedRoles = append(out.LimitedRoles, RuleSubject{ID: n.GetId(), Name: ngac.DisplayName(n.GetName(), n.GetProperties()), MemberCount: len(holders)})
		}
	}
	for _, list := range [][]RuleSubject{out.HiddenDepartments, out.LimitedRoles, out.LimitedDepartments} {
		sort.SliceStable(list, func(i, j int) bool { return strings.ToLower(list[i].Name) < strings.ToLower(list[j].Name) })
	}
	return out, nil
}

func subjectOf(d *store.Department, people int) RuleSubject {
	s := RuleSubject{ID: d.ID, Name: d.Name, MemberCount: people}
	if d.ParentID != nil {
		s.ParentID = *d.ParentID
	}
	return s
}

// peopleBeneath counts the people in a department and in the departments beneath it.
func (s *Service) peopleBeneath(ctx context.Context, deptUA string) (int, error) {
	desc, err := s.policyWrite.GetDescendants(ctx, &policypb.GetDescendantsRequest{NodeId: deptUA})
	if err != nil {
		return 0, fmt.Errorf("count department people: %w", err)
	}
	return len(uniqueNodes(userNodes(desc.GetNodes()))), nil
}

// HideDepartment hides a department, and the departments beneath it, from Danh bạ for
// everyone who is not in it (or beneath it) or an administrator. The caller must hold manage
// on the Mgmt OA. Hiding what is already hidden changes nothing.
func (s *Service) HideDepartment(ctx context.Context, callerNodeID, wsID, deptID string) error {
	return s.fresh(func() error { return s.setHidden(ctx, callerNodeID, wsID, deptID, true) })
}

// UnhideDepartment puts a hidden department back where it was: under its parent's directory
// nodes, or under the open branch for a top-level one.
func (s *Service) UnhideDepartment(ctx context.Context, callerNodeID, wsID, deptID string) error {
	return s.fresh(func() error { return s.setHidden(ctx, callerNodeID, wsID, deptID, false) })
}

func (s *Service) setHidden(ctx context.Context, caller, wsID, deptID string, hide bool) error {
	defer s.lockDirectory(wsID)()
	ws, dir, err := s.writableRules(ctx, caller, wsID)
	if err != nil {
		return err
	}
	dept, err := s.departmentInWorkspace(ctx, wsID, deptID)
	if err != nil {
		return err
	}
	dd, ok, err := s.deptDirNodes(ctx, wsID, dept.ID)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("%w: the workspace's directory is not set up yet", ErrUnavailable)
	}

	done, err := s.alreadyAsAsked(ctx, dept, dd, dir, hide)
	if err != nil {
		return err
	}
	if done {
		return nil // nothing to write, nothing to announce
	}
	if hide {
		// The department's own people keep reading it: the grant first, then the nodes move,
		// so a failure between the two never leaves it hidden from the people it belongs to.
		if _, err := s.policyWrite.CreateAssociation(ctx, &policypb.CreateAssociationRequest{
			UaId: dept.NGACUaID, OaId: dd.Dir, Operations: []string{ngac.OpRead},
		}); err != nil {
			return fmt.Errorf("keep the department readable by its own people: %w", err)
		}
		for _, oa := range []string{dd.Dir, dd.Peer} {
			under, err := s.isChildOf(ctx, oa, dir.Directory)
			if err != nil {
				return err
			}
			if !under {
				if err := s.placeUnder(ctx, oa, dir.Directory); err != nil {
					return err
				}
			}
		}
	} else {
		dirUnder, peerUnder := dir.Open, dir.Directory
		if dept.ParentID != nil {
			parent, ok, err := s.deptDirNodes(ctx, wsID, *dept.ParentID)
			if err != nil {
				return err
			}
			if !ok {
				return fmt.Errorf("%w: the workspace's directory is not set up yet", ErrUnavailable)
			}
			dirUnder, peerUnder = parent.Dir, parent.Peer
		}
		// Back first, then the grant goes: a failure between the two leaves a department
		// that is visible again with an extra read for its own people, which changes nothing.
		for _, p := range []struct{ oa, parent string }{{dd.Dir, dirUnder}, {dd.Peer, peerUnder}} {
			only, err := s.hasOnlyParent(ctx, p.oa, p.parent)
			if err != nil {
				return err
			}
			if !only {
				if err := s.placeUnder(ctx, p.oa, p.parent); err != nil {
					return err
				}
			}
		}
		if _, err := s.policyWrite.RemoveAssociation(ctx, &policypb.RemoveAssociationRequest{UaId: dept.NGACUaID, OaId: dd.Dir}); err != nil && !policyclient.IsNotFound(err) {
			return fmt.Errorf("take back the department's own read: %w", err)
		}
	}
	s.announce(ctx, realtime.KindDepartmentChanged, ws.ID, dept.ID)
	return nil
}

// lockDirectory serialises the directory writes of one workspace in this process (hide,
// unhide, limit, department moves): each works from a reading of the graph, and two at once
// would act on a picture the other has already changed. The returned func releases the lock.
func (s *Service) lockDirectory(wsID string) func() {
	m, _ := s.dirLocks.LoadOrStore(wsID, &sync.Mutex{})
	mu := m.(*sync.Mutex)
	mu.Lock()
	return mu.Unlock
}

// alreadyAsAsked reports whether the department already is hidden (both nodes directly under
// Directory and its own UA reading its Dir) or already is not (both nodes only under their
// parent's, the grant gone), so a repeated request writes and announces nothing.
func (s *Service) alreadyAsAsked(ctx context.Context, dept *store.Department, dd *deptDir, dir *dirNodes, hide bool) (bool, error) {
	assocs, err := s.policyWrite.GetAssociations(ctx, &policypb.GetAssociationsRequest{UaId: dept.NGACUaID})
	if err != nil {
		return false, fmt.Errorf("%w: read the department's grants: %v", ErrUnavailable, err)
	}
	granted := slices.ContainsFunc(assocs.GetAssociations(), func(a *policypb.Association) bool {
		return a.GetOaId() == dd.Dir && slices.Contains(a.GetOperations(), ngac.OpRead)
	})
	if hide {
		if !granted {
			return false, nil
		}
		for _, oa := range []string{dd.Dir, dd.Peer} {
			under, err := s.isChildOf(ctx, oa, dir.Directory)
			if err != nil || !under {
				return false, err
			}
		}
		return true, nil
	}
	if granted {
		return false, nil
	}
	dirUnder, peerUnder := dir.Open, dir.Directory
	if dept.ParentID != nil {
		parent, ok, err := s.deptDirNodes(ctx, dept.WorkspaceID, *dept.ParentID)
		if err != nil || !ok {
			return false, err
		}
		dirUnder, peerUnder = parent.Dir, parent.Peer
	}
	for _, p := range []struct{ oa, parent string }{{dd.Dir, dirUnder}, {dd.Peer, peerUnder}} {
		only, err := s.hasOnlyParent(ctx, p.oa, p.parent)
		if err != nil || !only {
			return false, err
		}
	}
	return true, nil
}

func (s *Service) isChildOf(ctx context.Context, child, parent string) (bool, error) {
	r, err := s.policyWrite.IsAssigned(ctx, &policypb.IsAssignedRequest{ChildId: child, ParentId: parent})
	if err != nil {
		return false, fmt.Errorf("%w: read the directory: %v", ErrUnavailable, err)
	}
	return r.GetValue(), nil
}

// hasOnlyParent reports whether parent is the one parent of child.
func (s *Service) hasOnlyParent(ctx context.Context, child, parent string) (bool, error) {
	have, err := s.policyWrite.GetParents(ctx, &policypb.GetParentsRequest{NodeId: child})
	if err != nil {
		return false, fmt.Errorf("%w: read the directory: %v", ErrUnavailable, err)
	}
	return len(have.GetNodes()) == 1 && have.GetNodes()[0].GetId() == parent, nil
}

// LimitRole limits a custom role to its own department: everyone who holds it sees their own
// department's people and the owners. Never Owners or Members. The caller must hold manage.
func (s *Service) LimitRole(ctx context.Context, callerNodeID, wsID, roleID string) error {
	return s.setLimited(ctx, callerNodeID, wsID, roleID, true, true)
}

// UnlimitRole undoes LimitRole.
func (s *Service) UnlimitRole(ctx context.Context, callerNodeID, wsID, roleID string) error {
	return s.setLimited(ctx, callerNodeID, wsID, roleID, true, false)
}

// LimitDepartment limits a department's people, and the people of the departments beneath it.
func (s *Service) LimitDepartment(ctx context.Context, callerNodeID, wsID, deptID string) error {
	return s.setLimited(ctx, callerNodeID, wsID, deptID, false, true)
}

// UnlimitDepartment undoes LimitDepartment.
func (s *Service) UnlimitDepartment(ctx context.Context, callerNodeID, wsID, deptID string) error {
	return s.setLimited(ctx, callerNodeID, wsID, deptID, false, false)
}

func (s *Service) setLimited(ctx context.Context, caller, wsID, id string, role, limit bool) error {
	return s.fresh(func() error { return s.setLimitedOnce(ctx, caller, wsID, id, role, limit) })
}

func (s *Service) setLimitedOnce(ctx context.Context, caller, wsID, id string, role, limit bool) error {
	defer s.lockDirectory(wsID)()
	ws, dir, err := s.writableRules(ctx, caller, wsID)
	if err != nil {
		return err
	}
	if id == "" {
		return fmt.Errorf("%w: nothing to limit", ErrNotFound)
	}
	var subject, kind string
	if role {
		if err := s.requireRole(ctx, ws, id); err != nil {
			return err
		}
		subject, kind = id, realtime.KindRoleChanged
	} else {
		dept, err := s.departmentInWorkspace(ctx, wsID, id)
		if err != nil {
			return err
		}
		subject, kind = dept.NGACUaID, realtime.KindDepartmentChanged
	}

	limited, err := s.isChildOf(ctx, subject, dir.Limited)
	if err != nil {
		return err
	}
	switch {
	case limit && !limited:
		if _, err := s.policyWrite.CreateAssignment(ctx, &policypb.CreateAssignmentRequest{ChildId: subject, ParentId: dir.Limited}); err != nil {
			return fmt.Errorf("limit: %w", err)
		}
	case !limit && limited:
		if _, err := s.policyWrite.RemoveAssignment(ctx, &policypb.RemoveAssignmentRequest{ChildId: subject, ParentId: dir.Limited}); err != nil {
			return fmt.Errorf("unlimit: %w", err)
		}
	default:
		return nil // already as asked: nothing written, nothing announced
	}
	s.announce(ctx, kind, ws.ID, id)
	return nil
}
