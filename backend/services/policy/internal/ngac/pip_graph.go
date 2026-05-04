package ngac

import (
	"fmt"
	"sync"
)

// Graph is the in-memory NGAC graph for fast traversal.
//
// Method groups:
//   - PAP (mutations): AddNode, RemoveNode, AddAssignment, RemoveAssignment,
//     AddAssociation, RemoveAssociationByID  (see pap_graph.go)
//   - PIP (reads): GetNode, FindNodeByName, GetAncestors, GetAncestorsWithPaths,
//     GetDescendants, GetChildren, GetParents, GetNodesByType, GetAssociationsFromUA, IsAssigned
//   - PDP (evaluation): CheckAccess (in pdp_access.go), bfsCollectAttributesAndPCs
type Graph struct {
	mu sync.RWMutex

	Nodes        map[string]*NGACNode
	Assignments  map[string]*Assignment   // id -> assignment
	Associations map[string]*Association   // id -> association

	// Indexes for fast traversal
	childToParents   map[string]map[string]bool // childID -> set of parentIDs
	parentToChildren map[string]map[string]bool // parentID -> set of childIDs
	uaToAssociations map[string][]*Association  // uaID -> associations from this UA
	oaToAssociations map[string][]*Association  // oaID -> associations to this OA
	nameTypeIndex    map[string]*NGACNode       // "name\x00type" -> node for O(1) lookup
}

func NewGraph() *Graph {
	return &Graph{
		Nodes:            make(map[string]*NGACNode),
		Assignments:      make(map[string]*Assignment),
		Associations:     make(map[string]*Association),
		childToParents:   make(map[string]map[string]bool),
		parentToChildren: make(map[string]map[string]bool),
		uaToAssociations: make(map[string][]*Association),
		oaToAssociations: make(map[string][]*Association),
		nameTypeIndex:    make(map[string]*NGACNode),
	}
}

// --- PIP: Read-only graph traversal ---

// GetAncestors returns all ancestors of a node using iterative BFS (upward traversal).
func (g *Graph) GetAncestors(nodeID string) map[string]*NGACNode {
	g.mu.RLock()
	defer g.mu.RUnlock()

	result := make(map[string]*NGACNode)
	visited := map[string]bool{nodeID: true}
	queue := []string{nodeID}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		for pid := range g.childToParents[current] {
			if visited[pid] {
				continue
			}
			visited[pid] = true
			if n, ok := g.Nodes[pid]; ok {
				result[pid] = n
				queue = append(queue, pid)
			}
		}
	}
	return result
}

// GetAncestorsWithPaths returns ancestors with the path taken to reach each.
func (g *Graph) GetAncestorsWithPaths(nodeID string) map[string][]string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	type pathEntry struct {
		id   string
		path []string
	}

	paths := make(map[string][]string)
	visited := map[string]bool{nodeID: true}

	var startStep string
	if node := g.Nodes[nodeID]; node != nil {
		startStep = fmt.Sprintf("%s (%s)", node.Name, node.NodeType)
	} else {
		startStep = nodeID
	}

	queue := []pathEntry{{id: nodeID, path: []string{startStep}}}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		for pid := range g.childToParents[current.id] {
			if visited[pid] {
				continue
			}
			visited[pid] = true

			newPath := make([]string, len(current.path))
			copy(newPath, current.path)
			paths[pid] = newPath

			var step string
			if n := g.Nodes[pid]; n != nil {
				step = fmt.Sprintf("%s (%s)", n.Name, n.NodeType)
			} else {
				step = pid
			}
			queue = append(queue, pathEntry{id: pid, path: append(newPath, step)})
		}
	}
	return paths
}

// GetDescendants returns all descendants of a node using iterative BFS (downward traversal).
func (g *Graph) GetDescendants(nodeID string) map[string]*NGACNode {
	g.mu.RLock()
	defer g.mu.RUnlock()

	result := make(map[string]*NGACNode)
	visited := map[string]bool{nodeID: true}
	queue := []string{nodeID}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		for cid := range g.parentToChildren[current] {
			if visited[cid] {
				continue
			}
			visited[cid] = true
			if n, ok := g.Nodes[cid]; ok {
				result[cid] = n
				queue = append(queue, cid)
			}
		}
	}
	return result
}

// GetNodesByType returns all nodes of a given type
func (g *Graph) GetNodesByType(nodeType string) []*NGACNode {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var result []*NGACNode
	for _, n := range g.Nodes {
		if n.NodeType == nodeType {
			result = append(result, n)
		}
	}
	return result
}

// GetParents returns direct parents of a node
func (g *Graph) GetParents(nodeID string) []*NGACNode {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var result []*NGACNode
	if parents, ok := g.childToParents[nodeID]; ok {
		for pid := range parents {
			if n, ok := g.Nodes[pid]; ok {
				result = append(result, n)
			}
		}
	}
	return result
}

// GetChildren returns direct children of a node
func (g *Graph) GetChildren(nodeID string) []*NGACNode {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var result []*NGACNode
	if children, ok := g.parentToChildren[nodeID]; ok {
		for cid := range children {
			if n, ok := g.Nodes[cid]; ok {
				result = append(result, n)
			}
		}
	}
	return result
}

// GetAssociationsFromUA returns associations originating from a UA
func (g *Graph) GetAssociationsFromUA(uaID string) []*Association {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.uaToAssociations[uaID]
}

// GetNode returns a node by ID
func (g *Graph) GetNode(id string) *NGACNode {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.Nodes[id]
}

// FindNodeByName finds a node by name and type via O(1) index lookup.
func (g *Graph) FindNodeByName(name, nodeType string) *NGACNode {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.nameTypeIndex[nameTypeKey(name, nodeType)]
}

// IsAssigned checks if child is assigned to parent
func (g *Graph) IsAssigned(childID, parentID string) bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	if parents, ok := g.childToParents[childID]; ok {
		return parents[parentID]
	}
	return false
}

// --- PDP support: BFS helpers used by pdp_access.go ---

// bfsCollectAttributesAndPCs performs a single-pass upward BFS from startID,
// collecting all ancestor nodes of attrType and all reachable PolicyClass nodes.
// This replaces the old pattern of separate collectAncestorsOfType + collectPCsReachable calls.
func (g *Graph) bfsCollectAttributesAndPCs(startID, attrType string) (attrs map[string]bool, pcs map[string]bool) {
	attrs = map[string]bool{startID: true}
	pcs = make(map[string]bool)
	visited := map[string]bool{startID: true}
	queue := []string{startID}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		for pid := range g.childToParents[current] {
			if visited[pid] {
				continue
			}
			visited[pid] = true

			node := g.Nodes[pid]
			if node == nil {
				continue
			}

			switch node.NodeType {
			case attrType:
				attrs[pid] = true
				queue = append(queue, pid)
			case NodeTypePolicyClass:
				pcs[pid] = true
				// PC is a DAG root — don't continue BFS above it
			default:
				queue = append(queue, pid)
			}
		}
	}
	return attrs, pcs
}
