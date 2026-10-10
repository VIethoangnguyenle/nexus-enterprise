package domain

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"google.golang.org/protobuf/types/known/timestamppb"

	"ngac-platform/ngac"
	"ngac-platform/pkg/grpcauth"
	"ngac-platform/pkg/policyclient"
	pb "ngac-platform/proto/asset"
	policypb "ngac-platform/proto/policy"
	"ngac-platform/services/asset/internal/events"
	"ngac-platform/services/asset/internal/store"
)

// MaxReasonRunes bounds a request's reason and a rejection's reason.
const MaxReasonRunes = 1000

var validUrgency = map[string]bool{"low": true, "normal": true, "high": true, "urgent": true}

// AssetRequestService handles gRPC calls for the asset request/approve/assign/return flow.
type AssetRequestService struct {
	store       *store.Store
	policyRead  policypb.PolicyReadServiceClient
	policyWrite policypb.PolicyWriteServiceClient
	producer    events.Publisher
}

// NewAssetRequestService creates the asset request gRPC handler.
func NewAssetRequestService(s *store.Store, pr policypb.PolicyReadServiceClient, pw policypb.PolicyWriteServiceClient, p events.Publisher) *AssetRequestService {
	return &AssetRequestService{store: s, policyRead: pr, policyWrite: pw, producer: orDiscard(p)}
}

func (s *AssetRequestService) CreateRequest(ctx context.Context, req *pb.CreateAssetRequestReq) (*pb.AssetRequest, error) {
	req.Justification = strings.TrimSpace(req.Justification)
	if req.TypeId == "" || req.WorkspaceId == "" || req.Justification == "" {
		return nil, invalid("type_id, workspace_id, and justification are required")
	}
	urgency := req.Urgency
	if urgency == "" {
		urgency = "normal"
	}
	if utf8.RuneCountInString(req.Justification) > MaxReasonRunes {
		return nil, invalid("the reason is too long")
	}
	if !validUrgency[urgency] {
		return nil, invalid("urgency must be one of low, normal, high, urgent")
	}

	at, err := s.store.GetType(ctx, req.TypeId)
	if err != nil {
		return nil, lookupErr(err, "asset type")
	}

	// Check request permission on type OA
	if err := s.checkAccess(ctx, grpcauth.CallerFrom(ctx).NGACNodeID, at.NgacOAID, ngac.OpWrite); err != nil {
		return nil, err
	}

	// A request is filed in its type's workspace. Checked after authorization
	// so a caller without write on the type learns nothing about its workspace.
	if at.WorkspaceID != req.WorkspaceId {
		return nil, invalid("asset type does not belong to this workspace")
	}

	quantity := req.Quantity
	if quantity <= 0 {
		quantity = 1
	}

	assetReq := &store.AssetRequest{
		TypeID:        req.TypeId,
		WorkspaceID:   req.WorkspaceId,
		RequesterID:   grpcauth.CallerFrom(ctx).UserID,
		Status:        "pending",
		Justification: req.Justification,
		Quantity:      quantity,
		Urgency:       urgency,
	}
	if err := s.store.CreateRequest(ctx, assetReq); err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	// Emit Kafka event
	s.producer.PublishRequest(ctx, events.RequestEvent{
		RequestID:   assetReq.ID,
		TypeName:    at.Name,
		TypeID:      req.TypeId,
		RequesterID: grpcauth.CallerFrom(ctx).UserID,
		Status:      "pending",
		WorkspaceID: req.WorkspaceId,
	})

	created, err := s.store.GetRequest(ctx, assetReq.ID)
	if err != nil {
		return nil, fmt.Errorf("read back request: %w", err)
	}
	return requestToProto(created), nil
}

