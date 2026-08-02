// Package ngac provides a single source of truth for NGAC operations,
// well-known node names, and naming conventions used across all services.
// Every string that appears in policy API calls should originate from this package
// so that renames and additions are caught at compile time.
package ngac

import (
	"fmt"
	"strings"
)

// --- Operations ---
// These constants represent the access rights checked by the Policy Service.
// Add new operations here instead of sprinkling raw strings through service code.
const (
	OpRead          = "read"
	OpWrite         = "write"
	OpUpload        = "upload"
	OpApprove       = "approve"
	OpShare         = "share"
	OpManage        = "manage"
	OpInvite        = "invite"
	OpCreateChannel = "create_channel"
)

// AllOwnerOps returns the full set of operations granted to workspace owners.
func AllOwnerOps() []string {
	return []string{
		OpRead, OpWrite, OpApprove, OpUpload,
		OpShare, OpManage, OpInvite, OpCreateChannel,
	}
}

// MemberChannelOps returns operations granted to regular members on channels.
func MemberChannelOps() []string {
	return []string{OpRead, OpWrite, OpCreateChannel}
}

// ChannelMemberOps returns operations granted to channel members on content.
//
// OpInvite is included so that being in a channel is what lets you bring
// someone else into it. Membership changes are authorized against the channel's
// own Content OA, so this grant is scoped to that one channel — it confers
// nothing on any other channel, and nothing at the workspace level.
func ChannelMemberOps() []string {
	return []string{OpRead, OpWrite, OpInvite}
}

// ChannelDriveOps returns operations granted to channel members on their drive.
func ChannelDriveOps() []string {
	return []string{OpRead, OpWrite, OpUpload}
}

// MemberDocumentOps returns operations granted to workspace members on the
// Documents tree.
//
// Members hold write, not only read, because the drive gates file creation on
// write: uploading is CreateFile followed by ConfirmFile, and both check write
// on the destination folder. A read-only member could open the drive and see
// the Upload button but never complete an upload.
//
// This matches ChannelDriveOps — a member already has these rights on any
// channel drive they belong to, so withholding them on the workspace drive was
// the odd case rather than the careful one.
//
// Note what write carries in this model: permissions attach to the folder OA,
// not to individual files, so a member may also rename, move and delete files
// in Documents including ones they did not create. Narrowing that needs
// per-item attributes, not a smaller grant here.
func MemberDocumentOps() []string {
	return []string{OpRead, OpWrite, OpUpload}
}

// --- Well-known node names ---
// These are global NGAC nodes that must exist in the policy graph.
const (
	NodePCGlobal    = "PC_Global"
	NodePublicUsers = "PublicUsers"

	// NodePCAssetManagement is the policy class every workspace's asset tree
	// hangs under, in addition to the workspace's own PC.
	NodePCAssetManagement = "PC_AssetManagement"
)

// --- Asset naming conventions ---
//
// Named by workspace ID, not workspace name. Two workspaces are free to share a
// display name, and a name-derived node would then be shared between them —
// which in a graph that answers access questions means one tenant's assets
// resolving onto another's attributes.

func AssetsOAName(wsID string) string { return fmt.Sprintf("%s_Assets", wsID) }

func AssetCategoryOAName(wsID, category string) string {
	return fmt.Sprintf("%s_Category_%s", wsID, category)
}

func AssetTypeOAName(wsID, typeName string) string {
	return fmt.Sprintf("%s_Type_%s", wsID, typeName)
}

// AssetNodeName names the object node for a single asset.
func AssetNodeName(assetID string) string { return fmt.Sprintf("Asset_%s", assetID) }

// --- Workspace naming conventions ---
// Every workspace creates a set of NGAC nodes named by workspace ID.
// Using ID (UUID) instead of display name prevents collisions.

func PCName(wsID string) string             { return fmt.Sprintf("PC_%s", wsID) }
func OwnersUAName(wsID string) string       { return fmt.Sprintf("%s_Owners", wsID) }
func MembersUAName(wsID string) string      { return fmt.Sprintf("%s_Members", wsID) }
func MgmtOAName(wsID string) string         { return fmt.Sprintf("%s_Mgmt", wsID) }
func DocumentsOAName(wsID string) string    { return fmt.Sprintf("%s_Documents", wsID) }
func DraftDocsOAName(wsID string) string    { return fmt.Sprintf("%s_DraftDocs", wsID) }
func ApprovedDocsOAName(wsID string) string { return fmt.Sprintf("%s_ApprovedDocs", wsID) }
func ChannelsOAName(wsID string) string     { return fmt.Sprintf("%s_Channels", wsID) }

// --- Department naming conventions ---

func DeptUAName(name string) string { return fmt.Sprintf("Dept_%s", name) }

// --- Channel naming conventions ---

func ChannelContentOAName(chID string) string { return fmt.Sprintf("Ch_%s_Content", chID) }
func ChannelMembersUAName(chID string) string { return fmt.Sprintf("Ch_%s_Members", chID) }
func ChannelDriveName(chID string) string     { return fmt.Sprintf("Ch_%s_Drive", chID) }

// DMChannelName builds the display name for a direct message from the two
// participants' display names.
//
// This is a channel title that reaches the screen, so it must never be
// assembled from user IDs. Callers resolve display names first and pass them
// here; an unresolved side degrades to a neutral label rather than leaking an
// identifier.
func DMChannelName(displayNameA, displayNameB string) string {
	a, b := strings.TrimSpace(displayNameA), strings.TrimSpace(displayNameB)
	if a == "" {
		a = "Unknown"
	}
	if b == "" {
		b = "Unknown"
	}
	return fmt.Sprintf("%s, %s", a, b)
}

// --- Tenant naming conventions ---

// TenantMemberUAName returns the UA name for regular members of a tenant.
func TenantMemberUAName(tenantID string) string { return fmt.Sprintf("TenantMember_%s", tenantID) }

// TenantOwnerUAName returns the UA name for owners of a tenant.
func TenantOwnerUAName(tenantID string) string { return fmt.Sprintf("TenantOwner_%s", tenantID) }

// --- Drive naming conventions ---

// DriveRootName names the root drive OA for a workspace.
//
// It carries the whole workspace ID. An earlier version truncated to eight
// characters, which meant two workspaces whose UUIDs shared a prefix produced
// the same node name — and names are matched exactly, so the second workspace
// would have resolved onto the first one's drive root.
func DriveRootName(workspaceID string) string {
	return fmt.Sprintf("DriveRoot_%s", workspaceID)
}

func FolderNodeName(name string) string { return fmt.Sprintf("Folder_%s", name) }

func ShareOAName(itemName, uniqueSuffix string) string {
	return fmt.Sprintf("Share_%s_%s", itemName, uniqueSuffix)
}

// --- Node types ---
// Short aliases for the NGAC node type strings used in CreateNodeRequest.
const (
	TypePC = "PC"
	TypeUA = "UA"
	TypeOA = "OA"
	TypeU  = "U"
	TypeO  = "O"
)

// --- Access decisions ---
const (
	DecisionAllow = "ALLOW"
	DecisionDeny  = "DENY"
)
