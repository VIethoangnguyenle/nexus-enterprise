//go:build realpolicy

package domain_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	policypb "ngac-platform/proto/policy"
	"ngac-platform/services/messaging/internal/domain"
	"ngac-platform/testutil"
)

// Putting someone into a group puts them under a UA that sits beneath the
// workspace's policy class, so what AddMember accepts as a target decides who the
// workspace's other services take for a member. The target must be a person who is
// already under the workspace's Members UA; a group's own UA, another workspace's
// people, and a UA passed as a person are not.

// A group member adds an active member of the workspace; the same caller, in the
// same group, is refused every target that is not one.
func TestRealPolicy_AddMemberTargetMustBeAnActiveWorkspaceMember(t *testing.T) {
	w := newRPWorld(t)
	g := w.createGroup(w.creator, "rp-target")
	other := w.createGroup(w.creator, "rp-other-group")
	ctx := w.as(w.creator)

	// Allow: an active member of this workspace.
	require.NoError(t, w.svc.AddMember(ctx, g.Id, w.creator.node, w.member.node))
	_, err := w.svc.GetChannel(w.as(w.member), g.Id, w.member.node)
	require.NoError(t, err, "the member added to the group reads it")

	// Deny: a person of another workspace, who would become a member of this one
	// by sitting under a UA beneath its policy class.
	err = w.svc.AddMember(ctx, g.Id, w.creator.node, w.outsider.node)
	assert.ErrorIs(t, err, domain.ErrAccessDenied, "a person of another tenant")
	_, err = w.svc.GetChannel(w.as(w.outsider), g.Id, w.outsider.node)
	assert.ErrorIs(t, err, domain.ErrAccessDenied)
	assert.False(t, w.inWorkspace(w.outsider), "and the workspace did not gain a member")

	// Deny: a UA passed as a person (another workspace's Members UA would put the
	// whole of that workspace under this group, hence under this workspace).
	foreign := testutil.SeedWorkspaceGraph(t, w.write)
	for name, ua := range map[string]string{
		"another workspace's Members UA": foreign.Members,
		"this workspace's Members UA":    w.g.Members,
		"this workspace's Owners UA":     w.g.Owners,
		"another group's UA":             other.NgacUaId,
	} {
		err = w.svc.AddMember(ctx, g.Id, w.creator.node, ua)
		assert.ErrorIs(t, err, domain.ErrInvalidInput, name)
	}
	err = w.svc.AddMember(ctx, g.Id, w.creator.node, "no-such-node")
	assert.ErrorIs(t, err, domain.ErrInvalidInput, "a node that does not exist")
	assert.False(t, w.assigned(foreign.Members, g.NgacUaId), "no edge was written for a refused target")

	// Deny: a person who reaches the workspace only through another group's UA.
	chanOnly := w.person(other.NgacUaId)
	require.False(t, w.inWorkspace(chanOnly))
	err = w.svc.AddMember(ctx, g.Id, w.creator.node, chanOnly.node)
	assert.ErrorIs(t, err, domain.ErrAccessDenied, "a channel UA is not the workspace's membership")

	// The same person, once a member, is accepted: the fixture decides, not the person.
	testutil.Assign(t, w.write, chanOnly.node, w.g.Members)
	require.NoError(t, w.svc.AddMember(ctx, g.Id, w.creator.node, chanOnly.node))
}

// Removing someone from the workspace is not undone by a group: a person detached
// from the Members UA who still sits in a group cannot be added to another.
func TestRealPolicy_AddMemberCannotReverseARemoval(t *testing.T) {
	w := newRPWorld(t)
	first := w.createGroup(w.creator, "rp-first")
	second := w.createGroup(w.creator, "rp-second")
	ctx := w.as(w.creator)

	require.NoError(t, w.svc.AddMember(ctx, first.Id, w.creator.node, w.member.node))
	require.NoError(t, w.svc.AddMember(ctx, second.Id, w.creator.node, w.member.node), "while a member, they can be added")

	// Removal detaches them from the Members UA; the group edges are what is left.
	testutil.Unassign(t, w.write, w.member.node, w.g.Members)
	require.False(t, w.inWorkspace(w.member))
	third := w.createGroup(w.creator, "rp-third")
	err := w.svc.AddMember(ctx, third.Id, w.creator.node, w.member.node)
	assert.ErrorIs(t, err, domain.ErrAccessDenied, "a removed person is not brought back by a group")
	assert.False(t, w.inWorkspace(w.member), "the workspace did not take them back")
	assert.False(t, w.assigned(w.member.node, third.NgacUaId))

	// A member who stayed is added to the same group.
	require.NoError(t, w.svc.AddMember(ctx, third.Id, w.creator.node, w.latecomer.node))
}

// inWorkspace asks the real policy whether the person is under the workspace's Members UA.
func (w *rpWorld) inWorkspace(p rpPerson) bool {
	w.t.Helper()
	anc, err := w.read.GetAncestors(testutil.PolicyAdmin(context.Background()), &policypb.GetAncestorsRequest{NodeId: p.node})
	require.NoError(w.t, err)
	for _, n := range anc.GetNodes() {
		if n.GetId() == w.g.Members {
			return true
		}
	}
	return false
}

func (w *rpWorld) assigned(child, parent string) bool {
	w.t.Helper()
	res, err := w.read.IsAssigned(testutil.PolicyAdmin(context.Background()), &policypb.IsAssignedRequest{ChildId: child, ParentId: parent})
	require.NoError(w.t, err)
	return res.GetValue()
}
