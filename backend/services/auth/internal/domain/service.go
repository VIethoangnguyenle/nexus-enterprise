package domain

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

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
	InsertTenantUser(ctx context.Context, tenantID, userID, role, status, ngacNodeID string) error
	ListTenantsByUser(ctx context.Context, userID string) ([]store.TenantMembership, error)
	GetTenantUser(ctx context.Context, tenantID, userID string) (*store.TenantMembership, error)
	FindTenantByDomain(ctx context.Context, domain string) (*store.Tenant, error)
	// UpdateProfile writes only the fields set in ch and marks the profile done.
	UpdateProfile(ctx context.Context, userID string, ch store.ProfileChanges) error
	// ListWorkspaceSummaries lists the workspaces a person actively belongs to.
	ListWorkspaceSummaries(ctx context.Context, userID string) ([]store.WorkspaceSummary, error)
	ListContactsByWorkspace(ctx context.Context, workspaceID string, f store.ContactFilter) ([]store.User, int, *store.ContactCursor, error)

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
	// Department is the one an administrator assigned in this workspace.
	Department string
}

// UserInfo is the domain representation of a user (no password).
type UserInfo struct {
	ID          string
	Username    string
	NGACNodeID  string
	Email       string
	UnionID     string
	DisplayName string
	Title       string
	Location    string
	AvatarURL   string
	// EmailVerified: the address on the account has been proved by its owner.
	EmailVerified bool
	// NeedsProfile: the person has not yet been asked for the name colleagues see.
	NeedsProfile bool
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

// joinTenantByDomain adds the user as a member of the tenant that owns
// companyDomain and returns that tenant with the user's role in it. It returns a
// nil tenant when the domain is empty, is a public mailbox provider, or is owned
// by no tenant. A user who already has a membership there keeps it unchanged —
// including a disabled one, which signing in must not re-activate.
//
// This is the one place a domain turns into tenant membership. Callers must
// hold proof that the user controls an address at companyDomain — today only
// a verified Google Workspace `hd` claim qualifies. A one-time code (not
// delivered to the address) and a consumer Google account do not.
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

// createTenantForUser creates a workspace/tenant, initializes tenant NGAC UAs,
// and assigns the user as owner. It is the lenient path sign-in uses: a tenant
// graph that is only partly built is logged and repaired later, because the
// person must still get into their new account.
func (s *Service) createTenantForUser(ctx context.Context, name, userID, ngacNodeID string) (string, string, string, error) {
	return s.provisionTenant(ctx, name, userID, ngacNodeID, false)
}

// provisionTenant is createTenantForUser with a choice of what a failure means.
//
// Lenient (sign-in): a tenant graph that did not finish, or a #general channel
// that could not be made, is logged and tolerated.
//
// Strict (a person creating a workspace on purpose): any failure after the
// workspace exists undoes it through the workspace service and is returned, so
// no half-built workspace that nobody can use stays in the system.
func (s *Service) provisionTenant(ctx context.Context, name, userID, ngacNodeID string, strict bool) (string, string, string, error) {
	if s.wsClient == nil {
		return "", "", "", fmt.Errorf("workspace service unavailable")
	}

	ws, err := s.wsClient.CreateWorkspace(actingAs(ctx, userID, ngacNodeID), &workspacepb.CreateWorkspaceRequest{
		Name: name,
	})
	if err != nil {
		return "", "", "", fmt.Errorf("create workspace: %w", err)
	}
	fail := func(step string, cause error) (string, string, string, error) {
		s.undoWorkspace(ctx, ws.Id, userID, ngacNodeID)
		return "", "", "", fmt.Errorf("%s: %w", step, cause)
	}

	// Create tenant-scoped NGAC UAs and assign under the workspace PC.
	if err := s.initTenantNGAC(ctx, ws.Id, ws.PcNodeId, ws.OwnersUaId, ws.MembersUaId); err != nil {
		if strict {
			return fail("init tenant graph", err)
		}
		// Not fatal here, as before: the workspace exists and its owner is in
		// it. initTenantNGAC removed what it had half-built and creates only
		// what is missing, so running it again repairs the tenant.
		slog.Error("tenant NGAC init incomplete — users of this tenant will be denied",
			"tenant", ws.Id, "error", err)
	}

	if strict {
		if err := s.store.InsertTenantUser(ctx, ws.Id, userID, "owner", "active", ngacNodeID); err != nil {
			return fail("insert tenant user", err)
		}
		if err := s.assignTenantNGAC(ctx, ws.Id, ngacNodeID, true); err != nil {
			return fail("assign owner", err)
		}
		if err := s.provisionChannel(ctx, ws.Id, userID, ngacNodeID); err != nil {
			return fail("create #general", err)
		}
		return ws.Id, name, "owner", nil
	}

	if err := s.joinTenant(ctx, ws.Id, userID, ngacNodeID, "owner"); err != nil {
		return "", "", "", fmt.Errorf("join as owner: %w", err)
	}
	s.autoProvisionChannel(ctx, ws.Id, userID, ngacNodeID)
	return ws.Id, name, "owner", nil
}

// undoWorkspace removes a workspace this service has just created, through the
// workspace service (graph, rows, drive, channels). It runs on a context
// detached from the request: the commonest reason provisioning fails is that
// the request was cancelled, and a cleanup that dies with it would leave the
// very orphan it exists to remove. A failure is logged loudly; the caller still
// reports the original error.
func (s *Service) undoWorkspace(ctx context.Context, workspaceID, userID, ngacNodeID string) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 20*time.Second)
	defer cancel()
	if _, err := s.wsClient.DeleteWorkspace(actingAs(ctx, userID, ngacNodeID), &workspacepb.DeleteWorkspaceRequest{WorkspaceId: workspaceID}); err != nil {
		slog.Error("could not remove a workspace whose provisioning failed; an orphan remains",
			"workspace", workspaceID, "error", err)
		return
	}
	slog.Warn("removed a workspace whose provisioning failed", "workspace", workspaceID)
}

