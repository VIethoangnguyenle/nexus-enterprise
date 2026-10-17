package domain

import (
	"context"
	"fmt"
	"log/slog"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"ngac-platform/ngac"
	"ngac-platform/pkg/provision"
	policypb "ngac-platform/proto/policy"
	"ngac-platform/services/workspace/internal/store"
)

// The contacts directory graph (docs/specs/contacts-visibility): per workspace a
// Directory OA under Mgmt with an open branch and an owners branch, and per
// department a Dir OA, a DirPeer OA and a flat DeptPeers UA. This file creates
// the workspace's part, mirrors departments into it, and keeps people's peers
// edges with their department edges. Every write is a policy-service call, so
// each one takes the EPP invalidation path.
//
// A workspace that has no Directory node (one that predates migration 064) has no rules:
// every operation here is skipped for it, and migration 064 builds everything for it later.
// A department of a workspace that has a directory but whose own nodes are missing (it was
// created while the migration ran, or before it) gets them built the first time something
// needs them, its parent's first (deptDirNodes); migration 064 does the same in bulk. Anything
// that is not a plain "not found" fails closed.

// dirNodes are the ids of a workspace's five directory nodes.
type dirNodes struct {
	Directory, Open, NoDept, Owners, Limited string
}

// deptDir are the ids of a department's three directory nodes.
type deptDir struct {
	Dir, Peer, Peers string
}

// directoryNode finds one directory node by its ID-keyed name. A node is accepted
// only when it carries the directory mark of this workspace, as a folder is only
// trusted with its Folder_ name and workspace_id: a node anyone could name
// "Directory_<ws>" must not stand in for it. Hits are remembered for the life of the
// service (a node's id never changes and a workspace or department id is never
// reused); a miss or an error is never stored, so a directory built after a miss is
// picked up without a restart. found=false is a plain NotFound; every other failure
// is an error (ErrUnavailable), never "there is none".
func (s *Service) directoryNode(ctx context.Context, ws ngac.WorkspaceID, name, nodeType string) (id string, found bool, err error) {
	if cached, ok := s.dirIDs.Load(name); ok {
		return cached.(string), true, nil
	}
	n, err := s.policyWrite.FindNodeByName(ctx, &policypb.FindNodeByNameRequest{Name: name, NodeType: nodeType})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return "", false, nil
		}
		return "", false, fmt.Errorf("%w: look up directory node %s: %v", ErrUnavailable, name, err)
	}
	props := n.GetProperties()
	if n.GetId() == "" || n.GetNodeType() != nodeType ||
		props[ngac.PropType] != ngac.PropTypeDirectory || props["workspace_id"] != string(ws) {
		return "", false, fmt.Errorf("%w: node %s is not workspace %s's directory node", ErrUnavailable, name, ws)
	}
	s.dirIDs.Store(name, n.GetId())
	return n.GetId(), true, nil
}

// forgetDirectoryNodes drops cached ids of nodes that were deleted.
func (s *Service) forgetDirectoryNodes(names ...string) {
	for _, n := range names {
		s.dirIDs.Delete(n)
	}
}

// directoryNodes returns the workspace's directory nodes; false when it has no
// directory (Directory_<ws> is not there). A directory missing any of its five
// nodes is an error: it is half built, and nothing may guess.
func (s *Service) directoryNodes(ctx context.Context, wsID string) (*dirNodes, bool, error) {
	ws := ngac.WorkspaceID(wsID)
	root, found, err := s.directoryNode(ctx, ws, ngac.DirectoryOAName(ws), ngac.TypeOA)
	if err != nil || !found {
		return nil, false, err
	}
	d := &dirNodes{Directory: root}
	for _, part := range []struct {
		into     *string
		name, ty string
	}{
		{&d.Open, ngac.DirectoryOpenOAName(ws), ngac.TypeOA},
		{&d.NoDept, ngac.DirectoryNoDeptOAName(ws), ngac.TypeOA},
		{&d.Owners, ngac.DirectoryOwnersOAName(ws), ngac.TypeOA},
		{&d.Limited, ngac.DirectoryLimitedUAName(ws), ngac.TypeUA},
	} {
		id, ok, err := s.directoryNode(ctx, ws, part.name, part.ty)
		if err != nil {
			return nil, false, err
		}
		if !ok {
			return nil, false, fmt.Errorf("%w: workspace %s has a directory without %s", ErrUnavailable, wsID, part.name)
		}
		*part.into = id
	}
	return d, true, nil
}

