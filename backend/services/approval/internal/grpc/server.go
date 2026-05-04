// Package grpc implements the ApprovalService gRPC handler.
// Each method: validate → delegate to domain → map error → respond.
package grpc

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "ngac-platform/proto/approval"
	"ngac-platform/services/approval/internal/domain"
)

// Server implements the ApprovalService gRPC API.
type Server struct {
	pb.UnimplementedApprovalServiceServer
	svc *domain.Service
}

// NewServer creates a gRPC handler with domain service dependency.
func NewServer(svc *domain.Service) *Server {
	return &Server{svc: svc}
}

// mapError translates domain sentinel errors to gRPC status codes.
func mapError(err error) error {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, domain.ErrAccessDenied):
		return status.Error(codes.PermissionDenied, err.Error())
	case errors.Is(err, domain.ErrAlreadyExists):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, domain.ErrInvalidInput):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, domain.ErrStepNotActive):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, domain.ErrRequestCompleted):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, domain.ErrNoMatchingTemplate):
		return status.Error(codes.NotFound, err.Error())
	default:
		return status.Errorf(codes.Internal, "internal: %v", err)
	}
}

// --- Proto conversion helpers ---

func requestToProto(r *domain.Request) *pb.ApprovalRequest {
	pr := &pb.ApprovalRequest{
		Id:           r.ID,
		EntityType:   r.EntityType,
		EntityId:     r.EntityID,
		TemplateId:   r.TemplateID,
		TemplateName: r.TemplateName,
		CurrentStep:  int32(r.CurrentStep),
		Status:       r.Status,
		ScopeOaId:    r.ScopeOAID,
		DepartmentId: r.DepartmentID,
		CreatedBy:    r.CreatedBy,
		CreatedAt:    timestamppb.New(r.CreatedAt),
	}
	if r.CompletedAt != nil {
		pr.CompletedAt = timestamppb.New(*r.CompletedAt)
	}
	return pr
}

func auditToProto(e *domain.AuditEntry) *pb.AuditEntry {
	return &pb.AuditEntry{
		Id:          e.ID,
		RequestId:   e.RequestID,
		Action:      e.Action,
		ActorNodeId: e.ActorNodeID,
		StepOrder:   int32(e.StepOrder),
		DetailJson:  e.DetailJSON,
		IpAddress:   e.IPAddress,
		CreatedAt:   timestamppb.New(e.CreatedAt),
	}
}

// --- Query RPCs ---

// GetPending returns all pending assignments for the requesting user.
func (s *Server) GetPending(ctx context.Context, req *pb.GetPendingRequest) (*pb.ApprovalList, error) {
	if req.UserNodeId == "" {
		return nil, status.Error(codes.InvalidArgument, "user_node_id required")
	}

	items, err := s.svc.GetPending(ctx, req.UserNodeId)
	if err != nil {
		return nil, mapError(err)
	}

	result := &pb.ApprovalList{}
	for _, item := range items {
		result.Requests = append(result.Requests, requestToProto(item.Request))
	}
	return result, nil
}

// GetHistory returns actioned assignments with cursor paging.
func (s *Server) GetHistory(ctx context.Context, req *pb.GetHistoryRequest) (*pb.ApprovalList, error) {
	if req.UserNodeId == "" {
		return nil, status.Error(codes.InvalidArgument, "user_node_id required")
	}

	items, nextCursor, err := s.svc.GetHistory(ctx, req.UserNodeId, req.Cursor, int(req.Limit))
	if err != nil {
		return nil, mapError(err)
	}

	result := &pb.ApprovalList{NextCursor: nextCursor}
	for _, item := range items {
		result.Requests = append(result.Requests, requestToProto(item.Request))
	}
	return result, nil
}

// GetMyRequests returns requests created by the user.
func (s *Server) GetMyRequests(ctx context.Context, req *pb.GetMyRequestsRequest) (*pb.ApprovalList, error) {
	if req.UserNodeId == "" {
		return nil, status.Error(codes.InvalidArgument, "user_node_id required")
	}

	items, nextCursor, err := s.svc.GetMyRequests(ctx, req.UserNodeId, req.Cursor, int(req.Limit))
	if err != nil {
		return nil, mapError(err)
	}

	result := &pb.ApprovalList{NextCursor: nextCursor}
	for _, item := range items {
		result.Requests = append(result.Requests, requestToProto(item))
	}
	return result, nil
}

// GetDepartmentRequests returns requests visible via scope-based access.
func (s *Server) GetDepartmentRequests(ctx context.Context, req *pb.GetDepartmentRequestsRequest) (*pb.ApprovalList, error) {
	if req.UserNodeId == "" {
		return nil, status.Error(codes.InvalidArgument, "user_node_id required")
	}

	items, nextCursor, err := s.svc.GetDepartmentRequests(ctx, req.UserNodeId, req.Cursor, int(req.Limit))
	if err != nil {
		return nil, mapError(err)
	}

	result := &pb.ApprovalList{NextCursor: nextCursor}
	for _, item := range items {
		result.Requests = append(result.Requests, requestToProto(item))
	}
	return result, nil
}

// GetAuditLog returns the audit trail for a request.
func (s *Server) GetAuditLog(ctx context.Context, req *pb.GetAuditLogRequest) (*pb.AuditLogList, error) {
	if req.RequestId == "" {
		return nil, status.Error(codes.InvalidArgument, "request_id required")
	}

	entries, err := s.svc.GetAuditLog(ctx, req.RequestId)
	if err != nil {
		return nil, mapError(err)
	}

	result := &pb.AuditLogList{}
	for _, e := range entries {
		result.Entries = append(result.Entries, auditToProto(e))
	}
	return result, nil
}
