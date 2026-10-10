// Package realtime is the contract between the services that change state and
// the messaging hub that tells connected browsers about it.
//
// A service that has committed a change emits an Event onto the Redpanda topic
// "<domain>.events". The messaging consumer turns it into a DomainEvent frame
// and hands it to the hub, which fans it out to the right sessions. Events
// carry ids and the kind of change, never content: clients refetch under their
// own authorization.
package realtime

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"ngac-platform/pkg/grpcauth"
)

// Domains. Each owns the topic "<domain>.events".
const (
	DomainDrive      = "drive"
	DomainChannel    = "channel"
	DomainWorkspace  = "workspace"
	DomainDocument   = "document"
	DomainAsset      = "asset"
	DomainPermission = "permission"
)

// Kinds of change. A domain uses the ones that make sense for it; the full
// pairing is listed in docs/specs/realtime-event-delivery/spec.md.
const (
	KindCreated = "created"
	KindUpdated = "updated"
	KindDeleted = "deleted"
	KindMoved   = "moved"
	KindRenamed = "renamed"

	KindShareCreated = "share_created"
	KindShareRevoked = "share_revoked"

	KindMemberAdded        = "member_added"
	KindMemberRemoved      = "member_removed"
	KindRoleChanged        = "role_changed"
	KindDepartmentChanged  = "department_changed"
	KindInvitationAccepted = "invitation_accepted"

	KindRequestChanged = "request_changed"
	KindAssigned       = "assigned"

	KindChanged = "changed"
)

// Event is one committed change.
type Event struct {
	Domain      string `json:"domain"`
	Kind        string `json:"kind"`
	TenantID    string `json:"tenant_id"`
	WorkspaceID string `json:"workspace_id"`
	// IDs are the entities the change touched.
	IDs []string `json:"ids,omitempty"`
	// ParentID is the containing folder after the change, OldParentID the one an
	// item left (moves).
	ParentID    string `json:"parent_id,omitempty"`
	OldParentID string `json:"old_parent_id,omitempty"`
	// ActorUserID is the users.id of whoever caused the change; empty for
	// changes with no human behind them.
	ActorUserID string `json:"actor_user_id,omitempty"`
	// ChannelID addresses the event to one channel's subscribers.
	ChannelID string `json:"channel_id,omitempty"`
	// UserNodeIDs addresses the event to those users' sessions only (NGAC user
	// node ids). It takes precedence over ChannelID.
	UserNodeIDs []string `json:"user_node_ids,omitempty"`
	// At is the emit time in Unix milliseconds; the consumer drops stale events.
	At int64 `json:"at"`
}

// Level is how far an event fans out.
type Level int

const (
	// LevelWorkspace reaches every session subscribed to the workspace.
	LevelWorkspace Level = iota
	// LevelChannel reaches the subscribers of one channel.
	LevelChannel
	// LevelUser reaches the named users' sessions.
	LevelUser
)

// Level reports the fan-out level the event's addressing selects.
func (e Event) Level() Level {
	switch {
	case len(e.UserNodeIDs) > 0:
		return LevelUser
	case e.ChannelID != "":
		return LevelChannel
	default:
		return LevelWorkspace
	}
}

// Validate rejects an event the hub could not route safely. Every event must
// name its tenant and workspace: an event without a tenant would have no
// boundary to stay inside.
func (e Event) Validate() error {
	switch {
	case e.Domain == "":
		return errors.New("realtime: event has no domain")
	case e.Kind == "":
		return errors.New("realtime: event has no kind")
	case e.TenantID == "":
		return errors.New("realtime: event has no tenant_id")
	case e.WorkspaceID == "":
		return errors.New("realtime: event has no workspace_id")
	}
	// Ids are joined into pub/sub channel names with ':' as the separator.
	for _, id := range []string{e.TenantID, e.WorkspaceID, e.ChannelID} {
		if strings.Contains(id, ":") {
			return fmt.Errorf("realtime: id %q contains ':'", id)
		}
	}
	for _, id := range e.UserNodeIDs {
		if strings.Contains(id, ":") {
			return fmt.Errorf("realtime: id %q contains ':'", id)
		}
	}
	return nil
}

// Topic is the Redpanda topic a domain's events travel on.
func Topic(domain string) string { return domain + ".events" }

// Topics lists the topics the hub consumes, one per domain this package defines.
func Topics() []string {
	return []string{
		Topic(DomainDrive), Topic(DomainChannel), Topic(DomainWorkspace),
		Topic(DomainDocument), Topic(DomainPermission),
	}
}

// Emitter is what a service holds to announce committed changes. A nil
// Emitter is valid and drops everything, so wiring it is optional.
type Emitter interface {
	Emit(Event)
}

// For starts an event on behalf of the request in ctx: the tenant and acting
// user come from the verified caller, never from the request body. The
// workspace is the one the changed data belongs to.
func For(ctx context.Context, domain, kind, workspaceID string, ids ...string) Event {
	who := grpcauth.CallerFrom(ctx)
	return Event{
		Domain: domain, Kind: kind,
		TenantID: who.TenantID, WorkspaceID: workspaceID,
		IDs: ids, ActorUserID: who.UserID,
	}
}
