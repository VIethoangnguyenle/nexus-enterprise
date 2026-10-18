package domain

import (
	"context"
	"fmt"
	"strings"

	"ngac-platform/ngac"
	"ngac-platform/pkg/grpcauth"
	pb "ngac-platform/proto/drive"
)

// searchDefaultLimit is how many results a search returns when the caller does
// not say; searchMaxLimit is the most it ever returns.
const (
	searchDefaultLimit = 20
	searchMaxLimit     = 50
)

// SearchItems finds the active drive items of one workspace whose name matches
// query, returning only those the caller may read.
//
// The caller must be an active member of the named workspace: a member of A
// naming B is refused outright, so nothing of B is ever looked up. Every
// candidate is then checked for read in one batch call, and one the caller
// cannot read is neither returned nor counted. A policy failure fails closed.
func (s *Service) SearchItems(ctx context.Context, workspaceID, query string, limit int) (*pb.DriveItemList, error) {
	query = strings.TrimSpace(query)
	if workspaceID == "" {
		return nil, invalid("workspace_id is required")
	}
	if query == "" {
		return nil, invalid("query is required")
	}
	if limit <= 0 {
		limit = searchDefaultLimit
	}
	if limit > searchMaxLimit {
		limit = searchMaxLimit
	}

	caller := grpcauth.CallerFrom(ctx)
	if err := s.requireWorkspaceMember(ctx, workspaceID, caller.UserID); err != nil {
		return nil, err
	}

	// Ask for more candidates than will be shown: some are dropped by the
	// read check below, and a short page would hide readable matches.
	items, err := s.store.SearchItems(ctx, workspaceID, query, limit*4)
	if err != nil {
		return nil, fmt.Errorf("search items: %w", err)
	}
	if len(items) == 0 {
		return &pb.DriveItemList{}, nil
	}

	objectIDs := make([]string, 0, len(items))
	for _, item := range items {
		objectIDs = append(objectIDs, item.NGACNodeID)
	}
	batch, err := s.policy.BatchCheckCaller(ctx, objectIDs, []string{ngac.OpRead})
	if err != nil {
		// Fail closed: an unreadable policy answer must not list anything.
		return nil, fmt.Errorf("batch access check: %w", err)
	}

	visible := make([]*pb.DriveItem, 0, limit)
	for _, item := range items {
		if item.WorkspaceID != workspaceID || !batch.Has(item.NGACNodeID, ngac.OpRead) {
			continue
		}
		visible = append(visible, itemToProto(item))
		if len(visible) == limit {
			break
		}
	}
	return &pb.DriveItemList{Items: visible}, nil
}
