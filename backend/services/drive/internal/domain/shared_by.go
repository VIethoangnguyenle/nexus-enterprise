package domain

import (
	"context"
	"fmt"
	"strings"

	"ngac-platform/ngac"
	"ngac-platform/pkg/grpcauth"
	pb "ngac-platform/proto/drive"
)

// SharedWithMeBy lists the items of one workspace that a person owns and that
// are shared with the caller: what a profile in Danh bạ calls "Tệp đã chia sẻ
// với bạn".
//
// The caller must be an active member of the workspace before anything is
// looked up. The items are the caller's shared-with-me list narrowed to that
// workspace and that owner (an owner is kept as the user id on files and as the
// NGAC node on folders; both are the person's), and then read is checked again
// in one batch: a share that no longer grants read lists nothing. A policy
// failure fails the request rather than shortening the list.
func (s *Service) SharedWithMeBy(ctx context.Context, workspaceID, ownerUserID string) (*pb.DriveItemList, error) {
	workspaceID = strings.TrimSpace(workspaceID)
	ownerUserID = strings.TrimSpace(ownerUserID)
	if workspaceID == "" {
		return nil, invalid("workspace_id is required")
	}
	if ownerUserID == "" {
		return nil, invalid("owner is required")
	}
	caller := grpcauth.CallerFrom(ctx)
	if caller.UserID != "" && ownerUserID == caller.UserID {
		return nil, invalid("files shared with you by yourself are not a question")
	}
	if err := s.requireWorkspaceMember(ctx, workspaceID, caller.UserID); err != nil {
		return nil, err
	}

	ownerNode, err := s.store.UserNodeOf(ctx, ownerUserID)
	if err != nil {
		return nil, fmt.Errorf("look up the owner: %w", err)
	}
	ownedBy := func(key string) bool { return key == ownerUserID || (ownerNode != "" && key == ownerNode) }

	shared, err := s.sharedWithCaller(ctx)
	if err != nil {
		return nil, err
	}
	candidates := shared[:0:0]
	objectIDs := make([]string, 0, len(shared))
	for _, item := range shared {
		if item.WorkspaceID != workspaceID || item.Status != "active" || item.NGACNodeID == "" || !ownedBy(item.OwnerID) {
			continue
		}
		candidates = append(candidates, item)
		objectIDs = append(objectIDs, item.NGACNodeID)
	}
	if len(candidates) == 0 {
		return &pb.DriveItemList{Items: []*pb.DriveItem{}}, nil
	}

	batch, err := s.policy.BatchCheckCaller(ctx, objectIDs, []string{ngac.OpRead})
	if err != nil {
		return nil, fmt.Errorf("batch access check: %w", err)
	}
	items := make([]*pb.DriveItem, 0, len(candidates))
	for _, item := range candidates {
		if batch.Has(item.NGACNodeID, ngac.OpRead) {
			items = append(items, itemToProto(item))
		}
	}
	return &pb.DriveItemList{Items: items}, nil
}
