package grpc_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pb "ngac-platform/proto/messaging"
)

func isNotification(e *pb.ServerEnvelope) bool { return e.GetNotification() != nil }

func notice(id string) *pb.Notification {
	return &pb.Notification{
		Id: id, Type: "approval_approved", TargetType: "approval", TargetId: "r1",
		ActorUserId: "u-boss", ActorName: "Vinh", TargetName: "Tạm ứng", WorkspaceId: "ws-a",
	}
}

// A live notification reaches its recipient in its own workspace and nobody else:
// not another person, and not the same person's session in another workspace.
func TestSendNotification_ReachesOnlyTheRecipientInTheWorkspace(t *testing.T) {
	hub, url := startHub(t, nil)
	inA := connect(t, url, identity{userID: "alice", username: "alice", nodeID: "n-alice", tenantID: "ws-a"})
	inB := connect(t, url, identity{userID: "alice", username: "alice", nodeID: "n-alice", tenantID: "ws-b"})
	bob := connect(t, url, identity{userID: "bob", username: "bob", nodeID: "n-bob", tenantID: "ws-a"})

	hub.SendNotification("ws-a", "alice", notice("n1"))

	got := inA.next(t, isNotification, 2*time.Second)
	require.NotNil(t, got, "the recipient's session in the workspace is told")
	ev := got.GetNotification()
	assert.Equal(t, "n1", ev.Id)
	assert.Equal(t, "Tạm ứng", ev.TargetName)
	assert.Equal(t, "Vinh", ev.ActorName)
	assert.Equal(t, "ws-a", ev.WorkspaceId)

	assert.Nil(t, inB.next(t, isNotification, 300*time.Millisecond), "the same person in another workspace is not")
	assert.Nil(t, bob.next(t, isNotification, 300*time.Millisecond), "another person in the same workspace is not")
}

// A personal notification (an invitation) reaches every session of the invitee,
// whichever workspace it is in, and still nobody else.
func TestSendNotification_PersonalReachesEverySessionOfTheUserOnly(t *testing.T) {
	hub, url := startHub(t, nil)
	inA := connect(t, url, identity{userID: "alice", username: "alice", nodeID: "n-alice", tenantID: "ws-a"})
	inB := connect(t, url, identity{userID: "alice", username: "alice", nodeID: "n-alice", tenantID: "ws-b"})
	bob := connect(t, url, identity{userID: "bob", username: "bob", nodeID: "n-bob", tenantID: "ws-a"})

	hub.SendNotification("", "alice", notice("inv"))

	assert.NotNil(t, inA.next(t, isNotification, 2*time.Second))
	assert.NotNil(t, inB.next(t, isNotification, 2*time.Second))
	assert.Nil(t, bob.next(t, isNotification, 300*time.Millisecond))
}
