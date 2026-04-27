package grpc

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "ngac-platform/proto/asset"
	policypb "ngac-platform/proto/policy"
	"ngac-platform/services/asset/internal/events"
	"ngac-platform/services/asset/internal/store"
)

// AssetRequestServer handles gRPC calls for the asset request/approve/assign/return flow.
type AssetRequestServer struct {
	pb.UnimplementedAssetRequestServiceServer
	store        *store.Store
	policyRead  policypb.PolicyReadServiceClient
	policyWrite policypb.PolicyWriteServiceClient
	producer     *events.Producer
}

// NewAssetRequestServer creates the asset request gRPC handler.
func NewAssetRequestServer(s *store.Store, pr policypb.PolicyReadServiceClient, pw policypb.PolicyWriteServiceClient, p *events.Producer) *AssetRequestServer {
	return &AssetRequestServer{store: s, policyRead: pr, policyWrite: pw, producer: p}
}

func (s *AssetRequestServer) CreateRequest(ctx context.Context, req *pb.CreateAssetRequestReq) (*pb.AssetRequest, error) {
	if req.TypeId == "" || req.WorkspaceId == "" || req.Justification == "" {
		return nil, status.Errorf(codes.InvalidArgument, "type_id, workspace_id, and justification are required")
	}

	at, err := s.store.GetType(ctx, req.TypeId)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "asset type not found: %v", err)
	}

	// Check request permission on type OA
	if err := s.checkAccess(ctx, req.UserNgacNodeId, at.NgacOAID, "request"); err != nil {
		return nil, err
	}

	quantity := req.Quantity
	if quantity <= 0 {
		quantity = 1
	}

	assetReq := &store.AssetRequest{
		TypeID:        req.TypeId,
		WorkspaceID:   req.WorkspaceId,
		RequesterID:   req.UserId,
		Status:        "pending",
		Justification: req.Justification,
		Quantity:      quantity,
	}
	if err := s.store.CreateRequest(ctx, assetReq); err != nil {
		return nil, status.Errorf(codes.Internal, "create request: %v", err)
	}

	// Emit Kafka event
	s.producer.PublishRequest(events.RequestEvent{
		RequestID:   assetReq.ID,
		TypeName:    at.Name,
		TypeID:      req.TypeId,
		RequesterID: req.UserId,
		Status:      "pending",
		WorkspaceID: req.WorkspaceId,
	})

	created, err := s.store.GetRequest(ctx, assetReq.ID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "read back request: %v", err)
	}
	return requestToProto(created), nil
}

func (s *AssetRequestServer) ApproveRequest(ctx context.Context, req *pb.ApproveRequestReq) (*pb.AssetRequest, error) {
	assetReq, err := s.store.GetRequest(ctx, req.RequestId)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "request not found: %v", err)
	}
	if assetReq.Status != "pending" {
		return nil, status.Errorf(codes.FailedPrecondition, "request is not pending (current: %s)", assetReq.Status)
	}

	// Cannot approve own request
	if assetReq.RequesterID == req.UserId {
		return nil, status.Errorf(codes.PermissionDenied, "cannot approve own request")
	}

	at, err := s.store.GetType(ctx, assetReq.TypeID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get type: %v", err)
	}

	// Check approve permission on type OA
	if err := s.checkAccess(ctx, req.UserNgacNodeId, at.NgacOAID, "approve"); err != nil {
		return nil, err
	}

	if err := s.store.UpdateRequestStatus(ctx, req.RequestId, "approved", req.UserId, req.Comment); err != nil {
		return nil, status.Errorf(codes.Internal, "update request: %v", err)
	}

	s.producer.PublishRequest(events.RequestEvent{
		RequestID:   req.RequestId,
		TypeName:    at.Name,
		TypeID:      assetReq.TypeID,
		RequesterID: assetReq.RequesterID,
		Status:      "approved",
		ApproverID:  req.UserId,
		WorkspaceID: assetReq.WorkspaceID,
	})

	updated, err := s.store.GetRequest(ctx, req.RequestId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "read back request: %v", err)
	}
	return requestToProto(updated), nil
}

