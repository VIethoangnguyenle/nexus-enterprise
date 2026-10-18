package grpc

import (
	"testing"

	"github.com/stretchr/testify/assert"

	pb "ngac-platform/proto/messaging"
	"ngac-platform/services/messaging/internal/domain"
)

// The event a session receives names the workspace of the call's conversation.
func TestCallEvent_NamesTheWorkspace(t *testing.T) {
	ev := callEvent(domain.CallChange{Type: domain.CallChangeRing, WorkspaceID: "ws-a", Call: &pb.Call{Id: "c1", ChannelId: "ch1", Kind: "audio"}})
	assert.Equal(t, "ws-a", ev.WorkspaceId)
	assert.Equal(t, "ch1", ev.ChannelId)
}
