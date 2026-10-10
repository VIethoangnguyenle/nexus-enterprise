package grpc

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"ngac-platform/pkg/grpcauth"
	"ngac-platform/pkg/realtime"
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
	realtime     realtime.Emitter
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
	// The graph holds attributes, not objects: files, messages, assets and
	// requests live in Postgres under a parent OA and are authorized on it. An
	// O node would never be loaded into the in-memory graph and every check on
	// it would fall through to the SQL fallback.
	if req.NodeType == ngac.NodeTypeObject {
		return nil, status.Errorf(codes.InvalidArgument,
			"object (O) nodes are not created: authorize on the parent object attribute instead")
	}

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
		return nil, grpcauth.Internal(fmt.Errorf("create node: %w", err))
	}

	// Invalidate caches and publish event (consistent with all other mutations)
	s.invalidateShards(node.ID)
	s.invalidation.InvalidateForNodes(ctx, node.ID)
	s.publishEvent(ngac.MutationCreateNode, []string{node.ID})

	return nodeToProto(node), nil
}

// DeleteNode removes a node and invalidates affected caches.
//
// What the deletion touches is resolved BEFORE the delete and invalidated
// AFTER it. Resolving afterwards finds nothing: the node, its path up to its
// policy class (hence its workspace shard) and its descendants (the users of a
// UA, the sub-containers of an OA) are no longer reachable in the graph, so the
// shard kept serving the deleted node and its users kept their cached ALLOWs.
// Invalidating before the delete would let a concurrent check re-cache the old
// answer in between.
func (s *WriteServer) DeleteNode(ctx context.Context, req *pb.DeleteNodeRequest) (*pb.Empty, error) {
	impact := ngac.ResolveRemovalImpact(s.store.GetGraph(), req.NodeId)
	// Who the node reached is only knowable while it is still in the graph.
	var users []string
	if s.realtime != nil {
		users = affectedUsers(s.store.GetGraph(), req.NodeId)
	}

	if err := s.store.DeleteNode(ctx, req.NodeId); err != nil {
		return nil, grpcauth.Internal(fmt.Errorf("delete node: %w", err))
	}

	s.invalidateResolvedShards(impact.Workspaces)
	if impact.PolicyClass {
		// Everything inside the policy class may have changed.
		s.invalidation.InvalidateAll(ctx)
	} else {
		s.invalidation.InvalidateForNodes(ctx, impact.NodeIDs...)
	}
	s.publishEvent(ngac.MutationDeleteNode, []string{req.NodeId})
	s.announcePermissionChange(users, impact.Workspaces)

	return &pb.Empty{}, nil
}

// CreateAssignment modifies graph structure — targeted cache invalidation.
func (s *WriteServer) CreateAssignment(ctx context.Context, req *pb.CreateAssignmentRequest) (*pb.Assignment, error) {
	a, err := s.store.CreateAssignment(ctx, req.ChildId, req.ParentId)
	if err != nil {
		return nil, grpcauth.Internal(fmt.Errorf("create assignment: %w", err))
	}

	s.invalidateShards(req.ChildId, req.ParentId)
	s.invalidation.InvalidateForNodes(ctx, req.ChildId, req.ParentId)
	s.publishEvent(ngac.MutationCreateAssignment, []string{req.ChildId, req.ParentId})
	s.announceUsersOf(req.ChildId, req.ParentId)

	return &pb.Assignment{Id: a.ID, ChildId: a.ChildID, ParentId: a.ParentID}, nil
}

// RemoveAssignment modifies graph structure — targeted cache invalidation.
func (s *WriteServer) RemoveAssignment(ctx context.Context, req *pb.RemoveAssignmentRequest) (*pb.Empty, error) {
	if err := s.store.RemoveAssignment(ctx, req.ChildId, req.ParentId); err != nil {
		return nil, grpcauth.Internal(fmt.Errorf("remove assignment: %w", err))
	}

	s.invalidateShards(req.ChildId, req.ParentId)
	s.invalidation.InvalidateForNodes(ctx, req.ChildId, req.ParentId)
	s.publishEvent(ngac.MutationRemoveAssignment, []string{req.ChildId, req.ParentId})
	s.announceUsersOf(req.ChildId, req.ParentId)

	return &pb.Empty{}, nil
}

