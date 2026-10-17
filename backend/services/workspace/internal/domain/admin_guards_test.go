package domain_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/ngac"
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
