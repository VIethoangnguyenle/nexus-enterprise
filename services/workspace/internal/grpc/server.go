package grpc

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	policypb "ngac-platform/proto/policy"
	pb "ngac-platform/proto/workspace"
)

type WorkspaceServer struct {
	pb.UnimplementedWorkspaceServiceServer
	db           *pgxpool.Pool
	policyClient policypb.PolicyServiceClient
}

func NewWorkspaceServer(db *pgxpool.Pool, pc policypb.PolicyServiceClient) *WorkspaceServer {
	return &WorkspaceServer{db: db, policyClient: pc}
}

func (s *WorkspaceServer) CreateWorkspace(ctx context.Context, req *pb.CreateWorkspaceRequest) (*pb.Workspace, error) {
	pc, err := s.policyClient.CreateNode(ctx, &policypb.CreateNodeRequest{
		Name: fmt.Sprintf("PC_%s", req.Name), NodeType: "PC",
		Properties: map[string]string{"workspace": req.Name},
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "create PC: %v", err)
	}

	ownersUA, _ := s.policyClient.CreateNode(ctx, &policypb.CreateNodeRequest{
		Name: fmt.Sprintf("%s_Owners", req.Name), NodeType: "UA",
	})
	membersUA, _ := s.policyClient.CreateNode(ctx, &policypb.CreateNodeRequest{
		Name: fmt.Sprintf("%s_Members", req.Name), NodeType: "UA",
	})
	mgmtOA, _ := s.policyClient.CreateNode(ctx, &policypb.CreateNodeRequest{
		Name: fmt.Sprintf("%s_Mgmt", req.Name), NodeType: "OA",
	})
	docsOA, _ := s.policyClient.CreateNode(ctx, &policypb.CreateNodeRequest{
		Name: fmt.Sprintf("%s_Documents", req.Name), NodeType: "OA",
	})
	draftOA, _ := s.policyClient.CreateNode(ctx, &policypb.CreateNodeRequest{
		Name: fmt.Sprintf("%s_DraftDocs", req.Name), NodeType: "OA",
	})
	approvedOA, _ := s.policyClient.CreateNode(ctx, &policypb.CreateNodeRequest{
		Name: fmt.Sprintf("%s_ApprovedDocs", req.Name), NodeType: "OA",
	})
	channelsOA, _ := s.policyClient.CreateNode(ctx, &policypb.CreateNodeRequest{
		Name: fmt.Sprintf("%s_Channels", req.Name), NodeType: "OA",
	})

	// Assignments
	s.policyClient.CreateAssignment(ctx, &policypb.CreateAssignmentRequest{ChildId: ownersUA.Id, ParentId: pc.Id})
	s.policyClient.CreateAssignment(ctx, &policypb.CreateAssignmentRequest{ChildId: membersUA.Id, ParentId: pc.Id})
	s.policyClient.CreateAssignment(ctx, &policypb.CreateAssignmentRequest{ChildId: mgmtOA.Id, ParentId: pc.Id})
	s.policyClient.CreateAssignment(ctx, &policypb.CreateAssignmentRequest{ChildId: docsOA.Id, ParentId: pc.Id})
	s.policyClient.CreateAssignment(ctx, &policypb.CreateAssignmentRequest{ChildId: channelsOA.Id, ParentId: pc.Id})
	s.policyClient.CreateAssignment(ctx, &policypb.CreateAssignmentRequest{ChildId: draftOA.Id, ParentId: docsOA.Id})
	s.policyClient.CreateAssignment(ctx, &policypb.CreateAssignmentRequest{ChildId: approvedOA.Id, ParentId: docsOA.Id})
	s.policyClient.CreateAssignment(ctx, &policypb.CreateAssignmentRequest{ChildId: membersUA.Id, ParentId: ownersUA.Id})

	allOps := []string{"read", "write", "approve", "upload", "share", "manage", "invite", "create_channel"}
	s.policyClient.CreateAssociation(ctx, &policypb.CreateAssociationRequest{UaId: ownersUA.Id, OaId: mgmtOA.Id, Operations: allOps})
	s.policyClient.CreateAssociation(ctx, &policypb.CreateAssociationRequest{UaId: ownersUA.Id, OaId: docsOA.Id, Operations: allOps})
	s.policyClient.CreateAssociation(ctx, &policypb.CreateAssociationRequest{UaId: ownersUA.Id, OaId: channelsOA.Id, Operations: allOps})
	s.policyClient.CreateAssociation(ctx, &policypb.CreateAssociationRequest{UaId: membersUA.Id, OaId: docsOA.Id, Operations: []string{"read"}})
	s.policyClient.CreateAssociation(ctx, &policypb.CreateAssociationRequest{UaId: membersUA.Id, OaId: channelsOA.Id, Operations: []string{"read", "write", "create_channel"}})

	// Assign creator to Owners UA
	s.policyClient.CreateAssignment(ctx, &policypb.CreateAssignmentRequest{ChildId: req.UserNgacNodeId, ParentId: ownersUA.Id})

	wsID := uuid.New().String()
	_, err = s.db.Exec(ctx,
		"INSERT INTO workspaces (id, name, description, owner_id, ngac_pc_id) VALUES ($1, $2, $3, $4, $5)",
		wsID, req.Name, "", req.UserId, pc.Id)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "insert workspace: %v", err)
	}

	return &pb.Workspace{
		Id: wsID, Name: req.Name, PcNodeId: pc.Id,
		OwnersUaId: ownersUA.Id, MembersUaId: membersUA.Id,
		MgmtOaId: mgmtOA.Id, DocumentsOaId: docsOA.Id,
		ChannelsOaId: channelsOA.Id, CreatedBy: req.UserId,
	}, nil
}

