package provision

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	policypb "ngac-platform/proto/policy"
)

// Writer is the part of the policy write client provisioning uses.
type Writer interface {
	NodeDeleter
	CreateNode(ctx context.Context, in *policypb.CreateNodeRequest, opts ...grpc.CallOption) (*policypb.NGACNode, error)
	CreateAssignment(ctx context.Context, in *policypb.CreateAssignmentRequest, opts ...grpc.CallOption) (*policypb.Assignment, error)
	CreateAssociation(ctx context.Context, in *policypb.CreateAssociationRequest, opts ...grpc.CallOption) (*policypb.Association, error)
}

// Finder is the part of the policy read client EnsureNode uses.
type Finder interface {
	FindNodeByName(ctx context.Context, in *policypb.FindNodeByNameRequest, opts ...grpc.CallOption) (*policypb.NGACNode, error)
}

// Creator performs the graph writes of one provisioning run and remembers how to
// undo the nodes it created. Every write goes through the policy service, so each
// one takes the EPP invalidation path; the rollback's deletions do as well.
//
// A Creator is single-use and single-goroutine: build one per provisioning call,
// call Fail on the first error, and Done once everything has succeeded.
type Creator struct {
	w  Writer
	rb Rollback
}

// NewCreator returns a Creator writing through w.
func NewCreator(w Writer) *Creator { return &Creator{w: w} }

// Node creates a node and records its removal. The error is the policy
// service's, unwrapped; wrap it with what was being created and pass the result
// to Fail.
func (c *Creator) Node(ctx context.Context, req *policypb.CreateNodeRequest) (*policypb.NGACNode, error) {
	n, err := c.w.CreateNode(ctx, req)
	if err != nil {
		return nil, err
	}
	c.rb.NodeCreated(c.w, n.GetId(), req.GetNodeType()+" "+req.GetName())
	return n, nil
}

// EnsureNode returns the node of this name and type if it already exists, and
// creates it otherwise — so a provisioning that is run again after a partial
// failure continues instead of duplicating. Only a positive NotFound counts as
// absent: any other lookup failure is returned, because treating an outage as
// "does not exist" is how a second copy of a node gets created. Only a node this
// call created is rolled back; one that was already there is not ours to delete.
//
// Names are matched exactly, so the name must be ID-keyed (backend/ngac helpers);
// a name two tenants can share would hand one of them the other's node.
// A create that fails is followed by one more lookup (see below), so two runs
// racing for the same name end up with the same node.
func (c *Creator) EnsureNode(ctx context.Context, f Finder, req *policypb.CreateNodeRequest) (*policypb.NGACNode, error) {
	found, err := f.FindNodeByName(ctx, &policypb.FindNodeByNameRequest{Name: req.GetName(), NodeType: req.GetNodeType()})
	switch {
	case err == nil && found.GetId() != "":
		return found, nil
	case err != nil && status.Code(err) != codes.NotFound:
		return nil, fmt.Errorf("look up %s %q: %w", req.GetNodeType(), req.GetName(), err)
	}
	n, createErr := c.Node(ctx, req)
	if createErr == nil {
		return n, nil
	}
	// The create can lose a race: another run created the node between the lookup
	// and the write, and the graph holds one node per platform name. Look again;
	// a node that is there now is the one to use, and is not ours to delete.
	again, err := f.FindNodeByName(ctx, &policypb.FindNodeByNameRequest{Name: req.GetName(), NodeType: req.GetNodeType()})
	if err == nil && again.GetId() != "" {
		return again, nil
	}
	return nil, createErr
}

// Assign adds a containment edge. Edges need no recorded undo: deleting either
// node a run created removes every edge on it, and an edge between two nodes it
// did not create is not something a run adds.
func (c *Creator) Assign(ctx context.Context, childID, parentID string) error {
	_, err := c.w.CreateAssignment(ctx, &policypb.CreateAssignmentRequest{ChildId: childID, ParentId: parentID})
	return err
}

// Associate adds a permission edge; see Assign for why it needs no undo.
func (c *Creator) Associate(ctx context.Context, uaID, oaID string, ops []string) error {
	_, err := c.w.CreateAssociation(ctx, &policypb.CreateAssociationRequest{UaId: uaID, OaId: oaID, Operations: ops})
	return err
}

// Fail removes everything this run created, newest first, and returns cause.
func (c *Creator) Fail(ctx context.Context, cause error) error { return c.rb.Fail(ctx, cause) }

// Done marks the provisioning complete; nothing is rolled back afterwards.
func (c *Creator) Done() { c.rb.Discard() }
