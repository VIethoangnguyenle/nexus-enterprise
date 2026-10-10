package domain_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"ngac-platform/ngac"
	policypb "ngac-platform/proto/policy"
	"ngac-platform/services/workspace/internal/domain"
	"ngac-platform/services/workspace/internal/store"
)

// DeleteWorkspace is the compensation for a workspace that was just created. It
// must refuse everyone but a lone owner, remove everything, and be harmless to
// repeat. These tests run it against an in-memory graph that records the order
// of deletions.

// tdGraph is a policy service holding an assignment graph, answering the reads
// DeleteWorkspace makes and recording deletions.
type tdGraph struct {
	policypb.PolicyWriteServiceClient
	nodes   map[string]*policypb.NGACNode
	parents map[string][]string // child -> parents
	deleted []string
	failOn  string // node id whose deletion fails once
	clock   time.Time
}

func newTDGraph() *tdGraph {
	return &tdGraph{nodes: map[string]*policypb.NGACNode{}, parents: map[string][]string{}, clock: time.Unix(1_700_000_000, 0)}
}

// add creates a node a moment after the previous one, as provisioning does.
func (g *tdGraph) add(id, name, typ string, parents ...string) {
	g.clock = g.clock.Add(time.Second)
	g.nodes[id] = &policypb.NGACNode{Id: id, Name: name, NodeType: typ, CreatedAt: timestamppb.New(g.clock)}
	g.parents[id] = parents
}

func (g *tdGraph) live() []string {
	var out []string
	for id := range g.nodes {
		out = append(out, id)
	}
	slices.Sort(out)
	return out
}

func (g *tdGraph) descendantsOf(root string) []*policypb.NGACNode {
	var out []*policypb.NGACNode
	for id, n := range g.nodes {
		if id == root {
			continue
		}
		seen := map[string]bool{}
		var up func(string) bool
		up = func(x string) bool {
			if x == root {
				return true
			}
			if seen[x] {
				return false
			}
			seen[x] = true
			for _, p := range g.parents[x] {
				if _, ok := g.nodes[p]; ok && up(p) {
					return true
				}
			}
			return false
		}
		if up(id) {
			out = append(out, n)
		}
	}
	return out
}

func (g *tdGraph) GetDescendants(_ context.Context, in *policypb.GetDescendantsRequest, _ ...grpc.CallOption) (*policypb.NodeList, error) {
	return &policypb.NodeList{Nodes: g.descendantsOf(in.NodeId)}, nil
}

func (g *tdGraph) GetChildren(_ context.Context, in *policypb.GetChildrenRequest, _ ...grpc.CallOption) (*policypb.NodeList, error) {
	var out []*policypb.NGACNode
	for id, ps := range g.parents {
		if slices.Contains(ps, in.NodeId) {
			if n, ok := g.nodes[id]; ok {
				out = append(out, n)
			}
		}
	}
	return &policypb.NodeList{Nodes: out}, nil
}

func (g *tdGraph) DeleteNode(_ context.Context, in *policypb.DeleteNodeRequest, _ ...grpc.CallOption) (*policypb.Empty, error) {
	if g.failOn == in.NodeId {
		g.failOn = ""
		return nil, errors.New("policy service unavailable")
	}
	if _, ok := g.nodes[in.NodeId]; ok {
		g.deleted = append(g.deleted, in.NodeId)
		delete(g.nodes, in.NodeId)
	}
	return &policypb.Empty{}, nil
}

// tdRead is the read side of the same graph.
type tdRead struct {
	policypb.PolicyReadServiceClient
	g *tdGraph
}

func (r tdRead) FindNodeByName(_ context.Context, in *policypb.FindNodeByNameRequest, _ ...grpc.CallOption) (*policypb.NGACNode, error) {
	for _, n := range r.g.nodes {
		if n.Name == in.Name && n.NodeType == in.NodeType {
			return n, nil
		}
	}
	return nil, status.Error(codes.NotFound, "node not found")
}

// tdStore is the workspace store with the teardown queries, in memory.
type tdStore struct {
	domain.WorkspaceStore // the operations this test does not reach
	rows                  map[string]*store.Workspace
	creator               map[string]string
	members               map[string][]string // workspace -> user ids listed
	purged                []string
	failOn                bool
}

func (s *tdStore) Insert(_ context.Context, w *store.Workspace) error { s.rows[w.ID] = w; return nil }
func (s *tdStore) GetByID(_ context.Context, id string) (*store.Workspace, error) {
	if w, ok := s.rows[id]; ok {
		return w, nil
	}
	return nil, errors.New("no rows")
}
func (s *tdStore) ListAll(context.Context) ([]*store.Workspace, error) { return nil, nil }
func (s *tdStore) UpdateDetails(context.Context, string, *string, *string) (string, string, error) {
	return "", "", nil
}
func (s *tdStore) WithOwnerLock(ctx context.Context, _ string, fn func(context.Context) error) error {
	return fn(ctx)
}
func (s *tdStore) WorkspaceCreator(_ context.Context, id string) (string, bool, error) {
	if _, ok := s.rows[id]; !ok {
		return "", false, nil
	}
	return s.creator[id], true, nil
}
func (s *tdStore) OtherMembers(_ context.Context, id, userID string) (int, error) {
	n := 0
	for _, u := range s.members[id] {
		if u != userID {
			n++
		}
	}
	return n, nil
}
func (s *tdStore) PurgeWorkspace(_ context.Context, id string) error {
	if s.failOn {
		return errors.New("db down")
	}
	delete(s.rows, id)
	delete(s.members, id)
	s.purged = append(s.purged, id)
	return nil
}

