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
//   - prohibitions are looked up once per operation instead of once per
//     (object, operation) pair.
//
// Prohibitions are read from the in-memory graph; the batch path touches the
// database only for the CTE fallback.
func (e *decisionEngine) DecideBatch(ctx context.Context, req BatchAccessRequest) map[string]map[string]bool {
	graph := e.resolveGraph(ctx, AccessRequest{
		UserNodeID:  req.UserNodeID,
		WorkspaceID: req.WorkspaceID,
	})

	results := graph.CheckAccessBatch(req.UserNodeID, req.ObjectNodeIDs, req.Operations)

	e.applyCTEFallbackBatch(ctx, graph, req, results)
	e.applyProhibitionsBatch(graph, req, results)

	return results
}

// applyCTEFallbackBatch promotes DENY→ALLOW for objects that are not in the
// in-memory graph at all (every object, when the user is the missing node),
// matching the single-object fallback.
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
	// Prohibition data is computed lazily, the first time the SQL fallback is about
	// to hand out an ALLOW, and once per batch: the user's covering prohibitions
	// per operation, an object's targets per object (see cteProhibited).
	type covering struct {
		list []*Prohibition
		err  error
	}
	coveringByOp := make(map[string]covering, len(req.Operations))
	// A user missing from memory takes the fallback for every object, as the
	// single check does (either node missing is a node_not_found there).
	userMissing := graph.GetNode(req.UserNodeID) == nil
	promoted := false
	for objID, perms := range results {
		if !userMissing && graph.GetNode(objID) != nil {
			continue // both in the graph — the traversal already gave the final answer
		}
		var targets map[string]bool // this object's, resolved at most once
		for _, op := range req.Operations {
			if perms[op] {
				continue
			}
			allowed, err := e.cte.CheckAccess(ctx, req.UserNodeID, objID, op)
			if err != nil || !allowed {
				continue
			}
			c, ok := coveringByOp[op]
			if !ok {
				c.list, c.err = e.cteCoveringProhibitions(ctx, graph, req.UserNodeID, op)
				coveringByOp[op] = c
			}
			if c.err != nil {
				slog.Warn("CTE prohibition lookup failed; denying",
					"user_node_id", req.UserNodeID, "operation", op, "error", c.err)
				continue
			}
			if len(c.list) > 0 {
				if targets == nil {
					if targets, err = e.cteTargetSet(ctx, graph, objID); err != nil {
						slog.Warn("CTE prohibition lookup failed; denying",
							"user_node_id", req.UserNodeID, "object_node_id", objID, "error", err)
						continue
					}
				}
				if denied, _, _ := matchProhibitions(c.list, targets); denied {
					continue
				}
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
	graph GraphReader, req BatchAccessRequest, results map[string]map[string]bool,
) {
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

	// Looked up once per operation (an in-memory index read) rather than once
	// per (object, operation). Most deployments hold few or no prohibitions, so
	// this usually returns nothing and the per-object work below is skipped.
	perOperation := make(map[string][]*Prohibition, len(req.Operations))
	anyProhibition := false
	for _, op := range req.Operations {
		found := e.graph.ProhibitionsForSubjects(subjectIDs, op)
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
