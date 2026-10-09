package grpc

import (
	"context"
	"os"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pb "ngac-platform/proto/policy"
	"ngac-platform/services/policy/internal/ngac"
)

type recordingShards struct {
	mu          sync.Mutex
	invalidated []string
	all         int
}

func (r *recordingShards) GetGraph(context.Context, string) (ngac.GraphReader, error) {
	return nil, os.ErrNotExist
}
func (r *recordingShards) InvalidateShard(ws string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.invalidated = append(r.invalidated, ws)
}
func (r *recordingShards) InvalidateAll()         { r.mu.Lock(); r.all++; r.mu.Unlock() }
func (r *recordingShards) Stats() ngac.ShardStats { return ngac.ShardStats{} }

func setupWriteTestStore(t *testing.T) (*ngac.Store, *pgxpool.Pool) {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		url = "postgres://ngac:ngac_secret@localhost:5432/ngac?sslmode=disable"
	}
	pool, err := pgxpool.New(context.Background(), url)
	require.NoError(t, err)
	if err := pool.Ping(context.Background()); err != nil {
		pool.Close()
		t.Skipf("test DB not available: %v", err)
	}
	t.Cleanup(pool.Close)
	s := ngac.NewStore(pool, ngac.NewGraph())
	require.NoError(t, s.LoadGraph(context.Background()))
	return s, pool
}

func setupWriteTestRedis(t *testing.T) *redis.Client {
	t.Helper()
	addr := os.Getenv("TEST_REDIS_ADDR")
	if addr == "" {
		addr = os.Getenv("REDIS_ADDR")
	}
	if addr == "" {
		t.Skip("TEST_REDIS_ADDR not set")
	}
	rdb := redis.NewClient(&redis.Options{Addr: addr, DB: 12})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		rdb.Close()
		t.Skipf("test redis not available: %v", err)
	}
	rdb.FlushDB(context.Background())
	t.Cleanup(func() { rdb.FlushDB(context.Background()); rdb.Close() })
	return rdb
}

// tenantFixture: PC(workspace) ← UA ← user, PC ← OA, UA → OA [read].
type tenantFixture struct {
	ws               string
	pc, ua, oa, user string
}

func newTenantFixture(t *testing.T, s *ngac.Store, pool *pgxpool.Pool) tenantFixture {
	t.Helper()
	ctx := context.Background()
	sfx := uuid.NewString()[:8]
	f := tenantFixture{ws: "ws-" + uuid.NewString()}

	mk := func(name, typ string, props map[string]string) string {
		n, err := s.CreateNode(ctx, name+"_"+sfx, typ, props)
		require.NoError(t, err)
		return n.ID
	}
	f.pc = mk("PC_del", ngac.NodeTypePolicyClass, map[string]string{"workspace_id": f.ws})
	f.ua = mk("UA_del", ngac.NodeTypeUserAttribute, nil)
	f.oa = mk("OA_del", ngac.NodeTypeObjectAttr, nil)
	f.user = mk("U_del", ngac.NodeTypeUser, nil)
	t.Cleanup(func() {
		pool.Exec(context.Background(), "DELETE FROM ngac_nodes WHERE id = ANY($1)",
			[]string{f.pc, f.ua, f.oa, f.user})
	})
	for _, a := range [][2]string{{f.ua, f.pc}, {f.oa, f.pc}, {f.user, f.ua}} {
		_, err := s.CreateAssignment(ctx, a[0], a[1])
		require.NoError(t, err)
	}
	_, err := s.CreateAssociation(ctx, f.ua, f.oa, []string{"read"})
	require.NoError(t, err)
	return f
}

// Deleting a node must invalidate what the node touched. Before the fix the
// shards were resolved AFTER the delete, when the node and its path to the
// workspace PC were gone: no shard was invalidated, and the deleted UA's users
// kept their cached ALLOWs.
func TestDeleteNode_ResolvesBeforeDeleteAndInvalidatesAfter(t *testing.T) {
	store, pool := setupWriteTestStore(t)
	f := newTenantFixture(t, store, pool)
	ctx := context.Background()

	shards := &recordingShards{}
	coord := ngac.NewInvalidationCoordinator(nil, nil, nil)
	var rdb *redis.Client
	if os.Getenv("TEST_REDIS_ADDR") != "" || os.Getenv("REDIS_ADDR") != "" {
		rdb = setupWriteTestRedis(t)
		coord = ngac.NewInvalidationCoordinator(nil, nil, ngac.NewCacheInvalidator(rdb, store.GetGraph))
	}
	ws := NewWriteServer(store, nil, coord, nil, nil, false)
	ws.SetShardManager(shards)

	userReq := ngac.AccessRequest{UserNodeID: f.user, ObjectNodeID: f.oa, Operation: "read", WorkspaceID: f.ws}
	otherReq := ngac.AccessRequest{UserNodeID: "someone-else", ObjectNodeID: "other-oa", Operation: "read", WorkspaceID: f.ws}
	if rdb != nil {
		cache := ngac.NewLayeredCache(rdb, nil, nil)
		cache.Set(ctx, userReq, &ngac.AccessDecision{Decision: ngac.DecisionAllow})
		cache.Set(ctx, otherReq, &ngac.AccessDecision{Decision: ngac.DecisionAllow})
	}

	_, err := ws.DeleteNode(ctx, &pb.DeleteNodeRequest{NodeId: f.ua})
	require.NoError(t, err)

	assert.Nil(t, store.GetNode(f.ua), "node deleted")
	assert.Contains(t, shards.invalidated, f.ws, "the deleted UA's workspace shard must be invalidated")
	assert.Equal(t, ngac.DecisionDeny, store.GetGraph().CheckAccess(f.user, f.oa, "read").Decision,
		"the former member is denied by the graph")

	if rdb == nil {
		t.Log("TEST_REDIS_ADDR not set: skipped the Redis half of this test")
		return
	}
	n, err := rdb.Exists(ctx, ngac.DecisionCacheKey(userReq)).Result()
	require.NoError(t, err)
	assert.Zero(t, n, "the deleted UA's user must lose the cached ALLOW")
	n, err = rdb.Exists(ctx, ngac.DecisionCacheKey(otherReq)).Result()
	require.NoError(t, err)
	assert.EqualValues(t, 1, n, "unrelated decisions survive a targeted invalidation")
}

// Deleting a policy class may change every decision inside it.
func TestDeleteNode_PolicyClass_InvalidatesEverything(t *testing.T) {
	store, pool := setupWriteTestStore(t)
	rdb := setupWriteTestRedis(t)
	f := newTenantFixture(t, store, pool)
	ctx := context.Background()

	shards := &recordingShards{}
	coord := ngac.NewInvalidationCoordinator(nil, nil, ngac.NewCacheInvalidator(rdb, store.GetGraph))
	ws := NewWriteServer(store, nil, coord, nil, nil, false)
	ws.SetShardManager(shards)

	other := ngac.AccessRequest{UserNodeID: "someone-else", ObjectNodeID: "other-oa", Operation: "read"}
	ngac.NewLayeredCache(rdb, nil, nil).Set(ctx, other, &ngac.AccessDecision{Decision: ngac.DecisionAllow})

	_, err := ws.DeleteNode(ctx, &pb.DeleteNodeRequest{NodeId: f.pc})
	require.NoError(t, err)

	assert.Contains(t, shards.invalidated, f.ws, "the PC's own workspace shard must be invalidated")
	n, err := rdb.Exists(ctx, ngac.DecisionCacheKey(other)).Result()
	require.NoError(t, err)
	assert.Zero(t, n, "a PC deletion flushes every cached decision")
}