const (
	meNode, meUser       = "node-me", "user-me"
	otherNode, otherUser = "node-other", "user-other"
)

type tdFixture struct {
	g   *tdGraph
	st  *tdStore
	svc *domain.Service
}

// workspaceGraph builds what CreateWorkspace builds, for workspace id, with
// me as its only owner.
func (f *tdFixture) workspaceGraph(id string) {
	w := ngac.WorkspaceID(id)
	f.g.add("pc-"+id, ngac.PCName(w), ngac.TypePC)
	f.g.add("own-"+id, ngac.OwnersUAName(w), ngac.TypeUA, "pc-"+id)
	f.g.add("mem-"+id, ngac.MembersUAName(w), ngac.TypeUA, "pc-"+id)
	f.g.add("mgmt-"+id, ngac.MgmtOAName(w), ngac.TypeOA, "pc-"+id)
	f.g.add("docs-"+id, ngac.DocumentsOAName(w), ngac.TypeOA, "pc-"+id)
	f.g.add("chan-"+id, ngac.ChannelsOAName(w), ngac.TypeOA, "pc-"+id)
	f.g.add("drive-"+id, "drive root "+id, ngac.TypeOA, "docs-"+id)
	f.st.rows[id] = &store.Workspace{ID: id, Name: "WS " + id, NGACPcID: "pc-" + id}
	f.st.creator[id] = meUser
	f.st.members[id] = []string{meUser}
}

func newTDFixture(t *testing.T) *tdFixture {
	t.Helper()
	g := newTDGraph()
	st := &tdStore{rows: map[string]*store.Workspace{}, creator: map[string]string{}, members: map[string][]string{}}
	f := &tdFixture{g: g, st: st}
	f.svc = domain.NewService(st, nil, tdRead{g: g}, g, nil, nil)
	g.add(meNode, "U_me", ngac.TypeU)
	g.add(otherNode, "U_other", ngac.TypeU)
	return f
}

// join places a person's node under a workspace attribute.
func (f *tdFixture) join(node, ua string) { f.g.parents[node] = append(f.g.parents[node], ua) }

func TestDeleteWorkspace_RemovesEverythingNewestFirst(t *testing.T) {
	f := newTDFixture(t)
	f.workspaceGraph("w1")
	f.join(meNode, "own-w1")
	f.join(meNode, "mem-w1")

	require.NoError(t, f.svc.DeleteWorkspace(context.Background(), meNode, meUser, "w1"))

	assert.Equal(t, []string{meNode, otherNode}, f.g.live(), "only the people are left, never deleted")
	assert.Equal(t, []string{"drive-w1", "chan-w1", "docs-w1", "mgmt-w1", "mem-w1", "own-w1", "pc-w1"}, f.g.deleted,
		"newest first, the policy class last")
	assert.Equal(t, []string{"w1"}, f.st.purged)
	assert.Empty(t, f.st.rows, "no workspace row remains")
	assert.Empty(t, f.st.members["w1"])
}

func TestDeleteWorkspace_IsIdempotent(t *testing.T) {
	f := newTDFixture(t)
	f.workspaceGraph("w1")
	f.join(meNode, "own-w1")
	require.NoError(t, f.svc.DeleteWorkspace(context.Background(), meNode, meUser, "w1"))
	before := slices.Clone(f.g.deleted)

	require.NoError(t, f.svc.DeleteWorkspace(context.Background(), meNode, meUser, "w1"), "a second call finds nothing and succeeds")
	assert.Equal(t, before, f.g.deleted)
	require.NoError(t, f.svc.DeleteWorkspace(context.Background(), meNode, meUser, "never-existed"))
}

