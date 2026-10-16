package events

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/pkg/realtime"
)

type bumps struct {
	mu      sync.Mutex
	tenants []string
}

func (b *bumps) BumpChat(tenantID string, _ ...string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.tenants = append(b.tenants, tenantID)
}

func (b *bumps) seen() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]string(nil), b.tenants...)
}

// Changes other services announce reach the chat state without passing through
// a messaging request, so the consumer moves the tenant's counter for them.
func TestHandle_ChatRelevantEventsMoveTheTenantVersion(t *testing.T) {
	b := &bumps{}
	c := &RealtimeConsumer{pub: &capture{}, now: time.Now}
	c.SetChatVersions(b)

	c.handle("workspace.events", eventBytes(realtime.Event{
		Domain: realtime.DomainWorkspace, Kind: realtime.KindMemberRemoved, TenantID: "t1", WorkspaceID: "t1", IDs: []string{"n"},
	}))
	c.handle("channel.events", eventBytes(realtime.Event{
		Domain: realtime.DomainChannel, Kind: realtime.KindCreated, TenantID: "t2", WorkspaceID: "t2", IDs: []string{"c"},
	}))
	assert.Equal(t, []string{"t1", "t2"}, b.seen())

	// Files and documents cannot change a conversations list.
	c.handle("drive.events", eventBytes(realtime.Event{
		Domain: realtime.DomainDrive, Kind: realtime.KindCreated, TenantID: "t1", WorkspaceID: "t1", IDs: []string{"f"},
	}))
	assert.Len(t, b.seen(), 2)

	// A malformed or tenant-less event moves nothing.
	c.handle("workspace.events", []byte(`{"domain":"workspace","kind":"member_removed","workspace_id":"w"}`))
	assert.Len(t, b.seen(), 2)
}

// The counter moves when clients are told, after the policy replicas settled: a
// refetch answered earlier would be kept under the new version.
func TestHandle_PermissionEventMovesTheVersionOnlyOnceSettled(t *testing.T) {
	b := &bumps{}
	c := &RealtimeConsumer{pub: &capture{}, now: time.Now, settle: 150 * time.Millisecond}
	c.SetChatVersions(b)

	c.handle("permission.events", eventBytes(realtime.Event{
		Domain: realtime.DomainPermission, Kind: realtime.KindChanged, TenantID: "t1", WorkspaceID: "t1", UserNodeIDs: []string{"n"},
	}))
	assert.Empty(t, b.seen(), "not before the replicas have had their time")

	require.Eventually(t, func() bool { return len(b.seen()) == 1 }, 2*time.Second, 10*time.Millisecond)
	assert.Equal(t, []string{"t1"}, b.seen())
}

func TestHandle_NoVersionsWiredIsHarmless(t *testing.T) {
	c := &RealtimeConsumer{pub: &capture{}, now: time.Now}
	c.handle("workspace.events", eventBytes(realtime.Event{
		Domain: realtime.DomainWorkspace, Kind: realtime.KindMemberRemoved, TenantID: "t1", WorkspaceID: "t1", IDs: []string{"n"},
	}))
}
