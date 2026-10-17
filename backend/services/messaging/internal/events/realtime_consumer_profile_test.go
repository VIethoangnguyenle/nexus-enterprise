package events

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/pkg/realtime"
	"ngac-platform/services/messaging/internal/domain"
)

type counters struct {
	mu   sync.Mutex
	vals map[string]int64
}

func (c *counters) Get(_ context.Context, keys ...string) ([]int64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]int64, len(keys))
	for i, k := range keys {
		out[i] = c.vals[k]
	}
	return out, nil
}

func (c *counters) Bump(_ context.Context, keys ...string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.vals == nil {
		c.vals = map[string]int64{}
	}
	for _, k := range keys {
		c.vals[k]++
	}
	return nil
}

func profileUpdated(tenant string) []byte {
	return eventBytes(realtime.Event{
		Domain: realtime.DomainWorkspace, Kind: realtime.KindProfileUpdated,
		TenantID: tenant, WorkspaceID: tenant, IDs: []string{"node-1"}, ActorUserID: "u1",
	})
}

// What the chat state shows about other people (a direct message's name and
// picture) must not outlive a profile change: the tag of everyone in the
// tenant moves, nobody else's does.
func TestProfileUpdated_MovesTheChatVersionOfTheWholeTenantOnly(t *testing.T) {
	v := &counters{}
	svc := domain.NewService(nil, nil, nil, nil, nil)
	svc.SetVersions(v)
	pub := &capture{}
	c := &RealtimeConsumer{pub: pub, now: time.Now}
	c.SetChatVersions(svc)

	version := func(tenant, user string) []int64 {
		got, err := v.Get(context.Background(), domain.ChatVersionKeys(tenant, user)...)
		require.NoError(t, err)
		return got
	}
	colleague, otherTenant := version("t1", "colleague"), version("t2", "colleague")

	c.handle("workspace.events", profileUpdated("t1"))

	assert.Greater(t, version("t1", "colleague")[0], colleague[0], "a colleague's tag changes")
	assert.Greater(t, version("t1", "someone-else")[0], colleague[0], "everyone in the tenant, not one person")
	assert.Equal(t, otherTenant, version("t2", "colleague"), "another tenant's tag is untouched")
	assert.Len(t, pub.got, 1, "open screens are told as well")
}

func TestProfileUpdated_WithoutATenantMovesNothing(t *testing.T) {
	v := &counters{}
	svc := domain.NewService(nil, nil, nil, nil, nil)
	svc.SetVersions(v)
	pub := &capture{}
	c := &RealtimeConsumer{pub: pub, now: time.Now}
	c.SetChatVersions(svc)

	c.handle("workspace.events", []byte(`{"domain":"workspace","kind":"profile_updated","workspace_id":"t1","ids":["n"]}`))

	assert.Empty(t, v.vals)
	assert.Empty(t, pub.got, "an event with no tenant has no boundary and is dropped")
}