// ApproveRequest approves a pending request as the authenticated caller.
//
// With req.AssetId it also gives that asset to the requester, as one step: the
// approval, the hand-over and the history entry are written in one transaction,
// so a request is never approved without its asset nor an asset given without
// the request being closed. That needs approve (the decision) and manage (the
// hand-over) on the request's type OA; either missing refuses the whole thing.
func (s *AssetRequestService) ApproveRequest(ctx context.Context, req *pb.ApproveRequestReq) (*pb.AssetRequest, error) {
	who := grpcauth.CallerFrom(ctx)
	// Authorization first: nothing about the request is said to someone who may not decide it.
	assetReq, at, err := s.loadForDecision(ctx, req.RequestId, ngac.OpApprove)
	if err != nil {
		return nil, err
	}
	if assetReq.Status != "pending" {
		return nil, Refuse(ErrConflict, ReasonRequestNotOpen, "request is not pending")
	}
	// Cannot approve own request
	if assetReq.RequesterID == who.UserID {
		return nil, denied("cannot approve own request")
	}

	if req.AssetId != "" {
		if err := s.checkAccess(ctx, who.NGACNodeID, at.NgacOAID, ngac.OpManage); err != nil {
			return nil, err
		}
		if err := s.store.ApproveAndAssign(ctx, store.FulfilParams{
			RequestID: req.RequestId, AssetID: req.AssetId, ActorID: who.UserID, Comment: req.Comment,
		}); err != nil {
			return nil, storeErr(err, "approve and assign")
		}
		s.producer.PublishAssignment(ctx, events.AssignmentEvent{
			AssetID: req.AssetId, ToUserID: assetReq.RequesterID, Action: "assign",
			ActorID: who.UserID, WorkspaceID: assetReq.WorkspaceID,
		})
		s.publishDecision(ctx, assetReq, at, "fulfilled", who.UserID)
		return s.readBack(ctx, req.RequestId)
	}

	if err := s.store.UpdateRequestStatus(ctx, req.RequestId, "approved", who.UserID, req.Comment); err != nil {
		return nil, storeErr(err, "update request")
	}
	s.publishDecision(ctx, assetReq, at, "approved", who.UserID)
	return s.readBack(ctx, req.RequestId)
}

func (s *AssetRequestService) publishDecision(ctx context.Context, r *store.AssetRequest, at *store.AssetType, statusName, approverID string) {
	s.producer.PublishRequest(ctx, events.RequestEvent{
		RequestID:   r.ID,
		TypeName:    at.Name,
		TypeID:      r.TypeID,
		RequesterID: r.RequesterID,
		Status:      statusName,
		ApproverID:  approverID,
		WorkspaceID: r.WorkspaceID,
	})
}

func (s *AssetRequestService) readBack(ctx context.Context, requestID string) (*pb.AssetRequest, error) {
	updated, err := s.store.GetRequest(ctx, requestID)
	if err != nil {
		return nil, fmt.Errorf("read back request: %w", err)
	}
	return requestToProto(updated), nil
}

func (s *AssetRequestService) RejectRequest(ctx context.Context, req *pb.RejectRequestReq) (*pb.AssetRequest, error) {
	req.Reason = strings.TrimSpace(req.Reason)
	assetReq, at, err := s.loadForDecision(ctx, req.RequestId, ngac.OpApprove)
	if err != nil {
		return nil, err
	}
	if assetReq.Status != "pending" {
		return nil, Refuse(ErrConflict, ReasonRequestNotOpen, "request is not pending")
	}

	// The requester reads this; a rejection without a reason tells them nothing.
	if req.Reason == "" {
		return nil, invalid("a reason is required")
	}
	if utf8.RuneCountInString(req.Reason) > MaxReasonRunes {
		return nil, invalid("the reason is too long")
	}

	if err := s.store.UpdateRequestStatus(ctx, req.RequestId, "rejected", grpcauth.CallerFrom(ctx).UserID, req.Reason); err != nil {
		return nil, storeErr(err, "update request")
	}

	s.producer.PublishRequest(ctx, events.RequestEvent{
		RequestID:   req.RequestId,
		TypeName:    at.Name,
		TypeID:      assetReq.TypeID,
		RequesterID: assetReq.RequesterID,
		Status:      "rejected",
		ApproverID:  grpcauth.CallerFrom(ctx).UserID,
		WorkspaceID: assetReq.WorkspaceID,
	})

	updated, err := s.store.GetRequest(ctx, req.RequestId)
	if err != nil {
		return nil, fmt.Errorf("read back request: %w", err)
	}
	return requestToProto(updated), nil
}

