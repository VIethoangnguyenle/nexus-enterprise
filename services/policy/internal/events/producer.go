package events

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
)

// AccessCheckedEvent records each access decision for audit trail.
type AccessCheckedEvent struct {
	UserID    string `json:"user_id"`
	ObjectID  string `json:"object_id"`
	Operation string `json:"operation"`
	Decision  string `json:"decision"`
	Cached    bool   `json:"cached"`
	Timestamp int64  `json:"timestamp"`
}

// GraphMutatedEvent records graph structure changes for audit and replay.
type GraphMutatedEvent struct {
	MutationType string   `json:"mutation_type"`
	NodeIDs      []string `json:"node_ids"`
	OperatorID   string   `json:"operator_id,omitempty"`
	Timestamp    int64    `json:"timestamp"`
}

// Producer wraps a Kafka client for publishing NGAC domain events.
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

	// Verify connectivity
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx); err != nil {
		client.Close()
		return nil, err
	}

	slog.Info("kafka producer connected", "brokers", brokers)
	return &Producer{client: client}, nil
}

// Close shuts down the Kafka producer.
func (p *Producer) Close() {
	if p != nil && p.client != nil {
		p.client.Close()
	}
}

// PublishAccessChecked publishes an access decision event asynchronously.
func (p *Producer) PublishAccessChecked(userID, objectID, operation, decision string, cached bool) {
	if p == nil {
		return
	}
	evt := AccessCheckedEvent{
		UserID:    userID,
		ObjectID:  objectID,
		Operation: operation,
		Decision:  decision,
		Cached:    cached,
		Timestamp: time.Now().UnixMilli(),
	}
	p.publishAsync("ngac.access.checked", evt)
}

// PublishGraphMutated publishes a graph mutation event asynchronously.
func (p *Producer) PublishGraphMutated(mutationType string, nodeIDs []string) {
	if p == nil {
		return
	}
	evt := GraphMutatedEvent{
		MutationType: mutationType,
		NodeIDs:      nodeIDs,
		Timestamp:    time.Now().UnixMilli(),
	}
	p.publishAsync("ngac.graph.mutated", evt)
}

// publishAsync sends an event to a Kafka topic without blocking.
func (p *Producer) publishAsync(topic string, event any) {
	data, err := json.Marshal(event)
	if err != nil {
		slog.Warn("failed to marshal kafka event", "topic", topic, "error", err)
		return
	}
	record := &kgo.Record{
		Topic: topic,
		Value: data,
	}
	p.client.Produce(context.Background(), record, func(_ *kgo.Record, err error) {
		if err != nil {
			slog.Warn("kafka publish failed", "topic", topic, "error", err)
		}
	})
}
