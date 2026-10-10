package grpc

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"ngac-platform/ngac"
	"ngac-platform/pkg/grpcauth"
	"ngac-platform/pkg/provision"
	docpb "ngac-platform/proto/document"
	pb "ngac-platform/proto/drive"
	policypb "ngac-platform/proto/policy"
	"ngac-platform/services/drive/internal/reason"
	"ngac-platform/services/drive/internal/store"
)

// DriveServer implements the DriveService gRPC API.
type DriveServer struct {
	pb.UnimplementedDriveServiceServer
	store       *store.Store
	policyRead  policypb.PolicyReadServiceClient
	policyWrite policypb.PolicyWriteServiceClient
	docStorage  docpb.DocumentStorageServiceClient
}

// NewDriveServer creates a drive handler with all required dependencies.
func NewDriveServer(
	db *pgxpool.Pool,
	pr policypb.PolicyReadServiceClient,
	pw policypb.PolicyWriteServiceClient,
	ds docpb.DocumentStorageServiceClient,
) *DriveServer {
	return &DriveServer{
		store:       store.NewStore(db),
		policyRead:  pr,
		policyWrite: pw,
		docStorage:  ds,
	}
}

// checkAccess verifies NGAC access, returning an error if denied.
func (s *DriveServer) checkAccess(ctx context.Context, userNodeID, objectNodeID, operation string) error {
	resp, err := s.policyRead.CheckAccess(ctx, &policypb.CheckAccessRequest{
		UserNodeId: userNodeID, ObjectNodeId: objectNodeID, Operation: operation,
	})
	if !ngac.Allowed(resp.GetDecision(), err) {
		return status.Errorf(codes.PermissionDenied, "access denied")
	}
	return nil
}

// checkAccessOnNamedOA verifies NGAC access on a well-known OA identified by
// name (always built with a helper from package ngac). The name is resolved to
// its node first; a caller with no identity, or an OA that cannot be resolved,
// denies — there is nothing that could grant the right.
func (s *DriveServer) checkAccessOnNamedOA(ctx context.Context, userNodeID, oaName, operation string) error {
	if userNodeID == "" {
		return status.Errorf(codes.PermissionDenied, "access denied")
	}
	node, err := s.policyRead.FindNodeByName(ctx, &policypb.FindNodeByNameRequest{
		Name: oaName, NodeType: ngac.TypeOA,
	})
	if err != nil || node.GetId() == "" {
		return status.Errorf(codes.PermissionDenied, "access denied")
	}
	return s.checkAccess(ctx, userNodeID, node.GetId(), operation)
}

// liveFolder loads a folder that can be opened or filled: it exists, is active
// (a trashed or still-uploading item is not a place anyone can browse into) and
// belongs to workspaceID. An empty workspaceID skips the workspace check.
//
// Every refusal is NotFound, so a caller learns nothing about a folder in
// another workspace or in the trash beyond "not there".
func (s *DriveServer) liveFolder(ctx context.Context, id, workspaceID string) (*store.DriveItem, error) {
	item, err := s.store.GetItem(ctx, id)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "load folder: %v", err)
	}
	if item == nil || item.Status != "active" || item.ItemType != "folder" ||
		(workspaceID != "" && item.WorkspaceID != workspaceID) {
		return nil, status.Errorf(codes.NotFound, "folder not found")
	}
	return item, nil
}

