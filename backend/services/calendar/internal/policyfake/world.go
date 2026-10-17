// Package policyfake is an in-memory policy service for tests: it records every
// node, edge and removal a service makes, answers reads from them, and decides
// access from the associations written or from explicit grants, so a test states
// exactly who may do what.
package policyfake

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"ngac-platform/ngac"
	policypb "ngac-platform/proto/policy"
)

// World is the fake policy service.
type World struct {
	mu      sync.Mutex
	seq     int
	Nodes   map[string]*policypb.NGACNode // by id
	byName  map[string]*policypb.NGACNode // "<type> <name>"
	Parents map[string][]string
	Assocs  map[[2]string][]string // {ua, oa} -> operations
	grants  map[[3]string]bool     // {user node, object id, op}
	Log     []string
	// FailAt, when non-zero, is the 1-based mutating call that fails.
	FailAt, calls int
	// IDPrefix is put in the ID of every node the world creates, so runs against
	// a shared database do not collide.
	IDPrefix string
	// OnNodeCreated and OnNodeDeleted let a test mirror the nodes into a database
	// whose tables have foreign keys onto them.
	OnNodeCreated func(*policypb.NGACNode) error
	OnNodeDeleted func(id string)
}

// NewWorld returns an empty world.
func NewWorld() *World {
	return &World{
		Nodes: map[string]*policypb.NGACNode{}, byName: map[string]*policypb.NGACNode{},
		Parents: map[string][]string{}, Assocs: map[[2]string][]string{}, grants: map[[3]string]bool{},
	}
}

func (w *World) Seed(id, name, typ string, props map[string]string) *policypb.NGACNode {
	n := &policypb.NGACNode{Id: id, Name: name, NodeType: typ, Properties: props}
	w.Nodes[id] = n
	w.byName[typ+" "+name] = n
	return n
}

func (w *World) Assign(child, parent string) { w.Parents[child] = append(w.Parents[child], parent) }

func (w *World) Allow(user, object string, ops ...string) {
	for _, op := range ops {
		w.grants[[3]string{user, object, op}] = true
	}
}

// nodeNamed returns the id of the live node of this type and name, or "".
func (w *World) NodeNamed(typ, name string) string {
	if n := w.byName[typ+" "+name]; n != nil {
		return n.Id
	}
	return ""
}

// created lists the names of nodes created through the write client, in order.
func (w *World) Created() []string {
	var out []string
	for _, l := range w.Log {
		if strings.HasPrefix(l, "create ") {
			out = append(out, strings.TrimPrefix(l, "create "))
		}
	}
	return out
}

func (w *World) Mutations() int { return w.calls }

func (w *World) step() error {
	w.calls++
	if w.FailAt != 0 && w.calls == w.FailAt {
		return fmt.Errorf("injected failure at call %d", w.calls)
	}
	return nil
}

func (w *World) label(id string) string {
	if n, ok := w.Nodes[id]; ok {
		return n.Name
	}
	return id
}

// allowed decides like a graph would, short of policy classes: an explicit grant,
// or an association from the user or one of its ancestors to the object or to
// one of the object's ancestors that carries the operation.
func (w *World) Allowed(user, object, op string) bool {
	if w.grants[[3]string{user, object, op}] {
		return true
	}
	holders := map[string]bool{user: true}
	for _, a := range w.ancestors(user) {
		holders[a] = true
	}
	targets := map[string]bool{object: true}
	for _, a := range w.ancestors(object) {
		targets[a] = true
	}
	for k, ops := range w.Assocs {
		if !holders[k[0]] || !targets[k[1]] {
			continue
		}
		for _, o := range ops {
			if o == op {
				return true
			}
		}
	}
	return false
}

func (w *World) ancestors(id string) []string {
	seen := map[string]bool{}
	var out []string
	var walk func(string)
	walk = func(n string) {
		for _, p := range w.Parents[n] {
			if !seen[p] {
				seen[p] = true
				out = append(out, p)
				walk(p)
			}
		}
	}
	walk(id)
	return out
}

type writeClient struct {
	policypb.PolicyWriteServiceClient
	w *World
}

// The writer answers the graph reads the services make about who belongs where,
// from the same world.
func (x writeClient) GetNode(ctx context.Context, in *policypb.GetNodeRequest, o ...grpc.CallOption) (*policypb.NGACNode, error) {
	return readClient{w: x.w}.GetNode(ctx, in, o...)
}

