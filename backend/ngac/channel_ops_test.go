package ngac_test

import (
	"slices"
	"testing"

	"ngac-platform/ngac"
)

// Groups are private: nothing granted on the workspace's Channels OA may reach a
// group's messages, so no workspace-wide grant there carries read, write or
// invite. A grant on Channels is inherited by every content OA beneath it.
func TestWorkspaceGrantsOnChannelsNeverReachGroupContent(t *testing.T) {
	for _, op := range []string{ngac.OpRead, ngac.OpWrite, ngac.OpInvite, ngac.OpUpload, ngac.OpShare, ngac.OpApprove} {
		if slices.Contains(ngac.MemberChannelOps(), op) {
			t.Errorf("members hold %q on Channels: every group would be readable by the whole workspace", op)
		}
		if slices.Contains(ngac.AreaOps(ngac.AreaChannels), op) {
			t.Errorf("the channels area offers %q: a custom role would reach every group", op)
		}
	}
}

// What stays: any member may start a group, and a custom role may be granted
// exactly that.
func TestMembersAndCustomRolesMayStillCreateGroups(t *testing.T) {
	if !slices.Equal(ngac.MemberChannelOps(), []string{ngac.OpCreateChannel}) {
		t.Errorf("members on Channels: got %v, want only create_channel", ngac.MemberChannelOps())
	}
	if !slices.Equal(ngac.AreaOps(ngac.AreaChannels), []string{ngac.OpCreateChannel}) {
		t.Errorf("channels area: got %v, want only create_channel", ngac.AreaOps(ngac.AreaChannels))
	}
}

// Owners moderate groups they are not in (rename, remove people) but do not read
// them: manage and create_channel, nothing else.
func TestOwnersOnChannelsModerateButDoNotRead(t *testing.T) {
	got := ngac.OwnerChannelOps()
	if !slices.Equal(got, []string{ngac.OpManage, ngac.OpCreateChannel}) {
		t.Fatalf("owners on Channels: got %v, want manage and create_channel", got)
	}
	for _, op := range []string{ngac.OpRead, ngac.OpWrite, ngac.OpInvite} {
		if slices.Contains(got, op) {
			t.Errorf("owners hold %q on Channels: every group would be readable by owners", op)
		}
	}
	got[0] = "tampered"
	if ngac.OwnerChannelOps()[0] == "tampered" {
		t.Error("OwnerChannelOps must not share its slice")
	}
}
