package domain

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"ngac-platform/ngac"
	"ngac-platform/services/auth/internal/auth"
	"ngac-platform/services/auth/internal/store"
	messagingpb "ngac-platform/proto/messaging"
	policypb "ngac-platform/proto/policy"
	workspacepb "ngac-platform/proto/workspace"
)

// AuthStore defines the database operations the domain layer needs.
type AuthStore interface {
	CreateUser(ctx context.Context, id, username, password, ngacNodeID, email, unionID, displayName, phone string) error
	GetUserByUsername(ctx context.Context, username string) (*store.User, error)
	GetUserByEmail(ctx context.Context, email string) (*store.User, error)
	GetUserByPhone(ctx context.Context, phone string) (*store.User, error)
	GetUserByID(ctx context.Context, userID string) (*store.User, error)
	GetUserByNGACNodeID(ctx context.Context, ngacNodeID string) (*store.User, error)
	ListUsers(ctx context.Context) ([]store.User, error)
	InsertTenantUser(ctx context.Context, tenantID, userID, role, status, ngacNodeID string) error
	ListTenantsByUser(ctx context.Context, userID string) ([]store.TenantMembership, error)
	GetTenantUser(ctx context.Context, tenantID, userID string) (*store.TenantMembership, error)
	FindTenantByDomain(ctx context.Context, domain string) (*store.Tenant, error)
	UpdateProfile(ctx context.Context, userID, displayName, title, department, location, avatarURL string) error
	ListContactsByWorkspace(ctx context.Context, workspaceID, departmentFilter, locationFilter string) ([]store.User, error)
}

// AuthResponse is the domain output for legacy register/login operations.
type AuthResponse struct {
	Token      string
	UserID     string
	Username   string
	NGACNodeID string
}

// SignupResult is the domain output for multi-tenant signup.
type SignupResult struct {
	Token      string
	UserID     string
	Username   string
	NGACNodeID string
	Email      string
	UnionID    string
	TenantID   string
	TenantName string
	TenantRole string
	OpenID     string
}

// SigninResult is the domain output for multi-tenant signin.
type SigninResult struct {
	Token           string
	UserID          string
	Username        string
	NGACNodeID      string
	Email           string
	UnionID         string
	DisplayName     string
	DefaultTenantID string
	Tenants         []TenantInfo
}

// TenantInfo represents a tenant in domain responses.
type TenantInfo struct {
	ID     string
	Name   string
	Role   string
	OpenID string
}

// UserInfo is the domain representation of a user (no password).
type UserInfo struct {
	ID          string
	Username    string
	NGACNodeID  string
	Email       string
	UnionID     string
	DisplayName string
}

// ProfileUpdateInput contains fields to update on a user profile.
type ProfileUpdateInput struct {
	DisplayName string
	Title       string
	Department  string
	Location    string
	AvatarURL   string
}

// ContactInfo is a user enriched with profile data for the contacts directory.
type ContactInfo struct {
	UserID      string
	NGACNodeID  string
	Username    string
	DisplayName string
	Email       string
	Title       string
	Department  string
	Location    string
	AvatarURL   string
}

// Service orchestrates auth business logic.
type Service struct {
	store       AuthStore
	rdb         *redis.Client
	policyRead  policypb.PolicyReadServiceClient
	policyWrite policypb.PolicyWriteServiceClient
	wsClient    workspacepb.WorkspaceServiceClient
	msgClient   messagingpb.MessagingServiceClient
}

// NewService creates an auth domain service.
func NewService(
	st AuthStore,
	rdb *redis.Client,
	pr policypb.PolicyReadServiceClient,
	pw policypb.PolicyWriteServiceClient,
	wsClient workspacepb.WorkspaceServiceClient,
	msgClient messagingpb.MessagingServiceClient,
) *Service {
	return &Service{
		store:       st,
		rdb:         rdb,
		policyRead:  pr,
		policyWrite: pw,
		wsClient:    wsClient,
		msgClient:   msgClient,
	}
}

