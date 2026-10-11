package domain

import (
	"context"
	"errors"
	"fmt"

	"ngac-platform/pkg/grpcauth"
)

// The values of workspaces.type. Whether a workspace has approvals is a business
// attribute of the workspace, not a node of the policy graph.
const (
	workspaceTypePersonal     = "personal"
	workspaceTypeOrganization = "organization"
)

// WorkspaceStore answers what kind of workspace a tenant is.
type WorkspaceStore interface {
	// WorkspaceType returns "personal" or "organization", or ErrNotFound for a
	// workspace that does not exist.
	WorkspaceType(ctx context.Context, workspaceID string) (string, error)
}

// RequireOrganization refuses unless the tenant is an organization workspace.
// A personal workspace has no approvals. Every entry point of the service runs
// it before anything else, so it comes before any policy check or data access.
//
// It fails closed: a tenant that cannot be found, or whose type is not exactly
// "organization", is refused (ErrAccessDenied); a failure to read the type is
// ours (unclassified, so a transport answers 500), never an allow. The messages
// name no ID.
func (s *Service) RequireOrganization(ctx context.Context, tenantID string) error {
	kind, err := s.store.WorkspaceType(ctx, tenantID)
	switch {
	case errors.Is(err, ErrNotFound):
		return fmt.Errorf("%w: not available in this workspace", ErrAccessDenied)
	case err != nil:
		return fmt.Errorf("check workspace type: %w", err)
	case kind == workspaceTypePersonal:
		return fmt.Errorf("%w: not available in a personal workspace", ErrAccessDenied)
	case kind != workspaceTypeOrganization:
		return fmt.Errorf("%w: not available in this workspace", ErrAccessDenied)
	}
	return nil
}

// requireCallerOrganization is RequireOrganization for the workspace the caller
// is signed in to — the one whose schema the call reads and writes. The caller
// comes from the context (REST: the verified claims; gRPC: the signed identity),
// never from a request field.
func (s *Service) requireCallerOrganization(ctx context.Context) error {
	return s.RequireOrganization(ctx, grpcauth.CallerFrom(ctx).TenantID)
}
