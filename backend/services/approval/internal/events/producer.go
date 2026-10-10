// Package events provides Kafka producer for approval lifecycle events.
// Publishes events to "approval.events" topic when approval requests
// are created, approved, or rejected.
package events

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
)

const approvalEventsTopic = "approval.events"

// publishTimeout bounds how long one event may wait for the broker.
const publishTimeout = 10 * time.Second

// ApprovalEventPayload is the JSON schema published to Kafka.
type ApprovalEventPayload struct {
	RequestID       string   `json:"request_id"`
	TemplateName    string   `json:"template_name"`
	EntityType      string   `json:"entity_type"`
	Status          string   `json:"status"`
	Action          string   `json:"action"`
	ActorNodeID     string   `json:"actor_node_id"`
	CreatedBy       string   `json:"created_by"`
	AssigneeNodeIDs []string `json:"assignee_node_ids"`
	ScopeOaID       string   `json:"scope_oa_id"`
	// TenantID is the tenant the acting request was made in (from its JWT).
	// Consumers use it to keep the event inside that tenant.
	TenantID string `json:"tenant_id,omitempty"`
	// WorkspaceID is the workspace the request belongs to. Approval data lives
	// in a per-tenant schema and a tenant is a workspace, so it equals TenantID.
	WorkspaceID string `json:"workspace_id,omitempty"`
	Comment     string `json:"comment,omitempty"`
	Timestamp   int64  `json:"timestamp"`
}

// Producer publishes approval lifecycle events to Kafka.
type Producer struct {
	client *kgo.Client
}

// NewProducer creates a Kafka producer connected to the given brokers.
// Returns nil if connection fails (graceful degradation).
func NewProducer(brokers []string) (*Producer, error) {
	client, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.AllowAutoTopicCreation(),
		// A broker that is down costs events, never request latency.
		kgo.RecordDeliveryTimeout(publishTimeout),
		kgo.MaxBufferedRecords(2000),
	)
	if err != nil {
		return nil, err
	}
	slog.Info("approval event producer connected", "brokers", brokers, "topic", approvalEventsTopic)
	return &Producer{client: client}, nil
}

// Publish sends an approval event to Kafka asynchronously (fire-and-forget).
// If the producer is nil or Kafka is unavailable, the error is logged but
// the caller's operation is not affected.
func (p *Producer) Publish(ctx context.Context, evt ApprovalEventPayload) {
	if p == nil || p.client == nil {
		return
	}
	if evt.Timestamp == 0 {
		evt.Timestamp = time.Now().Unix()
	}

	data, err := json.Marshal(evt)
	if err != nil {
		slog.Warn("marshal approval event", "error", err)
		return
	}

	// The request that caused the event ends as soon as it answers, so the
	// produce must not hang on its context; it gets its own bounded one, which
	// the promise releases.
	pctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), publishTimeout)
	p.client.TryProduce(pctx, &kgo.Record{
		Topic: approvalEventsTopic,
		Key:   []byte(evt.RequestID),
		Value: data,
	}, func(_ *kgo.Record, err error) {
		cancel()
		if err != nil {
			slog.Warn("publish approval event failed", "request_id", evt.RequestID, "error", err)
		}
	})
}

// Close shuts down the producer.
func (p *Producer) Close() {
	if p != nil && p.client != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = p.client.Flush(ctx)
		p.client.Close()
	}
}