// fresh runs an operation that uses cached directory node ids. When a write answers NotFound
// a node has been deleted and rebuilt since its id was cached (a migration reversed and
// applied again): the cache is dropped and the operation runs once more with fresh lookups.
func (s *Service) fresh(fn func() error) error {
	err := fn()
	if err != nil && status.Code(err) == codes.NotFound {
		s.dirIDs.Range(func(k, _ any) bool { s.dirIDs.Delete(k); return true })
		err = fn()
	}
	return err
}

// deptDirNodes returns a department's directory nodes. A workspace that has a directory
// but a department with none (it was created while the migration ran, or before it) gets
// them built here, parents first, so whatever needs them first makes them: false only when
// the workspace has no directory.
func (s *Service) deptDirNodes(ctx context.Context, wsID, deptID string) (*deptDir, bool, error) {
	dd, ok, err := s.findDeptDir(ctx, wsID, deptID)
	if err != nil || ok {
		return dd, ok, err
	}
	dir, has, err := s.directoryNodes(ctx, wsID)
	if err != nil || !has {
		return nil, false, err
	}
	ws, err := s.GetWorkspace(ctx, wsID)
	if err != nil {
		return nil, false, err
	}
	all, err := s.deptStore.ListDepartmentsByWorkspace(ctx, wsID)
	if err != nil {
		return nil, false, err
	}
	byID := make(map[string]*store.Department, len(all))
	for _, d := range all {
		byID[d.ID] = d
	}
	dd, err = s.buildDeptDir(ctx, ws.PcNodeID, dir, byID, deptID, map[string]bool{})
	return dd, err == nil, err
}

// buildDeptDir creates whichever of a department's three nodes are missing, under its
// parent's (built first) or under Open and Directory, like a department created with its
// nodes: a department under a hidden parent is built hidden with it. Idempotent, and safe
// against another writer making the same node (the loser looks it up). People already
// directly in the department get their peers edge. A failure removes what this call made.
func (s *Service) buildDeptDir(ctx context.Context, pcID string, dir *dirNodes, byID map[string]*store.Department, deptID string, seen map[string]bool) (*deptDir, error) {
	d, ok := byID[deptID]
	if !ok || seen[deptID] {
		return nil, fmt.Errorf("%w: department %s", ErrNotFound, deptID)
	}
	seen[deptID] = true
	var parent *deptDir
	if d.ParentID != nil {
		var err error
		if parent, ok, err = s.findDeptDir(ctx, d.WorkspaceID, *d.ParentID); err != nil {
			return nil, err
		} else if !ok {
			if parent, err = s.buildDeptDir(ctx, pcID, dir, byID, *d.ParentID, seen); err != nil {
				return nil, err
			}
		}
	}
	ws, did := ngac.WorkspaceID(d.WorkspaceID), ngac.DeptID(d.ID)
	dirUnder, peerUnder := dir.Open, dir.Directory
	if parent != nil {
		dirUnder, peerUnder = parent.Dir, parent.Peer
	}
	prov := provision.NewCreator(s.policyWrite)
	made := map[string]bool{}
	node := func(name, nodeType, under string) (string, error) {
		if id, found, err := s.directoryNode(ctx, ws, name, nodeType); err != nil || found {
			return id, err
		}
		n, err := prov.Node(ctx, &policypb.CreateNodeRequest{Name: name, NodeType: nodeType, Properties: ngac.DirectoryProps(ws)})
		if err != nil {
			// Another writer may have made it a moment ago.
			if id, found, lookErr := s.directoryNode(ctx, ws, name, nodeType); lookErr == nil && found {
				return id, nil
			}
			return "", fmt.Errorf("create directory node %s: %w", name, err)
		}
		if err := prov.Assign(ctx, n.Id, under); err != nil {
			return "", fmt.Errorf("assign directory node %s: %w", name, err)
		}
		made[name] = true
		return n.Id, nil
	}
	out := &deptDir{}
	var err error
	if out.Dir, err = node(ngac.DeptDirOAName(did), ngac.TypeOA, dirUnder); err != nil {
		return nil, prov.Fail(ctx, err)
	}
	if out.Peer, err = node(ngac.DeptPeerOAName(did), ngac.TypeOA, peerUnder); err != nil {
		return nil, prov.Fail(ctx, err)
	}
	if out.Peers, err = node(ngac.DeptPeersUAName(did), ngac.TypeUA, pcID); err != nil {
		return nil, prov.Fail(ctx, err)
	}
	if made[ngac.DeptPeerOAName(did)] || made[ngac.DeptPeersUAName(did)] {
		if err := prov.Associate(ctx, out.Peers, out.Peer, []string{ngac.OpRead}); err != nil {
			return nil, prov.Fail(ctx, fmt.Errorf("associate department peers: %w", err))
		}
	}
	if made[ngac.DeptPeersUAName(did)] {
		kids, err := s.policyWrite.GetChildren(ctx, &policypb.GetChildrenRequest{NodeId: d.NGACUaID})
		if err != nil {
			return nil, prov.Fail(ctx, fmt.Errorf("read who is in the department: %w", err))
		}
		for _, u := range userNodes(kids.GetNodes()) {
			if err := prov.Assign(ctx, u.Id, out.Peers); err != nil {
				return nil, prov.Fail(ctx, fmt.Errorf("give %s the department's peers edge: %w", u.Id, err))
			}
		}
	}
	prov.Done()
	slog.Info("built a department's directory nodes", "workspace", d.WorkspaceID, "department", d.ID)
	return out, nil
}

