package ngac_test

import (
	"context"
	"os"
	"slices"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/redis/go-redis/v9"

	"ngac-platform/services/policy/internal/ngac"
)

// fakeProhibitions is an in-memory ProhibitionFinder with the same matching
// rule as the real store: subject_id ∈ subjectIDs AND operation ∈ operations.
type fakeProhibitions struct {
	mu    sync.Mutex
	items []*ngac.Prohibition
	err   error // returned instead of results when set

	calls atomic.Int64
}

func (f *fakeProhibitions) FindForSubjects(_ context.Context, subjectIDs []string, operation string) ([]*ngac.Prohibition, error) {
	f.calls.Add(1)
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	var out []*ngac.Prohibition
	for _, p := range f.items {
		if slices.Contains(subjectIDs, p.SubjectID) && slices.Contains(p.Operations, operation) {
			out = append(out, p)
		}
	}
	return out, nil
}

func (f *fakeProhibitions) setErr(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.err = err
}

// fakeCTE is a CTEChecker that answers from a fixed table.
type fakeCTE struct {
	allowed map[string]bool // "user|object|op" → allowed
	err     error
	calls   atomic.Int64
}

func (f *fakeCTE) CheckAccess(_ context.Context, user, object, op string) (bool, error) {
	f.calls.Add(1)
	if f.err != nil {
		return false, f.err
	}
	return f.allowed[user+"|"+object+"|"+op], nil
}

// recordingCache misses on every Get and records every Set.
type recordingCache struct {
	mu   sync.Mutex
	sets []*ngac.AccessDecision
}

func (c *recordingCache) Get(context.Context, ngac.AccessRequest) (*ngac.AccessDecision, string) {
	return nil, ""
}

func (c *recordingCache) Set(_ context.Context, _ ngac.AccessRequest, d *ngac.AccessDecision) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sets = append(c.sets, d)
}

func (c *recordingCache) stored() []*ngac.AccessDecision {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]*ngac.AccessDecision(nil), c.sets...)
}

// recordingShards is a ShardManager that only records invalidations.
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
func (r *recordingShards) InvalidateAll() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.all++
}
func (r *recordingShards) Stats() ngac.ShardStats { return ngac.ShardStats{} }

// --- Fixture: one tenant PC plus a second, unrelated PC ---
//
//	PC_A
//	├── UA staff ── user alice
//	│   └── (assoc staff → OA docs [read, write])
//	├── OA docs
//	│   └── OA secret
//	PC_B
//	└── UA outsiders ── user bob   (assoc outsiders → OA docs [read]: crosses PCs)
//	user carol — assigned to no UA at all
//
// alice may read/write docs and secret; bob's association crosses policy
// classes, so the intersection principle denies him; carol has no path.
func buildFailClosedGraph() *ngac.Graph {
	g := ngac.NewGraph()
	for _, n := range []*ngac.NGACNode{
		{ID: "pc-a", Name: "PC_A", NodeType: ngac.NodeTypePolicyClass},
		{ID: "pc-b", Name: "PC_B", NodeType: ngac.NodeTypePolicyClass},
		{ID: "ua-staff", Name: "Staff", NodeType: ngac.NodeTypeUserAttribute},
		{ID: "ua-outsiders", Name: "Outsiders", NodeType: ngac.NodeTypeUserAttribute},
		{ID: "oa-docs", Name: "Docs", NodeType: ngac.NodeTypeObjectAttr},
		{ID: "oa-secret", Name: "Secret", NodeType: ngac.NodeTypeObjectAttr},
		{ID: "u-alice", Name: "alice", NodeType: ngac.NodeTypeUser},
		{ID: "u-bob", Name: "bob", NodeType: ngac.NodeTypeUser},
		{ID: "u-carol", Name: "carol", NodeType: ngac.NodeTypeUser},
	} {
		g.AddNode(n)
	}
	for _, a := range []*ngac.Assignment{
		{ID: "as-staff", ChildID: "ua-staff", ParentID: "pc-a"},
		{ID: "as-outsiders", ChildID: "ua-outsiders", ParentID: "pc-b"},
		{ID: "as-docs", ChildID: "oa-docs", ParentID: "pc-a"},
		{ID: "as-secret", ChildID: "oa-secret", ParentID: "oa-docs"},
		{ID: "as-alice", ChildID: "u-alice", ParentID: "ua-staff"},
		{ID: "as-bob", ChildID: "u-bob", ParentID: "ua-outsiders"},
	} {
		if err := g.AddAssignment(a); err != nil {
			panic(err)
		}
	}
	for _, a := range []*ngac.Association{
		{ID: "assoc-staff", UAID: "ua-staff", OAID: "oa-docs", Operations: []string{"read", "write"}},
		{ID: "assoc-outsiders", UAID: "ua-outsiders", OAID: "oa-docs", Operations: []string{"read"}},
	} {
		if err := g.AddAssociation(a); err != nil {
			panic(err)
		}
	}
	return g
}

// testRedis returns a client on a dedicated logical DB of the test Redis,
// flushed before and after, or skips when no test Redis is configured.
func testRedis(t *testing.T, db int) *redis.Client {
	t.Helper()
	addr := os.Getenv("TEST_REDIS_ADDR")
	if addr == "" {
		addr = os.Getenv("REDIS_ADDR")
	}
	if addr == "" {
		t.Skip("TEST_REDIS_ADDR not set")
	}
	rdb := redis.NewClient(&redis.Options{Addr: addr, DB: db})
	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		rdb.Close()
		t.Skipf("test redis not available: %v", err)
	}
	rdb.FlushDB(ctx)
	t.Cleanup(func() {
		rdb.FlushDB(context.Background())
		rdb.Close()
	})
	return rdb
}
