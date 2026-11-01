// Package domain contains the business logic for the messaging service.
// It orchestrates between the store (database), NGAC policy (access control),
// and external services (auth, drive). No SQL or protobuf lives here.
package domain

import (
	"context"
	"fmt"
	"log/slog"
	"slices"

	"ngac-platform/ngac"
	"ngac-platform/pkg/grpcauth"
	"ngac-platform/pkg/policyclient"
	"ngac-platform/services/messaging/internal/store"
)

// checkAccess verifies NGAC access and returns an error if denied.
func (s *Service) checkAccess(ctx context.Context, userNodeID, objectNodeID, operation string) error {
	if ok, _ := policyclient.New(s.policyRead).Check(ctx, userNodeID, objectNodeID, operation); !ok {
		return fmt.Errorf("%w: %s on %s", ErrAccessDenied, operation, objectNodeID)
	}
	return nil
}

// AuthorizeChannelAccess reports whether the user holds operation on the
// channel's content OA. The WebSocket hub delegates to it before letting a
// connection join a channel's live stream. It fails closed: an unknown channel
// or a policy error is a refusal.
func (s *Service) AuthorizeChannelAccess(ctx context.Context, channelID, userNodeID, operation string) error {
	_, err := s.authorizeChannel(ctx, channelID, userNodeID, operation)
	return err
}

// AuthorizeChannelSubscribe authorizes following a channel's live stream, which
// is a read of the channel, and returns the workspace the channel belongs to so
// the subscription can be ended when the user leaves that workspace.
func (s *Service) AuthorizeChannelSubscribe(ctx context.Context, channelID, userNodeID string) (string, error) {
	ch, err := s.authorizeChannel(ctx, channelID, userNodeID, ngac.OpRead)
	if err != nil {
		return "", err
	}
	return ch.WorkspaceID, nil
}

// loadChannel returns the channel, ErrNotFound when there is none, and a plain
// wrapped error when the lookup itself failed.
func (s *Service) loadChannel(ctx context.Context, channelID string) (*store.Channel, error) {
	ch, err := s.store.GetChannel(ctx, channelID)
	if err != nil {
		return nil, fmt.Errorf("load channel: %w", err)
	}
	if ch == nil {
		return nil, fmt.Errorf("%w: channel not found", ErrNotFound)
	}
	return ch, nil
}

// loadChannelFor is loadChannel for a request made by a person: a direct message
// belongs to one workspace and is answered as not found to anyone signed in to
// another (or to none), the way a conversation the caller is not in is. Work the service starts on its own (a push, a call
// hang-up timer) has no signed-in workspace and uses loadChannel.
func (s *Service) loadChannelFor(ctx context.Context, channelID string) (*store.Channel, error) {
	ch, err := s.loadChannel(ctx, channelID)
	if err != nil {
		return nil, err
	}
	if !inCallersWorkspace(ctx, ch) {
		return nil, fmt.Errorf("%w: channel not found", ErrNotFound)
	}
	return ch, nil
}

// inCallersWorkspace reports whether the caller may address the channel from the
// workspace they are signed in to. Only a direct message is held to it: a group
// is tied to its workspace by the graph (its Members UA sits under the
// workspace's policy class), a DM sits under PC_Global and so is tied here.
func inCallersWorkspace(ctx context.Context, ch *store.Channel) bool {
	if ch.ChannelType != ChannelTypeDM {
		return true
	}
	tenant := grpcauth.CallerFrom(ctx).TenantID
	return tenant != "" && ch.WorkspaceID == tenant
}

// authorizeChannel loads a channel and verifies the user holds op on its
// content OA, returning the channel so callers do not fetch it twice.
func (s *Service) authorizeChannel(ctx context.Context, channelID, userNodeID, operation string) (*store.Channel, error) {
	if channelID == "" {
		return nil, ErrInvalidInput
	}
	ch, err := s.loadChannelFor(ctx, channelID)
	if err != nil {
		return nil, err
	}
	if err := s.authorizeChannelOp(ctx, ch, userNodeID, operation); err != nil {
		return nil, err
	}
	return ch, nil
}

// authorizeBackOut authorizes leaving a conversation on one's own terms —
// declining or cancelling a call, deleting one's own message, taking back a
// reaction or a vote. It asks the graph alone: someone who no longer shares a
// workspace with the other person of a direct message may still withdraw what
// they put there.
func (s *Service) authorizeBackOut(ctx context.Context, channelID, userNodeID, operation string) (*store.Channel, error) {
	if channelID == "" {
		return nil, ErrInvalidInput
	}
	ch, err := s.loadChannelFor(ctx, channelID)
	if err != nil {
		return nil, err
	}
	if err := s.checkAccess(ctx, userNodeID, ch.NGACOaID, operation); err != nil {
		return nil, err
	}
	return ch, nil
}

