package domain

import (
	"context"
	"fmt"
	"sort"

	"google.golang.org/protobuf/types/known/timestamppb"

	"ngac-platform/ngac"
	"ngac-platform/pkg/grpcauth"
	pb "ngac-platform/proto/asset"
)

// readableTypes returns the IDs of the workspace's asset types the caller may
// read, failing closed when the policy service cannot answer.
func (s *AssetService) readableTypes(ctx context.Context, workspaceID string) ([]string, error) {
	types, err := s.store.ListTypes(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list asset types: %w", err)
	}
	readable, err := permittedTypeIDs(ctx, s.policyRead, grpcauth.CallerFrom(ctx).NGACNodeID, types, ngac.OpRead)
	if err != nil {
		return nil, fmt.Errorf("batch access check: %w", err)
	}
	return readable, nil
}

// GetSummary counts the workspace's assets the caller may read.
func (s *AssetService) GetSummary(ctx context.Context, req *pb.GetSummaryRequest) (*pb.AssetSummary, error) {
	readable, err := s.readableTypes(ctx, req.WorkspaceId)
	if err != nil {
		return nil, err
	}
	sum, err := s.store.Summary(ctx, req.WorkspaceId, readable)
	if err != nil {
		return nil, fmt.Errorf("summary: %w", err)
	}
	out := &pb.AssetSummary{Total: sum.Total, ByState: sum.ByState, Holders: sum.Holders, MaintenanceOverdue: sum.MaintenanceOverdue}
	for _, tc := range sum.ByType {
		out.ByType = append(out.ByType, &pb.TypeCount{TypeId: tc.TypeID, TypeName: tc.TypeName, Count: tc.Count})
	}
	return out, nil
}

// ListActivity returns the newest lifecycle steps on assets the caller may read.
func (s *AssetService) ListActivity(ctx context.Context, req *pb.ListActivityRequest) (*pb.ActivityList, error) {
	readable, err := s.readableTypes(ctx, req.WorkspaceId)
	if err != nil {
		return nil, err
	}
	entries, err := s.store.ListActivity(ctx, req.WorkspaceId, readable, req.Limit)
	if err != nil {
		return nil, fmt.Errorf("activity: %w", err)
	}
	// Decisions on requests of the same readable types, merged by time.
	decisions, err := s.store.ListRequestDecisions(ctx, req.WorkspaceId, readable, req.Limit)
	if err != nil {
		return nil, fmt.Errorf("activity: %w", err)
	}
	entries = append(entries, decisions...)
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].CreatedAt.After(entries[j].CreatedAt) })
	limit := int(req.Limit)
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	if len(entries) > limit {
		entries = entries[:limit]
	}
	out := &pb.ActivityList{}
	for _, e := range entries {
		out.Entries = append(out.Entries, &pb.ActivityEntry{
			Id: e.ID, AssetId: e.AssetID, AssetName: e.AssetName, TypeName: e.TypeName,
			FromState: e.FromState, ToState: e.ToState, Action: e.Action,
			ActorId: e.ActorID, ActorName: e.ActorName,
			SubjectUserId: e.SubjectUserID, SubjectName: e.SubjectName,
			Comment: e.Comment, CreatedAt: timestamppb.New(e.CreatedAt), RequestStatus: e.RequestStatus,
		})
	}
	return out, nil
}
