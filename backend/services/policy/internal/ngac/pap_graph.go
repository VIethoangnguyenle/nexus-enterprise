package ngac

import (
	"errors"
	"fmt"
	"slices"
)

// --- PAP: Graph mutations ---

// AddNode adds a node to the graph and updates the name+type index.
func (g *Graph) AddNode(node *NGACNode) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.Nodes[node.ID] = node
	g.nameTypeIndex[nameTypeKey(node.Name, node.NodeType)] = node
}

// RemoveNode removes a node, all its edges, and cleans up the name+type index.
func (g *Graph) RemoveNode(nodeID string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if node, ok := g.Nodes[nodeID]; ok {
		// Only if the entry is this node's: another node may have been written
		// under the same name since, and it must stay findable.
		key := nameTypeKey(node.Name, node.NodeType)
		if g.nameTypeIndex[key] == node {
			delete(g.nameTypeIndex, key)
		}
	}
	delete(g.Nodes, nodeID)

	// Remove assignments where this node is child or parent
	for id, a := range g.Assignments {
		if a.ChildID == nodeID || a.ParentID == nodeID {
			g.removeAssignmentIndexes(a)
			delete(g.Assignments, id)
		}
	}

	// Remove associations where this node is UA or OA
	for id, a := range g.Associations {
		if a.UAID == nodeID || a.OAID == nodeID {
			g.removeAssociationIndexes(a)
			delete(g.Associations, id)
		}
	}
}

// ValidateAssignment checks whether an assignment is valid without mutating the graph.
// Validates: node existence, type compatibility, and cycle detection.
func (g *Graph) ValidateAssignment(a *Assignment) error {
	g.mu.RLock()
	defer g.mu.RUnlock()

	child, ok := g.Nodes[a.ChildID]
	if !ok {
		return fmt.Errorf("child node %s not found", a.ChildID)
	}
	parent, ok := g.Nodes[a.ParentID]
	if !ok {
		return fmt.Errorf("parent node %s not found", a.ParentID)
	}

	if !IsValidAssignment(child.NodeType, parent.NodeType) {
		return fmt.Errorf("invalid assignment: %s -> %s", child.NodeType, parent.NodeType)
	}

	if g.wouldCreateCycle(a.ParentID, a.ChildID) {
		return fmt.Errorf("assignment would create a cycle")
	}

	return nil
}

// AddAssignment adds a containment edge
func (g *Graph) AddAssignment(a *Assignment) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	child, ok := g.Nodes[a.ChildID]
	if !ok {
		return fmt.Errorf("child node %s not found", a.ChildID)
	}
	parent, ok := g.Nodes[a.ParentID]
	if !ok {
		return fmt.Errorf("parent node %s not found", a.ParentID)
	}

	if !IsValidAssignment(child.NodeType, parent.NodeType) {
		return fmt.Errorf("invalid assignment: %s -> %s", child.NodeType, parent.NodeType)
	}

	// Cycle detection
	if g.wouldCreateCycle(a.ParentID, a.ChildID) {
		return fmt.Errorf("assignment would create a cycle")
	}

	// One entry per edge: adding an edge that is there replaces its entry.
	if g.childToParents[a.ChildID][a.ParentID] {
		for id, existing := range g.Assignments {
			if existing.ChildID == a.ChildID && existing.ParentID == a.ParentID {
				delete(g.Assignments, id)
			}
		}
	}
	g.Assignments[a.ID] = a
	if g.childToParents[a.ChildID] == nil {
		g.childToParents[a.ChildID] = make(map[string]bool)
	}
	g.childToParents[a.ChildID][a.ParentID] = true

	if g.parentToChildren[a.ParentID] == nil {
		g.parentToChildren[a.ParentID] = make(map[string]bool)
	}
	g.parentToChildren[a.ParentID][a.ChildID] = true

	return nil
}

// RemoveAssignment removes a containment edge
func (g *Graph) RemoveAssignment(childID, parentID string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	for id, a := range g.Assignments {
		if a.ChildID == childID && a.ParentID == parentID {
			g.removeAssignmentIndexes(a)
			delete(g.Assignments, id)
			return
		}
	}
}

// ErrInvalidAssociation marks an association that the graph cannot hold: a
// source that is not a UA, a target that is not an OA, or a node that does not
// exist. It is the caller's mistake, not a server fault.
var ErrInvalidAssociation = errors.New("invalid association")

// validateAssociationLocked checks node existence and types. g.mu must be held.
func (g *Graph) validateAssociationLocked(a *Association) error {
	ua, ok := g.Nodes[a.UAID]
	if !ok {
		return fmt.Errorf("%w: UA node %s not found", ErrInvalidAssociation, a.UAID)
	}
	if ua.NodeType != NodeTypeUserAttribute {
		return fmt.Errorf("%w: source must be UA, got %s", ErrInvalidAssociation, ua.NodeType)
	}
	oa, ok := g.Nodes[a.OAID]
	if !ok {
		return fmt.Errorf("%w: OA node %s not found", ErrInvalidAssociation, a.OAID)
	}
	if oa.NodeType != NodeTypeObjectAttr {
		return fmt.Errorf("%w: target must be OA, got %s", ErrInvalidAssociation, oa.NodeType)
	}
	return nil
}

