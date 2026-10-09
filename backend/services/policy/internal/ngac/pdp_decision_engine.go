package ngac

import (
	"context"
	"fmt"
	"log/slog"
)

// DecisionEngine computes access decisions from the NGAC graph.
// This is the PDP — pure decision logic, no caching.
type DecisionEngine interface {
	// Decide evaluates access using graph traversal + prohibition checks.
	// Returns the FINAL decision (includes prohibition deny overrides).
	//
	// When a step of the evaluation fails, the decision is DENY and
	// AccessDecision.ErrorDerived() reports true; such a decision must not be
	// cached.
	Decide(ctx context.Context, req AccessRequest) *AccessDecision
}

// ProhibitionFinder is the read side of the prohibition store the PDP needs.
// *ProhibitionStore implements it; tests substitute fakes.
type ProhibitionFinder interface {
	FindForSubjects(ctx context.Context, subjectIDs []string, operation string) ([]*Prohibition, error)
}

// CTEChecker answers an access question with the SQL recursive-CTE fallback,
// for objects that are not in the in-memory graph. *CTEEvaluator implements it.
type CTEChecker interface {
	CheckAccess(ctx context.Context, userNodeID, objectNodeID, operation string) (bool, error)
}

// Compile-time checks: the concrete stores satisfy the PDP's dependencies.
var (
	_ ProhibitionFinder = (*ProhibitionStore)(nil)
	_ CTEChecker        = (*CTEEvaluator)(nil)
)

// decisionEngine implements DecisionEngine using BFS traversal,
// CTE SQL fallback, shard-based evaluation, and prohibition evaluation.
type decisionEngine struct {
	graph        GraphReader
	cte          CTEChecker
	prohibitions ProhibitionFinder
	shardManager ShardManager
}

// NewDecisionEngine creates a PDP engine with graph reader, CTE fallback, and prohibition evaluation.
// cte and prohibitions may be nil to disable that step.
func NewDecisionEngine(graph GraphReader, cte CTEChecker, prohibitions ProhibitionFinder) DecisionEngine {
	// A typed nil pointer inside an interface is not == nil; normalise it so the
	// "step disabled" checks cannot be fooled into calling through a nil store.
	if c, ok := cte.(*CTEEvaluator); ok && c == nil {
		cte = nil
	}
	if p, ok := prohibitions.(*ProhibitionStore); ok && p == nil {
		prohibitions = nil
	}
	return &decisionEngine{
		graph:        graph,
		cte:          cte,
		prohibitions: prohibitions,
	}
}

// SetShardManager enables shard-based graph evaluation.
func (e *decisionEngine) SetShardManager(sm ShardManager) {
	e.shardManager = sm
}

// Decide performs the full NGAC access decision:
//  1. Resolve graph: shard (if workspace_id set) → global graph fallback
//  2. BFS graph traversal (in-memory)
//  3. CTE SQL fallback (if object node not in graph)
//  4. Prohibition evaluation (deny overrides on ALLOW)
//
// A step that fails closes the decision to DENY and marks it error-derived
// (AccessDecision.EvaluationErr), so the caller knows not to cache it.
func (e *decisionEngine) Decide(ctx context.Context, req AccessRequest) *AccessDecision {
	// Step 1: Resolve the graph to evaluate against
	graph := e.resolveGraph(ctx, req)

	// Step 2: BFS access check on resolved graph
	decision := graph.CheckAccess(req.UserNodeID, req.ObjectNodeID, req.Operation)

	// Step 3: CTE fallback for O nodes (not loaded into in-memory graph)
	e.tryCTEFallback(ctx, req, decision)

	// Step 4: Prohibition check: if BFS says ALLOW, check for deny overrides.
	if decision.Decision == DecisionAllow && e.prohibitions != nil {
		denied, prohibName, subjectID, err := e.checkProhibitions(ctx, req, graph)
		switch {
		case err != nil:
			// Fail closed, consistent with the batch path: a prohibition that
			// cannot be read might be the one that denies this request.
			slog.Error("prohibition lookup failed; denying",
				"user_node_id", req.UserNodeID, "object_node_id", req.ObjectNodeID,
				"operation", req.Operation, "error", err)
			decision.failClosed(DenyReasonProhibitionCheckFailed, err)
		case denied:
			decision.Decision = DecisionDeny
			decision.Explanation.Reason = fmt.Sprintf("Denied by prohibition %q", prohibName)
			decision.Explanation.ProhibitionDenied = &ProhibitionDenial{
				ProhibitionName: prohibName,
				SubjectID:       subjectID,
			}
		}
	}

	return decision
}

