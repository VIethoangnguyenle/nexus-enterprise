package domain_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/ngac"
	policypb "ngac-platform/proto/policy"
	"ngac-platform/services/workspace/internal/domain"
)

// Removing a member takes invite on Mgmt; removing someone who holds manage on any
// area takes manage on Mgmt as well.

func removedAnyone(f *fixture) bool {
	for _, m := range f.write.mutations {
		if strings.HasPrefix(m, "RemoveAssignment ") {
			return true
		}
	}
	return false
}

func TestRemoveMember_ManagerOfAnAreaNeedsManage(t *testing.T) {
	areas := map[string]string{"management": mgmt1, "documents": docs1, "channels": chat1, "assets": assets1}
	for area, oa := range areas {
		t.Run(area, func(t *testing.T) {
			f := newFixture(t)
			f.read.grants[grant{leaver, oa, ngac.OpManage}] = true
			ctx := context.Background()

			// invite alone: refused, nothing detached
			err := f.svc.RemoveMember(ctx, inviter, ws1, leaver)
			assert.ErrorIs(t, err, domain.ErrAccessDenied)
			assert.False(t, removedAnyone(f), "nothing detached")

			// the same removal by a caller who also holds manage on Mgmt
			f.read.grants[grant{inviter, mgmt1, ngac.OpManage}] = true
			require.NoError(t, f.svc.RemoveMember(ctx, inviter, ws1, leaver))
			assert.True(t, removedAnyone(f), "detached")
		})
	}
}

func TestRemoveMember_PlainMemberNeedsOnlyInvite(t *testing.T) {
	f := newFixture(t)
	require.NoError(t, f.svc.RemoveMember(context.Background(), inviter, ws1, leaver))
	assert.True(t, removedAnyone(f))
}

// Removing a member removes a person. The target comes from the client, and the
// detach strips it from every UA of the workspace: a role or department UA, the
// Members or Owners UA, a node of another workspace, or an ID that names nothing
// must all be refused before any edge is touched, whoever asks. Each UA here is
// scripted to sit under Members, so it is the node's type, not its ancestry, that
// refuses it.
func TestRemoveMember_RefusesEverythingButAPersonOfTheWorkspace(t *testing.T) {
	const deptUA = "ua-dept-root-1"
	underMembers := []*policypb.NGACNode{
		node(members1, ngac.MembersUAName(ws1), ngac.TypeUA),
		node(pc1, ngac.PCName(ws1), ngac.TypePC),
	}
	targets := map[string]string{
		"a role UA":                     role1,
		"a department UA":               deptUA,
		"the Members UA":                members1,
		"the Owners UA":                 owners1,
		"a person of another workspace": otherOwner,
		"the Owners UA of another":      owners2,
		"a node that does not exist":    "u-nobody",
		"an empty id":                   "",
	}
	for name, targetID := range targets {
		for _, caller := range []string{inviter, owner} {
			t.Run(name+" by "+caller, func(t *testing.T) {
				f := newFixture(t)
				f.read.nodes[deptUA] = node(deptUA, ngac.DeptUAName("root-1"), ngac.TypeUA)
				f.read.nodes[members1] = node(members1, ngac.MembersUAName(ws1), ngac.TypeUA)
				f.read.nodes[owners1] = node(owners1, ngac.OwnersUAName(ws1), ngac.TypeUA)
				f.read.nodes[owners2] = node(owners2, ngac.OwnersUAName(ws2), ngac.TypeUA)
				for _, ua := range []string{role1, deptUA, owners1} {
					f.read.ancestors[ua] = underMembers
				}

				err := f.svc.RemoveMember(context.Background(), caller, ws1, targetID)

				require.Error(t, err)
				assert.True(t, errors.Is(err, domain.ErrNotFound) || errors.Is(err, domain.ErrInvalidInput),
					"refused as not a person of the workspace, got %v", err)
				assert.False(t, f.mutated(), "no edge touched: %v", f.write.mutations)
			})
		}
	}
}

// The paired allow: the same callers still remove a person of the workspace, and
// only that person's edges go.
func TestRemoveMember_DetachesOnlyThePerson(t *testing.T) {
	for _, caller := range []string{inviter, owner} {
		f := newFixture(t)
		require.NoError(t, f.svc.RemoveMember(context.Background(), caller, ws1, leaver))
		require.NotEmpty(t, f.write.mutations)
		for _, m := range f.write.mutations {
			assert.True(t, strings.HasPrefix(m, "RemoveAssignment "+leaver+"->"), "only the person's edges: %s", m)
		}
	}
}

// Removal acts only on a member, so the edges that make the person one go last:
// a failure on any other edge leaves the Members edge in place, and the retry is
// accepted and finishes the job.
func TestRemoveMember_MembershipGoesLastSoAFailedRemovalCanBeRetried(t *testing.T) {
	f := newFixture(t)
	failing := &failingRemove{fakePolicyWrite: f.write, ua: role1}
	svc := domain.NewService(f.wsStore, f.depts, f.read, failing, nil, nil)

	require.Error(t, svc.RemoveMember(context.Background(), inviter, ws1, leaver))
	assert.NotContains(t, f.write.mutations, "RemoveAssignment "+leaver+"->"+members1, "still a member after the failure")

	f.write.mutations = nil
	require.NoError(t, f.svc.RemoveMember(context.Background(), inviter, ws1, leaver), "the retry is accepted")
	require.NotEmpty(t, f.write.mutations)
	assert.Equal(t, "RemoveAssignment "+leaver+"->"+members1, f.write.mutations[len(f.write.mutations)-1], "Members goes last")
}

func TestRemoveMember_RefusedWhenWhatTheyManageCannotBeChecked(t *testing.T) {
	f := newFixture(t)
	f.read.batchErr = errors.New("policy down")
	err := f.svc.RemoveMember(context.Background(), inviter, ws1, leaver)
	assert.ErrorIs(t, err, domain.ErrUnavailable)
	assert.NotErrorIs(t, err, domain.ErrAccessDenied)
	assert.False(t, removedAnyone(f))

	f.read.batchErr = nil
	require.NoError(t, f.svc.RemoveMember(context.Background(), inviter, ws1, leaver))
}

func anyNodeDeleted(f *fixture) bool {
	for _, m := range f.write.mutations {
		if strings.HasPrefix(m, "DeleteNode ") {
			return true
		}
	}
	return false
}

func TestDeleteFolder_RefusesTheWorkspacesOwnContainers(t *testing.T) {
	for name, oa := range map[string]string{"Mgmt": mgmt1, "Documents": docs1, "Channels": chat1, "Assets": assets1} {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			err := f.svc.DeleteFolder(context.Background(), owner, ws1, oa)
			assert.ErrorIs(t, err, domain.ErrInvalidInput)
			assert.False(t, anyNodeDeleted(f), "nothing deleted")

			// the same caller deletes a real folder of the workspace
			require.NoError(t, f.svc.DeleteFolder(context.Background(), owner, ws1, folder1))
			assert.True(t, anyNodeDeleted(f))
		})
	}
}
