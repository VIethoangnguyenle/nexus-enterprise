package grpc

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"ngac-platform/ngac"
	"ngac-platform/pkg/grpcauth"
	"ngac-platform/pkg/provision"
	pb "ngac-platform/proto/asset"
	policypb "ngac-platform/proto/policy"
	"ngac-platform/services/asset/internal/domain"
	"ngac-platform/services/asset/internal/store"
)

// AssetTypeServer handles gRPC calls for asset type management.
type AssetTypeServer struct {
	pb.UnimplementedAssetTypeServiceServer
	store       *store.Store
	policyRead  policypb.PolicyReadServiceClient
	policyWrite policypb.PolicyWriteServiceClient
}

// NewAssetTypeServer creates the asset type gRPC handler.
func NewAssetTypeServer(s *store.Store, pr policypb.PolicyReadServiceClient, pw policypb.PolicyWriteServiceClient) *AssetTypeServer {
	return &AssetTypeServer{store: s, policyRead: pr, policyWrite: pw}
}

func (s *AssetTypeServer) CreateType(ctx context.Context, req *pb.CreateTypeRequest) (*pb.AssetType, error) {
	if req.Name == "" || req.WorkspaceId == "" || req.Category == "" {
		return nil, status.Errorf(codes.InvalidArgument, "name, workspace_id, and category are required")
	}

	// Defining asset types administers the workspace's asset tree. Checked
	// before anything below writes to the graph.
	if err := s.authorizeCreateType(ctx, grpcauth.CallerFrom(ctx).NGACNodeID, req.WorkspaceId); err != nil {
		return nil, err
	}

	// Validate and prepare fields schema
	fieldsSchema := json.RawMessage("{}")
	if req.FieldsSchema != "" {
		if err := domain.ValidateSchema(json.RawMessage(req.FieldsSchema)); err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid fields_schema: %v", err)
		}
		fieldsSchema = json.RawMessage(req.FieldsSchema)
	}

	// Validate or use default lifecycle
	ld := domain.DefaultLifecycle()
	if req.Lifecycle != nil && len(req.Lifecycle.States) > 0 {
		ld = protoToLifecycle(req.Lifecycle)
	}
	if err := domain.ValidateLifecycle(ld); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid lifecycle: %v", err)
	}
	lifecycleJSON, err := json.Marshal(ld)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "marshal lifecycle: %v", err)
	}

	// The type's ID is chosen first so its OA can be named by it. Provisioning
	// is a run of graph writes followed by the row; a failure at any step removes
	// the nodes this call created.
	at := &store.AssetType{
		ID:           uuid.New().String(),
		Name:         req.Name,
		Description:  req.Description,
		Category:     req.Category,
		WorkspaceID:  req.WorkspaceId,
		FieldsSchema: fieldsSchema,
		Lifecycle:    lifecycleJSON,
	}

	// NGAC: create or find workspace Assets OA hierarchy
	prov := provision.NewCreator(s.policyWrite)
	ngacOAID, err := s.ensureNGACHierarchy(ctx, prov, req.WorkspaceId, req.Category, at.ID, req.Name)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "ngac setup: %v", prov.Fail(ctx, err))
	}
	at.NgacOAID = ngacOAID
	if err := s.store.CreateType(ctx, at); err != nil {
		return nil, status.Errorf(codes.Internal, "create type: %v", prov.Fail(ctx, err))
	}
	prov.Done()

	return assetTypeToProto(at), nil
}