// findDeptDir looks a department's nodes up and builds nothing; false when it has none.
func (s *Service) findDeptDir(ctx context.Context, wsID, deptID string) (*deptDir, bool, error) {
	ws, d := ngac.WorkspaceID(wsID), ngac.DeptID(deptID)
	dir, found, err := s.directoryNode(ctx, ws, ngac.DeptDirOAName(d), ngac.TypeOA)
	if err != nil || !found {
		return nil, false, err
	}
	out := &deptDir{Dir: dir}
	for _, part := range []struct {
		into     *string
		name, ty string
	}{
		{&out.Peer, ngac.DeptPeerOAName(d), ngac.TypeOA},
		{&out.Peers, ngac.DeptPeersUAName(d), ngac.TypeUA},
	} {
		id, ok, err := s.directoryNode(ctx, ws, part.name, part.ty)
		if err != nil {
			return nil, false, err
		}
		if !ok {
			return nil, false, fmt.Errorf("%w: department %s has Dir without %s", ErrUnavailable, deptID, part.name)
		}
		*part.into = id
	}
	return out, true, nil
}

// skipDept says why a department's directory is left alone (the workspace has none), at warn level.
func skipDept(wsID, deptID, why string) {
	slog.Warn("department has no directory nodes and the workspace has no directory to build them in", "workspace", wsID, "department", deptID, "why", why)
}

// createDirectory builds the workspace's directory inside a provisioning run:
// the five nodes, their edges, and the grants of Members. The prohibition is
// created separately (createDirectoryRule) because it needs its own undo.
func (s *Service) createDirectory(ctx context.Context, prov *provision.Creator, id ngac.WorkspaceID, pcID, membersID, mgmtID string) (*dirNodes, error) {
	node := func(name, nodeType string, parent string) (string, error) {
		n, err := prov.Node(ctx, &policypb.CreateNodeRequest{Name: name, NodeType: nodeType, Properties: ngac.DirectoryProps(id)})
		if err != nil {
			return "", fmt.Errorf("create directory node %s: %w", name, err)
		}
		if err := prov.Assign(ctx, n.Id, parent); err != nil {
			return "", fmt.Errorf("assign directory node %s: %w", name, err)
		}
		return n.Id, nil
	}
	var d dirNodes
	var err error
	if d.Directory, err = node(ngac.DirectoryOAName(id), ngac.TypeOA, mgmtID); err != nil {
		return nil, err
	}
	if d.Open, err = node(ngac.DirectoryOpenOAName(id), ngac.TypeOA, d.Directory); err != nil {
		return nil, err
	}
	if d.NoDept, err = node(ngac.DirectoryNoDeptOAName(id), ngac.TypeOA, d.Open); err != nil {
		return nil, err
	}
	// Beside Open, never under it: the limit must not reach the owners.
	if d.Owners, err = node(ngac.DirectoryOwnersOAName(id), ngac.TypeOA, d.Directory); err != nil {
		return nil, err
	}
	// Under the class, never under Members: it grants nothing and makes nobody a member.
	if d.Limited, err = node(ngac.DirectoryLimitedUAName(id), ngac.TypeUA, pcID); err != nil {
		return nil, err
	}
	for _, oa := range []string{d.Open, d.Owners} {
		if err := prov.Associate(ctx, membersID, oa, []string{ngac.OpRead}); err != nil {
			return nil, fmt.Errorf("associate members with directory: %w", err)
		}
	}
	return &d, nil
}

