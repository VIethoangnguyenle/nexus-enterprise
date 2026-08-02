package ngac

import (
	"context"
	"log/slog"
)

// BatchAccessRequest asks one question about many objects for a single user.
type BatchAccessRequest struct {
	UserNodeID    string
	ObjectNodeIDs []string
	Operations    []string
	WorkspaceID   string // Optional: enables shard-based evaluation when set
}

// DecideBatch evaluates many objects for one user in a single pass.
//
// It produces exactly what a loop over Decide would produce — graph traversal,
// CTE fallback for objects outside the in-memory graph, and prohibitions as
// deny-overrides on an ALLOW — but hoists everything that does not vary across
// the batch:
//
//   - the graph is resolved once instead of once per object;
//   - the user's attribute and policy-class sets are walked once (inside
//     CheckAccessBatch) instead of once per object;
//   - the user's prohibition subjects are collected once instead of once per
//     object;
//   - prohibitions are queried once per operation instead of once per
//     (object, operation) pair.
//
// The last two matter most. Prohibition checking runs on the ALLOW path, which
// is the common one for a list the user can mostly see, and it previously
// re-walked the user's ancestors and re-queried the prohibition store for every
// single item on the page.
func (e *decisionEngine) DecideBatch(ctx context.Context, req BatchAccessRequest) map[string]map[string]bool {
	graph := e.resolveGraph(ctx, AccessRequest{
		UserNodeID:  req.UserNodeID,
		WorkspaceID: req.WorkspaceID,
	})

	results := graph.CheckAccessBatch(req.UserNodeID, req.ObjectNodeIDs, req.Operations)

	e.applyCTEFallbackBatch(ctx, graph, req, results)
	e.applyProhibitionsBatch(ctx, graph, req, results)

	return results
}

// applyCTEFallbackBatch promotes DENY→ALLOW for objects that are not in the
// in-memory graph at all, matching the single-object fallback.
//
// Objects live in Postgres rather than the graph, so a check on one of them
// misses in memory and has to be answered by the CTE. Only those objects take
// this path; anything already resolved from the graph is left alone.
func (e *decisionEngine) applyCTEFallbackBatch(
	ctx context.Context, graph GraphReader, req BatchAccessRequest, results map[string]map[string]bool,
) {
	if e.cte == nil {
		return
	}
	promoted := false
	for objID, perms := range results {
		if graph.GetNode(objID) != nil {
			continue // in the graph — the traversal already gave the final answer
		}
		for _, op := range req.Operations {
			if perms[op] {
				continue
			}
			allowed, err := e.cte.CheckAccess(ctx, req.UserNodeID, objID, op)
			if err != nil || !allowed {
				continue
			}
			perms[op] = true
			promoted = true
		}
	}
	if promoted {
		e.triggerAsyncShardPromotion(AccessRequest{
			UserNodeID:  req.UserNodeID,
			WorkspaceID: req.WorkspaceID,
		})
	}
}

// applyProhibitionsBatch turns ALLOW into DENY wherever a prohibition applies.
//
// Prohibitions are deny-overrides evaluated after the traversal, never an
// independent grant, so this only ever flips true to false.
func (e *decisionEngine) applyProhibitionsBatch(
	ctx context.Context, graph GraphReader, req BatchAccessRequest, results map[string]map[string]bool,
) {
	if e.prohibitions == nil {
		return
	}

	// Any ALLOW at all? If the whole page is denied there is nothing to override.
	anyAllowed := false
	for _, perms := range results {
		for _, allowed := range perms {
			if allowed {
				anyAllowed = true
				break
			}
		}
		if anyAllowed {
			break
		}
	}
	if !anyAllowed {
		return
	}

	// Collected once: the same user is the subject for every object.
	subjectIDs := []string{req.UserNodeID}
	for id, node := range graph.GetAncestors(req.UserNodeID) {
		if node.NodeType == NodeTypeUserAttribute {
			subjectIDs = append(subjectIDs, id)
		}
	}

	// Queried once per operation rather than once per (object, operation).
	// Most deployments hold few or no prohibitions, so this usually returns
	// nothing and the per-object work below is skipped entirely.
	perOperation := make(map[string][]*Prohibition, len(req.Operations))
	anyProhibition := false
	for _, op := range req.Operations {
		found, err := e.prohibitions.FindForSubjects(ctx, subjectIDs, op)
		if err != nil {
			// Fail closed: a prohibition that cannot be read might be the one
			// that denies this request, so nothing may be reported as allowed.
			slog.Error("prohibition lookup failed; denying the batch",
				"user_node_id", req.UserNodeID, "operation", op, "error", err)
			for _, perms := range results {
				perms[op] = false
			}
			continue
		}
		if len(found) > 0 {
			anyProhibition = true
		}
		perOperation[op] = found
	}
	if !anyProhibition {
		return
	}

	for objID, perms := range results {
		needsCheck := false
		for _, op := range req.Operations {
			if perms[op] && len(perOperation[op]) > 0 {
				needsCheck = true
				break
			}
		}
		if !needsCheck {
			continue
		}

		objectOAIDs := map[string]bool{objID: true}
		for id, node := range graph.GetAncestors(objID) {
			if node.NodeType == NodeTypeObjectAttr {
				objectOAIDs[id] = true
			}
		}

		for _, op := range req.Operations {
			if !perms[op] {
				continue
			}
			if denied, _, _ := matchProhibitions(perOperation[op], objectOAIDs); denied {
				perms[op] = false
			}
		}
	}
}
