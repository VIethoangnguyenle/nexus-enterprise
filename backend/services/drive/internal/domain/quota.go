package domain

import (
	"context"
	"fmt"

	"ngac-platform/ngac"
	"ngac-platform/pkg/grpcauth"
	pb "ngac-platform/proto/drive"
)

// GetQuota returns workspace storage quota.
func (s *Service) GetQuota(ctx context.Context, req *pb.GetQuotaRequest) (*pb.Quota, error) {
	// Storage consumption describes the workspace, so reading it requires
	// reaching that workspace's drive rather than merely holding a valid token.
	root, err := s.ensureRoot(ctx, req.WorkspaceId, "workspace", "", grpcauth.CallerFrom(ctx).NGACNodeID)
	if err != nil {
		return nil, err
	}
	if err := s.checkAccess(ctx, grpcauth.CallerFrom(ctx).NGACNodeID, root.NGACNodeID, ngac.OpRead); err != nil {
		return nil, err
	}

	q, err := s.store.GetOrCreateQuota(ctx, req.WorkspaceId)
	if err != nil {
		return nil, fmt.Errorf("get quota: %w", err)
	}
	return &pb.Quota{
		WorkspaceId: q.WorkspaceID, MaxBytes: q.MaxBytes, UsedBytes: q.UsedBytes,
		MaxFiles: q.MaxFiles, UsedFiles: q.UsedFiles,
	}, nil
}

// UpdateQuota sets workspace quota limits.
//
// Quota limits are workspace administration, so this takes manage on the
// workspace's Mgmt OA, for the caller on the context (see package grpcauth).
func (s *Service) UpdateQuota(ctx context.Context, req *pb.UpdateQuotaRequest) (*pb.Quota, error) {
	userNodeID := grpcauth.CallerFrom(ctx).NGACNodeID
	if err := s.checkAccessOnNamedOA(ctx, userNodeID, ngac.MgmtOAName(ngac.WorkspaceID(req.WorkspaceId)), ngac.OpManage); err != nil {
		return nil, err
	}
	if err := s.store.UpdateQuotaLimits(ctx, req.WorkspaceId, req.MaxBytes, req.MaxFiles); err != nil {
		return nil, fmt.Errorf("update quota: %w", err)
	}
	return s.GetQuota(ctx, &pb.GetQuotaRequest{WorkspaceId: req.WorkspaceId})
}
