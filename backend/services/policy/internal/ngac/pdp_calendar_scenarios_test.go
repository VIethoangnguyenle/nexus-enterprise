package ngac_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	vocab "ngac-platform/ngac"
	"ngac-platform/services/policy/internal/ngac"
)

// Test vectors for the calendar capability (docs/specs/calendar).
//
// Events are not nodes. A calendar is authorized on its OA, an event that has
// guests on its own OA beneath the calendar's. The tree built here is the one
// the calendar service creates:
//
//	PC_ws1 ← Cal_alice (personal)      ← CalEv_e1
//	       ← Cal_ws   (workspace)
//	       ← ws1_Channels ← Ch_c1_Content ← Cal_chan (channel)
//
// Personal UAs are created without a parent, exactly as the drive does it; the
// holder reaches the policy class through the workspace UAs they also belong to.
// Each id below is the node's id; names always come from the vocabulary package.

const (
	calWS      = vocab.WorkspaceID("ws1")
	calOtherWS = vocab.WorkspaceID("ws2")
	calAlice   = vocab.CalendarID("alice")
	calWSCal   = vocab.CalendarID("ws")
	calChan    = vocab.CalendarID("chan")
	calEv      = vocab.CalendarEventID("e1")
	calCh      = vocab.ChannelID("c1")
)

type calGraph struct {
	*ngac.Graph
	t  *testing.T
	n  int
	pc string
}

func (c *calGraph) node(id, name, typ string) {
	c.Graph.AddNode(&ngac.NGACNode{ID: id, Name: name, NodeType: typ})
}

func (c *calGraph) assign(child, parent string) {
	c.t.Helper()
	c.n++
	require.NoError(c.t, c.AddAssignment(&ngac.Assignment{ID: "as-" + child + "-" + parent, ChildID: child, ParentID: parent}))
}

func (c *calGraph) associate(ua, oa string, ops []string) {
	c.t.Helper()
	c.n++
	require.NoError(c.t, c.AddAssociation(&ngac.Association{ID: "assoc-" + ua + "-" + oa, UAID: ua, OAID: oa, Operations: ops}))
}

// person adds a user to the workspace's Members UA and gives them a personal UA
// that has no parent, as the drive's lazily created personal UAs do.
func (c *calGraph) person(id string) {
	c.node("u-"+id, "U_"+id, "U")
	c.assign("u-"+id, "ua-members")
	c.node("pua-"+id, vocab.PersonalUAName(vocab.UserNodeID(id)), "UA")
	c.assign("u-"+id, "pua-"+id)
}

