// Package domain provides business logic orchestration for the workspace service.
// It delegates to store for persistence and policy clients for NGAC graph operations.
package domain

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"

	"ngac-platform/ngac"
	drivepb "ngac-platform/proto/drive"
	policypb "ngac-platform/proto/policy"
	"ngac-platform/services/workspace/internal/store"
)

// WorkspaceStore defines persistence operations the domain needs.
type WorkspaceStore interface {
	Insert(ctx context.Context, ws *store.Workspace) error
	GetByID(ctx context.Context, id string) (*store.Workspace, error)
	ListAll(ctx context.Context) ([]*store.Workspace, error)
}

// Service orchestrates workspace business logic.
type Service struct {
	store       WorkspaceStore
	deptStore   DepartmentStore
	policyRead  policypb.PolicyReadServiceClient
	policyWrite policypb.PolicyWriteServiceClient
	minioClient *minio.Client
	driveClient drivepb.DriveServiceClient
}

// NewService creates a workspace domain service.
func NewService(
	st WorkspaceStore,
	ds DepartmentStore,
	pr policypb.PolicyReadServiceClient,
	pw policypb.PolicyWriteServiceClient,
	mc *minio.Client,
	dc drivepb.DriveServiceClient,
) *Service {
	return &Service{store: st, deptStore: ds, policyRead: pr, policyWrite: pw, minioClient: mc, driveClient: dc}
}

// CreateWorkspaceInput holds the parameters for creating a workspace.
type CreateWorkspaceInput struct {
	Name           string
	UserID         string
	UserNGACNodeID string
}

// WorkspaceResult is the domain output for workspace operations.
type WorkspaceResult struct {
	ID            string
	Name          string
	PcNodeID      string
	OwnersUaID    string
	MembersUaID   string
	MgmtOaID      string
	DocumentsOaID string
	ChannelsOaID  string
	CreatedBy     string
}

