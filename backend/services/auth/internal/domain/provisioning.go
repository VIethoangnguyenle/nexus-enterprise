package domain

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"ngac-platform/ngac"
	"ngac-platform/pkg/grpcauth"
	"ngac-platform/pkg/provision"
	messagingpb "ngac-platform/proto/messaging"
	policypb "ngac-platform/proto/policy"
	workspacepb "ngac-platform/proto/workspace"
)

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
	// A person whose address is already proved is created verified, in one
	// statement: creating the row and then marking it would leave an unverified
	// account behind if the second write failed.
	create := s.store.CreateUser
	if u.EmailVerified && u.Email != "" {
		create = s.store.CreateUserWithVerifiedEmail
	}
	if err := create(ctx, u.ID, u.Username, u.PasswordHash, ngacNode, u.Email, u.UnionID, u.DisplayName, u.Phone); err != nil {
		return "", prov.Fail(ctx, fmt.Errorf("create user: %w", err))
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