func (s *AssetRequestServer) RejectRequest(ctx context.Context, req *pb.RejectRequestReq) (*pb.AssetRequest, error) {
	assetReq, err := s.store.GetRequest(ctx, req.RequestId)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "request not found: %v", err)
	}
	if assetReq.Status != "pending" {
		return nil, status.Errorf(codes.FailedPrecondition, "request is not pending")
	}

	at, err := s.store.GetType(ctx, assetReq.TypeID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get type: %v", err)
	}

	if err := s.checkAccess(ctx, req.UserNgacNodeId, at.NgacOAID, "approve"); err != nil {
		return nil, err
	}

	if err := s.store.UpdateRequestStatus(ctx, req.RequestId, "rejected", req.UserId, req.Reason); err != nil {
		return nil, status.Errorf(codes.Internal, "update request: %v", err)
	}

	s.producer.PublishRequest(events.RequestEvent{
		RequestID:   req.RequestId,
		TypeName:    at.Name,
		TypeID:      assetReq.TypeID,
		RequesterID: assetReq.RequesterID,
		Status:      "rejected",
		ApproverID:  req.UserId,
		WorkspaceID: assetReq.WorkspaceID,
	})

	updated, err := s.store.GetRequest(ctx, req.RequestId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "read back request: %v", err)
	}
	return requestToProto(updated), nil
}

func (s *AssetRequestServer) AssignAsset(ctx context.Context, req *pb.AssignAssetReq) (*pb.AssetRequest, error) {
	assetReq, err := s.store.GetRequest(ctx, req.RequestId)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "request not found: %v", err)
	}
	if assetReq.Status != "approved" {
		return nil, status.Errorf(codes.FailedPrecondition, "request must be approved before assignment (current: %s)", assetReq.Status)
	}

	// Check if asset is already assigned
	assigned, err := s.store.IsAssetAssigned(ctx, req.AssetId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "check assignment: %v", err)
	}
	if assigned {
		return nil, status.Errorf(codes.FailedPrecondition, "asset is currently assigned to another user")
	}

	asset, err := s.store.GetAsset(ctx, req.AssetId)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "asset not found: %v", err)
	}

	// Check assign permission
	if err := s.checkAccess(ctx, req.UserNgacNodeId, asset.NgacNodeID, "assign"); err != nil {
		return nil, err
	}

	// Update asset: set assigned_to and transition to "assigned" state
	assignedTo := assetReq.RequesterID
	if err := s.store.UpdateAssetState(ctx, req.AssetId, "assigned", &assignedTo); err != nil {
		return nil, status.Errorf(codes.Internal, "assign asset: %v", err)
	}

	// Record transition
	s.store.InsertTransition(ctx, &store.TransitionRecord{
		AssetID:   req.AssetId,
		FromState: asset.State,
		ToState:   "assigned",
		Action:    "assign",
		ActorID:   req.UserId,
		Comment:   "Assigned via request " + req.RequestId,
	})

	// Fulfill the request
	if err := s.store.FulfillRequest(ctx, req.RequestId, req.AssetId); err != nil {
		return nil, status.Errorf(codes.Internal, "fulfill request: %v", err)
	}

	// Create NGAC assignment: requester → asset object (direct read access)
	// Find requester's NGAC node from users table — we use the policy graph for this
	// For simplicity, we grant access at the type OA level which is inherited
	s.producer.PublishAssignment(events.AssignmentEvent{
		AssetID:     req.AssetId,
		AssetName:   asset.Name,
		ToUserID:    assetReq.RequesterID,
		Action:      "assign",
		ActorID:     req.UserId,
		WorkspaceID: asset.WorkspaceID,
	})

	updated, err := s.store.GetRequest(ctx, req.RequestId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "read back request: %v", err)
	}
	return requestToProto(updated), nil
}

