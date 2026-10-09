package domain

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"

	"ngac-platform/services/auth/internal/auth"
	"ngac-platform/services/auth/internal/store"
)

// ProviderGoogle is the provider key under which Google identities are stored.
const ProviderGoogle = "google"

// ExternalIdentity is what an identity provider vouched for after its token was
// verified. Only fields taken from a verified ID token belong here.
type ExternalIdentity struct {
	Provider string
	// Subject is the provider's stable, never-reassigned user ID (`sub`).
	// It — not the email — is what identifies a returning user.
	Subject       string
	Email         string
	EmailVerified bool
	// HostedDomain is Google's `hd` claim. Google sets it only for Google
	// Workspace accounts, and it proves the account is managed by that
	// domain. It is the only basis on which a Google user is placed in a
	// company tenant.
	HostedDomain string
	DisplayName  string
}

// SignInWithGoogle signs in (or registers) the person behind a verified Google
// ID token and returns the same result shape as password Signin.
//
// Account resolution, in order: the linked (provider, subject); else an
// existing account with the same verified email, which gets linked; else a new
// account. Tenant resolution: with a hosted domain, the user is placed in the
// tenant that owns it (created on first use, with the user as owner). Without
// one — consumer accounts — the email domain is ignored entirely; a new user
// gets a personal workspace exactly as an OTP or signup user with no company
// does, and an existing user keeps the tenants they already have.
func (s *Service) SignInWithGoogle(ctx context.Context, ext ExternalIdentity) (*SigninResult, error) {
	email := strings.ToLower(strings.TrimSpace(ext.Email))
	if ext.Subject == "" || email == "" {
		return nil, ErrInvalidInput
	}
	if !ext.EmailVerified {
		return nil, ErrEmailNotVerified
	}

	hostedDomain := normalizeDomain(ext.HostedDomain)
	if IsPublicEmailDomain(hostedDomain) {
		hostedDomain = ""
	}

	user, created, err := s.resolveGoogleUser(ctx, ext.Subject, email, strings.TrimSpace(ext.DisplayName))
	if err != nil {
		return nil, err
	}

	preferredTenant := ""
	switch {
	case hostedDomain != "":
		preferredTenant, err = s.joinOrCreateDomainTenant(ctx, hostedDomain, user)
	case created:
		preferredTenant, _, _, err = s.createTenantForUser(ctx, personalWorkspaceName(user.DisplayName), user.ID, user.NGACNodeID)
	}
	if err != nil {
		return nil, fmt.Errorf("resolve tenant: %w", err)
	}

	return s.signinResultFor(ctx, user, preferredTenant)
}

// resolveGoogleUser finds or creates the account for a Google subject and makes
// sure the (provider, subject) link exists. created reports a brand-new account.
func (s *Service) resolveGoogleUser(ctx context.Context, subject, email, displayName string) (*store.User, bool, error) {
	user, err := s.store.FindUserByIdentity(ctx, ProviderGoogle, subject)
	if err != nil {
		return nil, false, fmt.Errorf("find identity: %w", err)
	}
	if user != nil {
		return user, false, nil
	}

	created := false
	user, err = s.store.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, false, fmt.Errorf("get user by email: %w", err)
	}
	if user != nil {
		linked, err := s.store.GetIdentitySubject(ctx, ProviderGoogle, user.ID)
		if err != nil {
			return nil, false, fmt.Errorf("get identity: %w", err)
		}
		if linked != "" && linked != subject {
			return nil, false, ErrIdentityConflict
		}
	} else {
		user, err = s.createExternalUser(ctx, email, displayName)
		if err != nil {
			return nil, false, err
		}
		created = true
	}

	if err := s.store.LinkIdentity(ctx, ProviderGoogle, subject, user.ID, email); err != nil {
		return nil, false, fmt.Errorf("link identity: %w", err)
	}
	return user, created, nil
}

// createExternalUser registers a password-less account for a verified email.
func (s *Service) createExternalUser(ctx context.Context, email, displayName string) (*store.User, error) {
	username := emailToUsername(email)
	if taken, err := s.store.GetUserByUsername(ctx, username); err != nil {
		return nil, fmt.Errorf("check username: %w", err)
	} else if taken != nil {
		username = username + "." + uuid.New().String()[:8]
	}
	if displayName == "" {
		displayName = username
	}

	ngacNode, err := s.createUserNGACNode(ctx, username)
	if err != nil {
		return nil, fmt.Errorf("create ngac node: %w", err)
	}

	userID := uuid.New().String()
	unionID := uuid.New().String()
	if err := s.store.CreateUser(ctx, userID, username, "", ngacNode, email, unionID, displayName, ""); err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}
	slog.Info("registered user from external identity", "user_id", userID, "provider", ProviderGoogle)

	return &store.User{
		ID: userID, Username: username, NGACNodeID: ngacNode,
		Email: email, UnionID: unionID, DisplayName: displayName,
	}, nil
}

// joinOrCreateDomainTenant places the user in the tenant that owns a verified
// company domain, creating that tenant (user as owner) if none does yet.
func (s *Service) joinOrCreateDomainTenant(ctx context.Context, companyDomain string, user *store.User) (string, error) {
	tenant, _, err := s.joinTenantByDomain(ctx, companyDomain, user.ID, user.NGACNodeID)
	if err != nil {
		return "", err
	}
	if tenant != nil {
		return tenant.ID, nil
	}

	tenantID, _, _, err := s.createTenantForUser(ctx, companyDomain, user.ID, user.NGACNodeID)
	if err != nil {
		return "", err
	}
	claimed, err := s.store.ClaimTenantDomain(ctx, tenantID, companyDomain)
	switch {
	case err != nil:
		// The tenant exists and the user owns it; failing the sign-in now would
		// only strand them. Colleagues will not auto-join until it is fixed.
		slog.Error("claim tenant domain failed", "tenant", tenantID, "domain", companyDomain, "error", err)
	case !claimed:
		// Another sign-in from the same domain won the race and created its own
		// tenant first. This one stays a regular tenant without a domain.
		slog.Warn("tenant domain already claimed", "tenant", tenantID, "domain", companyDomain)
	}
	return tenantID, nil
}

// signinResultFor mints a session for user, scoped to preferredTenant when the
// user is an active member of it, else to the usual default tenant.
func (s *Service) signinResultFor(ctx context.Context, user *store.User, preferredTenant string) (*SigninResult, error) {
	tenants, err := s.store.ListTenantsByUser(ctx, user.ID)
	if err != nil {
		return nil, fmt.Errorf("list tenants: %w", err)
	}

	defaultTenantID := ""
	for _, t := range tenants {
		if preferredTenant != "" && t.TenantID == preferredTenant {
			defaultTenantID = preferredTenant
		}
	}
	if defaultTenantID == "" {
		defaultTenantID = s.selectDefaultTenant(tenants)
	}

	token, sessionID, err := auth.GenerateToken(user.ID, user.Username, user.NGACNodeID, defaultTenantID)
	if err != nil {
		return nil, fmt.Errorf("generate token: %w", err)
	}

	result := &SigninResult{
		Token: token, SessionID: sessionID, UserID: user.ID, Username: user.Username,
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

// personalWorkspaceName names the workspace created for a user with no company.
func personalWorkspaceName(displayName string) string {
	return fmt.Sprintf("%s's Workspace", displayName)
}
