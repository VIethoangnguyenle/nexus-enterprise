package ngac

import "sort"

// Graph-mutation kinds carried on the ngac.graph.mutated event. The writer
// (WriteServer) publishes them and the read replica (ReplicaGraphRefresher)
// interprets them, so both sides use these constants.
const (
	MutationCreateNode        = "create_node"
	MutationDeleteNode        = "delete_node"
	MutationCreateAssignment  = "create_assignment"
	MutationRemoveAssignment  = "remove_assignment"
	MutationCreateAssociation = "create_association"
	MutationRemoveAssociation = "remove_association"
	MutationCreateProhibition = "create_prohibition"
	MutationRemoveProhibition = "remove_prohibition"

	// MutationLoadGraph reports a full reload of the writer's graph. It carries
	// no node IDs: every decision may have changed.
	MutationLoadGraph = "load_graph"
)

// AffectedWorkspaces returns the workspaces whose shards contain any of
// nodeIDs: the workspace_id of every policy class that is one of the nodes or
// an ancestor of one. Sorted, without duplicates.
//
// It reads the graph as it is now, so for a removal it must be called BEFORE
// the node is removed — afterwards the node and its path to the policy class
// are gone and nothing resolves.
func AffectedWorkspaces(g GraphReader, nodeIDs ...string) []string {
	if g == nil {
		return nil
	}
	seen := make(map[string]bool)
	consider := func(n *NGACNode) {
		if n == nil || n.NodeType != NodeTypePolicyClass {
			return
		}
		if ws := n.Properties["workspace_id"]; ws != "" {
			seen[ws] = true
		}
	}
	for _, id := range nodeIDs {
		node := g.GetNode(id)
		if node == nil {
			continue
		}
		consider(node)
		for _, anc := range g.GetAncestors(id) {
			consider(anc)
		}
	}
	return sortedKeys(seen)
}

// RemovalImpact is what removing one node touches, resolved against the graph
// as it was before the removal.
type RemovalImpact struct {
	// NodeIDs is the node itself plus everything below it. Once the node is
	// gone the cache invalidator can no longer expand it (e.g. a UA into the
	// users it contained), so the expansion is captured up front.
	NodeIDs []string
	// Workspaces whose shards held the node.
	Workspaces []string
	// PolicyClass is true when the node is a policy class: every decision
	// within it may change, so only a full invalidation is safe.
	PolicyClass bool
}

// ResolveRemovalImpact must be called BEFORE nodeID is removed from g.
func ResolveRemovalImpact(g GraphReader, nodeID string) RemovalImpact {
	impact := RemovalImpact{NodeIDs: []string{nodeID}}
	if g == nil {
		return impact
	}
	node := g.GetNode(nodeID)
	if node == nil {
		return impact
	}
	impact.PolicyClass = node.NodeType == NodeTypePolicyClass
	impact.Workspaces = AffectedWorkspaces(g, nodeID)

	descendants := g.GetDescendants(nodeID)
	ids := make([]string, 0, len(descendants))
	for id := range descendants {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	impact.NodeIDs = append(impact.NodeIDs, ids...)
	return impact
}

// FirstWorkspace returns the first workspace ID, or "" (global scope) if none.
func FirstWorkspace(wsIDs []string) string {
	if len(wsIDs) > 0 {
		return wsIDs[0]
	}
	return ""
}

func sortedKeys(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
