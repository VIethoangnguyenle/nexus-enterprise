package events

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
)

// Topic constants for asset domain events.
const (
	TopicLifecycle  = "asset.lifecycle"
	TopicRequest    = "asset.request"
	TopicAssignment = "asset.assignment"
)

// LifecycleEvent records an asset state transition.
type LifecycleEvent struct {
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

// RequestEvent records asset request lifecycle changes.
type RequestEvent struct {
	RequestID   string `json:"request_id"`
	TypeName    string `json:"type_name"`
	TypeID      string `json:"type_id"`
	RequesterID string `json:"requester_id"`
	Status      string `json:"status"` // "pending", "approved", "rejected", "fulfilled"
	ApproverID  string `json:"approver_id,omitempty"`
	WorkspaceID string `json:"workspace_id"`
	Timestamp   int64  `json:"timestamp"`
}

// AssignmentEvent records asset assignment/return changes.
type AssignmentEvent struct {
	AssetID     string `json:"asset_id"`
	AssetName   string `json:"asset_name"`
	FromUserID  string `json:"from_user_id,omitempty"`
	ToUserID    string `json:"to_user_id,omitempty"`
	Action      string `json:"action"` // "assign", "return"
	ActorID     string `json:"actor_id"`
	WorkspaceID string `json:"workspace_id"`
	Timestamp   int64  `json:"timestamp"`
}

// Producer wraps a Kafka client for publishing asset domain events.
type Producer struct {
	client *kgo.Client
}

// NewProducer creates a Kafka producer connected to the given brokers.
func NewProducer(brokers []string) (*Producer, error) {
	client, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.AllowAutoTopicCreation(),
	)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx); err != nil {
		client.Close()
		return nil, err
	}

	slog.Info("kafka producer connected for asset service")
	return &Producer{client: client}, nil
}

// Close shuts down the Kafka producer.
func (p *Producer) Close() {
	if p != nil && p.client != nil {
		p.client.Close()
	}
}

// PublishLifecycle emits an asset lifecycle state change event.
func (p *Producer) PublishLifecycle(evt LifecycleEvent) {
	if p == nil {
		return
	}
	evt.Timestamp = time.Now().UnixMilli()
	p.publish(TopicLifecycle, evt.AssetID, evt)
}

// PublishRequest emits an asset request event.
func (p *Producer) PublishRequest(evt RequestEvent) {
	if p == nil {
		return
	}
	evt.Timestamp = time.Now().UnixMilli()
	p.publish(TopicRequest, evt.RequestID, evt)
}

// PublishAssignment emits an asset assignment change event.
func (p *Producer) PublishAssignment(evt AssignmentEvent) {
	if p == nil {
		return
	}
	evt.Timestamp = time.Now().UnixMilli()
	p.publish(TopicAssignment, evt.AssetID, evt)
}

func (p *Producer) publish(topic, key string, evt any) {
	data, err := json.Marshal(evt)
	if err != nil {
		slog.Error("failed to marshal event", "topic", topic, "error", err)
		return
	}
	record := &kgo.Record{
		Topic: topic,
		Key:   []byte(key),
		Value: data,
	}
	p.client.Produce(context.Background(), record, func(_ *kgo.Record, err error) {
		if err != nil {
			slog.Warn("kafka publish failed", "topic", topic, "error", err)
		}
	})
}
