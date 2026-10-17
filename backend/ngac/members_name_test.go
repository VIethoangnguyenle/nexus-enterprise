package ngac_test

import (
	"testing"

	"ngac-platform/ngac"
)

func TestWorkspaceIDFromMembersUAName(t *testing.T) {
	ws := ngac.WorkspaceID("5767afe0-e7a3-422e-bda7-1d6c6e5618d6")

	t.Run("round trip", func(t *testing.T) {
		got, ok := ngac.WorkspaceIDFromMembersUAName(ngac.MembersUAName(ws))
		if !ok || got != ws {
			t.Fatalf("got (%q, %v), want (%q, true)", got, ok, ws)
		}
	})

	for name, in := range map[string]string{
		"channel members UA": ngac.ChannelMembersUAName("c1"),
		"tenant member UA":   ngac.TenantMemberUAName(ws),
		"owners UA":          ngac.OwnersUAName(ws),
		"empty workspace id": "_Members",
		"empty":              "",
		"no suffix":          string(ws),
	} {
		t.Run("rejects "+name, func(t *testing.T) {
			if got, ok := ngac.WorkspaceIDFromMembersUAName(in); ok {
				t.Fatalf("accepted %q as workspace %q", in, got)
			}
		})
	}
}
