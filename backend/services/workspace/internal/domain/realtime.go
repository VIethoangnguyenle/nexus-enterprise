package domain

import (
	"context"

	"ngac-platform/pkg/grpcauth"
	"ngac-platform/pkg/realtime"
)

// WithEmitter sets where committed changes are announced for live clients. A
// service without one announces nothing.
func (s *Service) WithEmitter(e realtime.Emitter) *Service {
	s.emitter = e
	return s
}

// announce tells the workspace's connected clients that something changed. It
// is called only after the change has been written, on the success path, never
// before and never from a path that is undone.
//
// The audience of a workspace event is the workspace's own tenant, whichever
// tenant the acting token was issued for (an administrator may hold the token
// of another workspace they also belong to); the actor is always the verified
// caller. ids are what the change touched: a person is named by their NGAC
// user node id, which is what the admin screens key people by, a role or
// department by its own id.
func (s *Service) announce(ctx context.Context, kind, wsID string, ids ...string) {
	if s.emitter == nil {
		return
	}
	s.emitter.Emit(realtime.Event{
		Domain: realtime.DomainWorkspace, Kind: kind,
		TenantID: wsID, WorkspaceID: wsID,
		IDs: ids, ActorUserID: grpcauth.CallerFrom(ctx).UserID,
	})
}

// announceAccessChanged tells those people's own sessions to drop what they
// cached about what they may do.
func (s *Service) announceAccessChanged(ctx context.Context, wsID string, userNodeIDs ...string) {
	if s.emitter == nil {
		return
	}
	var nodes []string
	for _, id := range userNodeIDs {
		if id != "" {
			nodes = append(nodes, id)
		}
	}
	if len(nodes) == 0 {
		return
	}
	s.emitter.Emit(realtime.Event{
		Domain: realtime.DomainPermission, Kind: realtime.KindChanged,
		TenantID: wsID, WorkspaceID: wsID,
		UserNodeIDs: nodes, ActorUserID: grpcauth.CallerFrom(ctx).UserID,
	})
}
