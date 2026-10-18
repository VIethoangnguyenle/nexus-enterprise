package rest

import (
	"context"
	"log/slog"

	pb "ngac-platform/proto/drive"
	"ngac-platform/services/drive/internal/domain"
)

// OwnerNameResolver turns the owner keys stored on drive items into the
// display names of the people behind them.
type OwnerNameResolver interface {
	DisplayNames(ctx context.Context, ownerKeys []string) (map[string]string, error)
}

// WithOwnerNames wraps a DriveService so every item it returns for reading
// carries `owner_name`, joined from the users table. The client shows people by
// name and avatar, never by id, and `owner_id` alone cannot give it that: it is
// a user id on files and an NGAC node id on folders.
//
// A failed lookup is logged and the response goes out without names rather
// than failing the read; the client falls back to a neutral label.
func WithOwnerNames(svc DriveService, names OwnerNameResolver) DriveService {
	return &ownerNamed{DriveService: svc, names: names}
}

type ownerNamed struct {
	DriveService
	names OwnerNameResolver
}

func (o *ownerNamed) ListFolder(ctx context.Context, req *pb.ListFolderRequest) (*pb.DriveItemList, error) {
	list, err := o.DriveService.ListFolder(ctx, req)
	if err == nil {
		o.fill(ctx, list.GetItems()...)
	}
	return list, err
}

func (o *ownerNamed) GetItem(ctx context.Context, req *pb.GetItemRequest) (*pb.DriveItem, error) {
	item, err := o.DriveService.GetItem(ctx, req)
	if err == nil && item != nil {
		o.fill(ctx, item)
	}
	return item, err
}

func (o *ownerNamed) GetSharedWithMe(ctx context.Context, req *pb.GetSharedWithMeRequest) (*pb.DriveItemList, error) {
	list, err := o.DriveService.GetSharedWithMe(ctx, req)
	if err == nil {
		o.fill(ctx, list.GetItems()...)
	}
	return list, err
}

func (o *ownerNamed) SearchItems(ctx context.Context, workspaceID, query string, limit int) (*pb.DriveItemList, error) {
	list, err := o.DriveService.SearchItems(ctx, workspaceID, query, limit)
	if err == nil {
		o.fill(ctx, list.GetItems()...)
	}
	return list, err
}

func (o *ownerNamed) ListStarredItems(ctx context.Context, workspaceID string) (*pb.DriveItemList, error) {
	list, err := o.DriveService.ListStarredItems(ctx, workspaceID)
	if err == nil {
		o.fill(ctx, list.GetItems()...)
	}
	return list, err
}

func (o *ownerNamed) RecentFiles(ctx context.Context, workspaceID string) (*pb.DriveItemList, error) {
	list, err := o.DriveService.RecentFiles(ctx, workspaceID)
	if err == nil {
		o.fill(ctx, list.GetItems()...)
	}
	return list, err
}

// QuickAccessItems names each file's owner and, on a shared one, the person who
// shared it. A sharer who cannot be named gets no name; the client then shows
// the reason without one.
func (o *ownerNamed) QuickAccessItems(ctx context.Context, workspaceID string) ([]*domain.QuickAccessEntry, error) {
	entries, err := o.DriveService.QuickAccessItems(ctx, workspaceID)
	if err != nil || len(entries) == 0 {
		return entries, err
	}
	items := make([]*pb.DriveItem, 0, len(entries))
	var sharers []string
	for _, e := range entries {
		items = append(items, e.Item)
		if e.SharedBy != "" {
			sharers = append(sharers, e.SharedBy)
		}
	}
	o.fill(ctx, items...)
	if len(sharers) > 0 {
		names, err := o.names.DisplayNames(ctx, sharers)
		if err != nil {
			slog.Warn("resolve drive sharer names", "error", err)
			return entries, nil
		}
		for _, e := range entries {
			if e.SharedBy != "" {
				e.SharedByName = names[e.SharedBy]
			}
		}
	}
	return entries, nil
}

// fill sets OwnerName on each item whose owner is a known person.
func (o *ownerNamed) fill(ctx context.Context, items ...*pb.DriveItem) {
	seen := make(map[string]struct{}, len(items))
	var keys []string
	for _, it := range items {
		if it == nil || it.OwnerId == "" {
			continue
		}
		if _, dup := seen[it.OwnerId]; dup {
			continue
		}
		seen[it.OwnerId] = struct{}{}
		keys = append(keys, it.OwnerId)
	}
	if len(keys) == 0 {
		return
	}
	names, err := o.names.DisplayNames(ctx, keys)
	if err != nil {
		slog.Warn("resolve drive owner names", "error", err)
		return
	}
	for _, it := range items {
		if it != nil {
			it.OwnerName = names[it.OwnerId]
		}
	}
}
