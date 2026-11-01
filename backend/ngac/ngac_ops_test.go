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
		"FileOAName":         func(s string) string { return ngac.FileOAName(ngac.FileID(s)) },
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
		"file a":   ngac.FileOAName("a"),
		"file b":   ngac.FileOAName("b"),
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
		"Share_x_1", "File_x", " User_abc", "Role_abc", "role_abc", "U_abc",
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

func TestCalendarNamesAndOps(t *testing.T) {
	if got := ngac.CalendarOAName("c1"); got != "Cal_c1" {
		t.Errorf("calendar OA = %q", got)
	}
	if got := ngac.CalendarEventOAName("e1"); got != "CalEv_e1" {
		t.Errorf("event OA = %q", got)
	}
	if got := ngac.CalendarEventGuestsUAName("e1"); got != "CalEvGuests_e1" {
		t.Errorf("guests UA = %q", got)
	}
	want := map[string][]string{
		ngac.CalendarShareView:   {ngac.OpRead},
		ngac.CalendarShareEdit:   {ngac.OpRead, ngac.OpWrite},
		ngac.CalendarShareManage: {ngac.OpRead, ngac.OpWrite, ngac.OpShare, ngac.OpManage},
	}
	for level, ops := range want {
		got, ok := ngac.CalendarShareOps(level)
		if !ok || strings.Join(got, ",") != strings.Join(ops, ",") {
			t.Errorf("share %s = %v, %v", level, got, ok)
		}
	}
	for _, bad := range []string{"", "owner", "read", "invite"} {
		if _, ok := ngac.CalendarShareOps(bad); ok {
			t.Errorf("share level %q must be refused", bad)
		}
	}
	if g := ngac.CalendarGuestOps(false); len(g) != 1 || g[0] != ngac.OpRead {
		t.Errorf("guest ops = %v", g)
	}
	if g := ngac.CalendarGuestOps(true); len(g) != 2 || g[1] != ngac.OpInvite {
		t.Errorf("guest ops with invite = %v", g)
	}
	if err := ngac.ValidateRoleName("Cal_team"); err == nil {
		t.Error("role names in the calendar namespace must be rejected")
	}
}

// Only a folder's OA is a folder. The names of the workspace's own containers and
// of the OAs beneath them are not, whatever properties the node carries.
func TestIsFolderNodeName(t *testing.T) {
	ws := ngac.WorkspaceID("w1")
	yes := map[string]string{
		"a folder":      ngac.FolderNodeName("f1"),
		"a uuid folder": ngac.FolderNodeName("0b1c2d3e-0000-4000-8000-000000000001"),
	}
	no := map[string]string{
		"Mgmt":                ngac.MgmtOAName(ws),
		"Documents":           ngac.DocumentsOAName(ws),
		"Channels":            ngac.ChannelsOAName(ws),
		"Assets":              ngac.AssetsOAName(ws),
		"an asset type":       ngac.AssetTypeOAName(ws, "t1"),
		"an asset category":   ngac.AssetCategoryOAName(ws, "c1"),
		"a channel's content": ngac.ChannelContentOAName("c1"),
		"a share":             ngac.ShareOAName("s1"),
		"the prefix alone":    ngac.FolderNodeName(""),
		"empty":               "",
		"prefix not first":    "x" + ngac.FolderNodeName("f1"),
	}
	for name, n := range yes {
		if !ngac.IsFolderNodeName(n) {
			t.Errorf("%s (%q) must be a folder", name, n)
		}
	}
	for name, n := range no {
		if ngac.IsFolderNodeName(n) {
			t.Errorf("%s (%q) must not be a folder", name, n)
		}
	}
}

