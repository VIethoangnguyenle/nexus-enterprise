package grpc

import (
	"context"
	"encoding/json"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"ngac-platform/ngac"
	pb "ngac-platform/proto/asset"
	policypb "ngac-platform/proto/policy"
	"ngac-platform/services/asset/internal/domain"
	"ngac-platform/services/asset/internal/store"
)

// AssetTypeServer handles gRPC calls for asset type management.
type AssetTypeServer struct {
	pb.UnimplementedAssetTypeServiceServer
	store        *store.Store
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

func (s *AssetTypeServer) GetType(ctx context.Context, req *pb.GetTypeRequest) (*pb.AssetType, error) {
	at, err := s.store.GetType(ctx, req.TypeId)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "type not found: %v", err)
	}
	return assetTypeToProto(at), nil
}

func (s *AssetTypeServer) ListTypes(ctx context.Context, req *pb.ListTypesRequest) (*pb.AssetTypeList, error) {
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

func (s *AssetTypeServer) UpdateTypeSchema(ctx context.Context, req *pb.UpdateTypeSchemaRequest) (*pb.AssetType, error) {
	if err := domain.ValidateSchema(json.RawMessage(req.FieldsSchema)); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid schema: %v", err)
	}
	if err := s.store.UpdateTypeSchema(ctx, req.TypeId, json.RawMessage(req.FieldsSchema)); err != nil {
		return nil, status.Errorf(codes.Internal, "update schema: %v", err)
	}
	return s.GetType(ctx, &pb.GetTypeRequest{TypeId: req.TypeId})
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
		Name: "PC_AssetManagement", NodeType: "PC",
	})
	if err != nil {
		// Create it
		assetsPC, err = s.policyWrite.CreateNode(ctx, &policypb.CreateNodeRequest{
			Name: "PC_AssetManagement", NodeType: "PC",
			Properties: map[string]string{"scope": "global"},
		})
		if err != nil {
			return "", fmt.Errorf("create PC_AssetManagement: %w", err)
		}
	}

	// Ensure {ws}_Assets OA under PC_AssetManagement
	assetsOAName := fmt.Sprintf("%s_Assets", wsName)
	assetsOA, err := s.policyRead.FindNodeByName(ctx, &policypb.FindNodeByNameRequest{
		Name: assetsOAName, NodeType: "OA",
	})
	if err != nil {
		assetsOA, err = s.policyWrite.CreateNode(ctx, &policypb.CreateNodeRequest{
			Name: assetsOAName, NodeType: "OA",
			Properties: map[string]string{"workspace_id": workspaceID},
		})
		if err != nil {
			return "", fmt.Errorf("create %s OA: %w", assetsOAName, err)
		}
		s.policyWrite.CreateAssignment(ctx, &policypb.CreateAssignmentRequest{
			ChildId: assetsOA.Id, ParentId: assetsPC.Id,
		})
	}

	// Ensure category OA under Assets OA
	categoryOAName := fmt.Sprintf("%s_%s", wsName, sanitizeName(category))
	categoryOA, err := s.policyRead.FindNodeByName(ctx, &policypb.FindNodeByNameRequest{
		Name: categoryOAName, NodeType: "OA",
	})
	if err != nil {
		categoryOA, err = s.policyWrite.CreateNode(ctx, &policypb.CreateNodeRequest{
			Name: categoryOAName, NodeType: "OA",
			Properties: map[string]string{"category": category, "workspace_id": workspaceID},
		})
		if err != nil {
			return "", fmt.Errorf("create category OA %s: %w", categoryOAName, err)
		}
		s.policyWrite.CreateAssignment(ctx, &policypb.CreateAssignmentRequest{
			ChildId: categoryOA.Id, ParentId: assetsOA.Id,
		})
	}

	// Create type OA under category OA
	typeOAName := fmt.Sprintf("%s_%s", wsName, sanitizeName(typeName))
	typeOA, err := s.policyWrite.CreateNode(ctx, &policypb.CreateNodeRequest{
		Name: typeOAName, NodeType: "OA",
		Properties: map[string]string{"asset_type": typeName, "workspace_id": workspaceID},
	})
	if err != nil {
		return "", fmt.Errorf("create type OA %s: %w", typeOAName, err)
	}
	s.policyWrite.CreateAssignment(ctx, &policypb.CreateAssignmentRequest{
		ChildId: typeOA.Id, ParentId: categoryOA.Id,
	})

	// Also assign to workspace PC if available for cross-PC visibility
	if pcNodeID != "" {
		s.policyWrite.CreateAssignment(ctx, &policypb.CreateAssignmentRequest{
			ChildId: assetsOA.Id, ParentId: pcNodeID,
		})

		// Grant workspace owners full access to assets
		children, _ := s.policyRead.GetChildren(ctx, &policypb.GetChildrenRequest{NodeId: pcNodeID})
		if children != nil {
			for _, n := range children.Nodes {
				if n.NodeType == ngac.TypeUA {
					allOps := []string{"read", "write", "approve", "assign", "manage", "dispose", "request"}
					s.policyWrite.CreateAssociation(ctx, &policypb.CreateAssociationRequest{
						UaId: n.Id, OaId: assetsOA.Id, Operations: allOps,
					})
				}
			}
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