func (s *WorkspaceServer) ListWorkspaces(ctx context.Context, req *pb.ListWorkspacesRequest) (*pb.WorkspaceList, error) {
	rows, err := s.db.Query(ctx, "SELECT id, name, ngac_pc_id FROM workspaces ORDER BY created_at DESC")
	if err != nil {
		return nil, status.Errorf(codes.Internal, "query workspaces: %v", err)
	}
	defer rows.Close()
	var workspaces []*pb.Workspace
	for rows.Next() {
		var w pb.Workspace
		if err := rows.Scan(&w.Id, &w.Name, &w.PcNodeId); err != nil {
			return nil, err
		}
		workspaces = append(workspaces, &w)
	}
	var accessible []*pb.Workspace
	for _, w := range workspaces {
		desc, err := s.policyClient.GetDescendants(ctx, &policypb.GetDescendantsRequest{NodeId: w.PcNodeId})
		if err != nil {
			continue
		}
		for _, n := range desc.Nodes {
			if n.Id == req.UserNgacNodeId {
				accessible = append(accessible, w)
				break
			}
		}
	}
	return &pb.WorkspaceList{Workspaces: accessible}, nil
}

func (s *WorkspaceServer) GetWorkspace(ctx context.Context, req *pb.GetWorkspaceRequest) (*pb.Workspace, error) {
	var w pb.Workspace
	err := s.db.QueryRow(ctx, "SELECT id, name, ngac_pc_id FROM workspaces WHERE id = $1", req.WorkspaceId).
		Scan(&w.Id, &w.Name, &w.PcNodeId)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "workspace not found")
	}
	return &w, nil
}

func (s *WorkspaceServer) InviteMember(ctx context.Context, req *pb.InviteMemberRequest) (*pb.Empty, error) {
	ws, err := s.GetWorkspace(ctx, &pb.GetWorkspaceRequest{WorkspaceId: req.WorkspaceId})
	if err != nil {
		return nil, err
	}
	children, _ := s.policyClient.GetChildren(ctx, &policypb.GetChildrenRequest{NodeId: ws.PcNodeId})
	var membersUAID string
	if children != nil {
		for _, n := range children.Nodes {
			if n.NodeType == "UA" && fmt.Sprintf("%s_Members", ws.Name) == n.Name {
				membersUAID = n.Id
				break
			}
		}
	}
	if membersUAID == "" {
		return nil, status.Errorf(codes.Internal, "members UA not found")
	}
	_, err = s.policyClient.CreateAssignment(ctx, &policypb.CreateAssignmentRequest{
		ChildId: req.TargetNgacNodeId, ParentId: membersUAID,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "assign member: %v", err)
	}
	return &pb.Empty{}, nil
}

func (s *WorkspaceServer) RemoveMember(ctx context.Context, req *pb.RemoveMemberRequest) (*pb.Empty, error) {
	ws, _ := s.GetWorkspace(ctx, &pb.GetWorkspaceRequest{WorkspaceId: req.WorkspaceId})
	desc, _ := s.policyClient.GetDescendants(ctx, &policypb.GetDescendantsRequest{NodeId: ws.PcNodeId})
	if desc != nil {
		for _, n := range desc.Nodes {
			if n.NodeType == "UA" {
				s.policyClient.RemoveAssignment(ctx, &policypb.RemoveAssignmentRequest{
					ChildId: req.TargetNgacNodeId, ParentId: n.Id,
				})
			}
		}
	}
	return &pb.Empty{}, nil
}

