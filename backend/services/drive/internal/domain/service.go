package domain

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"

	"ngac-platform/ngac"
	"ngac-platform/pkg/grpcauth"
	"ngac-platform/pkg/policyclient"
	"ngac-platform/pkg/provision"
	"ngac-platform/pkg/realtime"
	docpb "ngac-platform/proto/document"
	pb "ngac-platform/proto/drive"
	policypb "ngac-platform/proto/policy"
	"ngac-platform/services/drive/internal/store"
)

// Service is the drive's business logic: what a caller may do with the items of
// a workspace's drive, and the policy-graph and storage work that doing it
// takes. The gRPC server and the REST handlers are thin adapters over it. It
// speaks the pb messages as plain data; it knows nothing of gRPC status codes
// (see errors.go for how a refusal is classified).
type Service struct {
	store       *store.Store
	policyRead  policypb.PolicyReadServiceClient
	policy      *policyclient.Client
	policyWrite policypb.PolicyWriteServiceClient
	docStorage  docpb.DocumentStorageServiceClient
	emitter     realtime.Emitter
}

// NewService creates the drive service with all required dependencies.
func NewService(
	st *store.Store,
	pr policypb.PolicyReadServiceClient,
	pw policypb.PolicyWriteServiceClient,
	ds docpb.DocumentStorageServiceClient,
) *Service {
	return &Service{
		store:       st,
		policyRead:  pr,
		policy:      policyclient.New(pr),
		policyWrite: pw,
		docStorage:  ds,
	}
}

// SetEmitter installs the sink for realtime change events. Without one the
// server runs silently.
func (s *Service) SetEmitter(e realtime.Emitter) { s.emitter = e }

// announce tells connected browsers that item changed. Call it only once the
// change is committed: an event for state that is not yet readable sends
// clients to refetch what they already have. The tenant and actor are the
// verified caller's. ParentID is the containing folder ("" for the drive's top
// level); adjust may refine the event (moves name the folder they left).
func (s *Service) announce(ctx context.Context, kind string, item *store.DriveItem, adjust ...func(*realtime.Event)) {
	if s.emitter == nil || item == nil {
		return
	}
	e := realtime.For(ctx, realtime.DomainDrive, kind, item.WorkspaceID, item.ID)
	if item.ParentID != nil {
		e.ParentID = *item.ParentID
	}
	for _, f := range adjust {
		f(&e)
	}
	s.emitter.Emit(e)
}

// checkAccess verifies NGAC access, returning an error if denied.
func (s *Service) checkAccess(ctx context.Context, userNodeID, objectNodeID, operation string) error {
	if ok, _ := s.policy.Check(ctx, userNodeID, objectNodeID, operation); !ok {
		return denied("access denied")
	}
	return nil
}

// checkAccessOnNamedOA verifies NGAC access on a well-known OA identified by
// name (always built with a helper from package ngac). The name is resolved to
// its node first; a caller with no identity, or an OA that cannot be resolved,
// denies — there is nothing that could grant the right.
func (s *Service) checkAccessOnNamedOA(ctx context.Context, userNodeID, oaName, operation string) error {
	if userNodeID == "" {
		return denied("access denied")
	}
	node, err := s.policyRead.FindNodeByName(ctx, &policypb.FindNodeByNameRequest{
		Name: oaName, NodeType: ngac.TypeOA,
	})
	if err != nil || node.GetId() == "" {
		return denied("access denied")
	}
	return s.checkAccess(ctx, userNodeID, node.GetId(), operation)
}

// liveFolder loads a folder that can be opened or filled: it exists, is active
// (a trashed or still-uploading item is not a place anyone can browse into) and
// belongs to workspaceID. An empty workspaceID skips the workspace check.
//
// Every refusal is NotFound, so a caller learns nothing about a folder in
// another workspace or in the trash beyond "not there".
func (s *Service) liveFolder(ctx context.Context, id, workspaceID string) (*store.DriveItem, error) {
	item, err := s.store.GetItem(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("load folder: %w", err)
	}
	if item == nil || item.Status != "active" || item.ItemType != "folder" ||
		(workspaceID != "" && item.WorkspaceID != workspaceID) {
		return nil, notFound("folder not found")
	}
	return item, nil
}