// authorizeChannelOp is the one place a write, invite or manage on a loaded
// channel is decided. Beyond holding the operation in the graph, a direct message
// needs its two people to be in the DM's workspace: a DM hangs under PC_Global, so
// its grants outlive any workspace and would otherwise keep a removed colleague
// able to write to, call and invite people into a conversation with the
// workspace's members. Reads stay graph-only, so the history of such a DM remains
// readable.
//
// Every check of an operation on a channel's OA goes through here (or through
// authorizeChannel / authorizeMessage, which call it); a direct checkAccess on a
// channel OA would skip the relationship rule.
func (s *Service) authorizeChannelOp(ctx context.Context, ch *store.Channel, userNodeID, operation string) error {
	if err := s.checkAccess(ctx, userNodeID, ch.NGACOaID, operation); err != nil {
		return err
	}
	if ch.ChannelType == ChannelTypeDM && dmRelationshipOps[operation] {
		return s.requireDMRelationship(ctx, ch, userNodeID)
	}
	return nil
}

// dmRelationshipOps are the operations on a DM that need the two people to be in
// its workspace.
var dmRelationshipOps = map[string]bool{ngac.OpWrite: true, ngac.OpInvite: true, ngac.OpManage: true}

// requireDMRelationship refuses unless the user and the DM's other member are
// both in the DM's own workspace. It fails closed: a DM with no other member
// recorded, or a policy service that cannot answer, is a refusal.
func (s *Service) requireDMRelationship(ctx context.Context, ch *store.Channel, userNodeID string) error {
	denied := fmt.Errorf("%w: a direct message needs both people in its workspace", ErrAccessDenied)
	if ch.WorkspaceID == "" {
		return denied
	}
	peers, err := s.store.DMPeerNodes(ctx, ch.ID, userNodeID)
	if err != nil {
		slog.Error("direct message peers unknown; refusing", "channel", ch.ID, "error", err)
		return denied
	}
	for _, peer := range peers {
		shared, err := policyclient.SharedWorkspaces(ctx, s.policyRead, userNodeID, peer)
		if err != nil {
			slog.Error("direct message relationship unknown; refusing", "channel", ch.ID, "error", err)
			return denied
		}
		if slices.Contains(shared, ngac.WorkspaceID(ch.WorkspaceID)) {
			return nil
		}
	}
	return denied
}

// authorizeMessage authorizes an operation on the channel that owns a message.
//
// Messages are not nodes in the NGAC graph — they live in Postgres with a
// foreign key to the channel whose content OA *is* in the graph. So the check
// belongs on that OA, never on the message ID the caller supplied.
func (s *Service) authorizeMessage(ctx context.Context, messageID, userNodeID, operation string) error {
	if messageID == "" {
		return ErrInvalidInput
	}
	channelID, err := s.store.GetChannelIDForMessage(ctx, messageID)
	if err != nil {
		return fmt.Errorf("load message: %w", err)
	}
	if channelID == "" {
		return fmt.Errorf("%w: message not found", ErrNotFound)
	}
	_, err = s.authorizeChannel(ctx, channelID, userNodeID, operation)
	return err
}

// AuthorizeWorkspaceAccess reports whether the user holds operation on the
// workspace's Documents OA — the workspace-level attribute that stands for
// "being in the workspace". The WebSocket hub delegates to it before a session
// may follow the workspace's live changes. The workspace must be the session's
// own tenant: a token scoped to one tenant never follows another. It fails
// closed: an unknown workspace, a mismatch or a policy error is a refusal.
func (s *Service) AuthorizeWorkspaceAccess(ctx context.Context, workspaceID, userNodeID, tenantID, operation string) error {
	if workspaceID == "" || userNodeID == "" || tenantID == "" {
		return ErrInvalidInput
	}
	if workspaceID != tenantID {
		return fmt.Errorf("%w: workspace is outside the session's tenant", ErrAccessDenied)
	}
	oaID, err := s.store.WorkspaceDocumentsOA(ctx, workspaceID)
	if err != nil || oaID == "" {
		return fmt.Errorf("%w: workspace not found", ErrAccessDenied)
	}
	return s.checkAccess(ctx, userNodeID, oaID, operation)
}
