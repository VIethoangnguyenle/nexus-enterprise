package domain

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"ngac-platform/ngac"
	"ngac-platform/pkg/grpcauth"
	docpb "ngac-platform/proto/document"
	pb "ngac-platform/proto/drive"
	policypb "ngac-platform/proto/policy"
	"ngac-platform/services/drive/internal/store"
)

// Why a file is offered in Truy cập nhanh. Only reasons drawn from the drive's
// own records exist, and each is shown only on a file the caller can still read.
const (
	ReasonShared  = "shared"  // someone else shared it with the caller (or a group of theirs)
	ReasonStarred = "starred" // the caller starred it
	ReasonYours   = "yours"   // the caller put it there
)

// QuickAccessLimit is how many cards Truy cập nhanh shows.
const QuickAccessLimit = 6

// RecentLimit is how many files Gần đây lists.
const RecentLimit = 50

// candidateFactor: how many candidates are read per slot, so files the caller
// can no longer read do not leave the list short.
const candidateFactor = 4

// QuickAccessEntry is one card of Truy cập nhanh.
type QuickAccessEntry struct {
	Item   *pb.DriveItem `json:"item"`
	Reason string        `json:"reason"`
	// SharedBy is the NGAC node of whoever shared the file. It never leaves the
	// service: the REST layer turns it into SharedByName.
	SharedBy     string `json:"-"`
	SharedByName string `json:"shared_by_name,omitempty"`
	At           time.Time `json:"at"`
}

// StarItem stars an item for the caller. The item must be active, the caller
// a member of the item's own workspace, and able to read it now.
func (s *Service) StarItem(ctx context.Context, itemID string) error {
	caller := grpcauth.CallerFrom(ctx)
	if !caller.Authenticated() {
		return denied("access denied")
	}
	item, err := s.store.GetItem(ctx, itemID)
	if err != nil {
		return fmt.Errorf("load item: %w", err)
	}
	if item == nil || item.Status != "active" {
		return notFound("item not found")
	}
	if err := s.requireWorkspaceMember(ctx, item.WorkspaceID, caller.UserID); err != nil {
		return err
	}
	if err := s.checkAccess(ctx, caller.NGACNodeID, item.NGACNodeID, ngac.OpRead); err != nil {
		return err
	}
	if err := s.store.StarItem(ctx, caller.UserID, itemID); err != nil {
		return fmt.Errorf("star item: %w", err)
	}
	return nil
}

// UnstarItem removes the caller's own star. No read check: a person may always
// take back their own star, also on an item they can no longer read, and the
// row is keyed by the caller so no one else's star can be touched.
func (s *Service) UnstarItem(ctx context.Context, itemID string) error {
	caller := grpcauth.CallerFrom(ctx)
	if !caller.Authenticated() {
		return denied("access denied")
	}
	if err := s.store.UnstarItem(ctx, caller.UserID, itemID); err != nil {
		return fmt.Errorf("unstar item: %w", err)
	}
	return nil
}

// ListStarredItems returns the caller's starred items in workspaceID that the
// caller can still read. A star on an item read was taken from is kept, not shown.
func (s *Service) ListStarredItems(ctx context.Context, workspaceID string) (*pb.DriveItemList, error) {
	caller := grpcauth.CallerFrom(ctx)
	if err := s.requireWorkspaceMember(ctx, workspaceID, caller.UserID); err != nil {
		return nil, err
	}
	items, err := s.store.ListStarredItems(ctx, caller.UserID, workspaceID)
	if err != nil {
		return nil, err
	}
	readable, err := s.readableItems(ctx, items, len(items))
	if err != nil {
		return nil, err
	}
	return &pb.DriveItemList{Items: readable}, nil
}

// RecentFiles lists the files of workspaceID the caller can read, last edited first.
func (s *Service) RecentFiles(ctx context.Context, workspaceID string) (*pb.DriveItemList, error) {
	caller := grpcauth.CallerFrom(ctx)
	if err := s.requireWorkspaceMember(ctx, workspaceID, caller.UserID); err != nil {
		return nil, err
	}
	items, err := s.store.RecentFiles(ctx, workspaceID, RecentLimit*candidateFactor)
	if err != nil {
		return nil, err
	}
	readable, err := s.readableItems(ctx, items, RecentLimit)
	if err != nil {
		return nil, err
	}
	return &pb.DriveItemList{Items: readable}, nil
}

