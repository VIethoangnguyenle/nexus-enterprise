package events

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"

	"ngac-platform/pkg/realtime"
	"ngac-platform/services/messaging/internal/domain"
)

// AssetLifecycleEvent matches the event structure from asset service. It feeds
// the live workspace updates only: a state change tells nobody a notification.
type AssetLifecycleEvent struct {
	AssetID     string `json:"asset_id"`
	AssetName   string `json:"asset_name"`
	TypeName    string `json:"type_name"`
	FromState   string `json:"from_state"`
	ToState     string `json:"to_state"`
	Action      string `json:"action"`
	ActorID     string `json:"actor_id"`
	WorkspaceID string `json:"workspace_id"`
	TenantID    string `json:"tenant_id"`
	Timestamp   int64  `json:"timestamp"`
}

// AssetRequestEvent matches the event structure from asset service.
type AssetRequestEvent struct {
	RequestID   string `json:"request_id"`
	TypeName    string `json:"type_name"`
	TypeID      string `json:"type_id"`
	RequesterID string `json:"requester_id"`
	Status      string `json:"status"`
	ApproverID  string `json:"approver_id,omitempty"`
	WorkspaceID string `json:"workspace_id"`
	TenantID    string `json:"tenant_id"`
	Timestamp   int64  `json:"timestamp"`
}

// AssetAssignmentEvent matches the event structure from asset service.
type AssetAssignmentEvent struct {
	AssetID     string `json:"asset_id"`
	AssetName   string `json:"asset_name"`
	FromUserID  string `json:"from_user_id,omitempty"`
	ToUserID    string `json:"to_user_id,omitempty"`
	Action      string `json:"action"`
	ActorID     string `json:"actor_id"`
	WorkspaceID string `json:"workspace_id"`
	TenantID    string `json:"tenant_id"`
	Timestamp   int64  `json:"timestamp"`
}

// NotificationCreator defines the interface for creating notifications from events.
// *domain.NotificationService implements it.
type NotificationCreator interface {
	CreateNotification(ctx context.Context, n domain.NewNotification) error
	NotifyInvitation(ctx context.Context, invitationID string) error
}

// Notification types an event can raise. The screen words each from the fields
// the notification stores.
const (
	typeApprovalApproved     = "approval_approved"      // the request is complete
	typeApprovalStepApproved = "approval_step_approved" // a step passed, the request goes on
	typeApprovalRejected     = "approval_rejected"
	typeAssetRequestApproved = "asset_request_approved"
	typeAssetRequestRejected = "asset_request_rejected"
	typeAssetAssigned        = "asset_assigned"
	typeAssetReturned        = "asset_returned"
)

// Target types: what a notification is about.
const (
	targetApproval     = "approval"
	targetAssetRequest = "asset_request"
	targetAsset        = "asset"
)

// maxInvitationAge is how old an invitation event may be and still tell its
// invitee: a consumer that starts from the beginning of the topic must not
// announce invitations from last month.
const maxInvitationAge = time.Hour

// ApprovalNotice is an approval status change addressed to specific people.
//
// RecipientNodeIDs are NGAC user node IDs. The notice is delivered to the
// WebSocket sessions of those users only; an empty list reaches nobody. When
// TenantID is set it narrows delivery further to sessions in that tenant.
type ApprovalNotice struct {
	RequestID        string
	Status           string
	Action           string
	ActorNodeID      string
	TemplateName     string
	TenantID         string
	WorkspaceID      string
	RecipientNodeIDs []string
}

// ApprovalBroadcaster defines the interface for broadcasting approval events via WebSocket.
type ApprovalBroadcaster interface {
	BroadcastApprovalEvent(n ApprovalNotice)
}

// ApprovalEvent matches the event structure from approval service.
//
// TenantID and WorkspaceID come from the acting request's token; delivery is
// narrowed to that tenant and the frontend scopes its refetch by the workspace.
type ApprovalEvent struct {
	RequestID       string   `json:"request_id"`
	TemplateName    string   `json:"template_name"`
	EntityType      string   `json:"entity_type"`
	Status          string   `json:"status"`
	Action          string   `json:"action"`
	ActorNodeID     string   `json:"actor_node_id"`
	CreatedBy       string   `json:"created_by"`
	AssigneeNodeIDs []string `json:"assignee_node_ids"`
	ScopeOaID       string   `json:"scope_oa_id"`
	TenantID        string   `json:"tenant_id"`
	WorkspaceID     string   `json:"workspace_id"`
	Comment         string   `json:"comment"`
	Timestamp       int64    `json:"timestamp"`
}

