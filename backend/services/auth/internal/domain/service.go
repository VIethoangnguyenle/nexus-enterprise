package domain

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"ngac-platform/ngac"
	"ngac-platform/pkg/grpcauth"
	"ngac-platform/pkg/provision"
	messagingpb "ngac-platform/proto/messaging"
	policypb "ngac-platform/proto/policy"
	workspacepb "ngac-platform/proto/workspace"
	"ngac-platform/services/auth/internal/auth"
	"ngac-platform/services/auth/internal/store"
)

// AuthStore defines the database operations the domain layer needs.
type AuthStore interface {
	CreateUser(ctx context.Context, id, username, password, ngacNodeID, email, unionID, displayName, phone string) error
	GetUserByUsername(ctx context.Context, username string) (*store.User, error)
	// GetUserByEmail matches the address case-insensitively.
	GetUserByEmail(ctx context.Context, email string) (*store.User, error)
	// MarkEmailVerified records proof of the account's address; true when this call set it.
	MarkEmailVerified(ctx context.Context, userID string) (bool, error)
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

	// External identities (user_identities). Provider + subject is the key;
	// email is recorded for audit only and never used to look a user up.
	FindUserByIdentity(ctx context.Context, provider, subject string) (*store.User, error)
	GetIdentitySubject(ctx context.Context, provider, userID string) (string, error)
	LinkIdentity(ctx context.Context, provider, subject, userID, email string) error
	// ClaimTenantDomain sets a tenant's domain if neither it nor any other
	// tenant holds that domain yet, and reports whether it did.
	ClaimTenantDomain(ctx context.Context, tenantID, domain string) (bool, error)
	// ClearPassword removes a user's password (password sign-in stops working).
	ClearPassword(ctx context.Context, userID string) error
}

// AuthResponse is the domain output for legacy register/login operations.
type AuthResponse struct {
	Token      string
	SessionID  string
	UserID     string
	Username   string
	NGACNodeID string
}

// SignupResult is the domain output for multi-tenant signup.
type SignupResult struct {
	Token      string
	SessionID  string
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
	SessionID       string
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
	refresh     *RefreshStore
	otp         OTPOptions
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
		refresh:     NewRefreshStore(rdb),
		// Fixed-code test mode unless ConfigureOTP says otherwise.
		otp: OTPOptions{FixedCode: DefaultFixedOTPCode, Secret: newOTPSecret()},
	}
}

