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
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "ngac-platform/proto/policy"
	"ngac-platform/services/policy/internal/events"
	"ngac-platform/services/policy/internal/ngac"
)

const accessCacheTTL = 30 * time.Second

// PolicyServer implements the gRPC PolicyService with optional Redis caching and Kafka events.
type PolicyServer struct {
	pb.UnimplementedPolicyServiceServer
	store      *ngac.Store
	constraint *ngac.ConstraintEngine
	rdb        *redis.Client
	producer   *events.Producer
}

// NewPolicyServer creates a PolicyServer with optional Redis caching and Kafka event producer.
func NewPolicyServer(store *ngac.Store, constraint *ngac.ConstraintEngine, rdb *redis.Client, producer *events.Producer) *PolicyServer {
	return &PolicyServer{store: store, constraint: constraint, rdb: rdb, producer: producer}
}

// CheckAccess evaluates access with Redis caching (30s TTL) for performance.
func (s *PolicyServer) CheckAccess(ctx context.Context, req *pb.CheckAccessRequest) (*pb.AccessDecision, error) {
	cacheKey := accessCacheKey(req.UserNodeId, req.ObjectNodeId, req.Operation)

	if s.rdb != nil {
		if cached, err := s.getFromCache(ctx, cacheKey); err == nil {
			s.producer.PublishAccessChecked(req.UserNodeId, req.ObjectNodeId, req.Operation, cached.Decision, true)
			return cached, nil
		}
	}

	resp := s.computeAccess(req)

	if s.rdb != nil {
		s.setCache(ctx, cacheKey, resp)
	}

	s.producer.PublishAccessChecked(req.UserNodeId, req.ObjectNodeId, req.Operation, resp.Decision, false)

	return resp, nil
}

// computeAccess performs the actual NGAC graph traversal and constraint check.
func (s *PolicyServer) computeAccess(req *pb.CheckAccessRequest) *pb.AccessDecision {
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

func (s *PolicyServer) CreateNode(ctx context.Context, req *pb.CreateNodeRequest) (*pb.NGACNode, error) {
	props := make(map[string]string)
	for k, v := range req.Properties {
		props[k] = v
	}
	node, err := s.store.CreateNode(ctx, req.Name, req.NodeType, props)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "create node: %v", err)
	}
	return nodeToProto(node), nil
}

// DeleteNode removes a node and invalidates access cache.
func (s *PolicyServer) DeleteNode(ctx context.Context, req *pb.DeleteNodeRequest) (*pb.Empty, error) {
	if err := s.store.DeleteNode(ctx, req.NodeId); err != nil {
		return nil, status.Errorf(codes.Internal, "delete node: %v", err)
	}
	s.invalidateCache(ctx)
	s.producer.PublishGraphMutated("delete_node", []string{req.NodeId})
	return &pb.Empty{}, nil
}

func (s *PolicyServer) GetNode(ctx context.Context, req *pb.GetNodeRequest) (*pb.NGACNode, error) {
	node := s.store.GetNode(req.NodeId)
	if node == nil {
		return nil, status.Errorf(codes.NotFound, "node %s not found", req.NodeId)
	}
	return nodeToProto(node), nil
}

func (s *PolicyServer) FindNodeByName(ctx context.Context, req *pb.FindNodeByNameRequest) (*pb.NGACNode, error) {
	node := s.store.FindNodeByName(req.Name, req.NodeType)
	if node == nil {
		return nil, status.Errorf(codes.NotFound, "node %s (%s) not found", req.Name, req.NodeType)
	}
	return nodeToProto(node), nil
}

func (s *PolicyServer) GetNodesByType(ctx context.Context, req *pb.GetNodesByTypeRequest) (*pb.NodeList, error) {
	nodes := s.store.GetNodesByType(req.NodeType)
	return &pb.NodeList{Nodes: nodesToProto(nodes)}, nil
}

// CreateAssignment modifies graph structure — invalidates access cache.
func (s *PolicyServer) CreateAssignment(ctx context.Context, req *pb.CreateAssignmentRequest) (*pb.Assignment, error) {
	a, err := s.store.CreateAssignment(ctx, req.ChildId, req.ParentId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "create assignment: %v", err)
	}
	s.invalidateCache(ctx)
	s.producer.PublishGraphMutated("create_assignment", []string{req.ChildId, req.ParentId})
	return &pb.Assignment{Id: a.ID, ChildId: a.ChildID, ParentId: a.ParentID}, nil
}

// RemoveAssignment modifies graph structure — invalidates access cache.
func (s *PolicyServer) RemoveAssignment(ctx context.Context, req *pb.RemoveAssignmentRequest) (*pb.Empty, error) {
	if err := s.store.RemoveAssignment(ctx, req.ChildId, req.ParentId); err != nil {
		return nil, status.Errorf(codes.Internal, "remove assignment: %v", err)
	}
	s.invalidateCache(ctx)
	s.producer.PublishGraphMutated("remove_assignment", []string{req.ChildId, req.ParentId})
	return &pb.Empty{}, nil
}

func (s *PolicyServer) IsAssigned(ctx context.Context, req *pb.IsAssignedRequest) (*pb.BoolResponse, error) {
	return &pb.BoolResponse{Value: s.store.IsAssigned(req.ChildId, req.ParentId)}, nil
}