// CreateAssociation modifies permissions — targeted cache invalidation.
// When STRICT_OPERATIONS=true, validates that all operations are registered.
func (s *WriteServer) CreateAssociation(ctx context.Context, req *pb.CreateAssociationRequest) (*pb.Association, error) {
	// Strict mode: reject unregistered operations
	if s.strictOps && s.operations != nil {
		invalid, err := s.operations.ValidateOperations(ctx, req.Operations)
		if err != nil {
			return nil, grpcauth.Internal(fmt.Errorf("validating operations: %w", err))
		}
		if len(invalid) > 0 {
			return nil, status.Errorf(codes.InvalidArgument,
				"unregistered operations: %v — register them first via RegisterOperations RPC", invalid)
		}
	}

	a, err := s.store.CreateAssociation(ctx, req.UaId, req.OaId, req.Operations)
	if err != nil {
		if errors.Is(err, ngac.ErrInvalidAssociation) {
			return nil, status.Errorf(codes.InvalidArgument, "create association: %v", err)
		}
		return nil, grpcauth.Internal(fmt.Errorf("create association: %w", err))
	}

	s.invalidateShards(req.UaId, req.OaId)
	s.invalidation.InvalidateForNodes(ctx, req.UaId, req.OaId)
	s.publishEvent(ngac.MutationCreateAssociation, []string{req.UaId, req.OaId})
	s.announceUsersOf(req.UaId, req.OaId)

	return &pb.Association{Id: a.ID, UaId: a.UAID, OaId: a.OAID, Operations: a.Operations}, nil
}

// RemoveAssociation modifies permissions — targeted cache invalidation.
func (s *WriteServer) RemoveAssociation(ctx context.Context, req *pb.RemoveAssociationRequest) (*pb.Empty, error) {
	if err := s.store.RemoveAssociationByUAOA(ctx, req.UaId, req.OaId); err != nil {
		return nil, grpcauth.Internal(fmt.Errorf("remove association: %w", err))
	}

	s.invalidateShards(req.UaId, req.OaId)
	s.invalidation.InvalidateForNodes(ctx, req.UaId, req.OaId)
	s.publishEvent(ngac.MutationRemoveAssociation, []string{req.UaId, req.OaId})
	s.announceUsersOf(req.UaId, req.OaId)

	return &pb.Empty{}, nil
}

// GetAssociations answers from the writer's own graph, which every write has
// already updated.
func (s *WriteServer) GetAssociations(ctx context.Context, req *pb.GetAssociationsRequest) (*pb.AssociationList, error) {
	return associationsOf(s.store, req.UaId)
}

// The graph reads below answer from the writer's own graph, the same way the
// read service does from its graph (see ReadServer).
func (s *WriteServer) reads() *ReadServer { return &ReadServer{store: s.store} }

func (s *WriteServer) GetNode(ctx context.Context, req *pb.GetNodeRequest) (*pb.NGACNode, error) {
	return s.reads().GetNode(ctx, req)
}

func (s *WriteServer) GetChildren(ctx context.Context, req *pb.GetChildrenRequest) (*pb.NodeList, error) {
	return s.reads().GetChildren(ctx, req)
}

func (s *WriteServer) GetParents(ctx context.Context, req *pb.GetParentsRequest) (*pb.NodeList, error) {
	return s.reads().GetParents(ctx, req)
}

func (s *WriteServer) GetAncestors(ctx context.Context, req *pb.GetAncestorsRequest) (*pb.NodeList, error) {
	return s.reads().GetAncestors(ctx, req)
}

func (s *WriteServer) GetDescendants(ctx context.Context, req *pb.GetDescendantsRequest) (*pb.NodeList, error) {
	return s.reads().GetDescendants(ctx, req)
}

