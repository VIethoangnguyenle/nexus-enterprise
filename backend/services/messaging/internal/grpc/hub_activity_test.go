package grpc

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	pb "ngac-platform/proto/messaging"
)

// fakeDirectory lists conversations' members; each lookup is counted.
type fakeDirectory struct {
	mu      sync.Mutex
	ws      map[string]string
	members map[string][]string
	loads   int
	hold    chan struct{}
}

func (d *fakeDirectory) ChannelMemberNodes(_ context.Context, channelID string) (string, []string, error) {
	d.mu.Lock()
	d.loads++
	ws, members, hold := d.ws[channelID], append([]string(nil), d.members[channelID]...), d.hold
	d.mu.Unlock()
	if hold != nil {
		<-hold // the list is read; its answer is slow to come back
	}
	return ws, members, nil
}

// holdNextLoad makes the next reads of the directory wait for release.
func (d *fakeDirectory) holdNextLoad() (release func()) {
	ch := make(chan struct{})
	d.mu.Lock()
	d.hold = ch
	d.mu.Unlock()
	return func() {
		d.mu.Lock()
		d.hold = nil
		d.mu.Unlock()
		close(ch)
	}
}

func (d *fakeDirectory) remove(channelID, node string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	var kept []string
	for _, n := range d.members[channelID] {
		if n != node {
			kept = append(kept, n)
		}
	}
	d.members[channelID] = kept
}

// followerOf is a session of node in tenant that follows the tenant's stream,
// as every app session does.
func followerOf(h *Hub, tenant, node string) *Client {
	c := sessionOf(h, tenant, node, 16)
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.dom.workspaces[tenant] == nil {
		h.dom.workspaces[tenant] = map[*Client]bool{}
	}
	h.dom.workspaces[tenant][c] = true
	return c
}

// activities waits briefly and returns the activity frames a session holds.
func activities(t *testing.T, c *Client) []*pb.ConversationActivityEvent {
	t.Helper()
	var out []*pb.ConversationActivityEvent
	deadline := time.After(200 * time.Millisecond)
	for {
		select {
		case data := <-c.send:
			var env pb.ServerEnvelope
			require.NoError(t, proto.Unmarshal(data, &env))
			if ev := env.GetConversationActivity(); ev != nil {
				out = append(out, ev)
			}
		case <-deadline:
			return out
		}
	}
}

func activityWorld(t *testing.T) (*Hub, *fakeDirectory) {
	t.Helper()
	h := newInternalHub(t)
	dir := &fakeDirectory{
		ws:      map[string]string{"ch": "t1"},
		members: map[string][]string{"ch": {"n-member", "n-other-device"}},
	}
	h.SetChannelDirectory(dir)
	return h, dir
}

// A message reaches every session of every member, open or not, with its
// number and mentions and no text; a non-member of the same workspace, and a
// session of the same person signed in to another workspace, hear nothing.
func TestActivity_ReachesMembersOnly(t *testing.T) {
	h, _ := activityWorld(t)
	member := followerOf(h, "t1", "n-member")
	otherDevice := followerOf(h, "t1", "n-other-device")
	stranger := followerOf(h, "t1", "n-stranger")
	elsewhere := followerOf(h, "t2", "n-member")

	h.BroadcastToChannel("ch", &pb.Message{Id: "m1", ChannelId: "ch", SenderId: "u-a", Content: "secret",
		Mentions: []string{"u-member"}, Seq: 7})

	for _, c := range []*Client{member, otherDevice} {
		got := activities(t, c)
		require.Len(t, got, 1)
		assert.Equal(t, "sent", got[0].Kind)
		assert.EqualValues(t, 7, got[0].Seq)
		assert.EqualValues(t, 7, got[0].LastSeq)
		assert.Equal(t, []string{"u-member"}, got[0].Mentions)
	}
	assert.Empty(t, activities(t, stranger), "not a member")
	assert.Empty(t, activities(t, elsewhere), "signed in to another workspace")
}

