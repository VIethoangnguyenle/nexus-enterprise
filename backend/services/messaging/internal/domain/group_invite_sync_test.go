package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/ngac"
	"ngac-platform/services/messaging/internal/domain"
)

// "Ai được thêm người" lives in two places: the channel row (what the screen
// shows) and the Members → Content grant (what the policy enforces). The grant
// is written on every save of the rule, whatever the row already says, so a
// pair that drifted apart is repaired by saving; and the rule the group shows
// is read from the grant.

// The row says everyone may add people but the grant lost invite (a save whose
// row write failed, or two managers saving at once). The group shows the rule
// the policy enforces, and saving "everyone" again gives the members invite back.
func TestUpdateGroupSettings_SavingTheRuleRepairsADriftedGrant(t *testing.T) {
	w := newGroupWorld(t, ngac.GroupSettingEveryone, ngac.GroupSettingEveryone, ngac.GroupSettingEveryone)
	w.f.write.seedGrant(w.uaID, w.oaID, ngac.ChannelMemberOpsRestricted())

	got, err := w.f.svc.GetGroupManagement(w.ctx(w.owner()), w.chID, userNode)
	require.NoError(t, err)
	assert.Equal(t, ngac.GroupSettingManagers, got.InvitePolicy, "the enforced rule, not the stale row")

	require.NoError(t, w.f.svc.UpdateGroupSettings(w.ctx(w.owner()), w.chID, userNode, "", ngac.GroupSettingEveryone, ""))

	assert.Contains(t, w.f.write.grantWrites, [2]string{w.uaID, w.oaID}, "the grant is written although the row already said everyone")
	got, err = w.f.svc.GetGroupManagement(w.ctx(w.owner()), w.chID, userNode)
	require.NoError(t, err)
	assert.Equal(t, ngac.GroupSettingEveryone, got.InvitePolicy)
	assert.Equal(t, ngac.GroupSettingEveryone, ngac.InviteRule(w.f.write.grants[[2]string{w.uaID, w.oaID}]))
}

// The reverse drift: the row says managers only, the grant still carries invite.
// Saving "managers" again takes invite away.
func TestUpdateGroupSettings_SavingManagersAgainWithdrawsAStrayInvite(t *testing.T) {
	w := newGroupWorld(t, ngac.GroupSettingEveryone, ngac.GroupSettingManagers, ngac.GroupSettingEveryone)
	w.f.write.seedGrant(w.uaID, w.oaID, ngac.ChannelMemberOps())

	require.NoError(t, w.f.svc.UpdateGroupSettings(w.ctx(w.owner()), w.chID, userNode, "", ngac.GroupSettingManagers, ""))

	ops := w.f.write.grants[[2]string{w.uaID, w.oaID}]
	assert.NotContains(t, ops, ngac.OpInvite)
	assert.ElementsMatch(t, ngac.ChannelMemberOpsRestricted(), ops)
}

// Deny: a plain member's save is refused before anything is written, the grant included.
func TestUpdateGroupSettings_PlainMemberWritesNoGrant(t *testing.T) {
	w := newGroupWorld(t, ngac.GroupSettingEveryone, ngac.GroupSettingEveryone, ngac.GroupSettingEveryone)
	p := w.member(t)
	w.f.write.seedGrant(w.uaID, w.oaID, ngac.ChannelMemberOps())

	err := w.f.svc.UpdateGroupSettings(w.ctx(p), w.chID, p.node, "", ngac.GroupSettingManagers, "")

	require.ErrorIs(t, err, domain.ErrAccessDenied)
	assert.Empty(t, w.f.write.grantWrites)
	assert.Contains(t, w.f.write.grants[[2]string{w.uaID, w.oaID}], ngac.OpInvite)
}

// A save that does not touch the rule leaves the grant alone.
func TestUpdateGroupSettings_OtherSettingsLeaveTheGrantAlone(t *testing.T) {
	w := newGroupWorld(t, ngac.GroupSettingEveryone, ngac.GroupSettingEveryone, ngac.GroupSettingEveryone)

	require.NoError(t, w.f.svc.UpdateGroupSettings(w.ctx(w.owner()), w.chID, userNode, ngac.GroupSettingManagers, "", ""))

	assert.Empty(t, w.f.write.grantWrites)
}
