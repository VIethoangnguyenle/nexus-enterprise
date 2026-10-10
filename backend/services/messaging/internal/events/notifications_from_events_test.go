package events

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/services/messaging/internal/domain"
)

type recordingNotifier struct {
	got         []domain.NewNotification
	invitations []string
	err         error
}

func (r *recordingNotifier) CreateNotification(_ context.Context, n domain.NewNotification) error {
	r.got = append(r.got, n)
	return r.err
}

func (r *recordingNotifier) NotifyInvitation(_ context.Context, id string) error {
	r.invitations = append(r.invitations, id)
	return r.err
}

func consumer() (*Consumer, *recordingNotifier) {
	n := &recordingNotifier{}
	return &Consumer{notifSv: n}, n
}

// A state change is not news to the person who made it, and there is no one else
// to tell: the lifecycle topic raises no notification at all.
func TestLifecycleEvents_RaiseNoNotification(t *testing.T) {
	c, n := consumer()
	c.handleRecord(context.Background(), "asset.lifecycle",
		[]byte(`{"asset_id":"a1","asset_name":"Laptop","action":"approve","actor_id":"u-actor","tenant_id":"w"}`))
	assert.Empty(t, n.got)
}

func TestRequestEvent_TellsTheRequesterOnlyOfADecisionAndNamesWhoDecided(t *testing.T) {
	for status, wantKind := range map[string]string{
		"approved": "asset_request_approved", "rejected": "asset_request_rejected",
	} {
		c, n := consumer()
		c.handleRequestEvent(context.Background(), []byte(
			`{"request_id":"r1","type_name":"Laptop","requester_id":"u-req","approver_id":"u-lan","status":"`+status+`","tenant_id":"w"}`))
		require.Len(t, n.got, 1, status)
		got := n.got[0]
		assert.Equal(t, "u-req", got.Recipient, status)
		assert.Equal(t, "u-lan", got.Actor, status)
		assert.Equal(t, wantKind, got.Type, status)
		assert.Equal(t, "w", got.WorkspaceID, status)
		assert.Equal(t, "asset_request", got.TargetType, status)
		assert.Equal(t, "r1", got.TargetID, status)
		assert.Equal(t, "Laptop", got.TargetName, status)
	}

	// Submitting a request does not notify the requester about their own request.
	for _, status := range []string{"pending", "fulfilled", ""} {
		c, n := consumer()
		c.handleRequestEvent(context.Background(), []byte(`{"request_id":"r1","requester_id":"u-req","status":"`+status+`","tenant_id":"w"}`))
		assert.Empty(t, n.got, "status %q tells nobody", status)
	}
}

func TestAssignmentEvent_NotifiesTheRightPersonAndNamesTheActor(t *testing.T) {
	c, n := consumer()
	c.handleAssignmentEvent(context.Background(), []byte(
		`{"asset_id":"a1","asset_name":"Laptop","action":"assign","to_user_id":"u-to","from_user_id":"u-from","actor_id":"u-lan","tenant_id":"w"}`))
	require.Len(t, n.got, 1)
	assert.Equal(t, "u-to", n.got[0].Recipient, "an assignment tells the new holder")
	assert.Equal(t, "u-lan", n.got[0].Actor)
	assert.Equal(t, "asset_assigned", n.got[0].Type)
	assert.Equal(t, "Laptop", n.got[0].TargetName)

	c, n = consumer()
	c.handleAssignmentEvent(context.Background(), []byte(
		`{"asset_id":"a1","asset_name":"Laptop","action":"return","to_user_id":"u-to","from_user_id":"u-from","actor_id":"u-yen","tenant_id":"w"}`))
	require.Len(t, n.got, 1)
	assert.Equal(t, "u-from", n.got[0].Recipient, "a return tells the one who held it")
	assert.Equal(t, "asset_returned", n.got[0].Type)

	for _, body := range []string{
		`{"action":"assign","tenant_id":"w"}`,                 // nobody to tell
		`{"action":"return","tenant_id":"w"}`,                 // nobody to tell
		`{"action":"other","to_user_id":"x","tenant_id":"w"}`, // an action nobody is told about
		`{"action":"assign","to_user_id":"x"}`,                // no tenant: no workspace to belong to
	} {
		c, n = consumer()
		c.handleAssignmentEvent(context.Background(), []byte(body))
		assert.Empty(t, n.got, body)
	}
}

