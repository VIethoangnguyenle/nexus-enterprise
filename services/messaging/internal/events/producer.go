package events

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
)

// MessageSentEvent records each message for analytics and future search indexing.
type MessageSentEvent struct {
	ChannelID   string `json:"channel_id"`
	SenderID    string `json:"sender_id"`
	ContentHash string `json:"content_hash,omitempty"`
	Timestamp   int64  `json:"timestamp"`
}

// Producer wraps a Kafka client for publishing messaging domain events.
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

// PublishMessageSent publishes a message event asynchronously.
func (p *Producer) PublishMessageSent(channelID, senderID string) {
	if p == nil {
		return
	}
	evt := MessageSentEvent{
		ChannelID: channelID,
		SenderID:  senderID,
		Timestamp: time.Now().UnixMilli(),
	}
	data, err := json.Marshal(evt)
	if err != nil {
		return
	}
	record := &kgo.Record{
		Topic: "ngac.messages",
		Value: data,
	}
	p.client.Produce(context.Background(), record, func(_ *kgo.Record, err error) {
		if err != nil {
			slog.Warn("kafka publish failed", "topic", "ngac.messages", "error", err)
		}
	})
}
