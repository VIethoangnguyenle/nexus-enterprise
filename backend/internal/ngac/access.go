package ngac

import (
	"fmt"

	"ngac-document-platform/internal/models"
)

// CheckAccess performs the NGAC access decision
func (g *Graph) CheckAccess(userNodeID, objectNodeID, operation string) *models.AccessDecision {
	g.mu.RLock()
	defer g.mu.RUnlock()

	userNode := g.Nodes[userNodeID]
	objectNode := g.Nodes[objectNodeID]

	if userNode == nil || objectNode == nil {
		return &models.AccessDecision{
			Decision:  "DENY",
			User:      userNodeID,
			Object:    objectNodeID,
			Operation: operation,
			Explanation: models.AccessExplanation{
				Reason: "User or object node not found in graph",
			},
		}
	}

	// 1. Find all UAs reachable from user (upward traversal)
	userUAs := make(map[string]*models.NGACNode)
	userUAs[userNodeID] = userNode // include the user node itself
	visited := make(map[string]bool)
	g.collectAncestorsOfType(userNodeID, models.NodeTypeUserAttribute, userUAs, visited)

	// 2. Find all OAs reachable from object (upward traversal)
	objectOAs := make(map[string]*models.NGACNode)
	objectOAs[objectNodeID] = objectNode
	visited = make(map[string]bool)
	g.collectAncestorsOfType(objectNodeID, models.NodeTypeObjectAttr, objectOAs, visited)

	// 3. Find PCs reachable from user's UAs
	userPCs := make(map[string]bool)
	for uaID := range userUAs {
		uaVisited := make(map[string]bool)
		g.collectPCsReachable(uaID, userPCs, uaVisited)
	}

	// 4. Find PCs reachable from object's OAs
	objectPCs := make(map[string]bool)
	for oaID := range objectOAs {
		oaVisited := make(map[string]bool)
		g.collectPCsReachable(oaID, objectPCs, oaVisited)
	}

	// 5. Find associations from any user UA to any object OA with the requested operation
	var uaNames, oaNames []string
	for _, n := range userUAs {
		if n.NodeType == models.NodeTypeUserAttribute {
			uaNames = append(uaNames, n.Name)
		}
	}
	for _, n := range objectOAs {
		if n.NodeType == models.NodeTypeObjectAttr {
			oaNames = append(oaNames, n.Name)
		}
	}

	for uaID := range userUAs {
		assocs := g.uaToAssociations[uaID]
		for _, assoc := range assocs {
			// Check if this association's OA is in the object's OA set
			if _, inOAs := objectOAs[assoc.OAID]; !inOAs {
				continue
			}

			// Check if the operation is in the association
			if !containsOp(assoc.Operations, operation) {
				continue
			}

			// Check PC intersection: both user and object must share a PC
			// (or object is under PC_Global which is accessible)
			commonPC := ""
			for pc := range userPCs {
				if objectPCs[pc] {
					commonPC = g.Nodes[pc].Name
					break
				}
			}
			if commonPC == "" {
				// Check if PC_Global is in objectPCs (cross-tenant sharing)
				for pc := range objectPCs {
					if n := g.Nodes[pc]; n != nil && n.Name == "PC_Global" {
						commonPC = "PC_Global"
						break
					}
				}
			}

			if commonPC != "" {
				uaNode := g.Nodes[uaID]
				oaNode := g.Nodes[assoc.OAID]

				path := []string{
					fmt.Sprintf("%s → %s (user attribute)", userNode.Name, uaNode.Name),
					fmt.Sprintf("%s --[%s]--> %s (association)", uaNode.Name, operation, oaNode.Name),
					fmt.Sprintf("%s → %s (object attribute)", objectNode.Name, oaNode.Name),
				}

				return &models.AccessDecision{
					Decision:  "ALLOW",
					User:      userNode.Name,
					Object:    objectNode.Name,
					Operation: operation,
					Explanation: models.AccessExplanation{
						Path:             path,
						PolicyClass:      commonPC,
						UserAttributes:   uaNames,
						ObjectAttributes: oaNames,
					},
				}
			}
		}
	}

	return &models.AccessDecision{
		Decision:  "DENY",
		User:      userNode.Name,
		Object:    objectNode.Name,
		Operation: operation,
		Explanation: models.AccessExplanation{
			Reason:           "No association path found with matching operation and common policy class",
			UserAttributes:   uaNames,
			ObjectAttributes: oaNames,
		},
	}
}

func (g *Graph) collectAncestorsOfType(nodeID, nodeType string, result map[string]*models.NGACNode, visited map[string]bool) {
	if visited[nodeID] {
		return
	}
	visited[nodeID] = true

	if parents, ok := g.childToParents[nodeID]; ok {
		for pid := range parents {
			if n, ok := g.Nodes[pid]; ok {
				if n.NodeType == nodeType {
					result[pid] = n
				}
				g.collectAncestorsOfType(pid, nodeType, result, visited)
			}
		}
	}
}

func (g *Graph) collectPCsReachable(nodeID string, pcs map[string]bool, visited map[string]bool) {
	if visited[nodeID] {
		return
	}
	visited[nodeID] = true

	if n, ok := g.Nodes[nodeID]; ok && n.NodeType == models.NodeTypePolicyClass {
		pcs[nodeID] = true
		return
	}

	if parents, ok := g.childToParents[nodeID]; ok {
		for pid := range parents {
			g.collectPCsReachable(pid, pcs, visited)
		}
	}
}

func containsOp(ops []string, target string) bool {
	for _, op := range ops {
		if op == target {
			return true
		}
	}
	return false
}