func TestApprovalEvent_NotifiesTheRequesterNamingTheNodesAndWorkspace(t *testing.T) {
	c, n := consumer()
	c.handleApprovalEvent(context.Background(), []byte(`{"request_id":"r1","template_name":"Leave","action":"rejected","comment":"no budget","actor_node_id":"n-boss","created_by":"n-me","tenant_id":"w","status":"rejected"}`))
	require.Len(t, n.got, 1)
	got := n.got[0]
	assert.Equal(t, "n-me", got.Recipient, "the service resolves the node to the requester")
	assert.Equal(t, "n-boss", got.Actor)
	assert.Equal(t, "approval_rejected", got.Type)
	assert.Equal(t, "w", got.WorkspaceID)
	assert.Equal(t, "approval", got.TargetType)
	assert.Equal(t, "Leave", got.TargetName)
	assert.Equal(t, map[string]string{"reason": "no budget"}, got.Params, "the reason travels as a field, not inside a sentence")
}

// A step passing is not the request finishing: the two are different types.
func TestApprovalEvent_SeparatesAStepFromTheCompletedRequest(t *testing.T) {
	c, n := consumer()
	c.handleApprovalEvent(context.Background(), []byte(`{"request_id":"r1","action":"approved","status":"pending","actor_node_id":"n-boss","created_by":"n-me","tenant_id":"w"}`))
	require.Len(t, n.got, 1)
	assert.Equal(t, "approval_step_approved", n.got[0].Type, "pending after the approval means more steps remain")

	c, n = consumer()
	c.handleApprovalEvent(context.Background(), []byte(`{"request_id":"r1","action":"approved","status":"approved","actor_node_id":"n-boss","created_by":"n-me","tenant_id":"w"}`))
	require.Len(t, n.got, 1)
	assert.Equal(t, "approval_approved", n.got[0].Type, "no longer pending means the request is complete")
}

func TestApprovalEvent_NoTenantNoCreatorOrAnUnknownActionNotifiesNobody(t *testing.T) {
	for _, body := range []string{
		`{"request_id":"r1","action":"approved","actor_node_id":"n-boss","created_by":"n-me"}`,              // no tenant
		`{"request_id":"r1","action":"approved","actor_node_id":"n-boss","tenant_id":"w"}`,                  // no creator
		`{"request_id":"r1","action":"created","actor_node_id":"n-me","created_by":"n-me","tenant_id":"w"}`, // not a decision
	} {
		c, n := consumer()
		c.handleApprovalEvent(context.Background(), []byte(body))
		assert.Empty(t, n.got, body)
	}
}

func TestWorkspaceEvent_OnlyAnInvitationCreatedRaisesANotification(t *testing.T) {
	c, n := consumer()
	c.handleWorkspaceEvent(context.Background(), []byte(`{"domain":"workspace","kind":"invitation_created","tenant_id":"w","workspace_id":"w","ids":["inv-1","inv-2"]}`))
	assert.Equal(t, []string{"inv-1", "inv-2"}, n.invitations)

	for _, body := range []string{
		`{"domain":"workspace","kind":"role_changed","tenant_id":"w","workspace_id":"w","ids":["x"]}`,
		`{"domain":"drive","kind":"invitation_created","tenant_id":"w","workspace_id":"w","ids":["x"]}`,
		`{not json`,
		`{"domain":"workspace","kind":"invitation_created","tenant_id":"w","workspace_id":"w","ids":["x"],"at":1000}`, // long ago
	} {
		c, n = consumer()
		c.handleWorkspaceEvent(context.Background(), []byte(body))
		assert.Empty(t, n.invitations, body)
	}
}

func TestEvents_MalformedOrUnstorableOnesAreSkippedNotFatal(t *testing.T) {
	c, n := consumer()
	for _, h := range []func(context.Context, []byte){c.handleRequestEvent, c.handleAssignmentEvent, c.handleApprovalEvent, c.handleWorkspaceEvent} {
		h(context.Background(), []byte(`{not json`))
	}
	assert.Empty(t, n.got, "garbage creates no notification")

	c, n = consumer()
	n.err = errors.New("db down")
	c.handleAssignmentEvent(context.Background(), []byte(`{"asset_id":"a1","action":"assign","to_user_id":"u","tenant_id":"w"}`))
	assert.Len(t, n.got, 1, "the failure is logged and the handler returns")

	n.err = domain.ErrNotAMember
	c.handleAssignmentEvent(context.Background(), []byte(`{"asset_id":"a1","action":"assign","to_user_id":"u","tenant_id":"w"}`))
}
