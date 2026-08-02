package ngac

// CheckAccessBatch answers several objects for one user in a single pass.
//
// A per-object loop over CheckAccess re-walks the user's attribute chain for
// every object, because each call resolves both sides from scratch. That side
// does not change within a batch: listing a folder of 200 files asks about one
// user and 200 objects, so the user-side traversal — and the read lock around
// it — is paid 200 times to produce the same answer.
//
// This resolves the user side once and then walks only the object side per
// item. The decision rule is unchanged: same association matching, same
// ALL-PC intersection. It is an evaluation-order optimisation, not a policy
// change, and pdp_batch_test.go pins it against the single-object path
// object-by-object so the two cannot drift.
//
// Returns object ID → operation → allowed. Objects that are not in the graph
// are present in the result with every operation false, so a caller can tell
// "denied" from "absent from the response" without a second lookup.
func (g *Graph) CheckAccessBatch(userNodeID string, objectNodeIDs []string, operations []string) map[string]map[string]bool {
	results := make(map[string]map[string]bool, len(objectNodeIDs))

	deny := func() map[string]bool {
		perms := make(map[string]bool, len(operations))
		for _, op := range operations {
			perms[op] = false
		}
		return perms
	}

	g.mu.RLock()
	defer g.mu.RUnlock()

	if g.Nodes[userNodeID] == nil {
		for _, objID := range objectNodeIDs {
			results[objID] = deny()
		}
		return results
	}

	// The one traversal this whole method exists to hoist.
	userUAs, userPCs := g.bfsCollectAttributesAndPCs(userNodeID, NodeTypeUserAttribute)

	for _, objID := range objectNodeIDs {
		if _, seen := results[objID]; seen {
			continue // duplicate ID in the request
		}
		if g.Nodes[objID] == nil {
			results[objID] = deny()
			continue
		}

		objectOAs, objectPCs := g.bfsCollectAttributesAndPCs(objID, NodeTypeObjectAttr)

		// The PC intersection does not depend on the operation, so it is
		// checked once per object rather than once per operation.
		if allPCsSatisfied(objectPCs, userPCs, g.Nodes) == nil {
			results[objID] = deny()
			continue
		}

		perms := make(map[string]bool, len(operations))
		for _, op := range operations {
			perms[op] = g.hasMatchingAssociation(userUAs, objectOAs, op)
		}
		results[objID] = perms
	}

	return results
}

// hasMatchingAssociation reports whether any association from the user's
// attributes to the object's attributes grants the operation.
//
// The caller holds the read lock and has already established that the PC
// intersection holds.
func (g *Graph) hasMatchingAssociation(userUAs, objectOAs map[string]bool, operation string) bool {
	for uaID := range userUAs {
		for _, assoc := range g.uaToAssociations[uaID] {
			if objectOAs[assoc.OAID] && containsOp(assoc.Operations, operation) {
				return true
			}
		}
	}
	return false
}
