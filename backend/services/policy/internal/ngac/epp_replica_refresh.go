package ngac

import (
	"context"
	"fmt"
	"log/slog"
)

// GraphMutation is one graph change reported by the policy writer on the
// ngac.graph.mutated event.
type GraphMutation struct {
	Type    string   // one of the Mutation* constants
	NodeIDs []string // nodes the writer mutated; empty means "everything"
}

// GraphReloader is the store surface the replica refresher needs.
// *Store implements it.
type GraphReloader interface {
	ReloadGraph(ctx context.Context) error
	GetGraph() *Graph
}

// GraphInvalidator is the EPP invalidation surface for the decision cache (L1).
// *InvalidationCoordinator implements it.
type GraphInvalidator interface {
	InvalidateForNodes(ctx context.Context, nodeIDs ...string)
	InvalidateAll(ctx context.Context)
}

var (
	_ GraphReloader    = (*Store)(nil)
	_ GraphInvalidator = (*InvalidationCoordinator)(nil)
)

// ReplicaGraphRefresher keeps a read replica (policy-read) in step with the
// writer (policy).
//
// The writer mutates its own in-memory graph and runs EPP invalidation
// in-process. A replica has its own in-memory graph and its own shard cache,
// which nothing in the writer can reach; without this it serves decisions from
// the graph it loaded at startup for as long as it lives.
//
// For each batch of mutation events it:
//  1. resolves, against the graph as it was, what each mutation touches
//     (workspaces; for a deleted node, everything that was below it);
//  2. reloads the graph from the database (the event does not carry enough to
//     replay a mutation, and the database is the source of truth);
//  3. runs the same invalidation the writer runs — shard invalidation, then
//     InvalidationCoordinator.InvalidateForNodes — or a full invalidation when
//     a policy class was removed or the writer reloaded its whole graph.
//
// Step 3 is not redundant with the writer's own invalidation of the shared
// caches: between the writer's invalidation and this replica's reload, the
// replica can recompute a decision from its stale graph and write it back to
// Redis. Running
// the invalidation again after the reload removes those.
type ReplicaGraphRefresher struct {
	store        GraphReloader
	shards       ShardManager // optional
	invalidation GraphInvalidator
}

// NewReplicaGraphRefresher wires a refresher. shards may be nil.
func NewReplicaGraphRefresher(store GraphReloader, shards ShardManager, invalidation GraphInvalidator) *ReplicaGraphRefresher {
	return &ReplicaGraphRefresher{store: store, shards: shards, invalidation: invalidation}
}

// Apply brings the replica up to date with a batch of mutations. The graph is
// reloaded once per batch, however many mutations it holds.
//
// On error nothing has been invalidated and the graph is unchanged; the caller
// must retry the same batch, or the replica stays stale.
func (r *ReplicaGraphRefresher) Apply(ctx context.Context, batch []GraphMutation) error {
	if len(batch) == 0 {
		return nil
	}

	// Step 1: resolve against the graph BEFORE the reload. After it, a deleted
	// node — and with it its workspace and its descendants — is unresolvable.
	graph := r.store.GetGraph()
	full := false
	before := make([][]string, len(batch))  // workspaces per mutation
	targets := make([][]string, len(batch)) // nodes to invalidate per mutation
	for i, m := range batch {
		if m.Type == MutationLoadGraph || len(m.NodeIDs) == 0 {
			full = true
			continue
		}
		before[i] = AffectedWorkspaces(graph, m.NodeIDs...)
		targets[i] = append([]string(nil), m.NodeIDs...) // never alias the event's slice
		if m.Type == MutationDeleteNode {
			for _, id := range m.NodeIDs {
				impact := ResolveRemovalImpact(graph, id)
				if impact.PolicyClass {
					full = true
				}
				targets[i] = append(targets[i], impact.NodeIDs...)
			}
		}
	}

	// Step 2: reload. On failure the graph is untouched (ReloadGraph builds
	// off to the side) and we invalidate nothing: the retry redoes everything.
	if err := r.store.ReloadGraph(ctx); err != nil {
		return fmt.Errorf("reload graph: %w", err)
	}

	// Step 3: the writer's invalidation, replayed on this replica.
	if full {
		if r.shards != nil {
			r.shards.InvalidateAll()
		}
		r.invalidation.InvalidateAll(ctx)
		slog.Info("replica graph refreshed: full invalidation", "mutations", len(batch))
		return nil
	}

	after := r.store.GetGraph()
	for i, m := range batch {
		// A created node only resolves to its workspace after the reload; a
		// deleted one only before it. Take both.
		ws := unionSorted(before[i], AffectedWorkspaces(after, m.NodeIDs...))
		if r.shards != nil {
			for _, w := range ws {
				r.shards.InvalidateShard(w)
			}
		}
		r.invalidation.InvalidateForNodes(ctx, dedupe(targets[i])...)
	}
	slog.Info("replica graph refreshed", "mutations", len(batch))
	return nil
}

func unionSorted(a, b []string) []string {
	set := make(map[string]bool, len(a)+len(b))
	for _, s := range a {
		set[s] = true
	}
	for _, s := range b {
		set[s] = true
	}
	return sortedKeys(set)
}

func dedupe(ids []string) []string {
	seen := make(map[string]bool, len(ids))
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}
