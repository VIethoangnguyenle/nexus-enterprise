package events

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type nopNotifier struct{}

func (nopNotifier) CreateNotification(context.Context, string, string, string, string, string, string) error {
	return nil
}

type capturingBroadcaster struct{ got []ApprovalNotice }

func (c *capturingBroadcaster) BroadcastApprovalEvent(n ApprovalNotice) { c.got = append(c.got, n) }

// ── Mocks for node-ID resolution tests ──────────────────────────────────

// capturingNotifier records every CreateNotification call so tests can inspect
// which user IDs the consumer stored notifications for.
type capturingNotifier struct {
	calls []notifCall
}

type notifCall struct {
	userID, notifType, title, body, entityType, entityID string
}

func (c *capturingNotifier) CreateNotification(_ context.Context, userID, notifType, title, body, entityType, entityID string) error {
	c.calls = append(c.calls, notifCall{userID, notifType, title, body, entityType, entityID})
	return nil
}

// mockNodeResolver maps NGAC node IDs to real user UUIDs; unknown nodes return "".
type mockNodeResolver struct {
	m map[string]string
}

func (r *mockNodeResolver) UserIDByNode(_ context.Context, nodeID string) (string, error) {
	return r.m[nodeID], nil
}

// ── WebSocket broadcast tests (unchanged behaviour) ─────────────────────

// The WebSocket fan-out of an approval event is addressed to the people the
// event names — actor, requester, assignees — and to nobody else.
func TestHandleApprovalEvent_AddressesNamedParticipants(t *testing.T) {
	b := &capturingBroadcaster{}
	resolver := &mockNodeResolver{m: map[string]string{
		"n-actor":    "uid-actor",
		"n-creator":  "uid-creator",
		"n-approver": "uid-approver",
	}}
	c := &Consumer{notifSv: nopNotifier{}, broadcast: b, resolver: resolver}

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
	// WebSocket broadcast still uses node IDs — that is correct.
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

// ── Node-ID → user-ID resolution tests ──────────────────────────────────

// Approval notifications must be stored with real user IDs, not NGAC node IDs.
// The resolver translates node IDs before the consumer calls notify().
func TestHandleApprovalEvent_ResolvesNodeIDsToUserIDs(t *testing.T) {
	notif := &capturingNotifier{}
	resolver := &mockNodeResolver{m: map[string]string{
		"n-creator":   "uid-creator",
		"n-approver1": "uid-approver1",
		"n-approver2": "uid-approver2",
		"n-actor":     "uid-actor",
	}}
	c := &Consumer{notifSv: notif, resolver: resolver}

	// "approved" by n-actor → notify the creator (result notification) and
	// pending approvers via notifyPendingApprovers.
	c.handleApprovalEvent(context.Background(), []byte(`{
		"request_id": "req-10", "template_name": "PTO",
		"status": "approved", "action": "approved",
		"actor_node_id": "n-actor", "created_by": "n-creator",
		"assignee_node_ids": ["n-approver1", "n-approver2"],
		"tenant_id": "t1", "workspace_id": "ws1"
	}`))

	// Collect all user IDs that received a notification.
	var userIDs []string
	for _, c := range notif.calls {
		userIDs = append(userIDs, c.userID)
	}

	// The creator should get an approval_approved notification with their
	// real user ID, not the NGAC node ID.
	assert.Contains(t, userIDs, "uid-creator", "creator must receive notification with resolved user ID")
	assert.NotContains(t, userIDs, "n-creator", "NGAC node ID must not be stored as user ID")

	// Approvers (pending) should also be stored with real user IDs.
	assert.Contains(t, userIDs, "uid-approver1")
	assert.Contains(t, userIDs, "uid-approver2")
	assert.NotContains(t, userIDs, "n-approver1")
	assert.NotContains(t, userIDs, "n-approver2")
}

// When a node ID cannot be resolved (user deleted, stale graph), the
// notification is silently skipped rather than stored with an invalid ID.
func TestHandleApprovalEvent_SkipsUnresolvableNodeIDs(t *testing.T) {
	notif := &capturingNotifier{}
	resolver := &mockNodeResolver{m: map[string]string{
		// n-creator is known, but n-ghost is not.
		"n-creator": "uid-creator",
	}}
	c := &Consumer{notifSv: notif, resolver: resolver}

	c.handleApprovalEvent(context.Background(), []byte(`{
		"request_id": "req-11", "template_name": "Travel",
		"status": "pending", "action": "created",
		"actor_node_id": "n-creator", "created_by": "n-creator",
		"assignee_node_ids": ["n-ghost"]
	}`))

	// n-ghost cannot be resolved → no notification should be created for it.
	for _, call := range notif.calls {
		assert.NotEqual(t, "n-ghost", call.userID, "unresolvable node ID must not appear as user ID")
		assert.NotEqual(t, "", call.userID, "empty user ID must not be stored")
	}
}

// WS broadcast still uses raw node IDs even after the resolution fix — hub
// matches by client.ngacNodeID, so node IDs are correct for WS delivery.
func TestHandleApprovalEvent_WSBroadcastKeepsNodeIDs(t *testing.T) {
	notif := &capturingNotifier{}
	b := &capturingBroadcaster{}
	resolver := &mockNodeResolver{m: map[string]string{
		"n-actor":    "uid-actor",
		"n-creator":  "uid-creator",
		"n-approver": "uid-approver",
	}}
	c := &Consumer{notifSv: notif, broadcast: b, resolver: resolver}

	c.handleApprovalEvent(context.Background(), []byte(`{
		"request_id": "req-12", "template_name": "Leave",
		"status": "pending", "action": "created",
		"actor_node_id": "n-actor", "created_by": "n-creator",
		"assignee_node_ids": ["n-approver"],
		"tenant_id": "t1"
	}`))

	require.Len(t, b.got, 1)
	// The broadcast must carry the original NGAC node IDs, NOT the resolved user IDs.
	assert.ElementsMatch(t, []string{"n-actor", "n-creator", "n-approver"}, b.got[0].RecipientNodeIDs)
}
