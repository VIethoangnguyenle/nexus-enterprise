// Package grpc provides thin gRPC handlers for the auth service.
// Each method validates input, delegates to the domain layer, and maps errors.
// No SQL, no business logic, no password hashing.
package grpc

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"ngac-platform/pkg/grpcauth"
	"ngac-platform/pkg/grpcutil"
	pb "ngac-platform/proto/auth"
	"ngac-platform/services/auth/internal/domain"
)

// AuthServer handles gRPC auth requests.
type AuthServer struct {
	pb.UnimplementedAuthServiceServer
	svc *domain.Service
	rdb *redis.Client
}

// NewAuthServer creates a gRPC handler backed by the domain service.
func NewAuthServer(svc *domain.Service, rdb *redis.Client) *AuthServer {
	return &AuthServer{svc: svc, rdb: rdb}
}

// GetUserByID delegates to domain.Service.GetUserByID.
func (s *AuthServer) GetUserByID(ctx context.Context, req *pb.GetUserByIDRequest) (*pb.UserInfo, error) {
	user, err := s.svc.GetUserByID(ctx, req.UserId)
	if err != nil {
		return nil, mapError(err)
	}
	return toUserInfo(user), nil
}

// GetUserByNGACNodeID delegates to domain.Service.GetUserByNGACNodeID.
func (s *AuthServer) GetUserByNGACNodeID(ctx context.Context, req *pb.GetUserByNGACNodeIDRequest) (*pb.UserInfo, error) {
	user, err := s.svc.GetUserByNGACNodeID(ctx, req.NgacNodeId)
	if err != nil {
		return nil, mapError(err)
	}
	return toUserInfo(user), nil
}

// RevokeToken adds a JWT ID to the Redis blacklist.
func (s *AuthServer) RevokeToken(ctx context.Context, req *pb.RevokeTokenRequest) (*pb.RevokeTokenResponse, error) {
	if s.rdb == nil {
		return nil, status.Error(codes.Unavailable, "jwt blacklist unavailable")
	}
	remaining := time.Until(time.Unix(req.ExpiresAtUnix, 0))
	if remaining <= 0 {
		return &pb.RevokeTokenResponse{Revoked: true}, nil
	}
	if err := s.rdb.Set(ctx, jwtBlacklistKey(req.Jti), "1", remaining).Err(); err != nil {
		return nil, grpcauth.Internal(fmt.Errorf("blacklist token: %w", err))
	}
	return &pb.RevokeTokenResponse{Revoked: true}, nil
}

// IsTokenRevoked checks if a JWT ID exists in the blacklist.
func (s *AuthServer) IsTokenRevoked(ctx context.Context, req *pb.IsTokenRevokedRequest) (*pb.IsTokenRevokedResponse, error) {
	if s.rdb == nil {
		return &pb.IsTokenRevokedResponse{Revoked: false}, nil
	}
	exists, err := s.rdb.Exists(ctx, jwtBlacklistKey(req.Jti)).Result()
	if err != nil {
		return &pb.IsTokenRevokedResponse{Revoked: false}, nil
	}
	return &pb.IsTokenRevokedResponse{Revoked: exists > 0}, nil
}

func jwtBlacklistKey(jti string) string {
	return fmt.Sprintf("jwt:blacklist:%s", jti)
}

func toUserInfo(u *domain.UserInfo) *pb.UserInfo {
	return &pb.UserInfo{Id: u.ID, Username: u.Username, NgacNodeId: u.NGACNodeID, Email: u.Email, UnionId: u.UnionID, DisplayName: u.DisplayName}
}

// mapError translates a domain error to a gRPC status; anything that is not the
// domain's own refusal becomes a generic Internal (see grpcutil.Status).
func mapError(err error) error {
	return grpcutil.Status(err,
		grpcutil.Mapping{Is: domain.ErrInvalidCredentials, Code: codes.Unauthenticated},
		grpcutil.Mapping{Is: domain.ErrUserExists, Code: codes.AlreadyExists},
		grpcutil.Mapping{Is: domain.ErrTenantNotFound, Code: codes.NotFound},
	)
}