// Signup creates a new user and joins or creates a tenant.
func (s *Service) Signup(ctx context.Context, email, password, displayName, tenantName string) (*SignupResult, error) {
	if password == "" {
		return nil, ErrInvalidInput
	}
	// The address is stored trimmed and lower-cased: two spellings of one address
	// are one account. It is only a claim, so nothing here marks it verified.
	email, err := normalizeEmail(email)
	if err != nil {
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

	userID := uuid.New().String()
	unionID := uuid.New().String()
	ngacNode, err := s.createUserWithNode(ctx, newUser{
		ID: userID, Username: username, PasswordHash: hash, Email: email, UnionID: unionID, DisplayName: displayName,
	})
	if err != nil {
		return nil, err
	}

	tenantID, tName, role, err := s.resolveOrCreateTenant(ctx, userID, ngacNode, tenantName, displayName)
	if err != nil {
		return nil, fmt.Errorf("resolve tenant: %w", err)
	}

	token, sessionID, err := auth.GenerateToken(userID, username, ngacNode, tenantID)
	if err != nil {
		return nil, fmt.Errorf("generate token: %w", err)
	}

	membership, _ := s.store.GetTenantUser(ctx, tenantID, userID)
	openID := ""
	if membership != nil {
		openID = membership.OpenID
	}

	return &SignupResult{
		Token: token, SessionID: sessionID, UserID: userID, Username: username,
		NGACNodeID: ngacNode, Email: email, UnionID: unionID,
		TenantID: tenantID, TenantName: tName, TenantRole: role, OpenID: openID,
	}, nil
}

// resolveOrCreateTenant picks the tenant for a password signup.
//
// It never joins an existing tenant by email domain. Signup does not verify
// the email, so the domain is only a claim — anyone can register ceo@acme.com
// and would otherwise land inside Acme's tenant. Company membership comes from
// proof of ownership instead: a Google Workspace sign-in (the `hd` claim, see
// SignInWithGoogle) or an invitation.
func (s *Service) resolveOrCreateTenant(ctx context.Context, userID, ngacNodeID, tenantName, displayName string) (string, string, string, error) {
	// Explicit tenant name → a new tenant with that name.
	if tenantName != "" {
		return s.createTenantForUser(ctx, tenantName, userID, ngacNodeID)
	}
	// Otherwise the no-company case: a personal workspace.
	return s.createTenantForUser(ctx, personalWorkspaceName(displayName), userID, ngacNodeID)
}

// joinTenantByDomain adds the user as a member of the tenant that owns
// companyDomain and returns that tenant with the user's role in it. It returns a
// nil tenant when the domain is empty, is a public mailbox provider, or is owned
// by no tenant. A user who already has a membership there keeps it unchanged —
// including a disabled one, which signing in must not re-activate.
//
// This is the one place a domain turns into tenant membership. Callers must
// hold proof that the user controls an address at companyDomain — today only
// a verified Google Workspace `hd` claim qualifies. Password signup, legacy
// register and OTP (whose code is not delivered to the address) do not.
func (s *Service) joinTenantByDomain(ctx context.Context, companyDomain, userID, ngacNodeID string) (*store.Tenant, string, error) {
	companyDomain = normalizeDomain(companyDomain)
	if companyDomain == "" || IsPublicEmailDomain(companyDomain) {
		return nil, "", nil
	}

	tenant, err := s.store.FindTenantByDomain(ctx, companyDomain)
	if err != nil {
		return nil, "", fmt.Errorf("find tenant by domain: %w", err)
	}
	if tenant == nil {
		return nil, "", nil
	}

	existing, err := s.store.GetTenantUser(ctx, tenant.ID, userID)
	if err != nil {
		return nil, "", fmt.Errorf("get tenant user: %w", err)
	}
	if existing != nil {
		return tenant, existing.Role, nil
	}

	if err := s.joinTenant(ctx, tenant.ID, userID, ngacNodeID, "member"); err != nil {
		return nil, "", fmt.Errorf("join tenant: %w", err)
	}
	return tenant, "member", nil
}

// createTenantForUser creates a workspace/tenant, initializes tenant NGAC UAs, and assigns the user as owner.
func (s *Service) createTenantForUser(ctx context.Context, name, userID, ngacNodeID string) (string, string, string, error) {
	if s.wsClient == nil {
		return "", "", "", fmt.Errorf("workspace service unavailable")
	}

	ws, err := s.wsClient.CreateWorkspace(actingAs(ctx, userID, ngacNodeID), &workspacepb.CreateWorkspaceRequest{
		Name: name,
	})
	if err != nil {
		return "", "", "", fmt.Errorf("create workspace: %w", err)
	}

	// Create tenant-scoped NGAC UAs and assign under the workspace PC.
	if err := s.initTenantNGAC(ctx, ws.Id, ws.PcNodeId, ws.OwnersUaId, ws.MembersUaId); err != nil {
		// Not fatal, as before: the workspace exists and its owner is in it.
		// initTenantNGAC removed what it had half-built and creates only what is
		// missing, so running it again repairs the tenant.
		slog.Error("tenant NGAC init incomplete — users of this tenant will be denied",
			"tenant", ws.Id, "error", err)
	}

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
	email = strings.ToLower(strings.TrimSpace(email))
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

	return s.signinResultFor(ctx, user, "")
}

// SwitchTenant verifies membership and issues a new JWT scoped to the target tenant.
//
// It starts a fresh session rather than re-scoping the current one. The refresh
// token records the tenant it was issued for, so keeping the old family alive
// would mean the next refresh silently handed back a token for the tenant the
// user just left.
func (s *Service) SwitchTenant(ctx context.Context, userID, ngacNodeID, username, targetTenantID string) (string, string, *TenantInfo, error) {
	membership, err := s.store.GetTenantUser(ctx, targetTenantID, userID)
	if err != nil {
		return "", "", nil, fmt.Errorf("get tenant user: %w", err)
	}
	if membership == nil {
		return "", "", nil, ErrAccessDenied
	}

	token, sessionID, err := auth.GenerateToken(userID, username, ngacNodeID, targetTenantID)
	if err != nil {
		return "", "", nil, fmt.Errorf("generate token: %w", err)
	}

	info := &TenantInfo{
		ID: membership.TenantID, Name: membership.TenantName,
		Role: membership.Role, OpenID: membership.OpenID,
	}
	return token, sessionID, info, nil
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

	userID := uuid.New().String()
	unionID := uuid.New().String()
	ngacNode, err := s.createUserWithNode(ctx, newUser{
		ID: userID, Username: username, PasswordHash: hash, UnionID: unionID, DisplayName: username,
	})
	if err != nil {
		return nil, err
	}

	// Auto-provision workspace + tenant_users + #general channel
	s.autoProvisionWorkspace(ctx, userID, username, ngacNode)

	token, sessionID, err := auth.GenerateToken(userID, username, ngacNode, "")
	if err != nil {
		return nil, fmt.Errorf("generate token: %w", err)
	}

	return &AuthResponse{Token: token, SessionID: sessionID, UserID: userID, Username: username, NGACNodeID: ngacNode}, nil
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

	token, sessionID, err := auth.GenerateToken(user.ID, user.Username, user.NGACNodeID, "")
	if err != nil {
		return nil, fmt.Errorf("generate token: %w", err)
	}

	return &AuthResponse{Token: token, SessionID: sessionID, UserID: user.ID, Username: user.Username, NGACNodeID: user.NGACNodeID}, nil
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

// actingAs returns ctx carrying the user this service has just authenticated
// (password, Google token or OTP verified, or an existing session) as the
// caller of downstream RPCs. Signup and sign-in run before any token exists,
// so there is no REST-edge caller to forward; auth itself is the authority
// that established the identity, and the downstream services authorize it like
// any other caller.
func actingAs(ctx context.Context, userID, ngacNodeID string) context.Context {
	return grpcauth.WithCaller(ctx, grpcauth.Caller{UserID: userID, NGACNodeID: ngacNodeID})
}

// autoProvisionWorkspace creates a default workspace and #general channel (legacy flow).
func (s *Service) autoProvisionWorkspace(ctx context.Context, userID, username, ngacNodeID string) {
	if s.wsClient == nil {
		slog.Warn("workspace client unavailable, skipping auto-provision")
		return
	}

	wsName := fmt.Sprintf("%s's Workspace", username)
	ws, err := s.wsClient.CreateWorkspace(actingAs(ctx, userID, ngacNodeID), &workspacepb.CreateWorkspaceRequest{
		Name: wsName,
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
	_, err := s.msgClient.CreateChannel(actingAs(ctx, userID, ngacNodeID), &messagingpb.CreateChannelRequest{
		Name: "general", WorkspaceId: workspaceID, ChannelType: "workspace",
	})
	if err != nil {
		slog.Error("auto-provision #general channel failed", "workspace", workspaceID, "error", err)
		return
	}
	slog.Info("auto-provisioned #general channel", "workspace_id", workspaceID)
}

// newUser is everything needed to create an account and its graph node.
type newUser struct {
	ID           string
	Username     string
	PasswordHash string
	Email        string
	UnionID      string
	DisplayName  string
	Phone        string
	// EmailVerified marks the address as proved when the account is created:
	// only for a verified Google email or a code delivered to the address.
	EmailVerified bool
}

// createUserWithNode creates the user's U node and the users row, and returns
// the node ID. If the row cannot be written, the node is removed again, so a
// failed signup leaves no user node that belongs to nobody.
func (s *Service) createUserWithNode(ctx context.Context, u newUser) (string, error) {
	prov := provision.NewCreator(s.policyWrite)
	ngacNode, err := s.createUserNGACNode(ctx, prov, u.ID, u.Username)
	if err != nil {
		return "", fmt.Errorf("create ngac node: %w", err)
	}
	if err := s.store.CreateUser(ctx, u.ID, u.Username, u.PasswordHash, ngacNode, u.Email, u.UnionID, u.DisplayName, u.Phone); err != nil {
		return "", prov.Fail(ctx, fmt.Errorf("create user: %w", err))
	}
	if u.EmailVerified && u.Email != "" {
		if _, err := s.store.MarkEmailVerified(ctx, u.ID); err != nil {
			return "", prov.Fail(ctx, err)
		}
	}
	prov.Done()
	return ngacNode, nil
}

// createUserNGACNode creates a user node in the NGAC graph and assigns to
// PublicUsers. The node is named by the user's ID: a username is chosen by the
// user, and the graph resolves nodes by exact name. The username rides along as
// the display name. A failure after the node exists removes it.
func (s *Service) createUserNGACNode(ctx context.Context, prov *provision.Creator, userID, username string) (string, error) {
	userNode, err := prov.Node(ctx, &policypb.CreateNodeRequest{
		Name: ngac.UserNodeName(ngac.UserID(userID)), NodeType: ngac.TypeU,
		Properties: map[string]string{
			"type":               "user",
			"user_id":            userID,
			ngac.PropDisplayName: username,
		},
	})
	if err != nil {
		return "", fmt.Errorf("create node: %w", err)
	}

	publicUA, err := s.policyRead.FindNodeByName(ctx, &policypb.FindNodeByNameRequest{
		Name: ngac.NodePublicUsers, NodeType: ngac.TypeUA,
	})
	if err == nil && publicUA != nil {
		if err := prov.Assign(ctx, userNode.Id, publicUA.Id); err != nil {
			return "", prov.Fail(ctx, fmt.Errorf("assign to public: %w", err))
		}
	}

	return userNode.Id, nil
}

// initTenantNGAC creates the TenantMember and TenantOwner UAs for a new tenant
// and chains them into the workspace's existing NGAC graph.
//
// Safe to run again: a UA that already exists is reused rather than duplicated,
// and the edges are idempotent. A failure part way removes the UAs this call
// created, so it never leaves a UA that reaches no policy class.
func (s *Service) initTenantNGAC(ctx context.Context, tenantID, pcNodeID, ownersUAID, membersUAID string) error {
	tid := ngac.WorkspaceID(tenantID)
	prov := provision.NewCreator(s.policyWrite)

	memberUA, err := prov.EnsureNode(ctx, s.policyRead, &policypb.CreateNodeRequest{
		Name: ngac.TenantMemberUAName(tid), NodeType: ngac.TypeUA,
	})
	if err != nil {
		return prov.Fail(ctx, fmt.Errorf("create TenantMember UA: %w", err))
	}

	ownerUA, err := prov.EnsureNode(ctx, s.policyRead, &policypb.CreateNodeRequest{
		Name: ngac.TenantOwnerUAName(tid), NodeType: ngac.TypeUA,
	})
	if err != nil {
		return prov.Fail(ctx, fmt.Errorf("create TenantOwner UA: %w", err))
	}

	// Assign UAs under the workspace PC for NGAC scoping, then chain them:
	// TenantMember inherits workspace Members permissions, TenantOwner inherits
	// workspace Owners permissions.
	//
	// A UA that fails to attach reaches no Policy Class, so every check for
	// users under it denies. Silently returning here produced tenants where
	// nobody could see anything and no error had been recorded anywhere.
	assignments := []struct {
		what            string
		childID, parent string
	}{
		{"TenantMember under workspace PC", memberUA.Id, pcNodeID},
		{"TenantOwner under workspace PC", ownerUA.Id, pcNodeID},
		{"TenantMember under workspace Members", memberUA.Id, membersUAID},
		{"TenantOwner under workspace Owners", ownerUA.Id, ownersUAID},
	}
	for _, a := range assignments {
		if a.parent == "" {
			continue
		}
		if err := prov.Assign(ctx, a.childID, a.parent); err != nil {
			return prov.Fail(ctx, fmt.Errorf("%s: %w", a.what, err))
		}
	}
	prov.Done()

	slog.Info("tenant NGAC initialized", "tenant", tenantID, "member_ua", memberUA.Id, "owner_ua", ownerUA.Id)
	return nil
}

// assignUserToTenantNGAC assigns a user's NGAC node to the tenant's member/owner UAs.
// Uses find-or-log pattern: if the UA doesn't exist yet, logs an error instead of silently skipping.
func (s *Service) assignUserToTenantNGAC(ctx context.Context, tenantID, userNodeID string, isOwner bool) {
	// Assign to TenantMember UA
	memberUA, err := s.policyRead.FindNodeByName(ctx, &policypb.FindNodeByNameRequest{
		Name: ngac.TenantMemberUAName(ngac.WorkspaceID(tenantID)), NodeType: ngac.TypeUA,
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
		Name: ngac.TenantOwnerUAName(ngac.WorkspaceID(tenantID)), NodeType: ngac.TypeUA,
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
