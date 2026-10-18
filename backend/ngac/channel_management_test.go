package ngac_test

import (
	"slices"
	"testing"

	"ngac-platform/ngac"
)

func TestChannelManagersUAName_Pattern(t *testing.T) {
	if got := ngac.ChannelManagersUAName("abc"); got != "Ch_abc_Managers" {
		t.Errorf("got %q, want Ch_abc_Managers", got)
	}
}

func TestChannelManagersUAName_DistinctFromMembers(t *testing.T) {
	chID := ngac.ChannelID("c1")
	if ngac.ChannelManagersUAName(chID) == ngac.ChannelMembersUAName(chID) {
		t.Fatal("Managers and Members must have distinct names")
	}
}

func TestChannelManagersUAName_DistinctAcrossChannels(t *testing.T) {
	if ngac.ChannelManagersUAName("a") == ngac.ChannelManagersUAName("b") {
		t.Fatal("different channels must get different Managers UAs")
	}
}

func TestChannelManagersUAName_SurvivesShortInput(t *testing.T) {
	for _, in := range []string{"", "x", "ab"} {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("ChannelManagersUAName(%q) panicked: %v", in, r)
				}
			}()
			if got := ngac.ChannelManagersUAName(ngac.ChannelID(in)); got == "" {
				t.Errorf("ChannelManagersUAName(%q) returned empty", in)
			}
		}()
	}
}

// The Managers UA never parses as a workspace's Members UA.
func TestChannelManagersUAName_NotAMembersUA(t *testing.T) {
	name := ngac.ChannelManagersUAName("c1")
	if _, ok := ngac.WorkspaceIDFromMembersUAName(name); ok {
		t.Errorf("%q must not parse as a workspace Members UA", name)
	}
}

// A role named like a Managers UA must be rejected by ValidateRoleName.
func TestChannelManagersUAName_RejectedAsRoleName(t *testing.T) {
	name := ngac.ChannelManagersUAName("c1")
	if err := ngac.ValidateRoleName(name); err == nil {
		t.Errorf("ValidateRoleName(%q) accepted, want rejected", name)
	}
}

func TestChannelManagerOps_ContainsOnlyInvite(t *testing.T) {
	ops := ngac.ChannelManagerOps()
	if !slices.Equal(ops, []string{ngac.OpInvite}) {
		t.Errorf("ChannelManagerOps() = %v, want only invite", ops)
	}
}

func TestChannelManagerOps_DoesNotShareSlice(t *testing.T) {
	a := ngac.ChannelManagerOps()
	a[0] = "tampered"
	if ngac.ChannelManagerOps()[0] == "tampered" {
		t.Error("ChannelManagerOps must return a fresh slice")
	}
}

func TestChannelMemberOpsRestricted_HasNoInvite(t *testing.T) {
	ops := ngac.ChannelMemberOpsRestricted()
	if slices.Contains(ops, ngac.OpInvite) {
		t.Errorf("restricted ops must not contain invite: %v", ops)
	}
}

func TestChannelMemberOpsRestricted_KeepsReadAndWrite(t *testing.T) {
	ops := ngac.ChannelMemberOpsRestricted()
	for _, want := range []string{ngac.OpRead, ngac.OpWrite} {
		if !slices.Contains(ops, want) {
			t.Errorf("restricted ops must contain %s: %v", want, ops)
		}
	}
}

func TestChannelMemberOpsRestricted_DoesNotModifyOriginal(t *testing.T) {
	before := slices.Clone(ngac.ChannelMemberOps())
	_ = ngac.ChannelMemberOpsRestricted()
	if !slices.Equal(ngac.ChannelMemberOps(), before) {
		t.Error("ChannelMemberOpsRestricted modified the original ChannelMemberOps")
	}
}

func TestIsGroupSetting(t *testing.T) {
	if !ngac.IsGroupSetting(ngac.GroupSettingEveryone) {
		t.Error("everyone must be valid")
	}
	if !ngac.IsGroupSetting(ngac.GroupSettingManagers) {
		t.Error("managers must be valid")
	}
	for _, bad := range []string{"", "admins", "owner", "EVERYONE"} {
		if ngac.IsGroupSetting(bad) {
			t.Errorf("%q must not be a valid group setting", bad)
		}
	}
}

func TestInviteRule_ReadsTheGrant(t *testing.T) {
	for _, tc := range []struct {
		name string
		ops  []string
		want string
	}{
		{"full member ops", ngac.ChannelMemberOps(), ngac.GroupSettingEveryone},
		{"restricted ops", ngac.ChannelMemberOpsRestricted(), ngac.GroupSettingManagers},
		{"no grant", nil, ngac.GroupSettingManagers},
		{"read only", []string{ngac.OpRead}, ngac.GroupSettingManagers},
	} {
		if got := ngac.InviteRule(tc.ops); got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}
}
