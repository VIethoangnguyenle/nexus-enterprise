package domain

import (
	"context"
	"log/slog"

	"ngac-platform/pkg/realtime"
)

// WithEmitter sets where committed changes are announced for live clients. A
// service without one announces nothing.
func (s *Service) WithEmitter(e realtime.Emitter) *Service {
	s.emitter = e
	return s
}

// announceProfileChanged tells every workspace the person actively belongs to
// that their name or picture changed, so lists that show them (direct
// messages, the people directory) and the cached chat state are refreshed. Call
// it only after the change is stored.
//
// One event goes to each workspace, naming the person by their node id there;
// the event carries no name or picture, and viewers refetch under their own
// authorization. A workspace the person does not belong to is never told.
func (s *Service) announceProfileChanged(ctx context.Context, userID string) {
	if s.emitter == nil {
		return
	}
	memberships, err := s.store.ListTenantsByUser(ctx, userID)
	if err != nil {
		// The change is stored; the cached chat state heals within its ETag
		// lifetime.
		slog.Warn("profile change not announced", "user", userID, "error", err)
		return
	}
	for _, m := range memberships {
		s.emitter.Emit(realtime.Event{
			Domain: realtime.DomainWorkspace, Kind: realtime.KindProfileUpdated,
			TenantID: m.TenantID, WorkspaceID: m.TenantID,
			IDs: []string{m.NGACNodeID}, ActorUserID: userID,
		})
	}
}
