// Package grpc is the asset service's gRPC transport. It adapts the asset
// services' APIs to the domain and turns the domain's refusals into gRPC
// statuses. It holds no business logic.
package grpc

import (
	"context"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"ngac-platform/pkg/grpcutil"
	pb "ngac-platform/proto/asset"
	policypb "ngac-platform/proto/policy"
	"ngac-platform/services/asset/internal/domain"
	"ngac-platform/services/asset/internal/events"
	"ngac-platform/services/asset/internal/store"
)

// The reasons a refusal carries, beside its status code, as an ErrorInfo detail.
const (
	ReasonAssetUnavailable = domain.ReasonAssetUnavailable
	ReasonRequestNotOpen   = domain.ReasonRequestNotOpen
	ReasonWrongType        = domain.ReasonWrongType
	ReasonNotAMember       = domain.ReasonNotAMember
	ReasonSameHolder       = domain.ReasonSameHolder
	ReasonStateChanged     = domain.ReasonStateChanged
)

// mapError turns the domain's classified refusals into gRPC statuses, with the
// refusal's reason as an ErrorInfo detail; anything else is a generic Internal
// whose detail stays in the log (see grpcutil.Status).
func mapError(err error) error {
	if err == nil {
		return nil
	}
	mapped := grpcutil.Status(err,
		grpcutil.Mapping{Is: domain.ErrConflict, Code: codes.FailedPrecondition},
		grpcutil.Mapping{Is: domain.ErrUnauthenticated, Code: codes.Unauthenticated},
		grpcutil.Mapping{Is: domain.ErrUnavailable, Code: codes.Unavailable},
	)
	reason := domain.Reason(err)
	if reason == "" {
		return mapped
	}
	st := status.Convert(mapped)
	if d, derr := st.WithDetails(&errdetails.ErrorInfo{Reason: reason, Domain: "asset"}); derr == nil {
		st = d
	}
	return st.Err()
}

// AssetServer serves AssetServer over the domain service.
type AssetServer struct {
	pb.UnimplementedAssetServiceServer
	svc *domain.AssetService
}

// NewAssetServer creates the gRPC server.
func NewAssetServer(s *store.Store, pr policypb.PolicyReadServiceClient, p events.Publisher) *AssetServer {
	return ServeAssetService(domain.NewAssetService(s, pr, p))
}

func (s *AssetServer) CreateAsset(ctx context.Context, req *pb.CreateAssetRequest) (*pb.Asset, error) {
	resp, err := s.svc.CreateAsset(ctx, req)
	return resp, mapError(err)
}

func (s *AssetServer) GetAsset(ctx context.Context, req *pb.GetAssetRequest) (*pb.Asset, error) {
	resp, err := s.svc.GetAsset(ctx, req)
	return resp, mapError(err)
}

func (s *AssetServer) ListAssets(ctx context.Context, req *pb.ListAssetsRequest) (*pb.AssetList, error) {
	resp, err := s.svc.ListAssets(ctx, req)
	return resp, mapError(err)
}

func (s *AssetServer) UpdateAsset(ctx context.Context, req *pb.UpdateAssetRequest) (*pb.Asset, error) {
	resp, err := s.svc.UpdateAsset(ctx, req)
	return resp, mapError(err)
}

func (s *AssetServer) DeleteAsset(ctx context.Context, req *pb.DeleteAssetRequest) (*pb.Empty, error) {
	resp, err := s.svc.DeleteAsset(ctx, req)
	return resp, mapError(err)
}

func (s *AssetServer) TransitionAsset(ctx context.Context, req *pb.TransitionRequest) (*pb.Asset, error) {
	resp, err := s.svc.TransitionAsset(ctx, req)
	return resp, mapError(err)
}

func (s *AssetServer) GetAvailableTransitions(ctx context.Context, req *pb.GetTransitionsRequest) (*pb.TransitionList, error) {
	resp, err := s.svc.GetAvailableTransitions(ctx, req)
	return resp, mapError(err)
}

func (s *AssetServer) GetAssetHistory(ctx context.Context, req *pb.GetHistoryRequest) (*pb.TransitionHistoryList, error) {
	resp, err := s.svc.GetAssetHistory(ctx, req)
	return resp, mapError(err)
}

func (s *AssetServer) HandOverAsset(ctx context.Context, req *pb.HandOverRequest) (*pb.Asset, error) {
	resp, err := s.svc.HandOverAsset(ctx, req)
	return resp, mapError(err)
}

func (s *AssetServer) GetSummary(ctx context.Context, req *pb.GetSummaryRequest) (*pb.AssetSummary, error) {
	resp, err := s.svc.GetSummary(ctx, req)
	return resp, mapError(err)
}

