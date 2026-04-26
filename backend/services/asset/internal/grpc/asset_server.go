package grpc

import (
	"context"
	"encoding/json"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "ngac-platform/proto/asset"
	policypb "ngac-platform/proto/policy"
	"ngac-platform/services/asset/internal/domain"
	"ngac-platform/services/asset/internal/events"
	"ngac-platform/services/asset/internal/store"
)

// AssetServer handles gRPC calls for asset CRUD and lifecycle management.
type AssetServer struct {
	pb.UnimplementedAssetServiceServer
	store        *store.Store
	policyClient policypb.PolicyServiceClient
	producer     *events.Producer
}

// NewAssetServer creates the asset gRPC handler.
func NewAssetServer(s *store.Store, pc policypb.PolicyServiceClient, p *events.Producer) *AssetServer {
	return &AssetServer{store: s, policyClient: pc, producer: p}
}

func (s *AssetServer) CreateAsset(ctx context.Context, req *pb.CreateAssetRequest) (*pb.Asset, error) {
	if req.Name == "" || req.TypeId == "" || req.WorkspaceId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "name, type_id, and workspace_id are required")
	}

	// Fetch type for schema validation and lifecycle initial state
	at, err := s.store.GetType(ctx, req.TypeId)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "asset type not found: %v", err)
	}

	// Check write permission on type's OA
	if err := s.checkAccess(ctx, req.UserNgacNodeId, at.NgacOAID, "write"); err != nil {
		return nil, err
	}

	// Validate custom fields against type schema
	fieldsJSON := json.RawMessage("{}")
	if req.CustomFields != nil {
		b, err := req.CustomFields.MarshalJSON()
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid custom_fields: %v", err)
		}
		fieldsJSON = b
	}
	if err := domain.ValidateCustomFields(at.FieldsSchema, fieldsJSON); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "field validation failed: %v", err)
	}

	// Get initial state from lifecycle
	var ld domain.LifecycleDefinition
	if err := json.Unmarshal(at.Lifecycle, &ld); err != nil {
		return nil, status.Errorf(codes.Internal, "parse lifecycle: %v", err)
	}

	// Create NGAC Object node for this asset
	ngacNode, err := s.policyClient.CreateNode(ctx, &policypb.CreateNodeRequest{
		Name:     fmt.Sprintf("Asset_%s", req.Name),
		NodeType: "O",
		Properties: map[string]string{
			"asset_type":   req.TypeId,
			"workspace_id": req.WorkspaceId,
		},
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "create asset NGAC node: %v", err)
	}

	// Assign asset O to type OA
	if at.NgacOAID != "" {
		s.policyClient.CreateAssignment(ctx, &policypb.CreateAssignmentRequest{
			ChildId: ngacNode.Id, ParentId: at.NgacOAID,
		})
	}

	asset := &store.Asset{
		Name:         req.Name,
		TypeID:       req.TypeId,
		WorkspaceID:  req.WorkspaceId,
		State:        ld.InitialState,
		CustomFields: fieldsJSON,
		NgacNodeID:   ngacNode.Id,
		CreatedBy:    req.UserId,
	}
	if err := s.store.CreateAsset(ctx, asset); err != nil {
		return nil, status.Errorf(codes.Internal, "create asset: %v", err)
	}

	// Re-read for complete data (type_name, etc.)
	created, err := s.store.GetAsset(ctx, asset.ID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "read back asset: %v", err)
	}
	return assetToProto(created), nil
}

func (s *AssetServer) GetAsset(ctx context.Context, req *pb.GetAssetRequest) (*pb.Asset, error) {
	asset, err := s.store.GetAsset(ctx, req.AssetId)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "asset not found: %v", err)
	}
	if asset.Deleted {
		return nil, status.Errorf(codes.NotFound, "asset has been deleted")
	}

	// Check read permission on asset's NGAC node
	if err := s.checkAccess(ctx, req.UserNgacNodeId, asset.NgacNodeID, "read"); err != nil {
		return nil, err
	}
	return assetToProto(asset), nil
}

