package domain

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
)

// ErrLastOwner is a refusal: the person leaving is the workspace's only Owner,
// and a workspace without one cannot be managed by anybody.
var ErrLastOwner = errors.New("last owner")

// LeaveWorkspace removes the caller from a workspace they belong to.
//
// It takes no admin right: anyone may walk out of a workspace, and nobody may
// make another person leave through it (the person is always the verified
// caller). What it must not do is orphan the workspace, so the Owners are read
// and the removal done inside the workspace's owner lock, the same lock owner
// changes take: two Owners leaving at once cannot both be "the other Owner".
//
// Access goes through the policy writer (every UA of the workspace the person
// is assigned to, so the change reaches every decision point), then the listing
// in the directory, and last any invitation still open to their verified
// address, so an old offer cannot bring them straight back.
func (s *Service) LeaveWorkspace(ctx context.Context, callerNodeID, wsID string) error {
	ws, err := s.authorizeMember(ctx, callerNodeID, wsID)
	if err != nil {
		return err
	}
	leave := func(ctx context.Context) error {
		_, owners, err := s.ownersOf(ctx, ws)
		if err != nil {
			return err
		}
		if containsNode(owners, callerNodeID) && len(owners) <= 1 {
			return fmt.Errorf("%w: the last owner cannot leave", ErrLastOwner)
		}
		return s.detachMember(ctx, ws, callerNodeID)
	}
	if err := s.store.WithOwnerLock(ctx, ws.ID, leave); err != nil {
		return err
	}
	s.withdrawOffers(ctx, ws.ID, callerNodeID)
	if s.directory != nil {
		if err := s.directory.RemoveTenantUser(ctx, ws.ID, callerNodeID); err != nil {
			slog.Warn("member left but their listing remains", "workspace", ws.ID, "error", err)
		}
	}
	return nil
}
