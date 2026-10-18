package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/ngac"
	policypb "ngac-platform/proto/policy"
	"ngac-platform/services/workspace/internal/domain"
)

// lockFixture is the shared workspace with the Channels grants the platform writes:
// Members and Owners on the Channels OA, and two custom roles granted create_channel.
func lockFixture(t *testing.T) *fixture {
	t.Helper()
	f := newFixture(t)
	f.write.applyWrites = true
	f.read.grants[grant{owner, chat1, ngac.OpCreateChannel}] = true
	f.read.assocs[members1] = []*policypb.Association{{Id: "as-m", UaId: members1, OaId: chat1, Operations: ngac.MemberChannelOps()}}
	f.read.assocs[owners1] = []*policypb.Association{{Id: "as-o", UaId: owners1, OaId: chat1, Operations: ngac.OwnerChannelOps()}}
	f.read.assocs[roleRead] = append(f.read.assocs[roleRead], &policypb.Association{Id: "as-rc", UaId: roleRead, OaId: chat1, Operations: []string{ngac.OpCreateChannel}})
	f.read.assocs[roleEdit] = append(f.read.assocs[roleEdit], &policypb.Association{Id: "as-ec", UaId: roleEdit, OaId: chat1, Operations: []string{ngac.OpCreateChannel}})
	return f
}

func holds(f *fixture, ua, oa, op string) bool {
	for _, a := range f.read.assocs[ua] {
		if a.OaId == oa {
			for _, o := range a.Operations {
				if o == op {
					return true
				}
			}
		}
	}
	return false
}

func TestGroupCreationLock_StartsOpenAndIsReadFromTheGraph(t *testing.T) {
	f := lockFixture(t)
	rule, err := f.svc.GroupCreationRule(ctx(), member, ws1)
	require.NoError(t, err)
	assert.Equal(t, ngac.GroupCreationEveryone, rule)

	delete(f.read.assocs, members1)
	rule, err = f.svc.GroupCreationRule(ctx(), member, ws1)
	require.NoError(t, err)
	assert.Equal(t, ngac.GroupCreationAdmins, rule, "no Members grant on Channels is a lock")
}

func TestGroupCreationLock_ReadNeedsMembership(t *testing.T) {
	f := lockFixture(t)
	for _, c := range []string{outsider, otherOwner, ""} {
		_, err := f.svc.GroupCreationRule(ctx(), c, ws1)
		require.ErrorIs(t, err, domain.ErrAccessDenied, "%q", c)
	}
}

func TestGroupCreationLock_NonAdminCannotSetAndGraphIsUntouched(t *testing.T) {
	for _, rule := range []string{ngac.GroupCreationAdmins, ngac.GroupCreationEveryone} {
		for _, c := range nonManagers() {
			f := lockFixture(t)
			_, err := f.svc.SetGroupCreationRule(ctx(), c, ws1, rule)
			require.ErrorIs(t, err, domain.ErrAccessDenied, "%s as %q", rule, c)
			assert.False(t, f.mutated(), "%s as %q wrote", rule, c)
		}
	}
}

func TestGroupCreationLock_PersonalWorkspaceHasNoLocks(t *testing.T) {
	f := lockFixture(t)
	f.wsStore.ws[ws1].Type = domain.TypePersonal
	_, err := f.svc.SetGroupCreationRule(ctx(), owner, ws1, ngac.GroupCreationAdmins)
	require.ErrorIs(t, err, domain.ErrInvalidInput)
	assert.False(t, f.mutated())
}

func TestGroupCreationLock_UnknownRuleIsRefused(t *testing.T) {
	f := lockFixture(t)
	for _, rule := range []string{"", "nobody", "Admins", "members"} {
		_, err := f.svc.SetGroupCreationRule(ctx(), owner, ws1, rule)
		require.ErrorIs(t, err, domain.ErrInvalidInput, "%q", rule)
	}
	assert.False(t, f.mutated())
}

func TestGroupCreationLock_LockRemovesTheRightFromMembersAndEveryCustomRoleButNotOwners(t *testing.T) {
	f := lockFixture(t)
	rule, err := f.svc.SetGroupCreationRule(ctx(), owner, ws1, ngac.GroupCreationAdmins)
	require.NoError(t, err)
	assert.Equal(t, ngac.GroupCreationAdmins, rule)

	assert.False(t, holds(f, members1, chat1, ngac.OpCreateChannel), "Members lost it")
	assert.False(t, holds(f, roleRead, chat1, ngac.OpCreateChannel), "custom role lost it")
	assert.False(t, holds(f, roleEdit, chat1, ngac.OpCreateChannel), "custom role lost it")
	assert.True(t, holds(f, owners1, chat1, ngac.OpCreateChannel), "Owners keep their own")
	assert.True(t, holds(f, roleEdit, docs1, ngac.OpWrite), "what the role holds elsewhere is untouched")

	got, err := f.svc.GroupCreationRule(ctx(), member, ws1)
	require.NoError(t, err)
	assert.Equal(t, ngac.GroupCreationAdmins, got, "state is read back from the graph")
	// Every graph write is a policy-service call, which is the EPP invalidation path.
	for _, m := range f.write.mutations {
		assert.Regexp(t, `^(RemoveAssociation|CreateAssociation) `, m)
		assert.NotContains(t, m, owners1)
	}
}

