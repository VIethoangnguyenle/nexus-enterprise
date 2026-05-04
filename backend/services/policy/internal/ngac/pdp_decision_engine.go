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
	Decide(ctx context.Context, req AccessRequest) *AccessDecision
}

// decisionEngine implements DecisionEngine using BFS traversal,
// CTE SQL fallback, shard-based evaluation, and prohibition evaluation.
type decisionEngine struct {
	graph        GraphReader
	cte          *CTEEvaluator
	prohibitions *ProhibitionStore
	shardManager ShardManager
}

// NewDecisionEngine creates a PDP engine with graph reader, CTE fallback, and prohibition evaluation.
func NewDecisionEngine(graph GraphReader, cte *CTEEvaluator, prohibitions *ProhibitionStore) DecisionEngine {
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
func (e *decisionEngine) Decide(ctx context.Context, req AccessRequest) *AccessDecision {
	// Step 1: Resolve the graph to evaluate against
	graph := e.resolveGraph(ctx, req)

	// Step 2: BFS access check on resolved graph
	decision := graph.CheckAccess(req.UserNodeID, req.ObjectNodeID, req.Operation)

	// Step 3: CTE fallback for O nodes (not loaded into in-memory graph)
	e.tryCTEFallback(ctx, req, decision)

	// Step 4: Prohibition check: if BFS says ALLOW, check for deny overrides.
	if decision.Decision == DecisionAllow && e.prohibitions != nil {
		if denied, prohibName, subjectID := e.checkProhibitions(ctx, req, graph); denied {
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
func (e *decisionEngine) tryCTEFallback(ctx context.Context, req AccessRequest, decision *AccessDecision) {
	if decision.Decision != DecisionDeny || decision.Explanation.Reason != DenyReasonNodeNotFound {
		return
	}
	if e.cte == nil {
		return
	}
	allowed, err := e.cte.CheckAccess(ctx, req.UserNodeID, req.ObjectNodeID, req.Operation)
	if err != nil || !allowed {
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
func (e *decisionEngine) checkProhibitions(ctx context.Context, req AccessRequest, graph GraphReader) (bool, string, string) {
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
		slog.Warn("failed to query prohibitions", "error", err)
		return false, "", ""
	}
	if len(prohibitions) == 0 {
		return false, "", ""
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
	return matchProhibitions(prohibitions, objectOAIDs)
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
