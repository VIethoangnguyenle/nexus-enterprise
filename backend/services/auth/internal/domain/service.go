package domain

import (
	"context"

	"github.com/redis/go-redis/v9"

	messagingpb "ngac-platform/proto/messaging"
	policypb "ngac-platform/proto/policy"
	workspacepb "ngac-platform/proto/workspace"
	"ngac-platform/services/auth/internal/store"
)

// AuthStore defines the database operations the domain layer needs.
type AuthStore interface {
	CreateUser(ctx context.Context, id, username, password, ngacNodeID, email, unionID, displayName, phone string) error
	// CreateUserWithVerifiedEmail creates the user with the address already proved.
	CreateUserWithVerifiedEmail(ctx context.Context, id, username, password, ngacNodeID, email, unionID, displayName, phone string) error
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

// --- Private helpers ---

// --- Private helpers ---
