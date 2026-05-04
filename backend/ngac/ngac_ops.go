// Package ngac provides a single source of truth for NGAC operations,
// well-known node names, and naming conventions used across all services.
// Every string that appears in policy API calls should originate from this package
// so that renames and additions are caught at compile time.
package ngac

import "fmt"

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
func ChannelMemberOps() []string {
	return []string{OpRead, OpWrite}
}

// ChannelDriveOps returns operations granted to channel members on their drive.
func ChannelDriveOps() []string {
	return []string{OpRead, OpWrite, OpUpload}
}

// --- Well-known node names ---
// These are global NGAC nodes that must exist in the policy graph.
const (
	NodePCGlobal    = "PC_Global"
	NodePublicUsers = "PublicUsers"
)

// --- Workspace naming conventions ---
// Every workspace creates a set of NGAC nodes named by workspace ID.
// Using ID (UUID) instead of display name prevents collisions.

func PCName(wsID string) string            { return fmt.Sprintf("PC_%s", wsID) }
func OwnersUAName(wsID string) string      { return fmt.Sprintf("%s_Owners", wsID) }
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

// --- Tenant naming conventions ---

// TenantMemberUAName returns the UA name for regular members of a tenant.
func TenantMemberUAName(tenantID string) string { return fmt.Sprintf("TenantMember_%s", tenantID) }

// TenantOwnerUAName returns the UA name for owners of a tenant.
func TenantOwnerUAName(tenantID string) string { return fmt.Sprintf("TenantOwner_%s", tenantID) }

// --- Drive naming conventions ---

func DriveRootName(workspaceID string) string {
	if len(workspaceID) > 8 {
		return fmt.Sprintf("DriveRoot_%s", workspaceID[:8])
	}
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
