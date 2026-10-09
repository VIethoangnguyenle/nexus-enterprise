package ngac_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/services/policy/internal/ngac"
)

// fakeReloader serves `current` and, on reload, switches to `next`.
type fakeReloader struct {
	current *ngac.Graph
	next    *ngac.Graph
	err     error
	reloads int
}

func (f *fakeReloader) GetGraph() *ngac.Graph { return f.current }
func (f *fakeReloader) ReloadGraph(context.Context) error {
	f.reloads++
	if f.err != nil {
		return f.err
	}
	f.current = f.next
	return nil
}

type invalidateCall struct {
	workspace string
	nodes     []string
}

type recordingInvalidator struct {
	calls []invalidateCall
	all   int
}

func (r *recordingInvalidator) InvalidateForNodes(_ context.Context, ws string, ids ...string) {
	r.calls = append(r.calls, invalidateCall{workspace: ws, nodes: ids})
}
func (r *recordingInvalidator) InvalidateAll(context.Context) { r.all++ }

// tenantGraph: PC(ws-1) ← UA team ← user u1, and optionally OA files ← PC.
func tenantGraph(withTeam, withFiles bool) *ngac.Graph {
	g := ngac.NewGraph()
	g.AddNode(&ngac.NGACNode{ID: "pc-1", Name: "PC_1", NodeType: ngac.NodeTypePolicyClass,
		Properties: map[string]string{"workspace_id": "ws-1"}})
	g.AddNode(&ngac.NGACNode{ID: "u1", Name: "u1", NodeType: ngac.NodeTypeUser})
	if withTeam {
		g.AddNode(&ngac.NGACNode{ID: "ua-team", Name: "Team", NodeType: ngac.NodeTypeUserAttribute})
		_ = g.AddAssignment(&ngac.Assignment{ID: "a1", ChildID: "ua-team", ParentID: "pc-1"})
		_ = g.AddAssignment(&ngac.Assignment{ID: "a2", ChildID: "u1", ParentID: "ua-team"})
	}
	if withFiles {
		g.AddNode(&ngac.NGACNode{ID: "oa-files", Name: "Files", NodeType: ngac.NodeTypeObjectAttr})
		_ = g.AddAssignment(&ngac.Assignment{ID: "a3", ChildID: "oa-files", ParentID: "pc-1"})
	}
	return g
}

// A deleted node is resolved against the graph as it was: its workspace shard
// is dropped and its former members are invalidated, although after the reload
// none of that is reachable any more.
func TestReplicaRefresh_DeleteNode_ResolvesBeforeReload(t *testing.T) {
	store := &fakeReloader{current: tenantGraph(true, false), next: tenantGraph(false, false)}
	shards := &recordingShards{}
	inv := &recordingInvalidator{}
	r := ngac.NewReplicaGraphRefresher(store, shards, inv)

	require.NoError(t, r.Apply(context.Background(), []ngac.GraphMutation{
		{Type: ngac.MutationDeleteNode, NodeIDs: []string{"ua-team"}},
	}))

	assert.Equal(t, 1, store.reloads)
	assert.Equal(t, []string{"ws-1"}, shards.invalidated, "the deleted UA's shard must be dropped")
	require.Len(t, inv.calls, 1)
	assert.Equal(t, "ws-1", inv.calls[0].workspace, "version bump must hit the workspace scope")
	assert.ElementsMatch(t, []string{"ua-team", "u1"}, inv.calls[0].nodes,
		"the UA's users lose access and must be invalidated")
	assert.Zero(t, inv.all)
}

// A created node only resolves to its workspace after the reload.
func TestReplicaRefresh_CreatedNode_ResolvesAfterReload(t *testing.T) {
	store := &fakeReloader{current: tenantGraph(true, false), next: tenantGraph(true, true)}
	shards := &recordingShards{}
	inv := &recordingInvalidator{}
	r := ngac.NewReplicaGraphRefresher(store, shards, inv)

	require.NoError(t, r.Apply(context.Background(), []ngac.GraphMutation{
		{Type: ngac.MutationCreateAssignment, NodeIDs: []string{"oa-files", "pc-1"}},
	}))

	assert.Equal(t, []string{"ws-1"}, shards.invalidated)
	require.Len(t, inv.calls, 1)
	assert.Equal(t, "ws-1", inv.calls[0].workspace)
	assert.ElementsMatch(t, []string{"oa-files", "pc-1"}, inv.calls[0].nodes)
}

func TestReplicaRefresh_FullInvalidation(t *testing.T) {
	for name, m := range map[string]ngac.GraphMutation{
		"writer reloaded its graph": {Type: ngac.MutationLoadGraph},
		"event without node IDs":    {Type: ngac.MutationCreateNode},
		"policy class deleted":      {Type: ngac.MutationDeleteNode, NodeIDs: []string{"pc-1"}},
	} {
		t.Run(name, func(t *testing.T) {
			store := &fakeReloader{current: tenantGraph(true, true), next: ngac.NewGraph()}
			shards := &recordingShards{}
			inv := &recordingInvalidator{}
			r := ngac.NewReplicaGraphRefresher(store, shards, inv)

			require.NoError(t, r.Apply(context.Background(), []ngac.GraphMutation{m}))

			assert.Equal(t, 1, store.reloads)
			assert.Equal(t, 1, shards.all, "every shard must be dropped")
			assert.Equal(t, 1, inv.all, "every cache layer must be flushed")
			assert.Empty(t, inv.calls)
		})
	}
}

