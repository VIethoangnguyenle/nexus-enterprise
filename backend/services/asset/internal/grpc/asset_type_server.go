package grpc

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"ngac-platform/ngac"
	"ngac-platform/pkg/grpcauth"
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

	// NGAC: create or find workspace Assets OA hierarchy
	ngacOAID, err := s.ensureNGACHierarchy(ctx, req.WorkspaceId, req.Category, req.Name)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "ngac setup: %v", err)
	}

	at := &store.AssetType{
		Name:         req.Name,
		Description:  req.Description,
		Category:     req.Category,
		WorkspaceID:  req.WorkspaceId,
		FieldsSchema: fieldsSchema,
		Lifecycle:    lifecycleJSON,
		NgacOAID:     ngacOAID,
	}
	if err := s.store.CreateType(ctx, at); err != nil {
		return nil, status.Errorf(codes.Internal, "create type: %v", err)
	}

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
	assetsOA, found, err := resolveOA(ctx, s.policyRead, ngac.AssetsOAName(workspaceID))
	if err != nil {
		return errDenied(ngac.OpManage)
	}
	if found {
		return authorize(ctx, s.policyRead, userNodeID, assetsOA, ngac.OpManage)
	}
	return authorizeOnNamedOA(ctx, s.policyRead, userNodeID, ngac.MgmtOAName(workspaceID), ngac.OpManage)
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
	at, err := s.store.GetType(ctx, req.TypeId)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "type not found: %v", err)
	}
	if err := authorizeOnNamedOA(ctx, s.policyRead, userNodeID, ngac.AssetsOAName(at.WorkspaceID), ngac.OpRead); err != nil {
		return nil, err
	}
	return assetTypeToProto(at), nil
}