// tryCTEFallback promotes DENY→ALLOW when CTE succeeds for O nodes not loaded in graph.
//
// A CTE failure leaves the decision DENY but marks it error-derived: that DENY
// reflects a failed query, not the policy, and must not be cached.
func (e *decisionEngine) tryCTEFallback(ctx context.Context, req AccessRequest, decision *AccessDecision) {
	if decision.Decision != DecisionDeny || decision.Explanation.Reason != DenyReasonNodeNotFound {
		return
	}
	if e.cte == nil {
		return
	}
	allowed, err := e.cte.CheckAccess(ctx, req.UserNodeID, req.ObjectNodeID, req.Operation)
	if err != nil {
		slog.Warn("CTE fallback failed; denying",
			"user_node_id", req.UserNodeID, "object_node_id", req.ObjectNodeID,
			"operation", req.Operation, "error", err)
		decision.failClosed(DenyReasonCTEFallbackFailed, err)
		return
	}
	if !allowed {
		return
	}
	decision.Decision = DecisionAllow
	decision.Explanation.Reason = "Resolved via CTE fallback (O node not in graph)"
	e.triggerAsyncShardPromotion(req)
}

// triggerAsyncShardPromotion loads the workspace shard in background after CTE fallback,
// so the next access check for this workspace can use the fast in-memory path.
func (e *decisionEngine) triggerAsyncShardPromotion(req AccessRequest) {
	if req.WorkspaceID == "" || e.shardManager == nil {
		return
	}
	go func() {
		if _, err := e.shardManager.GetGraph(context.Background(), req.WorkspaceID); err != nil {
			slog.Warn("async shard promotion failed",
				"workspace_id", req.WorkspaceID, "error", err)
		}
	}()
}

// resolveGraph returns the best available graph for the request.
// Priority: shard (if workspace_id set and available) → global graph.
func (e *decisionEngine) resolveGraph(ctx context.Context, req AccessRequest) GraphReader {
	if req.WorkspaceID != "" && e.shardManager != nil {
		shardGraph, err := e.shardManager.GetGraph(ctx, req.WorkspaceID)
		if err == nil {
			return shardGraph
		}
		slog.Debug("shard miss, falling back to global graph",
			"workspace_id", req.WorkspaceID, "error", err)
	}
	return e.graph
}

// checkProhibitions evaluates all applicable prohibitions for an access request.
// The graph parameter must be the same resolved graph used for BFS evaluation
// to prevent prohibition bypass when nodes exist only in a shard.
//
// A non-nil error means the prohibitions could not be evaluated; the caller
// must treat that as DENY, never as "no prohibition".
func (e *decisionEngine) checkProhibitions(ctx context.Context, req AccessRequest, graph GraphReader) (bool, string, string, error) {
	// Step 1: Collect user + all UA ancestors (prohibition subjects)
	subjectIDs := []string{req.UserNodeID}
	ancestors := graph.GetAncestors(req.UserNodeID)
	for id, node := range ancestors {
		if node.NodeType == NodeTypeUserAttribute {
			subjectIDs = append(subjectIDs, id)
		}
	}

	// Step 2: Query prohibitions matching subjects + operation
	prohibitions, err := e.prohibitions.FindForSubjects(ctx, subjectIDs, req.Operation)
	if err != nil {
		return false, "", "", fmt.Errorf("query prohibitions: %w", err)
	}
	if len(prohibitions) == 0 {
		return false, "", "", nil
	}

	// Step 3: Collect object's OA ancestors (prohibition targets)
	objectOAIDs := make(map[string]bool)
	objectOAIDs[req.ObjectNodeID] = true // include self
	objAncestors := graph.GetAncestors(req.ObjectNodeID)
	for id, node := range objAncestors {
		if node.NodeType == NodeTypeObjectAttr {
			objectOAIDs[id] = true
		}
	}

	// Step 4: Match prohibitions against object's OA set
	denied, name, subject := matchProhibitions(prohibitions, objectOAIDs)
	return denied, name, subject, nil
}

// --- PDP: Prohibition matching logic ---

// matchProhibitions evaluates whether any prohibitions deny the access.
// Returns (denied bool, prohibitionName string, subjectID string).
//
// Algorithm:
//   - For each matching prohibition:
//     intersection=false → ANY target_oa_id in objectOAIDs → DENY
//     intersection=true  → ALL target_oa_ids in objectOAIDs → DENY
func matchProhibitions(prohibitions []*Prohibition, objectOAIDs map[string]bool) (bool, string, string) {
	for _, p := range prohibitions {
		if p.Intersection {
			// ALL targets must match
			allMatch := true
			for _, targetOA := range p.TargetOAIDs {
				if !objectOAIDs[targetOA] {
					allMatch = false
					break
				}
			}
			if allMatch {
				return true, p.Name, p.SubjectID
			}
		} else {
			// ANY target matches
			for _, targetOA := range p.TargetOAIDs {
				if objectOAIDs[targetOA] {
					return true, p.Name, p.SubjectID
				}
			}
		}
	}
	return false, "", ""
}
