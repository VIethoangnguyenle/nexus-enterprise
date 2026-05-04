package grpc

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"ngac-platform/ngac"
	docpb "ngac-platform/proto/document"
	pb "ngac-platform/proto/drive"
	policypb "ngac-platform/proto/policy"
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
	if err != nil {
		return status.Errorf(codes.Internal, "check access: %v", err)
	}
	if resp.Decision == ngac.DecisionDeny {
		return status.Errorf(codes.PermissionDenied, "access denied")
	}
	return nil
}

// itemToProto converts a store.DriveItem to a protobuf DriveItem.
func itemToProto(item *store.DriveItem) *pb.DriveItem {
	if item == nil {
		return nil
	}
	p := &pb.DriveItem{
		Id:            item.ID,
		WorkspaceId:   item.WorkspaceID,
		DriveContext:  item.DriveContext,
		DriveContextId: item.DriveContextID,
		ItemType:      item.ItemType,
		Name:          item.Name,
		NgacNodeId:    item.NGACNodeID,
		OwnerId:       item.OwnerID,
		Status:        item.Status,
		CreatedAt:     timestamppb.New(item.CreatedAt),
		UpdatedAt:     timestamppb.New(item.UpdatedAt),
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
	if req.ParentId != "" {
		parent, err := s.store.GetItem(ctx, req.ParentId)
		if err != nil || parent == nil {
			return nil, status.Errorf(codes.NotFound, "parent folder not found")
		}
		if err := s.checkAccess(ctx, req.UserNgacNodeId, parent.NGACNodeID, ngac.OpWrite); err != nil {
			return nil, err
		}
		parentNGACID = parent.NGACNodeID
	} else {
		root, err := s.ensureRoot(ctx, req.WorkspaceId, driveCtx, req.DriveContextId, req.UserNgacNodeId)
		if err != nil {
			return nil, err
		}
		parentNGACID = root.NGACNodeID
	}

	// Create NGAC OA node for the folder
	folderNode, err := s.policyWrite.CreateNode(ctx, &policypb.CreateNodeRequest{
		Name: ngac.FolderNodeName(req.Name), NodeType: ngac.TypeOA,
		Properties: map[string]string{"type": "drive_folder", "workspace_id": req.WorkspaceId},
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "create folder node: %v", err)
	}

	// Assign folder OA under parent OA (inherits permissions)
	s.policyWrite.CreateAssignment(ctx, &policypb.CreateAssignmentRequest{
		ChildId: folderNode.Id, ParentId: parentNGACID,
	})

	// Determine scope OA: inherit from parent, or use own OA for root-level folders
	var scopeOAID string
	if req.ParentId != "" {
		parent, _ := s.store.GetItem(ctx, req.ParentId)
		if parent != nil && parent.ScopeOAID != "" {
			scopeOAID = parent.ScopeOAID
		}
	}
	if scopeOAID == "" {
		scopeOAID = folderNode.Id
	}

	item := &store.DriveItem{
		ID:           uuid.New().String(),
		WorkspaceID:  req.WorkspaceId,
		DriveContext: driveCtx,
		DriveContextID: req.DriveContextId,
		ParentID:     nilStr(req.ParentId),
		ItemType:     "folder",
		Name:         req.Name,
		NGACNodeID:   folderNode.Id,
		ScopeOAID:    scopeOAID,
		OwnerID:      req.UserNgacNodeId,
		Status:       "active",
	}
	if err := s.store.InsertItem(ctx, item); err != nil {
		return nil, status.Errorf(codes.Internal, "insert folder: %v", err)
	}

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
		folder, err := s.store.GetItem(ctx, req.FolderId)
		if err != nil || folder == nil {
			return nil, status.Errorf(codes.NotFound, "folder not found")
		}
		if err := s.checkAccess(ctx, req.UserNgacNodeId, folder.NGACNodeID, "read"); err != nil {
			return nil, err
		}
	}

	items, err := s.store.ListChildren(ctx, parentID, req.WorkspaceId, driveCtx, req.DriveContextId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list folder: %v", err)
	}

	// Filter by NGAC access
	var visible []*pb.DriveItem
	for _, item := range items {
		resp, _ := s.policyRead.CheckAccess(ctx, &policypb.CheckAccessRequest{
			UserNodeId: req.UserNgacNodeId, ObjectNodeId: item.NGACNodeID, Operation: ngac.OpRead,
		})
		if resp != nil && resp.Decision == ngac.DecisionAllow {
			visible = append(visible, itemToProto(item))
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
	if err != nil || item == nil {
		return nil, status.Errorf(codes.NotFound, "item not found")
	}
	if err := s.checkAccess(ctx, req.UserNgacNodeId, item.NGACNodeID, ngac.OpRead); err != nil {
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
		parent, err := s.store.GetItem(ctx, req.ParentId)
		if err != nil || parent == nil {
			return nil, status.Errorf(codes.NotFound, "parent folder not found")
		}
		if err := s.checkAccess(ctx, req.UserNgacNodeId, parent.NGACNodeID, ngac.OpWrite); err != nil {
			return nil, err
		}
		parentNGACID = parent.NGACNodeID
		parentScopeOAID = parent.ScopeOAID
	} else {
		root, err := s.ensureRoot(ctx, req.WorkspaceId, driveCtx, req.DriveContextId, req.UserNgacNodeId)
		if err != nil {
			return nil, err
		}
		parentNGACID = root.NGACNodeID
		parentScopeOAID = root.ScopeOAID
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
		OwnerID:        req.UserId,
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
	if err := s.checkAccess(ctx, req.UserNgacNodeId, item.NGACNodeID, ngac.OpRead); err != nil {
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
	if err := s.checkAccess(ctx, req.UserNgacNodeId, item.NGACNodeID, ngac.OpWrite); err != nil {
		return nil, err
	}
	if err := s.store.UpdateName(ctx, item.ID, req.NewName); err != nil {
		return nil, status.Errorf(codes.Internal, "rename: %v", err)
	}
	item.Name = req.NewName
	return itemToProto(item), nil
}

// MoveItem moves an item within the same drive context.
func (s *DriveServer) MoveItem(ctx context.Context, req *pb.MoveItemRequest) (*pb.DriveItem, error) {
	item, err := s.store.GetItem(ctx, req.ItemId)
	if err != nil || item == nil {
		return nil, status.Errorf(codes.NotFound, "item not found")
	}
	if err := s.checkAccess(ctx, req.UserNgacNodeId, item.NGACNodeID, ngac.OpWrite); err != nil {
		return nil, err
	}
	dest, err := s.store.GetItem(ctx, req.NewParentId)
	if err != nil || dest == nil {
		return nil, status.Errorf(codes.NotFound, "destination not found")
	}
	if err := s.checkAccess(ctx, req.UserNgacNodeId, dest.NGACNodeID, ngac.OpWrite); err != nil {
		return nil, err
	}

	// NGAC: only reassign node for folders (folders have their own OA node).
	// Files don't have NGAC nodes — they inherit from parent folder.
	if item.ItemType == "folder" {
		if item.ParentID != nil {
			old, _ := s.store.GetItem(ctx, *item.ParentID)
			if old != nil {
				s.policyWrite.RemoveAssignment(ctx, &policypb.RemoveAssignmentRequest{
					ChildId: item.NGACNodeID, ParentId: old.NGACNodeID,
				})
			}
		}
		s.policyWrite.CreateAssignment(ctx, &policypb.CreateAssignmentRequest{
			ChildId: item.NGACNodeID, ParentId: dest.NGACNodeID,
		})
	}

	// For files: update NGACNodeID to inherit from new parent folder.
	if item.ItemType == "file" {
		item.NGACNodeID = dest.NGACNodeID
		s.store.UpdateNGACNodeID(ctx, item.ID, dest.NGACNodeID)
	}

	newParent := req.NewParentId
	if err := s.store.UpdateParent(ctx, item.ID, &newParent); err != nil {
		return nil, status.Errorf(codes.Internal, "move: %v", err)
	}
	item.ParentID = &newParent
	return itemToProto(item), nil
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
	if err := s.checkAccess(ctx, req.UserNgacNodeId, src.NGACNodeID, ngac.OpRead); err != nil {
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
		OwnerID: req.UserId, Status: "active",
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
	if err := s.checkAccess(ctx, req.UserNgacNodeId, item.NGACNodeID, ngac.OpWrite); err != nil {
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

	if item.ItemType == "folder" {
		children, _ := s.store.GetChildFiles(ctx, item.ID)
		for _, child := range children {
			if child.ObjectKey != nil {
				s.docStorage.DeleteObject(ctx, &docpb.DeleteObjectRequest{
					WorkspaceId: child.WorkspaceID, ObjectKey: *child.ObjectKey,
				})
			}
			if child.SizeBytes != nil {
				s.store.DecrementQuota(ctx, child.WorkspaceID, *child.SizeBytes, 1)
			}
			// Files don't have their own NGAC nodes — no DeleteNode needed.
		}
		// Delete the folder's OA node from NGAC graph.
		s.policyWrite.DeleteNode(ctx, &policypb.DeleteNodeRequest{NodeId: item.NGACNodeID})
	} else if item.ObjectKey != nil {
		s.docStorage.DeleteObject(ctx, &docpb.DeleteObjectRequest{
			WorkspaceId: item.WorkspaceID, ObjectKey: *item.ObjectKey,
		})
		if item.SizeBytes != nil {
			s.store.DecrementQuota(ctx, item.WorkspaceID, *item.SizeBytes, 1)
		}
		// Files inherit parent OA — no NGAC node to delete.
	}

	s.store.DeleteItem(ctx, item.ID)
	return &pb.Empty{}, nil
}

// ensureRoot finds or auto-creates the root drive folder for a workspace/channel context.
// This self-heals workspaces created before drive tables existed.
func (s *DriveServer) ensureRoot(ctx context.Context, workspaceID, driveCtx, driveCtxID, userNodeID string) (*store.DriveItem, error) {
	root, err := s.store.FindRootByContext(ctx, workspaceID, driveCtx, driveCtxID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "find root: %v", err)
	}
	if root != nil {
		return root, nil
	}

	// Auto-create root — find workspace's Documents OA from NGAC graph
	ngacNodeID := ""
	pcID, err := s.store.GetWorkspacePCID(ctx, workspaceID)
	if err == nil && pcID != "" {
		desc, _ := s.policyRead.GetChildren(ctx, &policypb.GetChildrenRequest{NodeId: pcID})
		if desc != nil {
			for _, n := range desc.Nodes {
				if n.NodeType == ngac.TypeOA && len(n.Name) > 0 {
					// Prefer Documents OA, fall back to first OA
					if ngacNodeID == "" {
						ngacNodeID = n.Id
					}
					if contains(n.Name, "Documents") || contains(n.Name, "Docs") {
						ngacNodeID = n.Id
						break
					}
				}
			}
		}
	}
	if ngacNodeID == "" {
		// Fallback: create a new OA node under the workspace PC
		node, nErr := s.policyWrite.CreateNode(ctx, &policypb.CreateNodeRequest{
			Name: fmt.Sprintf("DriveRoot_%s", workspaceID[:8]), NodeType: "OA",
		})
		if nErr != nil {
			return nil, status.Errorf(codes.Internal, "create root node: %v", nErr)
		}
		ngacNodeID = node.Id

		// Assign DriveRoot OA under workspace PC so files inherit access associations
		if pcID != "" {
			s.policyWrite.CreateAssignment(ctx, &policypb.CreateAssignmentRequest{
				ChildId: ngacNodeID, ParentId: pcID,
			})
		}
	}

	contextID := driveCtxID
	if contextID == "" {
		contextID = workspaceID
	}
	item := &store.DriveItem{
		ID:             uuid.New().String(),
		WorkspaceID:    workspaceID,
		DriveContext:   driveCtx,
		DriveContextID: contextID,
		ItemType:       "folder",
		Name:           "Root",
		NGACNodeID:     ngacNodeID,
		ScopeOAID:      ngacNodeID,
		OwnerID:        userNodeID,
		Status:         "active",
	}
	if err := s.store.InsertItem(ctx, item); err != nil {
		return nil, status.Errorf(codes.Internal, "insert root: %v", err)
	}

	slog.Info("auto-created drive root", "workspace", workspaceID, "context", driveCtx, "root_id", item.ID)
	return item, nil
}

// contains checks if s contains substr (case-insensitive not needed here, names are exact).
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[len(s)-len(substr):] == substr || s[:len(substr)] == substr || strings.Contains(s, substr)))
}

func nilStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
