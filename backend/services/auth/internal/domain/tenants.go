package domain

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	workspacepb "ngac-platform/proto/workspace"
	"ngac-platform/services/auth/internal/auth"
	"ngac-platform/services/auth/internal/store"
)

// joinTenantByDomain adds the user as a member of the tenant that owns
// companyDomain and returns that tenant with the user's role in it. It returns a
// nil tenant when the domain is empty, is a public mailbox provider, or is owned
// by no tenant. A user who already has a membership there keeps it unchanged and
// is not joined again: a removed or disabled one stays as it is (nil tenant),
// because the domain claim is proof of an address, not an invitation, and signing
// in must not undo a removal or a suspension.
//
// A join that fails (the graph or the table cannot be written) is logged and
// answered with a nil tenant, never an error: signing in does not depend on it.
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
		if existing.Status != "active" {
			return nil, "", nil
		}
		return tenant, existing.Role, nil
	}

	if err := s.joinTenant(ctx, tenant.ID, userID, ngacNodeID, "member"); err != nil {
		// Signing in does not depend on being joined: the person gets their own
		// workspace, and the next sign-in tries the join again.
		slog.Error("could not join the tenant that owns the domain", "tenant", tenant.ID, "error", err)
		return nil, "", nil
	}
	return tenant, "member", nil
}

// createTenantForUser creates a workspace/tenant, initializes tenant NGAC UAs,
// and assigns the user as owner. It is the lenient path sign-in uses: a tenant
// graph that is only partly built is logged and repaired later, because the
// person must still get into their new account.
func (s *Service) createTenantForUser(ctx context.Context, name, wsType, userID, ngacNodeID string) (string, string, string, error) {
	return s.provisionTenant(ctx, name, wsType, userID, ngacNodeID, false)
}

// provisionTenant is createTenantForUser with a choice of what a failure means.
//
// Lenient (sign-in): a tenant graph that did not finish, or a #general channel
// that could not be made, is logged and tolerated.
//
// Strict (a person creating a workspace on purpose): any failure after the
// workspace exists undoes it through the workspace service and is returned, so
// no half-built workspace that nobody can use stays in the system.
func (s *Service) provisionTenant(ctx context.Context, name, wsType, userID, ngacNodeID string, strict bool) (string, string, string, error) {
	if s.wsClient == nil {
		return "", "", "", fmt.Errorf("workspace service unavailable")
	}

	ws, err := s.wsClient.CreateWorkspace(actingAs(ctx, userID, ngacNodeID), &workspacepb.CreateWorkspaceRequest{
		Name: name, Type: wsType,
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
		// Graph first, row after: the owner edge already exists (the workspace service
		// made it) and is confirmed here; a failure of either step undoes the whole
		// workspace, edges included.
		if _, err := s.assignTenantNGAC(ctx, ws.Id, userID, ngacNodeID, true); err != nil {
			return fail("assign owner", err)
		}
		if err := s.store.InsertTenantUser(ctx, ws.Id, userID, "owner", "active", ngacNodeID); err != nil {
			return fail("insert tenant user", err)
		}
		if err := s.provisionChannel(ctx, ws.Id, userID, ngacNodeID); err != nil {
			return fail("create #general", err)
		}
		return ws.Id, name, "owner", nil
	}

	if err := s.joinTenant(ctx, ws.Id, userID, ngacNodeID, "owner"); err != nil {
		// The lenient path: the workspace exists and the owner is in its graph, so the
		// person still gets in; the row is what is missing and the next sign-in writes it.
		slog.Error("owner not listed in the new workspace", "tenant", ws.Id, "error", err)
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

// takeBackEdges answers a row that could not be written after the edges were. Three
// cases keep it from damaging what another call built:
//   - a cancelled or timed-out write may have committed, so nothing is taken back (an
//     edge without a row is repaired at the next sign-in; a row without its edge would
//     be a person listed in a workspace the graph denies them);
//   - an edge the person already had is never in the list (assignTenantNGAC reports only
//     the edges it made);
//   - after taking edges back the row is read again: a racing join may have written an
//     active row in the meantime, and then its edges are put back.
func (s *Service) takeBackEdges(ctx context.Context, tenantID, userID, nodeID string, created []graphEdge, cause error) {
	if len(created) == 0 {
		return
	}
	if errors.Is(cause, context.Canceled) || errors.Is(cause, context.DeadlineExceeded) || ctx.Err() != nil {
		slog.Warn("row write ended by its context; the edges are kept", "tenant", tenantID, "user", userID)
		return
	}
	s.undoEdges(ctx, userID, nodeID, created)
	row, err := s.store.GetTenantUser(ctx, tenantID, userID)
	if err != nil || row == nil || row.Status != "active" {
		return
	}
	s.restoreEdges(ctx, userID, nodeID, created)
}

// joinTenant puts the user in the tenant: the graph first, the tenant_users row after.
//
// The graph is the truth and the row follows it, so a failure never leaves an active
// row for someone the graph does not hold. If the row cannot be written, the edges
// this call made are taken back (an edge the person already had stays). A join that
// fails is returned; lenient callers (sign-in) log it and carry on.
func (s *Service) joinTenant(ctx context.Context, tenantID, userID, ngacNodeID, role string) error {
	created, err := s.assignTenantNGAC(ctx, tenantID, userID, ngacNodeID, role == "owner")
	if err != nil {
		return fmt.Errorf("assign user to tenant UAs: %w", err)
	}
	if err := s.store.InsertTenantUser(ctx, tenantID, userID, role, "active", ngacNodeID); err != nil {
		s.takeBackEdges(ctx, tenantID, userID, ngacNodeID, created, err)
		return fmt.Errorf("insert tenant user: %w", err)
	}
	// A member of the workspace is a member of its #general; the owner of a
	// workspace being made is put there when the channel is created.
	if role != "owner" {
		s.joinWorkspaceChannel(ctx, tenantID, userID, ngacNodeID)
	}
	return nil
}

// SwitchTenant verifies membership and issues a new JWT scoped to the target tenant.
//
// It starts a fresh session rather than re-scoping the current one. The refresh
// token records the tenant it was issued for, so keeping the old family alive
// would mean the next refresh silently handed back a token for the tenant the
// user just left.
func (s *Service) SwitchTenant(ctx context.Context, userID, ngacNodeID, username, targetTenantID string) (string, string, *TenantInfo, error) {
	// Only an active member may act as the tenant. A suspended or not yet accepted
	// membership still has a row, and a token scoped to the tenant would be accepted by
	// every service that reads the tenant from the token. The row is a projection of the
	// graph and can lag it, so the graph has the last word; when it cannot be asked the
	// answer is "unavailable", never a token. One read of the row serves both.
	membership, err := s.activeMemberRow(ctx, targetTenantID, userID, ngacNodeID)
	if err != nil {
		return "", "", nil, err
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
			Type: membership.TenantType,
		}
	}
	return uInfo, tInfo, nil
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
