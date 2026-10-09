package events

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"

	"ngac-platform/services/policy/internal/ngac"
)

// TopicGraphMutated carries every graph mutation the policy writer makes.
const TopicGraphMutated = "ngac.graph.mutated"

// GraphMutationApplier brings local state up to date with a batch of graph
// mutations. *ngac.ReplicaGraphRefresher implements it.
type GraphMutationApplier interface {
	Apply(ctx context.Context, batch []ngac.GraphMutation) error
}

var _ GraphMutationApplier = (*ngac.ReplicaGraphRefresher)(nil)

// GraphMutationConsumer feeds ngac.graph.mutated events to a read replica's
// GraphMutationApplier, so the replica's in-memory graph and shard cache follow
// the writer instead of staying as they were at startup.
type GraphMutationConsumer struct {
	client  *kgo.Client
	applier GraphMutationApplier

	retryInitial time.Duration
	retryMax     time.Duration
}

// NewGraphMutationConsumer creates a consumer that starts reading at since.
//
// since must be no later than the moment the replica began loading its graph,
// so no mutation can fall between the load and the first event consumed.
// Re-applying a mutation the load already saw is harmless (a reload plus an
// invalidation), so a generous lookback is safe.
//
// There is deliberately no consumer group: every replica must see every
// mutation, whereas a group would split them across replicas.
//
// The broker is not required to be reachable now: the client keeps retrying,
// and because consumption starts from a timestamp, nothing published while the
// broker was unreachable is skipped once it comes back.
func NewGraphMutationConsumer(brokers []string, since time.Time, applier GraphMutationApplier) (*GraphMutationConsumer, error) {
	if applier == nil {
		return nil, fmt.Errorf("graph mutation consumer: nil applier")
	}
	client, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.ConsumeTopics(TopicGraphMutated),
		kgo.ConsumeResetOffset(kgo.NewOffset().AfterMilli(since.UnixMilli())),
		kgo.AllowAutoTopicCreation(),
	)
	if err != nil {
		return nil, fmt.Errorf("create kafka client: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(pingCtx); err != nil {
		slog.Error("graph mutation consumer cannot reach kafka yet; this replica serves its startup graph until it can",
			"brokers", brokers, "error", err)
	} else {
		slog.Info("graph mutation consumer connected", "brokers", brokers, "topic", TopicGraphMutated)
	}

	return newGraphMutationConsumer(client, applier), nil
}

func newGraphMutationConsumer(client *kgo.Client, applier GraphMutationApplier) *GraphMutationConsumer {
	return &GraphMutationConsumer{
		client:       client,
		applier:      applier,
		retryInitial: 500 * time.Millisecond,
		retryMax:     30 * time.Second,
	}
}

// Run consumes until ctx is cancelled.
func (c *GraphMutationConsumer) Run(ctx context.Context) {
	for {
		fetches := c.client.PollFetches(ctx)
		if ctx.Err() != nil {
			return
		}
		fetches.EachError(func(topic string, partition int32, err error) {
			slog.Warn("kafka fetch error", "topic", topic, "partition", partition, "error", err)
		})
		var records []*kgo.Record
		fetches.EachRecord(func(r *kgo.Record) { records = append(records, r) })
		c.handle(ctx, records)
	}
}

// Close shuts down the consumer.
func (c *GraphMutationConsumer) Close() {
	if c != nil && c.client != nil {
		c.client.Close()
	}
}

// handle applies one fetched batch, retrying until it succeeds or ctx ends.
//
// A batch is never dropped on failure: dropping it would leave the replica on
// a stale graph with nothing left to correct it. Later events wait behind it.
func (c *GraphMutationConsumer) handle(ctx context.Context, records []*kgo.Record) {
	batch := DecodeGraphMutations(records)
	if len(batch) == 0 {
		return
	}
	backoff := c.retryInitial
	for {
		err := c.applier.Apply(ctx, batch)
		if err == nil {
			return
		}
		slog.Error("applying graph mutations failed; retrying — replica is serving a stale graph until this succeeds",
			"mutations", len(batch), "retry_in", backoff, "error", err)
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		backoff = min(backoff*2, c.retryMax)
	}
}

// DecodeGraphMutations turns records from TopicGraphMutated into mutations.
//
// A record that cannot be decoded still says "the graph changed" — just not
// where. It becomes a full-reload mutation rather than being skipped, which
// would leave whatever it described unapplied.
func DecodeGraphMutations(records []*kgo.Record) []ngac.GraphMutation {
	out := make([]ngac.GraphMutation, 0, len(records))
	for _, r := range records {
		var evt GraphMutatedEvent
		if err := json.Unmarshal(r.Value, &evt); err != nil {
			slog.Warn("undecodable graph mutation event; treating it as a full reload",
				"offset", r.Offset, "partition", r.Partition, "error", err)
			out = append(out, ngac.GraphMutation{Type: ngac.MutationLoadGraph})
			continue
		}
		out = append(out, ngac.GraphMutation{Type: evt.MutationType, NodeIDs: evt.NodeIDs})
	}
	return out
}
