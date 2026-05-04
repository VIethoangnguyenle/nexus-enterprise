package ngac

import (
	"fmt"
)

// CheckAccess performs the NGAC access decision using optimized bidirectional BFS.
// Two single-pass BFS traversals collect user attributes + PCs and object attributes + PCs
// simultaneously. Association matching uses early termination for best-case O(branching_factor).
func (g *Graph) CheckAccess(userNodeID, objectNodeID, operation string) *AccessDecision {
	g.mu.RLock()
	defer g.mu.RUnlock()

	userNode := g.Nodes[userNodeID]
	objectNode := g.Nodes[objectNodeID]

	if userNode == nil || objectNode == nil {
		return &AccessDecision{
			Decision:  "DENY",
			User:      userNodeID,
			Object:    objectNodeID,
			Operation: operation,
			Explanation: AccessExplanation{
				Reason: "User or object node not found in graph",
			},
		}
	}

	// Single-pass BFS: collect UAs + PCs in one traversal (replaces 2 separate DFS calls)
	userUAs, userPCs := g.bfsCollectAttributesAndPCs(userNodeID, NodeTypeUserAttribute)

	// Single-pass BFS: collect OAs + PCs in one traversal (replaces 2 separate DFS calls)
	objectOAs, objectPCs := g.bfsCollectAttributesAndPCs(objectNodeID, NodeTypeObjectAttr)

	// Collect names for explanation output
	uaNames := g.collectNodeNames(userUAs, NodeTypeUserAttribute)
	oaNames := g.collectNodeNames(objectOAs, NodeTypeObjectAttr)

	// Find matching association with early termination
	if assocMatch := g.findMatchingAssociation(userUAs, objectOAs, userPCs, objectPCs, operation); assocMatch != nil {
		uaNode := g.Nodes[assocMatch.uaID]
		oaNode := g.Nodes[assocMatch.oaID]

		path := []string{
			fmt.Sprintf("%s → %s (user attribute)", userNode.Name, uaNode.Name),
			fmt.Sprintf("%s --[%s]--> %s (association)", uaNode.Name, operation, oaNode.Name),
			fmt.Sprintf("%s → %s (object attribute)", objectNode.Name, oaNode.Name),
		}

		return &AccessDecision{
			Decision:  "ALLOW",
			User:      userNode.Name,
			Object:    objectNode.Name,
			Operation: operation,
			Explanation: AccessExplanation{
				Path:             path,
				PolicyClass:      assocMatch.commonPC,
				UserAttributes:   uaNames,
				ObjectAttributes: oaNames,
			},
		}
	}

	return &AccessDecision{
		Decision:  "DENY",
		User:      userNode.Name,
		Object:    objectNode.Name,
		Operation: operation,
		Explanation: AccessExplanation{
			Reason:           "No association path found with matching operation and common policy class",
			UserAttributes:   uaNames,
			ObjectAttributes: oaNames,
		},
	}
}

// associationMatch holds the result of a successful association search.
type associationMatch struct {
	uaID     string
	oaID     string
	commonPC string
}

// findMatchingAssociation searches for an association granting the requested operation
// from any user UA to any object OA, requiring at least one common Policy Class.
// Returns on first match (early termination) instead of exhaustive search.
func (g *Graph) findMatchingAssociation(
	userUAs, objectOAs, userPCs, objectPCs map[string]bool,
	operation string,
) *associationMatch {
	for uaID := range userUAs {
		for _, assoc := range g.uaToAssociations[uaID] {
			if !objectOAs[assoc.OAID] {
				continue
			}
			if !containsOp(assoc.Operations, operation) {
				continue
			}
			// PC intersection: both user and object must share at least one PC
			if pc := findCommonPC(userPCs, objectPCs, g.Nodes); pc != "" {
				return &associationMatch{
					uaID:     uaID,
					oaID:     assoc.OAID,
					commonPC: pc,
				}
			}
		}
	}
	return nil
}

// findCommonPC returns the name of a common Policy Class between two PC sets.
// Iterates over the smaller set for efficiency.
func findCommonPC(userPCs, objectPCs map[string]bool, nodes map[string]*NGACNode) string {
	smaller, larger := userPCs, objectPCs
	if len(userPCs) > len(objectPCs) {
		smaller, larger = objectPCs, userPCs
	}
	for pc := range smaller {
		if larger[pc] {
			if n := nodes[pc]; n != nil {
				return n.Name
			}
		}
	}
	return ""
}

// collectNodeNames extracts node names of a specific type from a set of node IDs.
func (g *Graph) collectNodeNames(nodeIDs map[string]bool, nodeType string) []string {
	var names []string
	for id := range nodeIDs {
		if n := g.Nodes[id]; n != nil && n.NodeType == nodeType {
			names = append(names, n.Name)
		}
	}
	return names
}

func containsOp(ops []string, target string) bool {
	for _, op := range ops {
		if op == target {
			return true
		}
	}
	return false
}