// itemToProto converts a store.DriveItem to a protobuf DriveItem.
func itemToProto(item *store.DriveItem) *pb.DriveItem {
	if item == nil {
		return nil
	}
	p := &pb.DriveItem{
		Id:             item.ID,
		WorkspaceId:    item.WorkspaceID,
		DriveContext:   item.DriveContext,
		DriveContextId: item.DriveContextID,
		ItemType:       item.ItemType,
		Name:           item.Name,
		NgacNodeId:     item.NGACNodeID,
		OwnerId:        item.OwnerID,
		Status:         item.Status,
		CreatedAt:      timestamppb.New(item.CreatedAt),
		UpdatedAt:      timestamppb.New(item.UpdatedAt),
	}
	if item.ParentID != nil {
		p.ParentId = *item.ParentID
	}
	if item.MimeType != nil {
		p.MimeType = *item.MimeType
	}
	if item.SizeBytes != nil {
		p.SizeBytes = *item.SizeBytes
	}
	if item.ObjectKey != nil {
		p.ObjectKey = *item.ObjectKey
	}
	return p
}

// CreateFolder creates a new folder in the drive hierarchy.
func (s *DriveServer) CreateFolder(ctx context.Context, req *pb.CreateFolderRequest) (*pb.DriveItem, error) {
	driveCtx := req.DriveContext
	if driveCtx == "" {
		driveCtx = "workspace"
	}

	// Determine parent NGAC OA for the new folder
	var parentNGACID string
	var parent *store.DriveItem
	if req.ParentId != "" {
		var err error
		parent, err = s.liveFolder(ctx, req.ParentId, req.WorkspaceId)
		if err != nil {
			return nil, err
		}
		if err := s.checkAccess(ctx, grpcauth.CallerFrom(ctx).NGACNodeID, parent.NGACNodeID, ngac.OpWrite); err != nil {
			return nil, err
		}
		parentNGACID = parent.NGACNodeID
	} else {
		root, err := s.ensureRoot(ctx, req.WorkspaceId, driveCtx, req.DriveContextId, grpcauth.CallerFrom(ctx).NGACNodeID)
		if err != nil {
			return nil, err
		}
		parentNGACID = root.NGACNodeID
	}

	// The folder's ID is chosen first so its OA can be named by it. A name taken
	// from the folder would be shared by every folder called "Reports", in this
	// workspace and in other tenants', and the graph resolves nodes by exact name.
	itemID := uuid.New().String()

	// Create NGAC OA node for the folder. A failure at any later step removes it.
	prov := provision.NewCreator(s.policyWrite)
	folderNode, err := prov.Node(ctx, &policypb.CreateNodeRequest{
		Name: ngac.FolderNodeName(ngac.FolderID(itemID)), NodeType: ngac.TypeOA,
		Properties: map[string]string{
			"type": "drive_folder", "workspace_id": req.WorkspaceId, ngac.PropDisplayName: req.Name,
		},
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "create folder node: %v", err)
	}

	// Assign folder OA under parent OA (inherits permissions). Without this
	// edge the folder reaches no PC and every check on it denies.
	if err := prov.Assign(ctx, folderNode.Id, parentNGACID); err != nil {
		return nil, status.Errorf(codes.Internal, "assign folder under parent: %v", prov.Fail(ctx, err))
	}

	// Determine scope OA: inherit from parent, or use own OA for root-level folders
	var scopeOAID string
	if parent != nil && parent.ScopeOAID != "" {
		scopeOAID = parent.ScopeOAID
	}
	if scopeOAID == "" {
		scopeOAID = folderNode.Id
	}

	item := &store.DriveItem{
		ID:             itemID,
		WorkspaceID:    req.WorkspaceId,
		DriveContext:   driveCtx,
		DriveContextID: req.DriveContextId,
		ParentID:       nilStr(req.ParentId),
		ItemType:       "folder",
		Name:           req.Name,
		NGACNodeID:     folderNode.Id,
		ScopeOAID:      scopeOAID,
		OwnerID:        grpcauth.CallerFrom(ctx).NGACNodeID,
		Status:         "active",
	}
	if err := s.store.InsertItem(ctx, item); err != nil {
		return nil, status.Errorf(codes.Internal, "insert folder: %v", prov.Fail(ctx, err))
	}
	prov.Done()

	slog.Info("folder created", "id", item.ID, "name", req.Name)
	return itemToProto(item), nil
}

