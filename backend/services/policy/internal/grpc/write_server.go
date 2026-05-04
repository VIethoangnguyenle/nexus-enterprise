package grpc

import (
	"context"
	"log/slog"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "ngac-platform/proto/policy"
	"ngac-platform/services/policy/internal/events"
	"ngac-platform/services/policy/internal/ngac"
)

// WriteServer implements PolicyWriteService as a singleton mutation handler.
// Every write operation: persists → delegates invalidation to coordinator → publishes event.
type WriteServer struct {
	pb.UnimplementedPolicyWriteServiceServer
	store        *ngac.Store
	producer     *events.Producer
	invalidation *ngac.InvalidationCoordinator
	operations   *ngac.OperationStore
	prohibitions *ngac.ProhibitionStore
	shardManager ngac.ShardManager
	strictOps    bool
}

// NewWriteServer creates a PolicyWriteService with coordinated invalidation and event publishing.
func NewWriteServer(
	store *ngac.Store,
	producer *events.Producer,
	invalidation *ngac.InvalidationCoordinator,
	operations *ngac.OperationStore,
	prohibitions *ngac.ProhibitionStore,
	strictOps bool,
) *WriteServer {
	return &WriteServer{
		store:        store,
		producer:     producer,
		invalidation: invalidation,
		operations:   operations,
		prohibitions: prohibitions,
		strictOps:    strictOps,
	}
}

// SetShardManager enables shard-level cache invalidation for per-tenant graphs.
func (s *WriteServer) SetShardManager(sm ngac.ShardManager) {
	s.shardManager = sm
}

func (s *WriteServer) CreateNode(ctx context.Context, req *pb.CreateNodeRequest) (*pb.NGACNode, error) {
	// PC Authorization Guard: PolicyClass nodes require explicit scope + tenant_id metadata.
	// This prevents unauthorized/accidental PC creation which would break tenant isolation.
	if req.NodeType == ngac.NodeTypePolicyClass {
		scope := req.Properties["scope"]
		tenantID := req.Properties["tenant_id"]

		if scope == "" {
			return nil, status.Errorf(codes.PermissionDenied,
				"creating PolicyClass requires properties.scope (tenant|global|project|classification)")
		}

		// Non-global scopes require tenant_id for ownership tracking
		if scope != "global" && tenantID == "" {
			return nil, status.Errorf(codes.PermissionDenied,
				"creating PolicyClass with scope=%q requires properties.tenant_id", scope)
		}

		slog.Info("creating PolicyClass node",
			"name", req.Name, "scope", scope, "tenant_id", tenantID)
	}

	props := make(map[string]string)
	for k, v := range req.Properties {
		props[k] = v
	}
	node, err := s.store.CreateNode(ctx, req.Name, req.NodeType, props)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "create node: %v", err)
	}

	// Invalidate caches and publish event (consistent with all other mutations)
	wsIDs := s.invalidateShards(node.ID)
	s.invalidation.InvalidateForNodes(ctx, firstWorkspace(wsIDs), node.ID)
	s.publishEvent("create_node", []string{node.ID})

	return nodeToProto(node), nil
}

// DeleteNode removes a node and invalidates affected caches.
func (s *WriteServer) DeleteNode(ctx context.Context, req *pb.DeleteNodeRequest) (*pb.Empty, error) {
	if err := s.store.DeleteNode(ctx, req.NodeId); err != nil {
		return nil, status.Errorf(codes.Internal, "delete node: %v", err)
	}

	wsIDs := s.invalidateShards(req.NodeId)
	s.invalidation.InvalidateForNodes(ctx, firstWorkspace(wsIDs), req.NodeId)
	s.publishEvent("delete_node", []string{req.NodeId})

	return &pb.Empty{}, nil
}

// CreateAssignment modifies graph structure — targeted cache invalidation.
func (s *WriteServer) CreateAssignment(ctx context.Context, req *pb.CreateAssignmentRequest) (*pb.Assignment, error) {
	a, err := s.store.CreateAssignment(ctx, req.ChildId, req.ParentId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "create assignment: %v", err)
	}

	wsIDs := s.invalidateShards(req.ChildId, req.ParentId)
	s.invalidation.InvalidateForNodes(ctx, firstWorkspace(wsIDs), req.ChildId, req.ParentId)
	s.publishEvent("create_assignment", []string{req.ChildId, req.ParentId})

	return &pb.Assignment{Id: a.ID, ChildId: a.ChildID, ParentId: a.ParentID}, nil
}

