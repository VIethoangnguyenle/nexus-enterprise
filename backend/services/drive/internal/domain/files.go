package domain

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"ngac-platform/ngac"
	"ngac-platform/pkg/grpcauth"
	"ngac-platform/pkg/grpcutil"
	"ngac-platform/pkg/realtime"
	docpb "ngac-platform/proto/document"
	pb "ngac-platform/proto/drive"
	"ngac-platform/services/drive/internal/store"
)

// CreateFile initiates a file upload — creates NGAC node, drive_item, and returns presigned URL.
func (s *Service) CreateFile(ctx context.Context, req *pb.CreateFileRequest) (*pb.CreateFileResponse, error) {
	if req.SizeBytes < 0 {
		return nil, invalid("size_bytes must not be negative")
	}
	declared := req.SizeBytes
	driveCtx, driveCtxID := normalizeContext(req.DriveContext, req.DriveContextId)

	// Determine parent NGAC node and scope OA. The drive root is authorized
	// before it is created (driveTop); the check below covers a parent folder.
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
		root, err := s.driveTop(ctx, req.WorkspaceId, driveCtx, driveCtxID, ngac.OpWrite, true)
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

	// The quota is checked only once the caller is known to be allowed to
	// write: a caller who may not upload is refused (403) whatever room is
	// left, and learns nothing about the workspace's usage.
	ok, err := s.store.CheckQuota(ctx, req.WorkspaceId, req.SizeBytes)
	if err != nil {
		return nil, fmt.Errorf("check quota: %w", err)
	}
	if !ok {
		return nil, quotaExceeded("storage quota exceeded")
	}

	// Files inherit the parent folder's OA node — no NGAC node created.
	// checkAccess uses the folder OA for authorization.
	fileNGACNodeID := parentNGACID

	fileID := uuid.New().String()

	// Get presigned upload URL from Document Storage
	// The URL takes exactly the declared size, so what lands in storage is what
	// the quota was checked against.
	uploadResp, err := s.docStorage.GetUploadURL(ctx, &docpb.GetUploadURLRequest{
		WorkspaceId: req.WorkspaceId, Filename: req.Name,
		MimeType: req.MimeType, DocId: fileID, SizeBytes: &declared,
	})
	if err != nil {
		return nil, fmt.Errorf("get upload url: %w", err)
	}

	mimeType := req.MimeType
	sizeBytes := req.SizeBytes
	item := &store.DriveItem{
		ID:             fileID,
		WorkspaceID:    req.WorkspaceId,
		DriveContext:   driveCtx,
		DriveContextID: driveCtxID,
		ParentID:       store.NilIfEmpty(req.ParentId),
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
		return nil, fmt.Errorf("insert file: %w", err)
	}

	return &pb.CreateFileResponse{
		FileId:    fileID,
		UploadUrl: uploadResp.UploadUrl,
		ObjectKey: uploadResp.ObjectKey,
	}, nil
}

// ConfirmFile finalizes a file upload after the client has PUT to MinIO.
func (s *Service) ConfirmFile(ctx context.Context, req *pb.ConfirmFileRequest) (*pb.DriveItem, error) {
	item, err := s.store.GetItem(ctx, req.FileId)
	if err != nil || item == nil {
		return nil, notFound("file not found")
	}
	// Confirming publishes the upload and charges it against the workspace
	// quota, so it takes the same right as creating the file did.
	if err := s.checkAccess(ctx, grpcauth.CallerFrom(ctx).NGACNodeID, item.NGACNodeID, ngac.OpWrite); err != nil {
		return nil, err
	}
	if item.Status != "pending" {
		return nil, conflict("file not pending")
	}

	// Verify object in MinIO
	confirmResp, err := s.docStorage.ConfirmUpload(ctx, &docpb.ConfirmUploadRequest{
		WorkspaceId: item.WorkspaceID, ObjectKey: *item.ObjectKey,
	})
	if err != nil {
		// Only "the store has no such object" is a conflict the caller can act on;
		// anything else is the document service failing, and its text is for the log.
		if grpcutil.IsFailedPrecondition(err) {
			return nil, conflict("file not uploaded")
		}
		return nil, fmt.Errorf("confirm upload: %w", err)
	}

	// Publish the file and charge its quota in one step: a file that is active
	// but not counted would let a workspace outgrow its quota unseen. The quota
	// is held to the size actually stored, not the one declared at CreateFile: an
	// upload that does not fit is removed, row and bytes.
	actualSize := confirmResp.SizeBytes
	if err := s.store.ActivateFile(ctx, item.ID, item.WorkspaceID, actualSize); err != nil {
		if errors.Is(err, store.ErrNotPending) {
			return nil, conflict("file not pending")
		}
		if errors.Is(err, store.ErrQuotaExceeded) {
			s.discardUpload(ctx, item)
			return nil, quotaExceeded("storage quota exceeded")
		}
		return nil, fmt.Errorf("publish file: %w", err)
	}

	item.Status = "active"
	item.SizeBytes = &actualSize
	slog.Info("file confirmed", "id", item.ID, "name", item.Name, "size", actualSize)
	s.announce(ctx, realtime.KindCreated, item)
	return itemToProto(item), nil
}

// discardUpload removes a pending upload that will not be published, its row
// (only while still pending) and its stored object, on a context the caller
// cannot cancel. Failures are logged; the stale-upload sweep is the backstop.
func (s *Service) discardUpload(callCtx context.Context, item *store.DriveItem) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(callCtx), cleanupTimeout)
	defer cancel()
	if _, err := s.store.DeletePendingItem(ctx, item.ID); err != nil {
		slog.Error("refused upload's row not removed", "item", item.ID, "err", err)
	}
	if item.ObjectKey == nil {
		return
	}
	if _, err := s.docStorage.DeleteObject(ctx, &docpb.DeleteObjectRequest{
		WorkspaceId: item.WorkspaceID, ObjectKey: *item.ObjectKey,
	}); err != nil {
		slog.Error("refused upload's stored object not removed", "item", item.ID, "key", *item.ObjectKey, "err", err)
	}
}

// GetDownloadURL returns a presigned download URL after NGAC read check. Only
// an active file has one: a trashed file is reached again only by restoring it,
// a pending upload is not published yet, and a folder has no object.
func (s *Service) GetDownloadURL(ctx context.Context, req *pb.GetDownloadURLRequest) (*pb.GetDownloadURLResponse, error) {
	item, err := s.store.GetItem(ctx, req.FileId)
	if err != nil || item == nil || item.Status != "active" || item.ItemType != "file" || item.ObjectKey == nil {
		return nil, notFound("file not found")
	}
	if err := s.checkAccess(ctx, grpcauth.CallerFrom(ctx).NGACNodeID, item.NGACNodeID, ngac.OpRead); err != nil {
		return nil, err
	}

	dlResp, err := s.docStorage.GetDownloadURL(ctx, &docpb.GetDownloadURLRequest{
		WorkspaceId: item.WorkspaceID, ObjectKey: *item.ObjectKey,
	})
	if err != nil {
		return nil, fmt.Errorf("get download url: %w", err)
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
