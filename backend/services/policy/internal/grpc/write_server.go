package grpc

import (
	"context"
	"log/slog"

	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "ngac-platform/proto/policy"
	"ngac-platform/services/policy/internal/events"
	"ngac-platform/services/policy/internal/ngac"
)

// WriteServer implements PolicyWriteService as a singleton mutation handler.
// Every write operation: persists → increments version → invalidates targeted caches → publishes event.
type WriteServer struct {
	pb.UnimplementedPolicyWriteServiceServer
	store        *ngac.Store
	rdb          *redis.Client
	producer     *events.Producer
	materialized *ngac.MaterializedAccess
	version      *ngac.VersionTracker
}

// NewWriteServer creates a PolicyWriteService with targeted invalidation and event publishing.
func NewWriteServer(
	store *ngac.Store,
	rdb *redis.Client,
	producer *events.Producer,
	materialized *ngac.MaterializedAccess,
	version *ngac.VersionTracker,
) *WriteServer {
	return &WriteServer{
		store:        store,
		rdb:          rdb,
		producer:     producer,
		materialized: materialized,
		version:      version,
	}
}

func (s *WriteServer) CreateNode(ctx context.Context, req *pb.CreateNodeRequest) (*pb.NGACNode, error) {
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

// DeleteNode removes a node and invalidates affected caches.
func (s *WriteServer) DeleteNode(ctx context.Context, req *pb.DeleteNodeRequest) (*pb.Empty, error) {
	if err := s.store.DeleteNode(ctx, req.NodeId); err != nil {
		return nil, status.Errorf(codes.Internal, "delete node: %v", err)
	}

	s.incrementAndInvalidate(ctx, req.NodeId)
	s.publishEvent("delete_node", []string{req.NodeId})

	return &pb.Empty{}, nil
}

// CreateAssignment modifies graph structure — targeted cache invalidation.
func (s *WriteServer) CreateAssignment(ctx context.Context, req *pb.CreateAssignmentRequest) (*pb.Assignment, error) {
	a, err := s.store.CreateAssignment(ctx, req.ChildId, req.ParentId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "create assignment: %v", err)
	}

	s.incrementAndInvalidate(ctx, req.ChildId, req.ParentId)
	s.publishEvent("create_assignment", []string{req.ChildId, req.ParentId})

	return &pb.Assignment{Id: a.ID, ChildId: a.ChildID, ParentId: a.ParentID}, nil
}

// RemoveAssignment modifies graph structure — targeted cache invalidation.
func (s *WriteServer) RemoveAssignment(ctx context.Context, req *pb.RemoveAssignmentRequest) (*pb.Empty, error) {
	if err := s.store.RemoveAssignment(ctx, req.ChildId, req.ParentId); err != nil {
		return nil, status.Errorf(codes.Internal, "remove assignment: %v", err)
	}

	s.incrementAndInvalidate(ctx, req.ChildId, req.ParentId)
	s.publishEvent("remove_assignment", []string{req.ChildId, req.ParentId})

	return &pb.Empty{}, nil
}

// CreateAssociation modifies permissions — targeted cache invalidation.
func (s *WriteServer) CreateAssociation(ctx context.Context, req *pb.CreateAssociationRequest) (*pb.Association, error) {
	a, err := s.store.CreateAssociation(ctx, req.UaId, req.OaId, req.Operations)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "create association: %v", err)
	}

	s.incrementAndInvalidate(ctx, req.UaId, req.OaId)
	s.publishEvent("create_association", []string{req.UaId, req.OaId})

	return &pb.Association{Id: a.ID, UaId: a.UAID, OaId: a.OAID, Operations: a.Operations}, nil
}

// RemoveAssociation modifies permissions — targeted cache invalidation.
func (s *WriteServer) RemoveAssociation(ctx context.Context, req *pb.RemoveAssociationRequest) (*pb.Empty, error) {
	if err := s.store.RemoveAssociationByUAOA(ctx, req.UaId, req.OaId); err != nil {
		return nil, status.Errorf(codes.Internal, "remove association: %v", err)
	}

	s.incrementAndInvalidate(ctx, req.UaId, req.OaId)
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
	s.invalidateAllCaches(ctx)

	return &pb.Empty{}, nil
}

// --- Targeted invalidation helpers ---

// incrementAndInvalidate bumps the graph version and invalidates materialized entries
// only for the affected node IDs (targeted, not full flush).
func (s *WriteServer) incrementAndInvalidate(ctx context.Context, nodeIDs ...string) {
	if s.version != nil {
		if _, err := s.version.Increment(ctx, "global"); err != nil {
			slog.Warn("failed to increment graph version", "error", err)
		}
	}

	if s.materialized != nil {
		for _, nodeID := range nodeIDs {
			if err := s.materialized.InvalidateByUser(ctx, nodeID); err != nil {
				slog.Warn("failed to invalidate materialized user entries", "nodeID", nodeID, "error", err)
			}
			if err := s.materialized.InvalidateByObject(ctx, nodeID); err != nil {
				slog.Warn("failed to invalidate materialized object entries", "nodeID", nodeID, "error", err)
			}
		}
	}

	// Clear Redis L1 cache for affected keys
	s.invalidateRedisCache(ctx)
}

// invalidateAllCaches flushes all layers (used on full graph reload).
func (s *WriteServer) invalidateAllCaches(ctx context.Context) {
	if s.version != nil {
		if _, err := s.version.Increment(ctx, "global"); err != nil {
			slog.Warn("failed to increment graph version", "error", err)
		}
	}
	if s.materialized != nil {
		if err := s.materialized.InvalidateAll(ctx); err != nil {
			slog.Warn("failed to flush materialized cache", "error", err)
		}
	}
	s.invalidateRedisCache(ctx)
}

// invalidateRedisCache clears access decision entries from Redis.
func (s *WriteServer) invalidateRedisCache(ctx context.Context) {
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
			slog.Warn("failed to invalidate redis access cache", "count", len(keys), "error", err)
		} else {
			slog.Debug("redis access cache invalidated", "keys_deleted", len(keys))
		}
	}
}

func (s *WriteServer) publishEvent(action string, nodeIDs []string) {
	if s.producer != nil {
		s.producer.PublishGraphMutated(action, nodeIDs)
	}
}
