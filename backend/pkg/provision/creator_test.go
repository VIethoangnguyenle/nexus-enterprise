package provision_test

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"ngac-platform/pkg/provision"
	policypb "ngac-platform/proto/policy"
	"ngac-platform/testutil"
)

type finder struct {
	node *policypb.NGACNode
	err  error
}

func (f finder) FindNodeByName(context.Context, *policypb.FindNodeByNameRequest, ...grpc.CallOption) (*policypb.NGACNode, error) {
	return f.node, f.err
}

func req(name string) *policypb.CreateNodeRequest {
	return &policypb.CreateNodeRequest{Name: name, NodeType: "OA"}
}

func TestEnsureNodeReusesAnExistingNodeAndNeverDeletesIt(t *testing.T) {
	w := testutil.NewFakePolicyWrite()
	c := provision.NewCreator(w)

	n, err := c.EnsureNode(context.Background(), finder{node: &policypb.NGACNode{Id: "old", Name: "x"}}, req("x"))
	if err != nil || n.Id != "old" {
		t.Fatalf("got %v, %v; want the existing node", n, err)
	}
	if err := c.Fail(context.Background(), errors.New("later step failed")); err == nil {
		t.Fatal("Fail must return the cause")
	}
	if w.Calls() != 0 || len(w.Log()) != 0 {
		t.Errorf("an existing node is neither created nor deleted, log: %v", w.Log())
	}
}

func TestEnsureNodeCreatesOnlyOnAPositiveNotFound(t *testing.T) {
	w := testutil.NewFakePolicyWrite()
	c := provision.NewCreator(w)

	n, err := c.EnsureNode(context.Background(), finder{err: status.Error(codes.NotFound, "no such node")}, req("fresh"))
	if err != nil || n.GetName() != "fresh" {
		t.Fatalf("got %v, %v; want the node created", n, err)
	}
	_ = c.Fail(context.Background(), errors.New("later step failed"))
	if live := w.LiveNodes(); len(live) != 0 {
		t.Errorf("a node this run created is rolled back, left: %v", live)
	}
}

// Deny: a lookup that fails for any other reason must not be read as "absent".
func TestEnsureNodeDoesNotCreateWhenTheLookupFails(t *testing.T) {
	w := testutil.NewFakePolicyWrite()
	c := provision.NewCreator(w)

	_, err := c.EnsureNode(context.Background(), finder{err: status.Error(codes.Unavailable, "policy down")}, req("x"))

	if err == nil {
		t.Fatal("an unavailable policy service must fail the lookup")
	}
	if w.Calls() != 0 {
		t.Errorf("no node may be created on a failed lookup, calls: %d", w.Calls())
	}
}

func TestDoneKeepsEverything(t *testing.T) {
	w := testutil.NewFakePolicyWrite()
	c := provision.NewCreator(w)
	if _, err := c.Node(context.Background(), req("a")); err != nil {
		t.Fatal(err)
	}
	c.Done()
	_ = c.Fail(context.Background(), errors.New("after done"))
	if live := w.LiveNodes(); len(live) != 1 {
		t.Errorf("Done must keep the work, live: %v", live)
	}
}

// seqFinder answers NotFound the first time and the node afterwards: the other
// run created it between our lookup and our write.
type seqFinder struct {
	node  *policypb.NGACNode
	calls int
}

func (f *seqFinder) FindNodeByName(context.Context, *policypb.FindNodeByNameRequest, ...grpc.CallOption) (*policypb.NGACNode, error) {
	f.calls++
	if f.calls == 1 {
		return nil, status.Error(codes.NotFound, "no such node")
	}
	return f.node, nil
}

func TestEnsureNodeAdoptsTheNodeAConcurrentRunCreated(t *testing.T) {
	w := testutil.NewFakePolicyWrite()
	w.FailAt = 1 // our create conflicts with the other run's
	c := provision.NewCreator(w)

	n, err := c.EnsureNode(context.Background(), &seqFinder{node: &policypb.NGACNode{Id: "theirs", Name: "x"}}, req("x"))

	if err != nil || n.Id != "theirs" {
		t.Fatalf("got %v, %v; want the other run's node", n, err)
	}
	_ = c.Fail(context.Background(), errors.New("later step failed"))
	if len(w.Log()) != 0 {
		t.Errorf("the adopted node is not ours to delete: %v", w.Log())
	}
}

// Deny: a failed create with nothing to adopt is still a failure.
func TestEnsureNodeFailsWhenTheCreateFailsAndNothingIsThere(t *testing.T) {
	w := testutil.NewFakePolicyWrite()
	w.FailAt = 1
	c := provision.NewCreator(w)

	_, err := c.EnsureNode(context.Background(), finder{err: status.Error(codes.NotFound, "no")}, req("x"))

	if err == nil {
		t.Fatal("the create error must surface")
	}
}