// Consumer listens to Kafka topics and creates notifications from asset and approval events.
type Consumer struct {
	client    *kgo.Client
	notifSv   NotificationCreator
	broadcast ApprovalBroadcaster
	cancel    context.CancelFunc
}

// NewConsumer creates a Kafka consumer subscribing to asset and approval event topics.
func NewConsumer(brokers []string, notifSv NotificationCreator, broadcast ApprovalBroadcaster) (*Consumer, error) {
	client, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.ConsumerGroup("messaging-notification-consumer"),
		kgo.ConsumeTopics("asset.request", "asset.assignment", "approval.events", realtime.Topic(realtime.DomainWorkspace)),
		kgo.AllowAutoTopicCreation(),
	)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())
	c := &Consumer{
		client:    client,
		notifSv:   notifSv,
		broadcast: broadcast,
		cancel:    cancel,
	}
	go c.run(ctx)
	return c, nil
}

// Close stops the consumer.
func (c *Consumer) Close() {
	if c != nil {
		c.cancel()
		c.client.Close()
	}
}

// handleRecord routes one record. A topic that raises no notification (asset
// lifecycle, which only the live updates use) is ignored.
func (c *Consumer) handleRecord(ctx context.Context, topic string, value []byte) {
	switch topic {
	case "asset.request":
		c.handleRequestEvent(ctx, value)
	case "asset.assignment":
		c.handleAssignmentEvent(ctx, value)
	case "approval.events":
		c.handleApprovalEvent(ctx, value)
	case realtime.Topic(realtime.DomainWorkspace):
		c.handleWorkspaceEvent(ctx, value)
	}
}

func (c *Consumer) run(ctx context.Context) {
	slog.Info("asset event consumer started")
	for {
		fetches := c.client.PollRecords(ctx, 100)
		if ctx.Err() != nil {
			slog.Info("asset event consumer shutting down")
			return
		}
		if errs := fetches.Errors(); len(errs) > 0 {
			for _, e := range errs {
				slog.Warn("kafka consumer error", "topic", e.Topic, "error", e.Err)
			}
			time.Sleep(time.Second)
			continue
		}

		fetches.EachRecord(func(record *kgo.Record) { c.handleRecord(ctx, record.Topic, record.Value) })
	}
}

// handleWorkspaceEvent turns a created invitation into a notification for the
// account it reaches. Every other workspace event is none of this consumer's
// business (the hub carries those).
func (c *Consumer) handleWorkspaceEvent(ctx context.Context, data []byte) {
	var evt realtime.Event
	if err := json.Unmarshal(data, &evt); err != nil {
		slog.Warn("failed to unmarshal workspace event", "error", err)
		return
	}
	if evt.Domain != realtime.DomainWorkspace || evt.Kind != realtime.KindInvitationCreated {
		return
	}
	if evt.At > 0 && time.Since(time.UnixMilli(evt.At)) > maxInvitationAge {
		return
	}
	for _, id := range evt.IDs {
		if err := c.notifSv.NotifyInvitation(ctx, id); err != nil {
			slog.Error("invitation notification not recorded", "invitation", id, "error", err)
		}
	}
}

func (c *Consumer) handleRequestEvent(ctx context.Context, data []byte) {
	var evt AssetRequestEvent
	if err := json.Unmarshal(data, &evt); err != nil {
		slog.Warn("failed to unmarshal request event", "error", err)
		return
	}
	// "pending" tells nobody: the requester knows what they just asked for.
	var kind string
	switch evt.Status {
	case "approved":
		kind = typeAssetRequestApproved
	case "rejected":
		kind = typeAssetRequestRejected
	default:
		return
	}
	c.notify(ctx, domain.NewNotification{
		WorkspaceID: evt.TenantID, Type: kind, Recipient: evt.RequesterID, Actor: evt.ApproverID,
		TargetType: targetAssetRequest, TargetID: evt.RequestID, TargetName: evt.TypeName,
	})
}