func (s *AssetServer) ListActivity(ctx context.Context, req *pb.ListActivityRequest) (*pb.ActivityList, error) {
	resp, err := s.svc.ListActivity(ctx, req)
	return resp, mapError(err)
}

// AssetTypeServer serves AssetTypeServer over the domain service.
type AssetTypeServer struct {
	pb.UnimplementedAssetTypeServiceServer
	svc *domain.AssetTypeService
}

// NewAssetTypeServer creates the gRPC server.
func NewAssetTypeServer(s *store.Store, pr policypb.PolicyReadServiceClient, pw policypb.PolicyWriteServiceClient) *AssetTypeServer {
	return ServeAssetTypeService(domain.NewAssetTypeService(s, pr, pw))
}

func (s *AssetTypeServer) CreateType(ctx context.Context, req *pb.CreateTypeRequest) (*pb.AssetType, error) {
	resp, err := s.svc.CreateType(ctx, req)
	return resp, mapError(err)
}

func (s *AssetTypeServer) GetType(ctx context.Context, req *pb.GetTypeRequest) (*pb.AssetType, error) {
	resp, err := s.svc.GetType(ctx, req)
	return resp, mapError(err)
}

func (s *AssetTypeServer) ListTypes(ctx context.Context, req *pb.ListTypesRequest) (*pb.AssetTypeList, error) {
	resp, err := s.svc.ListTypes(ctx, req)
	return resp, mapError(err)
}

func (s *AssetTypeServer) UpdateTypeSchema(ctx context.Context, req *pb.UpdateTypeSchemaRequest) (*pb.AssetType, error) {
	resp, err := s.svc.UpdateTypeSchema(ctx, req)
	return resp, mapError(err)
}

// AssetRequestServer serves AssetRequestServer over the domain service.
type AssetRequestServer struct {
	pb.UnimplementedAssetRequestServiceServer
	svc *domain.AssetRequestService
}

// NewAssetRequestServer creates the gRPC server.
func NewAssetRequestServer(s *store.Store, pr policypb.PolicyReadServiceClient, pw policypb.PolicyWriteServiceClient, p events.Publisher) *AssetRequestServer {
	return ServeAssetRequestService(domain.NewAssetRequestService(s, pr, pw, p))
}

func (s *AssetRequestServer) CreateRequest(ctx context.Context, req *pb.CreateAssetRequestReq) (*pb.AssetRequest, error) {
	resp, err := s.svc.CreateRequest(ctx, req)
	return resp, mapError(err)
}

func (s *AssetRequestServer) ApproveRequest(ctx context.Context, req *pb.ApproveRequestReq) (*pb.AssetRequest, error) {
	resp, err := s.svc.ApproveRequest(ctx, req)
	return resp, mapError(err)
}

func (s *AssetRequestServer) RejectRequest(ctx context.Context, req *pb.RejectRequestReq) (*pb.AssetRequest, error) {
	resp, err := s.svc.RejectRequest(ctx, req)
	return resp, mapError(err)
}

func (s *AssetRequestServer) AssignAsset(ctx context.Context, req *pb.AssignAssetReq) (*pb.AssetRequest, error) {
	resp, err := s.svc.AssignAsset(ctx, req)
	return resp, mapError(err)
}

func (s *AssetRequestServer) ReturnAsset(ctx context.Context, req *pb.ReturnAssetReq) (*pb.Empty, error) {
	resp, err := s.svc.ReturnAsset(ctx, req)
	return resp, mapError(err)
}

func (s *AssetRequestServer) ListRequests(ctx context.Context, req *pb.ListRequestsReq) (*pb.AssetRequestList, error) {
	resp, err := s.svc.ListRequests(ctx, req)
	return resp, mapError(err)
}

func (s *AssetRequestServer) GetRequest(ctx context.Context, req *pb.GetRequestReq) (*pb.AssetRequest, error) {
	resp, err := s.svc.GetRequest(ctx, req)
	return resp, mapError(err)
}

// ServeAssetService wraps an existing domain service as a gRPC server.
func ServeAssetService(svc *domain.AssetService) *AssetServer { return &AssetServer{svc: svc} }

// ServeAssetTypeService wraps an existing domain service as a gRPC server.
func ServeAssetTypeService(svc *domain.AssetTypeService) *AssetTypeServer {
	return &AssetTypeServer{svc: svc}
}

// ServeAssetRequestService wraps an existing domain service as a gRPC server.
func ServeAssetRequestService(svc *domain.AssetRequestService) *AssetRequestServer {
	return &AssetRequestServer{svc: svc}
}
