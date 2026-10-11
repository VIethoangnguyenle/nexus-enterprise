package domain

import (
	"context"
	"errors"
	"fmt"

	"ngac-platform/services/asset/internal/store"
)

// ReasonPersonalWorkspace tells a refusal made because the workspace is personal
// from one made by the policy.
const ReasonPersonalWorkspace = "personal_workspace"

// The values of workspaces.type. Whether a workspace has assets is a business
// attribute of the workspace, not a node of the policy graph.
const (
	workspaceTypePersonal     = "personal"
	workspaceTypeOrganization = "organization"
)

// requireOrganization refuses unless the workspace is an organization. A
// personal workspace has no assets and no asset requests. It runs before the
// policy check, and it fails closed: a workspace that cannot be found or whose
// type is not exactly "organization" is refused, and a database failure is ours
// (unclassified, so a transport answers 500), never an allow.
func requireOrganization(ctx context.Context, st *store.Store, workspaceID string) error {
	kind, err := st.WorkspaceType(ctx, workspaceID)
	switch {
	case errors.Is(err, store.ErrNotFound):
		return denied("not available in this workspace")
	case err != nil:
		return fmt.Errorf("check workspace type: %w", err)
	case kind == workspaceTypePersonal:
		return Refuse(ErrAccessDenied, ReasonPersonalWorkspace, "not available in a personal workspace")
	case kind != workspaceTypeOrganization:
		return denied("not available in this workspace")
	}
	return nil
}
