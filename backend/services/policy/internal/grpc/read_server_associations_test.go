package grpc

import (
	"context"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "ngac-platform/proto/policy"
	"ngac-platform/services/policy/internal/ngac"
)

func associationsServer(t *testing.T) *ReadServer {
	t.Helper()
	g := ngac.NewGraph()
	for _, n := range []*ngac.NGACNode{
		{ID: "pc", Name: "pc", NodeType: "PC"},
		{ID: "role", Name: "role", NodeType: "UA"},
		{ID: "idle", Name: "idle", NodeType: "UA"},
		{ID: "docs", Name: "docs", NodeType: "OA"},
		{ID: "chat", Name: "chat", NodeType: "OA"},
		{ID: "alice", Name: "alice", NodeType: "U"},
	} {
		g.AddNode(n)
	}
	for _, a := range []*ngac.Association{
		{ID: "a1", UAID: "role", OAID: "docs", Operations: []string{"read", "write"}},
		{ID: "a2", UAID: "role", OAID: "chat", Operations: []string{"read"}},
	} {
		if err := g.AddAssociation(a); err != nil {
			t.Fatal(err)
		}
	}
	return NewReadServer(ngac.NewStore(nil, g), nil, nil, nil, nil)
}

func TestGetAssociations_ListsEveryGrantOfTheUA(t *testing.T) {
	s := associationsServer(t)
	got, err := s.GetAssociations(context.Background(), &pb.GetAssociationsRequest{UaId: "role"})
	if err != nil {
		t.Fatal(err)
	}
	byOA := map[string][]string{}
	for _, a := range got.Associations {
		byOA[a.OaId] = a.Operations
	}
	if len(byOA) != 2 || len(byOA["docs"]) != 2 || len(byOA["chat"]) != 1 {
		t.Fatalf("grants = %v", byOA)
	}
}

func TestGetAssociations_NoGrantsIsAnEmptyList(t *testing.T) {
	s := associationsServer(t)
	got, err := s.GetAssociations(context.Background(), &pb.GetAssociationsRequest{UaId: "idle"})
	if err != nil || len(got.Associations) != 0 {
		t.Fatalf("got %v, %v", got, err)
	}
}

// Refused, not answered with an empty list: an unknown node or a node that
// cannot hold an association is the caller's mistake and must say so.
func TestGetAssociations_RefusesWhatIsNotAUA(t *testing.T) {
	s := associationsServer(t)
	for name, tc := range map[string]struct {
		id   string
		code codes.Code
	}{
		"empty id":     {"", codes.InvalidArgument},
		"unknown node": {"nope", codes.NotFound},
		"a user":       {"alice", codes.InvalidArgument},
		"an OA":        {"docs", codes.InvalidArgument},
	} {
		_, err := s.GetAssociations(context.Background(), &pb.GetAssociationsRequest{UaId: tc.id})
		if status.Code(err) != tc.code {
			t.Errorf("%s: code = %v, want %v", name, status.Code(err), tc.code)
		}
	}
}

// What the caller gets back is a copy: changing it cannot reach the graph.
func TestGetAssociations_ReturnsACopy(t *testing.T) {
	s := associationsServer(t)
	got, _ := s.GetAssociations(context.Background(), &pb.GetAssociationsRequest{UaId: "role"})
	for _, a := range got.Associations {
		a.Operations[0] = "tampered"
	}
	again, _ := s.GetAssociations(context.Background(), &pb.GetAssociationsRequest{UaId: "role"})
	for _, a := range again.Associations {
		if a.Operations[0] == "tampered" {
			t.Fatal("the graph's operations were changed through the response")
		}
	}
}

// The writer answers the same, from its own graph.
func TestWriteServerGetAssociations_SameAnswerAndRefusals(t *testing.T) {
	rs := associationsServer(t)
	w := &WriteServer{store: rs.store}
	got, err := w.GetAssociations(context.Background(), &pb.GetAssociationsRequest{UaId: "role"})
	if err != nil || len(got.Associations) != 2 {
		t.Fatalf("got %v, %v", got, err)
	}
	for _, id := range []string{"", "nope", "alice", "docs"} {
		if _, err := w.GetAssociations(context.Background(), &pb.GetAssociationsRequest{UaId: id}); err == nil {
			t.Errorf("%q must be refused", id)
		}
	}
}

// The writer answers graph reads from the graph it has just written to, and a
// caller needs a verified end user for them (the auth policy treats every RPC
// other than the signup lookups alike).
func TestWriteServerGraphReads_AnswerFromTheWritersGraph(t *testing.T) {
	rs := associationsServer(t)
	// A write the writer's graph has and a stale replica would not.
	rs.store.GetGraph().AddNode(&ngac.NGACNode{ID: "bob", Name: "bob", NodeType: "U"})
	if err := rs.store.GetGraph().AddAssignment(&ngac.Assignment{ID: "a-bob", ChildID: "bob", ParentID: "role"}); err != nil {
		t.Fatal(err)
	}
	w := &WriteServer{store: rs.store}
	ctx := context.Background()

	kids, err := w.GetChildren(ctx, &pb.GetChildrenRequest{NodeId: "role"})
	if err != nil || len(kids.Nodes) != 1 || kids.Nodes[0].Id != "bob" {
		t.Fatalf("children = %v, %v", kids, err)
	}
	anc, err := w.GetAncestors(ctx, &pb.GetAncestorsRequest{NodeId: "bob"})
	if err != nil || len(anc.Nodes) != 1 || anc.Nodes[0].Id != "role" {
		t.Fatalf("ancestors = %v, %v", anc, err)
	}
	par, err := w.GetParents(ctx, &pb.GetParentsRequest{NodeId: "bob"})
	if err != nil || len(par.Nodes) != 1 {
		t.Fatalf("parents = %v, %v", par, err)
	}
	desc, err := w.GetDescendants(ctx, &pb.GetDescendantsRequest{NodeId: "role"})
	if err != nil || len(desc.Nodes) != 1 {
		t.Fatalf("descendants = %v, %v", desc, err)
	}
	n, err := w.GetNode(ctx, &pb.GetNodeRequest{NodeId: "bob"})
	if err != nil || n.NodeType != "U" {
		t.Fatalf("node = %v, %v", n, err)
	}
	if _, err := w.GetNode(ctx, &pb.GetNodeRequest{NodeId: "nope"}); status.Code(err) != codes.NotFound {
		t.Fatalf("unknown node: %v", err)
	}
}