// Signup creates a new user and joins or creates a tenant.
func (s *Service) Signup(ctx context.Context, email, password, displayName, tenantName string) (*SignupResult, error) {
	if email == "" || password == "" {
		return nil, ErrInvalidInput
	}

	existing, _ := s.store.GetUserByEmail(ctx, email)
	if existing != nil {
		return nil, ErrUserExists
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	username := emailToUsername(email)
	if displayName == "" {
		displayName = username
	}

	ngacNode, err := s.createUserNGACNode(ctx, username)
	if err != nil {
		return nil, fmt.Errorf("create ngac node: %w", err)
	}

	userID := uuid.New().String()
	unionID := uuid.New().String()
	if err := s.store.CreateUser(ctx, userID, username, hash, ngacNode, email, unionID, displayName, ""); err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}

	tenantID, tName, role, err := s.resolveOrCreateTenant(ctx, email, userID, ngacNode, tenantName, displayName)
	if err != nil {
		return nil, fmt.Errorf("resolve tenant: %w", err)
	}

	token, err := auth.GenerateToken(userID, username, ngacNode, tenantID)
	if err != nil {
		return nil, fmt.Errorf("generate token: %w", err)
	}

	membership, _ := s.store.GetTenantUser(ctx, tenantID, userID)
	openID := ""
	if membership != nil {
		openID = membership.OpenID
	}

	return &SignupResult{
		Token: token, UserID: userID, Username: username,
		NGACNodeID: ngacNode, Email: email, UnionID: unionID,
		TenantID: tenantID, TenantName: tName, TenantRole: role, OpenID: openID,
	}, nil
}

// resolveOrCreateTenant determines whether to join an existing tenant or create a new one.
func (s *Service) resolveOrCreateTenant(ctx context.Context, email, userID, ngacNodeID, tenantName, displayName string) (string, string, string, error) {
	// Case 1: explicit tenant name → always create new
	if tenantName != "" {
		return s.createTenantForUser(ctx, tenantName, userID, ngacNodeID)
	}

	// Case 2: check email domain for auto-join
	domain := extractDomain(email)
	if domain != "" {
		tenant, err := s.store.FindTenantByDomain(ctx, domain)
		if err != nil {
			return "", "", "", fmt.Errorf("find tenant by domain: %w", err)
		}
		if tenant != nil {
			if err := s.joinTenant(ctx, tenant.ID, userID, ngacNodeID, "member"); err != nil {
				return "", "", "", fmt.Errorf("join tenant: %w", err)
			}
			return tenant.ID, tenant.Name, "member", nil
		}
	}

	// Case 3: no match → create new tenant
	wsName := fmt.Sprintf("%s's Workspace", displayName)
	return s.createTenantForUser(ctx, wsName, userID, ngacNodeID)
}

// createTenantForUser creates a workspace/tenant, initializes tenant NGAC UAs, and assigns the user as owner.
func (s *Service) createTenantForUser(ctx context.Context, name, userID, ngacNodeID string) (string, string, string, error) {
	if s.wsClient == nil {
		return "", "", "", fmt.Errorf("workspace service unavailable")
	}

	ws, err := s.wsClient.CreateWorkspace(ctx, &workspacepb.CreateWorkspaceRequest{
		Name: name, UserId: userID, UserNgacNodeId: ngacNodeID,
	})
	if err != nil {
		return "", "", "", fmt.Errorf("create workspace: %w", err)
	}

	// Create tenant-scoped NGAC UAs and assign under the workspace PC.
	s.initTenantNGAC(ctx, ws.Id, ws.PcNodeId, ws.OwnersUaId, ws.MembersUaId)

	if err := s.joinTenant(ctx, ws.Id, userID, ngacNodeID, "owner"); err != nil {
		return "", "", "", fmt.Errorf("join as owner: %w", err)
	}

	s.autoProvisionChannel(ctx, ws.Id, userID, ngacNodeID)

	return ws.Id, name, "owner", nil
}

// joinTenant creates the tenant_users record and assigns the user in NGAC.
func (s *Service) joinTenant(ctx context.Context, tenantID, userID, ngacNodeID, role string) error {
	if err := s.store.InsertTenantUser(ctx, tenantID, userID, role, "active", ngacNodeID); err != nil {
		return fmt.Errorf("insert tenant user: %w", err)
	}
	s.assignUserToTenantNGAC(ctx, tenantID, ngacNodeID, role == "owner")
	return nil
}

