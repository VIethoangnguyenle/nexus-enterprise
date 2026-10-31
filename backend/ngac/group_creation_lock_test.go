package ngac

import (
	"slices"
	"testing"
)

func TestGroupCreationRule_ReadsTheGrant(t *testing.T) {
	for _, tc := range []struct {
		name string
		ops  []string
		want string
	}{
		{"members grant", MemberChannelOps(), GroupCreationEveryone},
		{"no grant", nil, GroupCreationAdmins},
		{"other rights only", []string{OpManage}, GroupCreationAdmins},
	} {
		if got := GroupCreationRule(tc.ops); got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestLockedChannelOps_DropsOnlyCreateChannelAndKeepsInput(t *testing.T) {
	in := []string{OpManage, OpCreateChannel}
	if got := LockedChannelOps(in); !slices.Equal(got, []string{OpManage}) {
		t.Errorf("got %v", got)
	}
	if !slices.Equal(in, []string{OpManage, OpCreateChannel}) {
		t.Errorf("input modified: %v", in)
	}
	if got := LockedChannelOps(MemberChannelOps()); len(got) != 0 {
		t.Errorf("the Members grant has nothing else, so it should go away; got %v", got)
	}
	if got := LockedChannelOps(nil); len(got) != 0 {
		t.Errorf("got %v", got)
	}
}

func TestUnlockedMemberChannelOps_RestoresCreateChannelOnce(t *testing.T) {
	if got := UnlockedMemberChannelOps(nil); !slices.Equal(got, MemberChannelOps()) {
		t.Errorf("got %v", got)
	}
	if got := UnlockedMemberChannelOps(MemberChannelOps()); !slices.Equal(got, MemberChannelOps()) {
		t.Errorf("got %v", got)
	}
	if got := UnlockedMemberChannelOps([]string{OpManage}); !slices.Equal(got, []string{OpManage, OpCreateChannel}) {
		t.Errorf("got %v", got)
	}
}