// AssignAsset gives an asset to a request that was approved without one. It
// needs manage on the OA of the request's type; the asset must be available and
// of that type. The request, the asset and the history change together.
func (s *AssetRequestService) AssignAsset(ctx context.Context, req *pb.AssignAssetReq) (*pb.AssetRequest, error) {
	who := grpcauth.CallerFrom(ctx)
	assetReq, at, err := s.loadForDecision(ctx, req.RequestId, ngac.OpManage)
	if err != nil {
		return nil, err
	}
	if assetReq.Status != "approved" {
		return nil, conflict("request must be approved before assignment (current: %s)", assetReq.Status)
	}
	if req.AssetId == "" {
		return nil, invalid("asset_id is required")
	}
	if err := s.store.AssignApproved(ctx, store.FulfilParams{RequestID: req.RequestId, AssetID: req.AssetId, ActorID: who.UserID}); err != nil {
		return nil, storeErr(err, "assign")
	}
	s.producer.PublishAssignment(ctx, events.AssignmentEvent{
		AssetID: req.AssetId, ToUserID: assetReq.RequesterID, Action: "assign",
		ActorID: who.UserID, WorkspaceID: assetReq.WorkspaceID,
	})
	s.publishDecision(ctx, assetReq, at, "fulfilled", who.UserID)
	return s.readBack(ctx, req.RequestId)
}

// ReturnAsset takes an assigned asset back into stock and clears its holder.
// Its holder may return it, and so may anyone with manage on its type's OA.
func (s *AssetRequestService) ReturnAsset(ctx context.Context, req *pb.ReturnAssetReq) (*pb.Empty, error) {
	asset, err := s.store.GetAsset(ctx, req.AssetId)
	if err != nil || asset.Deleted {
		return nil, notFound("asset not found")
	}

	who := grpcauth.CallerFrom(ctx)
	isAssignedUser := asset.AssignedTo != nil && who.UserID != "" && *asset.AssignedTo == who.UserID
	if !isAssignedUser {
		if err := s.checkAccess(ctx, who.NGACNodeID, asset.TypeOAID, ngac.OpManage); err != nil {
			return nil, denied("only the assigned user or a manager can return this asset")
		}
	}
	if asset.State != "assigned" {
		return nil, conflict("only an assigned asset can be returned (current: %s)", asset.State)
	}

	previousUser := derefString(asset.AssignedTo)
	if err := s.store.ApplyTransition(ctx, &store.TransitionRecord{
		AssetID: req.AssetId, FromState: asset.State, ToState: "available", Action: "return", ActorID: who.UserID,
		// The holder read above; refused if the asset moved on before the lock.
		ExpectHolder: previousUser,
	}, nil); err != nil {
		return nil, storeErr(err, "return")
	}

	s.producer.PublishAssignment(ctx, events.AssignmentEvent{
		AssetID:     req.AssetId,
		AssetName:   asset.Name,
		FromUserID:  previousUser,
		Action:      "return",
		ActorID:     who.UserID,
		WorkspaceID: asset.WorkspaceID,
	})

	return &pb.Empty{}, nil
}

// Request visibility.
//
// A request is visible to the person who made it, and to whoever may decide
// it: approve on the request's type OA, which is exactly what ApproveRequest
// and RejectRequest check. Read on the asset tree is deliberately not enough —
// a request carries the requester's justification, which is addressed to the
// approvers, not to everyone who can browse the catalogue. Approve granted on
// the Assets or category OA reaches the type OAs beneath it, so workspace
// owners (who hold every operation on the Assets OA) see every request.

// ListRequests returns the requests in a workspace the caller may see.
// Visibility is pushed into the query so the total counts only those.
func (s *AssetRequestService) ListRequests(ctx context.Context, req *pb.ListRequestsReq) (*pb.AssetRequestList, error) {
	types, err := s.store.ListTypes(ctx, req.WorkspaceId)
	if err != nil {
		return nil, fmt.Errorf("list asset types: %w", err)
	}
	approvable, err := permittedTypeIDs(ctx, s.policyRead, grpcauth.CallerFrom(ctx).NGACNodeID, types, ngac.OpApprove)
	if err != nil {
		// Fail closed: an unreadable policy answer must not list anything.
		return nil, fmt.Errorf("batch access check: %w", err)
	}
	if grpcauth.CallerFrom(ctx).UserID == "" && len(approvable) == 0 {
		return &pb.AssetRequestList{}, nil
	}

	requests, total, err := s.store.ListRequests(ctx, store.ListRequestsFilter{
		WorkspaceID: req.WorkspaceId,
		UserID:      grpcauth.CallerFrom(ctx).UserID,
		Status:      req.Status,
		MineOnly:    req.MineOnly,
		Limit:       req.Limit,
		Offset:      req.Offset,
		Visibility:  &store.RequestVisibility{RequesterID: grpcauth.CallerFrom(ctx).UserID, TypeIDs: approvable},
	})
	if err != nil {
		return nil, fmt.Errorf("list requests: %w", err)
	}

	result := &pb.AssetRequestList{Total: total}
	flags, err := s.flagsFor(ctx, req.WorkspaceId, requests)
	if err != nil {
		return nil, err
	}
	for _, r := range requests {
		p := requestToProto(r)
		flags.apply(p)
		result.Requests = append(result.Requests, p)
	}
	return result, nil
}