// CreateWorkspace provisions a new workspace: NGAC graph, DB row, MinIO bucket, Drive root.
func (s *Service) CreateWorkspace(ctx context.Context, in CreateWorkspaceInput) (*WorkspaceResult, error) {
	if in.Name == "" {
		return nil, fmt.Errorf("%w: name required", ErrInvalidInput)
	}

	wsID := uuid.New().String()

	// Build NGAC graph
	pc, err := s.policyWrite.CreateNode(ctx, &policypb.CreateNodeRequest{
		Name: ngac.PCName(wsID), NodeType: ngac.TypePC,
		Properties: map[string]string{
			"workspace":    in.Name,
			"workspace_id": wsID,
			"scope":        "tenant",
			"tenant_id":    wsID,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("create PC: %w", err)
	}

	ownersUA, err := s.policyWrite.CreateNode(ctx, &policypb.CreateNodeRequest{Name: ngac.OwnersUAName(wsID), NodeType: ngac.TypeUA})
	if err != nil {
		return nil, fmt.Errorf("create owners UA: %w", err)
	}
	membersUA, err := s.policyWrite.CreateNode(ctx, &policypb.CreateNodeRequest{Name: ngac.MembersUAName(wsID), NodeType: ngac.TypeUA})
	if err != nil {
		return nil, fmt.Errorf("create members UA: %w", err)
	}
	mgmtOA, err := s.policyWrite.CreateNode(ctx, &policypb.CreateNodeRequest{Name: ngac.MgmtOAName(wsID), NodeType: ngac.TypeOA})
	if err != nil {
		return nil, fmt.Errorf("create mgmt OA: %w", err)
	}
	docsOA, err := s.policyWrite.CreateNode(ctx, &policypb.CreateNodeRequest{Name: ngac.DocumentsOAName(wsID), NodeType: ngac.TypeOA})
	if err != nil {
		return nil, fmt.Errorf("create docs OA: %w", err)
	}
	draftOA, err := s.policyWrite.CreateNode(ctx, &policypb.CreateNodeRequest{Name: ngac.DraftDocsOAName(wsID), NodeType: ngac.TypeOA})
	if err != nil {
		return nil, fmt.Errorf("create draft OA: %w", err)
	}
	approvedOA, err := s.policyWrite.CreateNode(ctx, &policypb.CreateNodeRequest{Name: ngac.ApprovedDocsOAName(wsID), NodeType: ngac.TypeOA})
	if err != nil {
		return nil, fmt.Errorf("create approved OA: %w", err)
	}
	channelsOA, err := s.policyWrite.CreateNode(ctx, &policypb.CreateNodeRequest{Name: ngac.ChannelsOAName(wsID), NodeType: ngac.TypeOA})
	if err != nil {
		return nil, fmt.Errorf("create channels OA: %w", err)
	}

	// Assignments
	assignments := []struct{ child, parent string }{
		{ownersUA.Id, pc.Id}, {membersUA.Id, pc.Id},
		{mgmtOA.Id, pc.Id}, {docsOA.Id, pc.Id}, {channelsOA.Id, pc.Id},
		{draftOA.Id, docsOA.Id}, {approvedOA.Id, docsOA.Id},
		// Owners UA is assigned UNDER Members UA, not the other way round.
		// BFS walks child→parent, so the child inherits the parent's
		// associations: this gives owners the member grants on top of their
		// own. Reversing it hands every member the full owner association set
		// and silently makes the member-scoped associations below dead code.
		{ownersUA.Id, membersUA.Id},
		{in.UserNGACNodeID, ownersUA.Id},
	}
	for _, a := range assignments {
		if _, err := s.policyWrite.CreateAssignment(ctx, &policypb.CreateAssignmentRequest{ChildId: a.child, ParentId: a.parent}); err != nil {
			return nil, fmt.Errorf("assign %s→%s: %w", a.child, a.parent, err)
		}
	}

	// Associations
	associations := []struct {
		ua, oa string
		ops    []string
	}{
		{ownersUA.Id, mgmtOA.Id, ngac.AllOwnerOps()},
		{ownersUA.Id, docsOA.Id, ngac.AllOwnerOps()},
		{ownersUA.Id, channelsOA.Id, ngac.AllOwnerOps()},
		{membersUA.Id, docsOA.Id, ngac.MemberDocumentOps()},
		{membersUA.Id, channelsOA.Id, ngac.MemberChannelOps()},
	}
	for _, a := range associations {
		if _, err := s.policyWrite.CreateAssociation(ctx, &policypb.CreateAssociationRequest{UaId: a.ua, OaId: a.oa, Operations: a.ops}); err != nil {
			return nil, fmt.Errorf("associate %s→%s: %w", a.ua, a.oa, err)
		}
	}

	// Persist to DB
	if err := s.store.Insert(ctx, &store.Workspace{
		ID: wsID, Name: in.Name, Desc: "", OwnerID: in.UserID, NGACPcID: pc.Id,
	}); err != nil {
		return nil, err
	}

	// Create MinIO bucket (non-fatal)
	s.ensureMinioBucket(ctx, wsID)

	// Create workspace root drive folder (non-fatal)
	s.ensureDriveRoot(ctx, wsID, in.Name, docsOA.Id, ownersUA.Id)

	return &WorkspaceResult{
		ID: wsID, Name: in.Name, PcNodeID: pc.Id,
		OwnersUaID: ownersUA.Id, MembersUaID: membersUA.Id,
		MgmtOaID: mgmtOA.Id, DocumentsOaID: docsOA.Id,
		ChannelsOaID: channelsOA.Id, CreatedBy: in.UserID,
	}, nil
}

// ViewWorkspace returns a workspace to a caller who belongs to it.
func (s *Service) ViewWorkspace(ctx context.Context, callerNodeID, id string) (*WorkspaceResult, error) {
	return s.authorizeMember(ctx, callerNodeID, id)
}

// GetWorkspace retrieves a workspace by ID. It performs no authorization and is
// for internal use; caller-facing reads go through ViewWorkspace.
func (s *Service) GetWorkspace(ctx context.Context, id string) (*WorkspaceResult, error) {
	ws, err := s.store.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("%w: workspace %s", ErrNotFound, id)
	}
	return &WorkspaceResult{ID: ws.ID, Name: ws.Name, PcNodeID: ws.NGACPcID}, nil
}

// ListAccessibleWorkspaces returns workspaces the given user has access to.
func (s *Service) ListAccessibleWorkspaces(ctx context.Context, userNGACNodeID string) ([]*WorkspaceResult, error) {
	allWS, err := s.store.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	if len(allWS) == 0 {
		return nil, nil
	}

	// Ask from the user's side, once.
	//
	// This used to walk every workspace's entire subtree looking for the user —
	// one GetDescendants per workspace in the system, each returning every node
	// under that policy class. Walking up from the user instead is a single
	// call whose payload is just that user's attributes, and it answers the
	// same question: a workspace is accessible exactly when its policy class is
	// among the user's ancestors.
	ancestors, err := s.policyRead.GetAncestors(ctx, &policypb.GetAncestorsRequest{
		NodeId: userNGACNodeID,
	})
	if err != nil {
		return nil, fmt.Errorf("resolve user attributes: %w", err)
	}

	reachable := make(map[string]bool, len(ancestors.GetNodes()))
	for _, n := range ancestors.GetNodes() {
		reachable[n.Id] = true
	}

	var accessible []*WorkspaceResult
	for _, ws := range allWS {
		if reachable[ws.NGACPcID] {
			accessible = append(accessible, &WorkspaceResult{
				ID: ws.ID, Name: ws.Name, PcNodeID: ws.NGACPcID,
			})
		}
	}
	return accessible, nil
}

// FindUAByName finds a User Attribute node under a workspace PC by name convention.
func (s *Service) FindUAByName(ctx context.Context, wsID, uaName string) (string, error) {
	ws, err := s.GetWorkspace(ctx, wsID)
	if err != nil {
		return "", err
	}
	children, err := s.policyRead.GetChildren(ctx, &policypb.GetChildrenRequest{NodeId: ws.PcNodeID})
	if err != nil {
		return "", fmt.Errorf("get children: %w", err)
	}
	for _, n := range children.Nodes {
		if n.NodeType == ngac.TypeUA && n.Name == uaName {
			return n.Id, nil
		}
	}
	return "", fmt.Errorf("%w: UA %q not found in workspace %s", ErrNotFound, uaName, wsID)
}

// InviteMember adds a user to the Members UA of a workspace. The caller must
// hold invite on the workspace's Mgmt OA.
func (s *Service) InviteMember(ctx context.Context, callerNodeID, wsID, targetNGACNodeID string) error {
	ws, err := s.authorizeAdmin(ctx, callerNodeID, wsID, ngac.OpInvite)
	if err != nil {
		return err
	}
	membersUAID, err := s.FindUAByName(ctx, wsID, ngac.MembersUAName(ws.ID))
	if err != nil {
		return err
	}
	_, err = s.policyWrite.CreateAssignment(ctx, &policypb.CreateAssignmentRequest{
		ChildId: targetNGACNodeID, ParentId: membersUAID,
	})
	if err != nil {
		return fmt.Errorf("assign member: %w", err)
	}
	return nil
}

// RemoveMember removes a user from all UAs under the workspace PC. The caller
// must hold invite on the workspace's Mgmt OA.
func (s *Service) RemoveMember(ctx context.Context, callerNodeID, wsID, targetNGACNodeID string) error {
	ws, err := s.authorizeAdmin(ctx, callerNodeID, wsID, ngac.OpInvite)
	if err != nil {
		return err
	}
	return s.detachMember(ctx, ws, targetNGACNodeID)
}

// detachMember removes a user from every UA under the workspace PC. It performs
// no authorization; callers must have authorized already.
func (s *Service) detachMember(ctx context.Context, ws *WorkspaceResult, targetNGACNodeID string) error {
	desc, err := s.policyRead.GetDescendants(ctx, &policypb.GetDescendantsRequest{NodeId: ws.PcNodeID})
	if err != nil {
		return fmt.Errorf("get descendants: %w", err)
	}
	// Every UA the user is detached from is one revocation. A failure here
	// leaves the user assigned and therefore still authorized, so it has to
	// surface rather than be dropped — reporting "removed" while the graph
	// still grants access is the worst possible outcome.
	var failed []string
	for _, n := range desc.Nodes {
		if n.NodeType != ngac.TypeUA {
			continue
		}
		if _, err := s.policyWrite.RemoveAssignment(ctx, &policypb.RemoveAssignmentRequest{
			ChildId: targetNGACNodeID, ParentId: n.Id,
		}); err != nil {
			slog.Error("failed to detach user from UA during member removal",
				"user_node_id", targetNGACNodeID, "ua_id", n.Id, "error", err)
			failed = append(failed, n.Id)
		}
	}
	if len(failed) > 0 {
		return fmt.Errorf("member not fully removed, still assigned to %d attribute(s): %v", len(failed), failed)
	}
	return nil
}

// Member represents a workspace member.
type Member struct {
	NGACNodeID string
	Username   string
}

// ListMembers returns all unique users under the workspace PC. The caller must
// belong to the workspace.
func (s *Service) ListMembers(ctx context.Context, callerNodeID, wsID string) ([]*Member, error) {
	ws, err := s.authorizeMember(ctx, callerNodeID, wsID)
	if err != nil {
		return nil, err
	}
	desc, err := s.policyRead.GetDescendants(ctx, &policypb.GetDescendantsRequest{NodeId: ws.PcNodeID})
	if err != nil {
		return nil, fmt.Errorf("get descendants: %w", err)
	}
	seen := make(map[string]bool)
	var members []*Member
	for _, n := range desc.Nodes {
		if n.NodeType == ngac.TypeU && !seen[n.Id] {
			seen[n.Id] = true
			members = append(members, &Member{NGACNodeID: n.Id, Username: n.Name})
		}
	}
	return members, nil
}

// UpdateMemberRoles removes a user from all UAs then assigns to specified roles.
// The caller must hold manage on the workspace's Mgmt OA, and every role must be
// a UA of this workspace.
func (s *Service) UpdateMemberRoles(ctx context.Context, callerNodeID, wsID, targetNGACNodeID string, roleIDs []string) error {
	ws, err := s.authorizeAdmin(ctx, callerNodeID, wsID, ngac.OpManage)
	if err != nil {
		return err
	}
	for _, roleID := range roleIDs {
		if err := s.requireInWorkspace(ctx, ws, roleID, ngac.TypeUA); err != nil {
			return err
		}
	}
	if err := s.detachMember(ctx, ws, targetNGACNodeID); err != nil {
		return err
	}
	for _, roleID := range roleIDs {
		if _, err := s.policyWrite.CreateAssignment(ctx, &policypb.CreateAssignmentRequest{
			ChildId: targetNGACNodeID, ParentId: roleID,
		}); err != nil {
			return fmt.Errorf("assign role %s: %w", roleID, err)
		}
	}
	return nil
}

// TransferOwnership adds a user to the Owners UA of a workspace. The caller
// must hold manage on the workspace's Mgmt OA.
func (s *Service) TransferOwnership(ctx context.Context, callerNodeID, wsID, newOwnerNGACNodeID string) error {
	ws, err := s.authorizeAdmin(ctx, callerNodeID, wsID, ngac.OpManage)
	if err != nil {
		return err
	}
	ownersUAID, err := s.FindUAByName(ctx, wsID, ngac.OwnersUAName(ws.ID))
	if err != nil {
		return err
	}
	if _, err := s.policyWrite.CreateAssignment(ctx, &policypb.CreateAssignmentRequest{
		ChildId: newOwnerNGACNodeID, ParentId: ownersUAID,
	}); err != nil {
		return fmt.Errorf("assign owner: %w", err)
	}
	return nil
}

// RemoveOwner removes a user from the Owners UA, refusing if they are the last
// owner. The caller must hold manage on the workspace's Mgmt OA.
func (s *Service) RemoveOwner(ctx context.Context, callerNodeID, wsID, targetNGACNodeID string) error {
	ws, err := s.authorizeAdmin(ctx, callerNodeID, wsID, ngac.OpManage)
	if err != nil {
		return err
	}
	ownersUAID, err := s.FindUAByName(ctx, wsID, ngac.OwnersUAName(ws.ID))
	if err != nil {
		return err
	}

	ownerChildren, err := s.policyRead.GetChildren(ctx, &policypb.GetChildrenRequest{NodeId: ownersUAID})
	if err != nil {
		return fmt.Errorf("get owner children: %w", err)
	}
	userCount := 0
	for _, n := range ownerChildren.Nodes {
		if n.NodeType == ngac.TypeU {
			userCount++
		}
	}
	if userCount <= 1 {
		return fmt.Errorf("%w: cannot remove last owner", ErrInvalidInput)
	}

	if _, err := s.policyWrite.RemoveAssignment(ctx, &policypb.RemoveAssignmentRequest{
		ChildId: targetNGACNodeID, ParentId: ownersUAID,
	}); err != nil {
		return fmt.Errorf("remove assignment: %w", err)
	}
	return nil
}

// Role represents an NGAC role (UA) in a workspace.
type Role struct {
	ID         string
	Name       string
	NGACNodeID string
}

// CreateRole provisions a new UA role under the workspace PC. The caller must
// hold manage on the workspace's Mgmt OA.
func (s *Service) CreateRole(ctx context.Context, callerNodeID, wsID, roleName string) (*Role, error) {
	ws, err := s.authorizeAdmin(ctx, callerNodeID, wsID, ngac.OpManage)
	if err != nil {
		return nil, err
	}
	node, err := s.policyWrite.CreateNode(ctx, &policypb.CreateNodeRequest{Name: roleName, NodeType: ngac.TypeUA})
	if err != nil {
		return nil, fmt.Errorf("create role: %w", err)
	}
	if _, err := s.policyWrite.CreateAssignment(ctx, &policypb.CreateAssignmentRequest{
		ChildId: node.Id, ParentId: ws.PcNodeID,
	}); err != nil {
		return nil, fmt.Errorf("assign role: %w", err)
	}
	return &Role{ID: node.Id, Name: roleName, NGACNodeID: node.Id}, nil
}

// ListRoles returns all UA roles under the workspace PC. The caller must belong
// to the workspace.
func (s *Service) ListRoles(ctx context.Context, callerNodeID, wsID string) ([]*Role, error) {
	ws, err := s.authorizeMember(ctx, callerNodeID, wsID)
	if err != nil {
		return nil, err
	}
	children, err := s.policyRead.GetChildren(ctx, &policypb.GetChildrenRequest{NodeId: ws.PcNodeID})
	if err != nil {
		return nil, fmt.Errorf("get children: %w", err)
	}
	var roles []*Role
	for _, n := range children.Nodes {
		if n.NodeType == ngac.TypeUA {
			roles = append(roles, &Role{ID: n.Id, Name: n.Name, NGACNodeID: n.Id})
		}
	}
	return roles, nil
}

// DeleteRole removes a role (NGAC UA node). The caller must hold manage on the
// workspace's Mgmt OA, and the role must be a UA of this workspace.
func (s *Service) DeleteRole(ctx context.Context, callerNodeID, wsID, roleID string) error {
	ws, err := s.authorizeAdmin(ctx, callerNodeID, wsID, ngac.OpManage)
	if err != nil {
		return err
	}
	if err := s.requireInWorkspace(ctx, ws, roleID, ngac.TypeUA); err != nil {
		return err
	}
	if _, err := s.policyWrite.DeleteNode(ctx, &policypb.DeleteNodeRequest{NodeId: roleID}); err != nil {
		return fmt.Errorf("delete role: %w", err)
	}
	return nil
}

// Folder represents an NGAC OA folder in a workspace.
type Folder struct {
	ID         string
	Name       string
	NGACNodeID string
}

// CreateFolder provisions a new OA folder under a parent (or workspace PC if no
// parent). The caller must hold manage on the workspace's Mgmt OA, and a parent,
// if given, must be an OA of this workspace.
func (s *Service) CreateFolder(ctx context.Context, callerNodeID, wsID, name, parentOaID string) (*Folder, error) {
	ws, err := s.authorizeAdmin(ctx, callerNodeID, wsID, ngac.OpManage)
	if err != nil {
		return nil, err
	}
	if parentOaID != "" {
		if err := s.requireInWorkspace(ctx, ws, parentOaID, ngac.TypeOA); err != nil {
			return nil, err
		}
	}
	node, err := s.policyWrite.CreateNode(ctx, &policypb.CreateNodeRequest{Name: name, NodeType: ngac.TypeOA})
	if err != nil {
		return nil, fmt.Errorf("create folder: %w", err)
	}
	parentID := parentOaID
	if parentID == "" {
		parentID = ws.PcNodeID
	}
	if _, err := s.policyWrite.CreateAssignment(ctx, &policypb.CreateAssignmentRequest{
		ChildId: node.Id, ParentId: parentID,
	}); err != nil {
		return nil, fmt.Errorf("assign folder: %w", err)
	}
	return &Folder{ID: node.Id, Name: name, NGACNodeID: node.Id}, nil
}

// ListFolders returns all OA folders under the workspace PC. The caller must
// belong to the workspace.
func (s *Service) ListFolders(ctx context.Context, callerNodeID, wsID string) ([]*Folder, error) {
	ws, err := s.authorizeMember(ctx, callerNodeID, wsID)
	if err != nil {
		return nil, err
	}
	desc, err := s.policyRead.GetDescendants(ctx, &policypb.GetDescendantsRequest{NodeId: ws.PcNodeID})
	if err != nil {
		return nil, fmt.Errorf("get descendants: %w", err)
	}
	var folders []*Folder
	for _, n := range desc.Nodes {
		if n.NodeType == ngac.TypeOA {
			folders = append(folders, &Folder{ID: n.Id, Name: n.Name, NGACNodeID: n.Id})
		}
	}
	return folders, nil
}

// DeleteFolder removes a folder (NGAC OA node). The caller must hold manage on
// the workspace's Mgmt OA, and the folder must be an OA of this workspace.
func (s *Service) DeleteFolder(ctx context.Context, callerNodeID, wsID, folderID string) error {
	ws, err := s.authorizeAdmin(ctx, callerNodeID, wsID, ngac.OpManage)
	if err != nil {
		return err
	}
	if err := s.requireInWorkspace(ctx, ws, folderID, ngac.TypeOA); err != nil {
		return err
	}
	if _, err := s.policyWrite.DeleteNode(ctx, &policypb.DeleteNodeRequest{NodeId: folderID}); err != nil {
		return fmt.Errorf("delete folder: %w", err)
	}
	return nil
}

// Permission represents an NGAC association.
type Permission struct {
	ID         string
	UaID       string
	OaID       string
	Operations []string
}

// CreatePermission creates an association between a UA and OA.
//
// Two checks, both required:
//   - the caller holds manage on the workspace's Mgmt OA (may administer it);
//   - the caller holds every requested operation on the target OA. Without this,
//     manage would be a key to every other operation: a caller could grant a UA
//     they belong to any right on any OA. Delegation never exceeds what the
//     delegator has, and a single op not held rejects the whole request.
//
// Operations must be among the fixed NGAC operations.
func (s *Service) CreatePermission(ctx context.Context, callerNodeID, wsID, uaID, oaID string, ops []string) (*Permission, error) {
	if uaID == "" || oaID == "" {
		return nil, fmt.Errorf("%w: ua_id and oa_id required", ErrInvalidInput)
	}
	if err := validateOperations(ops); err != nil {
		return nil, err
	}
	if _, err := s.authorizeAdmin(ctx, callerNodeID, wsID, ngac.OpManage); err != nil {
		return nil, err
	}
	for _, op := range ops {
		if err := s.checkAccess(ctx, callerNodeID, oaID, op); err != nil {
			return nil, fmt.Errorf("cannot grant an operation you do not hold: %w", err)
		}
	}
	assoc, err := s.policyWrite.CreateAssociation(ctx, &policypb.CreateAssociationRequest{
		UaId: uaID, OaId: oaID, Operations: ops,
	})
	if err != nil {
		return nil, fmt.Errorf("create permission: %w", err)
	}
	return &Permission{ID: assoc.Id, UaID: uaID, OaID: oaID, Operations: ops}, nil
}

// DeletePermission authorizes removal of an association. Removal itself is not
// implemented yet (the RPC has always been a no-op); the check is in place so
// that implementing it cannot ship unguarded.
func (s *Service) DeletePermission(ctx context.Context, callerNodeID, wsID, _ string) error {
	_, err := s.authorizeAdmin(ctx, callerNodeID, wsID, ngac.OpManage)
	return err
}

// ensureMinioBucket creates a MinIO bucket for the workspace (non-fatal).
func (s *Service) ensureMinioBucket(ctx context.Context, wsID string) {
	if s.minioClient == nil {
		return
	}
	bucketName := fmt.Sprintf("ws-%s", wsID)
	err := s.minioClient.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{})
	if err != nil {
		exists, errExists := s.minioClient.BucketExists(ctx, bucketName)
		if errExists != nil || !exists {
			slog.Warn("failed to create minio bucket", "bucket", bucketName, "error", err)
		}
	} else {
		slog.Info("created minio bucket", "bucket", bucketName)
	}
}

// ensureDriveRoot creates the root drive folder for the workspace (non-fatal).
func (s *Service) ensureDriveRoot(ctx context.Context, wsID, wsName, docsOaID, ownersUaID string) {
	if s.driveClient == nil {
		return
	}
	_, err := s.driveClient.CreateDriveForChannel(ctx, &drivepb.CreateDriveForChannelRequest{
		WorkspaceId:     wsID,
		ChannelId:       wsID,
		ChannelName:     wsName + "_Root",
		ChannelNgacOaId: docsOaID,
		ChannelNgacUaId: ownersUaID,
	})
	if err != nil {
		slog.Warn("failed to create workspace drive", "workspace", wsID, "error", err)
	}
}