// ValidateAssociation checks whether an association is valid without mutating
// the graph, so a caller can refuse it before writing anything durable.
func (g *Graph) ValidateAssociation(a *Association) error {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.validateAssociationLocked(a)
}

// AddAssociation adds a permission edge
func (g *Graph) AddAssociation(a *Association) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if err := g.validateAssociationLocked(a); err != nil {
		return err
	}

	// One edge per UA and OA, as in the database (UNIQUE(ua_id, oa_id)): a second
	// grant replaces the first. Keeping both would make the operations a union
	// of every grant ever made, so narrowing a role's rights would change nothing.
	for _, existing := range slices.Clone(g.uaToAssociations[a.UAID]) {
		if existing.OAID == a.OAID {
			g.removeAssociationIndexes(existing)
			delete(g.Associations, existing.ID)
		}
	}

	g.Associations[a.ID] = a
	g.uaToAssociations[a.UAID] = append(g.uaToAssociations[a.UAID], a)
	g.oaToAssociations[a.OAID] = append(g.oaToAssociations[a.OAID], a)

	return nil
}

// RemoveAssociationByID removes a permission edge by ID
func (g *Graph) RemoveAssociationByID(id string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	a, ok := g.Associations[id]
	if !ok {
		return
	}
	g.removeAssociationIndexes(a)
	delete(g.Associations, id)
}

// replaceWith swaps the entire contents of g for those of src in one step.
//
// Every holder of the *Graph (the store, the decision engine, the cache
// invalidator) keeps its pointer and sees either the complete old graph or the
// complete new one — never a half-loaded mix. src is consumed: the caller must
// not use it afterwards.
func (g *Graph) replaceWith(src *Graph) {
	src.mu.Lock()
	defer src.mu.Unlock()
	g.mu.Lock()
	defer g.mu.Unlock()

	g.Nodes = src.Nodes
	g.Assignments = src.Assignments
	g.Associations = src.Associations
	g.childToParents = src.childToParents
	g.parentToChildren = src.parentToChildren
	g.uaToAssociations = src.uaToAssociations
	g.oaToAssociations = src.oaToAssociations
	g.nameTypeIndex = src.nameTypeIndex
	g.prohibitions = src.prohibitions
	g.prohibitionsBySubject = src.prohibitionsBySubject
}

// AddProhibition loads a prohibition into the graph, replacing any with the
// same name. The graph keeps its own copy.
func (g *Graph) AddProhibition(p *Prohibition) error {
	if p == nil {
		return fmt.Errorf("prohibition is nil")
	}
	if p.Name == "" || p.SubjectID == "" || len(p.Operations) == 0 || len(p.TargetOAIDs) == 0 {
		return fmt.Errorf("prohibition %q needs a name, a subject, operations and targets", p.Name)
	}
	cp := *p
	cp.Operations = slices.Clone(p.Operations)
	cp.TargetOAIDs = slices.Clone(p.TargetOAIDs)

	g.mu.Lock()
	defer g.mu.Unlock()
	g.removeProhibitionLocked(cp.Name)
	g.prohibitions[cp.Name] = &cp
	g.prohibitionsBySubject[cp.SubjectID] = append(g.prohibitionsBySubject[cp.SubjectID], &cp)
	return nil
}

// RemoveProhibition drops the named prohibition; unknown names are a no-op.
func (g *Graph) RemoveProhibition(name string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.removeProhibitionLocked(name)
}

func (g *Graph) removeProhibitionLocked(name string) {
	old, ok := g.prohibitions[name]
	if !ok {
		return
	}
	delete(g.prohibitions, name)
	rest := slices.DeleteFunc(slices.Clone(g.prohibitionsBySubject[old.SubjectID]),
		func(p *Prohibition) bool { return p.Name == name })
	if len(rest) == 0 {
		delete(g.prohibitionsBySubject, old.SubjectID)
		return
	}
	g.prohibitionsBySubject[old.SubjectID] = rest
}

// --- Internal helpers ---

func (g *Graph) wouldCreateCycle(fromID, toID string) bool {
	if fromID == toID {
		return true
	}
	visited := make(map[string]bool)
	return g.canReach(fromID, toID, visited)
}

func (g *Graph) canReach(current, target string, visited map[string]bool) bool {
	if current == target {
		return true
	}
	if visited[current] {
		return false
	}
	visited[current] = true
	if parents, ok := g.childToParents[current]; ok {
		for pid := range parents {
			if g.canReach(pid, target, visited) {
				return true
			}
		}
	}
	return false
}

func (g *Graph) removeAssignmentIndexes(a *Assignment) {
	if parents, ok := g.childToParents[a.ChildID]; ok {
		delete(parents, a.ParentID)
	}
	if children, ok := g.parentToChildren[a.ParentID]; ok {
		delete(children, a.ChildID)
	}
}

func (g *Graph) removeAssociationIndexes(a *Association) {
	// Remove from uaToAssociations
	assocs := g.uaToAssociations[a.UAID]
	for i, existing := range assocs {
		if existing.ID == a.ID {
			g.uaToAssociations[a.UAID] = append(assocs[:i], assocs[i+1:]...)
			break
		}
	}
	// Remove from oaToAssociations
	assocs = g.oaToAssociations[a.OAID]
	for i, existing := range assocs {
		if existing.ID == a.ID {
			g.oaToAssociations[a.OAID] = append(assocs[:i], assocs[i+1:]...)
			break
		}
	}
}
