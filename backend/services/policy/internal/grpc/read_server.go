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

const readCacheTTL = 30 * time.Second

// ReadServer implements PolicyReadService with a 3-layer cache:
//   - L1: Redis (microsecond decisions, 30s TTL)
//   - L2: Materialized access table (millisecond, version-checked)
//   - L3: SQL CTE (full graph traversal, always correct)
type ReadServer struct {
	pb.UnimplementedPolicyReadServiceServer
	store        *ngac.Store
	constraint   *ngac.ConstraintEngine
	rdb          *redis.Client
	cte          *ngac.CTEEvaluator
	materialized *ngac.MaterializedAccess
	version      *ngac.VersionTracker
}

// NewReadServer creates a PolicyReadService with 3-layer caching.
func NewReadServer(
	store *ngac.Store,
	constraint *ngac.ConstraintEngine,
	rdb *redis.Client,
	cte *ngac.CTEEvaluator,
	materialized *ngac.MaterializedAccess,
	version *ngac.VersionTracker,
) *ReadServer {
	return &ReadServer{
		store:        store,
		constraint:   constraint,
		rdb:          rdb,
		cte:          cte,
		materialized: materialized,
		version:      version,
	}
}

// CheckAccess evaluates access via 3-layer cache: L1 Redis → L2 Materialized → L3 CTE.
func (s *ReadServer) CheckAccess(ctx context.Context, req *pb.CheckAccessRequest) (*pb.AccessDecision, error) {
	cacheKey := accessCacheKey(req.UserNodeId, req.ObjectNodeId, req.Operation)

	// L1: Redis cache
	if s.rdb != nil {
		if cached, err := s.getRedisCache(ctx, cacheKey); err == nil {
			return cached, nil
		}
	}

	// L2: Materialized access table
	if s.materialized != nil && s.version != nil {
		if decision, err := s.checkMaterialized(ctx, req); err == nil && decision != nil {
			s.setRedisCache(ctx, cacheKey, decision)
			return decision, nil
		}
	}

	// L3: Compute via in-memory graph (fast path) or CTE (fallback)
	resp := s.computeAccess(req)

	// Populate L2 + L1
	s.populateCaches(ctx, req, cacheKey, resp)

	return resp, nil
}

// checkMaterialized attempts L2 lookup with version freshness check.
func (s *ReadServer) checkMaterialized(ctx context.Context, req *pb.CheckAccessRequest) (*pb.AccessDecision, error) {
	currentVersion, err := s.version.GetVersion(ctx, "global")
	if err != nil {
		return nil, err
	}

	cached, err := s.materialized.Lookup(ctx, req.UserNodeId, req.ObjectNodeId, req.Operation, currentVersion)
	if err != nil || cached == nil {
		return nil, err
	}

	decision := "DENY"
	if cached.Decision {
		decision = "ALLOW"
	}

	return &pb.AccessDecision{
		Decision:  decision,
		User:      req.UserNodeId,
		Object:    req.ObjectNodeId,
		Operation: req.Operation,
		Explanation: &pb.AccessExplanation{
			Reason: "resolved from materialized cache",
		},
	}, nil
}

// computeAccess performs the actual graph traversal using the in-memory BFS engine.
func (s *ReadServer) computeAccess(req *pb.CheckAccessRequest) *pb.AccessDecision {
	graph := s.store.GetGraph()
	decision := graph.CheckAccess(req.UserNodeId, req.ObjectNodeId, req.Operation)

	resp := &pb.AccessDecision{
		Decision:  decision.Decision,
		User:      decision.User,
		Object:    decision.Object,
		Operation: decision.Operation,
		Explanation: &pb.AccessExplanation{
			Path:               decision.Explanation.Path,
			PolicyClass:        decision.Explanation.PolicyClass,
			UserAttributes:     decision.Explanation.UserAttributes,
			ObjectAttributes:   decision.Explanation.ObjectAttributes,
			Reason:             decision.Explanation.Reason,
			ConstraintsChecked: decision.Explanation.ConstraintsChecked,
		},
	}

	if decision.Decision == "ALLOW" && s.constraint != nil {
		reqCtx := ngac.RequestContext{
			Time: time.Now(), UserID: req.UserNodeId,
			ObjectID: req.ObjectNodeId, Operation: req.Operation,
		}
		denied, name, msg, checked := s.constraint.Evaluate(reqCtx)
		resp.Explanation.ConstraintsChecked = checked
		if denied {
			resp.Decision = "DENY"
			resp.Explanation.Reason = msg
			resp.Explanation.ConstraintDenied = &pb.ConstraintDenial{Name: name, Message: msg}
		}
	}

	return resp
}

// populateCaches stores a computed decision in L2 (materialized) and L1 (Redis).
func (s *ReadServer) populateCaches(ctx context.Context, req *pb.CheckAccessRequest, cacheKey string, resp *pb.AccessDecision) {
	if s.materialized != nil && s.version != nil {
		currentVersion, err := s.version.GetVersion(ctx, "global")
		if err == nil {
			allowed := resp.Decision == "ALLOW"
			if storeErr := s.materialized.Store(ctx, req.UserNodeId, req.ObjectNodeId, req.Operation, allowed, currentVersion); storeErr != nil {
				slog.Warn("failed to store materialized decision", "error", storeErr)
			}
		}
	}

	s.setRedisCache(ctx, cacheKey, resp)
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

// --- Redis helpers ---

func (s *ReadServer) getRedisCache(ctx context.Context, key string) (*pb.AccessDecision, error) {
	if s.rdb == nil {
		return nil, fmt.Errorf("redis not available")
	}
	data, err := s.rdb.Get(ctx, key).Bytes()
	if err != nil {
		return nil, err
	}
	var resp pb.AccessDecision
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("unmarshaling cached decision: %w", err)
	}
	return &resp, nil
}

func (s *ReadServer) setRedisCache(ctx context.Context, key string, resp *pb.AccessDecision) {
	if s.rdb == nil {
		return
	}
	data, err := json.Marshal(resp)
	if err != nil {
		return
	}
	if err := s.rdb.Set(ctx, key, data, readCacheTTL).Err(); err != nil {
		slog.Warn("failed to cache access decision", "key", key, "error", err)
	}
}
