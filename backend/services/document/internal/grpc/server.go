// Package grpc is the document service's gRPC transport for object storage. It
// adapts the DocumentStorage API to the storage service and holds no logic.
package grpc

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/types/known/timestamppb"

	"ngac-platform/pkg/grpcutil"
	pb "ngac-platform/proto/document"
	"ngac-platform/services/document/internal/storage"
)

// DocumentStorageServer implements the pure storage API: no NGAC awareness.
// Access control is handled by the Drive Service before calling these RPCs.
type DocumentStorageServer struct {
	pb.UnimplementedDocumentStorageServiceServer
	svc *storage.Service
}

// NewDocumentStorageServer wraps the storage service.
func NewDocumentStorageServer(svc *storage.Service) *DocumentStorageServer {
	return &DocumentStorageServer{svc: svc}
}

// mapError turns the storage service's refusals into gRPC statuses; anything
// else is a generic Internal whose detail stays in the log.
func mapError(err error) error {
	return grpcutil.Status(err, grpcutil.Mapping{Is: storage.ErrNotUploaded, Code: codes.FailedPrecondition})
}

// GetUploadURL generates a presigned PUT URL for direct-to-MinIO upload.
func (s *DocumentStorageServer) GetUploadURL(ctx context.Context, req *pb.GetUploadURLRequest) (*pb.GetUploadURLResponse, error) {
	u, key, err := s.svc.UploadURL(ctx, req.WorkspaceId, req.DocId, req.Filename)
	if err != nil {
		return nil, mapError(err)
	}
	return &pb.GetUploadURLResponse{UploadUrl: u, ObjectKey: key}, nil
}

// ConfirmUpload verifies the file exists in MinIO and returns its metadata.
func (s *DocumentStorageServer) ConfirmUpload(ctx context.Context, req *pb.ConfirmUploadRequest) (*pb.ConfirmUploadResponse, error) {
	size, ctype, err := s.svc.Confirm(ctx, req.WorkspaceId, req.ObjectKey)
	if err != nil {
		return nil, mapError(err)
	}
	return &pb.ConfirmUploadResponse{SizeBytes: size, ContentType: ctype}, nil
}

// GetDownloadURL generates a presigned GET URL for downloading a file.
func (s *DocumentStorageServer) GetDownloadURL(ctx context.Context, req *pb.GetDownloadURLRequest) (*pb.GetDownloadURLResponse, error) {
	u, err := s.svc.DownloadURL(ctx, req.WorkspaceId, req.ObjectKey)
	if err != nil {
		return nil, mapError(err)
	}
	return &pb.GetDownloadURLResponse{DownloadUrl: u}, nil
}

// DeleteObject removes an object from MinIO.
func (s *DocumentStorageServer) DeleteObject(ctx context.Context, req *pb.DeleteObjectRequest) (*pb.Empty, error) {
	if err := s.svc.Delete(ctx, req.WorkspaceId, req.ObjectKey); err != nil {
		return nil, mapError(err)
	}
	return &pb.Empty{}, nil
}

// CopyObject performs a server-side copy of an object in MinIO.
func (s *DocumentStorageServer) CopyObject(ctx context.Context, req *pb.CopyObjectRequest) (*pb.CopyObjectResponse, error) {
	size, err := s.svc.Copy(ctx, req.SrcWorkspaceId, req.SrcObjectKey, req.DstWorkspaceId, req.DstObjectKey)
	if err != nil {
		return nil, mapError(err)
	}
	return &pb.CopyObjectResponse{ObjectKey: req.DstObjectKey, SizeBytes: size}, nil
}

// GetObjectInfo returns metadata about an object in MinIO.
func (s *DocumentStorageServer) GetObjectInfo(ctx context.Context, req *pb.GetObjectInfoRequest) (*pb.ObjectInfo, error) {
	info, err := s.svc.Info(ctx, req.WorkspaceId, req.ObjectKey)
	if err != nil {
		return nil, mapError(err)
	}
	return &pb.ObjectInfo{
		ObjectKey:    req.ObjectKey,
		SizeBytes:    info.Size,
		ContentType:  info.ContentType,
		LastModified: timestamppb.New(info.LastModified),
	}, nil
}
