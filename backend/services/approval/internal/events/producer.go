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
	Comment         string   `json:"comment,omitempty"`
	Timestamp       int64    `json:"timestamp"`
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

	p.client.Produce(ctx, &kgo.Record{
		Topic: approvalEventsTopic,
		Key:   []byte(evt.RequestID),
		Value: data,
	}, func(_ *kgo.Record, err error) {
		if err != nil {
			slog.Warn("publish approval event failed", "request_id", evt.RequestID, "error", err)
		}
	})
}

// Close shuts down the producer.
func (p *Producer) Close() {
	if p != nil && p.client != nil {
		p.client.Close()
	}
}
