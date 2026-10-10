package events

import (
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/pkg/realtime"
)

func TestTranslate_DomainTopics(t *testing.T) {
	data, _ := json.Marshal(realtime.Event{
		Domain: realtime.DomainDrive, Kind: realtime.KindMoved, TenantID: "t", WorkspaceID: "w",
		IDs: []string{"f1"}, ParentID: "new", OldParentID: "old",
	})
	e, err := Translate("drive.events", data)
	require.NoError(t, err)
	assert.Equal(t, "old", e.OldParentID)
	assert.Equal(t, "new", e.ParentID)

	_, err = Translate("drive.events", []byte(`{"domain":"drive","kind":"moved","workspace_id":"w"}`))
	assert.Error(t, err, "an event without a tenant is rejected")
	_, err = Translate("something.else", data)
	assert.Error(t, err)
}

func TestTranslate_AssetTopicsBecomeWorkspaceEvents(t *testing.T) {
	cases := []struct {
		topic, body, kind, id string
	}{
		{"asset.lifecycle", `{"asset_id":"a1","actor_id":"u1","workspace_id":"w","tenant_id":"t","timestamp":5}`, realtime.KindUpdated, "a1"},
		{"asset.request", `{"request_id":"r1","requester_id":"u1","workspace_id":"w","tenant_id":"t"}`, realtime.KindRequestChanged, "r1"},
		{"asset.assignment", `{"asset_id":"a2","actor_id":"u1","workspace_id":"w","tenant_id":"t"}`, realtime.KindAssigned, "a2"},
	}
	for _, c := range cases {
		e, err := Translate(c.topic, []byte(c.body))
		require.NoError(t, err, c.topic)
		assert.Equal(t, realtime.DomainAsset, e.Domain)
		assert.Equal(t, c.kind, e.Kind)
		assert.Equal(t, []string{c.id}, e.IDs)
		assert.Equal(t, realtime.LevelWorkspace, e.Level(), "assets are workspace-wide, not broadcast")
		assert.Equal(t, "w", e.WorkspaceID)
		assert.Equal(t, "t", e.TenantID)
	}
	// Producers that predate the tenant field cannot be routed.
	_, err := Translate("asset.lifecycle", []byte(`{"asset_id":"a1","workspace_id":"w"}`))
	assert.Error(t, err, "an asset event without a tenant is dropped, not broadcast")
}

type capture struct {
	mu  sync.Mutex
	got []realtime.Event
}

func (c *capture) PublishDomain(e realtime.Event) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.got = append(c.got, e)
}

func TestHandle_DropsStaleAndMalformed(t *testing.T) {
	cap := &capture{}
	now := time.UnixMilli(1_000_000_000)
	c := &RealtimeConsumer{pub: cap, now: func() time.Time { return now }}
	mk := func(at int64) []byte {
		b, _ := json.Marshal(realtime.Event{Domain: "drive", Kind: "created", TenantID: "t", WorkspaceID: "w", IDs: []string{"x"}, At: at})
		return b
	}

	c.handle("drive.events", mk(now.UnixMilli()-1000))
	c.handle("drive.events", mk(now.UnixMilli()-int64(maxEventAge/time.Millisecond)-1))
	c.handle("drive.events", []byte("not json"))
	c.handle("drive.events", mk(0))

	require.Len(t, cap.got, 2, "fresh and undated events pass; stale and malformed do not")
}

type revoking struct {
	capture
	revoked [][2]string
}

func (r *revoking) RevokeWorkspaceSubscriptions(ws, node string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.revoked = append(r.revoked, [2]string{ws, node})
}

func eventBytes(e realtime.Event) []byte {
	b, _ := json.Marshal(e)
	return b
}

// A member who left or was removed stops following the workspace before anyone
// is told they left.
func TestHandle_MemberRemovedRevokesTheirSubscriptions(t *testing.T) {
	rev := &revoking{}
	c := &RealtimeConsumer{pub: rev, now: time.Now}

	c.handle("workspace.events", eventBytes(realtime.Event{
		Domain: realtime.DomainWorkspace, Kind: realtime.KindMemberRemoved,
		TenantID: "w", WorkspaceID: "w", IDs: []string{"node-1"},
	}))
	c.handle("workspace.events", eventBytes(realtime.Event{
		Domain: realtime.DomainWorkspace, Kind: realtime.KindRoleChanged,
		TenantID: "w", WorkspaceID: "w", IDs: []string{"node-2"},
	}))

	assert.Equal(t, [][2]string{{"w", "node-1"}}, rev.revoked, "only a removal revokes, and only the person named")
	assert.Len(t, rev.got, 2, "both are still announced")
}

// Policy replicas apply a mutation from the graph topic at their own pace. A
// permission event is held for a short settle time so the refetch it triggers
// does not read the graph from before the change.
func TestHandle_PermissionEventsWaitForReplicasToSettle(t *testing.T) {
	cap := &capture{}
	c := &RealtimeConsumer{pub: cap, now: time.Now, settle: 150 * time.Millisecond}
	perm := realtime.Event{
		Domain: realtime.DomainPermission, Kind: realtime.KindChanged,
		TenantID: "w", WorkspaceID: "w", UserNodeIDs: []string{"n"},
	}
	drive := realtime.Event{Domain: realtime.DomainDrive, Kind: realtime.KindCreated, TenantID: "w", WorkspaceID: "w", IDs: []string{"f"}}

	c.handle("permission.events", eventBytes(perm))
	c.handle("drive.events", eventBytes(drive))

	cap.mu.Lock()
	require.Len(t, cap.got, 1, "other domains are not delayed")
	assert.Equal(t, realtime.DomainDrive, cap.got[0].Domain)
	cap.mu.Unlock()

	require.Eventually(t, func() bool {
		cap.mu.Lock()
		defer cap.mu.Unlock()
		return len(cap.got) == 2
	}, 2*time.Second, 20*time.Millisecond, "the permission event follows once the settle time has passed")
}
