package grpc

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "ngac-platform/proto/policy"
	"ngac-platform/services/policy/internal/ngac"
)

// ReadServer implements PolicyReadService.
// Access evaluation is fully delegated to AccessEvaluator (cache orchestrator + PDP).
// This struct handles only proto conversion and graph query RPCs.
type ReadServer struct {
	pb.UnimplementedPolicyReadServiceServer
	store        *ngac.Store
	rdb          *redis.Client // used only by ResolveAccessibleScopes (scope caching)
	evaluator    *ngac.AccessEvaluator
	operations   *ngac.OperationStore
	prohibitions *ngac.ProhibitionStore
}

// NewReadServer creates a PolicyReadService with decoupled access evaluation.
func NewReadServer(
	store *ngac.Store,
	rdb *redis.Client,
	evaluator *ngac.AccessEvaluator,
	operations *ngac.OperationStore,
	prohibitions *ngac.ProhibitionStore,
) *ReadServer {
	return &ReadServer{
		store:        store,
		rdb:          rdb,
		evaluator:    evaluator,
		operations:   operations,
		prohibitions: prohibitions,
	}
}

// CheckAccess evaluates access via the AccessEvaluator (L1 Redis → L2 Materialized → L3 BFS/CTE + prohibitions).
func (s *ReadServer) CheckAccess(ctx context.Context, req *pb.CheckAccessRequest) (*pb.AccessDecision, error) {
	accessReq := ngac.AccessRequest{
		UserNodeID:   req.UserNodeId,
		ObjectNodeID: req.ObjectNodeId,
		Operation:    req.Operation,
	}
	if req.WorkspaceId != nil {
		accessReq.WorkspaceID = *req.WorkspaceId
	}
	decision := s.evaluator.Evaluate(ctx, accessReq)
	return decisionToProto(decision), nil
}

// BatchCheckAccess resolves permissions for multiple objects and operations in
// a single RPC. Reuses the 3-layer cached CheckAccess path for each pair.
func (s *ReadServer) BatchCheckAccess(ctx context.Context, req *pb.BatchCheckAccessRequest) (*pb.BatchAccessResult, error) {
	if req.UserNodeId == "" {
		return nil, status.Error(codes.InvalidArgument, "user_node_id required")
	}

	results := make(map[string]*pb.ObjectPermissions, len(req.ObjectIds))

	for _, objID := range req.ObjectIds {
		perms := make(map[string]bool, len(req.Operations))
		for _, op := range req.Operations {
			decision, err := s.CheckAccess(ctx, &pb.CheckAccessRequest{
				UserNodeId:   req.UserNodeId,
				ObjectNodeId: objID,
				Operation:    op,
			})
			if err != nil {
				perms[op] = false
				continue
			}
			perms[op] = decision.Decision == ngac.DecisionAllow
		}
		results[objID] = &pb.ObjectPermissions{Permissions: perms}
	}

	return &pb.BatchAccessResult{Results: results}, nil
}

// decisionToProto converts internal AccessDecision to proto format.
func decisionToProto(d *ngac.AccessDecision) *pb.AccessDecision {
	resp := &pb.AccessDecision{
		Decision:  d.Decision,
		User:      d.User,
		Object:    d.Object,
		Operation: d.Operation,
		Explanation: &pb.AccessExplanation{
			Path:             d.Explanation.Path,
			PolicyClasses:    d.Explanation.PolicyClasses,
			UserAttributes:   d.Explanation.UserAttributes,
			ObjectAttributes: d.Explanation.ObjectAttributes,
			Reason:           d.Explanation.Reason,
		},
	}
	if d.Explanation.ProhibitionDenied != nil {
		resp.Explanation.ProhibitionDenied = &pb.ProhibitionDenial{
			ProhibitionName: d.Explanation.ProhibitionDenied.ProhibitionName,
			SubjectId:       d.Explanation.ProhibitionDenied.SubjectID,
		}
	}
	return resp
}

// --- Read-only graph query RPCs ---

func (s *ReadServer) GetNode(ctx context.Context, req *pb.GetNodeRequest) (*pb.NGACNode, error) {
	node := s.store.GetNode(req.NodeId)
	if node == nil {
		return nil, status.Errorf(codes.NotFound, "node %s not found", req.NodeId)
	}
	return nodeToProto(node), nil
}

func (s *ReadServer) FindNodeByName(ctx context.Context, req *pb.FindNodeByNameRequest) (*pb.NGACNode, error) {
	node := s.store.FindNodeByName(req.Name, req.NodeType)
	if node == nil {
		return nil, status.Errorf(codes.NotFound, "node %s (%s) not found", req.Name, req.NodeType)
	}
	return nodeToProto(node), nil
}

