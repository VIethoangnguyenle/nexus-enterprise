package domain

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"ngac-platform/services/auth/internal/auth"
	"ngac-platform/services/auth/internal/store"
	policypb "ngac-platform/proto/policy"
)

// AuthStore defines the database operations the domain layer needs.
type AuthStore interface {
	CreateUser(ctx context.Context, id, username, password, ngacNodeID string) error
	GetUserByUsername(ctx context.Context, username string) (*store.User, error)
	GetUserByID(ctx context.Context, userID string) (*store.User, error)
	GetUserByNGACNodeID(ctx context.Context, ngacNodeID string) (*store.User, error)
	ListUsers(ctx context.Context) ([]store.User, error)
}

// AuthResponse is the domain output for register/login operations.
type AuthResponse struct {
	Token      string
	UserID     string
	Username   string
	NGACNodeID string
}

// UserInfo is the domain representation of a user (no password).
type UserInfo struct {
	ID         string
	Username   string
	NGACNodeID string
}

// Service orchestrates auth business logic.
type Service struct {
	store       AuthStore
	policyRead  policypb.PolicyReadServiceClient
	policyWrite policypb.PolicyWriteServiceClient
}

// NewService creates an auth domain service.
func NewService(st AuthStore, pr policypb.PolicyReadServiceClient, pw policypb.PolicyWriteServiceClient) *Service {
	return &Service{store: st, policyRead: pr, policyWrite: pw}
}

// Register creates a new user with an NGAC node and returns a signed JWT.
func (s *Service) Register(ctx context.Context, username, password string) (*AuthResponse, error) {
	if username == "" || password == "" {
		return nil, ErrInvalidInput
	}

	existing, _ := s.store.GetUserByUsername(ctx, username)
	if existing != nil {
		return nil, ErrUserExists
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	ngacNode, err := s.createUserNGACNode(ctx, username)
	if err != nil {
		return nil, fmt.Errorf("create ngac node: %w", err)
	}

	userID := uuid.New().String()
	if err := s.store.CreateUser(ctx, userID, username, hash, ngacNode); err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}

	token, err := auth.GenerateToken(userID, username, ngacNode)
	if err != nil {
		return nil, fmt.Errorf("generate token: %w", err)
	}

	return &AuthResponse{Token: token, UserID: userID, Username: username, NGACNodeID: ngacNode}, nil
}

// Login validates credentials and returns a signed JWT.
func (s *Service) Login(ctx context.Context, username, password string) (*AuthResponse, error) {
	if username == "" || password == "" {
		return nil, ErrInvalidInput
	}

	user, err := s.store.GetUserByUsername(ctx, username)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	if user == nil {
		return nil, ErrInvalidCredentials
	}

	if !auth.CheckPassword(password, user.Password) {
		return nil, ErrInvalidCredentials
	}

	token, err := auth.GenerateToken(user.ID, user.Username, user.NGACNodeID)
	if err != nil {
		return nil, fmt.Errorf("generate token: %w", err)
	}

	return &AuthResponse{Token: token, UserID: user.ID, Username: user.Username, NGACNodeID: user.NGACNodeID}, nil
}

// GetUserByID retrieves a user by their primary key.
func (s *Service) GetUserByID(ctx context.Context, userID string) (*UserInfo, error) {
	user, err := s.store.GetUserByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get user by id: %w", err)
	}
	if user == nil {
		return nil, ErrNotFound
	}
	return &UserInfo{ID: user.ID, Username: user.Username, NGACNodeID: user.NGACNodeID}, nil
}

// GetUserByNGACNodeID retrieves a user by their NGAC graph node.
func (s *Service) GetUserByNGACNodeID(ctx context.Context, nodeID string) (*UserInfo, error) {
	user, err := s.store.GetUserByNGACNodeID(ctx, nodeID)
	if err != nil {
		return nil, fmt.Errorf("get user by ngac node: %w", err)
	}
	if user == nil {
		return nil, ErrNotFound
	}
	return &UserInfo{ID: user.ID, Username: user.Username, NGACNodeID: user.NGACNodeID}, nil
}

// ListUsers returns all users.
func (s *Service) ListUsers(ctx context.Context) ([]UserInfo, error) {
	users, err := s.store.ListUsers(ctx)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	result := make([]UserInfo, len(users))
	for i, u := range users {
		result[i] = UserInfo{ID: u.ID, Username: u.Username, NGACNodeID: u.NGACNodeID}
	}
	return result, nil
}

// createUserNGACNode creates a user node in the NGAC graph and assigns to PublicUsers.
func (s *Service) createUserNGACNode(ctx context.Context, username string) (string, error) {
	userNode, err := s.policyWrite.CreateNode(ctx, &policypb.CreateNodeRequest{
		Name: username, NodeType: "U",
		Properties: map[string]string{"type": "user"},
	})
	if err != nil {
		return "", fmt.Errorf("create node: %w", err)
	}

	publicUA, err := s.policyRead.FindNodeByName(ctx, &policypb.FindNodeByNameRequest{
		Name: "PublicUsers", NodeType: "UA",
	})
	if err == nil && publicUA != nil {
		if _, err := s.policyWrite.CreateAssignment(ctx, &policypb.CreateAssignmentRequest{
			ChildId: userNode.Id, ParentId: publicUA.Id,
		}); err != nil {
			return "", fmt.Errorf("assign to public: %w", err)
		}
	}

	return userNode.Id, nil
}
