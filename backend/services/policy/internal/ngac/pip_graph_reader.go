package ngac

// GraphReader provides read-only access to the NGAC graph (PIP).
// PDP components should depend on this interface, not *Graph directly.
// This enforces the boundary: PDP reads data, PAP mutates data.
type GraphReader interface {
	GetNode(id string) *NGACNode
	FindNodeByName(name, nodeType string) *NGACNode
	GetAncestors(nodeID string) map[string]*NGACNode
	GetAncestorsWithPaths(nodeID string) map[string][]string
	GetDescendants(nodeID string) map[string]*NGACNode
	GetChildren(nodeID string) []*NGACNode
	GetParents(nodeID string) []*NGACNode
	GetNodesByType(nodeType string) []*NGACNode
	GetAssociationsFromUA(uaID string) []*Association
	IsAssigned(childID, parentID string) bool
	CheckAccess(userID, objectID, operation string) *AccessDecision
}

// Compile-time check: *Graph implements GraphReader.
var _ GraphReader = (*Graph)(nil)
