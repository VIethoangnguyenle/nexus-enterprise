package ngac

import "slices"

// Area is a resource area of a workspace: the part of the graph a role can be
// granted rights on, named the way an administrator thinks about it. An area is
// not a node. It maps to the workspace-level OA of the same purpose, and a
// grant on that OA reaches every attribute beneath it (the channels under
// Channels, the folders under Documents, the asset types under Assets).
//
// Screens never hard-code which operations make sense where: they ask the
// workspace service, which answers from AreaOps.
type Area string

const (
	// AreaManagement is the workspace's Mgmt OA: members, roles, departments.
	AreaManagement Area = "management"
	// AreaDocuments is the Documents OA and the drive trees under it.
	AreaDocuments Area = "documents"
	// AreaChannels is the Channels OA and every channel's content under it.
	AreaChannels Area = "channels"
	// AreaAssets is the Assets OA and every category and type under it.
	AreaAssets Area = "assets"
)

// Areas lists every area in the order screens show them.
func Areas() []Area {
	return []Area{AreaDocuments, AreaChannels, AreaAssets, AreaManagement}
}

// IsArea reports whether a is one of the known areas.
func IsArea(a Area) bool { return slices.Contains(Areas(), a) }

// AreaOps returns the operations that mean something on an area, in the
// canonical order of AllOwnerOps. An operation is listed only where a service
// actually checks it on that kind of OA:
//
//   - management: manage (roles, departments, permissions, quotas, approval
//     templates) and invite (add and remove members).
//   - documents:  read, write and share on the drive. An upload is gated on
//     write (CreateFile and ConfirmFile both check it), so `upload` is not
//     offered: no service checks it, and a switch that changes nothing would
//     mislead. Members and channel members are still granted it (harmless).
//   - channels:   read and write messages, create_channel, invite someone into
//     a channel and manage (rename) it.
//   - assets:     read, write and manage assets and their types, approve
//     requests for them.
//
// The caller gets a copy; nil for an unknown area.
func AreaOps(a Area) []string {
	var want []string
	switch a {
	case AreaManagement:
		want = []string{OpManage, OpInvite}
	case AreaDocuments:
		want = []string{OpRead, OpWrite, OpShare}
	case AreaChannels:
		want = []string{OpRead, OpWrite, OpManage, OpInvite, OpCreateChannel}
	case AreaAssets:
		want = []string{OpRead, OpWrite, OpApprove, OpManage}
	default:
		return nil
	}
	// Canonical order, so the answer does not depend on how the list above is written.
	out := make([]string, 0, len(want))
	for _, op := range AllOwnerOps() {
		if slices.Contains(want, op) {
			out = append(out, op)
		}
	}
	return out
}

// AreaOAName is the name of the workspace-level OA an area stands for.
func AreaOAName(a Area, wsID WorkspaceID) (string, bool) {
	switch a {
	case AreaManagement:
		return MgmtOAName(wsID), true
	case AreaDocuments:
		return DocumentsOAName(wsID), true
	case AreaChannels:
		return ChannelsOAName(wsID), true
	case AreaAssets:
		return AssetsOAName(wsID), true
	}
	return "", false
}