func TestReplicaRefresh_ReloadsOncePerBatch(t *testing.T) {
	store := &fakeReloader{current: tenantGraph(true, true), next: tenantGraph(true, true)}
	inv := &recordingInvalidator{}
	r := ngac.NewReplicaGraphRefresher(store, &recordingShards{}, inv)

	require.NoError(t, r.Apply(context.Background(), []ngac.GraphMutation{
		{Type: ngac.MutationCreateAssignment, NodeIDs: []string{"u1", "ua-team"}},
		{Type: ngac.MutationRemoveAssignment, NodeIDs: []string{"u1", "ua-team"}},
		{Type: ngac.MutationCreateAssociation, NodeIDs: []string{"ua-team", "oa-files"}},
	}))

	assert.Equal(t, 1, store.reloads)
	assert.Len(t, inv.calls, 3, "each mutation is invalidated as the writer invalidated it")
}

// If the reload fails nothing is invalidated and the error is surfaced, so
// the caller retries the batch instead of considering it applied.
func TestReplicaRefresh_ReloadError_InvalidatesNothing(t *testing.T) {
	store := &fakeReloader{current: tenantGraph(true, false), err: errDBDown}
	shards := &recordingShards{}
	inv := &recordingInvalidator{}
	r := ngac.NewReplicaGraphRefresher(store, shards, inv)

	err := r.Apply(context.Background(), []ngac.GraphMutation{
		{Type: ngac.MutationDeleteNode, NodeIDs: []string{"ua-team"}},
	})

	require.ErrorIs(t, err, errDBDown)
	assert.Empty(t, shards.invalidated)
	assert.Empty(t, inv.calls)
	assert.Zero(t, inv.all)
}

// End to end against Postgres: a replica (its own Store and graph, as in
// policy-read) keeps allowing after the writer revokes access, until the
// mutation is applied — then it denies.
func TestReplicaRefresh_ReplicaStopsAllowingAfterRevocation(t *testing.T) {
	writer, pool := setupStore(t)
	ctx := context.Background()
	suffix := uuid.NewString()[:8]

	pc, err := writer.CreateNode(ctx, "PC_replica_"+suffix, ngac.NodeTypePolicyClass, nil)
	require.NoError(t, err)
	ua, err := writer.CreateNode(ctx, "UA_replica_"+suffix, ngac.NodeTypeUserAttribute, nil)
	require.NoError(t, err)
	oa, err := writer.CreateNode(ctx, "OA_replica_"+suffix, ngac.NodeTypeObjectAttr, nil)
	require.NoError(t, err)
	user, err := writer.CreateNode(ctx, "U_replica_"+suffix, ngac.NodeTypeUser, nil)
	require.NoError(t, err)
	t.Cleanup(func() {
		pool.Exec(context.Background(), "DELETE FROM ngac_nodes WHERE id = ANY($1)",
			[]string{pc.ID, ua.ID, oa.ID, user.ID})
	})
	_, err = writer.CreateAssignment(ctx, ua.ID, pc.ID)
	require.NoError(t, err)
	_, err = writer.CreateAssignment(ctx, oa.ID, pc.ID)
	require.NoError(t, err)
	_, err = writer.CreateAssignment(ctx, user.ID, ua.ID)
	require.NoError(t, err)
	_, err = writer.CreateAssociation(ctx, ua.ID, oa.ID, []string{"read"})
	require.NoError(t, err)

	// The replica loads its own graph, as policy-read does at startup.
	replica := ngac.NewStore(pool, ngac.NewGraph())
	require.NoError(t, replica.LoadGraph(ctx))
	engine := ngac.NewDecisionEngine(replica.GetGraph(), nil, nil)
	req := ngac.AccessRequest{UserNodeID: user.ID, ObjectNodeID: oa.ID, Operation: "read"}
	require.Equal(t, ngac.DecisionAllow, engine.Decide(ctx, req).Decision)

	// The writer revokes. The replica's graph does not know yet.
	require.NoError(t, writer.RemoveAssignment(ctx, user.ID, ua.ID))
	require.Equal(t, ngac.DecisionAllow, engine.Decide(ctx, req).Decision,
		"precondition: without the refresh the replica is stale (the bug)")

	inv := &recordingInvalidator{}
	r := ngac.NewReplicaGraphRefresher(replica, &recordingShards{}, inv)
	require.NoError(t, r.Apply(ctx, []ngac.GraphMutation{
		{Type: ngac.MutationRemoveAssignment, NodeIDs: []string{user.ID, ua.ID}},
	}))

	assert.Equal(t, ngac.DecisionDeny, engine.Decide(ctx, req).Decision,
		"after the mutation event the replica must deny")
	require.Len(t, inv.calls, 1)
	assert.ElementsMatch(t, []string{user.ID, ua.ID}, inv.calls[0].nodes)

	// A node deleted by the writer disappears from the replica (LoadGraph
	// alone would have kept it).
	require.NoError(t, writer.DeleteNode(ctx, oa.ID))
	require.NoError(t, r.Apply(ctx, []ngac.GraphMutation{
		{Type: ngac.MutationDeleteNode, NodeIDs: []string{oa.ID}},
	}))
	assert.Nil(t, replica.GetNode(oa.ID), "reload must reflect deletions")
}

func TestStoreReloadGraph_FailureLeavesGraphUntouched(t *testing.T) {
	g := tenantGraph(true, true)
	s := ngac.NewStore(unreachablePool(t), g)

	require.Error(t, s.ReloadGraph(context.Background()))
	assert.NotNil(t, g.GetNode("ua-team"), "a failed reload must not leave a partial or empty graph")
	assert.True(t, g.IsAssigned("u1", "ua-team"))
}