// createDirectoryRule creates the workspace's one directory prohibition: people
// under DirectoryLimited cannot read DirectoryOpen. A prohibition has no foreign
// key to the nodes it names, so a caller whose run fails afterwards must call
// removeDirectoryRule.
func (s *Service) createDirectoryRule(ctx context.Context, id ngac.WorkspaceID, d *dirNodes) error {
	_, err := s.policyWrite.CreateProhibition(ctx, &policypb.CreateProhibitionRequest{
		Name:         ngac.DirectoryLimitedProhibitionName(id),
		SubjectId:    d.Limited,
		Operations:   []string{ngac.OpRead},
		TargetOaIds:  []string{d.Open},
		Intersection: false,
	})
	if err != nil {
		return fmt.Errorf("create directory rule: %w", err)
	}
	return nil
}

// removeDirectoryRule removes the workspace's directory prohibition by name. A
// rule that is not there is not an error, so a teardown that is run again
// succeeds.
func (s *Service) removeDirectoryRule(ctx context.Context, wsID string) error {
	_, err := s.policyWrite.RemoveProhibition(ctx, &policypb.RemoveProhibitionRequest{
		Name: ngac.DirectoryLimitedProhibitionName(ngac.WorkspaceID(wsID)),
	})
	if err != nil && status.Code(err) != codes.NotFound {
		return fmt.Errorf("remove directory rule: %w", err)
	}
	return nil
}

// createDeptDirectory builds a new department's three nodes inside its provisioning
// run: Dir under the parent's Dir (or Open), DirPeer under the parent's DirPeer (or
// Directory), and a flat DeptPeers UA under the class that reads its DirPeer. A
// department created under a hidden one nests under the hidden Dir and is hidden
// with it.
func (s *Service) createDeptDirectory(ctx context.Context, prov *provision.Creator, id ngac.WorkspaceID, deptID, pcID string, dir *dirNodes, parent *deptDir) error {
	d := ngac.DeptID(deptID)
	dirUnder, peerUnder := dir.Open, dir.Directory
	if parent != nil {
		dirUnder, peerUnder = parent.Dir, parent.Peer
	}
	node := func(name, nodeType, under string) (string, error) {
		n, err := prov.Node(ctx, &policypb.CreateNodeRequest{Name: name, NodeType: nodeType, Properties: ngac.DirectoryProps(id)})
		if err != nil {
			return "", fmt.Errorf("create directory node %s: %w", name, err)
		}
		if err := prov.Assign(ctx, n.Id, under); err != nil {
			return "", fmt.Errorf("assign directory node %s: %w", name, err)
		}
		return n.Id, nil
	}
	if _, err := node(ngac.DeptDirOAName(d), ngac.TypeOA, dirUnder); err != nil {
		return err
	}
	peer, err := node(ngac.DeptPeerOAName(d), ngac.TypeOA, peerUnder)
	if err != nil {
		return err
	}
	peers, err := node(ngac.DeptPeersUAName(d), ngac.TypeUA, pcID)
	if err != nil {
		return err
	}
	if err := prov.Associate(ctx, peers, peer, []string{ngac.OpRead}); err != nil {
		return fmt.Errorf("associate department peers: %w", err)
	}
	return nil
}

// deptMirror is what a department's move or delete does to the directory, worked
// out (and every lookup made) before the first write, so a lookup that fails stops
// the operation with nothing half done.
type deptMirror struct {
	dir    *dirNodes
	dept   *deptDir
	parent *deptDir // the new parent's (move) or the parent's (delete); nil: the root
	hidden bool     // the department's Dir sits directly under Directory
}