// CreateAssociation modifies permissions — invalidates access cache.
func (s *PolicyServer) CreateAssociation(ctx context.Context, req *pb.CreateAssociationRequest) (*pb.Association, error) {
	a, err := s.store.CreateAssociation(ctx, req.UaId, req.OaId, req.Operations)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "create association: %v", err)
	}
	s.invalidateCache(ctx)
	s.producer.PublishGraphMutated("create_association", []string{req.UaId, req.OaId})
	return &pb.Association{Id: a.ID, UaId: a.UAID, OaId: a.OAID, Operations: a.Operations}, nil
}

// RemoveAssociation modifies permissions — invalidates access cache.
func (s *PolicyServer) RemoveAssociation(ctx context.Context, req *pb.RemoveAssociationRequest) (*pb.Empty, error) {
	if err := s.store.RemoveAssociationByUAOA(ctx, req.UaId, req.OaId); err != nil {
		return nil, status.Errorf(codes.Internal, "remove association: %v", err)
	}
	s.invalidateCache(ctx)
	return &pb.Empty{}, nil
}

func (s *PolicyServer) GetAncestors(ctx context.Context, req *pb.GetAncestorsRequest) (*pb.NodeList, error) {
	graph := s.store.GetGraph()
	ancestors := graph.GetAncestors(req.NodeId)
	var nodes []*ngac.NGACNode
	for _, n := range ancestors {
		nodes = append(nodes, n)
	}
	return &pb.NodeList{Nodes: nodesToProto(nodes)}, nil
}

func (s *PolicyServer) GetDescendants(ctx context.Context, req *pb.GetDescendantsRequest) (*pb.NodeList, error) {
	graph := s.store.GetGraph()
	desc := graph.GetDescendants(req.NodeId)
	var nodes []*ngac.NGACNode
	for _, n := range desc {
		nodes = append(nodes, n)
	}
	return &pb.NodeList{Nodes: nodesToProto(nodes)}, nil
}

func (s *PolicyServer) GetChildren(ctx context.Context, req *pb.GetChildrenRequest) (*pb.NodeList, error) {
	graph := s.store.GetGraph()
	children := graph.GetChildren(req.NodeId)
	return &pb.NodeList{Nodes: nodesToProto(children)}, nil
}

func (s *PolicyServer) GetParents(ctx context.Context, req *pb.GetParentsRequest) (*pb.NodeList, error) {
	graph := s.store.GetGraph()
	parents := graph.GetParents(req.NodeId)
	return &pb.NodeList{Nodes: nodesToProto(parents)}, nil
}

func (s *PolicyServer) InitSchema(ctx context.Context, _ *pb.Empty) (*pb.Empty, error) {
	if err := s.store.InitSchema(ctx); err != nil {
		return nil, status.Errorf(codes.Internal, "init schema: %v", err)
	}
	return &pb.Empty{}, nil
}

// LoadGraph reloads the graph and invalidates all cached access decisions.
func (s *PolicyServer) LoadGraph(ctx context.Context, _ *pb.Empty) (*pb.Empty, error) {
	if err := s.store.LoadGraph(ctx); err != nil {
		return nil, status.Errorf(codes.Internal, "load graph: %v", err)
	}
	s.invalidateCache(ctx)
	return &pb.Empty{}, nil
}

// --- Redis cache helpers ---

// accessCacheKey builds a deterministic cache key for an access decision.
func accessCacheKey(userID, objectID, op string) string {
	return fmt.Sprintf("ngac:access:%s:%s:%s", userID, objectID, op)
}

// getFromCache retrieves a cached access decision from Redis.
func (s *PolicyServer) getFromCache(ctx context.Context, key string) (*pb.AccessDecision, error) {
	data, err := s.rdb.Get(ctx, key).Bytes()
	if err != nil {
		return nil, err
	}
	var resp pb.AccessDecision
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("unmarshaling cached access decision: %w", err)
	}
	return &resp, nil
}

// setCache stores an access decision in Redis with TTL.
func (s *PolicyServer) setCache(ctx context.Context, key string, resp *pb.AccessDecision) {
	data, err := json.Marshal(resp)
	if err != nil {
		return
	}
	if err := s.rdb.Set(ctx, key, data, accessCacheTTL).Err(); err != nil {
		slog.Warn("failed to cache access decision", "key", key, "error", err)
	}
}

// invalidateCache flushes all access cache entries after graph mutations.
func (s *PolicyServer) invalidateCache(ctx context.Context) {
	if s.rdb == nil {
		return
	}
	iter := s.rdb.Scan(ctx, 0, "ngac:access:*", 1000).Iterator()
	var keys []string
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}
	if len(keys) > 0 {
		if err := s.rdb.Del(ctx, keys...).Err(); err != nil {
			slog.Warn("failed to invalidate access cache", "count", len(keys), "error", err)
		} else {
			slog.Debug("access cache invalidated", "keys_deleted", len(keys))
		}
	}
}

// --- Proto helpers ---

func nodeToProto(n *ngac.NGACNode) *pb.NGACNode {
	props := make(map[string]string)
	for k, v := range n.Properties {
		props[k] = v
	}
	return &pb.NGACNode{
		Id: n.ID, Name: n.Name, NodeType: n.NodeType,
		Properties: props, CreatedAt: timestamppb.New(n.CreatedAt),
	}
}

func nodesToProto(nodes []*ngac.NGACNode) []*pb.NGACNode {
	var result []*pb.NGACNode
	for _, n := range nodes {
		result = append(result, nodeToProto(n))
	}
	return result
}
