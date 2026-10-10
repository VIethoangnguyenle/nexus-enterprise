package domain

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"ngac-platform/ngac"
	"ngac-platform/pkg/grpcauth"
	"ngac-platform/pkg/realtime"
	docpb "ngac-platform/proto/document"
	pb "ngac-platform/proto/drive"
	policypb "ngac-platform/proto/policy"
	"ngac-platform/services/drive/internal/store"
)

// RenameItem renames a file or folder.
func (s *Service) RenameItem(ctx context.Context, req *pb.RenameItemRequest) (*pb.DriveItem, error) {
	item, err := s.store.GetItem(ctx, req.ItemId)
	if err != nil || item == nil {
		return nil, notFound("item not found")
	}
	if err := s.checkAccess(ctx, grpcauth.CallerFrom(ctx).NGACNodeID, item.NGACNodeID, ngac.OpWrite); err != nil {
		return nil, err
	}
	if err := s.store.UpdateName(ctx, item.ID, req.NewName); err != nil {
		return nil, fmt.Errorf("rename: %w", err)
	}
	item.Name = req.NewName
	s.announce(ctx, realtime.KindUpdated, item)
	return itemToProto(item), nil
}

// MoveItem moves an item within the same drive context. An empty NewParentId
// moves it to the top level of that drive.
//
// Moves of one item are serialised (store.LockItem), and the row is updated
// only if the item is still under the parent this move started from, so two
// concurrent moves cannot leave the item under both destinations in the graph.
func (s *Service) MoveItem(ctx context.Context, req *pb.MoveItemRequest) (*pb.DriveItem, error) {
	callerNode := grpcauth.CallerFrom(ctx).NGACNodeID
	item, err := s.store.GetItem(ctx, req.ItemId)
	if err != nil || item == nil || item.Status == "trashed" {
		return nil, notFound("item not found")
	}
	if err := s.checkAccess(ctx, callerNode, item.NGACNodeID, ngac.OpWrite); err != nil {
		return nil, err
	}

	unlock, err := s.store.LockItem(ctx, item.ID)
	if err != nil {
		return nil, unavailable("wait for concurrent move: %v", err)
	}
	defer unlock()

	// Read again under the lock: the item may have moved, or been trashed,
	// while this call waited, and what follows must start from where it is now.
	prevNode := item.NGACNodeID
	item, err = s.store.GetItem(ctx, req.ItemId)
	if err != nil || item == nil || item.Status == "trashed" {
		return nil, notFound("item not found")
	}
	if item.NGACNodeID != prevNode {
		if err := s.checkAccess(ctx, callerNode, item.NGACNodeID, ngac.OpWrite); err != nil {
			return nil, err
		}
	}

	// Resolve the destination. Everything that can be refused is refused here,
	// before any policy write.
	var dest *store.DriveItem
	var newParentID *string
	if req.NewParentId == "" {
		root, err := s.ensureRoot(ctx, item.WorkspaceID, item.DriveContext, item.DriveContextID, callerNode)
		if err != nil {
			return nil, err
		}
		if root.ID == item.ID {
			return nil, invalid("the drive root cannot be moved")
		}
		dest = root // top-level items have no parent row; the root is only their OA
	} else {
		dest, err = s.store.GetItem(ctx, req.NewParentId)
		if err != nil {
			return nil, fmt.Errorf("load destination: %w", err)
		}
		if dest == nil || dest.Status != "active" {
			return nil, notFound("destination not found")
		}
		if dest.ItemType != "folder" {
			return nil, invalid("destination is not a folder")
		}
		if dest.WorkspaceID != item.WorkspaceID || dest.DriveContext != item.DriveContext ||
			(item.DriveContext != "workspace" && dest.DriveContextID != item.DriveContextID) {
			return nil, invalid("an item can only be moved within its own drive")
		}
		if item.ItemType == "folder" {
			inside, err := s.store.IsAncestorOrSelf(ctx, item.ID, dest.ID)
			if err != nil {
				return nil, fmt.Errorf("check destination: %w", err)
			}
			if inside {
				return nil, invalid("a folder cannot be moved into itself or one of its subfolders")
			}
		}
		id := dest.ID
		newParentID = &id
	}
	if err := s.checkAccess(ctx, callerNode, dest.NGACNodeID, ngac.OpWrite); err != nil {
		return nil, err
	}

	newNodeID := item.NGACNodeID // a folder keeps its own OA
	var oldParentOA string
	if item.ItemType == "folder" {
		// Folders have their own OA; moving one means re-pointing its parent
		// edge. Files have none and inherit from the folder they sit in.
		oldParentOA, err = s.currentParentOA(ctx, item)
		if err != nil {
			return nil, err
		}
		if err := s.reparent(ctx, item.NGACNodeID, oldParentOA, dest.NGACNodeID); err != nil {
			return nil, fmt.Errorf("move folder: %w", err)
		}
	} else {
		newNodeID = dest.NGACNodeID
	}

	moved, err := s.store.UpdateParentAndNode(ctx, item.ID, item.ParentID, newParentID, newNodeID)
	if err != nil || !moved {
		// The row did not move, so neither may the policy edge.
		if item.ItemType == "folder" {
			if rerr := s.reparent(ctx, item.NGACNodeID, dest.NGACNodeID, oldParentOA); rerr != nil {
				slog.Error("move rolled back incompletely; folder edge needs repair",
					"item", item.ID, "error", rerr)
				return nil, fmt.Errorf("move failed and could not be undone: %w", rerr)
			}
		}
		if err != nil {
			return nil, fmt.Errorf("move: %w", err)
		}
		return nil, aborted("item was changed by another request; retry")
	}
	oldParent := ""
	if item.ParentID != nil {
		oldParent = *item.ParentID
	}
	item.NGACNodeID = newNodeID
	item.ParentID = newParentID
	s.announce(ctx, realtime.KindMoved, item, func(e *realtime.Event) { e.OldParentID = oldParent })
	return itemToProto(item), nil
}

