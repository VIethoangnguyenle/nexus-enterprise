package ngac

import "slices"

// ChannelManagersUAName names the UA of a group's managers. The Managers UA
// sits beside Ch_{id}_Members (never inside it) so managers keep their direct
// user → Members edge and the roster/removal/reconcile rules keep treating
// only direct user children of Members as members.
//
// Managers hold invite on the group's Content OA; all other rights (read,
// write) come through the Members association.
func ChannelManagersUAName(chID ChannelID) string { return "Ch_" + string(chID) + "_Managers" }

// ChannelManagerOps returns operations granted to a group's Managers UA on the
// group's Content OA: invite only. Managers read and write through their
// membership (Members UA), so those operations stay on Members; invite is the
// one right that only managers keep when "Ai được thêm người" is restricted.
func ChannelManagerOps() []string {
	return []string{OpInvite}
}

// ChannelMemberOpsRestricted returns the operations granted to a group's
// Members UA on its Content OA when "Ai được thêm người" (who may add people)
// is restricted to managers only: the normal ChannelMemberOps without invite.
// The input is not modified.
func ChannelMemberOpsRestricted() []string {
	out := make([]string, 0, len(ChannelMemberOps()))
	for _, op := range ChannelMemberOps() {
		if op != OpInvite {
			out = append(out, op)
		}
	}
	return out
}

// The two states of a group's per-setting rules (docs/specs/chat-group-management).
const (
	GroupSettingEveryone = "everyone"
	GroupSettingManagers = "managers"
)

// IsGroupSetting reports whether s is a valid group setting value.
func IsGroupSetting(s string) bool {
	return s == GroupSettingEveryone || s == GroupSettingManagers
}

// InviteRule reads the invite policy from the Members → Content grant:
// restricted when the grant lacks OpInvite.
func InviteRule(membersContentOps []string) string {
	if slices.Contains(membersContentOps, OpInvite) {
		return GroupSettingEveryone
	}
	return GroupSettingManagers
}