// RemoveAssignment modifies graph structure — targeted cache invalidation.
func (s *WriteServer) RemoveAssignment(ctx context.Context, req *pb.RemoveAssignmentRequest) (*pb.Empty, error) {
	if err := s.store.RemoveAssignment(ctx, req.ChildId, req.ParentId); err != nil {
		return nil, status.Errorf(codes.Internal, "remove assignment: %v", err)
	}

	wsIDs := s.invalidateShards(req.ChildId, req.ParentId)
	s.invalidation.InvalidateForNodes(ctx, firstWorkspace(wsIDs), req.ChildId, req.ParentId)
	s.publishEvent("remove_assignment", []string{req.ChildId, req.ParentId})

	return &pb.Empty{}, nil
}

// CreateAssociation modifies permissions — targeted cache invalidation.
// When STRICT_OPERATIONS=true, validates that all operations are registered.
func (s *WriteServer) CreateAssociation(ctx context.Context, req *pb.CreateAssociationRequest) (*pb.Association, error) {
	// Strict mode: reject unregistered operations
	if s.strictOps && s.operations != nil {
		invalid, err := s.operations.ValidateOperations(ctx, req.Operations)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "validating operations: %v", err)
		}
		if len(invalid) > 0 {
			return nil, status.Errorf(codes.InvalidArgument,
				"unregistered operations: %v — register them first via RegisterOperations RPC", invalid)
		}
	}

	a, err := s.store.CreateAssociation(ctx, req.UaId, req.OaId, req.Operations)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "create association: %v", err)
	}

	wsIDs := s.invalidateShards(req.UaId, req.OaId)
	s.invalidation.InvalidateForNodes(ctx, firstWorkspace(wsIDs), req.UaId, req.OaId)
	s.publishEvent("create_association", []string{req.UaId, req.OaId})

	return &pb.Association{Id: a.ID, UaId: a.UAID, OaId: a.OAID, Operations: a.Operations}, nil
}

// RemoveAssociation modifies permissions — targeted cache invalidation.
func (s *WriteServer) RemoveAssociation(ctx context.Context, req *pb.RemoveAssociationRequest) (*pb.Empty, error) {
	if err := s.store.RemoveAssociationByUAOA(ctx, req.UaId, req.OaId); err != nil {
		return nil, status.Errorf(codes.Internal, "remove association: %v", err)
	}

	wsIDs := s.invalidateShards(req.UaId, req.OaId)
	s.invalidation.InvalidateForNodes(ctx, firstWorkspace(wsIDs), req.UaId, req.OaId)
	s.publishEvent("remove_association", []string{req.UaId, req.OaId})

	return &pb.Empty{}, nil
}

func (s *WriteServer) InitSchema(ctx context.Context, _ *pb.Empty) (*pb.Empty, error) {
	if err := s.store.InitSchema(ctx); err != nil {
		return nil, status.Errorf(codes.Internal, "init schema: %v", err)
	}
	return &pb.Empty{}, nil
}

// LoadGraph reloads the graph and invalidates all caches.
func (s *WriteServer) LoadGraph(ctx context.Context, _ *pb.Empty) (*pb.Empty, error) {
	if err := s.store.LoadGraph(ctx); err != nil {
		return nil, status.Errorf(codes.Internal, "load graph: %v", err)
	}

	// Full invalidation on graph reload
	s.invalidation.InvalidateAll(ctx)

	// Full shard invalidation on graph reload
	if s.shardManager != nil {
		s.shardManager.InvalidateAll()
	}

	return &pb.Empty{}, nil
}



func (s *WriteServer) publishEvent(action string, nodeIDs []string) {
	if s.producer != nil {
		s.producer.PublishGraphMutated(action, nodeIDs)
	}
}

// invalidateShards resolves workspace_id from affected nodes and invalidates their shards.
// Returns the set of affected workspace IDs for per-workspace version bumping.
func (s *WriteServer) invalidateShards(nodeIDs ...string) []string {
	if s.shardManager == nil || s.store == nil {
		return nil
	}
	graph := s.store.GetGraph()
	if graph == nil {
		return nil
	}

	seen := make(map[string]bool)
	for _, nodeID := range nodeIDs {
		node := graph.GetNode(nodeID)
		if node == nil {
			continue
		}
		// Walk up to find the PC with workspace_id
		ancestors := graph.GetAncestors(nodeID)
		for _, anc := range ancestors {
			if anc.NodeType == ngac.NodeTypePolicyClass && anc.Properties["workspace_id"] != "" {
				wsID := anc.Properties["workspace_id"]
				if !seen[wsID] {
					seen[wsID] = true
					s.shardManager.InvalidateShard(wsID)
				}
			}
		}
	}

	result := make([]string, 0, len(seen))
	for wsID := range seen {
		result = append(result, wsID)
	}
	return result
}

// --- New RPCs: Operations, Prohibitions, InvalidateCache ---

