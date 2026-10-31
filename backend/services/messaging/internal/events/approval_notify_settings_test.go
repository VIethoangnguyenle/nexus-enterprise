package events

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func kinds(n *recordingNotifier) map[string][]string {
	out := map[string][]string{}
	for _, g := range n.got {
		out[g.kind] = append(out[g.kind], g.user)
	}
	return out
}

// "đề nghị chờ bạn": each approver the request now waits on is told, unless they
// turned it off; the actor is never told of their own doing.
func TestApprovalEvent_PendingNotifiesApproversWhoKeptItOn(t *testing.T) {
	c, n := consumer()
	c.handleApprovalEvent(context.Background(), []byte(`{"request_id":"r1","template_name":"Leave","action":"created",
		"actor_node_id":"n-req","created_by":"n-req","assignee_node_ids":["n-ap1","n-ap2","n-req"]}`))

	assert.ElementsMatch(t, []string{"n-ap1", "n-ap2"}, kinds(n)["approval_pending"])
	require.NotEmpty(t, n.got)
	assert.Equal(t, "r1", n.got[0].entityID)
	assert.Equal(t, "approval", n.got[0].entityType)
}

func TestApprovalEvent_PendingSkipsAnApproverWhoTurnedItOff(t *testing.T) {
	c, n := consumer()
	c.handleApprovalEvent(context.Background(), []byte(`{"request_id":"r1","action":"approved","actor_node_id":"n-ap1",
		"created_by":"n-req","assignee_node_ids":["n-ap2","n-ap3"],"pending_notify_off":["n-ap2"]}`))

	assert.Equal(t, []string{"n-ap3"}, kinds(n)["approval_pending"], "n-ap2 turned đề nghị chờ bạn off")
	assert.Equal(t, []string{"n-req"}, kinds(n)["approval_approved"], "the requester's own switch is separate")
}

// "An approval request is waiting for you" is said once per step. A co-approver
// approving a step that needs everyone leaves the step where it was, and the
// approvers still pending on it, who were told when it began, are not told
// again. A step that advanced tells its approvers; an event from an older
// producer, which does not say, tells them as before.
func TestApprovalEvent_PendingIsToldOncePerStep(t *testing.T) {
	for _, tc := range []struct {
		name, field string
		want        []string
	}{
		{"step stayed", `,"step_advanced":false`, nil},
		{"step advanced", `,"step_advanced":true`, []string{"n-ap2"}},
		{"older producer", ``, []string{"n-ap2"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, n := consumer()
			c.handleApprovalEvent(context.Background(), []byte(`{"request_id":"r1","action":"approved","actor_node_id":"n-ap1",
				"created_by":"n-req","assignee_node_ids":["n-ap2"]`+tc.field+`}`))

			assert.Equal(t, tc.want, kinds(n)["approval_pending"])
		})
	}
}

// A rejection waits on nobody.
func TestApprovalEvent_RejectionTellsNoApprover(t *testing.T) {
	c, n := consumer()
	c.handleApprovalEvent(context.Background(), []byte(`{"request_id":"r1","action":"rejected","actor_node_id":"n-ap1",
		"created_by":"n-req","assignee_node_ids":["n-ap2"]}`))

	assert.Empty(t, kinds(n)["approval_pending"])
}

// The result of a request reaches the requester only while their notify_result
// is on; an event from a producer that sends no switch counts as on.
func TestApprovalEvent_ResultFollowsTheRequesterSwitch(t *testing.T) {
	for _, tc := range []struct {
		name, field string
		want        int
	}{
		{"off", `,"notify_result":false`, 0},
		{"on", `,"notify_result":true`, 1},
		{"absent", ``, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, action := range []string{"approved", "rejected"} {
				c, n := consumer()
				c.handleApprovalEvent(context.Background(), []byte(`{"request_id":"r1","action":"`+action+
					`","actor_node_id":"n-boss","created_by":"n-me"`+tc.field+`}`))
				assert.Len(t, kinds(n)["approval_"+action], tc.want, action)
			}
		})
	}
}