// Signin authenticates by email and returns tenant list with a default-scoped JWT.
func (s *Service) Signin(ctx context.Context, email, password string) (*SigninResult, error) {
	if email == "" || password == "" {
		return nil, ErrInvalidInput
	}

	user, err := s.store.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	if user == nil {
		return nil, ErrInvalidCredentials
	}
	if !auth.CheckPassword(password, user.Password) {
		return nil, ErrInvalidCredentials
	}

	tenants, err := s.store.ListTenantsByUser(ctx, user.ID)
	if err != nil {
		return nil, fmt.Errorf("list tenants: %w", err)
	}

	defaultTenantID := s.selectDefaultTenant(tenants)

	token, err := auth.GenerateToken(user.ID, user.Username, user.NGACNodeID, defaultTenantID)
	if err != nil {
		return nil, fmt.Errorf("generate token: %w", err)
	}

	result := &SigninResult{
		Token: token, UserID: user.ID, Username: user.Username,
		NGACNodeID: user.NGACNodeID, Email: user.Email,
		UnionID: user.UnionID, DisplayName: user.DisplayName,
		DefaultTenantID: defaultTenantID,
	}
	for _, t := range tenants {
		result.Tenants = append(result.Tenants, TenantInfo{
			ID: t.TenantID, Name: t.TenantName, Role: t.Role, OpenID: t.OpenID,
		})
	}
	return result, nil
}

// SwitchTenant verifies membership and issues a new JWT scoped to the target tenant.
func (s *Service) SwitchTenant(ctx context.Context, userID, ngacNodeID, username, targetTenantID string) (string, *TenantInfo, error) {
	membership, err := s.store.GetTenantUser(ctx, targetTenantID, userID)
	if err != nil {
		return "", nil, fmt.Errorf("get tenant user: %w", err)
	}
	if membership == nil {
		return "", nil, ErrAccessDenied
	}

	token, err := auth.GenerateToken(userID, username, ngacNodeID, targetTenantID)
	if err != nil {
		return "", nil, fmt.Errorf("generate token: %w", err)
	}

	info := &TenantInfo{
		ID: membership.TenantID, Name: membership.TenantName,
		Role: membership.Role, OpenID: membership.OpenID,
	}
	return token, info, nil
}

// GetMe returns current user and tenant info.
func (s *Service) GetMe(ctx context.Context, userID, tenantID string) (*UserInfo, *TenantInfo, error) {
	user, err := s.store.GetUserByID(ctx, userID)
	if err != nil {
		return nil, nil, fmt.Errorf("get user: %w", err)
	}
	if user == nil {
		return nil, nil, ErrNotFound
	}

	uInfo := &UserInfo{
		ID: user.ID, Username: user.Username, NGACNodeID: user.NGACNodeID,
		Email: user.Email, UnionID: user.UnionID, DisplayName: user.DisplayName,
	}

	if tenantID == "" {
		return uInfo, nil, nil
	}

	membership, err := s.store.GetTenantUser(ctx, tenantID, userID)
	if err != nil {
		return nil, nil, fmt.Errorf("get tenant user: %w", err)
	}

	var tInfo *TenantInfo
	if membership != nil {
		tInfo = &TenantInfo{
			ID: membership.TenantID, Name: membership.TenantName,
			Role: membership.Role, OpenID: membership.OpenID,
		}
	}
	return uInfo, tInfo, nil
}

// Register is the legacy registration flow (backward-compatible).
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
	unionID := uuid.New().String()
	if err := s.store.CreateUser(ctx, userID, username, hash, ngacNode, "", unionID, username, ""); err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}

	// Auto-provision workspace + tenant_users + #general channel
	s.autoProvisionWorkspace(ctx, userID, username, ngacNode)

	token, err := auth.GenerateToken(userID, username, ngacNode, "")
	if err != nil {
		return nil, fmt.Errorf("generate token: %w", err)
	}

	return &AuthResponse{Token: token, UserID: userID, Username: username, NGACNodeID: ngacNode}, nil
}

// Login is the legacy login flow (backward-compatible, uses username).
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

	token, err := auth.GenerateToken(user.ID, user.Username, user.NGACNodeID, "")
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
	return &UserInfo{ID: user.ID, Username: user.Username, NGACNodeID: user.NGACNodeID, Email: user.Email, UnionID: user.UnionID, DisplayName: user.DisplayName}, nil
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

