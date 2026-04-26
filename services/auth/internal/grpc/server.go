package grpc

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"ngac-platform/services/auth/internal/auth"
	"ngac-platform/services/auth/internal/store"
	pb "ngac-platform/proto/auth"
	policypb "ngac-platform/proto/policy"
)

// AuthServer handles authentication with optional Redis JWT blacklisting.
type AuthServer struct {
	pb.UnimplementedAuthServiceServer
	store        *store.Store
	policyClient policypb.PolicyServiceClient
	rdb          *redis.Client
}

// NewAuthServer creates an AuthServer with optional Redis for JWT blacklisting.
func NewAuthServer(s *store.Store, pc policypb.PolicyServiceClient, rdb *redis.Client) *AuthServer {
	return &AuthServer{store: s, policyClient: pc, rdb: rdb}
}

func (s *AuthServer) Register(ctx context.Context, req *pb.RegisterRequest) (*pb.AuthResponse, error) {
	existing, _ := s.store.GetUserByUsername(ctx, req.Username)
	if existing != nil {
		return nil, status.Errorf(codes.AlreadyExists, "username already taken")
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "hash password: %v", err)
	}

	userNode, err := s.policyClient.CreateNode(ctx, &policypb.CreateNodeRequest{
		Name: req.Username, NodeType: "U",
		Properties: map[string]string{"type": "user"},
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "create ngac node: %v", err)
	}

	publicUA, err := s.policyClient.FindNodeByName(ctx, &policypb.FindNodeByNameRequest{
		Name: "PublicUsers", NodeType: "UA",
	})
	if err == nil && publicUA != nil {
		s.policyClient.CreateAssignment(ctx, &policypb.CreateAssignmentRequest{
			ChildId: userNode.Id, ParentId: publicUA.Id,
		})
	}

	userID := uuid.New().String()
	if err := s.store.CreateUser(ctx, userID, req.Username, hash, userNode.Id); err != nil {
		return nil, status.Errorf(codes.Internal, "create user: %v", err)
	}

	token, err := auth.GenerateToken(userID, req.Username, userNode.Id)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "generate token: %v", err)
	}

	return &pb.AuthResponse{
		Token: token,
		User:  &pb.UserInfo{Id: userID, Username: req.Username, NgacNodeId: userNode.Id},
	}, nil
}

func (s *AuthServer) Login(ctx context.Context, req *pb.LoginRequest) (*pb.AuthResponse, error) {
	user, err := s.store.GetUserByUsername(ctx, req.Username)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get user: %v", err)
	}
	if user == nil {
		return nil, status.Errorf(codes.NotFound, "invalid credentials")
	}

	if !auth.CheckPassword(req.Password, user.Password) {
		return nil, status.Errorf(codes.Unauthenticated, "invalid credentials")
	}

	token, err := auth.GenerateToken(user.ID, user.Username, user.NGACNodeID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "generate token: %v", err)
	}

	return &pb.AuthResponse{
		Token: token,
		User:  &pb.UserInfo{Id: user.ID, Username: user.Username, NgacNodeId: user.NGACNodeID},
	}, nil
}

// RevokeToken adds a JWT ID to the Redis blacklist with TTL matching token expiry.
func (s *AuthServer) RevokeToken(ctx context.Context, req *pb.RevokeTokenRequest) (*pb.RevokeTokenResponse, error) {
	if s.rdb == nil {
		return nil, status.Errorf(codes.Unavailable, "jwt blacklist unavailable: redis not connected")
	}

	remaining := time.Until(time.Unix(req.ExpiresAtUnix, 0))
	if remaining <= 0 {
		return &pb.RevokeTokenResponse{Revoked: true}, nil
	}

	key := jwtBlacklistKey(req.Jti)
	if err := s.rdb.Set(ctx, key, "1", remaining).Err(); err != nil {
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

func (s *AuthServer) GetUserByID(ctx context.Context, req *pb.GetUserByIDRequest) (*pb.UserInfo, error) {
	user, err := s.store.GetUserByID(ctx, req.UserId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get user: %v", err)
	}
	if user == nil {
		return nil, status.Errorf(codes.NotFound, "user not found")
	}
	return &pb.UserInfo{Id: user.ID, Username: user.Username, NgacNodeId: user.NGACNodeID}, nil
}

func (s *AuthServer) GetUserByNGACNodeID(ctx context.Context, req *pb.GetUserByNGACNodeIDRequest) (*pb.UserInfo, error) {
	user, err := s.store.GetUserByNGACNodeID(ctx, req.NgacNodeId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get user: %v", err)
	}
	if user == nil {
		return nil, status.Errorf(codes.NotFound, fmt.Sprintf("user not found for ngac node %s", req.NgacNodeId))
	}
	return &pb.UserInfo{Id: user.ID, Username: user.Username, NgacNodeId: user.NGACNodeID}, nil
}

func (s *AuthServer) ListUsers(ctx context.Context, _ *pb.ListUsersRequest) (*pb.UserListResponse, error) {
	users, err := s.store.ListUsers(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list users: %v", err)
	}
	var resp []*pb.UserInfo
	for _, u := range users {
		resp = append(resp, &pb.UserInfo{Id: u.ID, Username: u.Username, NgacNodeId: u.NGACNodeID})
	}
	return &pb.UserListResponse{Users: resp}, nil
}