func (s *AssetRequestServer) ReturnAsset(ctx context.Context, req *pb.ReturnAssetReq) (*pb.Empty, error) {
	asset, err := s.store.GetAsset(ctx, req.AssetId)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "asset not found: %v", err)
	}

	// Either the assigned user or someone with manage permission can return
	isAssignedUser := asset.AssignedTo != nil && *asset.AssignedTo == req.UserId
	if !isAssignedUser {
		if err := s.checkAccess(ctx, req.UserNgacNodeId, asset.NgacNodeID, "manage"); err != nil {
			return nil, status.Errorf(codes.PermissionDenied, "only the assigned user or a manager can return this asset")
		}
	}

	previousUser := ""
	if asset.AssignedTo != nil {
		previousUser = *asset.AssignedTo
	}

	// Transition to "available" and clear assignment
	if err := s.store.UpdateAssetState(ctx, req.AssetId, "available", nil); err != nil {
		return nil, status.Errorf(codes.Internal, "update state: %v", err)
	}
	if err := s.store.ClearAssignment(ctx, req.AssetId); err != nil {
		return nil, status.Errorf(codes.Internal, "clear assignment: %v", err)
	}

	s.store.InsertTransition(ctx, &store.TransitionRecord{
		AssetID:   req.AssetId,
		FromState: asset.State,
		ToState:   "available",
		Action:    "return",
		ActorID:   req.UserId,
	})

	s.producer.PublishAssignment(events.AssignmentEvent{
		AssetID:     req.AssetId,
		AssetName:   asset.Name,
		FromUserID:  previousUser,
		Action:      "return",
		ActorID:     req.UserId,
		WorkspaceID: asset.WorkspaceID,
	})

	return &pb.Empty{}, nil
}

func (s *AssetRequestServer) ListRequests(ctx context.Context, req *pb.ListRequestsReq) (*pb.AssetRequestList, error) {
	requests, total, err := s.store.ListRequests(ctx, store.ListRequestsFilter{
		WorkspaceID: req.WorkspaceId,
		UserID:      req.UserId,
		Status:      req.Status,
		MineOnly:    req.MineOnly,
		Limit:       req.Limit,
		Offset:      req.Offset,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list requests: %v", err)
	}

	result := &pb.AssetRequestList{Total: total}
	for _, r := range requests {
		result.Requests = append(result.Requests, requestToProto(r))
	}
	return result, nil
}

func (s *AssetRequestServer) GetRequest(ctx context.Context, req *pb.GetRequestReq) (*pb.AssetRequest, error) {
	r, err := s.store.GetRequest(ctx, req.RequestId)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "request not found: %v", err)
	}
	return requestToProto(r), nil
}

// ============================================
// Helpers
// ============================================

func (s *AssetRequestServer) checkAccess(ctx context.Context, userNodeID, objectNodeID, operation string) error {
	resp, err := s.policyRead.CheckAccess(ctx, &policypb.CheckAccessRequest{
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

func requestToProto(r *store.AssetRequest) *pb.AssetRequest {
	result := &pb.AssetRequest{
		Id:              r.ID,
		TypeId:          r.TypeID,
		TypeName:        r.TypeName,
		WorkspaceId:     r.WorkspaceID,
		RequesterId:     r.RequesterID,
		RequesterName:   r.RequesterName,
		Status:          r.Status,
		Justification:   r.Justification,
		Quantity:        r.Quantity,
		ApproverName:    r.ApproverName,
		ApproverComment: r.ApproverComment,
		CreatedAt:       timestamppb.New(r.CreatedAt),
		UpdatedAt:       timestamppb.New(r.UpdatedAt),
	}
	if r.ApproverID != nil {
		result.ApproverId = *r.ApproverID
	}
	if r.AssignedAssetID != nil {
		result.AssignedAssetId = *r.AssignedAssetID
	}
	return result
}
