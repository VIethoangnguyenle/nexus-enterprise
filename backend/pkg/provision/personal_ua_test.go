package provision_test

import (
	"context"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"ngac-platform/ngac"
	"ngac-platform/pkg/provision"
	policypb "ngac-platform/proto/policy"
	"ngac-platform/testutil"
)

type uaReader struct {
	ancestors []*policypb.NGACNode
	byName    *policypb.NGACNode
}

func (r uaReader) GetAncestors(context.Context, *policypb.GetAncestorsRequest, ...grpc.CallOption) (*policypb.NodeList, error) {
	return &policypb.NodeList{Nodes: r.ancestors}, nil
}

func (r uaReader) FindNodeByName(context.Context, *policypb.FindNodeByNameRequest, ...grpc.CallOption) (*policypb.NGACNode, error) {
	if r.byName == nil {
		return nil, status.Error(codes.NotFound, "no such node")
	}
	return r.byName, nil
}

func TestEnsurePersonalUAReusesTheOneTheUserIsAlreadyIn(t *testing.T) {
	w := testutil.NewFakePolicyWrite()
	mine := &policypb.NGACNode{Id: "ua-1", NodeType: ngac.TypeUA, Properties: ngac.PersonalUAProperties("u1")}
	id, err := provision.EnsurePersonalUA(context.Background(), uaReader{ancestors: []*policypb.NGACNode{mine}}, w, "u1")
	if err != nil || id != "ua-1" {
		t.Fatalf("got %q, %v; want the existing UA", id, err)
	}
	if w.Calls() != 0 {
		t.Errorf("an existing UA needs no write, log: %v", w.Log())
	}
}

// Deny: a UA is the person's by its properties, never by its name or by whose
// ancestor it is — a role an administrator called "User_u1" is not it.
func TestEnsurePersonalUAIgnoresAnImpostorAndCreatesTheRealOne(t *testing.T) {
	w := testutil.NewFakePolicyWrite()
	role := &policypb.NGACNode{Id: "role", Name: ngac.PersonalUAName("u1"), NodeType: ngac.TypeUA, Properties: map[string]string{ngac.PropType: ngac.PropTypeRole}}
	r := uaReader{ancestors: []*policypb.NGACNode{role}}
	id, err := provision.EnsurePersonalUA(context.Background(), r, w, "u1")
	if err != nil || id == "" || id == "role" {
		t.Fatalf("got %q, %v; want a new personal UA", id, err)
	}
	n, _ := w.Node(id)
	if !ngac.IsPersonalUAOf(n.GetProperties(), "u1") {
		t.Errorf("created UA is not marked as u1's: %v", n.GetProperties())
	}
}

func TestEnsurePersonalUAAdoptsAnUnassignedOneAndRollsBackItsOwnFailure(t *testing.T) {
	w := testutil.NewFakePolicyWrite()
	orphan := &policypb.NGACNode{Id: "ua-orphan", NodeType: ngac.TypeUA, Properties: ngac.PersonalUAProperties("u1")}
	id, err := provision.EnsurePersonalUA(context.Background(), uaReader{byName: orphan}, w, "u1")
	if err != nil || id != "ua-orphan" {
		t.Fatalf("got %q, %v; want the unassigned UA adopted", id, err)
	}

	// The assignment of a UA created now fails: the UA is not left behind.
	w2 := testutil.NewFakePolicyWrite()
	w2.FailAt = 2
	if _, err := provision.EnsurePersonalUA(context.Background(), uaReader{}, w2, "u1"); err == nil {
		t.Fatal("want the injected failure")
	}
	if live := w2.LiveNodes(); len(live) != 0 {
		t.Errorf("a failed creation left %v", live)
	}
}

func TestEnsurePersonalUARequiresAUser(t *testing.T) {
	if _, err := provision.EnsurePersonalUA(context.Background(), uaReader{}, testutil.NewFakePolicyWrite(), ""); err == nil {
		t.Fatal("no user must be refused")
	}
}
