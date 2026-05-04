// Package grpc provides the gRPC handler for the workspace service.
// Each method is thin: validate → delegate to domain → map error → respond.
package grpc

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"ngac-platform/services/workspace/internal/domain"
	pb "ngac-platform/proto/workspace"
)

// WorkspaceDomainService defines operations the gRPC handler delegates to.
type WorkspaceDomainService interface {
	CreateWorkspace(ctx context.Context, in domain.CreateWorkspaceInput) (*domain.WorkspaceResult, error)
	GetWorkspace(ctx context.Context, id string) (*domain.WorkspaceResult, error)
	ListAccessibleWorkspaces(ctx context.Context, userNGACNodeID string) ([]*domain.WorkspaceResult, error)
	InviteMember(ctx context.Context, wsID, targetNGACNodeID string) error
	RemoveMember(ctx context.Context, wsID, targetNGACNodeID string) error
	ListMembers(ctx context.Context, wsID string) ([]*domain.Member, error)
	UpdateMemberRoles(ctx context.Context, wsID, targetNGACNodeID string, roleIDs []string) error
	TransferOwnership(ctx context.Context, wsID, newOwnerNGACNodeID string) error
	RemoveOwner(ctx context.Context, wsID, targetNGACNodeID string) error
	CreateRole(ctx context.Context, wsID, roleName string) (*domain.Role, error)
	ListRoles(ctx context.Context, wsID string) ([]*domain.Role, error)
	DeleteRole(ctx context.Context, roleID string) error
	CreateFolder(ctx context.Context, wsID, name, parentOaID string) (*domain.Folder, error)
	ListFolders(ctx context.Context, wsID string) ([]*domain.Folder, error)
	DeleteFolder(ctx context.Context, folderID string) error
	CreatePermission(ctx context.Context, uaID, oaID string, ops []string) (*domain.Permission, error)
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
		Name: req.Name, UserID: req.UserId, UserNGACNodeID: req.UserNgacNodeId,
	})
	if err != nil {
		return nil, mapError(err)
	}
	return workspaceToProto(res), nil
}

// ListWorkspaces returns workspaces accessible to the calling user.
func (s *WorkspaceServer) ListWorkspaces(ctx context.Context, req *pb.ListWorkspacesRequest) (*pb.WorkspaceList, error) {
	results, err := s.svc.ListAccessibleWorkspaces(ctx, req.UserNgacNodeId)
	if err != nil {
		return nil, mapError(err)
	}
	var ws []*pb.Workspace
	for _, r := range results {
		ws = append(ws, workspaceToProto(r))
	}
	return &pb.WorkspaceList{Workspaces: ws}, nil
}

// GetWorkspace retrieves a single workspace by ID.
func (s *WorkspaceServer) GetWorkspace(ctx context.Context, req *pb.GetWorkspaceRequest) (*pb.Workspace, error) {
	res, err := s.svc.GetWorkspace(ctx, req.WorkspaceId)
	if err != nil {
		return nil, mapError(err)
	}
	return workspaceToProto(res), nil
}

// InviteMember adds a user to a workspace.
func (s *WorkspaceServer) InviteMember(ctx context.Context, req *pb.InviteMemberRequest) (*pb.Empty, error) {
	if err := s.svc.InviteMember(ctx, req.WorkspaceId, req.TargetNgacNodeId); err != nil {
		return nil, mapError(err)
	}
	return &pb.Empty{}, nil
}

// RemoveMember removes a user from a workspace.
func (s *WorkspaceServer) RemoveMember(ctx context.Context, req *pb.RemoveMemberRequest) (*pb.Empty, error) {
	if err := s.svc.RemoveMember(ctx, req.WorkspaceId, req.TargetNgacNodeId); err != nil {
		return nil, mapError(err)
	}
	return &pb.Empty{}, nil
}

// ListMembers returns all members of a workspace.
func (s *WorkspaceServer) ListMembers(ctx context.Context, req *pb.ListMembersRequest) (*pb.MemberList, error) {
	members, err := s.svc.ListMembers(ctx, req.WorkspaceId)
	if err != nil {
		return nil, mapError(err)
	}
	var pbMembers []*pb.Member
	for _, m := range members {
		pbMembers = append(pbMembers, &pb.Member{NgacNodeId: m.NGACNodeID, Username: m.Username})
	}
	return &pb.MemberList{Members: pbMembers}, nil
}

