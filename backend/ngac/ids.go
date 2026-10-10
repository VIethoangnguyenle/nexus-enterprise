package ngac

// Identifier types for node-name helpers.
//
// Every helper that builds a node name takes one of these instead of a bare
// string. A node name is matched exactly and the graph keeps one node per name
// and type, so a name derived from something two tenants can both choose — a
// workspace, department or folder display name — makes one tenant's node
// resolve onto another's. With a plain string parameter, passing such a name
// compiles; with these types it does not, and the explicit conversion at the
// call site (ngac.DeptID(dept.ID)) is what a reviewer or the identifier lint
// (scripts/check-ngac-identifiers.sh) then looks at.
//
// Tenants are workspaces: a tenant ID is a WorkspaceID.
type (
	// WorkspaceID is workspaces.id (the tenant ID).
	WorkspaceID string
	// ChannelID is channels.id.
	ChannelID string
	// DeptID is departments.id.
	DeptID string
	// FolderID is the identifier of a folder: drive_items.id for a drive
	// folder, or a freshly generated ID for a workspace-level folder.
	FolderID string
	// ShareID is drive_shares.id.
	ShareID string
	// RoleID is a generated identifier for a role; the role's node ID is not
	// known until the node exists, and its name has to be chosen first.
	RoleID string
	// AssetTypeID is asset_types.id.
	AssetTypeID string
	// UserID is users.id.
	UserID string
	// UserNodeID is the ID of a user's U node in the graph (users.ngac_node).
	UserNodeID string
)
