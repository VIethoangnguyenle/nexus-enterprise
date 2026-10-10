package grpc

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "ngac-platform/proto/policy"
	"ngac-platform/services/policy/internal/ngac"
)

// decisionsServer answers from an in-memory graph:
//
//	alice -> staff -> PC_A <- docs <- secret      staff has read, write on docs
//	bob   -> outsiders -> PC_B                    outsiders has read on docs, in another PC
//
// and a prohibition that forbids staff to write on secret.
func decisionsServer(t *testing.T) *ReadServer {
	t.Helper()
	g := ngac.NewGraph()
	for _, n := range []*ngac.NGACNode{
		{ID: "pc-a", Name: "PC_A", NodeType: ngac.NodeTypePolicyClass},
		{ID: "pc-b", Name: "PC_B", NodeType: ngac.NodeTypePolicyClass},
		{ID: "staff", Name: "Staff", NodeType: ngac.NodeTypeUserAttribute},
		{ID: "outsiders", Name: "Outsiders", NodeType: ngac.NodeTypeUserAttribute},
		{ID: "docs", Name: "Docs", NodeType: ngac.NodeTypeObjectAttr},
		{ID: "secret", Name: "Secret", NodeType: ngac.NodeTypeObjectAttr},
		{ID: "alice", Name: "alice", NodeType: ngac.NodeTypeUser},
		{ID: "bob", Name: "bob", NodeType: ngac.NodeTypeUser},
	} {
		g.AddNode(n)
	}
	for _, a := range []*ngac.Assignment{
		{ID: "1", ChildID: "staff", ParentID: "pc-a"},
		{ID: "2", ChildID: "outsiders", ParentID: "pc-b"},
		{ID: "3", ChildID: "docs", ParentID: "pc-a"},
		{ID: "4", ChildID: "secret", ParentID: "docs"},
		{ID: "5", ChildID: "alice", ParentID: "staff"},
		{ID: "6", ChildID: "bob", ParentID: "outsiders"},
	} {
		require.NoError(t, g.AddAssignment(a))
	}
	for _, a := range []*ngac.Association{
		{ID: "a1", UAID: "staff", OAID: "docs", Operations: []string{"read", "write"}},
		{ID: "a2", UAID: "outsiders", OAID: "docs", Operations: []string{"read"}},
	} {
		require.NoError(t, g.AddAssociation(a))
	}
	require.NoError(t, g.AddProhibition(&ngac.Prohibition{
		Name: "no-secret-writes", SubjectID: "staff", Operations: []string{"write"}, TargetOAIDs: []string{"secret"},
	}))
	evaluator := ngac.NewAccessEvaluator(ngac.NewLayeredCache(nil), ngac.NewDecisionEngine(g, nil))
	return NewReadServer(ngac.NewStore(nil, g), nil, evaluator, nil, nil)
}

func TestCheckAccess_AllowsOnlyWhatBothSidesReach(t *testing.T) {
	s := decisionsServer(t)
	check := func(user, object, op string) *pb.AccessDecision {
		t.Helper()
		d, err := s.CheckAccess(context.Background(), &pb.CheckAccessRequest{UserNodeId: user, ObjectNodeId: object, Operation: op})
		require.NoError(t, err)
		return d
	}

	assert.Equal(t, ngac.DecisionAllow, check("alice", "docs", "read").Decision)
	assert.Equal(t, ngac.DecisionAllow, check("alice", "secret", "read").Decision, "access flows down to a nested OA")
	assert.Equal(t, ngac.DecisionDeny, check("alice", "docs", "manage").Decision, "an operation nobody granted")
	assert.Equal(t, ngac.DecisionDeny, check("bob", "docs", "read").Decision, "an association in another policy class is no grant")
	assert.Equal(t, ngac.DecisionDeny, check("nobody", "docs", "read").Decision, "an unknown user is denied")
	assert.Equal(t, ngac.DecisionDeny, check("alice", "no-such-oa", "read").Decision, "an unknown object is denied")
	assert.Equal(t, ngac.DecisionDeny, check("alice", "docs", "").Decision, "no operation is denied")
}

func TestCheckAccess_ProhibitionIsNamedInTheAnswer(t *testing.T) {
	s := decisionsServer(t)
	d, err := s.CheckAccess(context.Background(), &pb.CheckAccessRequest{UserNodeId: "alice", ObjectNodeId: "secret", Operation: "write"})
	require.NoError(t, err)

	assert.Equal(t, ngac.DecisionDeny, d.Decision)
	require.NotNil(t, d.Explanation.ProhibitionDenied)
	assert.Equal(t, "no-secret-writes", d.Explanation.ProhibitionDenied.ProhibitionName)
	assert.Equal(t, "staff", d.Explanation.ProhibitionDenied.SubjectId)

	allowed, err := s.CheckAccess(context.Background(), &pb.CheckAccessRequest{UserNodeId: "alice", ObjectNodeId: "docs", Operation: "write"})
	require.NoError(t, err)
	assert.Equal(t, ngac.DecisionAllow, allowed.Decision, "the prohibition reaches only its target")
	assert.Nil(t, allowed.Explanation.ProhibitionDenied)
}

