package domain

import (
	"context"
	"fmt"
	"log/slog"
	"sort"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"ngac-platform/ngac"
	policypb "ngac-platform/proto/policy"
)

// teardownStore is what DeleteWorkspace needs of the store beyond
// WorkspaceStore. It is a separate interface so the compensation path does not
// widen the one every other operation depends on.
type teardownStore interface {
	WorkspaceCreator(ctx context.Context, id string) (userID string, found bool, err error)
	OtherMembers(ctx context.Context, id, userID string) (int, error)
	PurgeWorkspace(ctx context.Context, id string) error
}

// DeleteWorkspace undoes a workspace that was just created and should not stay.
// It exists for compensation: the auth service calls it when provisioning fails
// after CreateWorkspace. There is no REST route.
//
// Who may: the caller (always the verified gRPC caller, never the request body)
// must be an owner of the workspace, or, if its Owners attribute is already
// gone, the person who created it; and nobody else may belong to it. A
// workspace other people have joined is not a thing to roll back.
//
// What it removes: the rows (membership, drive, channels, departments, approval
// schema, the workspace itself), the MinIO bucket, and every attribute under the
// workspace's policy class and the policy class itself, newest first through
// the policy service so each deletion takes the EPP path. It removes the people
// nodes' assignments with the attributes and leaves the people. It is
// idempotent: when nothing is left it succeeds.
func (s *Service) DeleteWorkspace(ctx context.Context, callerNodeID, callerUserID, wsID string) error {
	if callerNodeID == "" || callerUserID == "" {
		return fmt.Errorf("%w: delete workspace requires an authenticated caller", ErrAccessDenied)
	}
	if wsID == "" {
		return fmt.Errorf("%w: workspace id required", ErrInvalidInput)
	}
	ts, ok := s.store.(teardownStore)
	if !ok {
		return fmt.Errorf("workspace store cannot tear a workspace down")
	}

	creator, rowFound, err := ts.WorkspaceCreator(ctx, wsID)
	if err != nil {
		return err
	}
	pcID, err := s.policyClassOf(ctx, wsID, rowFound)
	if err != nil {
		return err
	}
	if !rowFound && pcID == "" {
		return nil // nothing left: a second call, or it never got that far
	}

	var below []*policypb.NGACNode
	if pcID != "" {
		desc, err := s.policyWrite.GetDescendants(ctx, &policypb.GetDescendantsRequest{NodeId: pcID})
		if err != nil {
			return fmt.Errorf("resolve workspace nodes: %w", err)
		}
		below = desc.GetNodes()
	}

	if err := s.authorizeTeardown(ctx, wsID, callerNodeID, callerUserID, creator, rowFound, below); err != nil {
		return err
	}
	if rowFound {
		others, err := ts.OtherMembers(ctx, wsID, callerUserID)
		if err != nil {
			return err
		}
		if others > 0 {
			return fmt.Errorf("%w: other people belong to this workspace", ErrAccessDenied)
		}
	}

	// Rows first: the workspace row points at the policy class, and the graph
	// cannot lose its class while a row still names it.
	if err := ts.PurgeWorkspace(ctx, wsID); err != nil {
		return err
	}
	s.removeBucket(ctx, wsID)

	if pcID == "" {
		return nil
	}
	return s.deleteGraph(ctx, pcID, below)
}

// policyClassOf finds the workspace's policy class: from its row when there is
// one, otherwise by its ID-keyed name. Empty means there is none.
func (s *Service) policyClassOf(ctx context.Context, wsID string, rowFound bool) (string, error) {
	if rowFound {
		if ws, err := s.store.GetByID(ctx, wsID); err == nil && ws.NGACPcID != "" {
			return ws.NGACPcID, nil
		}
	}
	n, err := s.policyRead.FindNodeByName(ctx, &policypb.FindNodeByNameRequest{
		Name: ngac.PCName(ngac.WorkspaceID(wsID)), NodeType: ngac.TypePC,
	})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return "", nil
		}
		return "", fmt.Errorf("look up workspace policy class: %w", err)
	}
	return n.GetId(), nil
}