// ListFolder returns the contents of a folder with NGAC filtering.
func (s *DriveServer) ListFolder(ctx context.Context, req *pb.ListFolderRequest) (*pb.DriveItemList, error) {
	driveCtx := req.DriveContext
	if driveCtx == "" {
		driveCtx = "workspace"
	}

	var parentID *string
	if req.FolderId != "" {
		parentID = &req.FolderId
		// Check read access on the folder itself
		folder, err := s.liveFolder(ctx, req.FolderId, req.WorkspaceId)
		if err != nil {
			return nil, err
		}
		if err := s.checkAccess(ctx, grpcauth.CallerFrom(ctx).NGACNodeID, folder.NGACNodeID, ngac.OpRead); err != nil {
			return nil, err
		}
	}

	items, err := s.store.ListChildren(ctx, parentID, req.WorkspaceId, driveCtx, req.DriveContextId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list folder: %v", err)
	}

	// Filter by NGAC access. One batch call rather than one per item: a folder
	// of 200 files used to mean 200 sequential round-trips to the policy
	// service, which dominated the latency of listing anything.
	objectIDs := make([]string, 0, len(items))
	for _, item := range items {
		objectIDs = append(objectIDs, item.NGACNodeID)
	}

	var visible []*pb.DriveItem
	if len(objectIDs) > 0 {
		batch, err := s.policyRead.BatchCheckAccess(ctx, &policypb.BatchCheckAccessRequest{
			UserNodeId: grpcauth.CallerFrom(ctx).NGACNodeID,
			ObjectIds:  objectIDs,
			Operations: []string{ngac.OpRead},
		})
		if err != nil {
			// Fail closed: an unreadable policy answer must not list everything.
			return nil, status.Errorf(codes.Internal, "batch access check: %v", err)
		}
		for _, item := range items {
			if batch.GetResults()[item.NGACNodeID].GetPermissions()[ngac.OpRead] {
				visible = append(visible, itemToProto(item))
			}
		}
	}

	result := &pb.DriveItemList{Items: visible}

	// Build breadcrumb if inside a subfolder
	if req.FolderId != "" {
		crumbs, _ := s.store.GetBreadcrumb(ctx, req.FolderId)
		for _, c := range crumbs {
			result.Breadcrumb = append(result.Breadcrumb, &pb.BreadcrumbEntry{Id: c.ID, Name: c.Name})
		}
	}

	return result, nil
}

// GetItem returns a single drive item after NGAC read check.
func (s *DriveServer) GetItem(ctx context.Context, req *pb.GetItemRequest) (*pb.DriveItem, error) {
	item, err := s.store.GetItem(ctx, req.ItemId)
	// A trashed item is gone as far as this call is concerned; it is reached
	// again only through RestoreItem. (A pending upload stays readable: the
	// uploader looks it up between CreateFile and ConfirmFile.)
	if err != nil || item == nil || item.Status == "trashed" {
		return nil, status.Errorf(codes.NotFound, "item not found")
	}
	if err := s.checkAccess(ctx, grpcauth.CallerFrom(ctx).NGACNodeID, item.NGACNodeID, ngac.OpRead); err != nil {
		return nil, err
	}
	return itemToProto(item), nil
}

