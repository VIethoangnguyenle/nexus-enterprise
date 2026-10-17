package policyclient

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"

	"google.golang.org/grpc"

	"ngac-platform/ngac"
	policypb "ngac-platform/proto/policy"
)

// GraphReader is the part of the policy read API membership needs. Both
// policypb.PolicyReadServiceClient and PolicyWriteServiceClient satisfy it (the writer
// carries FindNodeByName, IsAssigned, GetNode and GetAncestors for this). A read
// replica may be behind a change made a moment ago, so a caller that decides a write
// from membership, or must see a removal at once, passes the writer.
type GraphReader interface {
	FindNodeByName(ctx context.Context, in *policypb.FindNodeByNameRequest, opts ...grpc.CallOption) (*policypb.NGACNode, error)
	GetAncestors(ctx context.Context, in *policypb.GetAncestorsRequest, opts ...grpc.CallOption) (*policypb.NodeList, error)
	GetNode(ctx context.Context, in *policypb.GetNodeRequest, opts ...grpc.CallOption) (*policypb.NGACNode, error)
	IsAssigned(ctx context.Context, in *policypb.IsAssignedRequest, opts ...grpc.CallOption) (*policypb.BoolResponse, error)
}

// errNotMembersUA means a node is named like a workspace's Members UA but is not
// assigned under that workspace's policy class, so it is not the workspace's.
var errNotMembersUA = errors.New("not the workspace's Members UA")

// membersUAIDs caches workspace id -> id of its "<ws>_Members" UA for the life
// of the process. A node's id never changes (a rename keeps it) and a workspace
// id is never reused, so an entry cannot go stale into a wrong answer; at worst
// a deleted workspace's entry matches nothing. Errors are never stored.
var membersUAIDs sync.Map

// MembersUA returns the node id of the workspace's Members UA.
func MembersUA(ctx context.Context, r GraphReader, ws ngac.WorkspaceID) (string, error) {
	if id, ok := membersUAIDs.Load(ws); ok {
		return id.(string), nil
	}
	n, err := r.FindNodeByName(ctx, &policypb.FindNodeByNameRequest{
		Name: ngac.MembersUAName(ws), NodeType: ngac.TypeUA,
	})
	if err != nil {
		return "", fmt.Errorf("find members UA of workspace %s: %w", ws, err)
	}
	if n.GetId() == "" {
		return "", fmt.Errorf("find members UA of workspace %s: empty node id", ws)
	}
	// The name alone is not proof (a UA anyone can name "<ws>_Members" must not be
	// cached as the workspace's): it must sit directly under the workspace's PC.
	pc, err := r.FindNodeByName(ctx, &policypb.FindNodeByNameRequest{
		Name: ngac.PCName(ws), NodeType: ngac.TypePC,
	})
	if err != nil {
		return "", fmt.Errorf("find policy class of workspace %s: %w", ws, err)
	}
	under, err := r.IsAssigned(ctx, &policypb.IsAssignedRequest{ChildId: n.GetId(), ParentId: pc.GetId()})
	if err != nil {
		return "", fmt.Errorf("check members UA of workspace %s: %w", ws, err)
	}
	if !under.GetValue() {
		return "", fmt.Errorf("workspace %s: %w", ws, errNotMembersUA)
	}
	membersUAIDs.Store(ws, n.GetId())
	return n.GetId(), nil
}

// IsWorkspaceMember reports whether the user is under the workspace's Members UA.
// Owners and TenantMember are assigned beneath it and so count; a channel's UA,
// an event's guest UA, and a role or department UA are not beneath it, so being
// in one of them is not membership. On any error the answer is unknown: the
// caller denies.
func IsWorkspaceMember(ctx context.Context, r GraphReader, userNodeID string, ws ngac.WorkspaceID) (bool, error) {
	if ok, err := isUserNode(ctx, r, userNodeID); err != nil || !ok {
		return false, err
	}
	membersID, err := MembersUA(ctx, r, ws)
	if err != nil {
		return false, err
	}
	ancestors, err := r.GetAncestors(ctx, &policypb.GetAncestorsRequest{NodeId: userNodeID})
	if err != nil {
		return false, fmt.Errorf("ancestors of %s: %w", userNodeID, err)
	}
	for _, n := range ancestors.GetNodes() {
		if n.GetId() == membersID {
			return true, nil
		}
	}
	return false, nil
}

// MemberWorkspaces lists the workspaces the user belongs to, from one ancestor
// lookup. A UA whose name merely looks like "<ws>_Members" is not trusted: it
// counts only if it is the node MembersUA resolves for that workspace, because
// "_Members" names are not unique in the graph.
func MemberWorkspaces(ctx context.Context, r GraphReader, userNodeID string) ([]ngac.WorkspaceID, error) {
	if ok, err := isUserNode(ctx, r, userNodeID); err != nil || !ok {
		return nil, err
	}
	ancestors, err := r.GetAncestors(ctx, &policypb.GetAncestorsRequest{NodeId: userNodeID})
	if err != nil {
		return nil, fmt.Errorf("ancestors of %s: %w", userNodeID, err)
	}
	var out []ngac.WorkspaceID
	for _, n := range ancestors.GetNodes() {
		if n.GetNodeType() != ngac.TypeUA {
			continue
		}
		ws, ok := ngac.WorkspaceIDFromMembersUAName(n.GetName())
		if !ok {
			continue
		}
		id, err := MembersUA(ctx, r, ws)
		if err != nil {
			// A "_Members" lookalike for a workspace that has none: not a membership.
			if IsNotFound(err) || errors.Is(err, errNotMembersUA) {
				continue
			}
			return nil, err
		}
		if id == n.GetId() && !slices.Contains(out, ws) {
			out = append(out, ws)
		}
	}
	return out, nil
}

// SharedWorkspaces lists the workspaces both users belong to.
func SharedWorkspaces(ctx context.Context, r GraphReader, a, b string) ([]ngac.WorkspaceID, error) {
	wa, err := MemberWorkspaces(ctx, r, a)
	if err != nil {
		return nil, err
	}
	wb, err := MemberWorkspaces(ctx, r, b)
	if err != nil {
		return nil, err
	}
	var out []ngac.WorkspaceID
	for _, ws := range wa {
		if slices.Contains(wb, ws) {
			out = append(out, ws)
		}
	}
	return out, nil
}

// isUserNode reports whether the id names a user (U) node. Membership is a
// property of a person: a UA that sits under Members (Owners, a department) is
// not a member and must not be passed off as one. An id that names nothing is
// not a user either; any other failure is an error, never an answer.
func isUserNode(ctx context.Context, r GraphReader, nodeID string) (bool, error) {
	n, err := r.GetNode(ctx, &policypb.GetNodeRequest{NodeId: nodeID})
	if err != nil {
		if IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("node %s: %w", nodeID, err)
	}
	return n.GetNodeType() == ngac.TypeU, nil
}
