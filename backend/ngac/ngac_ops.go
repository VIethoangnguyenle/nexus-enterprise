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
//
// OpShare is included so that members keep sharing the files of a channel they
// belong to, as they did before sharing was gated on the share operation. A
// share only ever grants read or write to the grantee (see ShareOps), so the
// right to share does not spread the right to share.
func ChannelDriveOps() []string {
	return []string{OpRead, OpWrite, OpUpload, OpShare}
}

// MemberDocumentOps returns operations granted to workspace members on the
// Documents tree.
//
// OpShare is included because members share what they work on; the drive gates
// CreateShare on it. Sharing grants the grantee read or write only, never
// share, manage or approve.
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
	return []string{OpRead, OpWrite, OpUpload, OpShare}
}

// SharePermission names how much a share lets the grantee do. It is the only
// vocabulary clients may use to describe a share; the operations behind it are
// decided here, never passed in by the caller.
const (
	SharePermissionRead  = "read"
	SharePermissionWrite = "write"
)

// ShareOps maps a share permission to the operations the share grants.
//
// "write" includes read because a grantee who can edit a file must be able to
// open it. Nothing else is accepted: in particular a caller cannot name an
// operation such as manage or share, which would let a share hand out rights
// the sharer was never meant to delegate.
func ShareOps(permission string) ([]string, bool) {
	switch permission {
	case SharePermissionRead:
		return []string{OpRead}, true
	case SharePermissionWrite:
		return []string{OpRead, OpWrite}, true
	}
	return nil, false
}

// --- Well-known node names ---
// These are global NGAC nodes that must exist in the policy graph.
const (
	NodePCGlobal    = "PC_Global"
	NodePublicUsers = "PublicUsers"
)

// --- Asset naming conventions ---
//
// Named by workspace ID, not workspace name. Two workspaces are free to share a
// display name, and a name-derived node would then be shared between them —
// which in a graph that answers access questions means one tenant's assets
// resolving onto another's attributes.
//
// Assets themselves have no node: the graph holds only attributes, so an asset
// is authorized through the OA of its type (AssetTypeOAName), and a grant on
// the Assets or a category OA reaches every type beneath it.

func AssetsOAName(wsID WorkspaceID) string { return fmt.Sprintf("%s_Assets", wsID) }

// AssetCategoryOAName names the OA of a category within one workspace. A
// category is a label the workspace chooses, not an entity with an ID; it is
// scoped by the workspace ID so two workspaces never share one.
func AssetCategoryOAName(wsID WorkspaceID, category string) string {
	return fmt.Sprintf("%s_Category_%s", wsID, category)
}

// AssetTypeOAName names the OA of one asset type. Keyed by the type's ID: two
// types may share a display name (or sanitize to the same string) and must not
// share an OA, since the OA is what their assets are authorized on.
func AssetTypeOAName(wsID WorkspaceID, typeID AssetTypeID) string {
	return fmt.Sprintf("%s_Type_%s", wsID, typeID)
}

// --- Workspace naming conventions ---
// Every workspace creates a set of NGAC nodes named by workspace ID.
// Using ID (UUID) instead of display name prevents collisions.

func PCName(wsID WorkspaceID) string             { return fmt.Sprintf("PC_%s", wsID) }
func OwnersUAName(wsID WorkspaceID) string       { return fmt.Sprintf("%s_Owners", wsID) }
func MembersUAName(wsID WorkspaceID) string      { return fmt.Sprintf("%s_Members", wsID) }
func MgmtOAName(wsID WorkspaceID) string         { return fmt.Sprintf("%s_Mgmt", wsID) }
func DocumentsOAName(wsID WorkspaceID) string    { return fmt.Sprintf("%s_Documents", wsID) }
func DraftDocsOAName(wsID WorkspaceID) string    { return fmt.Sprintf("%s_DraftDocs", wsID) }
func ApprovedDocsOAName(wsID WorkspaceID) string { return fmt.Sprintf("%s_ApprovedDocs", wsID) }
func ChannelsOAName(wsID WorkspaceID) string     { return fmt.Sprintf("%s_Channels", wsID) }

// --- Department naming conventions ---

// DeptUAName names a department's UA by the department's ID. The display name
// lives in the node properties (PropDisplayName), so renaming a department
// never renames a node and two tenants may both have a "Sales".
func DeptUAName(deptID DeptID) string { return fmt.Sprintf("Dept_%s", deptID) }

// --- Channel naming conventions ---

func ChannelContentOAName(chID ChannelID) string { return fmt.Sprintf("Ch_%s_Content", chID) }
func ChannelMembersUAName(chID ChannelID) string { return fmt.Sprintf("Ch_%s_Members", chID) }
func ChannelDriveName(chID ChannelID) string     { return fmt.Sprintf("Ch_%s_Drive", chID) }

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
func TenantMemberUAName(tenantID WorkspaceID) string { return fmt.Sprintf("TenantMember_%s", tenantID) }

// TenantOwnerUAName returns the UA name for owners of a tenant.
func TenantOwnerUAName(tenantID WorkspaceID) string { return fmt.Sprintf("TenantOwner_%s", tenantID) }