// CreateFile initiates a file upload — creates NGAC node, drive_item, and returns presigned URL.
func (s *DriveServer) CreateFile(ctx context.Context, req *pb.CreateFileRequest) (*pb.CreateFileResponse, error) {
	driveCtx := req.DriveContext
	if driveCtx == "" {
		driveCtx = "workspace"
	}

	// Check quota
	ok, err := s.store.CheckQuota(ctx, req.WorkspaceId, req.SizeBytes)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "check quota: %v", err)
	}
	if !ok {
		return nil, status.Errorf(codes.ResourceExhausted, "storage quota exceeded")
	}

	// Determine parent NGAC node and scope OA
	var parentNGACID string
	var parentScopeOAID string
	if req.ParentId != "" {
		parent, err := s.liveFolder(ctx, req.ParentId, req.WorkspaceId)
		if err != nil {
			return nil, err
		}
		parentNGACID = parent.NGACNodeID
		parentScopeOAID = parent.ScopeOAID
	} else {
		root, err := s.ensureRoot(ctx, req.WorkspaceId, driveCtx, req.DriveContextId, grpcauth.CallerFrom(ctx).NGACNodeID)
		if err != nil {
			return nil, err
		}
		parentNGACID = root.NGACNodeID
		parentScopeOAID = root.ScopeOAID
	}

	// One check covering both destinations.
	//
	// This used to sit inside the ParentId branch only, so uploading to the
	// drive root — which is what the Upload button does — was not authorized at
	// all. A workspace member, who holds only read on Documents, could create
	// the row and push the bytes; nothing refused until ConfirmFile, and before
	// ConfirmFile checked anything the upload simply succeeded.
	//
	// Checking here also means the caller is refused before uploading rather
	// than after, instead of leaving an orphaned object in storage and a
	// pending row behind.
	if err := s.checkAccess(ctx, grpcauth.CallerFrom(ctx).NGACNodeID, parentNGACID, ngac.OpWrite); err != nil {
		return nil, err
	}

	// Files inherit the parent folder's OA node — no NGAC node created.
	// checkAccess uses the folder OA for authorization.
	fileNGACNodeID := parentNGACID

	fileID := uuid.New().String()

	// Get presigned upload URL from Document Storage
	uploadResp, err := s.docStorage.GetUploadURL(ctx, &docpb.GetUploadURLRequest{
		WorkspaceId: req.WorkspaceId, Filename: req.Name,
		MimeType: req.MimeType, DocId: fileID,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get upload url: %v", err)
	}

	mimeType := req.MimeType
	sizeBytes := req.SizeBytes
	item := &store.DriveItem{
		ID:             fileID,
		WorkspaceID:    req.WorkspaceId,
		DriveContext:   driveCtx,
		DriveContextID: req.DriveContextId,
		ParentID:       nilStr(req.ParentId),
		ItemType:       "file",
		Name:           req.Name,
		MimeType:       &mimeType,
		SizeBytes:      &sizeBytes,
		ObjectKey:      &uploadResp.ObjectKey,
		NGACNodeID:     fileNGACNodeID,
		ScopeOAID:      parentScopeOAID,
		OwnerID:        grpcauth.CallerFrom(ctx).UserID,
		Status:         "pending",
	}
	if err := s.store.InsertItem(ctx, item); err != nil {
		return nil, status.Errorf(codes.Internal, "insert file: %v", err)
	}

	return &pb.CreateFileResponse{
		FileId:    fileID,
		UploadUrl: uploadResp.UploadUrl,
		ObjectKey: uploadResp.ObjectKey,
	}, nil
}

// ConfirmFile finalizes a file upload after the client has PUT to MinIO.
func (s *DriveServer) ConfirmFile(ctx context.Context, req *pb.ConfirmFileRequest) (*pb.DriveItem, error) {
	item, err := s.store.GetItem(ctx, req.FileId)
	if err != nil || item == nil {
		return nil, status.Errorf(codes.NotFound, "file not found")
	}
	// Confirming publishes the upload and charges it against the workspace
	// quota, so it takes the same right as creating the file did.
	if err := s.checkAccess(ctx, grpcauth.CallerFrom(ctx).NGACNodeID, item.NGACNodeID, ngac.OpWrite); err != nil {
		return nil, err
	}
	if item.Status != "pending" {
		return nil, status.Errorf(codes.FailedPrecondition, "file not pending")
	}

	// Verify object in MinIO
	confirmResp, err := s.docStorage.ConfirmUpload(ctx, &docpb.ConfirmUploadRequest{
		WorkspaceId: item.WorkspaceID, ObjectKey: *item.ObjectKey,
	})
	if err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "file not uploaded: %v", err)
	}

	// Update size from actual upload
	s.store.UpdateStatus(ctx, item.ID, "active")
	actualSize := confirmResp.SizeBytes
	s.store.UpdateFileSize(ctx, item.ID, actualSize)

	// Update quota
	s.store.IncrementQuota(ctx, item.WorkspaceID, actualSize, 1)

	item.Status = "active"
	item.SizeBytes = &actualSize
	slog.Info("file confirmed", "id", item.ID, "name", item.Name, "size", actualSize)
	return itemToProto(item), nil
}

