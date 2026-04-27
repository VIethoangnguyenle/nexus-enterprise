// Package grpc provides thin gRPC handlers for the auth service.
// Each method validates input, delegates to the domain layer, and maps errors.
// No SQL, no business logic, no password hashing.
package grpc

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "ngac-platform/proto/auth"
	"ngac-platform/services/auth/internal/domain"
)

// AuthService defines what the gRPC handler needs from the domain layer.
type AuthService interface {
	Register(ctx context.Context, username, password string) (*domain.AuthResponse, error)
	Login(ctx context.Context, username, password string) (*domain.AuthResponse, error)
	GetUserByID(ctx context.Context, userID string) (*domain.UserInfo, error)
	GetUserByNGACNodeID(ctx context.Context, nodeID string) (*domain.UserInfo, error)
	ListUsers(ctx context.Context) ([]domain.UserInfo, error)
}

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

// Register delegates to domain.Service.Register.
func (s *AuthServer) Register(ctx context.Context, req *pb.RegisterRequest) (*pb.AuthResponse, error) {
	resp, err := s.svc.Register(ctx, req.Username, req.Password)
	if err != nil {
		return nil, mapError(err)
	}
	return toAuthResponse(resp), nil
}

// Login delegates to domain.Service.Login.
func (s *AuthServer) Login(ctx context.Context, req *pb.LoginRequest) (*pb.AuthResponse, error) {
	resp, err := s.svc.Login(ctx, req.Username, req.Password)
	if err != nil {
		return nil, mapError(err)
	}
	return toAuthResponse(resp), nil
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

// ListUsers delegates to domain.Service.ListUsers.
func (s *AuthServer) ListUsers(ctx context.Context, _ *pb.ListUsersRequest) (*pb.UserListResponse, error) {
	users, err := s.svc.ListUsers(ctx)
	if err != nil {
		return nil, mapError(err)
	}
	var resp []*pb.UserInfo
	for _, u := range users {
		resp = append(resp, &pb.UserInfo{Id: u.ID, Username: u.Username, NgacNodeId: u.NGACNodeID})
	}
	return &pb.UserListResponse{Users: resp}, nil
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
		return nil, status.Errorf(codes.Internal, "blacklist token: %v", err)
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

func toAuthResponse(r *domain.AuthResponse) *pb.AuthResponse {
	return &pb.AuthResponse{
		Token: r.Token,
		User:  &pb.UserInfo{Id: r.UserID, Username: r.Username, NgacNodeId: r.NGACNodeID},
	}
}

func toUserInfo(u *domain.UserInfo) *pb.UserInfo {
	return &pb.UserInfo{Id: u.ID, Username: u.Username, NgacNodeId: u.NGACNodeID}
}

func mapError(err error) error {
	switch {
	case errors.Is(err, domain.ErrInvalidCredentials):
		return status.Error(codes.Unauthenticated, err.Error())
	case errors.Is(err, domain.ErrUserExists):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, domain.ErrNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, domain.ErrInvalidInput):
		return status.Error(codes.InvalidArgument, err.Error())
	default:
		return status.Errorf(codes.Internal, "internal: %v", err)
	}
}