// UpdateMemberRoles reassigns a user's roles in a workspace.
func (s *WorkspaceServer) UpdateMemberRoles(ctx context.Context, req *pb.UpdateMemberRolesRequest) (*pb.Empty, error) {
	if err := s.svc.UpdateMemberRoles(ctx, req.WorkspaceId, req.TargetNgacNodeId, req.RoleIds); err != nil {
		return nil, mapError(err)
	}
	return &pb.Empty{}, nil
}

// TransferOwnership adds a new owner to the workspace.
func (s *WorkspaceServer) TransferOwnership(ctx context.Context, req *pb.TransferOwnershipRequest) (*pb.Empty, error) {
	if err := s.svc.TransferOwnership(ctx, req.WorkspaceId, req.NewOwnerNgacNodeId); err != nil {
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
	if err := s.svc.RemoveOwner(ctx, req.WorkspaceId, req.TargetNgacNodeId); err != nil {
		return nil, mapError(err)
	}
	return &pb.Empty{}, nil
}

// CreateRole provisions a new role in a workspace.
func (s *WorkspaceServer) CreateRole(ctx context.Context, req *pb.CreateRoleRequest) (*pb.Role, error) {
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name required")
	}
	role, err := s.svc.CreateRole(ctx, req.WorkspaceId, req.Name)
	if err != nil {
		return nil, mapError(err)
	}
	return &pb.Role{Id: role.ID, Name: role.Name, NgacNodeId: role.NGACNodeID}, nil
}

// ListRoles returns all roles in a workspace.
func (s *WorkspaceServer) ListRoles(ctx context.Context, req *pb.ListRolesRequest) (*pb.RoleList, error) {
	roles, err := s.svc.ListRoles(ctx, req.WorkspaceId)
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
	if err := s.svc.DeleteRole(ctx, req.RoleId); err != nil {
		return nil, mapError(err)
	}
	return &pb.Empty{}, nil
}

// CreateFolder provisions a new folder in a workspace.
func (s *WorkspaceServer) CreateFolder(ctx context.Context, req *pb.CreateFolderRequest) (*pb.Folder, error) {
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name required")
	}
	f, err := s.svc.CreateFolder(ctx, req.WorkspaceId, req.Name, req.ParentOaId)
	if err != nil {
		return nil, mapError(err)
	}
	return &pb.Folder{Id: f.ID, Name: f.Name, NgacNodeId: f.NGACNodeID}, nil
}

// ListFolders returns all folders in a workspace.
func (s *WorkspaceServer) ListFolders(ctx context.Context, req *pb.ListFoldersRequest) (*pb.FolderList, error) {
	folders, err := s.svc.ListFolders(ctx, req.WorkspaceId)
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
	if err := s.svc.DeleteFolder(ctx, req.FolderId); err != nil {
		return nil, mapError(err)
	}
	return &pb.Empty{}, nil
}

// CreatePermission creates an association (permission) between a UA and OA.
func (s *WorkspaceServer) CreatePermission(ctx context.Context, req *pb.CreatePermissionRequest) (*pb.Permission, error) {
	p, err := s.svc.CreatePermission(ctx, req.UaId, req.OaId, req.Operations)
	if err != nil {
		return nil, mapError(err)
	}
	return &pb.Permission{Id: p.ID, UaId: p.UaID, OaId: p.OaID, Operations: p.Operations}, nil
}

// ListPermissions is a placeholder (not yet implemented).
func (s *WorkspaceServer) ListPermissions(ctx context.Context, req *pb.ListPermissionsRequest) (*pb.PermissionList, error) {
	return &pb.PermissionList{}, nil
}

// DeletePermission is a placeholder (not yet implemented).
func (s *WorkspaceServer) DeletePermission(ctx context.Context, req *pb.DeletePermissionRequest) (*pb.Empty, error) {
	return &pb.Empty{}, nil
}

// workspaceToProto converts a domain result to proto.
func workspaceToProto(r *domain.WorkspaceResult) *pb.Workspace {
	return &pb.Workspace{
		Id: r.ID, Name: r.Name, PcNodeId: r.PcNodeID,
		OwnersUaId: r.OwnersUaID, MembersUaId: r.MembersUaID,
		MgmtOaId: r.MgmtOaID, DocumentsOaId: r.DocumentsOaID,
		ChannelsOaId: r.ChannelsOaID, CreatedBy: r.CreatedBy,
	}
}

// mapError translates domain sentinel errors to gRPC status codes.
func mapError(err error) error {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, domain.ErrAccessDenied):
		return status.Error(codes.PermissionDenied, err.Error())
	case errors.Is(err, domain.ErrAlreadyExists):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, domain.ErrInvalidInput):
		return status.Error(codes.InvalidArgument, err.Error())
	default:
		return status.Errorf(codes.Internal, "internal: %v", err)
	}
}