// GetDownloadURL returns a presigned download URL after NGAC read check.
func (s *DriveServer) GetDownloadURL(ctx context.Context, req *pb.GetDownloadURLRequest) (*pb.GetDownloadURLResponse, error) {
	item, err := s.store.GetItem(ctx, req.FileId)
	if err != nil || item == nil {
		return nil, status.Errorf(codes.NotFound, "file not found")
	}
	if err := s.checkAccess(ctx, grpcauth.CallerFrom(ctx).NGACNodeID, item.NGACNodeID, ngac.OpRead); err != nil {
		return nil, err
	}

	dlResp, err := s.docStorage.GetDownloadURL(ctx, &docpb.GetDownloadURLRequest{
		WorkspaceId: item.WorkspaceID, ObjectKey: *item.ObjectKey,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get download url: %v", err)
	}

	var size int64
	var mime string
	if item.SizeBytes != nil {
		size = *item.SizeBytes
	}
	if item.MimeType != nil {
		mime = *item.MimeType
	}

	return &pb.GetDownloadURLResponse{
		DownloadUrl: dlResp.DownloadUrl,
		Filename:    item.Name,
		MimeType:    mime,
		SizeBytes:   size,
	}, nil
}

// RenameItem renames a file or folder.
func (s *DriveServer) RenameItem(ctx context.Context, req *pb.RenameItemRequest) (*pb.DriveItem, error) {
	item, err := s.store.GetItem(ctx, req.ItemId)
	if err != nil || item == nil {
		return nil, status.Errorf(codes.NotFound, "item not found")
	}
	if err := s.checkAccess(ctx, grpcauth.CallerFrom(ctx).NGACNodeID, item.NGACNodeID, ngac.OpWrite); err != nil {
		return nil, err
	}
	if err := s.store.UpdateName(ctx, item.ID, req.NewName); err != nil {
		return nil, status.Errorf(codes.Internal, "rename: %v", err)
	}
	item.Name = req.NewName
	return itemToProto(item), nil
}

// MoveItem moves an item within the same drive context. An empty NewParentId
// moves it to the top level of that drive.
//
// Moves of one item are serialised (store.LockItem), and the row is updated
// only if the item is still under the parent this move started from, so two
// concurrent moves cannot leave the item under both destinations in the graph.
func (s *DriveServer) MoveItem(ctx context.Context, req *pb.MoveItemRequest) (*pb.DriveItem, error) {
	callerNode := grpcauth.CallerFrom(ctx).NGACNodeID
	item, err := s.store.GetItem(ctx, req.ItemId)
	if err != nil || item == nil || item.Status == "trashed" {
		return nil, status.Errorf(codes.NotFound, "item not found")
	}
	if err := s.checkAccess(ctx, callerNode, item.NGACNodeID, ngac.OpWrite); err != nil {
		return nil, err
	}

	unlock, err := s.store.LockItem(ctx, item.ID)
	if err != nil {
		return nil, status.Errorf(codes.Unavailable, "wait for concurrent move: %v", err)
	}
	defer unlock()

	// Read again under the lock: the item may have moved, or been trashed,
	// while this call waited, and what follows must start from where it is now.
	prevNode := item.NGACNodeID
	item, err = s.store.GetItem(ctx, req.ItemId)
	if err != nil || item == nil || item.Status == "trashed" {
		return nil, status.Errorf(codes.NotFound, "item not found")
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
			return nil, status.Errorf(codes.InvalidArgument, "the drive root cannot be moved")
		}
		dest = root // top-level items have no parent row; the root is only their OA
	} else {
		dest, err = s.store.GetItem(ctx, req.NewParentId)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "load destination: %v", err)
		}
		if dest == nil || dest.Status != "active" {
			return nil, status.Errorf(codes.NotFound, "destination not found")
		}
		if dest.ItemType != "folder" {
			return nil, status.Errorf(codes.InvalidArgument, "destination is not a folder")
		}
		if dest.WorkspaceID != item.WorkspaceID || dest.DriveContext != item.DriveContext ||
			(item.DriveContext != "workspace" && dest.DriveContextID != item.DriveContextID) {
			return nil, status.Errorf(codes.InvalidArgument, "an item can only be moved within its own drive")
		}
		if item.ItemType == "folder" {
			inside, err := s.store.IsAncestorOrSelf(ctx, item.ID, dest.ID)
			if err != nil {
				return nil, status.Errorf(codes.Internal, "check destination: %v", err)
			}
			if inside {
				return nil, status.Errorf(codes.InvalidArgument,
					"a folder cannot be moved into itself or one of its subfolders")
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
			return nil, status.Errorf(codes.Internal, "move folder: %v", err)
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
				return nil, status.Errorf(codes.Internal, "move failed and could not be undone: %v", rerr)
			}
		}
		if err != nil {
			return nil, status.Errorf(codes.Internal, "move: %v", err)
		}
		return nil, status.Errorf(codes.Aborted, "item was changed by another request; retry")
	}
	item.NGACNodeID = newNodeID
	item.ParentID = newParentID
	return itemToProto(item), nil
}