func (s *AssetServer) ListAssets(ctx context.Context, req *pb.ListAssetsRequest) (*pb.AssetList, error) {
	assets, total, err := s.store.ListAssets(ctx, store.ListAssetsFilter{
		WorkspaceID: req.WorkspaceId,
		TypeID:      req.TypeId,
		State:       req.State,
		AssignedTo:  req.AssignedTo,
		Limit:       req.Limit,
		Offset:      req.Offset,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list assets: %v", err)
	}

	result := &pb.AssetList{Total: total}
	for _, a := range assets {
		result.Assets = append(result.Assets, assetToProto(a))
	}
	return result, nil
}

func (s *AssetServer) UpdateAsset(ctx context.Context, req *pb.UpdateAssetRequest) (*pb.Asset, error) {
	asset, err := s.store.GetAsset(ctx, req.AssetId)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "asset not found: %v", err)
	}

	if err := s.checkAccess(ctx, req.UserNgacNodeId, asset.NgacNodeID, "write"); err != nil {
		return nil, err
	}

	fieldsJSON := asset.CustomFields
	if req.CustomFields != nil {
		b, err := req.CustomFields.MarshalJSON()
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid custom_fields: %v", err)
		}
		fieldsJSON = b

		// Validate against type schema
		at, err := s.store.GetType(ctx, asset.TypeID)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "get type for validation: %v", err)
		}
		if err := domain.ValidateCustomFields(at.FieldsSchema, fieldsJSON); err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "field validation failed: %v", err)
		}
	}

	if err := s.store.UpdateAsset(ctx, req.AssetId, req.Name, fieldsJSON); err != nil {
		return nil, status.Errorf(codes.Internal, "update asset: %v", err)
	}

	updated, err := s.store.GetAsset(ctx, req.AssetId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "read back asset: %v", err)
	}
	return assetToProto(updated), nil
}

func (s *AssetServer) DeleteAsset(ctx context.Context, req *pb.DeleteAssetRequest) (*pb.Empty, error) {
	asset, err := s.store.GetAsset(ctx, req.AssetId)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "asset not found: %v", err)
	}

	if err := s.checkAccess(ctx, req.UserNgacNodeId, asset.NgacNodeID, "manage"); err != nil {
		return nil, err
	}

	if err := s.store.SoftDeleteAsset(ctx, req.AssetId); err != nil {
		return nil, status.Errorf(codes.Internal, "delete asset: %v", err)
	}

	// Remove NGAC assignments for the asset node
	if asset.NgacNodeID != "" {
		s.policyClient.DeleteNode(ctx, &policypb.DeleteNodeRequest{NodeId: asset.NgacNodeID})
	}

	return &pb.Empty{}, nil
}

// ============================================
// Lifecycle Operations
// ============================================

func (s *AssetServer) TransitionAsset(ctx context.Context, req *pb.TransitionRequest) (*pb.Asset, error) {
	asset, err := s.store.GetAsset(ctx, req.AssetId)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "asset not found: %v", err)
	}
	if asset.Deleted {
		return nil, status.Errorf(codes.FailedPrecondition, "cannot transition deleted asset")
	}

	// Load lifecycle from type
	at, err := s.store.GetType(ctx, asset.TypeID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get asset type: %v", err)
	}
	var ld domain.LifecycleDefinition
	if err := json.Unmarshal(at.Lifecycle, &ld); err != nil {
		return nil, status.Errorf(codes.Internal, "parse lifecycle: %v", err)
	}

	// Find the requested transition
	tr, found := domain.FindTransition(ld, asset.State, req.Action)
	if !found {
		return nil, status.Errorf(codes.FailedPrecondition,
			"no transition %q available from state %q", req.Action, asset.State)
	}

	// Check NGAC permission for the transition
	if err := s.checkAccess(ctx, req.UserNgacNodeId, asset.NgacNodeID, tr.NgacPermission); err != nil {
		return nil, err
	}

	// Execute state change
	if err := s.store.UpdateAssetState(ctx, req.AssetId, tr.ToState, nil); err != nil {
		return nil, status.Errorf(codes.Internal, "update state: %v", err)
	}

	// Record transition history
	s.store.InsertTransition(ctx, &store.TransitionRecord{
		AssetID:   req.AssetId,
		FromState: asset.State,
		ToState:   tr.ToState,
		Action:    req.Action,
		ActorID:   req.UserId,
		Comment:   req.Comment,
	})

	// Emit Kafka lifecycle event
	s.producer.PublishLifecycle(events.LifecycleEvent{
		AssetID:     req.AssetId,
		AssetName:   asset.Name,
		TypeName:    asset.TypeName,
		FromState:   asset.State,
		ToState:     tr.ToState,
		Action:      req.Action,
		ActorID:     req.UserId,
		WorkspaceID: asset.WorkspaceID,
	})

	updated, err := s.store.GetAsset(ctx, req.AssetId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "read back asset: %v", err)
	}
	return assetToProto(updated), nil
}

