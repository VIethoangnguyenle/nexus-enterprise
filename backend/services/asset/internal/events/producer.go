package events

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"

	"ngac-platform/pkg/grpcauth"
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
	// TenantID is the tenant of the request that caused the event, taken from
	// its verified caller. Consumers drop events that do not carry one.
	TenantID  string `json:"tenant_id"`
	Timestamp int64  `json:"timestamp"`
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
	// TenantID is the tenant of the request that caused the event, taken from
	// its verified caller. Consumers drop events that do not carry one.
	TenantID  string `json:"tenant_id"`
	Timestamp int64  `json:"timestamp"`
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
	// TenantID is the tenant of the request that caused the event, taken from
	// its verified caller. Consumers drop events that do not carry one.
	TenantID  string `json:"tenant_id"`
	Timestamp int64  `json:"timestamp"`
}

// Publisher is what the servers announce changes through. *Producer is the
// Kafka-backed implementation; a nil *Producer discards everything.
type Publisher interface {
	PublishLifecycle(ctx context.Context, evt LifecycleEvent)
	PublishRequest(ctx context.Context, evt RequestEvent)
	PublishAssignment(ctx context.Context, evt AssignmentEvent)
}

var _ Publisher = (*Producer)(nil)

// Producer wraps a Kafka client for publishing asset domain events.
type Producer struct {
	client *kgo.Client
	// send hands a serialized event to the broker. Tests replace it.
	send func(topic, key string, data []byte)
}

// NewProducer creates a Kafka producer connected to the given brokers.
func NewProducer(brokers []string) (*Producer, error) {
	client, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.AllowAutoTopicCreation(),
		// A broker that is down costs events, never request latency.
		kgo.RecordDeliveryTimeout(10*time.Second),
		kgo.MaxBufferedRecords(2000),
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
	p := &Producer{client: client}
	p.send = p.produce
	return p, nil
}

// Close shuts down the Kafka producer.
func (p *Producer) Close() {
	if p != nil && p.client != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = p.client.Flush(ctx)
		p.client.Close()
	}
}

// PublishLifecycle emits an asset lifecycle state change event.
func (p *Producer) PublishLifecycle(ctx context.Context, evt LifecycleEvent) {
	if p == nil {
		return
	}
	evt.TenantID = grpcauth.CallerFrom(ctx).TenantID
	evt.Timestamp = time.Now().UnixMilli()
	p.publish(TopicLifecycle, evt.WorkspaceID, evt)
}

// PublishRequest emits an asset request event.
func (p *Producer) PublishRequest(ctx context.Context, evt RequestEvent) {
	if p == nil {
		return
	}
	evt.TenantID = grpcauth.CallerFrom(ctx).TenantID
	evt.Timestamp = time.Now().UnixMilli()
	p.publish(TopicRequest, evt.WorkspaceID, evt)
}

// PublishAssignment emits an asset assignment change event.
func (p *Producer) PublishAssignment(ctx context.Context, evt AssignmentEvent) {
	if p == nil {
		return
	}
	evt.TenantID = grpcauth.CallerFrom(ctx).TenantID
	evt.Timestamp = time.Now().UnixMilli()
	p.publish(TopicAssignment, evt.WorkspaceID, evt)
}

// publish keys the record by workspace, so one workspace's events share a
// partition and reach the hub in the order they happened.
func (p *Producer) publish(topic, key string, evt any) {
	data, err := json.Marshal(evt)
	if err != nil {
		slog.Error("failed to marshal event", "topic", topic, "error", err)
		return
	}
	p.send(topic, key, data)
}

func (p *Producer) produce(topic, key string, data []byte) {
	record := &kgo.Record{
		Topic: topic,
		Key:   []byte(key),
		Value: data,
	}
	p.client.TryProduce(context.Background(), record, func(_ *kgo.Record, err error) {
		if err != nil {
			slog.Warn("kafka publish failed", "topic", topic, "error", err)
		}
	})
}