// currentParentOA returns the OA a folder is assigned under today: its parent
// folder's OA, or the drive root's for a top-level folder. "" means unknown
// (no parent row or root found), in which case there is no edge to remove.
func (s *DriveServer) currentParentOA(ctx context.Context, item *store.DriveItem) (string, error) {
	if item.ParentID != nil {
		old, err := s.store.GetItem(ctx, *item.ParentID)
		if err != nil {
			return "", status.Errorf(codes.Internal, "load current parent: %v", err)
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
		return "", status.Errorf(codes.Internal, "find drive root: %v", err)
	}
	if root == nil {
		return "", nil
	}
	if root.ID == item.ID {
		return "", status.Errorf(codes.InvalidArgument, "the drive root cannot be moved")
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
func (s *DriveServer) reparent(ctx context.Context, childOA, fromOA, toOA string) error {
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
func (s *DriveServer) CopyItem(ctx context.Context, req *pb.CopyItemRequest) (*pb.DriveItem, error) {
	src, err := s.store.GetItem(ctx, req.ItemId)
	if err != nil || src == nil {
		return nil, status.Errorf(codes.NotFound, "source not found")
	}
	if src.ItemType != "file" {
		return nil, status.Errorf(codes.InvalidArgument, "only files can be copied")
	}
	if err := s.checkAccess(ctx, grpcauth.CallerFrom(ctx).NGACNodeID, src.NGACNodeID, ngac.OpRead); err != nil {
		return nil, err
	}

	// Determine destination parent NGAC
	dest, err := s.store.GetItem(ctx, req.DestParentId)
	if err != nil || dest == nil {
		return nil, status.Errorf(codes.NotFound, "destination not found")
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
		return nil, status.Errorf(codes.Internal, "copy object: %v", err)
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
		return nil, status.Errorf(codes.Internal, "insert copy: %v", err)
	}
	return itemToProto(newItem), nil
}

// TrashItem soft-deletes an item (recursive for folders).
func (s *DriveServer) TrashItem(ctx context.Context, req *pb.TrashItemRequest) (*pb.Empty, error) {
	item, err := s.store.GetItem(ctx, req.ItemId)
	if err != nil || item == nil {
		return nil, status.Errorf(codes.NotFound, "item not found")
	}
	if err := s.checkAccess(ctx, grpcauth.CallerFrom(ctx).NGACNodeID, item.NGACNodeID, ngac.OpWrite); err != nil {
		return nil, err
	}
	s.store.UpdateStatus(ctx, item.ID, "trashed")
	if item.ItemType == "folder" {
		s.store.TrashChildren(ctx, item.ID)
	}
	return &pb.Empty{}, nil
}

// RestoreItem restores a trashed item.
func (s *DriveServer) RestoreItem(ctx context.Context, req *pb.RestoreItemRequest) (*pb.DriveItem, error) {
	item, err := s.store.GetItem(ctx, req.ItemId)
	if err != nil || item == nil {
		return nil, status.Errorf(codes.NotFound, "item not found")
	}
	if err := s.checkAccess(ctx, grpcauth.CallerFrom(ctx).NGACNodeID, item.NGACNodeID, ngac.OpWrite); err != nil {
		return nil, err
	}
	s.store.UpdateStatus(ctx, item.ID, "active")
	if item.ItemType == "folder" {
		s.store.RestoreChildren(ctx, item.ID)
	}
	item.Status = "active"
	return itemToProto(item), nil
}

// DeleteItem permanently removes an item and its storage.
func (s *DriveServer) DeleteItem(ctx context.Context, req *pb.DeleteItemRequest) (*pb.Empty, error) {
	item, err := s.store.GetItem(ctx, req.ItemId)
	if err != nil || item == nil {
		return nil, status.Errorf(codes.NotFound, "item not found")
	}
	// Same right as TrashItem: permanent deletion must not be reachable by a
	// user who cannot perform the reversible version of the same action.
	if err := s.checkAccess(ctx, grpcauth.CallerFrom(ctx).NGACNodeID, item.NGACNodeID, ngac.OpWrite); err != nil {
		return nil, err
	}

	var files []*store.DriveItem
	if item.ItemType == "folder" {
		// Refuse before anything is touched: text documents block the delete
		// (their folder reference is RESTRICT), and a half-finished delete would
		// leave the folder row with its access node gone.
		n, err := s.store.CountTextDocumentsUnder(ctx, item.ID)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "check folder contents: %v", err)
		}
		if n > 0 {
			return nil, status.Error(codes.FailedPrecondition, reason.FolderHasDocuments)
		}
		if files, err = s.store.GetChildFiles(ctx, item.ID); err != nil {
			return nil, status.Errorf(codes.Internal, "list folder files: %v", err)
		}
	} else {
		files = []*store.DriveItem{item}
	}

	// The row goes first. Stored objects, quota and the folder's access node are
	// only released once the database has agreed, so a refusal (a document saved
	// since the check above) leaves the folder exactly as it was.
	if err := s.store.DeleteItem(ctx, item.ID); err != nil {
		if errors.Is(err, store.ErrFolderHasDocuments) {
			return nil, status.Error(codes.FailedPrecondition, reason.FolderHasDocuments)
		}
		return nil, status.Errorf(codes.Internal, "delete item: %v", err)
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
		if f.SizeBytes != nil {
			if err := s.store.DecrementQuota(ctx, f.WorkspaceID, *f.SizeBytes, 1); err != nil {
				slog.Error("deleted file's quota not released", "item", f.ID, "err", err)
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
	return &pb.Empty{}, nil
}

// ensureRoot finds or auto-creates the root drive folder for a workspace/channel context.
// This self-heals workspaces created before drive tables existed.
func (s *DriveServer) ensureRoot(ctx context.Context, workspaceID, driveCtx, driveCtxID, userNodeID string) (*store.DriveItem, error) {
	// Normalise the context once, and use the same value to look up and to
	// store.
	//
	// These used to disagree: the root was written with the workspace ID
	// substituted for an empty context, while the lookup ran with the empty
	// value it was handed. The lookup therefore never matched the root — but
	// FindRootByContext selects on "top-level folder in this context" with no
	// further discriminator, so as soon as a user created a folder at the top
	// level, that folder was returned as the drive root. Everything created at
	// the top level afterwards was parented under it in the NGAC graph and
	// inherited its permissions, including any share placed on it.
	rootCtxID := driveCtxID
	if rootCtxID == "" {
		rootCtxID = workspaceID
	}

	root, err := s.store.FindRootByContext(ctx, workspaceID, driveCtx, rootCtxID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "find root: %v", err)
	}
	if root != nil {
		return root, nil
	}

	// Auto-create root. It roots on the workspace's Documents OA, which the
	// workspace records when it is created; the OA is never worked out from node
	// names (a scan for "Documents"/"Docs" with a first-OA fallback depended on
	// the order the graph returned children in, and could pick a folder somebody
	// had named "Docs").
	prov := provision.NewCreator(s.policyWrite)
	ngacNodeID, err := s.rootOA(ctx, prov, workspaceID)
	if err != nil {
		return nil, err
	}

	item := &store.DriveItem{
		ID:             uuid.New().String(),
		WorkspaceID:    workspaceID,
		DriveContext:   driveCtx,
		DriveContextID: rootCtxID,
		ItemType:       "folder",
		Name:           "Root",
		NGACNodeID:     ngacNodeID,
		ScopeOAID:      ngacNodeID,
		OwnerID:        userNodeID,
		Status:         "active",
		IsRoot:         true,
	}
	if err := s.store.InsertItem(ctx, item); err != nil {
		cause := fmt.Errorf("insert root: %w", err)
		rbErr := prov.Fail(ctx, cause)
		if errors.Is(err, store.ErrRootExists) {
			// Another request created the root first. Whatever this one
			// created for it is gone; use the winner's.
			if winner, ferr := s.store.FindRootByContext(ctx, workspaceID, driveCtx, rootCtxID); ferr == nil && winner != nil {
				return winner, nil
			}
		}
		return nil, status.Errorf(codes.Internal, "%v", rbErr)
	}
	prov.Done()

	slog.Info("auto-created drive root", "workspace", workspaceID, "context", driveCtx, "root_id", item.ID)
	return item, nil
}

// rootOA returns the OA a workspace's drive root hangs on: its recorded
// Documents OA. A workspace with none recorded (one created outside the
// workspace service) gets a DriveRoot OA of its own, named by the workspace ID,
// found again on the next call rather than created twice. Nodes this creates are
// recorded in prov so a later failure removes them.
func (s *DriveServer) rootOA(ctx context.Context, prov *provision.Creator, workspaceID string) (string, error) {
	docsOA, err := s.store.GetWorkspaceDocumentsOAID(ctx, workspaceID)
	if err != nil {
		return "", status.Errorf(codes.Internal, "look up documents OA: %v", err)
	}
	if docsOA != "" {
		return docsOA, nil
	}

	node, err := prov.EnsureNode(ctx, s.policyRead, &policypb.CreateNodeRequest{
		Name: ngac.DriveRootName(ngac.WorkspaceID(workspaceID)), NodeType: ngac.TypeOA,
	})
	if err != nil {
		return "", status.Errorf(codes.Internal, "create root node: %v", err)
	}

	// Assign DriveRoot OA under workspace PC so files inherit access associations
	pcID, err := s.store.GetWorkspacePCID(ctx, workspaceID)
	if err == nil && pcID != "" {
		if err := prov.Assign(ctx, node.Id, pcID); err != nil {
			return "", status.Errorf(codes.Internal, "assign drive root under workspace PC: %v", prov.Fail(ctx, err))
		}
	}
	return node.Id, nil
}

func nilStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