// GetUserByUsername looks up a user by username.
func (s *Service) GetUserByUsername(ctx context.Context, username string) (*UserInfo, error) {
	user, err := s.store.GetUserByUsername(ctx, username)
	if err != nil {
		return nil, fmt.Errorf("get user by username: %w", err)
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

// UpdateProfile updates a user's profile fields.
func (s *Service) UpdateProfile(ctx context.Context, userID string, in ProfileUpdateInput) error {
	if userID == "" {
		return ErrInvalidInput
	}
	return s.store.UpdateProfile(ctx, userID, in.DisplayName, in.Title, in.Department, in.Location, in.AvatarURL)
}

// ListContacts returns enriched user profiles for a workspace.
func (s *Service) ListContacts(ctx context.Context, workspaceID, department, location string) ([]ContactInfo, error) {
	if workspaceID == "" {
		return nil, ErrInvalidInput
	}
	users, err := s.store.ListContactsByWorkspace(ctx, workspaceID, department, location)
	if err != nil {
		return nil, fmt.Errorf("list contacts: %w", err)
	}
	contacts := make([]ContactInfo, len(users))
	for i, u := range users {
		contacts[i] = ContactInfo{
			UserID: u.ID, NGACNodeID: u.NGACNodeID, Username: u.Username,
			DisplayName: u.DisplayName, Email: u.Email,
			Title: u.Title, Department: u.Department,
			Location: u.Location, AvatarURL: u.AvatarURL,
		}
	}
	return contacts, nil
}

// --- Private helpers ---

// autoProvisionWorkspace creates a default workspace and #general channel (legacy flow).
func (s *Service) autoProvisionWorkspace(ctx context.Context, userID, username, ngacNodeID string) {
	if s.wsClient == nil {
		slog.Warn("workspace client unavailable, skipping auto-provision")
		return
	}

	wsName := fmt.Sprintf("%s's Workspace", username)
	ws, err := s.wsClient.CreateWorkspace(ctx, &workspacepb.CreateWorkspaceRequest{
		Name: wsName, UserId: userID, UserNgacNodeId: ngacNodeID,
	})
	if err != nil {
		slog.Error("auto-provision workspace failed", "user", username, "error", err)
		return
	}
	slog.Info("auto-provisioned workspace", "workspace_id", ws.Id, "user", username)

	// Create tenant_users record for the owner
	if err := s.store.InsertTenantUser(ctx, ws.Id, userID, "owner", "active", ngacNodeID); err != nil {
		slog.Error("auto-provision tenant_users failed", "workspace", ws.Id, "error", err)
	}

	s.autoProvisionChannel(ctx, ws.Id, userID, ngacNodeID)
}

// autoProvisionChannel creates a #general channel in the workspace.
func (s *Service) autoProvisionChannel(ctx context.Context, workspaceID, userID, ngacNodeID string) {
	if s.msgClient == nil {
		slog.Warn("messaging client unavailable, skipping #general channel")
		return
	}
	_, err := s.msgClient.CreateChannel(ctx, &messagingpb.CreateChannelRequest{
		Name: "general", WorkspaceId: workspaceID,
		UserId: userID, UserNgacNodeId: ngacNodeID, ChannelType: "workspace",
	})
	if err != nil {
		slog.Error("auto-provision #general channel failed", "workspace", workspaceID, "error", err)
		return
	}
	slog.Info("auto-provisioned #general channel", "workspace_id", workspaceID)
}

// createUserNGACNode creates a user node in the NGAC graph and assigns to PublicUsers.
func (s *Service) createUserNGACNode(ctx context.Context, username string) (string, error) {
	userNode, err := s.policyWrite.CreateNode(ctx, &policypb.CreateNodeRequest{
		Name: username, NodeType: ngac.TypeU,
		Properties: map[string]string{"type": "user"},
	})
	if err != nil {
		return "", fmt.Errorf("create node: %w", err)
	}

	publicUA, err := s.policyRead.FindNodeByName(ctx, &policypb.FindNodeByNameRequest{
		Name: ngac.NodePublicUsers, NodeType: ngac.TypeUA,
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

// initTenantNGAC creates the TenantMember and TenantOwner UAs for a new tenant
// and chains them into the workspace's existing NGAC graph.
func (s *Service) initTenantNGAC(ctx context.Context, tenantID, pcNodeID, ownersUAID, membersUAID string) {
	memberUA, err := s.policyWrite.CreateNode(ctx, &policypb.CreateNodeRequest{
		Name: ngac.TenantMemberUAName(tenantID), NodeType: ngac.TypeUA,
	})
	if err != nil {
		slog.Error("create TenantMember UA failed", "tenant", tenantID, "error", err)
		return
	}

	ownerUA, err := s.policyWrite.CreateNode(ctx, &policypb.CreateNodeRequest{
		Name: ngac.TenantOwnerUAName(tenantID), NodeType: ngac.TypeUA,
	})
	if err != nil {
		slog.Error("create TenantOwner UA failed", "tenant", tenantID, "error", err)
		return
	}

	// Assign UAs under the workspace PC for NGAC scoping
	s.policyWrite.CreateAssignment(ctx, &policypb.CreateAssignmentRequest{ChildId: memberUA.Id, ParentId: pcNodeID})
	s.policyWrite.CreateAssignment(ctx, &policypb.CreateAssignmentRequest{ChildId: ownerUA.Id, ParentId: pcNodeID})

	// Chain: TenantMember inherits workspace Members permissions,
	// TenantOwner inherits workspace Owners permissions.
	if membersUAID != "" {
		s.policyWrite.CreateAssignment(ctx, &policypb.CreateAssignmentRequest{ChildId: memberUA.Id, ParentId: membersUAID})
	}
	if ownersUAID != "" {
		s.policyWrite.CreateAssignment(ctx, &policypb.CreateAssignmentRequest{ChildId: ownerUA.Id, ParentId: ownersUAID})
	}

	slog.Info("tenant NGAC initialized", "tenant", tenantID, "member_ua", memberUA.Id, "owner_ua", ownerUA.Id)
}

// assignUserToTenantNGAC assigns a user's NGAC node to the tenant's member/owner UAs.
// Uses find-or-log pattern: if the UA doesn't exist yet, logs an error instead of silently skipping.
func (s *Service) assignUserToTenantNGAC(ctx context.Context, tenantID, userNodeID string, isOwner bool) {
	// Assign to TenantMember UA
	memberUA, err := s.policyRead.FindNodeByName(ctx, &policypb.FindNodeByNameRequest{
		Name: ngac.TenantMemberUAName(tenantID), NodeType: ngac.TypeUA,
	})
	if err != nil {
		slog.Error("TenantMember UA not found — was initTenantNGAC called?", "tenant", tenantID, "error", err)
		return
	}
	if _, err := s.policyWrite.CreateAssignment(ctx, &policypb.CreateAssignmentRequest{
		ChildId: userNodeID, ParentId: memberUA.Id,
	}); err != nil {
		slog.Error("assign to tenant member UA failed", "tenant", tenantID, "error", err)
	}

	if !isOwner {
		return
	}

	// Assign to TenantOwner UA
	ownerUA, err := s.policyRead.FindNodeByName(ctx, &policypb.FindNodeByNameRequest{
		Name: ngac.TenantOwnerUAName(tenantID), NodeType: ngac.TypeUA,
	})
	if err != nil {
		slog.Error("TenantOwner UA not found — was initTenantNGAC called?", "tenant", tenantID, "error", err)
		return
	}
	if _, err := s.policyWrite.CreateAssignment(ctx, &policypb.CreateAssignmentRequest{
		ChildId: userNodeID, ParentId: ownerUA.Id,
	}); err != nil {
		slog.Error("assign to tenant owner UA failed", "tenant", tenantID, "error", err)
	}
}

// selectDefaultTenant picks the default tenant (prefer owner, else first).
func (s *Service) selectDefaultTenant(tenants []store.TenantMembership) string {
	if len(tenants) == 0 {
		return ""
	}
	for _, t := range tenants {
		if t.Role == "owner" {
			return t.TenantID
		}
	}
	return tenants[0].TenantID
}

// extractDomain extracts the domain part from an email address.
func extractDomain(email string) string {
	parts := strings.SplitN(email, "@", 2)
	if len(parts) != 2 {
		return ""
	}
	return parts[1]
}

// emailToUsername derives a username from email.
// Uses "local" part first, falls back to "local.domain" if collision is likely.
func emailToUsername(email string) string {
	parts := strings.SplitN(email, "@", 2)
	if len(parts) != 2 {
		return email
	}
	local := parts[0]
	domain := strings.TrimSuffix(parts[1], ".com")
	domain = strings.TrimSuffix(domain, ".test")
	// Use local.domain to minimize collision risk across different email domains.
	if domain != "" {
		return fmt.Sprintf("%s.%s", local, domain)
	}
	return local
}