func (x writeClient) FindNodeByName(ctx context.Context, in *policypb.FindNodeByNameRequest, o ...grpc.CallOption) (*policypb.NGACNode, error) {
	return readClient{w: x.w}.FindNodeByName(ctx, in, o...)
}

func (x writeClient) IsAssigned(ctx context.Context, in *policypb.IsAssignedRequest, o ...grpc.CallOption) (*policypb.BoolResponse, error) {
	return readClient{w: x.w}.IsAssigned(ctx, in, o...)
}

func (x writeClient) GetAncestors(ctx context.Context, in *policypb.GetAncestorsRequest, o ...grpc.CallOption) (*policypb.NodeList, error) {
	return readClient{w: x.w}.GetAncestors(ctx, in, o...)
}

func (x writeClient) CreateNode(_ context.Context, in *policypb.CreateNodeRequest, _ ...grpc.CallOption) (*policypb.NGACNode, error) {
	x.w.mu.Lock()
	defer x.w.mu.Unlock()
	if err := x.w.step(); err != nil {
		return nil, err
	}
	if _, dup := x.w.byName[in.NodeType+" "+in.Name]; dup {
		return nil, status.Error(codes.AlreadyExists, "node exists")
	}
	x.w.seq++
	n := x.w.Seed(fmt.Sprintf("node-%s%d", x.w.IDPrefix, x.w.seq), in.Name, in.NodeType, in.Properties)
	if x.w.OnNodeCreated != nil {
		if err := x.w.OnNodeCreated(n); err != nil {
			delete(x.w.Nodes, n.Id)
			delete(x.w.byName, n.NodeType+" "+n.Name)
			return nil, err
		}
	}
	x.w.Log = append(x.w.Log, "create "+in.NodeType+" "+in.Name)
	return n, nil
}

func (x writeClient) DeleteNode(_ context.Context, in *policypb.DeleteNodeRequest, _ ...grpc.CallOption) (*policypb.Empty, error) {
	x.w.mu.Lock()
	defer x.w.mu.Unlock()
	if n, ok := x.w.Nodes[in.NodeId]; ok {
		delete(x.w.Nodes, in.NodeId)
		delete(x.w.byName, n.NodeType+" "+n.Name)
		// Deleting a node takes every edge on it, as the policy service does.
		delete(x.w.Parents, in.NodeId)
		for child, parents := range x.w.Parents {
			kept := parents[:0:0]
			for _, p := range parents {
				if p != in.NodeId {
					kept = append(kept, p)
				}
			}
			x.w.Parents[child] = kept
		}
		for k := range x.w.Assocs {
			if k[0] == in.NodeId || k[1] == in.NodeId {
				delete(x.w.Assocs, k)
			}
		}
		x.w.Log = append(x.w.Log, "delete "+n.Name)
		if x.w.OnNodeDeleted != nil {
			x.w.OnNodeDeleted(n.Id)
		}
	}
	return &policypb.Empty{}, nil
}

func (x writeClient) CreateAssignment(_ context.Context, in *policypb.CreateAssignmentRequest, _ ...grpc.CallOption) (*policypb.Assignment, error) {
	x.w.mu.Lock()
	defer x.w.mu.Unlock()
	if err := x.w.step(); err != nil {
		return nil, err
	}
	x.w.Assign(in.ChildId, in.ParentId)
	x.w.Log = append(x.w.Log, fmt.Sprintf("assign %s>%s", x.w.label(in.ChildId), x.w.label(in.ParentId)))
	return &policypb.Assignment{}, nil
}

func (x writeClient) RemoveAssignment(_ context.Context, in *policypb.RemoveAssignmentRequest, _ ...grpc.CallOption) (*policypb.Empty, error) {
	x.w.mu.Lock()
	defer x.w.mu.Unlock()
	if err := x.w.step(); err != nil {
		return nil, err
	}
	kept := x.w.Parents[in.ChildId][:0:0]
	for _, p := range x.w.Parents[in.ChildId] {
		if p != in.ParentId {
			kept = append(kept, p)
		}
	}
	x.w.Parents[in.ChildId] = kept
	x.w.Log = append(x.w.Log, fmt.Sprintf("unassign %s>%s", x.w.label(in.ChildId), x.w.label(in.ParentId)))
	return &policypb.Empty{}, nil
}