func (s *ReadServer) GetNodesByType(ctx context.Context, req *pb.GetNodesByTypeRequest) (*pb.NodeList, error) {
	nodes := s.store.GetNodesByType(req.NodeType)
	return &pb.NodeList{Nodes: nodesToProto(nodes)}, nil
}

func (s *ReadServer) IsAssigned(ctx context.Context, req *pb.IsAssignedRequest) (*pb.BoolResponse, error) {
	return &pb.BoolResponse{Value: s.store.IsAssigned(req.ChildId, req.ParentId)}, nil
}

func (s *ReadServer) GetAncestors(ctx context.Context, req *pb.GetAncestorsRequest) (*pb.NodeList, error) {
	graph := s.store.GetGraph()
	ancestors := graph.GetAncestors(req.NodeId)
	var nodes []*ngac.NGACNode
	for _, n := range ancestors {
		nodes = append(nodes, n)
	}
	return &pb.NodeList{Nodes: nodesToProto(nodes)}, nil
}

func (s *ReadServer) GetDescendants(ctx context.Context, req *pb.GetDescendantsRequest) (*pb.NodeList, error) {
	graph := s.store.GetGraph()
	desc := graph.GetDescendants(req.NodeId)
	var nodes []*ngac.NGACNode
	for _, n := range desc {
		nodes = append(nodes, n)
	}
	return &pb.NodeList{Nodes: nodesToProto(nodes)}, nil
}

func (s *ReadServer) GetChildren(ctx context.Context, req *pb.GetChildrenRequest) (*pb.NodeList, error) {
	graph := s.store.GetGraph()
	children := graph.GetChildren(req.NodeId)
	return &pb.NodeList{Nodes: nodesToProto(children)}, nil
}

func (s *ReadServer) GetParents(ctx context.Context, req *pb.GetParentsRequest) (*pb.NodeList, error) {
	graph := s.store.GetGraph()
	parents := graph.GetParents(req.NodeId)
	return &pb.NodeList{Nodes: nodesToProto(parents)}, nil
}



const scopeCacheTTL = 60 * time.Second

// ResolveAccessibleScopes returns the set of leaf OA IDs a user can access
// for a given operation. Algorithm:
//   1. BFS upward from user → find all UA ancestors
//   2. For each UA ancestor, find associations with the requested operation → collect target OA IDs
//   3. For each target OA, BFS downward → flatten to leaf OA IDs (no OA children)
// Results are cached in Redis (key: scopes:{user}:{op}) with 60s TTL.
func (s *ReadServer) ResolveAccessibleScopes(ctx context.Context, req *pb.ResolveAccessibleScopesRequest) (*pb.ResolveAccessibleScopesResponse, error) {
	if req.UserNodeId == "" || req.Operation == "" {
		return nil, status.Error(codes.InvalidArgument, "user_node_id and operation required")
	}

	// Check Redis cache
	// NOTE: scope cache key does not yet include workspace_id because
	// ResolveAccessibleScopesRequest proto lacks that field.
	// This will be addressed when proto is updated in a separate change.
	cacheKey := fmt.Sprintf("scopes:%s:%s", req.UserNodeId, req.Operation)
	if s.rdb != nil {
		if cached, err := s.getCachedScopes(ctx, cacheKey); err == nil {
			return cached, nil
		}
	}

	// Step 1: BFS upward from user → collect UA ancestors (including self if UA)
	graph := s.store.GetGraph()
	userNode := graph.GetNode(req.UserNodeId)
	if userNode == nil {
		return nil, status.Errorf(codes.NotFound, "user node %s not found", req.UserNodeId)
	}

	uaIDs := s.collectUAAncestors(graph, req.UserNodeId, userNode)

	// Step 2: For each UA, find associations granting the requested operation → collect target OA IDs
	targetOAIDs := s.collectTargetOAs(graph, uaIDs, req.Operation)

	// Step 3: For each target OA, find leaf descendants (OA nodes with no OA children)
	leafOAIDs := s.resolveLeafOAs(graph, targetOAIDs)

	resp := &pb.ResolveAccessibleScopesResponse{
		ScopeOaIds: leafOAIDs,
	}

	// Cache result
	if s.rdb != nil {
		s.setCachedScopes(ctx, cacheKey, resp)
	}

	return resp, nil
}

