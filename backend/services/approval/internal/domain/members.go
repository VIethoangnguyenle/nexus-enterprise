package domain

import (
	"context"
	"fmt"
)

// RequireMember refuses a person who is not in the workspace right now. The token
// names a tenant, but a token outlives the membership it was issued for (a removed
// person holds one for up to its lifetime), so an approval call is decided on the
// graph at the moment it is made. ErrAccessDenied says the graph does not hold
// them; any other error means it could not be asked, and the call stops there.
func (s *Service) RequireMember(ctx context.Context, userNodeID, workspaceID string) error {
	if userNodeID == "" || workspaceID == "" {
		return fmt.Errorf("%w: no workspace member in the request", ErrAccessDenied)
	}
	ok, err := s.policy.IsWorkspaceMember(ctx, userNodeID, workspaceID)
	if err != nil {
		return fmt.Errorf("%w: check workspace membership: %v", ErrUnavailable, err)
	}
	if !ok {
		return fmt.Errorf("%w: not a member of this workspace", ErrAccessDenied)
	}
	return nil
}
