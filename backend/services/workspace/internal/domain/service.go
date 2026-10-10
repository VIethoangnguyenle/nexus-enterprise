// Package domain provides business logic orchestration for the workspace service.
// It delegates to store for persistence and policy clients for NGAC graph operations.
package domain

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"

	"ngac-platform/ngac"
	"ngac-platform/pkg/provision"
	"ngac-platform/pkg/realtime"
	drivepb "ngac-platform/proto/drive"
	policypb "ngac-platform/proto/policy"
	"ngac-platform/services/workspace/internal/store"
)

// WorkspaceStore defines persistence operations the domain needs.
type WorkspaceStore interface {
	Insert(ctx context.Context, ws *store.Workspace) error
	GetByID(ctx context.Context, id string) (*store.Workspace, error)
	ListAll(ctx context.Context) ([]*store.Workspace, error)
	// UpdateDetails sets the name and/or description given (nil leaves a field
	// alone) in one statement and returns both as they are afterwards.
	UpdateDetails(ctx context.Context, id string, name, description *string) (string, string, error)
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
	// mail sends the invitation email; nil sends none. See WithInviteMail.
	mail       InviteMailer
	appBaseURL string
	mailResend time.Duration
	mailSlots  chan struct{}
	mailWG     sync.WaitGroup
	// emitter announces committed changes to live clients; see WithEmitter.
	emitter realtime.Emitter
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
	svc := &Service{store: st, deptStore: ds, policyRead: pr, policyWrite: pw, minioClient: mc, driveClient: dc,
		mailResend: DefaultInviteResendWindow, mailSlots: make(chan struct{}, inviteMailInFlight)}
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
