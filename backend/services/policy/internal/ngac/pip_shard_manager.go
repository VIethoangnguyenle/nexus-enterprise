package ngac

import (
	"container/list"
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ShardManager manages per-tenant in-memory graph shards with LRU eviction.
// Each shard contains the graph data for a single tenant workspace.
//
// Architecture:
//   - Key: workspace_id (maps to tenant PC)
//   - Value: *Graph loaded via recursive CTE from database
//   - Eviction: LRU with configurable max_shards (default 1000)
//   - Thread-safe: all operations are mutex-protected
type ShardManager interface {
	// GetGraph returns the in-memory graph for a workspace.
	// Loads from database on cache miss (lazy loading).
	GetGraph(ctx context.Context, workspaceID string) (GraphReader, error)

	// InvalidateShard removes a workspace's shard from the cache.
	// Next GetGraph call will reload from database.
	InvalidateShard(workspaceID string)

	// InvalidateAll removes all shards from the cache.
	InvalidateAll()

	// Stats returns current shard cache statistics.
	Stats() ShardStats
}

// ShardStats provides observability into the shard cache.
type ShardStats struct {
	ActiveShards int
	MaxShards    int
	Hits         int64
	Misses       int64
	Evictions    int64
	LoadErrors   int64
}

// shardEntry wraps a loaded graph with LRU metadata.
type shardEntry struct {
	graph      *Graph
	loadedAt   time.Time
	lastAccess time.Time
	nodeCount  int
}

// shardManager is the LRU-based implementation of ShardManager.
type shardManager struct {
	mu        sync.RWMutex
	db        *pgxpool.Pool
	shards    map[string]*shardEntry
	maxShards int

	// O(1) LRU via doubly-linked list + element index map
	lruList  *list.List
	lruIndex map[string]*list.Element // workspaceID → list element

	// Metrics
	hits       int64
	misses     int64
	evictions  int64
	loadErrors int64
}

// lruKey is the value stored in each list element for O(1) reverse lookup.
type lruKey struct {
	workspaceID string
}

// ShardManagerConfig configures the shard manager.
type ShardManagerConfig struct {
	MaxShards int // Maximum number of shards to keep in memory (default 1000)
}

// NewShardManager creates an LRU-based shard manager.
func NewShardManager(db *pgxpool.Pool, cfg ShardManagerConfig) ShardManager {
	if cfg.MaxShards <= 0 {
		cfg.MaxShards = 1000
	}
	return &shardManager{
		db:        db,
		shards:    make(map[string]*shardEntry, cfg.MaxShards),
		maxShards: cfg.MaxShards,
		lruList:   list.New(),
		lruIndex:  make(map[string]*list.Element, cfg.MaxShards),
	}
}

// GetGraph returns the graph for a workspace, loading it on cache miss.
func (sm *shardManager) GetGraph(ctx context.Context, workspaceID string) (GraphReader, error) {
	// Fast path: read lock check
	sm.mu.RLock()
	entry, ok := sm.shards[workspaceID]
	sm.mu.RUnlock()

	if ok {
		sm.mu.Lock()
		sm.hits++
		entry.lastAccess = time.Now()
		sm.promoteInAccessOrder(workspaceID)
		sm.mu.Unlock()
		return entry.graph, nil
	}

	// Slow path: load shard
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Double-check after acquiring write lock
	if entry, ok := sm.shards[workspaceID]; ok {
		sm.hits++
		entry.lastAccess = time.Now()
		sm.promoteInAccessOrder(workspaceID)
		return entry.graph, nil
	}

	sm.misses++

	graph, err := sm.loadShard(ctx, workspaceID)
	if err != nil {
		sm.loadErrors++
		return nil, err
	}

	// Evict if at capacity
	if len(sm.shards) >= sm.maxShards {
		sm.evictLRU()
	}

	entry = &shardEntry{
		graph:      graph,
		loadedAt:   time.Now(),
		lastAccess: time.Now(),
		nodeCount:  len(graph.Nodes),
	}
	sm.shards[workspaceID] = entry
	sm.lruIndex[workspaceID] = sm.lruList.PushBack(&lruKey{workspaceID: workspaceID})

	slog.Info("shard loaded",
		"workspace_id", workspaceID,
		"nodes", entry.nodeCount,
		"active_shards", len(sm.shards),
	)

	return graph, nil
}

// InvalidateShard removes a workspace's shard from the cache.
func (sm *shardManager) InvalidateShard(workspaceID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if _, ok := sm.shards[workspaceID]; ok {
		delete(sm.shards, workspaceID)
		sm.removeFromAccessOrder(workspaceID)
		slog.Info("shard invalidated", "workspace_id", workspaceID)
	}
}

// InvalidateAll removes all shards from the cache.
func (sm *shardManager) InvalidateAll() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.shards = make(map[string]*shardEntry, sm.maxShards)
	sm.lruList.Init()
	sm.lruIndex = make(map[string]*list.Element, sm.maxShards)
	slog.Info("all shards invalidated")
}

// Stats returns current shard cache statistics.
func (sm *shardManager) Stats() ShardStats {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	return ShardStats{
		ActiveShards: len(sm.shards),
		MaxShards:    sm.maxShards,
		Hits:         sm.hits,
		Misses:       sm.misses,
		Evictions:    sm.evictions,
		LoadErrors:   sm.loadErrors,
	}
}

