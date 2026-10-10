package events

import (
	"context"
	"testing"
	"time"
)

// With the broker unreachable, publishing from a request that has already
// ended must return at once and never pile up behind the broker.
func TestPublishDoesNotBlockWhenTheBrokerIsDown(t *testing.T) {
	p, err := NewProducer([]string{"127.0.0.1:1"})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // the request is over by the time the event goes out

	start := time.Now()
	for i := 0; i < 6000; i++ {
		p.Publish(ctx, ApprovalEventPayload{RequestID: "r", TenantID: "t", WorkspaceID: "w"})
	}
	if d := time.Since(start); d > 2*time.Second {
		t.Fatalf("publish blocked for %v with the broker down", d)
	}
}
