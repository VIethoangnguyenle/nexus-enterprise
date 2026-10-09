package events

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kgo"

	"ngac-platform/services/policy/internal/ngac"
)

func record(t *testing.T, evt GraphMutatedEvent) *kgo.Record {
	t.Helper()
	b, err := json.Marshal(evt)
	require.NoError(t, err)
	return &kgo.Record{Topic: TopicGraphMutated, Value: b}
}

// The consumer decodes exactly what the producer publishes.
func TestDecodeGraphMutations_RoundTripsProducerEvents(t *testing.T) {
	got := DecodeGraphMutations([]*kgo.Record{
		record(t, GraphMutatedEvent{MutationType: ngac.MutationRemoveAssignment, NodeIDs: []string{"u1", "ua1"}}),
		record(t, GraphMutatedEvent{MutationType: ngac.MutationLoadGraph}),
	})
	assert.Equal(t, []ngac.GraphMutation{
		{Type: ngac.MutationRemoveAssignment, NodeIDs: []string{"u1", "ua1"}},
		{Type: ngac.MutationLoadGraph},
	}, got)
}

// An undecodable event still says "the graph changed"; it must force a full
// reload rather than be skipped.
func TestDecodeGraphMutations_UndecodableEventForcesFullReload(t *testing.T) {
	got := DecodeGraphMutations([]*kgo.Record{{Topic: TopicGraphMutated, Value: []byte("{not json")}})
	assert.Equal(t, []ngac.GraphMutation{{Type: ngac.MutationLoadGraph}}, got)
}

type flakyApplier struct {
	failures int
	calls    [][]ngac.GraphMutation
}

func (f *flakyApplier) Apply(_ context.Context, batch []ngac.GraphMutation) error {
	f.calls = append(f.calls, batch)
	if len(f.calls) <= f.failures {
		return errors.New("reload failed")
	}
	return nil
}

// A failed batch is retried until it applies; it is never dropped.
func TestConsumerHandle_RetriesUntilApplied(t *testing.T) {
	applier := &flakyApplier{failures: 2}
	c := &GraphMutationConsumer{applier: applier, retryInitial: time.Millisecond, retryMax: time.Millisecond}

	c.handle(context.Background(), []*kgo.Record{
		record(t, GraphMutatedEvent{MutationType: ngac.MutationDeleteNode, NodeIDs: []string{"ua1"}}),
	})

	require.Len(t, applier.calls, 3)
	for _, b := range applier.calls {
		assert.Equal(t, []ngac.GraphMutation{{Type: ngac.MutationDeleteNode, NodeIDs: []string{"ua1"}}}, b,
			"the same batch is retried")
	}
}

func TestConsumerHandle_StopsRetryingWhenContextEnds(t *testing.T) {
	applier := &flakyApplier{failures: 1 << 30}
	c := &GraphMutationConsumer{applier: applier, retryInitial: time.Millisecond, retryMax: time.Millisecond}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	done := make(chan struct{})
	go func() {
		c.handle(ctx, []*kgo.Record{record(t, GraphMutatedEvent{MutationType: ngac.MutationLoadGraph})})
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("handle did not return after context cancellation")
	}
}

func TestConsumerHandle_EmptyFetchIsNoop(t *testing.T) {
	applier := &flakyApplier{}
	c := &GraphMutationConsumer{applier: applier, retryInitial: time.Millisecond, retryMax: time.Millisecond}
	c.handle(context.Background(), nil)
	assert.Empty(t, applier.calls)
}