// currentParentOA returns the OA a folder is assigned under today: its parent
// folder's OA, or the drive root's for a top-level folder. "" means unknown
// (no parent row or root found), in which case there is no edge to remove.
func (s *Service) currentParentOA(ctx context.Context, item *store.DriveItem) (string, error) {
	if item.ParentID != nil {
		old, err := s.store.GetItem(ctx, *item.ParentID)
		if err != nil {
			return "", fmt.Errorf("load current parent: %w", err)
		}
		if old == nil {
			return "", nil
		}
		return old.NGACNodeID, nil
	}
	rootCtxID := item.DriveContextID
	if rootCtxID == "" {
		rootCtxID = item.WorkspaceID
	}
	root, err := s.store.FindRootByContext(ctx, item.WorkspaceID, item.DriveContext, rootCtxID)
	if err != nil {
		return "", fmt.Errorf("find drive root: %w", err)
	}
	if root == nil {
		return "", nil
	}
	if root.ID == item.ID {
		return "", invalid("the drive root cannot be moved")
	}
	return root.NGACNodeID, nil
}

// reparent moves a folder OA from one parent OA to another. The new edge is
// made first and the old one removed after, so the folder never reaches fewer
// policy classes than it should: a shared folder detached from its workspace
// reaches only PC_Global, which would let users of other tenants through the
// share. If the new edge is refused (a cycle, say) nothing has changed; if the
// old edge cannot be removed the new one is withdrawn again. A failure that
// leaves the folder under both parents is returned, not just logged.
func (s *Service) reparent(ctx context.Context, childOA, fromOA, toOA string) error {
	if fromOA == toOA {
		return nil
	}
	if _, err := s.policyWrite.CreateAssignment(ctx, &policypb.CreateAssignmentRequest{
		ChildId: childOA, ParentId: toOA,
	}); err != nil {
		return fmt.Errorf("attach folder to new parent: %w", err)
	}
	if fromOA == "" {
		return nil
	}
	// If the old edge survives, the folder hangs under both parents and keeps
	// inheriting the permissions of the one it left.
	if _, err := s.policyWrite.RemoveAssignment(ctx, &policypb.RemoveAssignmentRequest{
		ChildId: childOA, ParentId: fromOA,
	}); err != nil {
		if _, rerr := s.policyWrite.RemoveAssignment(ctx, &policypb.RemoveAssignmentRequest{
			ChildId: childOA, ParentId: toOA,
		}); rerr != nil {
			return fmt.Errorf("detach folder from old parent: %w; withdrawing the new edge also failed (%v), so the folder is under both parents", err, rerr)
		}
		return fmt.Errorf("detach folder from old parent: %w", err)
	}
	return nil
}

// CopyItem copies a file to a destination (possibly cross-context).
func (s *Service) CopyItem(ctx context.Context, req *pb.CopyItemRequest) (*pb.DriveItem, error) {
	src, err := s.store.GetItem(ctx, req.ItemId)
	if err != nil || src == nil {
		return nil, notFound("source not found")
	}
	if src.ItemType != "file" {
		return nil, invalid("only files can be copied")
	}
	if err := s.checkAccess(ctx, grpcauth.CallerFrom(ctx).NGACNodeID, src.NGACNodeID, ngac.OpRead); err != nil {
		return nil, err
	}

	// Determine destination parent NGAC
	dest, err := s.store.GetItem(ctx, req.DestParentId)
	if err != nil || dest == nil {
		return nil, notFound("destination not found")
	}

	// Files inherit the destination folder's OA node — no NGAC node created.
	copyNGACNodeID := dest.NGACNodeID

	// MinIO server-side copy
	newID := uuid.New().String()
	newKey := fmt.Sprintf("drive/%s/%s", newID, src.Name)
	_, err = s.docStorage.CopyObject(ctx, &docpb.CopyObjectRequest{
		SrcWorkspaceId: src.WorkspaceID, SrcObjectKey: *src.ObjectKey,
		DstWorkspaceId: req.DestWorkspaceId, DstObjectKey: newKey,
	})
	if err != nil {
		return nil, fmt.Errorf("copy object: %w", err)
	}

	driveCtx := req.DestDriveContext
	if driveCtx == "" {
		driveCtx = "workspace"
	}
	destParent := req.DestParentId
	newItem := &store.DriveItem{
		ID: newID, WorkspaceID: req.DestWorkspaceId,
		DriveContext: driveCtx, DriveContextID: req.DestDriveContextId,
		ParentID: &destParent, ItemType: "file", Name: src.Name,
		MimeType: src.MimeType, SizeBytes: src.SizeBytes, ObjectKey: &newKey,
		NGACNodeID: copyNGACNodeID, ScopeOAID: dest.ScopeOAID,
		OwnerID: grpcauth.CallerFrom(ctx).UserID, Status: "active",
	}
	if err := s.store.InsertItem(ctx, newItem); err != nil {
		return nil, fmt.Errorf("insert copy: %w", err)
	}
	s.announce(ctx, realtime.KindCreated, newItem)
	return itemToProto(newItem), nil
}