func TestDeleteWorkspace_FinishesAHalfCreatedOrHalfRemovedWorkspace(t *testing.T) {
	t.Run("graph only: the row was never written", func(t *testing.T) {
		f := newTDFixture(t)
		f.workspaceGraph("w1")
		delete(f.st.rows, "w1")
		f.join(meNode, "own-w1")
		require.NoError(t, f.svc.DeleteWorkspace(context.Background(), meNode, meUser, "w1"))
		assert.Equal(t, []string{meNode, otherNode}, f.g.live())
	})
	t.Run("a class and one attribute, no owners, no row", func(t *testing.T) {
		f := newTDFixture(t)
		f.g.add("pc-w1", ngac.PCName(ngac.WorkspaceID("w1")), ngac.TypePC)
		f.g.add("own-w1", ngac.OwnersUAName(ngac.WorkspaceID("w1")), ngac.TypeUA, "pc-w1")
		delete(f.g.nodes, "own-w1")
		f.g.add("mem-w1", ngac.MembersUAName(ngac.WorkspaceID("w1")), ngac.TypeUA, "pc-w1")
		require.NoError(t, f.svc.DeleteWorkspace(context.Background(), meNode, meUser, "w1"))
		assert.Equal(t, []string{meNode, otherNode}, f.g.live())
	})
	t.Run("owners already gone: the creator finishes it", func(t *testing.T) {
		f := newTDFixture(t)
		f.workspaceGraph("w1")
		delete(f.g.nodes, "own-w1")
		require.NoError(t, f.svc.DeleteWorkspace(context.Background(), meNode, meUser, "w1"))
		assert.Equal(t, []string{meNode, otherNode}, f.g.live())
		assert.Empty(t, f.st.rows)
	})
	t.Run("owners already gone: someone who is not the creator may not", func(t *testing.T) {
		f := newTDFixture(t)
		f.workspaceGraph("w1")
		delete(f.g.nodes, "own-w1")
		err := f.svc.DeleteWorkspace(context.Background(), otherNode, otherUser, "w1")
		assert.ErrorIs(t, err, domain.ErrAccessDenied)
		assert.Empty(t, f.g.deleted)
		assert.Len(t, f.st.rows, 1)
	})
}

func TestDeleteWorkspace_RefusesAndRemovesNothing(t *testing.T) {
	cases := map[string]func(f *tdFixture) (callerNode, callerUser, ws string){
		"no caller node": func(f *tdFixture) (string, string, string) { return "", meUser, "w1" },
		"no caller user": func(f *tdFixture) (string, string, string) { return meNode, "", "w1" },
		"a non-owner member of a workspace someone else owns": func(f *tdFixture) (string, string, string) {
			f.join(otherNode, "mem-w1")
			return otherNode, otherUser, "w1"
		},
		"an owner with other people in the graph": func(f *tdFixture) (string, string, string) {
			f.join(otherNode, "mem-w1")
			return meNode, meUser, "w1"
		},
		"an owner with other people listed as members": func(f *tdFixture) (string, string, string) {
			f.st.members["w1"] = append(f.st.members["w1"], otherUser)
			return meNode, meUser, "w1"
		},
		"a stranger": func(f *tdFixture) (string, string, string) { return otherNode, otherUser, "w1" },
		"another workspace than the one the caller owns": func(f *tdFixture) (string, string, string) {
			f.workspaceGraph("w2")
			f.join(otherNode, "own-w2")
			f.st.creator["w2"] = otherUser
			f.st.members["w2"] = []string{otherUser}
			return meNode, meUser, "w2"
		},
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			f := newTDFixture(t)
			f.workspaceGraph("w1")
			f.join(meNode, "own-w1")
			node, user, ws := setup(f)
			rowsBefore, nodesBefore := len(f.st.rows), f.g.live()

			err := f.svc.DeleteWorkspace(context.Background(), node, user, ws)

			require.ErrorIs(t, err, domain.ErrAccessDenied)
			assert.Empty(t, f.g.deleted, "nothing was deleted from the graph")
			assert.Empty(t, f.st.purged, "no row was removed")
			assert.Len(t, f.st.rows, rowsBefore)
			assert.Equal(t, nodesBefore, f.g.live())
		})
	}
}

func TestDeleteWorkspace_NeedsAWorkspaceId(t *testing.T) {
	f := newTDFixture(t)
	assert.ErrorIs(t, f.svc.DeleteWorkspace(context.Background(), meNode, meUser, ""), domain.ErrInvalidInput)
}

func TestDeleteWorkspace_ARetryFinishesWhatAFailureLeft(t *testing.T) {
	f := newTDFixture(t)
	f.workspaceGraph("w1")
	f.join(meNode, "own-w1")
	f.g.failOn = "mgmt-w1"

	err := f.svc.DeleteWorkspace(context.Background(), meNode, meUser, "w1")
	require.Error(t, err)
	assert.NotContains(t, fmt.Sprint(err), "access denied")
	assert.NotEmpty(t, f.g.live()[2:], "some attributes remain after the failure")

	require.NoError(t, f.svc.DeleteWorkspace(context.Background(), meNode, meUser, "w1"))
	assert.Equal(t, []string{meNode, otherNode}, f.g.live(), "the retry removed the rest")
}

func TestDeleteWorkspace_AStoreFailureLeavesTheGraphAlone(t *testing.T) {
	f := newTDFixture(t)
	f.workspaceGraph("w1")
	f.join(meNode, "own-w1")
	f.st.failOn = true
	require.Error(t, f.svc.DeleteWorkspace(context.Background(), meNode, meUser, "w1"))
	assert.Empty(t, f.g.deleted, "the graph is removed only after the rows")
}
