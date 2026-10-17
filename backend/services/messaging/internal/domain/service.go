// Package domain contains the business logic for the messaging service.
// It orchestrates between the store (database), NGAC policy (access control),
// and external services (auth, drive). No SQL or protobuf lives here.
package domain

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"ngac-platform/pkg/grpcauth"
	"ngac-platform/pkg/httputil"
	"ngac-platform/pkg/realtime"
	authpb "ngac-platform/proto/auth"
	drivepb "ngac-platform/proto/drive"
	policypb "ngac-platform/proto/policy"
	"ngac-platform/services/messaging/internal/store"
)

// memberCache is the channel_members table, which DM lookup reads. The graph is
// the source of truth for membership; this is its denormalised copy.
type memberCache interface {
	InsertChannelMember(ctx context.Context, channelID, ngacNodeID string) error
}

// Service orchestrates messaging business logic.
type Service struct {
	store       *store.Store
	members     memberCache
	policyRead  policypb.PolicyReadServiceClient
	policyWrite policypb.PolicyWriteServiceClient
	authClient  authpb.AuthServiceClient
	driveClient drivepb.DriveServiceClient
	revoker     SubscriptionRevoker
	emitter     realtime.Emitter
	push        *pushState // nil: web push off

	media        CallMedia    // nil: calls unavailable
	callObserver CallObserver // nil: call changes are not announced

	callHangUpGrace time.Duration // 0: the default wait before a direct call left alone ends

	versions httputil.Versions // nil: no ETags; see chat_state.go

	// hasWideEdge remembers who was seen with their workspace-wide channel edge
	// (key: workspace|node), so listing channels does not ask the policy each time.
	// Only a positive is kept, and leaving the workspace removes the entry.
	hasWideEdge sync.Map

	rdb *redis.Client // nil: avatar uploads and downloads are not rate limited
	// avatarGate holds one slot: one group picture is decoded at a time.
	avatarGate chan struct{}
	avatarWait time.Duration
}

// SetRedis wires the counters behind the avatar rate limits. Without it
// (only unit tests run that way) nothing is counted.
func (s *Service) SetRedis(rdb *redis.Client) { s.rdb = rdb }

// SetEmitter wires the announcer of committed channel changes. A nil emitter
// is valid and announces nothing.
func (s *Service) SetEmitter(e realtime.Emitter) { s.emitter = e }

// emit announces a change that has already committed.
func (s *Service) emit(e realtime.Event) {
	// Whoever reads the chat state next must see this change, so the counters
	// move where the change is announced.
	s.BumpChat(e.TenantID)
	if s.emitter != nil {
		s.emitter.Emit(e)
	}
}

// channelEvent builds a channel-domain event for ch on behalf of the request in
// ctx. A DM has no workspace; its session tenant stands in so the event still
// has a boundary.
func channelEvent(ctx context.Context, kind string, ch *store.Channel) realtime.Event {
	ws := ch.WorkspaceID
	if ws == "" {
		ws = grpcauth.CallerFrom(ctx).TenantID
	}
	return realtime.For(ctx, realtime.DomainChannel, kind, ws, ch.ID)
}

// SubscriptionRevoker ends live (WebSocket) subscriptions a user holds on a
// channel. The hub implements it.
type SubscriptionRevoker interface {
	RevokeChannelSubscriptions(channelID, userNodeID string)
}

// SetSubscriptionRevoker wires the live-subscription revoker. It is a setter
// rather than a constructor argument because the hub itself depends on this
// service to authorize subscriptions.
func (s *Service) SetSubscriptionRevoker(r SubscriptionRevoker) {
	s.revoker = r
}

// NewService creates a messaging domain service.
func NewService(
	st *store.Store,
	pr policypb.PolicyReadServiceClient,
	pw policypb.PolicyWriteServiceClient,
	ac authpb.AuthServiceClient,
	dc drivepb.DriveServiceClient,
) *Service {
	return &Service{
		store:       st,
		members:     st,
		policyRead:  pr,
		policyWrite: pw,
		authClient:  ac,
		driveClient: dc,
		avatarGate:  make(chan struct{}, 1),
		avatarWait:  avatarBusyWait,
	}
}

// --- Channel operations ---

// --- DM operations ---

// --- Message operations ---

// --- Channel member operations ---

// --- Helpers ---

// lookupUsername resolves a user ID to a username via the auth service.
func (s *Service) lookupUsername(ctx context.Context, userID string) string {
	user, err := s.authClient.GetUserByID(ctx, &authpb.GetUserByIDRequest{UserId: userID})
	if err != nil {
		slog.Warn("username lookup failed", "user", userID, "error", err)
	}
	if user != nil {
		return user.Username
	}
	return ""
}

// --- Proto conversions ---