func buildCalendarGraph(t *testing.T) *calGraph {
	t.Helper()
	c := &calGraph{Graph: ngac.NewGraph(), t: t, pc: "pc-ws1"}
	c.node("pc-ws1", vocab.PCName(calWS), "PC")
	c.node("pc-ws2", vocab.PCName(calOtherWS), "PC")
	c.node("ua-owners", vocab.OwnersUAName(calWS), "UA")
	c.node("ua-members", vocab.MembersUAName(calWS), "UA")
	c.assign("ua-owners", "pc-ws1")
	c.assign("ua-members", "pc-ws1")

	// People: alice owns a personal calendar, bob/carol/dan are plain members,
	// olga is a workspace owner, gus is a member of another workspace.
	for _, p := range []string{"alice", "bob", "carol", "dan", "erin", "frank"} {
		c.person(p)
	}
	c.node("u-olga", "U_olga", "U")
	c.assign("u-olga", "ua-owners")
	c.node("u-gus", "U_gus", "U")
	c.node("ua-ws2-members", vocab.MembersUAName(calOtherWS), "UA")
	c.assign("ua-ws2-members", "pc-ws2")
	c.assign("u-gus", "ua-ws2-members")
	c.node("pua-gus", vocab.PersonalUAName("gus"), "UA")
	c.assign("u-gus", "pua-gus")

	// Personal calendar of alice, with two shares.
	c.node("oa-cal-alice", vocab.CalendarOAName(calAlice), "OA")
	c.assign("oa-cal-alice", "pc-ws1")
	c.associate("pua-alice", "oa-cal-alice", vocab.CalendarOwnerOps())
	viewOps, ok := vocab.CalendarShareOps(vocab.CalendarShareView)
	require.True(t, ok)
	editOps, ok := vocab.CalendarShareOps(vocab.CalendarShareEdit)
	require.True(t, ok)
	c.associate("pua-bob", "oa-cal-alice", viewOps)
	c.associate("pua-carol", "oa-cal-alice", editOps)
	// gus is shared too, but lives under another policy class.
	c.associate("pua-gus", "oa-cal-alice", viewOps)

	// Workspace calendar.
	c.node("oa-cal-ws", vocab.CalendarOAName(calWSCal), "OA")
	c.assign("oa-cal-ws", "pc-ws1")
	c.associate("ua-members", "oa-cal-ws", []string{vocab.OpRead})
	c.associate("ua-owners", "oa-cal-ws", vocab.CalendarOwnerOps())

	// Channel calendar beneath the channel's content OA; erin is in the channel.
	c.node("oa-channels", vocab.ChannelsOAName(calWS), "OA")
	c.assign("oa-channels", "pc-ws1")
	// The workspace's own grants on Channels, as CreateWorkspace makes them. They
	// are inherited by every channel beneath, calendar included, so they are what
	// keeps a group's calendar private to the group.
	c.associate("ua-members", "oa-channels", vocab.MemberChannelOps())
	c.associate("ua-owners", "oa-channels", vocab.OwnerChannelOps())
	c.node("oa-ch-content", vocab.ChannelContentOAName(calCh), "OA")
	c.assign("oa-ch-content", "oa-channels")
	c.node("ua-ch-members", vocab.ChannelMembersUAName(calCh), "UA")
	c.assign("ua-ch-members", "pc-ws1")
	c.assign("u-erin", "ua-ch-members")
	c.associate("ua-ch-members", "oa-ch-content", vocab.ChannelMemberOps())
	c.node("oa-cal-chan", vocab.CalendarOAName(calChan), "OA")
	c.assign("oa-cal-chan", "oa-ch-content")

	// An event of alice's calendar with guests: frank may read it, and invite.
	c.node("oa-ev", vocab.CalendarEventOAName(calEv), "OA")
	c.assign("oa-ev", "oa-cal-alice")
	c.node("ua-guests", vocab.CalendarEventGuestsUAName(calEv), "UA")
	c.assign("ua-guests", "pc-ws1")
	c.assign("u-frank", "ua-guests")
	c.associate("ua-guests", "oa-ev", vocab.CalendarGuestOps(false))
	return c
}

func (c *calGraph) decide(user, oa, op string) string {
	return c.CheckAccess(user, oa, op).Decision
}

func TestCalendar_Allow(t *testing.T) {
	c := buildCalendarGraph(t)
	cases := []struct{ name, user, oa, op string }{
		{"owner reads personal calendar", "u-alice", "oa-cal-alice", vocab.OpRead},
		{"owner writes personal calendar", "u-alice", "oa-cal-alice", vocab.OpWrite},
		{"owner shares personal calendar", "u-alice", "oa-cal-alice", vocab.OpShare},
		{"owner manages personal calendar", "u-alice", "oa-cal-alice", vocab.OpManage},
		{"owner reads an event under own calendar", "u-alice", "oa-ev", vocab.OpRead},
		{"view share reads", "u-bob", "oa-cal-alice", vocab.OpRead},
		{"view share reads an event", "u-bob", "oa-ev", vocab.OpRead},
		{"edit share writes", "u-carol", "oa-cal-alice", vocab.OpWrite},
		{"edit share writes an event", "u-carol", "oa-ev", vocab.OpWrite},
		{"member reads workspace calendar", "u-dan", "oa-cal-ws", vocab.OpRead},
		{"workspace owner writes workspace calendar", "u-olga", "oa-cal-ws", vocab.OpWrite},
		{"channel member reads channel calendar", "u-erin", "oa-cal-chan", vocab.OpRead},
		{"channel member writes channel calendar", "u-erin", "oa-cal-chan", vocab.OpWrite},
		{"guest reads the event", "u-frank", "oa-ev", vocab.OpRead},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, "ALLOW", c.decide(tc.user, tc.oa, tc.op))
		})
	}
}

func TestCalendar_GuestWithInviteFlag_MayInvite(t *testing.T) {
	c := buildCalendarGraph(t)
	assert.Equal(t, "DENY", c.decide("u-frank", "oa-ev", vocab.OpInvite), "no flag yet")
	// "Guests can invite others" is switched on: the association is replaced.
	c.RemoveAssociationByID("assoc-ua-guests-oa-ev")
	c.associate("ua-guests", "oa-ev", vocab.CalendarGuestOps(true))
	assert.Equal(t, "ALLOW", c.decide("u-frank", "oa-ev", vocab.OpInvite))
	assert.Equal(t, "ALLOW", c.decide("u-frank", "oa-ev", vocab.OpRead))
	assert.Equal(t, "DENY", c.decide("u-frank", "oa-ev", vocab.OpWrite), "inviting never grants editing")
}