// ListTypes returns a workspace's asset types to a caller holding read on its
// Assets OA. The caller comes from the context, as for GetType.
//
// A workspace whose Assets OA does not exist has no types the caller could be
// authorized for, so it lists nothing rather than refusing — that is the state
// of every workspace before its first type is created.
func (s *AssetTypeServer) ListTypes(ctx context.Context, req *pb.ListTypesRequest) (*pb.AssetTypeList, error) {
	userNodeID := grpcauth.CallerFrom(ctx).NGACNodeID
	if userNodeID == "" {
		return nil, errDenied(ngac.OpRead)
	}
	assetsOA, found, err := resolveOA(ctx, s.policyRead, ngac.AssetsOAName(req.WorkspaceId))
	if err != nil {
		return nil, errDenied(ngac.OpRead)
	}
	if !found {
		return &pb.AssetTypeList{}, nil
	}
	if err := authorize(ctx, s.policyRead, userNodeID, assetsOA, ngac.OpRead); err != nil {
		return nil, err
	}

	types, err := s.store.ListTypes(ctx, req.WorkspaceId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list types: %v", err)
	}
	result := &pb.AssetTypeList{}
	for _, at := range types {
		result.Types = append(result.Types, assetTypeToProto(at))
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
	if err := authorizeOnNamedOA(ctx, s.policyRead, grpcauth.CallerFrom(ctx).NGACNodeID, ngac.AssetsOAName(at.WorkspaceID), ngac.OpManage); err != nil {
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

// ensureNGACHierarchy creates the NGAC node hierarchy for a new asset type:
// PC_AssetManagement → {ws}_Assets → {ws}_{category} → {ws}_{typeName}
func (s *AssetTypeServer) ensureNGACHierarchy(ctx context.Context, workspaceID, category, typeName string) (string, error) {
	// Get workspace name and PC node ID from the database
	var wsName, pcNodeID string
	row := s.store.DB().QueryRow(ctx, "SELECT name, ngac_pc_id FROM workspaces WHERE id = $1", workspaceID)
	if err := row.Scan(&wsName, &pcNodeID); err != nil {
		wsName = "WS"
		pcNodeID = ""
	}

	// Ensure PC_AssetManagement exists
	assetsPC, err := s.policyRead.FindNodeByName(ctx, &policypb.FindNodeByNameRequest{
		Name: ngac.NodePCAssetManagement, NodeType: ngac.TypePC,
	})
	if err != nil {
		// Create it
		assetsPC, err = s.policyWrite.CreateNode(ctx, &policypb.CreateNodeRequest{
			Name: ngac.NodePCAssetManagement, NodeType: ngac.TypePC,
			Properties: map[string]string{"scope": "global"},
		})
		if err != nil {
			return "", fmt.Errorf("create PC_AssetManagement: %w", err)
		}
	}

	// Ensure {ws}_Assets OA under PC_AssetManagement
	assetsOAName := ngac.AssetsOAName(workspaceID)
	assetsOA, err := s.policyRead.FindNodeByName(ctx, &policypb.FindNodeByNameRequest{
		Name: assetsOAName, NodeType: ngac.TypeOA,
	})
	if err != nil {
		assetsOA, err = s.policyWrite.CreateNode(ctx, &policypb.CreateNodeRequest{
			Name: assetsOAName, NodeType: ngac.TypeOA,
			Properties: map[string]string{"workspace_id": workspaceID},
		})
		if err != nil {
			return "", fmt.Errorf("create %s OA: %w", assetsOAName, err)
		}
		if _, err := s.policyWrite.CreateAssignment(ctx, &policypb.CreateAssignmentRequest{
			ChildId: assetsOA.Id, ParentId: assetsPC.Id,
		}); err != nil {
			return "", fmt.Errorf("assign assets OA under PC_AssetManagement: %w", err)
		}
	}

	// Ensure category OA under Assets OA
	categoryOAName := ngac.AssetCategoryOAName(workspaceID, sanitizeName(category))
	categoryOA, err := s.policyRead.FindNodeByName(ctx, &policypb.FindNodeByNameRequest{
		Name: categoryOAName, NodeType: ngac.TypeOA,
	})
	if err != nil {
		categoryOA, err = s.policyWrite.CreateNode(ctx, &policypb.CreateNodeRequest{
			Name: categoryOAName, NodeType: ngac.TypeOA,
			Properties: map[string]string{"category": category, "workspace_id": workspaceID},
		})
		if err != nil {
			return "", fmt.Errorf("create category OA %s: %w", categoryOAName, err)
		}
		if _, err := s.policyWrite.CreateAssignment(ctx, &policypb.CreateAssignmentRequest{
			ChildId: categoryOA.Id, ParentId: assetsOA.Id,
		}); err != nil {
			return "", fmt.Errorf("assign category OA under assets OA: %w", err)
		}
	}

	// Create type OA under category OA
	typeOAName := ngac.AssetTypeOAName(workspaceID, sanitizeName(typeName))
	typeOA, err := s.policyWrite.CreateNode(ctx, &policypb.CreateNodeRequest{
		Name: typeOAName, NodeType: ngac.TypeOA,
		Properties: map[string]string{"asset_type": typeName, "workspace_id": workspaceID},
	})
	if err != nil {
		return "", fmt.Errorf("create type OA %s: %w", typeOAName, err)
	}
	if _, err := s.policyWrite.CreateAssignment(ctx, &policypb.CreateAssignmentRequest{
		ChildId: typeOA.Id, ParentId: categoryOA.Id,
	}); err != nil {
		return "", fmt.Errorf("assign type OA under category OA: %w", err)
	}

	// Also assign to workspace PC if available for cross-PC visibility
	if pcNodeID != "" {
		if _, err := s.policyWrite.CreateAssignment(ctx, &policypb.CreateAssignmentRequest{
			ChildId: assetsOA.Id, ParentId: pcNodeID,
		}); err != nil {
			return "", fmt.Errorf("assign assets OA under workspace PC: %w", err)
		}

		// Grant the workspace OWNERS full access to assets.
		//
		// This used to loop over every UA under the PC and grant all operations
		// to each one. That set includes the workspace Members UA, the tenant
		// member UA, department UAs and every channel's members UA — so every
		// one of them received manage and approve over all of the workspace's
		// assets. Match the Owners UA by name instead.
		ownersUAName := ngac.OwnersUAName(workspaceID)
		children, err := s.policyRead.GetChildren(ctx, &policypb.GetChildrenRequest{NodeId: pcNodeID})
		if err != nil {
			return "", fmt.Errorf("read workspace PC children: %w", err)
		}
		granted := false
		for _, n := range children.GetNodes() {
			if n.NodeType != ngac.TypeUA || n.Name != ownersUAName {
				continue
			}
			if _, err := s.policyWrite.CreateAssociation(ctx, &policypb.CreateAssociationRequest{
				UaId: n.Id, OaId: assetsOA.Id, Operations: ngac.AllOwnerOps(),
			}); err != nil {
				return "", fmt.Errorf("grant owners access to assets OA: %w", err)
			}
			granted = true
			break
		}
		if !granted {
			slog.Warn("owners UA not found under workspace PC; assets have no owner grant",
				"workspace_id", workspaceID, "expected_ua", ownersUAName)
		}
	}

	return typeOA.Id, nil
}

func assetTypeToProto(at *store.AssetType) *pb.AssetType {
	result := &pb.AssetType{
		Id:           at.ID,
		Name:         at.Name,
		Description:  at.Description,
		Category:     at.Category,
		WorkspaceId:  at.WorkspaceID,
		FieldsSchema: string(at.FieldsSchema),
		NgacOaId:     at.NgacOAID,
		AssetCount:   at.AssetCount,
		CreatedAt:    timestamppb.New(at.CreatedAt),
		UpdatedAt:    timestamppb.New(at.UpdatedAt),
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
