package realtime

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
)

const (
	deliveryTimeout    = 10 * time.Second
	maxBufferedRecords = 2000
	closeFlushTimeout  = 3 * time.Second
)

// Producer publishes Events to Redpanda. It is fire-and-forget: a broker that
// is down costs the realtime nudge, never the request that caused it, and
// clients resynchronise on their next reconnect or sequence gap.
type Producer struct {
	client *kgo.Client
}

// NewProducer connects to the brokers. The caller decides what a failure
// means; services log it and run without realtime.
func NewProducer(brokers []string) (*Producer, error) {
	client, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.AllowAutoTopicCreation(),
		// A broker that is down must cost events, never request latency or
		// memory: records give up after the timeout, and past the buffer limit
		// TryProduce drops instead of blocking the caller.
		kgo.RecordDeliveryTimeout(deliveryTimeout),
		kgo.MaxBufferedRecords(maxBufferedRecords),
	)
	if err != nil {
		return nil, err
	}
	return &Producer{client: client}, nil
}

// Connect builds the service's producer from KAFKA_BROKERS (default
// localhost:19092). A broker that cannot be reached is not fatal: the service
// runs without realtime and the error is logged. The result may be nil, which
// every Emit tolerates.
func Connect(service string) *Producer {
	brokers := os.Getenv("KAFKA_BROKERS")
	if brokers == "" {
		brokers = "localhost:19092"
	}
	p, err := NewProducer(strings.Split(brokers, ","))
	if err != nil {
		slog.Warn("realtime producer unavailable, live updates disabled", "service", service, "error", err)
		return nil
	}
	slog.Info("realtime producer ready", "service", service, "brokers", brokers)
	return p
}

// Emit publishes the event after validating it. Call it only once the change
// is committed: an event for a state that never became visible tells clients
// to look at something that is not there. Safe on a nil Producer.
func (p *Producer) Emit(e Event) {
	if p == nil || p.client == nil {
		return
	}
	if err := e.Validate(); err != nil {
		slog.Warn("realtime event dropped", "domain", e.Domain, "kind", e.Kind, "error", err)
		return
	}
	if e.At == 0 {
		e.At = time.Now().UnixMilli()
	}
	data, err := json.Marshal(e)
	if err != nil {
		slog.Warn("realtime event marshal failed", "domain", e.Domain, "error", err)
		return
	}
	// Keyed by workspace so one workspace's events stay in order on a partition.
	p.client.TryProduce(context.Background(), &kgo.Record{
		Topic: Topic(e.Domain),
		Key:   []byte(e.WorkspaceID),
		Value: data,
	}, func(_ *kgo.Record, err error) {
		if err != nil {
			// Includes kgo.ErrMaxBuffered: the buffer is full because the broker
			// is not keeping up, and the event is dropped, not queued behind it.
			slog.Warn("realtime event dropped", "topic", Topic(e.Domain), "error", err)
		}
	})
}

// Close gives buffered events a few seconds to reach the broker, then closes
// the client. An unreachable broker does not hold shutdown up.
func (p *Producer) Close() {
	if p != nil && p.client != nil {
		ctx, cancel := context.WithTimeout(context.Background(), closeFlushTimeout)
		defer cancel()
		_ = p.client.Flush(ctx)
		p.client.Close()
	}
}

// Decode parses a topic payload back into an Event and validates it.
func Decode(data []byte) (Event, error) {
	var e Event
	if err := json.Unmarshal(data, &e); err != nil {
		return Event{}, err
	}
	return e, e.Validate()
}
