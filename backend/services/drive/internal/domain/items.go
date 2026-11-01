package domain

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

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
	if err := s.refuseRoot(ctx, item, rootRename); err != nil {
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
	if err := s.refuseRoot(ctx, item, rootMove); err != nil {
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
		root, err := s.driveTop(ctx, item.WorkspaceID, item.DriveContext, item.DriveContextID, ngac.OpWrite, true)
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

	// Folders, and files that were shared, have an OA of their own; moving one
	// means re-pointing its parent edge, and it keeps its OA. Other files have
	// none and take the OA of the folder they land in.
	ownsOA := item.ItemType == "folder"
	if !ownsOA {
		if ownsOA, err = s.fileOwnsOA(ctx, item); err != nil {
			return nil, err
		}
	}
	newNodeID := item.NGACNodeID
	var oldParentOA string
	if ownsOA {
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
		if ownsOA {
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

// currentParentOA returns the OA an item with an OA of its own is assigned under
// today: its parent folder's OA, or the drive root's for a top-level item. "" means unknown
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

// copyPrefix starts the name a copy takes when its own name is already used in
// the destination, as Google Drive's "Copy of".
const copyPrefix = "Bản sao của "

// maxCopyNames bounds the "(n)" suffixes tried before a copy is refused.
const maxCopyNames = 100

// cleanupTimeout bounds the work that undoes a failed copy. It runs on a context
// the caller cannot cancel, so a client that hangs up mid-copy still leaves
// nothing behind.
const cleanupTimeout = 10 * time.Second

// CopyItem copies a file into a folder of the same workspace, or to the top
// level of a drive of that workspace; the destination may be in another of the
// workspace's drives (a channel drive, say). No destination means the top level
// of the source's own drive, and a drive root as destination means the top level
// of that drive, the way uploads and moves place top-level items.
//
// A copy writes a new file into the destination, so it takes what an upload
// there takes: read on the source, write on the destination, and room in the
// quota, which it is charged against like a confirmed upload. Where the copy
// lands (workspace, drive) comes from the destination and never from the
// request: a caller naming another workspace would otherwise place a file there,
// or, as REST sends none, leave it with no workspace at all. A name already used
// in the destination becomes "Bản sao của <name>", then "… (2)" and on.
func (s *Service) CopyItem(ctx context.Context, req *pb.CopyItemRequest) (*pb.DriveItem, error) {
	callerNode := grpcauth.CallerFrom(ctx).NGACNodeID
	src, err := s.store.GetItem(ctx, req.ItemId)
	if err != nil || src == nil || src.Status != "active" {
		return nil, notFound("source not found")
	}
	if src.ItemType != "file" {
		return nil, invalid("only files can be copied")
	}
	if src.ObjectKey == nil {
		return nil, notFound("source not found")
	}
	if err := s.checkAccess(ctx, callerNode, src.NGACNodeID, ngac.OpRead); err != nil {
		return nil, err
	}

	// Everything that can be refused is refused before anything is stored.
	place, err := s.copyDestination(ctx, src, req.DestParentId, callerNode)
	if err != nil {
		return nil, err
	}
	if err := s.checkAccess(ctx, callerNode, place.nodeID, ngac.OpWrite); err != nil {
		return nil, err
	}
	var size int64
	if src.SizeBytes != nil {
		size = *src.SizeBytes
	}
	// As for an upload, the quota is checked only once the caller may write.
	ok, err := s.store.CheckQuota(ctx, place.workspaceID, size)
	if err != nil {
		return nil, fmt.Errorf("check quota: %w", err)
	}
	if !ok {
		return nil, quotaExceeded("storage quota exceeded")
	}
	name, err := s.freeCopyName(ctx, place, src.Name)
	if err != nil {
		return nil, err
	}

	newID := uuid.New().String()
	newKey := fmt.Sprintf("drive/%s/%s", newID, name)
	copied, err := s.docStorage.CopyObject(ctx, &docpb.CopyObjectRequest{
		SrcWorkspaceId: src.WorkspaceID, SrcObjectKey: *src.ObjectKey,
		DstWorkspaceId: place.workspaceID, DstObjectKey: newKey,
	})
	if err != nil {
		return nil, fmt.Errorf("copy object: %w", err)
	}
	// The size the store reports for the new object is the one charged.
	if copied.GetSizeBytes() > 0 {
		size = copied.GetSizeBytes()
	}

	newItem := &store.DriveItem{
		ID: newID, WorkspaceID: place.workspaceID,
		DriveContext: place.driveContext, DriveContextID: place.driveContextID,
		ParentID: place.parentID, ItemType: "file", Name: name,
		MimeType: src.MimeType, SizeBytes: &size, ObjectKey: &newKey,
		// Files inherit the destination's OA node — no NGAC node created.
		NGACNodeID: place.nodeID, ScopeOAID: place.scopeOAID,
		OwnerID: grpcauth.CallerFrom(ctx).UserID, Status: "pending",
	}
	if err := s.store.InsertItem(ctx, newItem); err != nil {
		s.abandonCopy(ctx, "", place.workspaceID, newKey)
		return nil, fmt.Errorf("insert copy: %w", err)
	}
	// Publish and charge in one step, the way ConfirmFile does: a file is never
	// active without being counted.
	if err := s.store.ActivateFile(ctx, newID, place.workspaceID, size); err != nil {
		s.abandonCopy(ctx, newID, place.workspaceID, newKey)
		if errors.Is(err, store.ErrNameTaken) {
			// Another item took the name between the check and now.
			return nil, conflict("an item with that name already exists")
		}
		if errors.Is(err, store.ErrQuotaExceeded) {
			return nil, quotaExceeded("storage quota exceeded")
		}
		return nil, fmt.Errorf("publish copy: %w", err)
	}
	newItem.Status = "active"
	s.announce(ctx, realtime.KindCreated, newItem)
	return itemToProto(newItem), nil
}

// copyPlace is where a copy lands: under a folder, or at a drive's top level
// (parentID nil), with the OA it inherits.
type copyPlace struct {
	workspaceID, driveContext, driveContextID string
	parentID                                  *string
	nodeID, scopeOAID                         string
}

// copyDestination resolves the destination of a copy. It refuses a destination
// that is gone, not a folder, or in another workspace than the source.
func (s *Service) copyDestination(ctx context.Context, src *store.DriveItem, destID, callerNode string) (*copyPlace, error) {
	if destID == "" {
		// The top of the source's drive; write on it is checked before a root
		// is made, and again by the caller on the place returned.
		root, err := s.driveTop(ctx, src.WorkspaceID, src.DriveContext, src.DriveContextID, ngac.OpWrite, true)
		if err != nil {
			return nil, err
		}
		return &copyPlace{
			workspaceID: src.WorkspaceID, driveContext: src.DriveContext, driveContextID: src.DriveContextID,
			nodeID: root.NGACNodeID, scopeOAID: root.ScopeOAID,
		}, nil
	}
	dest, err := s.store.GetItem(ctx, destID)
	if err != nil {
		return nil, fmt.Errorf("load destination: %w", err)
	}
	if dest == nil || dest.Status != "active" {
		return nil, notFound("destination not found")
	}
	if dest.ItemType != "folder" {
		return nil, invalid("destination is not a folder")
	}
	if dest.WorkspaceID != src.WorkspaceID {
		return nil, invalid("a file can only be copied within its own workspace")
	}
	place := &copyPlace{
		workspaceID: dest.WorkspaceID, driveContext: dest.DriveContext, driveContextID: dest.DriveContextID,
		nodeID: dest.NGACNodeID, scopeOAID: dest.ScopeOAID,
	}
	if dest.ParentID == nil {
		root, err := s.store.FindRootByContext(ctx, dest.WorkspaceID, dest.DriveContext, dest.DriveContextID)
		if err != nil {
			return nil, fmt.Errorf("find drive root: %w", err)
		}
		if root != nil && root.ID == dest.ID {
			// The root is the drive's top level, not a folder to sit under. It
			// stores the workspace ID as its context; a workspace drive's items
			// store none.
			if dest.DriveContext == "workspace" {
				place.driveContextID = ""
			}
			return place, nil
		}
	}
	parent := dest.ID
	place.parentID = &parent
	return place, nil
}

// freeCopyName returns name if no active item in place uses it, otherwise the
// first free "Bản sao của <name>" / "Bản sao của <name> (n)".
func (s *Service) freeCopyName(ctx context.Context, place *copyPlace, name string) (string, error) {
	for n := 0; n <= maxCopyNames; n++ {
		candidate := name
		switch {
		case n == 1:
			candidate = copyPrefix + name
		case n > 1:
			candidate = fmt.Sprintf("%s%s (%d)", copyPrefix, name, n)
		}
		used, err := s.store.NameInUse(ctx, place.workspaceID, place.driveContext, place.driveContextID, place.parentID, candidate)
		if err != nil {
			return "", fmt.Errorf("check name: %w", err)
		}
		if !used {
			return candidate, nil
		}
	}
	return "", conflict("too many copies with this name in the destination")
}

// abandonCopy undoes a copy that failed after its object was stored: the row
// (when one was inserted) and the object, so a broken copy leaves nothing behind.
func (s *Service) abandonCopy(callCtx context.Context, rowID, workspaceID, objectKey string) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(callCtx), cleanupTimeout)
	defer cancel()
	if rowID != "" {
		if err := s.store.DeleteItem(ctx, rowID); err != nil {
			slog.Error("failed copy's row not removed", "item", rowID, "err", err)
		}
	}
	if _, err := s.docStorage.DeleteObject(ctx, &docpb.DeleteObjectRequest{
		WorkspaceId: workspaceID, ObjectKey: objectKey,
	}); err != nil {
		slog.Error("failed copy's stored object not removed", "key", objectKey, "err", err)
	}
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
	if err := s.refuseRoot(ctx, item, rootTrash); err != nil {
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
	if err := s.refuseRoot(ctx, item, rootDelete); err != nil {
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
		// A shared file's own OA goes with it; other files have none.
		s.releaseOwnOA(ctx, f)
		if f.ObjectKey != nil {
			s.removeRendition(ctx, f)
			s.removeThumbnails(ctx, f)
			if _, err := s.docStorage.DeleteObject(ctx, &docpb.DeleteObjectRequest{
				WorkspaceId: f.WorkspaceID, ObjectKey: *f.ObjectKey,
			}); err != nil {
				slog.Error("deleted file's stored object not removed", "item", f.ID, "err", err)
			}
		}
	}

	if item.ItemType == "folder" {
		// The folder is already gone, so a failure here cannot be retried by the
		// caller; it leaves an unreachable OA with no row, which is logged for
		// repair. Only an OA the folder owned alone is deleted.
		s.releaseOwnOA(ctx, item)
	}
	s.announce(ctx, realtime.KindDeleted, item)
	return &pb.Empty{}, nil
}