// An admin folder is a Folder_ OA of this workspace that the admin screen made (or
// made before folders were typed). The drive's folders share the name pattern and
// the workspace, and are not admin folders.
func TestIsAdminFolder(t *testing.T) {
	ws := ngac.WorkspaceID("w1")
	name := ngac.FolderNodeName("f1")
	props := func(t string) map[string]string {
		p := map[string]string{"workspace_id": "w1"}
		if t != "" {
			p[ngac.PropType] = t
		}
		return p
	}
	for label, c := range map[string]struct {
		name  string
		props map[string]string
		want  bool
	}{
		"made by the admin screen":       {name, ngac.AdminFolderProps(ws, "Legal"), true},
		"made before folders were typed": {name, props(""), true},
		"a drive folder":                 {name, props("drive_folder"), false},
		"another workspace's":            {name, map[string]string{"workspace_id": "w2", ngac.PropType: ngac.PropTypeAdminFolder}, false},
		"no workspace":                   {name, map[string]string{ngac.PropType: ngac.PropTypeAdminFolder}, false},
		"not named like a folder":        {ngac.MgmtOAName(ws), ngac.AdminFolderProps(ws, "x"), false},
		"nil properties":                 {name, nil, false},
	} {
		if got := ngac.IsAdminFolder(c.name, c.props, ws); got != c.want {
			t.Errorf("%s: got %v, want %v", label, got, c.want)
		}
	}
}

// The directory nodes are named by IDs, so two workspaces (or departments) never
// share one, and none of them can be mistaken for the node another helper builds:
// a name is matched exactly and a clash would silently re-aim an association.
func TestDirectoryNamesAreIDKeyedAndDistinct(t *testing.T) {
	names := map[string]string{}
	for what, name := range map[string]string{
		"directory w1":  ngac.DirectoryOAName("w1"),
		"directory w2":  ngac.DirectoryOAName("w2"),
		"open w1":       ngac.DirectoryOpenOAName("w1"),
		"open w2":       ngac.DirectoryOpenOAName("w2"),
		"no dept w1":    ngac.DirectoryNoDeptOAName("w1"),
		"no dept w2":    ngac.DirectoryNoDeptOAName("w2"),
		"owners w1":     ngac.DirectoryOwnersOAName("w1"),
		"owners w2":     ngac.DirectoryOwnersOAName("w2"),
		"limited w1":    ngac.DirectoryLimitedUAName("w1"),
		"limited w2":    ngac.DirectoryLimitedUAName("w2"),
		"rule w1":       ngac.DirectoryLimitedProhibitionName("w1"),
		"rule w2":       ngac.DirectoryLimitedProhibitionName("w2"),
		"dept dir d1":   ngac.DeptDirOAName("d1"),
		"dept dir d2":   ngac.DeptDirOAName("d2"),
		"dept peer d1":  ngac.DeptPeerOAName("d1"),
		"dept peer d2":  ngac.DeptPeerOAName("d2"),
		"dept peers d1": ngac.DeptPeersUAName("d1"),
		"dept peers d2": ngac.DeptPeersUAName("d2"),
		"dept ua d1":    ngac.DeptUAName("d1"),
		"members w1":    ngac.MembersUAName("w1"),
		"mgmt w1":       ngac.MgmtOAName("w1"),
	} {
		if prev, dup := names[name]; dup {
			t.Errorf("%s and %s share the node name %q", what, prev, name)
		}
		names[name] = what
	}
}

func TestDirectoryNamesSurviveShortInput(t *testing.T) {
	for _, in := range []string{"", "a", "ab"} {
		for what, name := range map[string]string{
			"directory":  ngac.DirectoryOAName(ngac.WorkspaceID(in)),
			"open":       ngac.DirectoryOpenOAName(ngac.WorkspaceID(in)),
			"no dept":    ngac.DirectoryNoDeptOAName(ngac.WorkspaceID(in)),
			"owners":     ngac.DirectoryOwnersOAName(ngac.WorkspaceID(in)),
			"limited":    ngac.DirectoryLimitedUAName(ngac.WorkspaceID(in)),
			"rule":       ngac.DirectoryLimitedProhibitionName(ngac.WorkspaceID(in)),
			"dept dir":   ngac.DeptDirOAName(ngac.DeptID(in)),
			"dept peer":  ngac.DeptPeerOAName(ngac.DeptID(in)),
			"dept peers": ngac.DeptPeersUAName(ngac.DeptID(in)),
		} {
			if name == "" {
				t.Errorf("%s(%q) returned an empty name", what, in)
			}
		}
	}
}

