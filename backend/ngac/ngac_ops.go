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
// Every workspace creates a set of NGAC nodes with predictable names.
// These functions are the ONLY place those name patterns should live.

func PCName(wsName string) string            { return fmt.Sprintf("PC_%s", wsName) }
func OwnersUAName(wsName string) string      { return fmt.Sprintf("%s_Owners", wsName) }
func MembersUAName(wsName string) string      { return fmt.Sprintf("%s_Members", wsName) }
func MgmtOAName(wsName string) string         { return fmt.Sprintf("%s_Mgmt", wsName) }
func DocumentsOAName(wsName string) string    { return fmt.Sprintf("%s_Documents", wsName) }
func DraftDocsOAName(wsName string) string    { return fmt.Sprintf("%s_DraftDocs", wsName) }
func ApprovedDocsOAName(wsName string) string { return fmt.Sprintf("%s_ApprovedDocs", wsName) }
func ChannelsOAName(wsName string) string     { return fmt.Sprintf("%s_Channels", wsName) }

// --- Channel naming conventions ---

func ChannelContentOAName(chName string) string { return fmt.Sprintf("Ch_%s_Content", chName) }
func ChannelMembersUAName(chName string) string { return fmt.Sprintf("Ch_%s_Members", chName) }
func ChannelDriveName(chName string) string     { return fmt.Sprintf("Ch_%s_Drive", chName) }

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