func (c *Consumer) handleAssignmentEvent(ctx context.Context, data []byte) {
	var evt AssetAssignmentEvent
	if err := json.Unmarshal(data, &evt); err != nil {
		slog.Warn("failed to unmarshal assignment event", "error", err)
		return
	}

	n := domain.NewNotification{
		WorkspaceID: evt.TenantID, Actor: evt.ActorID,
		TargetType: targetAsset, TargetID: evt.AssetID, TargetName: evt.AssetName,
	}
	switch evt.Action {
	case "assign":
		n.Type, n.Recipient = typeAssetAssigned, evt.ToUserID
	case "return":
		n.Type, n.Recipient = typeAssetReturned, evt.FromUserID
	default:
		return
	}
	if n.Recipient == "" {
		return
	}
	c.notify(ctx, n)
}

func (c *Consumer) handleApprovalEvent(ctx context.Context, data []byte) {
	var evt ApprovalEvent
	if err := json.Unmarshal(data, &evt); err != nil {
		slog.Warn("failed to unmarshal approval event", "error", err)
		return
	}

	slog.Info("approval event received",
		"request_id", evt.RequestID, "action", evt.Action, "actor", evt.ActorNodeID)

	// 1. Tell the requester what someone else decided. created_by and the actor
	// are NGAC user nodes; the notification service resolves them to people of
	// the event's workspace, and never tells a person about their own decision.
	n := domain.NewNotification{
		WorkspaceID: evt.TenantID, Recipient: evt.CreatedBy, Actor: evt.ActorNodeID,
		TargetType: targetApproval, TargetID: evt.RequestID, TargetName: evt.TemplateName,
	}
	switch evt.Action {
	case "approved":
		// Only a request that is no longer pending is finished; otherwise one
		// step passed and others remain.
		n.Type = typeApprovalStepApproved
		if evt.Status == "approved" {
			n.Type = typeApprovalApproved
		}
		c.notify(ctx, n)
	case "rejected":
		n.Type = typeApprovalRejected
		n.Params = map[string]string{"reason": evt.Comment}
		c.notify(ctx, n)
	}

	// 2. Push a WS event for real-time cache invalidation — to the people the
	// event names, not to every connected session. The event carries no tenant
	// today, so the participants it names are the only safe audience.
	if c.broadcast == nil {
		return
	}
	recipients := approvalRecipients(evt)
	if len(recipients) == 0 {
		return
	}
	c.broadcast.BroadcastApprovalEvent(ApprovalNotice{
		RequestID: evt.RequestID, Status: evt.Status, Action: evt.Action,
		ActorNodeID: evt.ActorNodeID, TemplateName: evt.TemplateName,
		TenantID: evt.TenantID, WorkspaceID: evt.WorkspaceID, RecipientNodeIDs: recipients,
	})
}

// approvalRecipients lists, without duplicates or blanks, the user nodes an
// approval event names: the actor, the requester and any assignees.
func approvalRecipients(evt ApprovalEvent) []string {
	seen := make(map[string]bool, 2+len(evt.AssigneeNodeIDs))
	var out []string
	for _, id := range append([]string{evt.ActorNodeID, evt.CreatedBy}, evt.AssigneeNodeIDs...) {
		if id != "" && !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}

// notify records a notification. A notification that cannot be stored is logged
// and does not stop the rest of the event from being handled. An event without a
// tenant has no workspace to belong to and is dropped.
func (c *Consumer) notify(ctx context.Context, n domain.NewNotification) {
	if n.WorkspaceID == "" || n.Recipient == "" {
		return
	}
	err := c.notifSv.CreateNotification(ctx, n)
	switch {
	case err == nil:
	case errors.Is(err, domain.ErrNotAMember):
		slog.Warn("notification skipped: recipient is not in the workspace", "type", n.Type, "target", n.TargetID)
	default:
		slog.Error("notification not recorded", "type", n.Type, "target", n.TargetID, "error", err)
	}
}
