package events

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type notice struct{ user, kind, title, body, entityType, entityID string }

type recordingNotifier struct {
	got []notice
	err error
}

func (r *recordingNotifier) CreateNotification(_ context.Context, user, kind, title, body, entityType, entityID string) error {
	r.got = append(r.got, notice{user, kind, title, body, entityType, entityID})
	return r.err
}

func consumer() (*Consumer, *recordingNotifier) {
	n := &recordingNotifier{}
	return &Consumer{notifSv: n}, n
}

func TestLifecycleEvent_ConfirmsToTheActorOnly(t *testing.T) {
	c, n := consumer()
	c.handleLifecycleEvent(context.Background(), []byte(`{"asset_id":"a1","asset_name":"Laptop","action":"approve","from_state":"requested","to_state":"approved","actor_id":"u-actor"}`))

	require.Len(t, n.got, 1)
	assert.Equal(t, "u-actor", n.got[0].user)
	assert.Equal(t, "asset_lifecycle", n.got[0].kind)
	assert.Equal(t, "a1", n.got[0].entityID)
	assert.Contains(t, n.got[0].body, "requested to approved")
}

func TestRequestEvent_TellsTheRequesterWhereTheirRequestStands(t *testing.T) {
	for status, wantKind := range map[string]string{
		"pending": "asset_request", "approved": "asset_request_approved", "rejected": "asset_request_rejected",
	} {
		c, n := consumer()
		c.handleRequestEvent(context.Background(), []byte(`{"request_id":"r1","type_name":"Laptop","requester_id":"u-req","status":"`+status+`"}`))
		require.Len(t, n.got, 1, status)
		assert.Equal(t, "u-req", n.got[0].user, status)
		assert.Equal(t, wantKind, n.got[0].kind, status)
		assert.Equal(t, "r1", n.got[0].entityID, status)
	}

	c, n := consumer()
	c.handleRequestEvent(context.Background(), []byte(`{"request_id":"r1","requester_id":"u-req","status":"fulfilled"}`))
	assert.Empty(t, n.got, "a status nobody is told about creates nothing")
}

func TestAssignmentEvent_NotifiesTheRightPerson(t *testing.T) {
	c, n := consumer()
	c.handleAssignmentEvent(context.Background(), []byte(`{"asset_id":"a1","asset_name":"Laptop","action":"assign","to_user_id":"u-to","from_user_id":"u-from"}`))
	require.Len(t, n.got, 1)
	assert.Equal(t, "u-to", n.got[0].user, "an assignment tells the new holder")
	assert.Equal(t, "asset_assigned", n.got[0].kind)

	c, n = consumer()
	c.handleAssignmentEvent(context.Background(), []byte(`{"asset_id":"a1","asset_name":"Laptop","action":"return","to_user_id":"u-to","from_user_id":"u-from"}`))
	require.Len(t, n.got, 1)
	assert.Equal(t, "u-from", n.got[0].user, "a return tells the one who held it")
	assert.Equal(t, "asset_returned", n.got[0].kind)

	for _, body := range []string{
		`{"action":"assign"}`,                 // nobody to tell
		`{"action":"return"}`,                 // nobody to tell
		`{"action":"other","to_user_id":"x"}`, // an action nobody is told about
	} {
		c, n = consumer()
		c.handleAssignmentEvent(context.Background(), []byte(body))
		assert.Empty(t, n.got, body)
	}
}

func TestApprovalEvent_NotifiesTheRequesterOnlyWhenSomeoneElseDecided(t *testing.T) {
	c, n := consumer()
	c.handleApprovalEvent(context.Background(), []byte(`{"request_id":"r1","template_name":"Leave","action":"rejected","comment":"no budget","actor_node_id":"n-boss","created_by":"n-me"}`))
	require.Len(t, n.got, 1)
	assert.Equal(t, "n-me", n.got[0].user)
	assert.Equal(t, "approval_rejected", n.got[0].kind)
	assert.Contains(t, n.got[0].body, "no budget", "the reason travels with the rejection")

	c, n = consumer()
	c.handleApprovalEvent(context.Background(), []byte(`{"request_id":"r1","action":"approved","actor_node_id":"n-boss","created_by":"n-me"}`))
	require.Len(t, n.got, 1)
	assert.Equal(t, "approval_approved", n.got[0].kind)

	// The requester deciding their own request is not told about it.
	c, n = consumer()
	c.handleApprovalEvent(context.Background(), []byte(`{"request_id":"r1","action":"approved","actor_node_id":"n-me","created_by":"n-me"}`))
	assert.Empty(t, n.got)
	c, n = consumer()
	c.handleApprovalEvent(context.Background(), []byte(`{"request_id":"r1","action":"rejected","actor_node_id":"n-me","created_by":"n-me"}`))
	assert.Empty(t, n.got)
}

func TestEvents_MalformedOrUnstorableOnesAreSkippedNotFatal(t *testing.T) {
	c, n := consumer()
	for _, h := range []func(context.Context, []byte){c.handleLifecycleEvent, c.handleRequestEvent, c.handleAssignmentEvent, c.handleApprovalEvent} {
		h(context.Background(), []byte(`{not json`))
	}
	assert.Empty(t, n.got, "garbage creates no notification")

	c, n = consumer()
	n.err = errors.New("db down")
	c.handleLifecycleEvent(context.Background(), []byte(`{"asset_id":"a1","actor_id":"u"}`))
	assert.Len(t, n.got, 1, "the failure is logged and the handler returns")
}