func (s *Service) authorizeTeardown(ctx context.Context, wsID, callerNodeID, callerUserID, creator string, rowFound bool, below []*policypb.NGACNode) error {
	// Only the caller may be a person in it.
	for _, n := range below {
		if n.GetNodeType() == ngac.TypeU && n.GetId() != callerNodeID {
			return fmt.Errorf("%w: other people belong to this workspace", ErrAccessDenied)
		}
	}

	ownersName := ngac.OwnersUAName(ngac.WorkspaceID(wsID))
	for _, n := range below {
		if n.GetNodeType() == ngac.TypeUA && n.GetName() == ownersName {
			kids, err := s.policyWrite.GetChildren(ctx, &policypb.GetChildrenRequest{NodeId: n.GetId()})
			if err != nil {
				return fmt.Errorf("%w: cannot resolve owners", ErrAccessDenied)
			}
			if containsNode(kids.GetNodes(), callerNodeID) {
				return nil
			}
			return fmt.Errorf("%w: only an owner removes a workspace", ErrAccessDenied)
		}
	}

	// No Owners attribute: a teardown that was interrupted, or a creation that
	// never finished. The creator may finish it; with no row and no people
	// there is no one it could belong to, and the ID cannot be guessed.
	if rowFound {
		if creator == callerUserID {
			return nil
		}
		return fmt.Errorf("%w: only an owner removes a workspace", ErrAccessDenied)
	}
	return nil
}

// deleteGraph removes the workspace's attributes and its policy class, newest
// first. Every deletion is a policy-service write, so each one takes the EPP
// invalidation path, exactly like the compensation in pkg/provision. People are never deleted: only their place under the attributes.
func (s *Service) deleteGraph(ctx context.Context, pcID string, below []*policypb.NGACNode) error {
	doomed := make([]*policypb.NGACNode, 0, len(below)+1)
	for _, n := range below {
		if n.GetNodeType() != ngac.TypeU {
			doomed = append(doomed, n)
		}
	}
	doomed = append(doomed, &policypb.NGACNode{Id: pcID, Name: "policy class", NodeType: ngac.TypePC})

	// The class was created first and the people's attributes after it; a node
	// without a time sorts as the oldest, and equal times keep their order.
	sort.SliceStable(doomed, func(i, j int) bool { return nodeTime(doomed[i]) < nodeTime(doomed[j]) })
	// The class is the oldest of all whatever the clock says.
	sort.SliceStable(doomed, func(i, j int) bool {
		return doomed[i].GetNodeType() == ngac.TypePC && doomed[j].GetNodeType() != ngac.TypePC
	})

	// Stop at the first failure: the class is the last to go, so it stays and a
	// retry can still find the workspace by it. Continuing past a failure would
	// delete the class and strand the attributes that could not be removed,
	// where nothing would ever find them again.
	for i := len(doomed) - 1; i >= 0; i-- {
		n := doomed[i]
		_, err := s.policyWrite.DeleteNode(ctx, &policypb.DeleteNodeRequest{NodeId: n.GetId()})
		if err != nil && status.Code(err) != codes.NotFound {
			slog.Error("workspace teardown incomplete", "node", n.GetId(), "error", err)
			return fmt.Errorf("remove %s %q: %w", n.GetNodeType(), n.GetName(), err)
		}
	}
	return nil
}

func nodeTime(n *policypb.NGACNode) int64 {
	if t := n.GetCreatedAt(); t != nil {
		return t.AsTime().UnixNano()
	}
	return 0
}

// removeBucket drops the workspace's MinIO bucket. Best effort: a bucket that
// still holds objects, or storage that is down, is logged and left, because the
// workspace itself is already gone.
func (s *Service) removeBucket(ctx context.Context, wsID string) {
	if s.minioClient == nil {
		return
	}
	name := fmt.Sprintf("ws-%s", wsID)
	if err := s.minioClient.RemoveBucket(ctx, name); err != nil {
		slog.Warn("could not remove workspace bucket", "bucket", name, "error", err)
	}
}
