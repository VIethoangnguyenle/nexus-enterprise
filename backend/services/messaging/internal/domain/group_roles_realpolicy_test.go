//go:build realpolicy

package domain_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/ngac"
	"ngac-platform/pkg/policyclient"
	policypb "ngac-platform/proto/policy"
	"ngac-platform/services/messaging/internal/domain"
	"ngac-platform/testutil"
)

// managersUA is the group's Managers UA in the real graph, "" when it has none.
func (w *rpWorld) managersUA(chID string) string {
	w.t.Helper()
	n, err := w.read.FindNodeByName(testutil.PolicyAdmin(context.Background()), &policypb.FindNodeByNameRequest{
		Name: ngac.ChannelManagersUAName(ngac.ChannelID(chID)), NodeType: ngac.TypeUA,
	})
	if policyclient.IsNotFound(err) {
		return ""
	}
	require.NoError(w.t, err)
	return n.GetId()
}

// The roles of a group, decided by the real policy service: the creator is born
// a manager; a manager adds people when only managers may; nobody demotes the
// owner; removing a manager withdraws both edges; disbanding takes the Managers
// UA with it.
func TestRealPolicy_GroupRolesLiveInTheGraph(t *testing.T) {
	w := newRPWorld(t)
	g := w.createGroup(w.creator, "rp-roles")
	owner := w.as(w.creator)
	managers := w.managersUA(g.Id)
	require.NotEmpty(t, managers, "a new group has its Managers UA")
	assert.True(t, w.assigned(w.creator.node, managers), "the owner is a manager")

	require.NoError(t, w.svc.AddMember(owner, g.Id, w.creator.node, w.member.node))
	require.NoError(t, w.svc.AddMember(owner, g.Id, w.creator.node, w.latecomer.node))
	require.NoError(t, w.svc.UpdateGroupSettings(owner, g.Id, w.creator.node, "", ngac.GroupSettingManagers, ""))

	// Restricted adding: the plain member's invite is gone, a manager's is not.
	bystander := w.person(w.g.Members)
	err := w.svc.AddMember(w.as(w.latecomer), g.Id, w.latecomer.node, bystander.node)
	assert.ErrorIs(t, err, domain.ErrAccessDenied, "a plain member adds nobody while adding is restricted")
	require.NoError(t, w.svc.AppointManager(owner, g.Id, w.creator.node, w.member.node))
	require.NoError(t, w.svc.AddMember(w.as(w.member), g.Id, w.member.node, bystander.node), "a manager adds people")

	// Nobody demotes the owner, a manager included.
	err = w.svc.RemoveManager(w.as(w.member), g.Id, w.member.node, w.creator.node)
	assert.ErrorIs(t, err, domain.ErrInvalidInput)
	assert.True(t, w.assigned(w.creator.node, managers))

	// Removing a manager withdraws both edges; they then read nothing and add nobody.
	require.NoError(t, w.svc.RemoveMember(owner, g.Id, w.creator.node, w.member.node))
	assert.False(t, w.assigned(w.member.node, managers))
	assert.False(t, w.inChannel(w.member, g))
	_, err = w.svc.GetChannel(w.as(w.member), g.Id, w.member.node)
	assert.ErrorIs(t, err, domain.ErrAccessDenied)

	// A manager cannot disband; the owner can, and the Managers UA goes too.
	require.NoError(t, w.svc.AppointManager(owner, g.Id, w.creator.node, w.latecomer.node))
	assert.ErrorIs(t, w.svc.DisbandGroup(w.as(w.latecomer), g.Id, w.latecomer.node), domain.ErrAccessDenied)
	require.NoError(t, w.svc.DisbandGroup(owner, g.Id, w.creator.node))
	assert.Empty(t, w.managersUA(g.Id), "no Managers UA outlives its group")
}
