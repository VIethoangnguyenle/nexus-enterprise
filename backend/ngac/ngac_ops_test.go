package ngac_test

import (
	"strings"
	"testing"

	"ngac-platform/ngac"
)

// Node names are built from IDs supplied by callers. A helper that slices a
// fixed prefix off its argument panics on anything shorter, and a panic in a
// name helper takes down whatever request is holding it.
func TestNameHelpersSurviveShortInput(t *testing.T) {
	shortInputs := []string{"", "a", "ab", "abc123"}

	helpers := map[string]func(string) string{
		"PCName":             func(s string) string { return ngac.PCName(ngac.WorkspaceID(s)) },
		"OwnersUAName":       func(s string) string { return ngac.OwnersUAName(ngac.WorkspaceID(s)) },
		"MembersUAName":      func(s string) string { return ngac.MembersUAName(ngac.WorkspaceID(s)) },
		"MgmtOAName":         func(s string) string { return ngac.MgmtOAName(ngac.WorkspaceID(s)) },
		"DocumentsOAName":    func(s string) string { return ngac.DocumentsOAName(ngac.WorkspaceID(s)) },
		"DraftDocsOAName":    func(s string) string { return ngac.DraftDocsOAName(ngac.WorkspaceID(s)) },
		"ApprovedDocsOAName": func(s string) string { return ngac.ApprovedDocsOAName(ngac.WorkspaceID(s)) },
		"ChannelsOAName":     func(s string) string { return ngac.ChannelsOAName(ngac.WorkspaceID(s)) },
		"AssetsOAName":       func(s string) string { return ngac.AssetsOAName(ngac.WorkspaceID(s)) },
		"DeptUAName":         func(s string) string { return ngac.DeptUAName(ngac.DeptID(s)) },
		"ChannelContentOA":   func(s string) string { return ngac.ChannelContentOAName(ngac.ChannelID(s)) },
		"ChannelMembersUA":   func(s string) string { return ngac.ChannelMembersUAName(ngac.ChannelID(s)) },
		"ChannelDriveName":   func(s string) string { return ngac.ChannelDriveName(ngac.ChannelID(s)) },
		"TenantMemberUAName": func(s string) string { return ngac.TenantMemberUAName(ngac.WorkspaceID(s)) },
		"TenantOwnerUAName":  func(s string) string { return ngac.TenantOwnerUAName(ngac.WorkspaceID(s)) },
		"DriveRootName":      func(s string) string { return ngac.DriveRootName(ngac.WorkspaceID(s)) },
		"FolderNodeName":     func(s string) string { return ngac.FolderNodeName(ngac.FolderID(s)) },
		"ShareOAName":        func(s string) string { return ngac.ShareOAName(ngac.ShareID(s)) },
		"RoleUAName":         func(s string) string { return ngac.RoleUAName(ngac.RoleID(s)) },
		"UserNodeName":       func(s string) string { return ngac.UserNodeName(ngac.UserID(s)) },
		"PersonalUAName":     func(s string) string { return ngac.PersonalUAName(ngac.UserNodeID(s)) },
		"AssetTypeOAName":    func(s string) string { return ngac.AssetTypeOAName("ws", ngac.AssetTypeID(s)) },
	}

	for name, fn := range helpers {
		for _, in := range shortInputs {
			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Errorf("%s(%q) panicked: %v", name, in, r)
					}
				}()
				if got := fn(in); got == "" {
					t.Errorf("%s(%q) returned an empty name", name, in)
				}
			}()
		}
	}
}

// Two entities of one kind with different IDs never share a node name, and the
// same ID in two kinds never does either. A display name is not an input of any
// helper, so two tenants who both call a department "Sales" cannot collide.
func TestIDKeyedNamesAreDistinct(t *testing.T) {
	names := map[string]string{}
	for what, name := range map[string]string{
		"dept a":   ngac.DeptUAName("a"),
		"dept b":   ngac.DeptUAName("b"),
		"folder a": ngac.FolderNodeName("a"),
		"folder b": ngac.FolderNodeName("b"),
		"share a":  ngac.ShareOAName("a"),
		"share b":  ngac.ShareOAName("b"),
		"role a":   ngac.RoleUAName("a"),
		"role b":   ngac.RoleUAName("b"),
		"user a":   ngac.UserNodeName("a"),
		"user b":   ngac.UserNodeName("b"),
		"type a":   ngac.AssetTypeOAName("ws", "a"),
		"type b":   ngac.AssetTypeOAName("ws", "b"),
		"type a/2": ngac.AssetTypeOAName("ws2", "a"),
		"drive a":  ngac.ChannelDriveName("a"),
		"drive b":  ngac.ChannelDriveName("b"),
	} {
		if prev, dup := names[name]; dup {
			t.Errorf("%s and %s share the node name %q", what, prev, name)
		}
		names[name] = what
	}
}

func TestDisplayName(t *testing.T) {
	if got := ngac.DisplayName("Dept_1", map[string]string{ngac.PropDisplayName: "Sales"}); got != "Sales" {
		t.Errorf("DisplayName with property = %q, want Sales", got)
	}
	// Nodes written before names became ID-keyed carry the display name itself.
	if got := ngac.DisplayName("Editor", nil); got != "Editor" {
		t.Errorf("DisplayName without property = %q, want the node name", got)
	}
	if got := ngac.DisplayName("Role_1", map[string]string{ngac.PropDisplayName: ""}); got != "Role_1" {
		t.Errorf("an empty display name must fall back to the node name, got %q", got)
	}
}