func (s *AssetServer) GetAvailableTransitions(ctx context.Context, req *pb.GetTransitionsRequest) (*pb.TransitionList, error) {
	asset, err := s.store.GetAsset(ctx, req.AssetId)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "asset not found: %v", err)
	}

	at, err := s.store.GetType(ctx, asset.TypeID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get asset type: %v", err)
	}
	var ld domain.LifecycleDefinition
	if err := json.Unmarshal(at.Lifecycle, &ld); err != nil {
		return nil, status.Errorf(codes.Internal, "parse lifecycle: %v", err)
	}

	allTransitions := domain.AvailableTransitions(ld, asset.State)
	result := &pb.TransitionList{CurrentState: asset.State}

	// Filter by NGAC permissions — only show transitions the user can execute
	for _, t := range allTransitions {
		resp, _ := s.policyClient.CheckAccess(ctx, &policypb.CheckAccessRequest{
			UserNodeId: req.UserNgacNodeId, ObjectNodeId: asset.NgacNodeID, Operation: t.NgacPermission,
		})
		if resp != nil && resp.Decision == "ALLOW" {
			result.Transitions = append(result.Transitions, &pb.AvailableTransition{
				Action:         t.Operation,
				ToState:        t.ToState,
				NgacPermission: t.NgacPermission,
			})
		}
	}
	return result, nil
}

func (s *AssetServer) GetAssetHistory(ctx context.Context, req *pb.GetHistoryRequest) (*pb.TransitionHistoryList, error) {
	// Check read access
	asset, err := s.store.GetAsset(ctx, req.AssetId)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "asset not found: %v", err)
	}
	if err := s.checkAccess(ctx, req.UserNgacNodeId, asset.NgacNodeID, "read"); err != nil {
		return nil, err
	}

	records, err := s.store.GetAssetHistory(ctx, req.AssetId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get history: %v", err)
	}

	result := &pb.TransitionHistoryList{}
	for _, r := range records {
		result.Records = append(result.Records, &pb.TransitionRecord{
			Id:        r.ID,
			AssetId:   r.AssetID,
			FromState: r.FromState,
			ToState:   r.ToState,
			Action:    r.Action,
			ActorId:   r.ActorID,
			ActorName: r.ActorName,
			Comment:   r.Comment,
			CreatedAt: timestamppb.New(r.CreatedAt),
		})
	}
	return result, nil
}

// ============================================
// Helpers
// ============================================

func (s *AssetServer) checkAccess(ctx context.Context, userNodeID, objectNodeID, operation string) error {
	resp, err := s.policyClient.CheckAccess(ctx, &policypb.CheckAccessRequest{
		UserNodeId: userNodeID, ObjectNodeId: objectNodeID, Operation: operation,
	})
	if err != nil {
		return status.Errorf(codes.Internal, "access check failed: %v", err)
	}
	if resp.Decision != "ALLOW" {
		return status.Errorf(codes.PermissionDenied, "no %s access", operation)
	}
	return nil
}

func assetToProto(a *store.Asset) *pb.Asset {
	result := &pb.Asset{
		Id:          a.ID,
		Name:        a.Name,
		TypeId:      a.TypeID,
		TypeName:    a.TypeName,
		WorkspaceId: a.WorkspaceID,
		State:       a.State,
		NgacNodeId:  a.NgacNodeID,
		CreatedBy:   a.CreatedBy,
		Deleted:     a.Deleted,
		CreatedAt:   timestamppb.New(a.CreatedAt),
		UpdatedAt:   timestamppb.New(a.UpdatedAt),
	}

	if a.AssignedTo != nil {
		result.AssignedToUserId = *a.AssignedTo
		result.AssignedToUsername = a.AssignedToUsername
	}

	// Convert custom_fields JSON to protobuf Struct
	if len(a.CustomFields) > 0 {
		var m map[string]any
		if err := json.Unmarshal(a.CustomFields, &m); err == nil {
			if s, err := structpb.NewStruct(m); err == nil {
				result.CustomFields = s
			}
		}
	}
	return result
}
