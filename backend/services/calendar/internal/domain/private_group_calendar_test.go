package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/ngac"
	"ngac-platform/services/calendar/internal/store"
)

// withChannelsGrants gives the workspace's own UAs what CreateWorkspace gives them
// on the Channels OA, and adds an owner who is not in the "team" group. A grant
// there is inherited by every group's content OA and so by its calendar.
func (f *fixture) withChannelsGrants() {
	f.w.Seed("n-owen", ngac.UserNodeName("n-owen"), "U", nil)
	f.w.Assign("n-owen", "ua-owners")
	f.st.people["n-owen"] = &store.Person{NodeID: "n-owen", DisplayName: "Owen"}
	f.w.Assocs[[2]string{"ua-members", "oa-channels"}] = ngac.MemberChannelOps()
	f.w.Assocs[[2]string{"ua-owners", "oa-channels"}] = ngac.OwnerChannelOps()
}

// An owner holds manage on every group (to moderate it) but a group's calendar is
// the group's alone: manage must not be read as the right to see it. The group's
// own member reads it, and the same owner still manages the workspace calendar.
func TestAnOwnerOutsideAGroupCannotReadItsCalendar(t *testing.T) {
	f := newFixture(t)
	f.withChannelsGrants()
	cal := f.groupCal(t)
	ev := f.create(t, "n-alice", newInput(cal))

	// Deny: the owner outside the group.
	_, err := f.svc.ListEvents(callerCtx("n-owen"), ws, ListEventsQuery{From: rangeStart, To: rangeEnd, Calendars: []string{cal}})
	requireKind(t, err, ErrAccessDenied)
	_, err = f.svc.GetEvent(callerCtx("n-owen"), ws, ev.EventID, "")
	requireKind(t, err, ErrAccessDenied)
	_, err = f.svc.EnsureChannelCalendar(callerCtx("n-owen"), ws, "ch-team")
	requireKind(t, err, ErrAccessDenied)
	for _, c := range mustList(t, f, "n-owen") {
		assert.NotEqual(t, store.KindChannel, c.Kind, "the group's calendar is not listed for an owner outside it")
	}
	// Deny: a plain member outside the group, as before.
	_, err = f.svc.GetEvent(callerCtx("n-carol"), ws, ev.EventID, "")
	requireKind(t, err, ErrAccessDenied)

	// Allow: the group's member, on the same calendar.
	got, err := f.svc.ListEvents(callerCtx("n-alice"), ws, ListEventsQuery{From: rangeStart, To: rangeEnd, Calendars: []string{cal}})
	require.NoError(t, err)
	assert.NotEmpty(t, got)
	_, err = f.svc.GetEvent(callerCtx("n-alice"), ws, ev.EventID, "")
	require.NoError(t, err)

	// Allow: the owner still manages the workspace calendar.
	ws := kinds(mustList(t, f, "n-owen"))[store.KindWorkspace]
	require.NotNil(t, ws)
	assert.Equal(t, AccessManage, ws.Access)
}