// Truncating the workspace ID into the node name meant two workspaces whose
// UUIDs share a prefix would produce the same DriveRoot node — and the graph
// looks names up by exact match.
func TestDriveRootNameDoesNotCollideOnSharedPrefix(t *testing.T) {
	var a ngac.WorkspaceID = "0a1b2c3d-1111-4444-8888-aaaaaaaaaaaa"
	var b ngac.WorkspaceID = "0a1b2c3d-2222-5555-9999-bbbbbbbbbbbb"

	if ngac.DriveRootName(a) == ngac.DriveRootName(b) {
		t.Errorf("DriveRootName collides on a shared 8-char prefix: %q", ngac.DriveRootName(a))
	}
	if !strings.Contains(ngac.DriveRootName(a), string(a)) {
		t.Errorf("DriveRootName(%q) = %q, want it to carry the full ID", a, ngac.DriveRootName(a))
	}
}

// DM channel names reach the screen, so they must not be assembled from raw
// identifiers, and must tolerate a short or missing ID.
func TestDMChannelName(t *testing.T) {
	if got := ngac.DMChannelName("Alice", "Bob"); got != "Alice, Bob" {
		t.Errorf("DMChannelName = %q, want %q", got, "Alice, Bob")
	}
	for _, tc := range [][2]string{{"", ""}, {"Alice", ""}, {"", "Bob"}} {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("DMChannelName(%q, %q) panicked: %v", tc[0], tc[1], r)
				}
			}()
			if got := ngac.DMChannelName(tc[0], tc[1]); got == "" {
				t.Errorf("DMChannelName(%q, %q) returned empty", tc[0], tc[1])
			}
		}()
	}
}

func TestShareOps(t *testing.T) {
	read, ok := ngac.ShareOps(ngac.SharePermissionRead)
	if !ok || len(read) != 1 || read[0] != ngac.OpRead {
		t.Fatalf("read share = %v, %v", read, ok)
	}
	write, ok := ngac.ShareOps(ngac.SharePermissionWrite)
	if !ok || len(write) != 2 || write[0] != ngac.OpRead || write[1] != ngac.OpWrite {
		t.Fatalf("write share = %v, %v", write, ok)
	}
	// Deny: nothing outside the two permissions maps to operations, least of
	// all an operation name a client might try to smuggle in.
	for _, bad := range []string{"", "manage", "share", "approve", "READ", "read,write", "upload"} {
		if ops, ok := ngac.ShareOps(bad); ok || ops != nil {
			t.Errorf("ngac.ShareOps(%q) = %v, %v; want rejected", bad, ops, ok)
		}
	}
}

func TestPersonalUANameIsPerUser(t *testing.T) {
	if ngac.PersonalUAName("a") == ngac.PersonalUAName("b") {
		t.Fatal("distinct users must get distinct personal UAs")
	}
}

func TestValidateRoleName(t *testing.T) {
	for _, ok := range []string{"Editor", "Kế toán trưởng", "Reviewers", "Members of QA", "Userland"} {
		if err := ngac.ValidateRoleName(ok); err != nil {
			t.Errorf("ValidateRoleName(%q) = %v, want accepted", ok, err)
		}
	}
	// Deny: every namespace the platform builds node names in.
	for _, bad := range []string{
		"", "   ", "User_abc", "user_abc", "PC_Global", "PC_ws1", "TenantMember_t", "TenantOwner_t",
		"Dept_Sales", "Ch_x_Members", "ws1_Owners", "ws1_members", "ws1_Mgmt", "ws1_Documents",
		"ws1_Channels", "ws1_Assets", "PublicUsers", "publicusers", "DriveRoot_ws", "Folder_x",
		"Share_x_1", " User_abc", "Role_abc", "role_abc", "U_abc",
	} {
		if err := ngac.ValidateRoleName(bad); err == nil {
			t.Errorf("ValidateRoleName(%q) accepted, want rejected", bad)
		}
	}
}

func TestPersonalUAProperties(t *testing.T) {
	props := ngac.PersonalUAProperties("u1")
	if !ngac.IsPersonalUAOf(props, "u1") {
		t.Fatal("own properties must match")
	}
	for name, p := range map[string]map[string]string{
		"other user": props, "no properties": nil, "wrong type": {ngac.PropType: "role", ngac.PropUserNodeID: "u2"},
		"missing user": {ngac.PropType: ngac.PropTypePersonalUA},
	} {
		if ngac.IsPersonalUAOf(p, "u2") && name == "other user" {
			t.Errorf("%s: matched", name)
		}
	}
	if ngac.IsPersonalUAOf(ngac.PersonalUAProperties(""), "") {
		t.Error("an empty user id must never match")
	}
	if ngac.IsPersonalUAOf(map[string]string{ngac.PropType: "role", ngac.PropUserNodeID: "u2"}, "u2") {
		t.Error("a node without the personal_ua type must not match")
	}
}

func TestMembersKeepShareButShareGranteesDoNot(t *testing.T) {
	has := func(ops []string, op string) bool {
		for _, o := range ops {
			if o == op {
				return true
			}
		}
		return false
	}
	if !has(ngac.MemberDocumentOps(), ngac.OpShare) || !has(ngac.ChannelDriveOps(), ngac.OpShare) {
		t.Fatal("members must hold share on Documents and on their channel drive")
	}
	for _, op := range []string{ngac.OpManage, ngac.OpApprove, ngac.OpInvite} {
		if has(ngac.MemberDocumentOps(), op) || has(ngac.ChannelDriveOps(), op) {
			t.Errorf("members must not hold %s", op)
		}
	}
	for _, perm := range []string{ngac.SharePermissionRead, ngac.SharePermissionWrite} {
		ops, _ := ngac.ShareOps(perm)
		if has(ops, ngac.OpShare) {
			t.Errorf("a %s share must not grant share", perm)
		}
	}
}