func TestGroupCreationLock_LockTwiceIsHarmless(t *testing.T) {
	f := lockFixture(t)
	_, err := f.svc.SetGroupCreationRule(ctx(), owner, ws1, ngac.GroupCreationAdmins)
	require.NoError(t, err)
	n := len(f.write.mutations)
	_, err = f.svc.SetGroupCreationRule(ctx(), owner, ws1, ngac.GroupCreationAdmins)
	require.NoError(t, err)
	assert.Equal(t, n, len(f.write.mutations), "nothing left to remove")
}

func TestGroupCreationLock_UnlockRestoresMembersOnly(t *testing.T) {
	f := lockFixture(t)
	_, err := f.svc.SetGroupCreationRule(ctx(), owner, ws1, ngac.GroupCreationAdmins)
	require.NoError(t, err)

	rule, err := f.svc.SetGroupCreationRule(ctx(), owner, ws1, ngac.GroupCreationEveryone)
	require.NoError(t, err)
	assert.Equal(t, ngac.GroupCreationEveryone, rule)
	assert.True(t, holds(f, members1, chat1, ngac.OpCreateChannel), "Members have it again")
	assert.False(t, holds(f, roleRead, chat1, ngac.OpCreateChannel), "custom roles are not restored")
	assert.False(t, holds(f, roleEdit, chat1, ngac.OpCreateChannel), "custom roles are not restored")

	n := len(f.write.mutations)
	_, err = f.svc.SetGroupCreationRule(ctx(), owner, ws1, ngac.GroupCreationEveryone)
	require.NoError(t, err)
	assert.Equal(t, n, len(f.write.mutations), "already open: no write")
}

func TestGroupCreationLock_RolesEndpointRefusesTheRightWhileLocked(t *testing.T) {
	f := lockFixture(t)
	_, err := f.svc.SetGroupCreationRule(ctx(), owner, ws1, ngac.GroupCreationAdmins)
	require.NoError(t, err)
	n := len(f.write.mutations)

	_, err = f.svc.SetRolePermissions(ctx(), owner, ws1, roleRead, ngac.AreaChannels, []string{ngac.OpCreateChannel})
	require.ErrorIs(t, err, domain.ErrGroupCreationLocked)
	assert.Equal(t, n, len(f.write.mutations), "a refusal writes nothing")
	assert.False(t, holds(f, roleRead, chat1, ngac.OpCreateChannel))

	// Other areas, and emptying the Channels area, are unaffected by the lock.
	_, err = f.svc.SetRolePermissions(ctx(), owner, ws1, roleRead, ngac.AreaDocuments, []string{ngac.OpRead, ngac.OpWrite})
	require.NoError(t, err)
	_, err = f.svc.SetRolePermissions(ctx(), owner, ws1, roleRead, ngac.AreaChannels, nil)
	require.NoError(t, err)
}

func TestGroupCreationLock_RolesEndpointGrantsTheRightWhenOpen(t *testing.T) {
	f := lockFixture(t)
	delete(f.read.assocs, roleRead)
	_, err := f.svc.SetRolePermissions(ctx(), owner, ws1, roleRead, ngac.AreaChannels, []string{ngac.OpCreateChannel})
	require.NoError(t, err)
	assert.True(t, holds(f, roleRead, chat1, ngac.OpCreateChannel))
}

func TestGroupCreationLock_RoleDetailShowsTheLock(t *testing.T) {
	f := lockFixture(t)
	d, err := f.svc.GetRoleDetail(ctx(), owner, ws1, roleRead)
	require.NoError(t, err)
	assert.Equal(t, ngac.GroupCreationEveryone, d.GroupCreation)

	_, err = f.svc.SetGroupCreationRule(ctx(), owner, ws1, ngac.GroupCreationAdmins)
	require.NoError(t, err)
	d, err = f.svc.GetRoleDetail(ctx(), owner, ws1, roleRead)
	require.NoError(t, err)
	assert.Equal(t, ngac.GroupCreationAdmins, d.GroupCreation)
}