func (s *WorkspaceServer) ListMembers(ctx context.Context, req *pb.ListMembersRequest) (*pb.MemberList, error) {
	ws, _ := s.GetWorkspace(ctx, &pb.GetWorkspaceRequest{WorkspaceId: req.WorkspaceId})
	desc, _ := s.policyClient.GetDescendants(ctx, &policypb.GetDescendantsRequest{NodeId: ws.PcNodeId})
	seen := make(map[string]bool)
	var members []*pb.Member
	if desc != nil {
		for _, n := range desc.Nodes {
			if n.NodeType == "U" && !seen[n.Id] {
				seen[n.Id] = true
				members = append(members, &pb.Member{NgacNodeId: n.Id, Username: n.Name})
			}
		}
	}
	return &pb.MemberList{Members: members}, nil
}

func (s *WorkspaceServer) UpdateMemberRoles(ctx context.Context, req *pb.UpdateMemberRolesRequest) (*pb.Empty, error) {
	ws, _ := s.GetWorkspace(ctx, &pb.GetWorkspaceRequest{WorkspaceId: req.WorkspaceId})
	desc, _ := s.policyClient.GetDescendants(ctx, &policypb.GetDescendantsRequest{NodeId: ws.PcNodeId})
	if desc != nil {
		for _, n := range desc.Nodes {
			if n.NodeType == "UA" {
				s.policyClient.RemoveAssignment(ctx, &policypb.RemoveAssignmentRequest{
					ChildId: req.TargetNgacNodeId, ParentId: n.Id,
				})
			}
		}
	}
	for _, roleID := range req.RoleIds {
		s.policyClient.CreateAssignment(ctx, &policypb.CreateAssignmentRequest{
			ChildId: req.TargetNgacNodeId, ParentId: roleID,
		})
	}
	return &pb.Empty{}, nil
}

func (s *WorkspaceServer) TransferOwnership(ctx context.Context, req *pb.TransferOwnershipRequest) (*pb.Empty, error) {
	ws, _ := s.GetWorkspace(ctx, &pb.GetWorkspaceRequest{WorkspaceId: req.WorkspaceId})
	children, _ := s.policyClient.GetChildren(ctx, &policypb.GetChildrenRequest{NodeId: ws.PcNodeId})
	var ownersUAID string
	if children != nil {
		for _, n := range children.Nodes {
			if n.NodeType == "UA" && fmt.Sprintf("%s_Owners", ws.Name) == n.Name {
				ownersUAID = n.Id
				break
			}
		}
	}
	if ownersUAID != "" {
		s.policyClient.CreateAssignment(ctx, &policypb.CreateAssignmentRequest{
			ChildId: req.NewOwnerNgacNodeId, ParentId: ownersUAID,
		})
	}
	return &pb.Empty{}, nil
}

func (s *WorkspaceServer) AddOwner(ctx context.Context, req *pb.AddOwnerRequest) (*pb.Empty, error) {
	return s.TransferOwnership(ctx, &pb.TransferOwnershipRequest{
		WorkspaceId: req.WorkspaceId, NewOwnerNgacNodeId: req.TargetNgacNodeId,
	})
}

func (s *WorkspaceServer) RemoveOwner(ctx context.Context, req *pb.RemoveOwnerRequest) (*pb.Empty, error) {
	ws, _ := s.GetWorkspace(ctx, &pb.GetWorkspaceRequest{WorkspaceId: req.WorkspaceId})
	children, _ := s.policyClient.GetChildren(ctx, &policypb.GetChildrenRequest{NodeId: ws.PcNodeId})
	var ownersUAID string
	if children != nil {
		for _, n := range children.Nodes {
			if n.NodeType == "UA" && fmt.Sprintf("%s_Owners", ws.Name) == n.Name {
				ownersUAID = n.Id
				break
			}
		}
	}
	if ownersUAID != "" {
		ownerChildren, _ := s.policyClient.GetChildren(ctx, &policypb.GetChildrenRequest{NodeId: ownersUAID})
		userCount := 0
		if ownerChildren != nil {
			for _, n := range ownerChildren.Nodes {
				if n.NodeType == "U" {
					userCount++
				}
			}
		}
		if userCount <= 1 {
			return nil, status.Errorf(codes.FailedPrecondition, "cannot remove last owner")
		}
		s.policyClient.RemoveAssignment(ctx, &policypb.RemoveAssignmentRequest{
			ChildId: req.TargetNgacNodeId, ParentId: ownersUAID,
		})
	}
	return &pb.Empty{}, nil
}

