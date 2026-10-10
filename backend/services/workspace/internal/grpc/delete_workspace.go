package grpc

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"ngac-platform/pkg/grpcauth"
	pb "ngac-platform/proto/workspace"
)

// workspaceDeleter is the domain operation behind DeleteWorkspace. It is a
// separate interface so the compensation RPC does not widen the one every other
// handler uses; the real domain service has it.
type workspaceDeleter interface {
	DeleteWorkspace(ctx context.Context, callerNodeID, callerUserID, wsID string) error
}

// DeleteWorkspace removes a workspace that was just created and should not
// stay. For compensation only: there is no REST route. Who is asking is the
// verified caller of the request, never a field of the body.
func (s *WorkspaceServer) DeleteWorkspace(ctx context.Context, req *pb.DeleteWorkspaceRequest) (*pb.Empty, error) {
	d, ok := s.svc.(workspaceDeleter)
	if !ok {
		return nil, status.Error(codes.Unimplemented, "workspace deletion is not available")
	}
	caller := grpcauth.CallerFrom(ctx)
	if err := d.DeleteWorkspace(ctx, caller.NGACNodeID, caller.UserID, req.GetWorkspaceId()); err != nil {
		return nil, mapError(err)
	}
	return &pb.Empty{}, nil
}