// authorizeCreateType requires manage on the workspace's Assets OA.
//
// The first type created in a workspace is what creates the Assets OA, so
// until it exists there is nothing to check manage on. In that case — and only
// when the policy service positively reports the node absent, not when the
// lookup fails — the check falls back to manage on the workspace's Mgmt OA.
// That right is held by the workspace Owners UA, which is the UA
// ensureNGACHierarchy then grants every operation on the new Assets OA; so the
// fallback admits exactly the people who would hold the right afterwards.
func (s *AssetTypeServer) authorizeCreateType(ctx context.Context, userNodeID, workspaceID string) error {
	if userNodeID == "" {
		return errDenied(ngac.OpManage)
	}
	assetsOA, found, err := resolveOA(ctx, s.policyRead, ngac.AssetsOAName(ngac.WorkspaceID(workspaceID)))
	if err != nil {
		return errDenied(ngac.OpManage)
	}
	if found {
		return authorize(ctx, s.policyRead, userNodeID, assetsOA, ngac.OpManage)
	}
	return authorizeOnNamedOA(ctx, s.policyRead, userNodeID, ngac.MgmtOAName(ngac.WorkspaceID(workspaceID)), ngac.OpManage)
}

// GetType returns one asset type to a caller holding read on its workspace's
// Assets OA.
//
// The caller comes from the context (see package grpcauth); a call without
// one is denied before the type is looked up.
func (s *AssetTypeServer) GetType(ctx context.Context, req *pb.GetTypeRequest) (*pb.AssetType, error) {
	userNodeID := grpcauth.CallerFrom(ctx).NGACNodeID
	if userNodeID == "" {
		return nil, errDenied(ngac.OpRead)
	}
	// A type that does not exist and one the caller holds nothing on look alike.
	at, err := s.store.GetType(ctx, req.TypeId)
	if err != nil {
		return nil, errDenied(ngac.OpRead)
	}
	held, err := heldOnTypes(ctx, s.policyRead, userNodeID, []*store.AssetType{at}, typeOps)
	if err != nil || len(held[at.ID]) == 0 {
		return nil, errDenied(ngac.OpRead)
	}
	return typeForCaller(at, held[at.ID]), nil
}

// typeForCaller is a type as the caller may see it: with the operations they
// hold, and with the counts of its assets only if they may read them. Holding
// only `write` lists the type (to ask for it) without telling how many assets
// of it exist or are free.
func typeForCaller(at *store.AssetType, ops []string) *pb.AssetType {
	p := assetTypeToProto(at)
	p.Permissions = ops
	for _, op := range ops {
		if op == ngac.OpRead {
			return p
		}
	}
	p.AssetCount, p.AvailableCount = 0, 0
	return p
}

// typeOps are the operations a screen cares about for each type, in the order
// they are reported.
var typeOps = []string{ngac.OpRead, ngac.OpWrite, ngac.OpApprove, ngac.OpManage}

// ListTypes returns the workspace's asset types the caller holds any of read,
// write, approve or manage on, each with the operations held. A type is decided
// on its own OA, so a grant on one type (or on a category) lists that type
// without handing over the whole catalogue; a grant on the Assets OA reaches
// every type beneath it. The caller comes from the context, as for GetType.
//
// CanManage says whether the caller may define types and edit their fields.
func (s *AssetTypeServer) ListTypes(ctx context.Context, req *pb.ListTypesRequest) (*pb.AssetTypeList, error) {
	userNodeID := grpcauth.CallerFrom(ctx).NGACNodeID
	if userNodeID == "" {
		return nil, errDenied(ngac.OpRead)
	}
	types, err := s.store.ListTypes(ctx, req.WorkspaceId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list types: %v", err)
	}
	held, err := heldOnTypes(ctx, s.policyRead, userNodeID, types, typeOps)
	if err != nil {
		// Fail closed: an unreadable policy answer must not list anything.
		return nil, status.Errorf(codes.Internal, "batch access check: %v", err)
	}
	result := &pb.AssetTypeList{CanManage: s.authorizeCreateType(ctx, userNodeID, req.WorkspaceId) == nil}
	for _, at := range types {
		ops := held[at.ID]
		if len(ops) == 0 {
			continue
		}
		result.Types = append(result.Types, typeForCaller(at, ops))
	}
	return result, nil
}