func TestCalendar_Deny(t *testing.T) {
	c := buildCalendarGraph(t)
	cases := []struct{ name, user, oa, op string }{
		{"plain member cannot read another's personal calendar", "u-dan", "oa-cal-alice", vocab.OpRead},
		{"plain member cannot read an event of it", "u-dan", "oa-ev", vocab.OpRead},
		{"view share cannot write", "u-bob", "oa-cal-alice", vocab.OpWrite},
		{"view share cannot write an event", "u-bob", "oa-ev", vocab.OpWrite},
		{"view share cannot share on", "u-bob", "oa-cal-alice", vocab.OpShare},
		{"edit share cannot share on", "u-carol", "oa-cal-alice", vocab.OpShare},
		{"edit share cannot manage", "u-carol", "oa-cal-alice", vocab.OpManage},
		{"member cannot write workspace calendar", "u-dan", "oa-cal-ws", vocab.OpWrite},
		{"member cannot share workspace calendar", "u-dan", "oa-cal-ws", vocab.OpShare},
		{"non-channel member cannot read channel calendar", "u-dan", "oa-cal-chan", vocab.OpRead},
		{"non-channel member cannot write channel calendar", "u-dan", "oa-cal-chan", vocab.OpWrite},
		{"workspace owner outside the group cannot read its calendar", "u-olga", "oa-cal-chan", vocab.OpRead},
		{"workspace owner outside the group cannot write its calendar", "u-olga", "oa-cal-chan", vocab.OpWrite},
		{"member of another workspace cannot read channel calendar", "u-gus", "oa-cal-chan", vocab.OpRead},
		{"guest cannot write the event", "u-frank", "oa-ev", vocab.OpWrite},
		{"guest cannot read the host's calendar", "u-frank", "oa-cal-alice", vocab.OpRead},
		{"guest without invite flag cannot invite", "u-frank", "oa-ev", vocab.OpInvite},
		{"share under another policy class is denied despite association", "u-gus", "oa-cal-alice", vocab.OpRead},
		{"another workspace cannot read workspace calendar", "u-gus", "oa-cal-ws", vocab.OpRead},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, "DENY", c.decide(tc.user, tc.oa, tc.op))
		})
	}
}

// A workspace member who is not in group G is denied G's calendar while the group's
// own member keeps it, on the same graph; owners keep manage on the Channels area
// (moderation) without gaining the calendar.
func TestCalendar_ChannelCalendarIsPrivateToItsGroup(t *testing.T) {
	c := buildCalendarGraph(t)
	for _, op := range []string{vocab.OpRead, vocab.OpWrite} {
		assert.Equal(t, "ALLOW", c.decide("u-erin", "oa-cal-chan", op), "group member: "+op)
		assert.Equal(t, "DENY", c.decide("u-dan", "oa-cal-chan", op), "member not in group: "+op)
		assert.Equal(t, "DENY", c.decide("u-olga", "oa-cal-chan", op), "owner not in group: "+op)
	}
	assert.Equal(t, "ALLOW", c.decide("u-olga", "oa-ch-content", vocab.OpManage), "owners moderate unseen groups")
	assert.Equal(t, "DENY", c.decide("u-dan", "oa-ch-content", vocab.OpManage), "plain members do not")
}

func TestCalendar_GuestRemovedLosesAccessAtOnce(t *testing.T) {
	c := buildCalendarGraph(t)
	require.Equal(t, "ALLOW", c.decide("u-frank", "oa-ev", vocab.OpRead))
	c.RemoveAssignment("u-frank", "ua-guests")
	assert.Equal(t, "DENY", c.decide("u-frank", "oa-ev", vocab.OpRead))
}

func TestCalendar_ShareRevokedLosesAccess(t *testing.T) {
	c := buildCalendarGraph(t)
	require.Equal(t, "ALLOW", c.decide("u-bob", "oa-cal-alice", vocab.OpRead))
	c.RemoveAssociationByID("assoc-pua-bob-oa-cal-alice")
	assert.Equal(t, "DENY", c.decide("u-bob", "oa-cal-alice", vocab.OpRead))
}
