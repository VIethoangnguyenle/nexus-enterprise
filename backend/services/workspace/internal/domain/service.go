// Package domain provides business logic orchestration for the workspace service.
// It delegates to store for persistence and policy clients for NGAC graph operations.
package domain

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"

	"ngac-platform/ngac"
	"ngac-platform/pkg/provision"
	drivepb "ngac-platform/proto/drive"
	policypb "ngac-platform/proto/policy"
	"ngac-platform/services/workspace/internal/store"
)

// WorkspaceStore defines persistence operations the domain needs.
type WorkspaceStore interface {
	Insert(ctx context.Context, ws *store.Workspace) error
	GetByID(ctx context.Context, id string) (*store.Workspace, error)
	ListAll(ctx context.Context) ([]*store.Workspace, error)
	// WithOwnerLock runs fn while holding the workspace's owner lock, so checks
	// of who the owners are and the changes that follow cannot interleave.
	WithOwnerLock(ctx context.Context, wsID string, fn func(ctx context.Context) error) error
}

// Service orchestrates workspace business logic.
type Service struct {
	store     WorkspaceStore
	deptStore DepartmentStore
	// policyRead answers CheckAccess only. A read replica may lag, so every graph
	// read that feeds an authorization or write decision goes through policyWrite.
	policyRead  policypb.PolicyReadServiceClient
	policyWrite policypb.PolicyWriteServiceClient
	minioClient *minio.Client
	driveClient drivepb.DriveServiceClient
	// directory names the people behind user nodes; see WithDirectory.
	directory DirectoryStore
	// invitations holds pending offers to join; see WithInvitations.
	invitations   InvitationStore
	now           func() time.Time
	inviteLimiter *windowLimiter
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
	svc := &Service{store: st, deptStore: ds, policyRead: pr, policyWrite: pw, minioClient: mc, driveClient: dc}
	svc.inviteLimiter = newWindowLimiter(defaultInviteMax, defaultInviteWindow, svc.clock)
	return svc
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
//
// The graph writes and the row insert are separate steps with no shared
// transaction. A failure at any of them removes the nodes already created,
// newest first, so a failed provisioning leaves nothing in the graph.
func (s *Service) CreateWorkspace(ctx context.Context, in CreateWorkspaceInput) (*WorkspaceResult, error) {
	if in.Name == "" {
		return nil, fmt.Errorf("%w: name required", ErrInvalidInput)
	}

	wsID := uuid.New().String()
	id := ngac.WorkspaceID(wsID)
	prov := provision.NewCreator(s.policyWrite)

	node := func(label, name, nodeType string, props map[string]string) (*policypb.NGACNode, error) {
		n, err := prov.Node(ctx, &policypb.CreateNodeRequest{Name: name, NodeType: nodeType, Properties: props})
		if err != nil {
			return nil, prov.Fail(ctx, fmt.Errorf("create %s: %w", label, err))
		}
		return n, nil
	}

	// Build NGAC graph
	pc, err := node("PC", ngac.PCName(id), ngac.TypePC, map[string]string{
		"workspace":    in.Name,
		"workspace_id": wsID,
		"scope":        "tenant",
		"tenant_id":    wsID,
	})
	if err != nil {
		return nil, err
	}
	ownersUA, err := node("owners UA", ngac.OwnersUAName(id), ngac.TypeUA, nil)
	if err != nil {
		return nil, err
	}
	membersUA, err := node("members UA", ngac.MembersUAName(id), ngac.TypeUA, nil)
	if err != nil {
		return nil, err
	}
	mgmtOA, err := node("mgmt OA", ngac.MgmtOAName(id), ngac.TypeOA, nil)
	if err != nil {
		return nil, err
	}
	docsOA, err := node("docs OA", ngac.DocumentsOAName(id), ngac.TypeOA, nil)
	if err != nil {
		return nil, err
	}
	draftOA, err := node("draft OA", ngac.DraftDocsOAName(id), ngac.TypeOA, nil)
	if err != nil {
		return nil, err
	}
	approvedOA, err := node("approved OA", ngac.ApprovedDocsOAName(id), ngac.TypeOA, nil)
	if err != nil {
		return nil, err
	}
	channelsOA, err := node("channels OA", ngac.ChannelsOAName(id), ngac.TypeOA, nil)
	if err != nil {
		return nil, err
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
		if err := prov.Assign(ctx, a.child, a.parent); err != nil {
			return nil, prov.Fail(ctx, fmt.Errorf("assign %s→%s: %w", a.child, a.parent, err))
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
		if err := prov.Associate(ctx, a.ua, a.oa, a.ops); err != nil {
			return nil, prov.Fail(ctx, fmt.Errorf("associate %s→%s: %w", a.ua, a.oa, err))
		}
	}

	// Persist to DB
	if err := s.store.Insert(ctx, &store.Workspace{
		ID: wsID, Name: in.Name, Desc: "", OwnerID: in.UserID, NGACPcID: pc.Id, DocumentsOAID: docsOA.Id,
	}); err != nil {
		return nil, prov.Fail(ctx, err)
	}
	prov.Done()

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
	ancestors, err := s.policyWrite.GetAncestors(ctx, &policypb.GetAncestorsRequest{
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
	children, err := s.policyWrite.GetChildren(ctx, &policypb.GetChildrenRequest{NodeId: ws.PcNodeID})
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

// withdrawOffers revokes what was still open to a removed person's proved
// address in this workspace, so an old invitation cannot bring them back.
// Access is already gone; a failure here is logged, not returned.
func (s *Service) withdrawOffers(ctx context.Context, wsID, nodeID string) {
	if s.invitations == nil {
		return
	}
	email, err := s.invitations.VerifiedEmailByNode(ctx, nodeID)
	if err != nil || email == "" {
		if err != nil {
			slog.Warn("could not look up a removed member's address", "workspace", wsID, "error", err)
		}
		return
	}
	if _, err := s.invitations.RevokePendingForEmail(ctx, wsID, email, s.clock()); err != nil {
		slog.Warn("member removed but their invitations remain", "workspace", wsID, "error", err)
	}
}

// ownersOf returns the people in the workspace's Owners UA and the UA's ID.
func (s *Service) ownersOf(ctx context.Context, ws *WorkspaceResult) (string, []*policypb.NGACNode, error) {
	ownersUAID, err := s.FindUAByName(ctx, ws.ID, ngac.OwnersUAName(ngac.WorkspaceID(ws.ID)))
	if err != nil {
		return "", nil, err
	}
	kids, err := s.policyWrite.GetChildren(ctx, &policypb.GetChildrenRequest{NodeId: ownersUAID})
	if err != nil {
		return "", nil, fmt.Errorf("get owners: %w", err)
	}
	return ownersUAID, userNodes(kids.GetNodes()), nil
}

func containsNode(nodes []*policypb.NGACNode, id string) bool {
	for _, n := range nodes {
		if n.Id == id {
			return true
		}
	}
	return false
}

// requireOwner confirms the caller is in the Owners UA. Holding manage is not
// enough to change who the owners are: a role can carry manage, and a manager
// must not be able to demote or replace the people above them.
func requireOwner(owners []*policypb.NGACNode, callerNodeID string) error {
	if !containsNode(owners, callerNodeID) {
		return fmt.Errorf("%w: only an owner changes the owners", ErrAccessDenied)
	}
	return nil
}

// RemoveMember removes a user from all UAs under the workspace PC. The caller
// must hold invite on the workspace's Mgmt OA.
//
// Removing someone in the Owners UA takes being an Owner and holding manage, and
// the last owner stays. That rule is checked and acted on under the workspace's
// owner lock, so two removals at once cannot both pass it.
func (s *Service) RemoveMember(ctx context.Context, callerNodeID, wsID, targetNGACNodeID string) error {
	ws, err := s.authorizeAdmin(ctx, callerNodeID, wsID, ngac.OpInvite)
	if err != nil {
		return err
	}
	remove := func(ctx context.Context) error {
		_, owners, err := s.ownersOf(ctx, ws)
		if err != nil {
			return err
		}
		if containsNode(owners, targetNGACNodeID) {
			if err := requireOwner(owners, callerNodeID); err != nil {
				return err
			}
			mgmtID, err := s.mgmtOAID(ctx, ws)
			if err != nil {
				return err
			}
			if err := s.checkAccess(ctx, callerNodeID, mgmtID, ngac.OpManage); err != nil {
				return err
			}
			if len(owners) <= 1 {
				return fmt.Errorf("%w: cannot remove last owner", ErrInvalidInput)
			}
		}
		return s.detachMember(ctx, ws, targetNGACNodeID)
	}
	if err := s.store.WithOwnerLock(ctx, ws.ID, remove); err != nil {
		return err
	}
	s.withdrawOffers(ctx, ws.ID, targetNGACNodeID)
	if s.directory != nil {
		// Access is already gone; a listing left behind only keeps the person in
		// contacts, so a failure is reported but does not undo the removal.
		if err := s.directory.RemoveTenantUser(ctx, ws.ID, targetNGACNodeID); err != nil {
			slog.Warn("member removed but their listing remains", "workspace", ws.ID, "error", err)
		}
	}
	return nil
}

// detachMember removes a user from every UA under the workspace PC. It performs
// no authorization; callers must have authorized already.
func (s *Service) detachMember(ctx context.Context, ws *WorkspaceResult, targetNGACNodeID string) error {
	desc, err := s.policyWrite.GetDescendants(ctx, &policypb.GetDescendantsRequest{NodeId: ws.PcNodeID})
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
	desc, err := s.policyWrite.GetDescendants(ctx, &policypb.GetDescendantsRequest{NodeId: ws.PcNodeID})
	if err != nil {
		return nil, fmt.Errorf("get descendants: %w", err)
	}
	seen := make(map[string]bool)
	var members []*Member
	for _, n := range desc.Nodes {
		if n.NodeType == ngac.TypeU && !seen[n.Id] {
			seen[n.Id] = true
			members = append(members, &Member{NGACNodeID: n.Id, Username: ngac.DisplayName(n.Name, n.Properties)})
		}
	}
	return members, nil
}

// TransferOwnership adds a user to the Owners UA of a workspace. The caller
// must hold manage on the workspace's Mgmt OA and be an Owner, and the new owner
// must already belong to the workspace.
func (s *Service) TransferOwnership(ctx context.Context, callerNodeID, wsID, newOwnerNGACNodeID string) error {
	ws, err := s.authorizeAdmin(ctx, callerNodeID, wsID, ngac.OpManage)
	if err != nil {
		return err
	}
	add := func(ctx context.Context) error {
		ownersUAID, owners, err := s.ownersOf(ctx, ws)
		if err != nil {
			return err
		}
		if err := requireOwner(owners, callerNodeID); err != nil {
			return err
		}
		if err := s.requireMemberUser(ctx, ws, newOwnerNGACNodeID); err != nil {
			return err
		}
		if _, err := s.policyWrite.CreateAssignment(ctx, &policypb.CreateAssignmentRequest{
			ChildId: newOwnerNGACNodeID, ParentId: ownersUAID,
		}); err != nil {
			return fmt.Errorf("assign owner: %w", err)
		}
		return nil
	}
	return s.store.WithOwnerLock(ctx, ws.ID, add)
}

// RemoveOwner removes a user from the Owners UA, refusing if they are the last
// owner. The caller must hold manage on the workspace's Mgmt OA and be an Owner;
// the count and the removal happen under the workspace's owner lock.
func (s *Service) RemoveOwner(ctx context.Context, callerNodeID, wsID, targetNGACNodeID string) error {
	ws, err := s.authorizeAdmin(ctx, callerNodeID, wsID, ngac.OpManage)
	if err != nil {
		return err
	}
	remove := func(ctx context.Context) error {
		ownersUAID, owners, err := s.ownersOf(ctx, ws)
		if err != nil {
			return err
		}
		if err := requireOwner(owners, callerNodeID); err != nil {
			return err
		}
		if !containsNode(owners, targetNGACNodeID) {
			return fmt.Errorf("%w: not an owner of this workspace", ErrNotFound)
		}
		if len(owners) <= 1 {
			return fmt.Errorf("%w: cannot remove last owner", ErrInvalidInput)
		}
		if _, err := s.policyWrite.RemoveAssignment(ctx, &policypb.RemoveAssignmentRequest{
			ChildId: targetNGACNodeID, ParentId: ownersUAID,
		}); err != nil {
			return fmt.Errorf("remove assignment: %w", err)
		}
		return nil
	}
	return s.store.WithOwnerLock(ctx, ws.ID, remove)
}

// Role represents an NGAC role (UA) in a workspace.
type Role struct {
	ID         string
	Name       string
	NGACNodeID string
}

// CreateRole provisions a new UA role under the workspace PC. The caller must
// hold manage on the workspace's Mgmt OA.
//
// The node is named by a generated ID; the name the administrator typed is kept
// as its display_name property. Node names are matched exactly, so a name taken
// from input would let a role pose as a node the platform builds itself.
func (s *Service) CreateRole(ctx context.Context, callerNodeID, wsID, roleName string) (*Role, error) {
	ws, err := s.authorizeAdmin(ctx, callerNodeID, wsID, ngac.OpManage)
	if err != nil {
		return nil, err
	}
	if err := ngac.ValidateRoleName(roleName); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}
	prov := provision.NewCreator(s.policyWrite)
	node, err := prov.Node(ctx, &policypb.CreateNodeRequest{
		Name:     ngac.RoleUAName(ngac.RoleID(uuid.New().String())),
		NodeType: ngac.TypeUA,
		Properties: map[string]string{
			ngac.PropType:        ngac.PropTypeRole,
			ngac.PropDisplayName: roleName,
			"workspace_id":       wsID,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("create role: %w", err)
	}
	if err := prov.Assign(ctx, node.Id, ws.PcNodeID); err != nil {
		return nil, prov.Fail(ctx, fmt.Errorf("assign role: %w", err))
	}
	prov.Done()
	return &Role{ID: node.Id, Name: roleName, NGACNodeID: node.Id}, nil
}

// ListRoles returns all UA roles under the workspace PC. The caller must belong
// to the workspace.
func (s *Service) ListRoles(ctx context.Context, callerNodeID, wsID string) ([]*Role, error) {
	ws, err := s.authorizeMember(ctx, callerNodeID, wsID)
	if err != nil {
		return nil, err
	}
	children, err := s.policyWrite.GetChildren(ctx, &policypb.GetChildrenRequest{NodeId: ws.PcNodeID})
	if err != nil {
		return nil, fmt.Errorf("get children: %w", err)
	}
	var roles []*Role
	for _, n := range children.Nodes {
		if isRole(n) {
			roles = append(roles, &Role{ID: n.Id, Name: ngac.DisplayName(n.Name, n.Properties), NGACNodeID: n.Id})
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
	if err := s.requireRole(ctx, ws, roleID); err != nil {
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
	prov := provision.NewCreator(s.policyWrite)
	node, err := prov.Node(ctx, &policypb.CreateNodeRequest{
		Name:       ngac.FolderNodeName(ngac.FolderID(uuid.New().String())),
		NodeType:   ngac.TypeOA,
		Properties: map[string]string{ngac.PropDisplayName: name, "workspace_id": wsID},
	})
	if err != nil {
		return nil, fmt.Errorf("create folder: %w", err)
	}
	parentID := parentOaID
	if parentID == "" {
		parentID = ws.PcNodeID
	}
	if err := prov.Assign(ctx, node.Id, parentID); err != nil {
		return nil, prov.Fail(ctx, fmt.Errorf("assign folder: %w", err))
	}
	prov.Done()
	return &Folder{ID: node.Id, Name: name, NGACNodeID: node.Id}, nil
}

// ListFolders returns all OA folders under the workspace PC. The caller must
// belong to the workspace.
func (s *Service) ListFolders(ctx context.Context, callerNodeID, wsID string) ([]*Folder, error) {
	ws, err := s.authorizeMember(ctx, callerNodeID, wsID)
	if err != nil {
		return nil, err
	}
	desc, err := s.policyWrite.GetDescendants(ctx, &policypb.GetDescendantsRequest{NodeId: ws.PcNodeID})
	if err != nil {
		return nil, fmt.Errorf("get descendants: %w", err)
	}
	var folders []*Folder
	for _, n := range desc.Nodes {
		if n.NodeType == ngac.TypeOA {
			folders = append(folders, &Folder{ID: n.Id, Name: ngac.DisplayName(n.Name, n.Properties), NGACNodeID: n.Id})
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
		ChannelName:     wsName,
		ChannelNgacOaId: docsOaID,
		ChannelNgacUaId: ownersUaID,
	})
	if err != nil {
		slog.Warn("failed to create workspace drive", "workspace", wsID, "error", err)
	}
}
