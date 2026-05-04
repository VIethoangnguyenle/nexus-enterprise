package ngac

import (
	"fmt"
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
		delete(g.nameTypeIndex, nameTypeKey(node.Name, node.NodeType))
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

// AddAssociation adds a permission edge
func (g *Graph) AddAssociation(a *Association) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	ua, ok := g.Nodes[a.UAID]
	if !ok {
		return fmt.Errorf("UA node %s not found", a.UAID)
	}
	if ua.NodeType != NodeTypeUserAttribute {
		return fmt.Errorf("source must be UA, got %s", ua.NodeType)
	}

	oa, ok := g.Nodes[a.OAID]
	if !ok {
		return fmt.Errorf("OA node %s not found", a.OAID)
	}
	if oa.NodeType != NodeTypeObjectAttr {
		return fmt.Errorf("target must be OA, got %s", oa.NodeType)
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
