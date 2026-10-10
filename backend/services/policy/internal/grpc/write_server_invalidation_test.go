package grpc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "ngac-platform/proto/policy"
	"ngac-platform/services/policy/internal/ngac"
)

// rig is a write server on a real database and Redis with the full
// invalidation coordinator, so a test can watch what a graph change does to
// every cache layer.
type rig struct {
	ws     *WriteServer
	store  *ngac.Store
	pool   *pgxpool.Pool
	rdb    *redis.Client
	shards *recordingShards
	cache  ngac.DecisionCache
	f      tenantFixture
}

func newRig(t *testing.T) *rig {
	t.Helper()
	store, pool := setupWriteTestStore(t)
	rdb := setupWriteTestRedis(t)
	r := &rig{
		store: store, pool: pool, rdb: rdb,
		shards: &recordingShards{},
		cache:  ngac.NewLayeredCache(rdb),
		f:      newTenantFixture(t, store, pool),
	}
	coord := ngac.NewInvalidationCoordinator(ngac.NewCacheInvalidator(rdb, store.GetGraph))
	r.ws = NewWriteServer(store, nil, coord, ngac.NewOperationStore(pool), ngac.NewProhibitionStore(pool, store.GetGraph()), false)
	r.ws.SetShardManager(r.shards)
	return r
}

func (r *rig) allowed(t *testing.T, req ngac.AccessRequest) {
	t.Helper()
	r.cache.Set(context.Background(), req, &ngac.AccessDecision{Decision: ngac.DecisionAllow})
}

func (r *rig) cached(t *testing.T, req ngac.AccessRequest) bool {
	t.Helper()
	n, err := r.rdb.Exists(context.Background(), ngac.DecisionCacheKey(req)).Result()
	require.NoError(t, err)
	return n == 1
}

// decision asks the PDP on the live graph, prohibitions included.
func (r *rig) decision(user, object, op string) string {
	return ngac.NewDecisionEngine(r.store.GetGraph(), nil).
		Decide(context.Background(), ngac.AccessRequest{UserNodeID: user, ObjectNodeID: object, Operation: op}).Decision
}

