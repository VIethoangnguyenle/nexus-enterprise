package grpc

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"ngac-platform/pkg/realtime"
	pb "ngac-platform/proto/messaging"
)

func newInternalHub(t *testing.T) *Hub {
	t.Helper()
	h := NewHub(nil, nil)
	t.Cleanup(h.Close)
	return h
}

func sessionOf(h *Hub, tenant, node string, queue int) *Client {
	c := &Client{hub: h, tenantID: tenant, ngacNodeID: node, userID: "u-" + node, send: make(chan []byte, queue)}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.users[c.userID] == nil {
		h.users[c.userID] = map[*Client]bool{}
	}
	h.users[c.userID][c] = true
	return c
}

func userFrame(t *testing.T, tenant string) []byte {
	t.Helper()
	frame := marshalEnvelope(domainEnvelope(realtime.Event{
		Domain: realtime.DomainDrive, Kind: realtime.KindUpdated, TenantID: tenant, WorkspaceID: tenant, IDs: []string{"f"},
	}, 0))
	require.NotNil(t, frame)
	return frame
}

func TestDeliverToUserNode_NumberAndFrameArriveTogetherOrNotAtAll(t *testing.T) {
	h := newInternalHub(t)
	frame := userFrame(t, "t1")

	t.Run("room for both queues the frame then its number", func(t *testing.T) {
		c := sessionOf(h, "t1", "n-room", 2)
		h.deliverToUserNode("t1", "n-room", wrapUserSeq(7, frame))
		require.Len(t, c.send, 2)
		assert.Equal(t, frame, <-c.send)
		assert.True(t, isControl(<-c.send))
	})

	t.Run("room for one drops both", func(t *testing.T) {
		c := sessionOf(h, "t1", "n-tight", 2)
		c.send <- []byte("earlier")
		h.deliverToUserNode("t1", "n-tight", wrapUserSeq(8, frame))
		assert.Len(t, c.send, 1, "a frame without its number, or a number without its frame, would move the cursor past a loss")
	})

	t.Run("a full queue drops both", func(t *testing.T) {
		c := sessionOf(h, "t1", "n-full", 2)
		c.send <- []byte("a")
		c.send <- []byte("b")
		h.deliverToUserNode("t1", "n-full", wrapUserSeq(9, frame))
		assert.Len(t, c.send, 2)
	})

	t.Run("an unnumbered frame needs one slot", func(t *testing.T) {
		c := sessionOf(h, "t1", "n-plain", 1)
		h.deliverToUserNode("t1", "n-plain", frame)
		assert.Len(t, c.send, 1)
	})
}

func TestPublishDomain_ASlowUserStreamDoesNotBlockWorkspaceEvents(t *testing.T) {
	h := newInternalHub(t)
	// Stand in for a user-stream append that is stuck on Redis: the user's own
	// lock is held, so a user event for that user waits on it.
	lock := h.dom.userLock("t-slow", "n-slow")
	lock.Lock()
	released := false
	release := func() {
		if !released {
			released = true
			lock.Unlock()
		}
	}
	defer release()

	userDone := make(chan struct{})
	go func() {
		defer close(userDone)
		h.PublishDomain(realtime.Event{
			Domain: realtime.DomainDrive, Kind: realtime.KindUpdated, TenantID: "t-slow", WorkspaceID: "t-slow",
			IDs: []string{"f"}, UserNodeIDs: []string{"n-slow"},
		})
	}()
	select {
	case <-userDone:
		t.Fatal("the user event should be waiting on its user's lock")
	case <-time.After(100 * time.Millisecond):
	}

	wsDone := make(chan struct{})
	go func() {
		defer close(wsDone)
		h.PublishDomain(driveEventFor("t-ws", "t-ws"))
	}()
	select {
	case <-wsDone:
	case <-time.After(2 * time.Second):
		t.Fatal("a workspace event waited behind a user event's stream append")
	}

	release()
	select {
	case <-userDone:
	case <-time.After(2 * time.Second):
		t.Fatal("the user event never finished once its lock was free")
	}
}

func driveEventFor(tenant, workspace string) realtime.Event {
	return realtime.Event{
		Domain: realtime.DomainDrive, Kind: realtime.KindUpdated,
		TenantID: tenant, WorkspaceID: workspace, IDs: []string{"f"}, ActorUserID: "u-actor",
	}
}

func TestEndHold_ReleasesAWholeBacklogToASlowReader(t *testing.T) {
	h := newInternalHub(t)
	const frames = 60
	c := &Client{hub: h, tenantID: "t1", send: make(chan []byte, 4)}

	c.holdWorkspace("ws")
	for i := 1; i <= frames; i++ {
		c.offerWorkspace("ws", marshalEnvelope(domainEnvelope(driveEventFor("t1", "ws"), uint64(i))))
	}
	require.Empty(t, c.send, "held frames wait for the replay to finish")

	got := make(chan uint64, frames)
	go func() {
		for i := 0; i < frames; i++ {
			data := <-c.send
			env := unmarshalEnvelopeForTest(t, data)
			got <- env
			time.Sleep(time.Millisecond) // a reader slower than the release
		}
	}()

	c.endHold(0)

	for want := uint64(1); want <= frames; want++ {
		select {
		case seq := <-got:
			require.Equal(t, want, seq, fmt.Sprintf("frame %d lost or out of order", want))
		case <-time.After(3 * time.Second):
			t.Fatalf("frame %d never arrived: held frames were dropped on release", want)
		}
	}
}

func TestEndHold_LiveFramesDuringTheReleaseStayInOrder(t *testing.T) {
	h := newInternalHub(t)
	c := &Client{hub: h, tenantID: "t1", send: make(chan []byte, 2)}
	c.holdWorkspace("ws")
	for i := 1; i <= 10; i++ {
		c.offerWorkspace("ws", marshalEnvelope(domainEnvelope(driveEventFor("t1", "ws"), uint64(i))))
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		c.endHold(0)
	}()
	// A live frame arriving while the backlog drains must not jump the queue
	// or make the publisher wait for the reader.
	start := time.Now()
	c.offerWorkspace("ws", marshalEnvelope(domainEnvelope(driveEventFor("t1", "ws"), 11)))
	assert.Less(t, time.Since(start), 500*time.Millisecond, "a publisher waited for a slow session")

	var seqs []uint64
	for len(seqs) < 11 {
		select {
		case data := <-c.send:
			seqs = append(seqs, unmarshalEnvelopeForTest(t, data))
		case <-time.After(3 * time.Second):
			t.Fatalf("only %v arrived", seqs)
		}
	}
	<-done
	for i, s := range seqs {
		assert.EqualValues(t, i+1, s)
	}
}

func unmarshalEnvelopeForTest(t *testing.T, data []byte) uint64 {
	t.Helper()
	var env pb.ServerEnvelope
	require.NoError(t, proto.Unmarshal(data, &env))
	return env.GetDomainEvent().GetSeq()
}