func (x writeClient) CreateAssociation(_ context.Context, in *policypb.CreateAssociationRequest, _ ...grpc.CallOption) (*policypb.Association, error) {
	x.w.mu.Lock()
	defer x.w.mu.Unlock()
	if err := x.w.step(); err != nil {
		return nil, err
	}
	x.w.Assocs[[2]string{in.UaId, in.OaId}] = in.Operations
	x.w.Log = append(x.w.Log, fmt.Sprintf("associate %s>%s %v", x.w.label(in.UaId), x.w.label(in.OaId), in.Operations))
	return &policypb.Association{}, nil
}

func (x writeClient) RemoveAssociation(_ context.Context, in *policypb.RemoveAssociationRequest, _ ...grpc.CallOption) (*policypb.Empty, error) {
	x.w.mu.Lock()
	defer x.w.mu.Unlock()
	if err := x.w.step(); err != nil {
		return nil, err
	}
	delete(x.w.Assocs, [2]string{in.UaId, in.OaId})
	x.w.Log = append(x.w.Log, fmt.Sprintf("unassociate %s>%s", x.w.label(in.UaId), x.w.label(in.OaId)))
	return &policypb.Empty{}, nil
}

type readClient struct {
	policypb.PolicyReadServiceClient
	w *World
}

func (x readClient) CheckAccess(_ context.Context, in *policypb.CheckAccessRequest, _ ...grpc.CallOption) (*policypb.AccessDecision, error) {
	x.w.mu.Lock()
	defer x.w.mu.Unlock()
	if x.w.Allowed(in.UserNodeId, in.ObjectNodeId, in.Operation) {
		return &policypb.AccessDecision{Decision: ngac.DecisionAllow}, nil
	}
	return &policypb.AccessDecision{Decision: ngac.DecisionDeny}, nil
}

func (x readClient) BatchCheckAccess(_ context.Context, in *policypb.BatchCheckAccessRequest, _ ...grpc.CallOption) (*policypb.BatchAccessResult, error) {
	x.w.mu.Lock()
	defer x.w.mu.Unlock()
	out := &policypb.BatchAccessResult{Results: map[string]*policypb.ObjectPermissions{}}
	for _, obj := range in.ObjectIds {
		perms := map[string]bool{}
		for _, op := range in.Operations {
			perms[op] = x.w.Allowed(in.UserNodeId, obj, op)
		}
		out.Results[obj] = &policypb.ObjectPermissions{Permissions: perms}
	}
	return out, nil
}

func (x readClient) FindNodeByName(_ context.Context, in *policypb.FindNodeByNameRequest, _ ...grpc.CallOption) (*policypb.NGACNode, error) {
	x.w.mu.Lock()
	defer x.w.mu.Unlock()
	if n := x.w.byName[in.NodeType+" "+in.Name]; n != nil {
		return n, nil
	}
	return nil, status.Errorf(codes.NotFound, "node %s not found", in.Name)
}

func (x readClient) GetNode(_ context.Context, in *policypb.GetNodeRequest, _ ...grpc.CallOption) (*policypb.NGACNode, error) {
	x.w.mu.Lock()
	defer x.w.mu.Unlock()
	if n, ok := x.w.Nodes[in.NodeId]; ok {
		return n, nil
	}
	return nil, status.Errorf(codes.NotFound, "node %s not found", in.NodeId)
}

// IsAssigned reports a direct edge, as the policy service does.
func (x readClient) IsAssigned(_ context.Context, in *policypb.IsAssignedRequest, _ ...grpc.CallOption) (*policypb.BoolResponse, error) {
	x.w.mu.Lock()
	defer x.w.mu.Unlock()
	for _, p := range x.w.Parents[in.ChildId] {
		if p == in.ParentId {
			return &policypb.BoolResponse{Value: true}, nil
		}
	}
	return &policypb.BoolResponse{}, nil
}

func (x readClient) GetAncestors(_ context.Context, in *policypb.GetAncestorsRequest, _ ...grpc.CallOption) (*policypb.NodeList, error) {
	x.w.mu.Lock()
	defer x.w.mu.Unlock()
	out := &policypb.NodeList{}
	for _, id := range x.w.ancestors(in.NodeId) {
		if n, ok := x.w.Nodes[id]; ok {
			out.Nodes = append(out.Nodes, n)
		}
	}
	return out, nil
}

// Read is the world as a policy read client.
func (w *World) Read() policypb.PolicyReadServiceClient { return readClient{w: w} }

// Write is the world as a policy write client.
func (w *World) Write() policypb.PolicyWriteServiceClient { return writeClient{w: w} }
