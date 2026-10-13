package grpc_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pb "ngac-platform/proto/messaging"
	"ngac-platform/services/messaging/internal/domain"
)

func isCall(callID, kind string) func(*pb.ServerEnvelope) bool {
	return func(e *pb.ServerEnvelope) bool {
		c := e.GetCallEvent()
		return c != nil && c.CallId == callID && c.Type == kind
	}
}

// A direct call rings every device of the callee (and the caller's, so their
// other devices know), wherever they are; nobody else hears it ring. The
// conversation's subscribers see its state; someone not subscribed does not.
func TestCallChanged_RingsThePeopleNamedStateToSubscribers(t *testing.T) {
	access := &fakeAccess{allow: map[[3]string]bool{
		{"dm-1", "n-a", "read"}: true,
		{"dm-1", "n-b", "read"}: true,
	}}
	hub, url := startHub(t, access)
	calleePhone := connect(t, url, identity{"u-b", "bob", "n-b", ""})
	calleeDesk := connect(t, url, identity{"u-b", "bob", "n-b", "tenant-a"})
	caller := connect(t, url, identity{"u-a", "alice", "n-a", "tenant-a"})
	stranger := connect(t, url, identity{"u-c", "carol", "n-c", "tenant-a"})
	subscribeAndWait(t, hub, caller, "dm-1")

	call := &pb.Call{Id: "call-1", ChannelId: "dm-1", Kind: "video", Status: "ringing", StartedBy: "u-a", StartedByName: "alice"}
	hub.CallChanged(context.Background(), domain.CallChange{Type: domain.CallChangeRing, Call: call, Users: []string{"u-b"}})

	for name, c := range map[string]*wsTestClient{"phone": calleePhone, "desk": calleeDesk} {
		got := c.next(t, isCall("call-1", "ring"), 2*time.Second)
		require.NotNil(t, got, "the callee's %s rings", name)
		assert.Equal(t, "alice", got.GetCallEvent().CallerName)
	}
	assert.NotNil(t, caller.next(t, isCall("call-1", "state"), 2*time.Second), "the subscribed caller sees the state")
	assert.Nil(t, stranger.next(t, isCall("call-1", "ring"), quiet), "a stranger hears no ring")
	assert.Nil(t, stranger.next(t, isCall("call-1", "state"), quiet), "nor the state of a conversation they do not follow")
}

// The card's new text reaches the conversation with the state.
func TestCallChanged_CardGoesOutAsAnEdit(t *testing.T) {
	access := &fakeAccess{allow: map[[3]string]bool{{"ch-1", "n-a", "read"}: true}}
	hub, url := startHub(t, access)
	watcher := connect(t, url, identity{"u-a", "alice", "n-a", "tenant-a"})
	subscribeAndWait(t, hub, watcher, "ch-1")

	call := &pb.Call{Id: "call-2", ChannelId: "ch-1", Kind: "audio", Status: "ended"}
	card := &pb.Message{Id: "card-2", ChannelId: "ch-1", Content: "📞 Cuộc gọi thoại · 3 phút"}
	hub.CallChanged(context.Background(), domain.CallChange{Type: domain.CallChangeState, Call: call, Card: card})

	got := watcher.next(t, func(e *pb.ServerEnvelope) bool { return e.GetMessageEdited() != nil }, 2*time.Second)
	require.NotNil(t, got)
	assert.Equal(t, "card-2", got.GetMessageEdited().Message.Id)
	assert.NotNil(t, watcher.next(t, isCall("call-2", "state"), 2*time.Second))
}
