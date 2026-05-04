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
			Decision:  DecisionDeny,
			User:      userNodeID,
			Object:    objectNodeID,
			Operation: operation,
			Explanation: AccessExplanation{
				Reason: DenyReasonNodeNotFound,
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
			Decision:  DecisionAllow,
			User:      userNode.Name,
			Object:    objectNode.Name,
			Operation: operation,
			Explanation: AccessExplanation{
				Path:             path,
				PolicyClasses:    assocMatch.matchedPCs,
				UserAttributes:   uaNames,
				ObjectAttributes: oaNames,
			},
		}
	}

	return &AccessDecision{
		Decision:  DecisionDeny,
		User:      userNode.Name,
		Object:    objectNode.Name,
		Operation: operation,
		Explanation: AccessExplanation{
			Reason:           DenyReasonNoAssociation,
			UserAttributes:   uaNames,
			ObjectAttributes: oaNames,
		},
	}
}

// associationMatch holds the result of a successful association search.
type associationMatch struct {
	uaID       string
	oaID       string
	matchedPCs []string
}

// findMatchingAssociation searches for an association granting the requested operation
// from any user UA to any object OA, requiring ALL object PCs to be covered by user PCs.
// Returns on first match (early termination) instead of exhaustive search.
//
// NIST NGAC spec: objectPCs ⊆ userPCs (ALL-intersection, not single-intersection).
func (g *Graph) findMatchingAssociation(
	userUAs, objectOAs, userPCs, objectPCs map[string]bool,
	operation string,
) *associationMatch {
	// Pre-check: ALL-PC intersection — every PC the object reaches,
	// the user must also reach. Checked once, not per-association.
	matched := allPCsSatisfied(objectPCs, userPCs, g.Nodes)
	if matched == nil {
		return nil
	}

	for uaID := range userUAs {
		for _, assoc := range g.uaToAssociations[uaID] {
			if !objectOAs[assoc.OAID] {
				continue
			}
			if !containsOp(assoc.Operations, operation) {
				continue
			}
			return &associationMatch{
				uaID:       uaID,
				oaID:       assoc.OAID,
				matchedPCs: matched,
			}
		}
	}
	return nil
}

// allPCsSatisfied checks that ALL PCs the object reaches are also reached by the user.
// Returns the list of matched PC names if satisfied, nil otherwise.
//
// NIST NGAC spec requires: objectPCs ⊆ userPCs.
// Previous implementation (findCommonPC) only checked for ANY common PC — incorrect for Multi-PC.
func allPCsSatisfied(objectPCs, userPCs map[string]bool, nodes map[string]*NGACNode) []string {
	if len(objectPCs) == 0 {
		return nil
	}
	matched := make([]string, 0, len(objectPCs))
	for pc := range objectPCs {
		if !userPCs[pc] {
			return nil // user missing a required PC → DENY
		}
		if n := nodes[pc]; n != nil {
			matched = append(matched, n.Name)
		}
	}
	return matched
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