func TestInvalidateCache_NeedsANodeID(t *testing.T) {
	store, _ := setupWriteTestStore(t)
	ws := NewWriteServer(store, nil, ngac.NewInvalidationCoordinator(nil), nil, nil, false)

	_, err := ws.InvalidateCache(context.Background(), &pb.InvalidateCacheRequest{})
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

// An external invalidation drops what the named nodes touched: the workspace's
// shard, the affected user's cached decisions —
// and nothing belonging to anyone else.
func TestInvalidateCache_DropsOnlyWhatTheNodesTouched(t *testing.T) {
	r := newRig(t)
	ctx := context.Background()
	mine := ngac.AccessRequest{UserNodeID: r.f.user, ObjectNodeID: r.f.oa, Operation: "read", WorkspaceID: r.f.ws}
	other := ngac.AccessRequest{UserNodeID: "someone-else", ObjectNodeID: "other-oa", Operation: "read", WorkspaceID: r.f.ws}
	r.allowed(t, mine)
	r.allowed(t, other)

	_, err := r.ws.InvalidateCache(ctx, &pb.InvalidateCacheRequest{NodeIds: []string{r.f.user}, Reason: "test"})
	require.NoError(t, err)

	assert.False(t, r.cached(t, mine), "the user's cached ALLOW is gone")
	assert.True(t, r.cached(t, other), "another user's decision survives")
	assert.Contains(t, r.shards.invalidated, r.f.ws)
}

// Reloading the graph is the one full flush: every cached decision, every
// shard.
func TestLoadGraph_FlushesEveryLayer(t *testing.T) {
	r := newRig(t)
	ctx := context.Background()
	a := ngac.AccessRequest{UserNodeID: r.f.user, ObjectNodeID: r.f.oa, Operation: "read", WorkspaceID: r.f.ws}
	b := ngac.AccessRequest{UserNodeID: "someone-else", ObjectNodeID: "other-oa", Operation: "read"}
	r.allowed(t, a)
	r.allowed(t, b)

	_, err := r.ws.LoadGraph(ctx, &pb.Empty{})
	require.NoError(t, err)

	assert.False(t, r.cached(t, a))
	assert.False(t, r.cached(t, b))
	assert.Equal(t, 1, r.shards.all, "shards are dropped wholesale")
}

// A prohibition is a deny override. Creating one must turn an ALLOW into a DENY
// for the subject's users at once — in the graph and in every cache — and
// removing it must give the access back the same way.
func TestProhibition_CreateDeniesAndRemoveRestoresWithInvalidation(t *testing.T) {
	r := newRig(t)
	ctx := context.Background()
	name := "deny-" + uuid.NewString()[:8]
	t.Cleanup(func() { r.pool.Exec(ctx, "DELETE FROM ngac_prohibitions WHERE name = $1", name) })

	mine := ngac.AccessRequest{UserNodeID: r.f.user, ObjectNodeID: r.f.oa, Operation: "read", WorkspaceID: r.f.ws}
	other := ngac.AccessRequest{UserNodeID: "someone-else", ObjectNodeID: "other-oa", Operation: "read", WorkspaceID: r.f.ws}
	require.Equal(t, ngac.DecisionAllow, r.decision(r.f.user, r.f.oa, "read"))
	r.allowed(t, mine)
	r.allowed(t, other)

	created, err := r.ws.CreateProhibition(ctx, &pb.CreateProhibitionRequest{
		Name: name, SubjectId: r.f.ua, Operations: []string{"read"}, TargetOaIds: []string{r.f.oa},
	})
	require.NoError(t, err)
	assert.Equal(t, name, created.Name)
	assert.Equal(t, ngac.DecisionDeny, r.decision(r.f.user, r.f.oa, "read"),
		"the prohibition denies a user the association allowed")
	assert.False(t, r.cached(t, mine), "the subject's user loses the cached ALLOW")
	assert.True(t, r.cached(t, other), "unrelated decisions survive")
	assert.Contains(t, r.shards.invalidated, r.f.ws)

	r.shards.invalidated = nil
	r.allowed(t, mine)
	_, err = r.ws.RemoveProhibition(ctx, &pb.RemoveProhibitionRequest{Name: name})
	require.NoError(t, err)
	assert.Equal(t, ngac.DecisionAllow, r.decision(r.f.user, r.f.oa, "read"))
	assert.False(t, r.cached(t, mine), "removing it invalidates too, or a stale deny would linger")
	assert.Contains(t, r.shards.invalidated, r.f.ws)
}

func TestRemoveProhibition_UnknownNameIsNotFoundAndTouchesNothing(t *testing.T) {
	r := newRig(t)
	mine := ngac.AccessRequest{UserNodeID: r.f.user, ObjectNodeID: r.f.oa, Operation: "read", WorkspaceID: r.f.ws}
	r.allowed(t, mine)

	_, err := r.ws.RemoveProhibition(context.Background(), &pb.RemoveProhibitionRequest{Name: "no-such-" + uuid.NewString()})
	assert.Equal(t, codes.NotFound, status.Code(err))
	assert.True(t, r.cached(t, mine), "a refused removal invalidates nothing")
	assert.Empty(t, r.shards.invalidated)
}

func TestCreateProhibition_RefusedInputChangesNothing(t *testing.T) {
	r := newRig(t)
	mine := ngac.AccessRequest{UserNodeID: r.f.user, ObjectNodeID: r.f.oa, Operation: "read", WorkspaceID: r.f.ws}
	r.allowed(t, mine)

	for name, req := range map[string]*pb.CreateProhibitionRequest{
		"no name":       {SubjectId: r.f.ua, Operations: []string{"read"}, TargetOaIds: []string{r.f.oa}},
		"no operations": {Name: "p-" + uuid.NewString()[:8], SubjectId: r.f.ua, TargetOaIds: []string{r.f.oa}},
		"no targets":    {Name: "p-" + uuid.NewString()[:8], SubjectId: r.f.ua, Operations: []string{"read"}},
	} {
		_, err := r.ws.CreateProhibition(context.Background(), req)
		require.Error(t, err, name)
	}
	assert.Equal(t, ngac.DecisionAllow, r.decision(r.f.user, r.f.oa, "read"))
	assert.True(t, r.cached(t, mine))
	assert.Empty(t, r.shards.invalidated)
}

func TestRegisterOperations_IsIdempotentAndReportsBoth(t *testing.T) {
	r := newRig(t)
	ctx := context.Background()
	op := "op_" + uuid.NewString()[:8]
	t.Cleanup(func() { r.pool.Exec(ctx, "DELETE FROM ngac_operations WHERE name = $1", op) })

	first, err := r.ws.RegisterOperations(ctx, &pb.RegisterOperationsRequest{Operations: []string{op, ""}})
	require.NoError(t, err)
	assert.Equal(t, []string{op}, first.Registered)
	assert.Empty(t, first.AlreadyExists, "an empty name is skipped, not reported")

	again, err := r.ws.RegisterOperations(ctx, &pb.RegisterOperationsRequest{Operations: []string{op}})
	require.NoError(t, err)
	assert.Empty(t, again.Registered)
	assert.Equal(t, []string{op}, again.AlreadyExists)
}

// Without the stores the optional RPCs refuse; they never pretend to succeed.
func TestOptionalStores_AbsentMeansUnimplemented(t *testing.T) {
	store, _ := setupWriteTestStore(t)
	ws := NewWriteServer(store, nil, ngac.NewInvalidationCoordinator(nil), nil, nil, false)
	ctx := context.Background()

	_, err := ws.RegisterOperations(ctx, &pb.RegisterOperationsRequest{Operations: []string{"x"}})
	assert.Equal(t, codes.Unimplemented, status.Code(err))
	_, err = ws.CreateProhibition(ctx, &pb.CreateProhibitionRequest{Name: "n"})
	assert.Equal(t, codes.Unimplemented, status.Code(err))
	_, err = ws.RemoveProhibition(ctx, &pb.RemoveProhibitionRequest{Name: "n"})
	assert.Equal(t, codes.Unimplemented, status.Code(err))
}
