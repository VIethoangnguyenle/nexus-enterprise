package ngac

// The kinds of role a workspace tells its clients apart. Every service that names a role's
// kind on the wire uses these, so a rename is a compile error and not a silent mismatch
// between the workspace service that sends it and the auth service that reads it.
const (
	// RoleKindOwners is the workspace's owners group; clients word it themselves.
	RoleKindOwners = "owners"
	// RoleKindMembers is the Members group (never listed as a role of a person).
	RoleKindMembers = "members"
	// RoleKindCustom is a role an administrator made.
	RoleKindCustom = "custom"
)