// requestFlags says, per request, what the caller may do with it, so a screen
// offers only what the server would accept. They are hints: every operation
// checks again.
type requestFlags struct {
	callerID string
	held     map[string][]string // type ID -> operations held on its OA
}

func (s *AssetRequestService) flagsFor(ctx context.Context, workspaceID string, requests []*store.AssetRequest) (requestFlags, error) {
	f := requestFlags{callerID: grpcauth.CallerFrom(ctx).UserID, held: map[string][]string{}}
	if len(requests) == 0 {
		return f, nil
	}
	types, err := s.store.ListTypes(ctx, workspaceID)
	if err != nil {
		return f, fmt.Errorf("list asset types: %w", err)
	}
	f.held, err = heldOnTypes(ctx, s.policyRead, grpcauth.CallerFrom(ctx).NGACNodeID, types, []string{ngac.OpApprove, ngac.OpManage})
	if err != nil {
		return f, fmt.Errorf("batch access check: %w", err)
	}
	return f, nil
}

func (f requestFlags) holds(typeID, op string) bool {
	for _, o := range f.held[typeID] {
		if o == op {
			return true
		}
	}
	return false
}

func (f requestFlags) apply(r *pb.AssetRequest) {
	own := f.callerID != "" && r.RequesterId == f.callerID
	r.CanDecide = r.Status == "pending" && !own && f.holds(r.TypeId, ngac.OpApprove)
	r.CanAssign = (r.Status == "pending" || r.Status == "approved") && f.holds(r.TypeId, ngac.OpManage)
}

// GetRequest returns one request if the caller may see it (see ListRequests).
//
// A call without a caller is denied before the request is even looked up, so
// an anonymous caller cannot probe which request IDs exist.
func (s *AssetRequestService) GetRequest(ctx context.Context, req *pb.GetRequestReq) (*pb.AssetRequest, error) {
	who := grpcauth.CallerFrom(ctx)
	if who.UserID == "" && who.NGACNodeID == "" {
		return nil, errDenied(ngac.OpRead)
	}
	r, err := s.store.GetRequest(ctx, req.RequestId)
	if err != nil {
		return nil, lookupErr(err, "request")
	}
	if who.UserID == "" || r.RequesterID != who.UserID {
		at, err := s.store.GetType(ctx, r.TypeID)
		if err != nil {
			return nil, errDenied(ngac.OpApprove)
		}
		if err := authorize(ctx, s.policyRead, who.NGACNodeID, at.NgacOAID, ngac.OpApprove); err != nil {
			return nil, err
		}
	}
	p := requestToProto(r)
	flags, err := s.flagsFor(ctx, r.WorkspaceID, []*store.AssetRequest{r})
	if err != nil {
		return nil, err
	}
	flags.apply(p)
	return p, nil
}

// ============================================
// Helpers
// ============================================

func (s *AssetRequestService) checkAccess(ctx context.Context, userNodeID, objectNodeID, operation string) error {
	allowed, err := policyclient.New(s.policyRead).Check(ctx, userNodeID, objectNodeID, operation)
	if err != nil {
		return fmt.Errorf("access check failed: %w", err)
	}
	if !allowed {
		return denied("no %s access", operation)
	}
	return nil
}

func requestToProto(r *store.AssetRequest) *pb.AssetRequest {
	result := &pb.AssetRequest{
		Id:                r.ID,
		TypeId:            r.TypeID,
		TypeName:          r.TypeName,
		WorkspaceId:       r.WorkspaceID,
		RequesterId:       r.RequesterID,
		RequesterName:     r.RequesterName,
		Status:            r.Status,
		Justification:     r.Justification,
		Quantity:          r.Quantity,
		ApproverName:      r.ApproverName,
		ApproverComment:   r.ApproverComment,
		Urgency:           r.Urgency,
		AssignedAssetName: r.AssignedAssetName,
		CreatedAt:         timestamppb.New(r.CreatedAt),
		UpdatedAt:         timestamppb.New(r.UpdatedAt),
	}
	if r.ApproverID != nil {
		result.ApproverId = *r.ApproverID
	}
	if r.AssignedAssetID != nil {
		result.AssignedAssetId = *r.AssignedAssetID
	}
	return result
}