// UpdateTypeSchema changes a type's custom-field schema. It requires manage on
// the Assets OA of the type's workspace — the same right as defining the type.
func (s *AssetTypeServer) UpdateTypeSchema(ctx context.Context, req *pb.UpdateTypeSchemaRequest) (*pb.AssetType, error) {
	at, err := s.store.GetType(ctx, req.TypeId)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "type not found: %v", err)
	}
	if err := authorizeOnNamedOA(ctx, s.policyRead, grpcauth.CallerFrom(ctx).NGACNodeID, ngac.AssetsOAName(ngac.WorkspaceID(at.WorkspaceID)), ngac.OpManage); err != nil {
		return nil, err
	}
	if err := domain.ValidateSchema(json.RawMessage(req.FieldsSchema)); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid schema: %v", err)
	}
	if err := s.store.UpdateTypeSchema(ctx, req.TypeId, json.RawMessage(req.FieldsSchema)); err != nil {
		return nil, status.Errorf(codes.Internal, "update schema: %v", err)
	}
	// Re-read without GetType's guard: the caller was just authorized to manage
	// this type, and the request names them in a field GetType does not read.
	updated, err := s.store.GetType(ctx, req.TypeId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "read back type: %v", err)
	}
	return assetTypeToProto(updated), nil
}

// ensureNGACHierarchy creates the NGAC node hierarchy for a new asset type and
// returns the type's OA:
//
//	workspace PC → {ws}_Assets → {ws}_Category_{category} → {ws}_Type_{type id}
//
// Assets are not nodes. The OA of the type is what every asset of the type is
// authorized on, and a grant on the Assets or category OA reaches it from above.
// The tree hangs under the workspace's own policy class only: an access needs
// the user to reach every policy class the object reaches, so a second class
// that no workspace UA is assigned to would make the whole tree unreachable.
//
// Safe to run again: the Assets and category OAs are found if they exist, and
// everything this call creates is recorded in prov for rollback.
func (s *AssetTypeServer) ensureNGACHierarchy(ctx context.Context, prov *provision.Creator, workspaceID, category, typeID, typeName string) (string, error) {
	var pcNodeID string
	row := s.store.DB().QueryRow(ctx, "SELECT COALESCE(ngac_pc_id, '') FROM workspaces WHERE id = $1", workspaceID)
	if err := row.Scan(&pcNodeID); err != nil {
		return "", fmt.Errorf("look up workspace %s: %w", workspaceID, err)
	}
	if pcNodeID == "" {
		return "", fmt.Errorf("workspace %s has no policy class", workspaceID)
	}
	wsID := ngac.WorkspaceID(workspaceID)

	// Ensure {ws}_Assets OA under the workspace PC
	assetsOA, err := prov.EnsureNode(ctx, s.policyRead, &policypb.CreateNodeRequest{
		Name: ngac.AssetsOAName(wsID), NodeType: ngac.TypeOA,
		Properties: map[string]string{"workspace_id": workspaceID},
	})
	if err != nil {
		return "", fmt.Errorf("create assets OA: %w", err)
	}
	if err := prov.Assign(ctx, assetsOA.Id, pcNodeID); err != nil {
		return "", fmt.Errorf("assign assets OA under workspace PC: %w", err)
	}

	// Ensure category OA under Assets OA
	categoryOA, err := prov.EnsureNode(ctx, s.policyRead, &policypb.CreateNodeRequest{
		Name: ngac.AssetCategoryOAName(wsID, sanitizeName(category)), NodeType: ngac.TypeOA,
		Properties: map[string]string{"category": category, "workspace_id": workspaceID},
	})
	if err != nil {
		return "", fmt.Errorf("create category OA: %w", err)
	}
	if err := prov.Assign(ctx, categoryOA.Id, assetsOA.Id); err != nil {
		return "", fmt.Errorf("assign category OA under assets OA: %w", err)
	}

	// Type OA under category OA, named by the type's ID
	typeOA, err := prov.EnsureNode(ctx, s.policyRead, &policypb.CreateNodeRequest{
		Name: ngac.AssetTypeOAName(wsID, ngac.AssetTypeID(typeID)), NodeType: ngac.TypeOA,
		Properties: map[string]string{"asset_type_id": typeID, ngac.PropDisplayName: typeName, "workspace_id": workspaceID},
	})
	if err != nil {
		return "", fmt.Errorf("create type OA: %w", err)
	}
	if err := prov.Assign(ctx, typeOA.Id, categoryOA.Id); err != nil {
		return "", fmt.Errorf("assign type OA under category OA: %w", err)
	}

	// Grant the workspace OWNERS full access to assets.
	//
	// This used to loop over every UA under the PC and grant all operations to
	// each one. That set includes the workspace Members UA, the tenant member
	// UA, department UAs and every channel's members UA — so every one of them
	// received manage and approve over all of the workspace's assets. The Owners
	// UA is named by the workspace ID, so it is looked up directly.
	owners, err := s.policyRead.FindNodeByName(ctx, &policypb.FindNodeByNameRequest{
		Name: ngac.OwnersUAName(wsID), NodeType: ngac.TypeUA,
	})
	switch {
	case err == nil && owners.GetId() != "":
		if err := prov.Associate(ctx, owners.Id, assetsOA.Id, ngac.AllOwnerOps()); err != nil {
			return "", fmt.Errorf("grant owners access to assets OA: %w", err)
		}
	case err != nil && status.Code(err) != codes.NotFound:
		return "", fmt.Errorf("look up owners UA: %w", err)
	default:
		slog.Warn("owners UA not found; assets have no owner grant",
			"workspace_id", workspaceID, "expected_ua", ngac.OwnersUAName(wsID))
	}

	return typeOA.Id, nil
}

