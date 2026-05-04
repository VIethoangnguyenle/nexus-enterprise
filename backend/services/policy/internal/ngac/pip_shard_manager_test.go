package ngac

import (
	"container/list"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Unit tests for ShardManager LRU logic ---
// These test the in-memory cache mechanics without a database.

// newTestShardManager creates a shardManager with initialized LRU structures.
func newTestShardManager(maxShards int) *shardManager {
	return &shardManager{
		shards:    make(map[string]*shardEntry),
		maxShards: maxShards,
		lruList:   list.New(),
		lruIndex:  make(map[string]*list.Element, maxShards),
	}
}

// addTestShard inserts a shard into the manager, mimicking GetGraph cache insert.
func (sm *shardManager) addTestShard(key string) {
	sm.shards[key] = &shardEntry{graph: NewGraph()}
	sm.lruIndex[key] = sm.lruList.PushBack(&lruKey{workspaceID: key})
}

// lruOrder returns the current LRU access order as a string slice (front=oldest).
func (sm *shardManager) lruOrder() []string {
	var order []string
	for e := sm.lruList.Front(); e != nil; e = e.Next() {
		order = append(order, e.Value.(*lruKey).workspaceID)
	}
	return order
}

func TestShardManager_Stats_Empty(t *testing.T) {
	sm := NewShardManager(nil, ShardManagerConfig{MaxShards: 10})
	stats := sm.Stats()
	assert.Equal(t, 0, stats.ActiveShards)
	assert.Equal(t, 10, stats.MaxShards)
	assert.Equal(t, int64(0), stats.Hits)
	assert.Equal(t, int64(0), stats.Misses)
}

func TestShardManager_DefaultMaxShards(t *testing.T) {
	sm := NewShardManager(nil, ShardManagerConfig{})
	stats := sm.Stats()
	assert.Equal(t, 1000, stats.MaxShards, "default max_shards should be 1000")
}

func TestShardManager_InvalidateShard_NonExistent(t *testing.T) {
	sm := NewShardManager(nil, ShardManagerConfig{MaxShards: 10})
	// Should not panic
	sm.InvalidateShard("nonexistent")
	assert.Equal(t, 0, sm.Stats().ActiveShards)
}

func TestShardManager_InvalidateAll(t *testing.T) {
	sm := newTestShardManager(10)
	sm.addTestShard("ws-1")
	sm.addTestShard("ws-2")

	sm.InvalidateAll()
	assert.Equal(t, 0, sm.Stats().ActiveShards)
	assert.Equal(t, 0, sm.lruList.Len())
	assert.Empty(t, sm.lruIndex)
}

func TestShardManager_LRU_EvictOldest(t *testing.T) {
	sm := newTestShardManager(3)
	sm.addTestShard("ws-1")
	sm.addTestShard("ws-2")
	sm.addTestShard("ws-3")

	require.Equal(t, 3, len(sm.shards))

	// Evict oldest → ws-1 should go
	sm.evictLRU()

	assert.Equal(t, 2, len(sm.shards))
	_, found := sm.shards["ws-1"]
	assert.False(t, found, "ws-1 should be evicted (oldest)")
	_, found = sm.shards["ws-2"]
	assert.True(t, found, "ws-2 should remain")
	_, found = sm.shards["ws-3"]
	assert.True(t, found, "ws-3 should remain")
	assert.Equal(t, int64(1), sm.evictions)
}

func TestShardManager_LRU_PromoteMoves(t *testing.T) {
	sm := newTestShardManager(3)
	sm.addTestShard("ws-1")
	sm.addTestShard("ws-2")
	sm.addTestShard("ws-3")

	// Promote ws-1 → should move to end
	sm.promoteInAccessOrder("ws-1")
	assert.Equal(t, []string{"ws-2", "ws-3", "ws-1"}, sm.lruOrder())

	// Evict → ws-2 should go (now oldest)
	sm.evictLRU()
	_, found := sm.shards["ws-2"]
	assert.False(t, found, "ws-2 should be evicted after ws-1 was promoted")
}

func TestShardManager_InvalidateShard_RemovesFromLRU(t *testing.T) {
	sm := newTestShardManager(10)
	sm.addTestShard("ws-1")
	sm.addTestShard("ws-2")

	sm.InvalidateShard("ws-1")

	assert.Equal(t, 1, len(sm.shards))
	assert.Equal(t, []string{"ws-2"}, sm.lruOrder())
	assert.Equal(t, 1, sm.lruList.Len())
	_, indexFound := sm.lruIndex["ws-1"]
	assert.False(t, indexFound, "ws-1 should be removed from LRU index")
}

func TestShardManager_LRU_O1_Complexity(t *testing.T) {
	// Verify that promote/evict don't break with many entries
	sm := newTestShardManager(1000)
	for i := 0; i < 500; i++ {
		sm.addTestShard(fmt.Sprintf("ws-%d", i))
	}

	// Promote the first entry — should move to back
	sm.promoteInAccessOrder("ws-0")
	order := sm.lruOrder()
	assert.Equal(t, "ws-0", order[len(order)-1], "promoted entry should be at back")
	assert.Equal(t, "ws-1", order[0], "ws-1 should now be oldest")

	// Evict oldest — should be ws-1 now
	sm.evictLRU()
	_, found := sm.shards["ws-1"]
	assert.False(t, found, "ws-1 should be evicted")
	assert.Equal(t, 499, len(sm.shards))
}

func TestShardManager_ConcurrentAccess(t *testing.T) {
	sm := newTestShardManager(100)

	// Pre-populate
	for i := 0; i < 50; i++ {
		sm.addTestShard(fmt.Sprintf("ws-%d", i))
	}

	var wg sync.WaitGroup

	// Concurrent reads
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			_ = sm.Stats()
		}(i)
	}

	// Concurrent invalidations
	for i := 0; i < 25; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			sm.InvalidateShard(fmt.Sprintf("ws-%d", n))
		}(i)
	}

	wg.Wait()

	stats := sm.Stats()
	assert.LessOrEqual(t, stats.ActiveShards, 50, "should have ≤ 50 after invalidations")
}