func TestBatchCheckAccess_AnswersEveryPairAndRefusesNoUser(t *testing.T) {
	s := decisionsServer(t)
	ctx := context.Background()

	_, err := s.BatchCheckAccess(ctx, &pb.BatchCheckAccessRequest{ObjectIds: []string{"docs"}, Operations: []string{"read"}})
	assert.Equal(t, codes.InvalidArgument, status.Code(err))

	res, err := s.BatchCheckAccess(ctx, &pb.BatchCheckAccessRequest{
		UserNodeId: "alice", ObjectIds: []string{"docs", "secret"}, Operations: []string{"read", "write"},
	})
	require.NoError(t, err)
	assert.True(t, res.Results["docs"].Permissions["write"])
	assert.True(t, res.Results["secret"].Permissions["read"])
	assert.False(t, res.Results["secret"].Permissions["write"], "the prohibition holds in a batch too")

	out, err := s.BatchCheckAccess(ctx, &pb.BatchCheckAccessRequest{
		UserNodeId: "bob", ObjectIds: []string{"docs"}, Operations: []string{"read"},
	})
	require.NoError(t, err)
	assert.False(t, out.Results["docs"].Permissions["read"])
}

func TestNodeLookups_FindOrSayNotFound(t *testing.T) {
	s := decisionsServer(t)
	ctx := context.Background()

	n, err := s.FindNodeByName(ctx, &pb.FindNodeByNameRequest{Name: "Docs", NodeType: ngac.NodeTypeObjectAttr})
	require.NoError(t, err)
	assert.Equal(t, "docs", n.Id)
	_, err = s.FindNodeByName(ctx, &pb.FindNodeByNameRequest{Name: "Docs", NodeType: ngac.NodeTypeUser})
	assert.Equal(t, codes.NotFound, status.Code(err), "the type is part of the key")
	_, err = s.GetNode(ctx, &pb.GetNodeRequest{NodeId: "missing"})
	assert.Equal(t, codes.NotFound, status.Code(err))

	users, err := s.GetNodesByType(ctx, &pb.GetNodesByTypeRequest{NodeType: ngac.NodeTypeUser})
	require.NoError(t, err)
	assert.Len(t, users.Nodes, 2)

	yes, err := s.IsAssigned(ctx, &pb.IsAssignedRequest{ChildId: "alice", ParentId: "staff"})
	require.NoError(t, err)
	assert.True(t, yes.Value)
	no, err := s.IsAssigned(ctx, &pb.IsAssignedRequest{ChildId: "alice", ParentId: "outsiders"})
	require.NoError(t, err)
	assert.False(t, no.Value)
}

func TestOperationAndProhibitionLists_AreEmptyWithoutTheirStores(t *testing.T) {
	s := decisionsServer(t)
	ops, err := s.ListOperations(context.Background(), &pb.Empty{})
	require.NoError(t, err)
	assert.Empty(t, ops.Operations)
	ps, err := s.ListProhibitions(context.Background(), &pb.ListProhibitionsRequest{})
	require.NoError(t, err)
	assert.Empty(t, ps.Prohibitions)
}

func TestOperationAndProhibitionLists_ReadTheStores(t *testing.T) {
	store, pool := setupWriteTestStore(t)
	ctx := context.Background()
	ops := ngac.NewOperationStore(pool)
	ps := ngac.NewProhibitionStore(pool, nil)
	s := NewReadServer(store, nil, nil, ops, ps)

	_, err := ps.Create(ctx, &ngac.Prohibition{Name: "list-p-" + t.Name(), SubjectID: "list-subject-" + t.Name(), Operations: []string{"read"}, TargetOAIDs: []string{"oa"}, Intersection: true})
	require.NoError(t, err)
	t.Cleanup(func() { pool.Exec(ctx, "DELETE FROM ngac_prohibitions WHERE name = $1", "list-p-"+t.Name()) })
	_, err = ops.Register(ctx, []string{"listed_op_for_test"})
	require.NoError(t, err)
	t.Cleanup(func() { pool.Exec(ctx, "DELETE FROM ngac_operations WHERE name = 'listed_op_for_test'") })

	gotOps, err := s.ListOperations(ctx, &pb.Empty{})
	require.NoError(t, err)
	assert.Contains(t, gotOps.Operations, "listed_op_for_test")

	gotPs, err := s.ListProhibitions(ctx, &pb.ListProhibitionsRequest{SubjectId: "list-subject-" + t.Name()})
	require.NoError(t, err)
	require.Len(t, gotPs.Prohibitions, 1)
	assert.True(t, gotPs.Prohibitions[0].Intersection)
	assert.Equal(t, []string{"oa"}, gotPs.Prohibitions[0].TargetOaIds)
}
