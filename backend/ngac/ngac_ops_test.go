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
		"PCName":             ngac.PCName,
		"OwnersUAName":       ngac.OwnersUAName,
		"MembersUAName":      ngac.MembersUAName,
		"MgmtOAName":         ngac.MgmtOAName,
		"DocumentsOAName":    ngac.DocumentsOAName,
		"DraftDocsOAName":    ngac.DraftDocsOAName,
		"ApprovedDocsOAName": ngac.ApprovedDocsOAName,
		"ChannelsOAName":     ngac.ChannelsOAName,
		"DeptUAName":         ngac.DeptUAName,
		"ChannelContentOA":   ngac.ChannelContentOAName,
		"ChannelMembersUA":   ngac.ChannelMembersUAName,
		"ChannelDriveName":   ngac.ChannelDriveName,
		"TenantMemberUAName": ngac.TenantMemberUAName,
		"TenantOwnerUAName":  ngac.TenantOwnerUAName,
		"DriveRootName":      ngac.DriveRootName,
		"FolderNodeName":     ngac.FolderNodeName,
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

// Truncating the workspace ID into the node name meant two workspaces whose
// UUIDs share a prefix would produce the same DriveRoot node — and the graph
// looks names up by exact match.
func TestDriveRootNameDoesNotCollideOnSharedPrefix(t *testing.T) {
	a := "0a1b2c3d-1111-4444-8888-aaaaaaaaaaaa"
	b := "0a1b2c3d-2222-5555-9999-bbbbbbbbbbbb"

	if ngac.DriveRootName(a) == ngac.DriveRootName(b) {
		t.Errorf("DriveRootName collides on a shared 8-char prefix: %q", ngac.DriveRootName(a))
	}
	if !strings.Contains(ngac.DriveRootName(a), a) {
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
		"Share_x_1", " User_abc",
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