// planDeptMirror resolves the nodes for a department that is about to move under
// parent (nil: the root) or be deleted. nil with no error: nothing to mirror.
func (s *Service) planDeptMirror(ctx context.Context, wsID string, dept, parent *store.Department) (*deptMirror, error) {
	dir, ok, err := s.directoryNodes(ctx, wsID)
	if err != nil || !ok {
		return nil, err
	}
	dd, ok, err := s.deptDirNodes(ctx, wsID, dept.ID)
	if err != nil {
		return nil, err
	}
	if !ok {
		skipDept(wsID, dept.ID, "department")
		return nil, nil
	}
	m := &deptMirror{dir: dir, dept: dd}
	if parent != nil {
		if m.parent, ok, err = s.deptDirNodes(ctx, wsID, parent.ID); err != nil {
			return nil, err
		} else if !ok {
			skipDept(wsID, parent.ID, "parent")
			return nil, nil
		}
	}
	if m.hidden, err = s.sitsUnderDirectory(ctx, dd.Dir, dir); err != nil {
		return nil, err
	}
	return m, nil
}

// placeUnder makes parent the one parent of child: the new edge first (so the node
// is never cut off), then every other edge away. It is idempotent.
func (s *Service) placeUnder(ctx context.Context, child, parent string) error {
	have, err := s.policyWrite.GetParents(ctx, &policypb.GetParentsRequest{NodeId: child})
	if err != nil {
		return fmt.Errorf("read parents of %s: %w", child, err)
	}
	if _, err := s.policyWrite.CreateAssignment(ctx, &policypb.CreateAssignmentRequest{ChildId: child, ParentId: parent}); err != nil {
		return fmt.Errorf("place directory node under its new parent: %w", err)
	}
	for _, p := range have.GetNodes() {
		if p.GetId() == parent {
			continue
		}
		if _, err := s.policyWrite.RemoveAssignment(ctx, &policypb.RemoveAssignmentRequest{ChildId: child, ParentId: p.GetId()}); err != nil {
			return fmt.Errorf("take directory node from its old parent: %w", err)
		}
	}
	return nil
}

// moveDeptDirectory re-parents a moved department's nodes: Dir under the new
// parent's Dir (or Open) and DirPeer under the new parent's DirPeer (or Directory).
// A hidden department stays where it is: both nodes under Directory, so hiding
// survives the move and nobody placed in the new parent gains a view.
func (s *Service) moveDeptDirectory(ctx context.Context, m *deptMirror) error {
	if m == nil || m.hidden {
		return nil
	}
	dirUnder, peerUnder := m.dir.Open, m.dir.Directory
	if m.parent != nil {
		dirUnder, peerUnder = m.parent.Dir, m.parent.Peer
	}
	if err := s.placeUnder(ctx, m.dept.Dir, dirUnder); err != nil {
		return err
	}
	return s.placeUnder(ctx, m.dept.Peer, peerUnder)
}

// subDeptMirror is the directory nodes of one sub-department of a deleted department.
type subDeptMirror struct {
	ua     string // the sub-department's UA
	dd     *deptDir
	hidden bool // its Dir already sits directly under Directory
}

// planSubDepartments resolves the nodes of the deleted department's direct
// sub-departments (those without nodes are skipped, logged).
func (s *Service) planSubDepartments(ctx context.Context, wsID, deptID string, dir *dirNodes, all []*store.Department) ([]subDeptMirror, error) {
	var out []subDeptMirror
	for _, d := range all {
		if d.ParentID == nil || *d.ParentID != deptID {
			continue
		}
		dd, ok, err := s.deptDirNodes(ctx, wsID, d.ID)
		if err != nil {
			return nil, err
		}
		if !ok {
			skipDept(wsID, d.ID, "sub-department")
			continue
		}
		hidden, err := s.sitsUnderDirectory(ctx, dd.Dir, dir)
		if err != nil {
			return nil, err
		}
		out = append(out, subDeptMirror{ua: d.NGACUaID, dd: dd, hidden: hidden})
	}
	return out, nil
}

