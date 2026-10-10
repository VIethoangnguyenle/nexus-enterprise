// Package grpc provides the gRPC handler for the workspace service.
// Each method is thin: validate → delegate to domain → map error → respond.
package grpc

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"ngac-platform/pkg/grpcauth"
	"ngac-platform/pkg/grpcutil"
	pb "ngac-platform/proto/workspace"
	"ngac-platform/services/workspace/internal/domain"
	"ngac-platform/services/workspace/internal/wire"
)

// WorkspaceDomainService defines operations the gRPC handler delegates to.
//
// Every operation other than workspace creation and listing takes the caller's
// NGAC user node ID and is authorized in the domain. The caller comes from
// grpcauth.CallerFrom(ctx) — request metadata on the wire, verified JWT claims
// in-process — never from a field of the request body. An empty caller is
// denied.
type WorkspaceDomainService interface {
	CreateWorkspace(ctx context.Context, in domain.CreateWorkspaceInput) (*domain.WorkspaceResult, error)
	ViewWorkspace(ctx context.Context, callerNodeID, id string) (*domain.WorkspaceResult, error)
	ListAccessibleWorkspaces(ctx context.Context, userNGACNodeID string) ([]*domain.WorkspaceResult, error)
	RemoveMember(ctx context.Context, callerNodeID, wsID, targetNGACNodeID string) error
	ListMembers(ctx context.Context, callerNodeID, wsID string) ([]*domain.Member, error)
	TransferOwnership(ctx context.Context, callerNodeID, wsID, newOwnerNGACNodeID string) error
	RemoveOwner(ctx context.Context, callerNodeID, wsID, targetNGACNodeID string) error
	CreateRole(ctx context.Context, callerNodeID, wsID, roleName string) (*domain.Role, error)
	ListRoles(ctx context.Context, callerNodeID, wsID string) ([]*domain.Role, error)
	DeleteRole(ctx context.Context, callerNodeID, wsID, roleID string) error
	CreateFolder(ctx context.Context, callerNodeID, wsID, name, parentOaID string) (*domain.Folder, error)
	ListFolders(ctx context.Context, callerNodeID, wsID string) ([]*domain.Folder, error)
	DeleteFolder(ctx context.Context, callerNodeID, wsID, folderID string) error
}

// WorkspaceServer implements the workspace gRPC service.
type WorkspaceServer struct {
	pb.UnimplementedWorkspaceServiceServer
	svc WorkspaceDomainService
}

// NewWorkspaceServer creates a workspace gRPC handler.
func NewWorkspaceServer(svc WorkspaceDomainService) *WorkspaceServer {
	return &WorkspaceServer{svc: svc}
}

// CreateWorkspace handles workspace creation.
func (s *WorkspaceServer) CreateWorkspace(ctx context.Context, req *pb.CreateWorkspaceRequest) (*pb.Workspace, error) {
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name required")
	}
	res, err := s.svc.CreateWorkspace(ctx, domain.CreateWorkspaceInput{
		Name: req.Name, UserID: grpcauth.CallerFrom(ctx).UserID, UserNGACNodeID: grpcauth.CallerFrom(ctx).NGACNodeID,
	})
	if err != nil {
		return nil, mapError(err)
	}
	return wire.Workspace(res), nil
}

// ListWorkspaces returns workspaces accessible to the calling user.
func (s *WorkspaceServer) ListWorkspaces(ctx context.Context, req *pb.ListWorkspacesRequest) (*pb.WorkspaceList, error) {
	results, err := s.svc.ListAccessibleWorkspaces(ctx, grpcauth.CallerFrom(ctx).NGACNodeID)
	if err != nil {
		return nil, mapError(err)
	}
	return wire.Workspaces(results), nil
}

// GetWorkspace retrieves a single workspace by ID.
func (s *WorkspaceServer) GetWorkspace(ctx context.Context, req *pb.GetWorkspaceRequest) (*pb.Workspace, error) {
	res, err := s.svc.ViewWorkspace(ctx, grpcauth.CallerFrom(ctx).NGACNodeID, req.WorkspaceId)
	if err != nil {
		return nil, mapError(err)
	}
	return wire.Workspace(res), nil
}

// RemoveMember removes a user from a workspace.
func (s *WorkspaceServer) RemoveMember(ctx context.Context, req *pb.RemoveMemberRequest) (*pb.Empty, error) {
	if err := s.svc.RemoveMember(ctx, grpcauth.CallerFrom(ctx).NGACNodeID, req.WorkspaceId, req.TargetNgacNodeId); err != nil {
		return nil, mapError(err)
	}
	return &pb.Empty{}, nil
}

// ListMembers returns all members of a workspace.
func (s *WorkspaceServer) ListMembers(ctx context.Context, req *pb.ListMembersRequest) (*pb.MemberList, error) {
	members, err := s.svc.ListMembers(ctx, grpcauth.CallerFrom(ctx).NGACNodeID, req.WorkspaceId)
	if err != nil {
		return nil, mapError(err)
	}
	return wire.Members(members), nil
}