// collectUAAncestors returns all UA node IDs reachable from the user (BFS upward).
func (s *ReadServer) collectUAAncestors(graph ngac.GraphReader, userNodeID string, userNode *ngac.NGACNode) []string {
	var uaIDs []string

	// Include self if UA type
	if userNode.NodeType == ngac.NodeTypeUserAttribute {
		uaIDs = append(uaIDs, userNodeID)
	}

	ancestors := graph.GetAncestors(userNodeID)
	for id, node := range ancestors {
		if node.NodeType == ngac.NodeTypeUserAttribute {
			uaIDs = append(uaIDs, id)
		}
	}
	return uaIDs
}

// collectTargetOAs finds all OA IDs associated with the given UAs for an operation.
func (s *ReadServer) collectTargetOAs(graph ngac.GraphReader, uaIDs []string, operation string) map[string]bool {
	targetOAs := make(map[string]bool)
	for _, uaID := range uaIDs {
		assocs := graph.GetAssociationsFromUA(uaID)
		for _, assoc := range assocs {
			for _, op := range assoc.Operations {
				if op == operation || op == "*" {
					targetOAs[assoc.OAID] = true
					break
				}
			}
		}
	}
	return targetOAs
}

// resolveLeafOAs traverses down from each target OA to find leaf OA nodes.
// A leaf is an OA that has no OA-type children.
func (s *ReadServer) resolveLeafOAs(graph ngac.GraphReader, targetOAs map[string]bool) []string {
	seen := make(map[string]bool)

	for oaID := range targetOAs {
		descendants := graph.GetDescendants(oaID)

		// Collect all OA-type descendants
		oaDescendants := make(map[string]bool)
		for id, node := range descendants {
			if node.NodeType == ngac.NodeTypeObjectAttr {
				oaDescendants[id] = true
			}
		}

		// If no OA descendants, the target itself is a leaf
		if len(oaDescendants) == 0 {
			seen[oaID] = true
			continue
		}

		// Find leaves: OA nodes that have no OA-type children
		for id := range oaDescendants {
			if !hasOAChildren(graph, id) {
				seen[id] = true
			}
		}
	}

	result := make([]string, 0, len(seen))
	for id := range seen {
		result = append(result, id)
	}
	return result
}

// hasOAChildren checks whether a node has any children of type OA.
func hasOAChildren(graph ngac.GraphReader, nodeID string) bool {
	for _, child := range graph.GetChildren(nodeID) {
		if child.NodeType == ngac.NodeTypeObjectAttr {
			return true
		}
	}
	return false
}

// --- Scope cache helpers ---

func (s *ReadServer) getCachedScopes(ctx context.Context, key string) (*pb.ResolveAccessibleScopesResponse, error) {
	data, err := s.rdb.Get(ctx, key).Bytes()
	if err != nil {
		return nil, err
	}
	var resp pb.ResolveAccessibleScopesResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (s *ReadServer) setCachedScopes(ctx context.Context, key string, resp *pb.ResolveAccessibleScopesResponse) {
	data, err := json.Marshal(resp)
	if err != nil {
		return
	}
	if err := s.rdb.Set(ctx, key, data, scopeCacheTTL).Err(); err != nil {
		slog.Warn("failed to cache scopes", "key", key, "error", err)
	}
}

// --- Operations + Prohibitions read-only RPCs ---

// ListOperations returns all registered operations.
func (s *ReadServer) ListOperations(ctx context.Context, _ *pb.Empty) (*pb.OperationList, error) {
	if s.operations == nil {
		return &pb.OperationList{}, nil
	}
	ops, err := s.operations.List(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "listing operations: %v", err)
	}
	return &pb.OperationList{Operations: ops}, nil
}

// ListProhibitions returns all prohibitions, optionally filtered by subject_id.
func (s *ReadServer) ListProhibitions(ctx context.Context, req *pb.ListProhibitionsRequest) (*pb.ProhibitionList, error) {
	if s.prohibitions == nil {
		return &pb.ProhibitionList{}, nil
	}
	prohibitions, err := s.prohibitions.List(ctx, req.SubjectId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "listing prohibitions: %v", err)
	}

	var result []*pb.Prohibition
	for _, p := range prohibitions {
		result = append(result, &pb.Prohibition{
			Id:           p.ID,
			Name:         p.Name,
			SubjectId:    p.SubjectID,
			Operations:   p.Operations,
			TargetOaIds:  p.TargetOAIDs,
			Intersection: p.Intersection,
		})
	}
	return &pb.ProhibitionList{Prohibitions: result}, nil
}
