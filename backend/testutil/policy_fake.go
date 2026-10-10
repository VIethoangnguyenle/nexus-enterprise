package testutil

import (
	"context"
	"fmt"
	"sync"

	"google.golang.org/grpc"

	policypb "ngac-platform/proto/policy"
)

// FakePolicyWrite is an in-memory PolicyWriteServiceClient that records every
// node it creates and deletes and can fail on demand, so a test can prove that a
// multi-step provisioning leaves nothing behind wherever it breaks.
//
// Methods it does not implement panic through the nil embedded client; a test
// that reaches one needs it added here rather than a silent no-op.
type FakePolicyWrite struct {
	policypb.PolicyWriteServiceClient

	mu sync.Mutex
	// NextIDs, when set, are handed out as the IDs of the next nodes created, in
	// order. A test whose code writes the ID to a table with a foreign key onto
	// ngac_nodes supplies the ID of a real node this way.
	NextIDs []string

	// FailAt is the 1-based number of the mutating call (CreateNode,
	// CreateAssignment, CreateAssociation, RemoveAssignment) that returns an
	// error; 0 never fails. Deletions are never failed: they are the compensation
	// under test.
	FailAt int

	calls int
	seq   int
	nodes map[string]*policypb.NGACNode // live nodes by ID
	log   []string
}

// NewFakePolicyWrite returns an empty fake.
func NewFakePolicyWrite() *FakePolicyWrite {
	return &FakePolicyWrite{nodes: map[string]*policypb.NGACNode{}}
}

// Calls reports how many mutating calls have been made.
func (f *FakePolicyWrite) Calls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

// Log returns the ordered record of every call, "create <type> <name>",
// "assign <child>><parent>", "associate ...", "delete <name>".
func (f *FakePolicyWrite) Log() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.log...)
}

// LiveNodes returns the "<type> <name>" of every node created and not deleted.
func (f *FakePolicyWrite) LiveNodes() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for _, n := range f.nodes {
		out = append(out, n.NodeType+" "+n.Name)
	}
	return out
}

// Seed registers a node that exists before the code under test runs.
func (f *FakePolicyWrite) Seed(n *policypb.NGACNode) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nodes[n.Id] = n
}

// Node returns the live node with the given ID, if any.
func (f *FakePolicyWrite) Node(id string) (*policypb.NGACNode, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	n, ok := f.nodes[id]
	return n, ok
}

// step counts a mutating call and reports whether it is the one to fail.
func (f *FakePolicyWrite) step() error {
	f.calls++
	if f.FailAt != 0 && f.calls == f.FailAt {
		return fmt.Errorf("injected failure at call %d", f.calls)
	}
	return nil
}

func (f *FakePolicyWrite) CreateNode(_ context.Context, in *policypb.CreateNodeRequest, _ ...grpc.CallOption) (*policypb.NGACNode, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.step(); err != nil {
		return nil, err
	}
	f.seq++
	id := fmt.Sprintf("node-%d", f.seq)
	if len(f.NextIDs) > 0 {
		id, f.NextIDs = f.NextIDs[0], f.NextIDs[1:]
	}
	n := &policypb.NGACNode{Id: id, Name: in.Name, NodeType: in.NodeType, Properties: in.Properties}
	f.nodes[n.Id] = n
	f.log = append(f.log, "create "+in.NodeType+" "+in.Name)
	return n, nil
}

func (f *FakePolicyWrite) DeleteNode(_ context.Context, in *policypb.DeleteNodeRequest, _ ...grpc.CallOption) (*policypb.Empty, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	n, ok := f.nodes[in.NodeId]
	if !ok {
		return &policypb.Empty{}, nil
	}
	delete(f.nodes, in.NodeId)
	f.log = append(f.log, "delete "+n.Name)
	return &policypb.Empty{}, nil
}

func (f *FakePolicyWrite) CreateAssignment(_ context.Context, in *policypb.CreateAssignmentRequest, _ ...grpc.CallOption) (*policypb.Assignment, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.step(); err != nil {
		return nil, err
	}
	f.log = append(f.log, fmt.Sprintf("assign %s>%s", f.label(in.ChildId), f.label(in.ParentId)))
	return &policypb.Assignment{Id: "a", ChildId: in.ChildId, ParentId: in.ParentId}, nil
}

func (f *FakePolicyWrite) RemoveAssignment(_ context.Context, in *policypb.RemoveAssignmentRequest, _ ...grpc.CallOption) (*policypb.Empty, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.step(); err != nil {
		return nil, err
	}
	f.log = append(f.log, fmt.Sprintf("unassign %s>%s", f.label(in.ChildId), f.label(in.ParentId)))
	return &policypb.Empty{}, nil
}

func (f *FakePolicyWrite) CreateAssociation(_ context.Context, in *policypb.CreateAssociationRequest, _ ...grpc.CallOption) (*policypb.Association, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.step(); err != nil {
		return nil, err
	}
	f.log = append(f.log, fmt.Sprintf("associate %s>%s %v", f.label(in.UaId), f.label(in.OaId), in.Operations))
	return &policypb.Association{Id: "x", UaId: in.UaId, OaId: in.OaId, Operations: in.Operations}, nil
}

// label names a node for the log; an ID the fake does not know stands for itself.
func (f *FakePolicyWrite) label(id string) string {
	if n, ok := f.nodes[id]; ok {
		return n.Name
	}
	return id
}