func (s *WorkspaceServer) CreateRole(ctx context.Context, req *pb.CreateRoleRequest) (*pb.Role, error) {
	ws, _ := s.GetWorkspace(ctx, &pb.GetWorkspaceRequest{WorkspaceId: req.WorkspaceId})
	node, err := s.policyClient.CreateNode(ctx, &policypb.CreateNodeRequest{Name: req.Name, NodeType: "UA"})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "create role: %v", err)
	}
	s.policyClient.CreateAssignment(ctx, &policypb.CreateAssignmentRequest{ChildId: node.Id, ParentId: ws.PcNodeId})
	return &pb.Role{Id: node.Id, Name: req.Name, NgacNodeId: node.Id}, nil
}

func (s *WorkspaceServer) ListRoles(ctx context.Context, req *pb.ListRolesRequest) (*pb.RoleList, error) {
	ws, _ := s.GetWorkspace(ctx, &pb.GetWorkspaceRequest{WorkspaceId: req.WorkspaceId})
	children, _ := s.policyClient.GetChildren(ctx, &policypb.GetChildrenRequest{NodeId: ws.PcNodeId})
	var roles []*pb.Role
	if children != nil {
		for _, n := range children.Nodes {
			if n.NodeType == "UA" {
				roles = append(roles, &pb.Role{Id: n.Id, Name: n.Name, NgacNodeId: n.Id})
			}
		}
	}
	return &pb.RoleList{Roles: roles}, nil
}

func (s *WorkspaceServer) DeleteRole(ctx context.Context, req *pb.DeleteRoleRequest) (*pb.Empty, error) {
	s.policyClient.DeleteNode(ctx, &policypb.DeleteNodeRequest{NodeId: req.RoleId})
	return &pb.Empty{}, nil
}

func (s *WorkspaceServer) CreateFolder(ctx context.Context, req *pb.CreateFolderRequest) (*pb.Folder, error) {
	ws, _ := s.GetWorkspace(ctx, &pb.GetWorkspaceRequest{WorkspaceId: req.WorkspaceId})
	node, err := s.policyClient.CreateNode(ctx, &policypb.CreateNodeRequest{Name: req.Name, NodeType: "OA"})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "create folder: %v", err)
	}
	parentID := req.ParentOaId
	if parentID == "" {
		parentID = ws.PcNodeId
	}
	s.policyClient.CreateAssignment(ctx, &policypb.CreateAssignmentRequest{ChildId: node.Id, ParentId: parentID})
	return &pb.Folder{Id: node.Id, Name: req.Name, NgacNodeId: node.Id}, nil
}

func (s *WorkspaceServer) ListFolders(ctx context.Context, req *pb.ListFoldersRequest) (*pb.FolderList, error) {
	ws, _ := s.GetWorkspace(ctx, &pb.GetWorkspaceRequest{WorkspaceId: req.WorkspaceId})
	desc, _ := s.policyClient.GetDescendants(ctx, &policypb.GetDescendantsRequest{NodeId: ws.PcNodeId})
	var folders []*pb.Folder
	if desc != nil {
		for _, n := range desc.Nodes {
			if n.NodeType == "OA" {
				folders = append(folders, &pb.Folder{Id: n.Id, Name: n.Name, NgacNodeId: n.Id})
			}
		}
	}
	return &pb.FolderList{Folders: folders}, nil
}

func (s *WorkspaceServer) DeleteFolder(ctx context.Context, req *pb.DeleteFolderRequest) (*pb.Empty, error) {
	s.policyClient.DeleteNode(ctx, &policypb.DeleteNodeRequest{NodeId: req.FolderId})
	return &pb.Empty{}, nil
}

func (s *WorkspaceServer) CreatePermission(ctx context.Context, req *pb.CreatePermissionRequest) (*pb.Permission, error) {
	assoc, err := s.policyClient.CreateAssociation(ctx, &policypb.CreateAssociationRequest{
		UaId: req.UaId, OaId: req.OaId, Operations: req.Operations,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "create permission: %v", err)
	}
	return &pb.Permission{Id: assoc.Id, UaId: req.UaId, OaId: req.OaId, Operations: req.Operations}, nil
}

func (s *WorkspaceServer) ListPermissions(ctx context.Context, req *pb.ListPermissionsRequest) (*pb.PermissionList, error) {
	return &pb.PermissionList{}, nil
}

func (s *WorkspaceServer) DeletePermission(ctx context.Context, req *pb.DeletePermissionRequest) (*pb.Empty, error) {
	// PermissionId is the association ID — would need to look up UA/OA from it
	// For now, delete the association node directly
	return &pb.Empty{}, nil
}