func assetTypeToProto(at *store.AssetType) *pb.AssetType {
	result := &pb.AssetType{
		Id:             at.ID,
		Name:           at.Name,
		Description:    at.Description,
		Category:       at.Category,
		WorkspaceId:    at.WorkspaceID,
		FieldsSchema:   string(at.FieldsSchema),
		NgacOaId:       at.NgacOAID,
		AssetCount:     at.AssetCount,
		AvailableCount: at.AvailableCount,
		CreatedAt:      timestamppb.New(at.CreatedAt),
		UpdatedAt:      timestamppb.New(at.UpdatedAt),
	}

	var ld domain.LifecycleDefinition
	if err := json.Unmarshal(at.Lifecycle, &ld); err == nil {
		result.Lifecycle = lifecycleToProto(ld)
	}
	return result
}

func lifecycleToProto(ld domain.LifecycleDefinition) *pb.LifecycleDefinition {
	result := &pb.LifecycleDefinition{
		States:       ld.States,
		InitialState: ld.InitialState,
	}
	for _, t := range ld.Transitions {
		result.Transitions = append(result.Transitions, &pb.TransitionRule{
			FromState:      t.FromState,
			ToState:        t.ToState,
			Operation:      t.Operation,
			NgacPermission: t.NgacPermission,
		})
	}
	return result
}

func protoToLifecycle(pb *pb.LifecycleDefinition) domain.LifecycleDefinition {
	ld := domain.LifecycleDefinition{
		States:       pb.States,
		InitialState: pb.InitialState,
	}
	for _, t := range pb.Transitions {
		ld.Transitions = append(ld.Transitions, domain.TransitionRule{
			FromState:      t.FromState,
			ToState:        t.ToState,
			Operation:      t.Operation,
			NgacPermission: t.NgacPermission,
		})
	}
	return ld
}

func sanitizeName(s string) string {
	result := make([]byte, 0, len(s))
	for _, c := range []byte(s) {
		if c == ' ' || c == '-' {
			result = append(result, '_')
		} else if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' {
			result = append(result, c)
		}
	}
	return string(result)
}
