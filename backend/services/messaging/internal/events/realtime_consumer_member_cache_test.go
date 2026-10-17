package events

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/pkg/realtime"
)

type forgetLog struct {
	mu  sync.Mutex
	got [][2]string
	err error
}

func (f *forgetLog) ForgetWorkspaceMember(_ context.Context, workspaceID, userNodeID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.got = append(f.got, [2]string{workspaceID, userNodeID})
	return f.err
}

func memberCacheEventBytes(t *testing.T, e realtime.Event) []byte {
	t.Helper()
	e.At = time.Now().UnixMilli()
	b, err := json.Marshal(e)
	require.NoError(t, err)
	return b
}

// Someone removed from a workspace leaves every channel of it in the graph, so
// the member cache (member_count, push recipients) must drop them too. Only a
// workspace member_removed event does that: the same person leaving one channel,
// or another change in the workspace, leaves the cache alone.
func TestHandle_WorkspaceMemberRemovedForgetsTheirChannelMemberships(t *testing.T) {
	cap := &capture{}
	cache := &forgetLog{}
	c := &RealtimeConsumer{pub: cap, now: time.Now}
	c.SetMemberCache(cache)

	c.handle("workspace.events", memberCacheEventBytes(t, realtime.Event{
		Domain: realtime.DomainWorkspace, Kind: realtime.KindMemberAdded, TenantID: "w", WorkspaceID: "w", IDs: []string{"u-added"},
	}))
	c.handle("channel.events", memberCacheEventBytes(t, realtime.Event{
		Domain: realtime.DomainChannel, Kind: realtime.KindMemberRemoved, TenantID: "w", WorkspaceID: "w", ChannelID: "ch", IDs: []string{"u-left-channel"},
	}))
	assert.Empty(t, cache.got, "neither a join nor a channel departure is a workspace departure")

	c.handle("workspace.events", memberCacheEventBytes(t, realtime.Event{
		Domain: realtime.DomainWorkspace, Kind: realtime.KindMemberRemoved, TenantID: "w", WorkspaceID: "w", IDs: []string{"u1", "u2"},
	}))
	assert.Equal(t, [][2]string{{"w", "u1"}, {"w", "u2"}}, cache.got)
	assert.Len(t, cap.got, 3, "the event is still published")
}

// A cache that cannot be cleaned must not stop the event: clients still have to
// hear that the person left.
func TestHandle_MemberCacheFailureStillPublishes(t *testing.T) {
	cap := &capture{}
	c := &RealtimeConsumer{pub: cap, now: time.Now}
	c.SetMemberCache(&forgetLog{err: assert.AnError})

	c.handle("workspace.events", memberCacheEventBytes(t, realtime.Event{
		Domain: realtime.DomainWorkspace, Kind: realtime.KindMemberRemoved, TenantID: "w", WorkspaceID: "w", IDs: []string{"u1"},
	}))
	require.Len(t, cap.got, 1)
}