func (s *WriteServer) InitSchema(ctx context.Context, _ *pb.Empty) (*pb.Empty, error) {
	if err := s.store.InitSchema(ctx); err != nil {
		return nil, grpcauth.Internal(fmt.Errorf("init schema: %w", err))
	}
	return &pb.Empty{}, nil
}

// LoadGraph reloads the graph and invalidates all caches.
func (s *WriteServer) LoadGraph(ctx context.Context, _ *pb.Empty) (*pb.Empty, error) {
	if err := s.store.LoadGraph(ctx); err != nil {
		return nil, grpcauth.Internal(fmt.Errorf("load graph: %w", err))
	}

	// Full invalidation on graph reload
	s.invalidation.InvalidateAll(ctx)

	// Full shard invalidation on graph reload
	if s.shardManager != nil {
		s.shardManager.InvalidateAll()
	}

	// Read replicas hold their own graph; tell them to reload theirs too.
	s.publishEvent(ngac.MutationLoadGraph, nil)

	return &pb.Empty{}, nil
}

func (s *WriteServer) publishEvent(action string, nodeIDs []string) {
	if s.producer != nil {
		s.producer.PublishGraphMutated(action, nodeIDs)
	}
}

// invalidateShards resolves workspace_id from affected nodes and invalidates their shards.
// Returns the set of affected workspace IDs for per-workspace version bumping.
//
// It resolves against the current graph, so it is only correct for nodes that
// still exist; a removal must resolve first (see DeleteNode).
func (s *WriteServer) invalidateShards(nodeIDs ...string) []string {
	if s.shardManager == nil || s.store == nil {
		return nil
	}
	graph := s.store.GetGraph()
	if graph == nil {
		return nil
	}
	wsIDs := ngac.AffectedWorkspaces(graph, nodeIDs...)
	s.invalidateResolvedShards(wsIDs)
	return wsIDs
}

// invalidateResolvedShards drops the shards of already-resolved workspaces.
func (s *WriteServer) invalidateResolvedShards(wsIDs []string) {
	if s.shardManager == nil {
		return
	}
	for _, wsID := range wsIDs {
		s.shardManager.InvalidateShard(wsID)
	}
}

// --- New RPCs: Operations, Prohibitions, InvalidateCache ---

// RegisterOperations registers domain-specific operations (idempotent).
func (s *WriteServer) RegisterOperations(ctx context.Context, req *pb.RegisterOperationsRequest) (*pb.RegisterOperationsResponse, error) {
	if s.operations == nil {
		return nil, status.Error(codes.Unimplemented, "operations store not configured")
	}

	result, err := s.operations.Register(ctx, req.Operations)
	if err != nil {
		return nil, grpcauth.Internal(fmt.Errorf("register operations: %w", err))
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

	s.invalidateShards(req.NodeIds...)
	s.invalidation.InvalidateForNodes(ctx, req.NodeIds...)

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
		return nil, grpcauth.Internal(fmt.Errorf("create prohibition: %w", err))
	}

	affectedNodes := s.resolveProhibitionAffectedNodes(req.SubjectId, req.TargetOaIds)
	s.invalidateShards(affectedNodes...)
	s.invalidation.InvalidateForNodes(ctx, affectedNodes...)
	s.publishEvent(ngac.MutationCreateProhibition, affectedNodes)
	s.announceUsersOf(req.SubjectId, req.TargetOaIds...)

	return &pb.Prohibition{
		Id:           p.ID,
		Name:         p.Name,
		SubjectId:    p.SubjectID,
		Operations:   p.Operations,
		TargetOaIds:  p.TargetOAIDs,
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
		return nil, grpcauth.Internal(fmt.Errorf("remove prohibition: %w", err))
	}

	affectedNodes := s.resolveProhibitionAffectedNodes(p.SubjectID, p.TargetOAIDs)
	s.invalidateShards(affectedNodes...)
	s.invalidation.InvalidateForNodes(ctx, affectedNodes...)
	s.publishEvent(ngac.MutationRemoveProhibition, affectedNodes)
	s.announceUsersOf(p.SubjectID, p.TargetOAIDs...)

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