// TransferOwnership adds a new owner to the workspace.
func (s *WorkspaceServer) TransferOwnership(ctx context.Context, req *pb.TransferOwnershipRequest) (*pb.Empty, error) {
	if err := s.svc.TransferOwnership(ctx, grpcauth.CallerFrom(ctx).NGACNodeID, req.WorkspaceId, req.NewOwnerNgacNodeId); err != nil {
		return nil, mapError(err)
	}
	return &pb.Empty{}, nil
}

// AddOwner is an alias for TransferOwnership.
func (s *WorkspaceServer) AddOwner(ctx context.Context, req *pb.AddOwnerRequest) (*pb.Empty, error) {
	return s.TransferOwnership(ctx, &pb.TransferOwnershipRequest{
		WorkspaceId: req.WorkspaceId, NewOwnerNgacNodeId: req.TargetNgacNodeId,
	})
}

// RemoveOwner removes an owner from the workspace (fails if last owner).
func (s *WorkspaceServer) RemoveOwner(ctx context.Context, req *pb.RemoveOwnerRequest) (*pb.Empty, error) {
	if err := s.svc.RemoveOwner(ctx, grpcauth.CallerFrom(ctx).NGACNodeID, req.WorkspaceId, req.TargetNgacNodeId); err != nil {
		return nil, mapError(err)
	}
	return &pb.Empty{}, nil
}

// CreateRole provisions a new role in a workspace.
func (s *WorkspaceServer) CreateRole(ctx context.Context, req *pb.CreateRoleRequest) (*pb.Role, error) {
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name required")
	}
	role, err := s.svc.CreateRole(ctx, grpcauth.CallerFrom(ctx).NGACNodeID, req.WorkspaceId, req.Name)
	if err != nil {
		return nil, mapError(err)
	}
	return &pb.Role{Id: role.ID, Name: role.Name, NgacNodeId: role.NGACNodeID}, nil
}

// ListRoles returns all roles in a workspace.
func (s *WorkspaceServer) ListRoles(ctx context.Context, req *pb.ListRolesRequest) (*pb.RoleList, error) {
	roles, err := s.svc.ListRoles(ctx, grpcauth.CallerFrom(ctx).NGACNodeID, req.WorkspaceId)
	if err != nil {
		return nil, mapError(err)
	}
	var pbRoles []*pb.Role
	for _, r := range roles {
		pbRoles = append(pbRoles, &pb.Role{Id: r.ID, Name: r.Name, NgacNodeId: r.NGACNodeID})
	}
	return &pb.RoleList{Roles: pbRoles}, nil
}

// DeleteRole removes a role from the NGAC graph.
func (s *WorkspaceServer) DeleteRole(ctx context.Context, req *pb.DeleteRoleRequest) (*pb.Empty, error) {
	if err := s.svc.DeleteRole(ctx, grpcauth.CallerFrom(ctx).NGACNodeID, req.WorkspaceId, req.RoleId); err != nil {
		return nil, mapError(err)
	}
	return &pb.Empty{}, nil
}

// CreateFolder provisions a new folder in a workspace.
func (s *WorkspaceServer) CreateFolder(ctx context.Context, req *pb.CreateFolderRequest) (*pb.Folder, error) {
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name required")
	}
	f, err := s.svc.CreateFolder(ctx, grpcauth.CallerFrom(ctx).NGACNodeID, req.WorkspaceId, req.Name, req.ParentOaId)
	if err != nil {
		return nil, mapError(err)
	}
	return wire.Folder(f), nil
}

// ListFolders returns all folders in a workspace.
func (s *WorkspaceServer) ListFolders(ctx context.Context, req *pb.ListFoldersRequest) (*pb.FolderList, error) {
	folders, err := s.svc.ListFolders(ctx, grpcauth.CallerFrom(ctx).NGACNodeID, req.WorkspaceId)
	if err != nil {
		return nil, mapError(err)
	}
	var pbFolders []*pb.Folder
	for _, f := range folders {
		pbFolders = append(pbFolders, &pb.Folder{Id: f.ID, Name: f.Name, NgacNodeId: f.NGACNodeID})
	}
	return &pb.FolderList{Folders: pbFolders}, nil
}

// DeleteFolder removes a folder from the NGAC graph.
func (s *WorkspaceServer) DeleteFolder(ctx context.Context, req *pb.DeleteFolderRequest) (*pb.Empty, error) {
	if err := s.svc.DeleteFolder(ctx, grpcauth.CallerFrom(ctx).NGACNodeID, req.WorkspaceId, req.FolderId); err != nil {
		return nil, mapError(err)
	}
	return &pb.Empty{}, nil
}

// mapError translates a domain error to a gRPC status; anything that is not the
// domain's own refusal becomes a generic Internal (see grpcutil.Status).
func mapError(err error) error {
	return grpcutil.Status(err)
}
