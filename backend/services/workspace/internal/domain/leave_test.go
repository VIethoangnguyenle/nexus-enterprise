package domain_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/ngac"
	policypb "ngac-platform/proto/policy"
	"ngac-platform/services/workspace/internal/domain"
	"ngac-platform/services/workspace/internal/store"
)

func TestLeaveWorkspace_AMemberLeavesAndLosesEveryAttribute(t *testing.T) {
	f := newInviteFixture(t)
	f.inv.byNode[member] = "member@novapay.vn"
	f.inv.byID["pending-1"] = &store.Invitation{ID: "pending-1", WorkspaceID: ws1, Email: "member@novapay.vn", Status: store.InvitationPending}
	f.inv.byID["pending-other"] = &store.Invitation{ID: "pending-other", WorkspaceID: ws1, Email: "someone@else.vn", Status: store.InvitationPending}

	require.NoError(t, f.svc.LeaveWorkspace(ctx(), member, ws1))

	assert.Contains(t, f.write.mutations, "RemoveAssignment "+member+"->"+members1, "the Members UA")
	assert.Contains(t, f.write.mutations, "RemoveAssignment "+member+"->"+role1, "a role UA too")
	for _, m := range f.write.mutations {
		assert.NotContains(t, m, "RemoveAssignment "+owner+"->", "nobody else is touched")
	}
	assert.Equal(t, []string{ws1 + "/" + member}, f.dir.removed, "the listing goes too")
	assert.Equal(t, store.InvitationRevoked, f.inv.byID["pending-1"].Status, "an offer to their verified address cannot bring them back")
	assert.Equal(t, store.InvitationPending, f.inv.byID["pending-other"].Status, "other people's offers stay")
}

func TestLeaveWorkspace_NobodyCanMakeSomeoneElseLeave(t *testing.T) {
	// The only person who leaves is the caller: there is no argument to name another.
	f := newAdminFixture(t)
	require.NoError(t, f.svc.LeaveWorkspace(ctx(), leaver, ws1))
	for _, m := range f.write.mutations {
		assert.NotContains(t, m, "RemoveAssignment "+member+"->")
		assert.NotContains(t, m, "RemoveAssignment "+owner+"->")
	}
}

func TestLeaveWorkspace_DeniedToWhoDoesNotBelong(t *testing.T) {
	for _, caller := range []string{outsider, otherOwner, ""} {
		t.Run("caller="+caller, func(t *testing.T) {
			f := newAdminFixture(t)
			err := f.svc.LeaveWorkspace(ctx(), caller, ws1)
			require.Error(t, err)
			assert.ErrorIs(t, err, domain.ErrAccessDenied)
			assert.False(t, f.mutated())
			assert.Empty(t, f.dir.removed)
		})
	}
}

func TestLeaveWorkspace_DeniedWhenThePolicyCannotAnswer(t *testing.T) {
	f := newAdminFixture(t)
	f.read.ancErr = errors.New("policy down")
	err := f.svc.LeaveWorkspace(ctx(), member, ws1)
	require.Error(t, err)
	assert.False(t, f.mutated())
}

func TestLeaveWorkspace_TheLastOwnerStays(t *testing.T) {
	f := newAdminFixture(t)
	f.read.children[owners1] = []*policypb.NGACNode{node(owner, "owner", ngac.TypeU)}

	err := f.svc.LeaveWorkspace(ctx(), owner, ws1)

	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrLastOwner)
	assert.False(t, f.mutated(), "nothing is detached")
	assert.Empty(t, f.dir.removed)
}

func TestLeaveWorkspace_AnOwnerMayLeaveWhenAnotherRemains(t *testing.T) {
	f := newAdminFixture(t) // two Owners in the fixture
	require.NoError(t, f.svc.LeaveWorkspace(ctx(), owner, ws1))
	assert.Contains(t, f.write.mutations, "RemoveAssignment "+owner+"->"+owners1)
}