// QuickAccessItems returns up to QuickAccessLimit files of workspaceID for the
// caller, each with why it is offered. Every file is re-checked for read now;
// one the caller cannot read is left out with its reason.
func (s *Service) QuickAccessItems(ctx context.Context, workspaceID string) ([]*QuickAccessEntry, error) {
	caller := grpcauth.CallerFrom(ctx)
	if err := s.requireWorkspaceMember(ctx, workspaceID, caller.UserID); err != nil {
		return nil, err
	}
	targets, err := s.shareTargets(ctx, caller.NGACNodeID)
	if err != nil {
		return nil, err
	}
	cands, err := s.store.QuickAccessCandidates(ctx, caller.UserID, caller.NGACNodeID, workspaceID, targets, QuickAccessLimit*candidateFactor)
	if err != nil {
		return nil, err
	}
	items := make([]*store.DriveItem, 0, len(cands))
	for _, c := range cands {
		items = append(items, c.Item)
	}
	readable, err := s.readableItems(ctx, items, QuickAccessLimit)
	if err != nil {
		return nil, err
	}
	byID := make(map[string]*store.QuickAccessCandidate, len(cands))
	for _, c := range cands {
		byID[c.Item.ID] = c
	}
	out := make([]*QuickAccessEntry, 0, len(readable))
	for _, it := range readable {
		c := byID[it.Id]
		e := &QuickAccessEntry{Item: it, Reason: c.Reason, At: c.At}
		if c.Reason == ReasonShared {
			e.SharedBy = c.SharedBy
		}
		out = append(out, e)
	}
	return out, nil
}

// shareTargets is every node a share to the caller can name: their own node
// and each group (UA) they belong to. A failed lookup fails the call; a partial
// answer would silently drop what was shared with their groups.
func (s *Service) shareTargets(ctx context.Context, userNode string) ([]string, error) {
	anc, err := s.policyRead.GetAncestors(ctx, &policypb.GetAncestorsRequest{NodeId: userNode})
	if err != nil {
		return nil, fmt.Errorf("look up the caller's groups: %w", err)
	}
	targets := []string{userNode}
	for _, n := range anc.GetNodes() {
		if n.NodeType == ngac.TypeUA {
			targets = append(targets, n.Id)
		}
	}
	return targets, nil
}

// readableItems keeps, in order, the items the caller may read now (one batch
// check on their OAs), up to limit. A failed check fails closed: no partial list.
func (s *Service) readableItems(ctx context.Context, items []*store.DriveItem, limit int) ([]*pb.DriveItem, error) {
	if len(items) == 0 {
		return []*pb.DriveItem{}, nil
	}
	seen := make(map[string]bool, len(items))
	nodes := make([]string, 0, len(items))
	for _, it := range items {
		if !seen[it.NGACNodeID] {
			seen[it.NGACNodeID] = true
			nodes = append(nodes, it.NGACNodeID)
		}
	}
	batch, err := s.policy.BatchCheckCaller(ctx, nodes, []string{ngac.OpRead})
	if err != nil {
		return nil, fmt.Errorf("batch access check: %w", err)
	}
	out := make([]*pb.DriveItem, 0, min(limit, len(items)))
	for _, it := range items {
		if len(out) == limit {
			break
		}
		if batch.Has(it.NGACNodeID, ngac.OpRead) {
			out = append(out, itemToProto(it))
		}
	}
	return out, nil
}

// cleanupBatch bounds one cleanup pass; the next pass picks up the rest.
const cleanupBatch = 500

// CleanupPendingUploads removes uploads still pending more than olderThan after
// they were started (the client never confirmed them), with their stored
// objects. A row confirmed meanwhile is left alone. The object is deleted on
// behalf of the person who started the upload, the only one the document
// service would let remove it; one whose uploader has no identity is logged.
// Returns how many rows were removed.
func (s *Service) CleanupPendingUploads(ctx context.Context, olderThan time.Duration) (int, error) {
	stale, err := s.store.StalePendingUploads(ctx, olderThan, cleanupBatch)
	if err != nil {
		return 0, err
	}
	removed := 0
	for _, u := range stale {
		ok, err := s.store.DeletePendingItem(ctx, u.Item.ID)
		if err != nil {
			return removed, fmt.Errorf("remove stale upload %s: %w", u.Item.ID, err)
		}
		if !ok {
			continue
		}
		removed++
		if u.Item.ObjectKey == nil || *u.Item.ObjectKey == "" {
			continue
		}
		if u.OwnerNode == "" {
			slog.Error("stale upload's stored object not removed: uploader has no identity", "item", u.Item.ID, "key", *u.Item.ObjectKey)
			continue
		}
		as := grpcauth.WithCaller(ctx, grpcauth.Caller{UserID: u.Item.OwnerID, NGACNodeID: u.OwnerNode})
		if _, err := s.docStorage.DeleteObject(as, &docpb.DeleteObjectRequest{
			WorkspaceId: u.Item.WorkspaceID, ObjectKey: *u.Item.ObjectKey,
		}); err != nil {
			slog.Error("stale upload's stored object not removed", "item", u.Item.ID, "key", *u.Item.ObjectKey, "err", err)
		}
	}
	if removed > 0 {
		slog.Info("removed stale pending uploads", "count", removed)
	}
	return removed, nil
}
