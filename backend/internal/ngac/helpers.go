package ngac

import "ngac-document-platform/internal/models"

// IsAssigned checks if child is directly assigned to parent
func (s *Store) IsAssigned(childID, parentID string) bool {
	return s.graph.IsAssigned(childID, parentID)
}

// GetParentsOfNode returns direct parents of a node
func (s *Store) GetParentsOfNode(nodeID string) []*models.NGACNode {
	return s.graph.GetParents(nodeID)
}

// GetChildrenOfNode returns direct children of a node
func (s *Store) GetChildrenOfNode(nodeID string) []*models.NGACNode {
	return s.graph.GetChildren(nodeID)
}

// GetNode returns a node by ID
func (s *Store) GetNode(nodeID string) *models.NGACNode {
	return s.graph.GetNode(nodeID)
}

// GetGraph returns the underlying graph (for access checks)
func (s *Store) GetGraph() *Graph {
	return s.graph
}