// TrashItem soft-deletes an item (recursive for folders).
func (s *Service) TrashItem(ctx context.Context, req *pb.TrashItemRequest) (*pb.Empty, error) {
	item, err := s.store.GetItem(ctx, req.ItemId)
	if err != nil || item == nil {
		return nil, notFound("item not found")
	}
	if err := s.checkAccess(ctx, grpcauth.CallerFrom(ctx).NGACNodeID, item.NGACNodeID, ngac.OpWrite); err != nil {
		return nil, err
	}
	if err := s.store.SetTrashed(ctx, item.ID, item.ItemType == "folder", true); err != nil {
		return nil, fmt.Errorf("trash: %w", err)
	}
	s.announce(ctx, realtime.KindDeleted, item)
	return &pb.Empty{}, nil
}

// RestoreItem restores a trashed item.
func (s *Service) RestoreItem(ctx context.Context, req *pb.RestoreItemRequest) (*pb.DriveItem, error) {
	item, err := s.store.GetItem(ctx, req.ItemId)
	if err != nil || item == nil {
		return nil, notFound("item not found")
	}
	if err := s.checkAccess(ctx, grpcauth.CallerFrom(ctx).NGACNodeID, item.NGACNodeID, ngac.OpWrite); err != nil {
		return nil, err
	}
	if err := s.store.SetTrashed(ctx, item.ID, item.ItemType == "folder", false); err != nil {
		return nil, fmt.Errorf("restore: %w", err)
	}
	item.Status = "active"
	s.announce(ctx, realtime.KindUpdated, item)
	return itemToProto(item), nil
}

// DeleteItem permanently removes an item and its storage.
func (s *Service) DeleteItem(ctx context.Context, req *pb.DeleteItemRequest) (*pb.Empty, error) {
	item, err := s.store.GetItem(ctx, req.ItemId)
	if err != nil || item == nil {
		return nil, notFound("item not found")
	}
	// Same right as TrashItem: permanent deletion must not be reachable by a
	// user who cannot perform the reversible version of the same action.
	if err := s.checkAccess(ctx, grpcauth.CallerFrom(ctx).NGACNodeID, item.NGACNodeID, ngac.OpWrite); err != nil {
		return nil, err
	}

	if item.ItemType == "folder" {
		// Refuse before anything is touched: text documents block the delete
		// (their folder reference is RESTRICT), and a half-finished delete would
		// leave the folder row with its access node gone.
		n, err := s.store.CountTextDocumentsUnder(ctx, item.ID)
		if err != nil {
			return nil, fmt.Errorf("check folder contents: %w", err)
		}
		if n > 0 {
			return nil, ErrFolderHasDocuments
		}
	}

	// The row goes first, together with the quota its files held, in one
	// transaction that reads the files itself. Stored objects and the folder's
	// access node are only released once the database has agreed, so a refusal (a
	// document saved since the check above) leaves the folder exactly as it was.
	files, err := s.store.DeleteItemReleasingQuota(ctx, item.ID)
	if err != nil {
		if errors.Is(err, store.ErrFolderHasDocuments) {
			return nil, ErrFolderHasDocuments
		}
		return nil, fmt.Errorf("delete item: %w", err)
	}

	for _, f := range files {
		// Files inherit their folder's OA: no NGAC node of their own to delete.
		if f.ObjectKey != nil {
			if _, err := s.docStorage.DeleteObject(ctx, &docpb.DeleteObjectRequest{
				WorkspaceId: f.WorkspaceID, ObjectKey: *f.ObjectKey,
			}); err != nil {
				slog.Error("deleted file's stored object not removed", "item", f.ID, "err", err)
			}
		}
	}

	if item.ItemType == "folder" {
		// The folder is already gone, so a failure here cannot be retried by the
		// caller; it leaves an unreachable OA with no row, which is logged for repair.
		if _, err := s.policyWrite.DeleteNode(ctx, &policypb.DeleteNodeRequest{NodeId: item.NGACNodeID}); err != nil {
			slog.Error("deleted folder's OA not removed; node needs cleanup", "item", item.ID, "node", item.NGACNodeID, "err", err)
		}
	}
	s.announce(ctx, realtime.KindDeleted, item)
	return &pb.Empty{}, nil
}