// --- Drive naming conventions ---

// DriveRootName names the root drive OA for a workspace.
//
// It carries the whole workspace ID. An earlier version truncated to eight
// characters, which meant two workspaces whose UUIDs shared a prefix produced
// the same node name — and names are matched exactly, so the second workspace
// would have resolved onto the first one's drive root.
func DriveRootName(workspaceID WorkspaceID) string {
	return fmt.Sprintf("DriveRoot_%s", workspaceID)
}

// Properties that mark a node as a person's own attribute. The name below is
// for readability only: nothing may trust a node because of its name, since a
// workspace administrator chooses role names. A personal UA is recognised by
// these properties and by the user being assigned to it.
const (
	PropType           = "type"
	PropTypePersonalUA = "personal_ua"
	PropTypeRole       = "role"
	PropUserNodeID     = "user_node_id"
)

// PersonalUAProperties returns the properties a personal UA is created with.
func PersonalUAProperties(userNodeID string) map[string]string {
	return map[string]string{PropType: PropTypePersonalUA, PropUserNodeID: userNodeID}
}

// IsPersonalUAOf reports whether props mark a UA as the personal attribute of
// exactly this user.
func IsPersonalUAOf(props map[string]string, userNodeID string) bool {
	return userNodeID != "" && props[PropType] == PropTypePersonalUA && props[PropUserNodeID] == userNodeID
}

// reservedRolePrefixes and reservedRoleSuffixes are the namespaces the platform
// builds its own node names in. A role's node is named by a generated ID
// (RoleUAName) and its display name is only a property, so a name an
// administrator types can no longer become a node name. The list stays as
// display-name hygiene: a role listed as "Dept_Sales" or "Role_1" would read as
// a platform node on screen and in logs, and legacy nodes written before names
// were ID-keyed still carry their display name as the node name.
var (
	reservedRolePrefixes = []string{
		"User_", "PC_", "TenantMember_", "TenantOwner_", "Dept_", "Ch_",
		"DriveRoot_", "Folder_", "Share_", "Asset_", "Role_", "U_",
	}
	reservedRoleSuffixes = []string{
		"_Owners", "_Members", "_Mgmt", "_Documents", "_DraftDocs", "_ApprovedDocs",
		"_Channels", "_Assets", "_Content", "_Drive",
	}
	reservedRoleNames = []string{NodePCGlobal, NodePublicUsers}
)

// ValidateRoleName rejects a role name that is empty or falls inside a
// namespace the platform reserves for its own nodes (case-insensitively).
func ValidateRoleName(name string) error {
	n := strings.TrimSpace(name)
	if n == "" {
		return fmt.Errorf("role name is required")
	}
	lower := strings.ToLower(n)
	for _, r := range reservedRoleNames {
		if lower == strings.ToLower(r) {
			return fmt.Errorf("role name %q is reserved", name)
		}
	}
	for _, p := range reservedRolePrefixes {
		if strings.HasPrefix(lower, strings.ToLower(p)) {
			return fmt.Errorf("role name may not start with %q", p)
		}
	}
	for _, sfx := range reservedRoleSuffixes {
		if strings.HasSuffix(lower, strings.ToLower(sfx)) {
			return fmt.Errorf("role name may not end with %q", sfx)
		}
	}
	return nil
}

// PersonalUAName names the user attribute that stands for exactly one user.
//
// A user node (U) cannot be the source of an association — only a UA can — so
// granting something to one person means granting it to a UA that contains only
// them. Named by the user's node ID, never by display name, so two people with
// the same name cannot resolve onto the same attribute.
func PersonalUAName(userNodeID UserNodeID) string { return fmt.Sprintf("User_%s", userNodeID) }

// FolderNodeName names a folder's OA by the folder's ID, never by what the
// folder is called: sibling folders in different tenants share names freely.
func FolderNodeName(folderID FolderID) string { return fmt.Sprintf("Folder_%s", folderID) }

// ShareOAName names the OA that wraps one shared item, by the share's ID.
func ShareOAName(shareID ShareID) string { return fmt.Sprintf("Share_%s", shareID) }

// RoleUAName names a role's UA by a generated ID. The role's display name — the
// one an administrator typed — is kept in PropDisplayName; it is never part of
// a node name, so a role cannot take over a name the platform builds itself.
func RoleUAName(roleID RoleID) string { return fmt.Sprintf("Role_%s", roleID) }

// UserNodeName names a user's U node by users.id, not by username.
func UserNodeName(userID UserID) string { return fmt.Sprintf("U_%s", userID) }

// PropDisplayName holds the human-readable name of a node whose own name is a
// platform-built identifier. Screens show it; nothing authorizes on it.
const PropDisplayName = "display_name"

// DisplayName returns the name to show for a node: its display_name property
// when it has one, else the node name (nodes created before names became
// ID-keyed carry their display name as the node name).
func DisplayName(name string, props map[string]string) string {
	if d := props[PropDisplayName]; d != "" {
		return d
	}
	return name
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
