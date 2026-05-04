package events

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
)

// AssetLifecycleEvent matches the event structure from asset service.
type AssetLifecycleEvent struct {
	AssetID     string `json:"asset_id"`
	AssetName   string `json:"asset_name"`
	TypeName    string `json:"type_name"`
	FromState   string `json:"from_state"`
	ToState     string `json:"to_state"`
	Action      string `json:"action"`
	ActorID     string `json:"actor_id"`
	WorkspaceID string `json:"workspace_id"`
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
	Timestamp   int64  `json:"timestamp"`
}

// NotificationCreator defines the interface for creating notifications from events.
type NotificationCreator interface {
	CreateNotification(ctx context.Context, userID, notifType, title, body, entityType, entityID string) error
}

// ApprovalBroadcaster defines the interface for broadcasting approval events via WebSocket.
type ApprovalBroadcaster interface {
	BroadcastApprovalEvent(requestID, status, action, actorNodeID, templateName string)
}

// ApprovalEvent matches the event structure from approval service.
type ApprovalEvent struct {
	RequestID    string `json:"request_id"`
	TemplateName string `json:"template_name"`
	EntityType   string `json:"entity_type"`
	Status       string `json:"status"`
	Action       string `json:"action"`
	ActorNodeID  string `json:"actor_node_id"`
	CreatedBy    string `json:"created_by"`
	ScopeOaID    string `json:"scope_oa_id"`
	Comment      string `json:"comment"`
	Timestamp    int64  `json:"timestamp"`
}

// Consumer listens to Kafka topics and creates notifications from asset and approval events.
type Consumer struct {
	client     *kgo.Client
	notifSv    NotificationCreator
	broadcast  ApprovalBroadcaster
	cancel     context.CancelFunc
}

// NewConsumer creates a Kafka consumer subscribing to asset and approval event topics.
func NewConsumer(brokers []string, notifSv NotificationCreator, broadcast ApprovalBroadcaster) (*Consumer, error) {
	client, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.ConsumerGroup("messaging-notification-consumer"),
		kgo.ConsumeTopics("asset.lifecycle", "asset.request", "asset.assignment", "approval.events"),
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

		fetches.EachRecord(func(record *kgo.Record) {
			switch record.Topic {
			case "asset.lifecycle":
				c.handleLifecycleEvent(ctx, record.Value)
			case "asset.request":
				c.handleRequestEvent(ctx, record.Value)
			case "asset.assignment":
				c.handleAssignmentEvent(ctx, record.Value)
			case "approval.events":
				c.handleApprovalEvent(ctx, record.Value)
			}
		})
	}
}

func (c *Consumer) handleLifecycleEvent(ctx context.Context, data []byte) {
	var evt AssetLifecycleEvent
	if err := json.Unmarshal(data, &evt); err != nil {
		slog.Warn("failed to unmarshal lifecycle event", "error", err)
		return
	}

	// Notify the actor about the transition (confirmation)
	title := fmt.Sprintf("Asset %s: %s", evt.AssetName, evt.Action)
	body := fmt.Sprintf("%s transitioned from %s to %s", evt.AssetName, evt.FromState, evt.ToState)

	if err := c.notifSv.CreateNotification(ctx,
		evt.ActorID, "asset_lifecycle", title, body, "asset", evt.AssetID,
	); err != nil {
		slog.Warn("failed to create lifecycle notification", "error", err)
	}
}

func (c *Consumer) handleRequestEvent(ctx context.Context, data []byte) {
	var evt AssetRequestEvent
	if err := json.Unmarshal(data, &evt); err != nil {
		slog.Warn("failed to unmarshal request event", "error", err)
		return
	}

	switch evt.Status {
	case "pending":
		// Notify workspace admins — for now notify the requester as confirmation
		title := fmt.Sprintf("Asset request submitted: %s", evt.TypeName)
		body := fmt.Sprintf("Your request for %s is pending approval", evt.TypeName)
		c.notifSv.CreateNotification(ctx,
			evt.RequesterID, "asset_request", title, body, "asset_request", evt.RequestID,
		)
	case "approved":
		title := fmt.Sprintf("Asset request approved: %s", evt.TypeName)
		body := fmt.Sprintf("Your request for %s has been approved", evt.TypeName)
		c.notifSv.CreateNotification(ctx,
			evt.RequesterID, "asset_request_approved", title, body, "asset_request", evt.RequestID,
		)
	case "rejected":
		title := fmt.Sprintf("Asset request rejected: %s", evt.TypeName)
		body := fmt.Sprintf("Your request for %s has been rejected", evt.TypeName)
		c.notifSv.CreateNotification(ctx,
			evt.RequesterID, "asset_request_rejected", title, body, "asset_request", evt.RequestID,
		)
	}
}

func (c *Consumer) handleAssignmentEvent(ctx context.Context, data []byte) {
	var evt AssetAssignmentEvent
	if err := json.Unmarshal(data, &evt); err != nil {
		slog.Warn("failed to unmarshal assignment event", "error", err)
		return
	}

	switch evt.Action {
	case "assign":
		if evt.ToUserID != "" {
			title := fmt.Sprintf("Asset assigned: %s", evt.AssetName)
			body := fmt.Sprintf("You have been assigned %s", evt.AssetName)
			c.notifSv.CreateNotification(ctx,
				evt.ToUserID, "asset_assigned", title, body, "asset", evt.AssetID,
			)
		}
	case "return":
		if evt.FromUserID != "" {
			title := fmt.Sprintf("Asset returned: %s", evt.AssetName)
			body := fmt.Sprintf("%s has been returned", evt.AssetName)
			c.notifSv.CreateNotification(ctx,
				evt.FromUserID, "asset_returned", title, body, "asset", evt.AssetID,
			)
		}
	}
}

func (c *Consumer) handleApprovalEvent(ctx context.Context, data []byte) {
	var evt ApprovalEvent
	if err := json.Unmarshal(data, &evt); err != nil {
		slog.Warn("failed to unmarshal approval event", "error", err)
		return
	}

	slog.Info("approval event received",
		"request_id", evt.RequestID, "action", evt.Action, "actor", evt.ActorNodeID)

	// 1. Create notification for the request creator (if someone else acted)
	switch evt.Action {
	case "approved":
		if evt.CreatedBy != "" && evt.CreatedBy != evt.ActorNodeID {
			title := fmt.Sprintf("Approval request: %s", evt.TemplateName)
			body := "Your approval request has been approved"
			c.notifSv.CreateNotification(ctx,
				evt.CreatedBy, "approval_approved", title, body, "approval", evt.RequestID,
			)
		}
	case "rejected":
		if evt.CreatedBy != "" && evt.CreatedBy != evt.ActorNodeID {
			title := fmt.Sprintf("Approval request: %s", evt.TemplateName)
			body := "Your approval request has been rejected"
			if evt.Comment != "" {
				body += ": " + evt.Comment
			}
			c.notifSv.CreateNotification(ctx,
				evt.CreatedBy, "approval_rejected", title, body, "approval", evt.RequestID,
			)
		}
	}

	// 2. Broadcast WS event for real-time cache invalidation on all clients
	if c.broadcast != nil {
		c.broadcast.BroadcastApprovalEvent(
			evt.RequestID, evt.Status, evt.Action, evt.ActorNodeID, evt.TemplateName,
		)
	}
}
