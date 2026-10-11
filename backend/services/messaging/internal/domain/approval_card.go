package domain

import (
	"context"
	"errors"
	"fmt"

	"ngac-platform/pkg/grpcauth"
	"ngac-platform/services/messaging/internal/store"
)

// linkedApproval is the linked_entity_type of an approval card.
const linkedApproval = "approval"

// The values of workspaces.type.
const (
	workspaceTypePersonal     = "personal"
	workspaceTypeOrganization = "organization"
)

// requireApprovalsAvailable refuses a message that links an approval request
// unless the workspace it is sent in is an organization: a personal workspace
// has no approvals, and the owning service refuses every one of them there.
//
// The workspace is the channel's; a direct message has none, so the workspace
// the sender is signed in to stands in, as it does for the channel's events.
// It fails closed: no workspace to judge by, a workspace that cannot be found,
// or a type that is not exactly "organization" is a refusal, and a failure to
// read the type is an error, never an allow. Other kinds of message are not
// looked at.
func (s *Service) requireApprovalsAvailable(ctx context.Context, ch *store.Channel, linkedType string) error {
	if linkedType != linkedApproval {
		return nil
	}
	workspaceID := ch.WorkspaceID
	if workspaceID == "" {
		workspaceID = grpcauth.CallerFrom(ctx).TenantID
	}
	if workspaceID == "" {
		return fmt.Errorf("%w: approvals are not available here", ErrAccessDenied)
	}
	kind, err := s.store.WorkspaceType(ctx, workspaceID)
	switch {
	case errors.Is(err, store.ErrWorkspaceNotFound):
		return fmt.Errorf("%w: approvals are not available here", ErrAccessDenied)
	case err != nil:
		return fmt.Errorf("check workspace type: %w", err)
	case kind == workspaceTypePersonal:
		return fmt.Errorf("%w: approvals are not available in a personal workspace", ErrAccessDenied)
	case kind != workspaceTypeOrganization:
		return fmt.Errorf("%w: approvals are not available here", ErrAccessDenied)
	}
	return nil
}
