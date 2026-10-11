package grpc_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pb "ngac-platform/proto/messaging"
)

func isThreadReply(rootID string) func(*pb.ServerEnvelope) bool {
	return func(e *pb.ServerEnvelope) bool {
		r := e.GetThreadReply()
		return r != nil && r.ParentMessageId == rootID
	}
}

func isThreadState(rootID string) func(*pb.ServerEnvelope) bool {
	return func(e *pb.ServerEnvelope) bool {
		s := e.GetThreadState()
		return s != nil && s.RootMessageId == rootID
	}
}

// A reply's summary reaches everyone watching the channel; the follower's own
// unread count reaches only that follower's sessions, in that tenant.
func TestThreadEvents_SummaryToTheChannelStateToTheFollowerOnly(t *testing.T) {
	access := allowAll([2]string{"ws-1", "n-a1"}, [2]string{"ws-1", "n-a2"})
	hub, url := startDomainHub(t, access)
	follower := connect(t, url, identity{"u-a1", "alice", "n-a1", "tenant-a"})
	followerTab2 := connect(t, url, identity{"u-a1", "alice", "n-a1", "tenant-a"})
	watcher := connect(t, url, identity{"u-a2", "bob", "n-a2", "tenant-a"})
	sameNodeOtherTenant := connect(t, url, identity{"u-a1", "alice", "n-a1", "tenant-b"})
	for _, c := range []*wsTestClient{follower, watcher} {
		subscribeAndWait(t, hub, c, "ch-1")
	}

	hub.BroadcastThreadReply("ch-1", &pb.ThreadReplyEvent{
		Message:         &pb.ChatMessage{Id: "reply-1", ChannelId: "ch-1", ParentMessageId: "root-1"},
		ParentMessageId: "root-1", ReplyCount: 3, ReplyParticipantIds: []string{"u-a2"},
	})
	hub.SendThreadStates("tenant-a", map[string]*pb.ThreadStateEvent{
		"n-a1": {RootMessageId: "root-1", ChannelId: "ch-1", Following: true, UnreadCount: 2},
	})

	for name, c := range map[string]*wsTestClient{"follower": follower, "watcher": watcher} {
		got := c.next(t, isThreadReply("root-1"), 2*time.Second)
		require.NotNil(t, got, name+" sees the summary")
		assert.EqualValues(t, 3, got.GetThreadReply().ReplyCount)
	}
	for _, c := range []*wsTestClient{follower, followerTab2} {
		got := c.next(t, isThreadState("root-1"), 2*time.Second)
		require.NotNil(t, got, "every session of the follower gets its state")
		assert.EqualValues(t, 2, got.GetThreadState().UnreadCount)
	}
	assert.Nil(t, watcher.next(t, isThreadState("root-1"), quiet), "a non-follower gets no unread count")
	assert.Nil(t, sameNodeOtherTenant.next(t, isThreadState("root-1"), quiet), "another tenant gets nothing")
	assert.Nil(t, sameNodeOtherTenant.next(t, isThreadReply("root-1"), quiet))
}

// Edits and deletions reach the channel's subscribers, and a deletion carries
// no body.
func TestLifecycleEvents_ReachTheChannelsSubscribers(t *testing.T) {
	hub, url := startDomainHub(t, allowAll())
	c := connect(t, url, identity{"u-a1", "alice", "n-a1", "tenant-a"})
	subscribeAndWait(t, hub, c, "ch-1")

	hub.BroadcastMessageEdited("ch-1", &pb.Message{Id: "m-1", ChannelId: "ch-1", Content: "sửa"})
	got := c.next(t, func(e *pb.ServerEnvelope) bool { return e.GetMessageEdited() != nil }, 2*time.Second)
	require.NotNil(t, got)
	assert.Equal(t, "sửa", got.GetMessageEdited().Message.Content)

	hub.BroadcastMessageDeleted("ch-1", &pb.MessageDeletedEvent{MessageId: "m-1", ChannelId: "ch-1", KeepPlaceholder: true})
	got = c.next(t, func(e *pb.ServerEnvelope) bool { return e.GetMessageDeleted() != nil }, 2*time.Second)
	require.NotNil(t, got)
	assert.True(t, got.GetMessageDeleted().KeepPlaceholder)
}

// Without a tenant there is no boundary to deliver inside: nothing is sent.
func TestThreadEvents_StateWithoutTenantIsDropped(t *testing.T) {
	hub, url := startDomainHub(t, allowAll())
	c := connect(t, url, identity{"u-a1", "alice", "n-a1", "tenant-a"})
	hub.SendThreadStates("", map[string]*pb.ThreadStateEvent{"n-a1": {RootMessageId: "root-1"}})
	assert.Nil(t, c.next(t, isThreadState("root-1"), quiet))
}