// loadShard loads a tenant's graph from the database using recursive CTE.
// Traces all nodes reachable from the tenant's PC(s) + PC_Global.
// Must be called with sm.mu held.
func (sm *shardManager) loadShard(ctx context.Context, workspaceID string) (*Graph, error) {
	start := time.Now()

	// Step 1: Find tenant PC node by workspace_id property
	var tenantPCID string
	err := sm.db.QueryRow(ctx,
		`SELECT id FROM ngac_nodes
		 WHERE node_type = 'PC' AND properties->>'workspace_id' = $1
		 LIMIT 1`,
		workspaceID,
	).Scan(&tenantPCID)
	if err != nil {
		return nil, fmt.Errorf("finding tenant PC for workspace %s: %w", workspaceID, err)
	}

	// Step 2: Find PC_Global (always included in every shard)
	var globalPCID string
	_ = sm.db.QueryRow(ctx,
		`SELECT id FROM ngac_nodes
		 WHERE node_type = 'PC' AND properties->>'scope' = 'global'
		 LIMIT 1`,
	).Scan(&globalPCID) // OK if not found — some deployments may not have PC_Global

	// Step 3: Recursive CTE to collect all nodes reachable from tenant PC(s)
	// Traces DOWN from PCs through assignments to find all UA/OA/U nodes.
	pcIDs := []string{tenantPCID}
	if globalPCID != "" {
		pcIDs = append(pcIDs, globalPCID)
	}

	graph := NewGraph()

	// Load PC nodes first
	for _, pcID := range pcIDs {
		var n NGACNode
		var props map[string]string
		err := sm.db.QueryRow(ctx,
			`SELECT id, name, node_type, properties FROM ngac_nodes WHERE id = $1`,
			pcID,
		).Scan(&n.ID, &n.Name, &n.NodeType, &props)
		if err == nil {
			n.Properties = props
			graph.AddNode(&n)
		}
	}

	// Step 4: Load all descendant nodes from each PC via recursive CTE
	rows, err := sm.db.Query(ctx,
		`WITH RECURSIVE descendants AS (
			-- Base: direct children of our PCs
			SELECT a.child_id AS node_id
			FROM ngac_assignments a
			WHERE a.parent_id = ANY($1)
			UNION
			-- Recurse: children of children
			SELECT a.child_id
			FROM ngac_assignments a
			JOIN descendants d ON a.parent_id = d.node_id
		)
		SELECT n.id, n.name, n.node_type, n.properties
		FROM ngac_nodes n
		JOIN descendants d ON n.id = d.node_id
		WHERE n.node_type IN ('U', 'UA', 'OA')`,
		pcIDs,
	)
	if err != nil {
		return nil, fmt.Errorf("loading shard descendants: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var n NGACNode
		var props map[string]string
		if err := rows.Scan(&n.ID, &n.Name, &n.NodeType, &props); err != nil {
			return nil, fmt.Errorf("scanning shard node: %w", err)
		}
		n.Properties = props
		graph.AddNode(&n)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating shard nodes: %w", err)
	}

	// Step 5: Load assignments between nodes in the shard
	nodeIDs := make([]string, 0, len(graph.Nodes))
	for id := range graph.Nodes {
		nodeIDs = append(nodeIDs, id)
	}

	rows, err = sm.db.Query(ctx,
		`SELECT id, child_id, parent_id FROM ngac_assignments
		 WHERE child_id = ANY($1) AND parent_id = ANY($1)`,
		nodeIDs,
	)
	if err != nil {
		return nil, fmt.Errorf("loading shard assignments: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var a Assignment
		if err := rows.Scan(&a.ID, &a.ChildID, &a.ParentID); err != nil {
			return nil, fmt.Errorf("scanning shard assignment: %w", err)
		}
		if err := graph.AddAssignment(&a); err != nil {
			slog.Debug("skipping shard assignment", "id", a.ID, "error", err)
		}
	}

	// Step 6: Load associations where UA is in the shard
	rows, err = sm.db.Query(ctx,
		`SELECT id, ua_id, oa_id, operations FROM ngac_associations
		 WHERE ua_id = ANY($1)`,
		nodeIDs,
	)
	if err != nil {
		return nil, fmt.Errorf("loading shard associations: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var a Association
		if err := rows.Scan(&a.ID, &a.UAID, &a.OAID, &a.Operations); err != nil {
			return nil, fmt.Errorf("scanning shard association: %w", err)
		}
		if err := graph.AddAssociation(&a); err != nil {
			slog.Debug("skipping shard association", "id", a.ID, "error", err)
		}
	}

	slog.Info("shard load complete",
		"workspace_id", workspaceID,
		"tenant_pc", tenantPCID,
		"nodes", len(graph.Nodes),
		"duration_ms", time.Since(start).Milliseconds(),
	)

	return graph, nil
}

// --- LRU helpers (must be called with sm.mu held) ---

// promoteInAccessOrder moves a key to the back of the LRU list (most recent). O(1).
func (sm *shardManager) promoteInAccessOrder(key string) {
	if elem, ok := sm.lruIndex[key]; ok {
		sm.lruList.MoveToBack(elem)
	}
}

// removeFromAccessOrder removes a key from the LRU list. O(1).
func (sm *shardManager) removeFromAccessOrder(key string) {
	if elem, ok := sm.lruIndex[key]; ok {
		sm.lruList.Remove(elem)
		delete(sm.lruIndex, key)
	}
}

// evictLRU removes the least recently used shard. O(1).
func (sm *shardManager) evictLRU() {
	front := sm.lruList.Front()
	if front == nil {
		return
	}
	entry := front.Value.(*lruKey)
	sm.lruList.Remove(front)
	delete(sm.lruIndex, entry.workspaceID)
	delete(sm.shards, entry.workspaceID)
	sm.evictions++

	slog.Info("shard evicted (LRU)", "workspace_id", entry.workspaceID, "active_shards", len(sm.shards))
}