// sitsUnderDirectory reports whether a Dir OA hangs directly under Directory,
// which is what a hidden department is.
func (s *Service) sitsUnderDirectory(ctx context.Context, dirOA string, dir *dirNodes) (bool, error) {
	parents, err := s.policyWrite.GetParents(ctx, &policypb.GetParentsRequest{NodeId: dirOA})
	if err != nil {
		return false, fmt.Errorf("%w: read where the department's directory sits: %v", ErrUnavailable, err)
	}
	for _, p := range parents.GetNodes() {
		if p.GetId() == dir.Directory {
			return true, nil
		}
	}
	return false, nil
}

// reparentSubDepartments moves a deleted department's sub-departments' nodes up to
// the deleted one's parent (or to Open and Directory). When the deleted department was
// hidden they stay hidden: their nodes go directly under Directory and their own UA
// is given read on their Dir, the grant the hidden department's UA held for them,
// so deleting a hidden department never publishes what it hid. A sub-department
// that is hidden in its own right already sits under Directory and is left there.
func (s *Service) reparentSubDepartments(ctx context.Context, m *deptMirror, subs []subDeptMirror) error {
	for _, sub := range subs {
		if m.hidden {
			// The grant first, and whether or not the nodes still have to move: an attempt that
			// was interrupted between the two finishes on the retry, and the nodes are never
			// under Directory without it.
			if _, err := s.policyWrite.CreateAssociation(ctx, &policypb.CreateAssociationRequest{
				UaId: sub.ua, OaId: sub.dd.Dir, Operations: []string{ngac.OpRead},
			}); err != nil {
				return fmt.Errorf("keep sub-department hidden: %w", err)
			}
			if !sub.hidden {
				if err := s.placeUnder(ctx, sub.dd.Dir, m.dir.Directory); err != nil {
					return err
				}
				if err := s.placeUnder(ctx, sub.dd.Peer, m.dir.Directory); err != nil {
					return err
				}
			}
			continue
		}
		if sub.hidden {
			continue
		}
		dirUnder, peerUnder := m.dir.Open, m.dir.Directory
		if m.parent != nil {
			dirUnder, peerUnder = m.parent.Dir, m.parent.Peer
		}
		if err := s.placeUnder(ctx, sub.dd.Dir, dirUnder); err != nil {
			return err
		}
		if err := s.placeUnder(ctx, sub.dd.Peer, peerUnder); err != nil {
			return err
		}
	}
	return nil
}

// deleteDeptDirectory removes a deleted department's three nodes, the peers UA
// first (it takes the people's peers edges with it). A node already gone is fine.
func (s *Service) deleteDeptDirectory(ctx context.Context, wsID, deptID string, dd *deptDir) error {
	d := ngac.DeptID(deptID)
	for _, id := range []string{dd.Peers, dd.Peer, dd.Dir} {
		if _, err := s.policyWrite.DeleteNode(ctx, &policypb.DeleteNodeRequest{NodeId: id}); err != nil && status.Code(err) != codes.NotFound {
			return fmt.Errorf("remove department directory node: %w", err)
		}
	}
	s.forgetDirectoryNodes(ngac.DeptDirOAName(d), ngac.DeptPeerOAName(d), ngac.DeptPeersUAName(d))
	return nil
}

// peersFor is the peers UA a person placed directly in dept belongs to; empty when
// there is nothing to write (no department, no directory, or the department has no
// nodes yet).
func (s *Service) peersFor(ctx context.Context, wsID string, dept *store.Department) (string, error) {
	if dept == nil {
		return "", nil
	}
	if _, ok, err := s.directoryNodes(ctx, wsID); err != nil || !ok {
		return "", err
	}
	dd, ok, err := s.deptDirNodes(ctx, wsID, dept.ID)
	if err != nil {
		return "", err
	}
	if !ok {
		skipDept(wsID, dept.ID, "department of a person")
		return "", nil
	}
	return dd.Peers, nil
}

// stalePeers lists the peers UAs of the workspace's departments that a person
// sits under other than keep: what a department change must take away.
func stalePeers(parents []*policypb.NGACNode, all []*store.Department, keep string) []string {
	peersNames := make(map[string]bool, len(all))
	for _, d := range all {
		peersNames[ngac.DeptPeersUAName(ngac.DeptID(d.ID))] = true
	}
	var out []string
	for _, p := range parents {
		if peersNames[p.GetName()] && p.GetId() != keep {
			out = append(out, p.GetId())
		}
	}
	return out
}
