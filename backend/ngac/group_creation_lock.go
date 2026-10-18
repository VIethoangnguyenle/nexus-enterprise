package ngac

import "slices"

// The two states of a workspace's "Ai được tạo nhóm" rule (docs/specs/user-settings).
// The state is never stored: it is read from the graph, as whether the workspace's
// Members UA still holds OpCreateChannel on the Channels OA.
const (
	GroupCreationEveryone = "everyone"
	GroupCreationAdmins   = "admins"
)

// GroupCreationRule names the rule that a Members → Channels grant of ops stands
// for: administrators only when the grant lacks OpCreateChannel (or is gone).
func GroupCreationRule(membersChannelOps []string) string {
	if slices.Contains(membersChannelOps, OpCreateChannel) {
		return GroupCreationEveryone
	}
	return GroupCreationAdmins
}

// LockedChannelOps is what a grant on the Channels OA becomes when group creation
// is locked to administrators: the same grant without OpCreateChannel, in its
// original order. An empty result means the grant is to be removed altogether.
// The input is not modified.
func LockedChannelOps(ops []string) []string {
	out := make([]string, 0, len(ops))
	for _, op := range ops {
		if op != OpCreateChannel {
			out = append(out, op)
		}
	}
	return out
}

// UnlockedMemberChannelOps is what the Members → Channels grant becomes when
// group creation is opened to everyone again: whatever it holds plus
// MemberChannelOps. Only the Members grant is restored this way; custom roles
// are never given the right back automatically.
func UnlockedMemberChannelOps(ops []string) []string {
	out := slices.Clone(ops)
	for _, op := range MemberChannelOps() {
		if !slices.Contains(out, op) {
			out = append(out, op)
		}
	}
	return out
}