// No directory node may parse as a workspace's Members UA: the membership helper
// trusts that name, and the directory must never be able to pose as it.
func TestDirectoryNamesAreNeverAMembersUA(t *testing.T) {
	for _, name := range []string{
		ngac.DirectoryOAName("w1"), ngac.DirectoryOpenOAName("w1"), ngac.DirectoryNoDeptOAName("w1"),
		ngac.DirectoryOwnersOAName("w1"), ngac.DirectoryLimitedUAName("w1"),
		ngac.DeptDirOAName("d1"), ngac.DeptPeerOAName("d1"), ngac.DeptPeersUAName("d1"),
	} {
		if ws, ok := ngac.WorkspaceIDFromMembersUAName(name); ok {
			t.Errorf("%q parses as the Members UA of workspace %q", name, ws)
		}
	}
}

func TestDirectoryPropsMarkEveryNodeAsDirectory(t *testing.T) {
	props := ngac.DirectoryProps("w1")
	if props[ngac.PropType] != ngac.PropTypeDirectory || ngac.PropTypeDirectory != "directory" {
		t.Errorf("type = %q, want directory", props[ngac.PropType])
	}
	if props["workspace_id"] != "w1" {
		t.Errorf("workspace_id = %q", props["workspace_id"])
	}
	// A directory UA is not a role: roles are recognised by type=role alone.
	if props[ngac.PropType] == ngac.PropTypeRole {
		t.Error("a directory node must not read as a role")
	}
	// The caller owns its copy.
	props["workspace_id"] = "other"
	if ngac.DirectoryProps("w1")["workspace_id"] != "w1" {
		t.Error("DirectoryProps shares its map between calls")
	}
}

// A role may not be named like a directory node on screen or in logs.
func TestValidateRoleNameRejectsDirectoryNamespaces(t *testing.T) {
	for _, bad := range []string{
		"Directory_w1", "DirectoryOpen_w1", "DirectoryNoDept_w1", "DirectoryOwners_w1", "DirectoryLimited_w1",
		"Dir_d1", "DirPeer_d1", "DeptPeers_d1", "dir_d1",
	} {
		if err := ngac.ValidateRoleName(bad); err == nil {
			t.Errorf("ValidateRoleName(%q) accepted, want rejected", bad)
		}
	}
	for _, ok := range []string{"Directory admins", "Director", "Dirk"} {
		if err := ngac.ValidateRoleName(ok); err != nil {
			t.Errorf("ValidateRoleName(%q) = %v, want accepted", ok, err)
		}
	}
}

// A name is compared as people read it: compatibility forms (fullwidth letters)
// fold to the plain ones and invisible format characters (zero width space and
// joiners, BOM, soft hyphen) are dropped before the reserved-namespace check, so
// neither can smuggle a platform-looking name past it.
func TestValidateRoleNameSeesThroughLookalikes(t *testing.T) {
	for name, bad := range map[string]string{
		"zero width space inside the prefix": "Dep\u200bt_Sales",
		"zero width joiner":                  "D\u200dept_Sales",
		"BOM first":                          "\ufeffUser_abc",
		"soft hyphen":                        "Dir\u00adectory_w1",
		"fullwidth letters and underscore":   "Ｄｅｐｔ＿Sales",
		"fullwidth suffix":                   "ws1＿Owners",
		"invisible inside a suffix":          "ws1_Mem\u2060bers",
		"BOM and padding":                    " \ufeff PC_Global",
	} {
		if err := ngac.ValidateRoleName(bad); err == nil {
			t.Errorf("%s: ValidateRoleName(%q) accepted, want rejected", name, bad)
		}
	}
	// A name that is only invisible is empty.
	if err := ngac.ValidateRoleName("\u200b\ufeff"); err == nil {
		t.Error("a name of invisible characters only must be refused as empty")
	}
	// Ordinary names, accents and Vietnamese included, are untouched.
	for _, ok := range []string{"Kế toán trưởng", "Editor", "Người duyệt", "Directory admins"} {
		if err := ngac.ValidateRoleName(ok); err != nil {
			t.Errorf("ValidateRoleName(%q) = %v, want accepted", ok, err)
		}
	}
}