// joinTenant creates the tenant_users record and assigns the user in NGAC.
func (s *Service) joinTenant(ctx context.Context, tenantID, userID, ngacNodeID, role string) error {
	if err := s.store.InsertTenantUser(ctx, tenantID, userID, role, "active", ngacNodeID); err != nil {
		return fmt.Errorf("insert tenant user: %w", err)
	}
	s.assignUserToTenantNGAC(ctx, tenantID, ngacNodeID, role == "owner")
	return nil
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
	// Only an active member may act as the tenant. A suspended or not yet
	// accepted membership still has a row, and a token scoped to the tenant
	// would be accepted by every service that reads the tenant from the token.
	if membership == nil || membership.Status != "active" {
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

	uInfo := userInfo(user)

	if tenantID == "" {
		return uInfo, nil, nil
	}

	membership, err := s.store.GetTenantUser(ctx, tenantID, userID)
	if err != nil {
		return nil, nil, fmt.Errorf("get tenant user: %w", err)
	}

	var tInfo *TenantInfo
	if membership != nil && membership.Status == "active" {
		tInfo = &TenantInfo{
			ID: membership.TenantID, Name: membership.TenantName,
			Role: membership.Role, OpenID: membership.OpenID, Department: membership.DepartmentName,
		}
	}
	return uInfo, tInfo, nil
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
	return userInfo(user), nil
}

// userInfo is the domain view of an account (no password).
func userInfo(u *store.User) *UserInfo {
	return &UserInfo{
		ID: u.ID, Username: u.Username, NGACNodeID: u.NGACNodeID,
		Email: u.Email, UnionID: u.UnionID, DisplayName: u.DisplayName,
		Title: u.Title, Location: u.Location, AvatarURL: u.AvatarURL,
		EmailVerified: u.EmailVerified, NeedsProfile: !u.ProfileCompleted,
	}
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

// ContactQuery is what a member asks of the workspace directory.
type ContactQuery struct {
	Department string
	Location   string
	Search     string
	// Cursor is the NextCursor of the page before; empty for the first page.
	Cursor string
	Limit  int
}

// Directory page sizes: the default, and the most one request may ask for.
const (
	DefaultContactsLimit = 50
	MaxContactsLimit     = 200
	maxCursorBytes       = 512
)

// encodeCursor and decodeCursor keep the keyset position opaque to clients: it
// is a place to continue from, not something to build.
func encodeCursor(c *store.ContactCursor) string {
	if c == nil {
		return ""
	}
	raw, _ := json.Marshal(c)
	return base64.RawURLEncoding.EncodeToString(raw)
}

func decodeCursor(s string) (*store.ContactCursor, error) {
	if s == "" {
		return nil, nil
	}
	if len(s) > maxCursorBytes {
		return nil, ErrInvalidInput
	}
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return nil, ErrInvalidInput
	}
	var c store.ContactCursor
	if err := json.Unmarshal(raw, &c); err != nil || c.ID == "" {
		return nil, ErrInvalidInput
	}
	return &c, nil
}

// ListContacts returns one page of the workspace directory, the true number of
// people matching, and the cursor of the next page ("" on the last). Only an
// active member of that workspace may read it. A person's display name is empty
// until they have saved their profile: the directory never shows a login handle
// as a name.
func (s *Service) ListContacts(ctx context.Context, callerID, workspaceID string, q ContactQuery) ([]ContactInfo, int, string, error) {
	if workspaceID == "" {
		return nil, 0, "", ErrInvalidInput
	}
	membership, err := s.store.GetTenantUser(ctx, workspaceID, callerID)
	if err != nil {
		return nil, 0, "", fmt.Errorf("check membership: %w", err)
	}
	if membership == nil || membership.Status != "active" {
		return nil, 0, "", ErrAccessDenied
	}
	limit := q.Limit
	if limit <= 0 {
		limit = DefaultContactsLimit
	}
	if limit > MaxContactsLimit || len([]rune(q.Search)) > 100 {
		return nil, 0, "", ErrInvalidInput
	}
	after, err := decodeCursor(q.Cursor)
	if err != nil {
		return nil, 0, "", err
	}
	users, total, next, err := s.store.ListContactsByWorkspace(ctx, workspaceID, store.ContactFilter{
		Department: strings.TrimSpace(q.Department), Location: strings.TrimSpace(q.Location),
		Search: strings.TrimSpace(q.Search), Limit: limit, After: after,
	})
	if err != nil {
		return nil, 0, "", fmt.Errorf("list contacts: %w", err)
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
	return contacts, total, encodeCursor(next), nil
}

// --- Private helpers ---

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

// autoProvisionChannel creates a #general channel in the workspace. A failure
// is logged and tolerated: it is the lenient path's.
func (s *Service) autoProvisionChannel(ctx context.Context, workspaceID, userID, ngacNodeID string) {
	if s.msgClient == nil {
		slog.Warn("messaging client unavailable, skipping #general channel")
		return
	}
	if err := s.provisionChannel(ctx, workspaceID, userID, ngacNodeID); err != nil {
		slog.Error("auto-provision #general channel failed", "workspace", workspaceID, "error", err)
		return
	}
	slog.Info("auto-provisioned #general channel", "workspace_id", workspaceID)
}

// provisionChannel creates #general and reports a failure. Without a messaging
// client there is nothing to create and nothing failed.
func (s *Service) provisionChannel(ctx context.Context, workspaceID, userID, ngacNodeID string) error {
	if s.msgClient == nil {
		return nil
	}
	_, err := s.msgClient.CreateChannel(actingAs(ctx, userID, ngacNodeID), &messagingpb.CreateChannelRequest{
		Name: "general", WorkspaceId: workspaceID, ChannelType: "workspace",
	})
	return err
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

// assignUserToTenantNGAC assigns a user's NGAC node to the tenant's member/owner
// UAs. A failure is logged, not returned: see assignTenantNGAC for the strict form.
func (s *Service) assignUserToTenantNGAC(ctx context.Context, tenantID, userNodeID string, isOwner bool) {
	if err := s.assignTenantNGAC(ctx, tenantID, userNodeID, isOwner); err != nil {
		slog.Error("assign user to tenant UAs failed", "tenant", tenantID, "error", err)
	}
}

// assignTenantNGAC puts the user under the tenant's TenantMember UA, and under
// TenantOwner when isOwner. A UA that does not exist is an error, not a skip:
// without them every check for the user denies.
func (s *Service) assignTenantNGAC(ctx context.Context, tenantID, userNodeID string, isOwner bool) error {
	memberUA, err := s.policyRead.FindNodeByName(ctx, &policypb.FindNodeByNameRequest{
		Name: ngac.TenantMemberUAName(ngac.WorkspaceID(tenantID)), NodeType: ngac.TypeUA,
	})
	if err != nil {
		return fmt.Errorf("TenantMember UA not found — was initTenantNGAC called?: %w", err)
	}
	if _, err := s.policyWrite.CreateAssignment(ctx, &policypb.CreateAssignmentRequest{
		ChildId: userNodeID, ParentId: memberUA.Id,
	}); err != nil {
		return fmt.Errorf("assign to tenant member UA: %w", err)
	}

	if !isOwner {
		return nil
	}
	ownerUA, err := s.policyRead.FindNodeByName(ctx, &policypb.FindNodeByNameRequest{
		Name: ngac.TenantOwnerUAName(ngac.WorkspaceID(tenantID)), NodeType: ngac.TypeUA,
	})
	if err != nil {
		return fmt.Errorf("TenantOwner UA not found — was initTenantNGAC called?: %w", err)
	}
	if _, err := s.policyWrite.CreateAssignment(ctx, &policypb.CreateAssignmentRequest{
		ChildId: userNodeID, ParentId: ownerUA.Id,
	}); err != nil {
		return fmt.Errorf("assign to tenant owner UA: %w", err)
	}
	return nil
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