// RegisterOperations registers domain-specific operations (idempotent).
func (s *WriteServer) RegisterOperations(ctx context.Context, req *pb.RegisterOperationsRequest) (*pb.RegisterOperationsResponse, error) {
	if s.operations == nil {
		return nil, status.Error(codes.Unimplemented, "operations store not configured")
	}

	result, err := s.operations.Register(ctx, req.Operations)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "register operations: %v", err)
	}

	return &pb.RegisterOperationsResponse{
		Registered:    result.Registered,
		AlreadyExists: result.AlreadyExist,
	}, nil
}

// InvalidateCache allows external consumers to trigger targeted cache invalidation.
func (s *WriteServer) InvalidateCache(ctx context.Context, req *pb.InvalidateCacheRequest) (*pb.InvalidateCacheResponse, error) {
	if len(req.NodeIds) == 0 {
		return nil, status.Error(codes.InvalidArgument, "at least one node_id required")
	}

	slog.Info("external cache invalidation", "node_ids", req.NodeIds, "reason", req.Reason)

	wsIDs := s.invalidateShards(req.NodeIds...)
	s.invalidation.InvalidateForNodes(ctx, firstWorkspace(wsIDs), req.NodeIds...)

	return &pb.InvalidateCacheResponse{
		L1KeysDeleted: 0,
		L2RowsDeleted: 0,
		NewVersion:    0, // version is managed by InvalidationCoordinator internally
	}, nil
}

// CreateProhibition creates a deny override and invalidates affected caches.
func (s *WriteServer) CreateProhibition(ctx context.Context, req *pb.CreateProhibitionRequest) (*pb.Prohibition, error) {
	if s.prohibitions == nil {
		return nil, status.Error(codes.Unimplemented, "prohibition store not configured")
	}

	p, err := s.prohibitions.Create(ctx, &ngac.Prohibition{
		Name:         req.Name,
		SubjectID:    req.SubjectId,
		Operations:   req.Operations,
		TargetOAIDs:  req.TargetOaIds,
		Intersection: req.Intersection,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "create prohibition: %v", err)
	}

	affectedNodes := s.resolveProhibitionAffectedNodes(req.SubjectId, req.TargetOaIds)
	wsIDs := s.invalidateShards(affectedNodes...)
	s.invalidation.InvalidateForNodes(ctx, firstWorkspace(wsIDs), affectedNodes...)
	s.publishEvent("create_prohibition", affectedNodes)

	return &pb.Prohibition{
		Id:          p.ID,
		Name:        p.Name,
		SubjectId:   p.SubjectID,
		Operations:  p.Operations,
		TargetOaIds: p.TargetOAIDs,
		Intersection: p.Intersection,
	}, nil
}

// RemoveProhibition deletes a deny override and invalidates affected caches.
func (s *WriteServer) RemoveProhibition(ctx context.Context, req *pb.RemoveProhibitionRequest) (*pb.Empty, error) {
	if s.prohibitions == nil {
		return nil, status.Error(codes.Unimplemented, "prohibition store not configured")
	}

	// Fetch before delete to know which caches to invalidate
	p, err := s.prohibitions.GetByName(ctx, req.Name)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "prohibition %q not found", req.Name)
	}

	if err := s.prohibitions.Remove(ctx, req.Name); err != nil {
		return nil, status.Errorf(codes.Internal, "remove prohibition: %v", err)
	}

	affectedNodes := s.resolveProhibitionAffectedNodes(p.SubjectID, p.TargetOAIDs)
	wsIDs := s.invalidateShards(affectedNodes...)
	s.invalidation.InvalidateForNodes(ctx, firstWorkspace(wsIDs), affectedNodes...)
	s.publishEvent("remove_prohibition", affectedNodes)

	return &pb.Empty{}, nil
}

// resolveProhibitionAffectedNodes collects all node IDs affected by a prohibition change.
// Includes the subject, all target OAs, and (if subject is a UA) all descendant users.
func (s *WriteServer) resolveProhibitionAffectedNodes(subjectID string, targetOAIDs []string) []string {
	affected := make([]string, 0, 1+len(targetOAIDs))
	affected = append(affected, subjectID)
	affected = append(affected, targetOAIDs...)

	graph := s.store.GetGraph()
	subjectNode := graph.GetNode(subjectID)
	if subjectNode != nil && subjectNode.NodeType == ngac.NodeTypeUserAttribute {
		descendants := graph.GetDescendants(subjectID)
		for id, node := range descendants {
			if node.NodeType == ngac.NodeTypeUser {
				affected = append(affected, id)
			}
		}
	}

	return affected
}

// firstWorkspace returns the first workspace ID from a slice, or empty string if none.
// Used to pass workspace context from shard invalidation to version bumping.
func firstWorkspace(wsIDs []string) string {
	if len(wsIDs) > 0 {
		return wsIDs[0]
	}
	return ""
}
