// Package grpc is the drive service's gRPC transport: it adapts the DriveService
// API to the domain service and turns the domain's refusals into gRPC statuses.
// It holds no business logic.
package grpc

import (
	"context"

	"google.golang.org/grpc/codes"

	"ngac-platform/pkg/grpcutil"
	"ngac-platform/pkg/realtime"
	pb "ngac-platform/proto/drive"
	"ngac-platform/services/drive/internal/domain"
)

// DriveServer implements the DriveService gRPC API over the domain service.
type DriveServer struct {
	pb.UnimplementedDriveServiceServer
	svc *domain.Service
}

// NewServer wraps an existing domain service.
func NewServer(svc *domain.Service) *DriveServer { return &DriveServer{svc: svc} }

// SetEmitter installs the sink for realtime change events. Without one the
// server runs silently.
func (s *DriveServer) SetEmitter(e realtime.Emitter) { s.svc.SetEmitter(e) }

// mapError turns the domain's classified refusals into gRPC statuses; anything
// else is a generic Internal whose detail stays in the log (see grpcutil.Status).
func mapError(err error) error {
	return grpcutil.Status(err,
		grpcutil.Mapping{Is: domain.ErrConflict, Code: codes.FailedPrecondition},
		grpcutil.Mapping{Is: domain.ErrQuotaExceeded, Code: codes.ResourceExhausted},
		grpcutil.Mapping{Is: domain.ErrAborted, Code: codes.Aborted},
		grpcutil.Mapping{Is: domain.ErrUnavailable, Code: codes.Unavailable},
	)
}

func (s *DriveServer) CreateFolder(ctx context.Context, req *pb.CreateFolderRequest) (*pb.DriveItem, error) {
	resp, err := s.svc.CreateFolder(ctx, req)
	return resp, mapError(err)
}

func (s *DriveServer) ListFolder(ctx context.Context, req *pb.ListFolderRequest) (*pb.DriveItemList, error) {
	resp, err := s.svc.ListFolder(ctx, req)
	return resp, mapError(err)
}

func (s *DriveServer) GetItem(ctx context.Context, req *pb.GetItemRequest) (*pb.DriveItem, error) {
	resp, err := s.svc.GetItem(ctx, req)
	return resp, mapError(err)
}

func (s *DriveServer) CreateFile(ctx context.Context, req *pb.CreateFileRequest) (*pb.CreateFileResponse, error) {
	resp, err := s.svc.CreateFile(ctx, req)
	return resp, mapError(err)
}

func (s *DriveServer) ConfirmFile(ctx context.Context, req *pb.ConfirmFileRequest) (*pb.DriveItem, error) {
	resp, err := s.svc.ConfirmFile(ctx, req)
	return resp, mapError(err)
}

func (s *DriveServer) GetDownloadURL(ctx context.Context, req *pb.GetDownloadURLRequest) (*pb.GetDownloadURLResponse, error) {
	resp, err := s.svc.GetDownloadURL(ctx, req)
	return resp, mapError(err)
}

func (s *DriveServer) RenameItem(ctx context.Context, req *pb.RenameItemRequest) (*pb.DriveItem, error) {
	resp, err := s.svc.RenameItem(ctx, req)
	return resp, mapError(err)
}

func (s *DriveServer) MoveItem(ctx context.Context, req *pb.MoveItemRequest) (*pb.DriveItem, error) {
	resp, err := s.svc.MoveItem(ctx, req)
	return resp, mapError(err)
}

func (s *DriveServer) CopyItem(ctx context.Context, req *pb.CopyItemRequest) (*pb.DriveItem, error) {
	resp, err := s.svc.CopyItem(ctx, req)
	return resp, mapError(err)
}

func (s *DriveServer) TrashItem(ctx context.Context, req *pb.TrashItemRequest) (*pb.Empty, error) {
	resp, err := s.svc.TrashItem(ctx, req)
	return resp, mapError(err)
}

func (s *DriveServer) RestoreItem(ctx context.Context, req *pb.RestoreItemRequest) (*pb.DriveItem, error) {
	resp, err := s.svc.RestoreItem(ctx, req)
	return resp, mapError(err)
}

func (s *DriveServer) DeleteItem(ctx context.Context, req *pb.DeleteItemRequest) (*pb.Empty, error) {
	resp, err := s.svc.DeleteItem(ctx, req)
	return resp, mapError(err)
}

func (s *DriveServer) CreateShare(ctx context.Context, req *pb.CreateShareRequest) (*pb.ShareInfo, error) {
	resp, err := s.svc.CreateShare(ctx, req)
	return resp, mapError(err)
}

func (s *DriveServer) RevokeShare(ctx context.Context, req *pb.RevokeShareRequest) (*pb.Empty, error) {
	resp, err := s.svc.RevokeShare(ctx, req)
	return resp, mapError(err)
}

func (s *DriveServer) ListShares(ctx context.Context, req *pb.ListSharesRequest) (*pb.ShareList, error) {
	resp, err := s.svc.ListShares(ctx, req)
	return resp, mapError(err)
}

func (s *DriveServer) GetSharedWithMe(ctx context.Context, req *pb.GetSharedWithMeRequest) (*pb.DriveItemList, error) {
	resp, err := s.svc.GetSharedWithMe(ctx, req)
	return resp, mapError(err)
}

func (s *DriveServer) CreateDriveForChannel(ctx context.Context, req *pb.CreateDriveForChannelRequest) (*pb.DriveItem, error) {
	resp, err := s.svc.CreateDriveForChannel(ctx, req)
	return resp, mapError(err)
}

func (s *DriveServer) GetChannelDrive(ctx context.Context, req *pb.GetChannelDriveRequest) (*pb.DriveItem, error) {
	resp, err := s.svc.GetChannelDrive(ctx, req)
	return resp, mapError(err)
}

func (s *DriveServer) GetQuota(ctx context.Context, req *pb.GetQuotaRequest) (*pb.Quota, error) {
	resp, err := s.svc.GetQuota(ctx, req)
	return resp, mapError(err)
}

func (s *DriveServer) UpdateQuota(ctx context.Context, req *pb.UpdateQuotaRequest) (*pb.Quota, error) {
	resp, err := s.svc.UpdateQuota(ctx, req)
	return resp, mapError(err)
}