// itemToProto converts a store.DriveItem to a protobuf DriveItem.
func itemToProto(item *store.DriveItem) *pb.DriveItem {
	if item == nil {
		return nil
	}
	p := &pb.DriveItem{
		Id:             item.ID,
		WorkspaceId:    item.WorkspaceID,
		DriveContext:   item.DriveContext,
		DriveContextId: item.DriveContextID,
		ItemType:       item.ItemType,
		Name:           item.Name,
		NgacNodeId:     item.NGACNodeID,
		OwnerId:        item.OwnerID,
		Status:         item.Status,
		CreatedAt:      timestamppb.New(item.CreatedAt),
		UpdatedAt:      timestamppb.New(item.UpdatedAt),
	}
	if item.ParentID != nil {
		p.ParentId = *item.ParentID
	}
	if item.MimeType != nil {
		p.MimeType = *item.MimeType
	}
	if item.SizeBytes != nil {
		p.SizeBytes = *item.SizeBytes
	}
	if item.ObjectKey != nil {
		p.ObjectKey = *item.ObjectKey
	}
	return p
}

// CreateFolder creates a new folder in the drive hierarchy.
func (s *Service) CreateFolder(ctx context.Context, req *pb.CreateFolderRequest) (*pb.DriveItem, error) {
	driveCtx := req.DriveContext
	if driveCtx == "" {
		driveCtx = "workspace"
	}

	// Determine parent NGAC OA for the new folder
	var parentNGACID string
	var parent *store.DriveItem
	if req.ParentId != "" {
		var err error
		parent, err = s.liveFolder(ctx, req.ParentId, req.WorkspaceId)
		if err != nil {
			return nil, err
		}
		if err := s.checkAccess(ctx, grpcauth.CallerFrom(ctx).NGACNodeID, parent.NGACNodeID, ngac.OpWrite); err != nil {
			return nil, err
		}
		parentNGACID = parent.NGACNodeID
	} else {
		root, err := s.ensureRoot(ctx, req.WorkspaceId, driveCtx, req.DriveContextId, grpcauth.CallerFrom(ctx).NGACNodeID)
		if err != nil {
			return nil, err
		}
		parentNGACID = root.NGACNodeID
	}

	// The folder's ID is chosen first so its OA can be named by it. A name taken
	// from the folder would be shared by every folder called "Reports", in this
	// workspace and in other tenants', and the graph resolves nodes by exact name.
	itemID := uuid.New().String()

	// Create NGAC OA node for the folder. A failure at any later step removes it.
	prov := provision.NewCreator(s.policyWrite)
	folderNode, err := prov.Node(ctx, &policypb.CreateNodeRequest{
		Name: ngac.FolderNodeName(ngac.FolderID(itemID)), NodeType: ngac.TypeOA,
		Properties: map[string]string{
			"type": "drive_folder", "workspace_id": req.WorkspaceId, ngac.PropDisplayName: req.Name,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("create folder node: %w", err)
	}

	// Assign folder OA under parent OA (inherits permissions). Without this
	// edge the folder reaches no PC and every check on it denies.
	if err := prov.Assign(ctx, folderNode.Id, parentNGACID); err != nil {
		return nil, fmt.Errorf("assign folder under parent: %w", prov.Fail(ctx, err))
	}

	// Determine scope OA: inherit from parent, or use own OA for root-level folders
	var scopeOAID string
	if parent != nil && parent.ScopeOAID != "" {
		scopeOAID = parent.ScopeOAID
	}
	if scopeOAID == "" {
		scopeOAID = folderNode.Id
	}

	item := &store.DriveItem{
		ID:             itemID,
		WorkspaceID:    req.WorkspaceId,
		DriveContext:   driveCtx,
		DriveContextID: req.DriveContextId,
		ParentID:       store.NilIfEmpty(req.ParentId),
		ItemType:       "folder",
		Name:           req.Name,
		NGACNodeID:     folderNode.Id,
		ScopeOAID:      scopeOAID,
		OwnerID:        grpcauth.CallerFrom(ctx).NGACNodeID,
		Status:         "active",
	}
	if err := s.store.InsertItem(ctx, item); err != nil {
		return nil, fmt.Errorf("insert folder: %w", prov.Fail(ctx, err))
	}
	prov.Done()

	slog.Info("folder created", "id", item.ID, "name", req.Name)
	s.announce(ctx, realtime.KindCreated, item)
	return itemToProto(item), nil
}

// ListFolder returns the contents of a folder with NGAC filtering.
func (s *Service) ListFolder(ctx context.Context, req *pb.ListFolderRequest) (*pb.DriveItemList, error) {
	driveCtx := req.DriveContext
	if driveCtx == "" {
		driveCtx = "workspace"
	}

	var parentID *string
	if req.FolderId != "" {
		parentID = &req.FolderId
		// Check read access on the folder itself
		folder, err := s.liveFolder(ctx, req.FolderId, req.WorkspaceId)
		if err != nil {
			return nil, err
		}
		if err := s.checkAccess(ctx, grpcauth.CallerFrom(ctx).NGACNodeID, folder.NGACNodeID, ngac.OpRead); err != nil {
			return nil, err
		}
	}

	items, err := s.store.ListChildren(ctx, parentID, req.WorkspaceId, driveCtx, req.DriveContextId)
	if err != nil {
		return nil, fmt.Errorf("list folder: %w", err)
	}

	// Filter by NGAC access. One batch call rather than one per item: a folder
	// of 200 files used to mean 200 sequential round-trips to the policy
	// service, which dominated the latency of listing anything.
	objectIDs := make([]string, 0, len(items))
	for _, item := range items {
		objectIDs = append(objectIDs, item.NGACNodeID)
	}

	var visible []*pb.DriveItem
	if len(objectIDs) > 0 {
		batch, err := s.policy.BatchCheckCaller(ctx, objectIDs, []string{ngac.OpRead})
		if err != nil {
			// Fail closed: an unreadable policy answer must not list everything.
			return nil, fmt.Errorf("batch access check: %w", err)
		}
		for _, item := range items {
			if batch.Has(item.NGACNodeID, ngac.OpRead) {
				visible = append(visible, itemToProto(item))
			}
		}
	}

	result := &pb.DriveItemList{Items: visible}

	// Build breadcrumb if inside a subfolder
	if req.FolderId != "" {
		crumbs, err := s.store.GetBreadcrumb(ctx, req.FolderId)
		if err != nil {
			return nil, fmt.Errorf("load breadcrumb: %w", err)
		}
		for _, c := range crumbs {
			result.Breadcrumb = append(result.Breadcrumb, &pb.BreadcrumbEntry{Id: c.ID, Name: c.Name})
		}
	}

	return result, nil
}

// GetItem returns a single drive item after NGAC read check.
func (s *Service) GetItem(ctx context.Context, req *pb.GetItemRequest) (*pb.DriveItem, error) {
	item, err := s.store.GetItem(ctx, req.ItemId)
	// A trashed item is gone as far as this call is concerned; it is reached
	// again only through RestoreItem. (A pending upload stays readable: the
	// uploader looks it up between CreateFile and ConfirmFile.)
	if err != nil || item == nil || item.Status == "trashed" {
		return nil, notFound("item not found")
	}
	if err := s.checkAccess(ctx, grpcauth.CallerFrom(ctx).NGACNodeID, item.NGACNodeID, ngac.OpRead); err != nil {
		return nil, err
	}
	return itemToProto(item), nil
}

// ensureRoot finds or auto-creates the root drive folder for a workspace/channel context.
// This self-heals workspaces created before drive tables existed.
func (s *Service) ensureRoot(ctx context.Context, workspaceID, driveCtx, driveCtxID, userNodeID string) (*store.DriveItem, error) {
	// Normalise the context once, and use the same value to look up and to
	// store.
	//
	// These used to disagree: the root was written with the workspace ID
	// substituted for an empty context, while the lookup ran with the empty
	// value it was handed. The lookup therefore never matched the root — but
	// FindRootByContext selects on "top-level folder in this context" with no
	// further discriminator, so as soon as a user created a folder at the top
	// level, that folder was returned as the drive root. Everything created at
	// the top level afterwards was parented under it in the NGAC graph and
	// inherited its permissions, including any share placed on it.
	rootCtxID := driveCtxID
	if rootCtxID == "" {
		rootCtxID = workspaceID
	}

	root, err := s.store.FindRootByContext(ctx, workspaceID, driveCtx, rootCtxID)
	if err != nil {
		return nil, fmt.Errorf("find root: %w", err)
	}
	if root != nil {
		return root, nil
	}

	// Auto-create root. It roots on the workspace's Documents OA, which the
	// workspace records when it is created; the OA is never worked out from node
	// names (a scan for "Documents"/"Docs" with a first-OA fallback depended on
	// the order the graph returned children in, and could pick a folder somebody
	// had named "Docs").
	prov := provision.NewCreator(s.policyWrite)
	ngacNodeID, err := s.rootOA(ctx, prov, workspaceID)
	if err != nil {
		return nil, err
	}

	item := &store.DriveItem{
		ID:             uuid.New().String(),
		WorkspaceID:    workspaceID,
		DriveContext:   driveCtx,
		DriveContextID: rootCtxID,
		ItemType:       "folder",
		Name:           "Root",
		NGACNodeID:     ngacNodeID,
		ScopeOAID:      ngacNodeID,
		OwnerID:        userNodeID,
		Status:         "active",
		IsRoot:         true,
	}
	if err := s.store.InsertItem(ctx, item); err != nil {
		cause := fmt.Errorf("insert root: %w", err)
		rbErr := prov.Fail(ctx, cause)
		if errors.Is(err, store.ErrRootExists) {
			// Another request created the root first. Whatever this one
			// created for it is gone; use the winner's.
			if winner, ferr := s.store.FindRootByContext(ctx, workspaceID, driveCtx, rootCtxID); ferr == nil && winner != nil {
				return winner, nil
			}
		}
		return nil, fmt.Errorf("%w", rbErr)
	}
	prov.Done()

	slog.Info("auto-created drive root", "workspace", workspaceID, "context", driveCtx, "root_id", item.ID)
	return item, nil
}

// rootOA returns the OA a workspace's drive root hangs on: its recorded
// Documents OA. A workspace with none recorded (one created outside the
// workspace service) gets a DriveRoot OA of its own, named by the workspace ID,
// found again on the next call rather than created twice. Nodes this creates are
// recorded in prov so a later failure removes them.
func (s *Service) rootOA(ctx context.Context, prov *provision.Creator, workspaceID string) (string, error) {
	docsOA, err := s.store.GetWorkspaceDocumentsOAID(ctx, workspaceID)
	if err != nil {
		return "", fmt.Errorf("look up documents OA: %w", err)
	}
	if docsOA != "" {
		return docsOA, nil
	}

	node, err := prov.EnsureNode(ctx, s.policyRead, &policypb.CreateNodeRequest{
		Name: ngac.DriveRootName(ngac.WorkspaceID(workspaceID)), NodeType: ngac.TypeOA,
	})
	if err != nil {
		return "", fmt.Errorf("create root node: %w", err)
	}

	// Assign DriveRoot OA under workspace PC so files inherit access associations
	pcID, err := s.store.GetWorkspacePCID(ctx, workspaceID)
	if err == nil && pcID != "" {
		if err := prov.Assign(ctx, node.Id, pcID); err != nil {
			return "", fmt.Errorf("assign drive root under workspace PC: %w", prov.Fail(ctx, err))
		}
	}
	return node.Id, nil
}
