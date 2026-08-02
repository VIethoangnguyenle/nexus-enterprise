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
