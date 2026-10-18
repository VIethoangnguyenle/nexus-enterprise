package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/ngac"
	"ngac-platform/services/messaging/internal/domain"
)

// Nobody takes the owner out of their group. A workspace owner moderates
// through manage and may remove anyone else, but removing the group's owner
// would leave a group nobody can hand over or disband: it is refused with 409
// until ownership has been transferred. The owner leaving is the leave rule's
// own 409.

func TestRemoveMember_WorkspaceOwnerOutsideTheGroupCannotRemoveItsOwner(t *testing.T) {
	w := newGroupWorld(t, ngac.GroupSettingEveryone, ngac.GroupSettingEveryone, ngac.GroupSettingEveryone)
	w.f.read.allow[grant{wsOwnerNode, w.oaID, ngac.OpManage}] = true
	w.f.read.addMember(w.f.wsID, wsOwnerNode)

	err := w.f.svc.RemoveMember(ctxForUser("ws-owner", wsOwnerNode, w.f.wsID), w.chID, wsOwnerNode, userNode)

	require.ErrorIs(t, err, domain.ErrAlreadyExists, "409")
	assert.Contains(t, err.Error(), "transfer ownership first")
	assert.Empty(t, w.f.write.removed, "the owner keeps every edge")
	assert.Equal(t, w.f.userID, w.createdBy(t))
}

func TestRemoveMember_WorkspaceOwnerInsideTheGroupCannotRemoveItsOwner(t *testing.T) {
	w := newGroupWorld(t, ngac.GroupSettingEveryone, ngac.GroupSettingEveryone, ngac.GroupSettingEveryone)
	admin := w.member(t)
	w.f.read.allow[grant{admin.node, w.oaID, ngac.OpManage}] = true

	err := w.f.svc.RemoveMember(w.ctx(admin), w.chID, admin.node, userNode)

	require.ErrorIs(t, err, domain.ErrAlreadyExists)
	assert.Contains(t, err.Error(), "transfer ownership first")
	assert.Empty(t, w.f.write.removed)
}

// Allow: the same workspace owner still removes a member who is not the owner.
func TestRemoveMember_WorkspaceOwnerStillRemovesAPlainMember(t *testing.T) {
	w := newGroupWorld(t, ngac.GroupSettingEveryone, ngac.GroupSettingEveryone, ngac.GroupSettingEveryone)
	p := w.member(t)
	w.f.read.allow[grant{wsOwnerNode, w.oaID, ngac.OpManage}] = true
	w.f.read.addMember(w.f.wsID, wsOwnerNode)

	require.NoError(t, w.f.svc.RemoveMember(ctxForUser("ws-owner", wsOwnerNode, w.f.wsID), w.chID, wsOwnerNode, p.node))

	assert.Contains(t, w.f.write.removed, [2]string{p.node, w.uaID})
}

// After a transfer the former owner is an ordinary member again, and the
// workspace owner may remove them.
func TestRemoveMember_FormerOwnerIsRemovableAfterTheTransfer(t *testing.T) {
	w := newGroupWorld(t, ngac.GroupSettingEveryone, ngac.GroupSettingEveryone, ngac.GroupSettingEveryone)
	heir := w.member(t)
	require.NoError(t, w.f.svc.TransferOwnership(w.ctx(w.owner()), w.chID, userNode, heir.node))
	w.f.read.allow[grant{wsOwnerNode, w.oaID, ngac.OpManage}] = true
	w.f.read.addMember(w.f.wsID, wsOwnerNode)

	require.NoError(t, w.f.svc.RemoveMember(ctxForUser("ws-owner", wsOwnerNode, w.f.wsID), w.chID, wsOwnerNode, userNode))

	assert.Contains(t, w.f.write.removed, [2]string{userNode, w.uaID})
}
