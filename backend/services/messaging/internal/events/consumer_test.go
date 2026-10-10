package events

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/services/messaging/internal/domain"
)

type nopNotifier struct{}

func (nopNotifier) CreateNotification(context.Context, domain.NewNotification) error { return nil }
func (nopNotifier) NotifyInvitation(context.Context, string) error                   { return nil }

type capturingBroadcaster struct{ got []ApprovalNotice }

func (c *capturingBroadcaster) BroadcastApprovalEvent(n ApprovalNotice) { c.got = append(c.got, n) }

// The WebSocket fan-out of an approval event is addressed to the people the
// event names — actor, requester, assignees — and to nobody else.
func TestHandleApprovalEvent_AddressesNamedParticipants(t *testing.T) {
	b := &capturingBroadcaster{}
	c := &Consumer{notifSv: nopNotifier{}, broadcast: b}

	c.handleApprovalEvent(context.Background(), []byte(`{
		"request_id": "req-1", "template_name": "Leave", "status": "pending", "action": "created",
		"actor_node_id": "n-actor", "created_by": "n-creator",
		"assignee_node_ids": ["n-approver", "n-actor", ""],
		"tenant_id": "tenant-a"
	}`))

	require.Len(t, b.got, 1)
	n := b.got[0]
	assert.Equal(t, "req-1", n.RequestID)
	assert.Equal(t, "tenant-a", n.TenantID)
	assert.ElementsMatch(t, []string{"n-actor", "n-creator", "n-approver"}, n.RecipientNodeIDs)
}

// An event naming nobody is not broadcast at all — the old behaviour of
// sending it to every connected user is exactly the leak being closed.
func TestHandleApprovalEvent_NamingNobodyIsNotBroadcast(t *testing.T) {
	b := &capturingBroadcaster{}
	c := &Consumer{notifSv: nopNotifier{}, broadcast: b}

	c.handleApprovalEvent(context.Background(), []byte(`{"request_id": "req-2", "action": "approved"}`))

	assert.Empty(t, b.got)
}
