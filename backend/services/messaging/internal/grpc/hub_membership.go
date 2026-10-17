package grpc

import (
	"context"

	"ngac-platform/ngac"
	"ngac-platform/pkg/policyclient"
)

// MembershipChecker answers whether a person belongs to a workspace in the graph
// right now. An error is an unknown, never a yes.
type MembershipChecker interface {
	IsWorkspaceMember(ctx context.Context, userNodeID, workspaceID string) (bool, error)
}

// PolicyMembership asks the policy service: the person is a member when they sit
// under the workspace's Members UA. Give it the policy writer's connection, not the
// read replica, so a removal made a moment ago is already in the answer.
type PolicyMembership struct{ graph policyclient.GraphReader }

// NewPolicyMembership builds the checker over a policy client.
func NewPolicyMembership(graph policyclient.GraphReader) *PolicyMembership {
	return &PolicyMembership{graph: graph}
}

// IsWorkspaceMember implements MembershipChecker.
func (p *PolicyMembership) IsWorkspaceMember(ctx context.Context, userNodeID, workspaceID string) (bool, error) {
	return policyclient.IsWorkspaceMember(ctx, p.graph, userNodeID, ngac.WorkspaceID(workspaceID))
}