// Someone removed from a conversation stops hearing of it at once, though the
// instance had their membership in memory.
func TestActivity_RemovedMemberStopsHearingAtOnce(t *testing.T) {
	h, dir := activityWorld(t)
	member := followerOf(h, "t1", "n-member")

	h.BroadcastToChannel("ch", &pb.Message{Id: "m1", ChannelId: "ch", SenderId: "u-a", Seq: 1})
	require.Len(t, activities(t, member), 1)

	dir.remove("ch", "n-member")
	h.revokeLocal("ch", "n-member")
	h.BroadcastToChannel("ch", &pb.Message{Id: "m2", ChannelId: "ch", SenderId: "u-a", Seq: 2})
	assert.Empty(t, activities(t, member))
}

// A member list read just before a removal, and answered just after it, is not
// remembered: the next message does not reach the removed person.
func TestActivity_AListReadBeforeARemovalIsNotKept(t *testing.T) {
	h, dir := activityWorld(t)
	member := followerOf(h, "t1", "n-member")

	release := dir.holdNextLoad()
	h.BroadcastToChannel("ch", &pb.Message{Id: "m1", ChannelId: "ch", SenderId: "u-a", Seq: 1})
	time.Sleep(50 * time.Millisecond) // the read of the list has begun and holds the old members
	dir.remove("ch", "n-member")
	h.forgetMember("", "ch", "n-member")
	release()
	activities(t, member) // what the message sent before the removal did is not the point

	h.BroadcastToChannel("ch", &pb.Message{Id: "m2", ChannelId: "ch", SenderId: "u-a", Seq: 2})
	for _, ev := range activities(t, member) {
		assert.NotEqualValues(t, 2, ev.Seq, "the removed member heard of a later message")
	}
}

// The same holds when they leave the conversation's whole workspace.
func TestActivity_MemberWhoLeftTheWorkspaceStopsHearing(t *testing.T) {
	h, dir := activityWorld(t)
	member := followerOf(h, "t1", "n-member")
	h.BroadcastToChannel("ch", &pb.Message{Id: "m1", ChannelId: "ch", SenderId: "u-a", Seq: 1})
	require.Len(t, activities(t, member), 1)

	dir.remove("ch", "n-member")
	h.forgetMember("t1", "", "n-member")
	h.BroadcastToChannel("ch", &pb.Message{Id: "m2", ChannelId: "ch", SenderId: "u-a", Seq: 2})
	assert.Empty(t, activities(t, member))
}

// A reply and a deletion are told too: a reply names its thread and has no main
// number, a deletion carries the deleted message's number.
func TestActivity_RepliesAndDeletionsAreTold(t *testing.T) {
	h, _ := activityWorld(t)
	member := followerOf(h, "t1", "n-member")

	h.BroadcastThreadReply("ch", &pb.ThreadReplyEvent{ParentMessageId: "root",
		Message: &pb.ChatMessage{Id: "r1", ChannelId: "ch", SenderId: "u-a", ParentMessageId: "root"}})
	h.BroadcastMessageDeleted("ch", &pb.MessageDeletedEvent{MessageId: "m1", ChannelId: "ch", Seq: 5})

	got := activities(t, member)
	require.Len(t, got, 2)
	assert.Equal(t, "sent", got[0].Kind)
	assert.Equal(t, "root", got[0].RootMessageId)
	assert.Zero(t, got[0].Seq)
	assert.Equal(t, "deleted", got[1].Kind)
	assert.EqualValues(t, 5, got[1].Seq)
}

// Members are read from the directory once while fresh, not per message.
func TestActivity_MembersAreReadOnceWhileFresh(t *testing.T) {
	h, dir := activityWorld(t)
	followerOf(h, "t1", "n-member")
	for i := range 20 {
		h.BroadcastToChannel("ch", &pb.Message{Id: "m", ChannelId: "ch", SenderId: "u-a", Seq: int64(i + 1)})
	}
	time.Sleep(50 * time.Millisecond)
	dir.mu.Lock()
	defer dir.mu.Unlock()
	assert.Equal(t, 1, dir.loads)
}

// Without a directory, activity is off: nothing is published or delivered.
func TestActivity_OffWithoutADirectory(t *testing.T) {
	h := newInternalHub(t)
	member := followerOf(h, "t1", "n-member")
	h.BroadcastToChannel("ch", &pb.Message{Id: "m1", ChannelId: "ch", SenderId: "u-a", Seq: 1})
	assert.Empty(t, activities(t, member))
}
