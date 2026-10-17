package domain

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/ngac"
	"ngac-platform/pkg/realtime"
	"ngac-platform/services/calendar/internal/policyfake"
	"ngac-platform/services/calendar/internal/store"
)

const ws = "ws1"

type fixture struct {
	w   *policyfake.World
	st  *memStore
	svc *Service
	rec *realtime.Recorder
}

func newFixture(t *testing.T) *fixture { return newFixtureOfType(t, "organization") }

// newFixtureOfType builds workspace ws1 with members alice, bob and carol, owner
// olga, and an outsider gus who belongs to no workspace. Group "team" has alice
// and olga; "dm" is a direct message.
func newFixtureOfType(t *testing.T, wsType string) *fixture {
	t.Helper()
	w := policyfake.NewWorld()
	st := newMemStore(&store.Workspace{ID: ws, Name: "Acme", Type: wsType, PCNodeID: "pc1"})
	w.Seed("pc1", ngac.PCName(ws), "PC", nil)
	w.Seed("ua-members", ngac.MembersUAName(ws), "UA", nil)
	w.Seed("ua-owners", ngac.OwnersUAName(ws), "UA", nil)
	w.Assign("ua-members", "pc1")
	w.Assign("ua-owners", "pc1")
	for _, p := range []struct{ node, name, group string }{
		{"n-alice", "Alice", "ua-members"}, {"n-bob", "Bob", "ua-members"},
		{"n-carol", "Carol", "ua-members"}, {"n-olga", "Olga", "ua-owners"},
	} {
		w.Seed(p.node, ngac.UserNodeName(ngac.UserID(p.node)), "U", nil)
		w.Assign(p.node, p.group)
		st.people[p.node] = &store.Person{NodeID: p.node, DisplayName: p.name}
	}
	w.Seed("n-gus", "U_gus", "U", nil) // belongs to no workspace

	w.Seed("oa-channels", ngac.ChannelsOAName(ws), "OA", nil)
	w.Assign("oa-channels", "pc1")
	w.Seed("oa-ch-team", ngac.ChannelContentOAName("ch-team"), "OA", nil)
	w.Assign("oa-ch-team", "oa-channels")
	w.Seed("ua-ch-team", ngac.ChannelMembersUAName("ch-team"), "UA", nil)
	w.Assign("ua-ch-team", "pc1")
	w.Assocs[[2]string{"ua-ch-team", "oa-ch-team"}] = ngac.ChannelMemberOps()
	for _, n := range []string{"n-alice", "n-olga"} {
		w.Assign(n, "ua-ch-team")
	}
	st.channels["ch-team"] = &store.Channel{ID: "ch-team", WorkspaceID: ws, Type: "private", Name: "Team", ContentOAID: "oa-ch-team"}
	st.members["ch-team"] = []string{"n-alice", "n-olga"}
	w.Seed("oa-ch-dm", ngac.ChannelContentOAName("ch-dm"), "OA", nil)
	w.Assign("oa-ch-dm", "oa-channels")
	w.Assocs[[2]string{"ua-ch-team", "oa-ch-dm"}] = ngac.ChannelMemberOps()
	st.channels["ch-dm"] = &store.Channel{ID: "ch-dm", WorkspaceID: ws, Type: "dm", Name: "Alice, Bob", ContentOAID: "oa-ch-dm"}
	st.members["ch-dm"] = []string{"n-alice", "n-bob"}

	rec := &realtime.Recorder{}
	return &fixture{w: w, st: st, rec: rec, svc: NewService(st, w.Read(), w.Write(), rec)}
}

func kinds(cals []*CalendarView) map[string]*CalendarView {
	out := map[string]*CalendarView{}
	for _, c := range cals {
		out[c.Kind] = c
	}
	return out
}

func requireKind(t *testing.T, err error, kind error) {
	t.Helper()
	require.Error(t, err)
	assert.True(t, errors.Is(err, kind), "got %v", err)
}

func TestListCreatesTheCallersCalendarsOnce(t *testing.T) {
	f := newFixture(t)
	got, err := f.svc.ListCalendars(callerCtx("n-alice"), ws)
	require.NoError(t, err)
	require.Len(t, got, 2)

	by := kinds(got)
	personal, workspace := by[store.KindPersonal], by[store.KindWorkspace]
	require.NotNil(t, personal)
	require.NotNil(t, workspace)
	assert.Equal(t, "Alice", personal.Name, "a personal calendar is named for its owner")
	assert.Equal(t, "Acme", workspace.Name)
	assert.Equal(t, AccessManage, personal.Access)
	assert.Equal(t, AccessRead, workspace.Access, "an ordinary member only reads the workspace calendar")
	assert.True(t, personal.IsMine)
	require.NotNil(t, personal.Owner)
	assert.Equal(t, "Alice", personal.Owner.DisplayName)
	assert.Equal(t, personHue("n-alice"), personal.Owner.Hue)
	assert.Equal(t, personal.Owner.Hue, personal.Color, "a personal calendar starts in its owner's hue")
	assert.Equal(t, -1, workspace.Color, "the workspace calendar starts in the accent colour")
	assert.Nil(t, personal.Channel)

	// The graph holds exactly what the spec says: an OA under the policy class,
	// the owner's personal UA with the owner operations; Members read, Owners all.
	pOA := ngac.CalendarOAName(ngac.CalendarID(personal.ID))
	wOA := ngac.CalendarOAName(ngac.CalendarID(workspace.ID))
	log := f.w.Log
	assert.Contains(t, log, "assign "+pOA+">"+ngac.PCName(ws))
	assert.Contains(t, log, "associate "+ngac.PersonalUAName("n-alice")+">"+pOA+" [read write share manage]")
	assert.Contains(t, log, "assign "+wOA+">"+ngac.PCName(ws))
	assert.Contains(t, log, "associate "+ngac.MembersUAName(ws)+">"+wOA+" [read]")
	assert.Contains(t, log, "associate "+ngac.OwnersUAName(ws)+">"+wOA+" [read write share manage]")

	// Opening it again creates no second calendar and writes nothing.
	before := f.w.Mutations()
	again, err := f.svc.ListCalendars(callerCtx("n-alice"), ws)
	require.NoError(t, err)
	assert.Equal(t, personal.ID, kinds(again)[store.KindPersonal].ID)
	assert.Equal(t, workspace.ID, kinds(again)[store.KindWorkspace].ID)
	assert.Equal(t, before, f.w.Mutations(), "a second list must not touch the graph")
	assert.Len(t, f.st.calendars, 2)
}

func TestTwoPeopleShareOneWorkspaceCalendar(t *testing.T) {
	f := newFixture(t)
	a, err := f.svc.ListCalendars(callerCtx("n-alice"), ws)
	require.NoError(t, err)
	o, err := f.svc.ListCalendars(callerCtx("n-olga"), ws)
	require.NoError(t, err)
	assert.Equal(t, kinds(a)[store.KindWorkspace].ID, kinds(o)[store.KindWorkspace].ID)
	assert.Equal(t, AccessManage, kinds(o)[store.KindWorkspace].Access, "an owner manages the workspace calendar")
	assert.Len(t, f.st.calendars, 3, "two personal calendars and one workspace calendar")
}

func TestPersonalWorkspaceHasOnlyThePersonalCalendar(t *testing.T) {
	f := newFixtureOfType(t, "personal")
	got, err := f.svc.ListCalendars(callerCtx("n-alice"), ws)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, store.KindPersonal, got[0].Kind)
}

func TestListRefusesWhoIsNotInTheWorkspace(t *testing.T) {
	f := newFixture(t)
	_, err := f.svc.ListCalendars(callerCtx("n-gus"), ws)
	requireKind(t, err, ErrAccessDenied)
	assert.Empty(t, f.w.Log, "nothing is created for someone outside the workspace")

	// A token scoped to another workspace cannot name this one.
	_, err = f.svc.ListCalendars(callerCtx("n-alice"), "ws-other")
	requireKind(t, err, ErrAccessDenied)

	_, err = f.svc.ListCalendars(context.Background(), ws)
	requireKind(t, err, ErrAccessDenied)
}

func TestFirstUseRacedByAnotherRequestConverges(t *testing.T) {
	f := newFixture(t)
	// Another request creates the same calendar while this one is writing.
	var winner store.Calendar
	f.st.onInsert = func(c *store.Calendar) {
		winner = *c
		f.st.mu.Lock()
		f.st.calendars[c.ID] = &winner
		f.st.mu.Unlock()
	}
	got, err := f.svc.ListCalendars(callerCtx("n-alice"), ws)
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, winner.ID, kinds(got)[store.KindPersonal].ID)
	assert.Len(t, f.st.calendars, 2)
	assert.NotContains(t, f.w.Log, "delete "+ngac.CalendarOAName(ngac.CalendarID(winner.ID)))
}

func TestFailedProvisioningLeavesNothingBehind(t *testing.T) {
	// Wherever the writes break, no node outlives the failed run and no row exists.
	for failAt := 1; failAt <= 5; failAt++ {
		f := newFixture(t)
		f.w.FailAt = failAt
		_, err := f.svc.ListCalendars(callerCtx("n-alice"), ws)
		require.Error(t, err, "failAt %d", failAt)
		for _, n := range f.w.Nodes {
			assert.NotContains(t, n.Name, "Cal_", "failAt %d left %s", failAt, n.Name)
		}
		assert.Empty(t, f.st.calendars, "failAt %d", failAt)
	}
}

func TestShareLevelsGrantExactlyTheirOperations(t *testing.T) {
	f := newFixture(t)
	mine := kinds(mustList(t, f, "n-alice"))[store.KindPersonal]
	ctx := callerCtx("n-alice")
	oa := f.w.NodeNamed(ngac.TypeOA, ngac.CalendarOAName(ngac.CalendarID(mine.ID)))
	bobUA := ngac.PersonalUAName("n-bob")

	sh, err := f.svc.PutShare(ctx, ws, mine.ID, "n-bob", ngac.CalendarShareView)
	require.NoError(t, err)
	assert.Equal(t, "Bob", sh.Grantee.DisplayName)
	assert.Equal(t, ngac.CalendarShareView, sh.Level)
	assert.Contains(t, f.w.Log, "associate "+bobUA+">"+ngac.CalendarOAName(ngac.CalendarID(mine.ID))+" [read]")
	assert.True(t, f.w.Allowed("n-bob", oa, ngac.OpRead))
	assert.False(t, f.w.Allowed("n-bob", oa, ngac.OpWrite), "a view share must not write")

	// Changing the level replaces the association; it never leaves both.
	_, err = f.svc.PutShare(ctx, ws, mine.ID, "n-bob", ngac.CalendarShareEdit)
	require.NoError(t, err)
	assert.True(t, f.w.Allowed("n-bob", oa, ngac.OpWrite))
	assert.False(t, f.w.Allowed("n-bob", oa, ngac.OpShare), "an edit share must not share")
	assert.Len(t, f.w.Assocs[[2]string{f.w.NodeNamed(ngac.TypeUA, bobUA), oa}], 2)

	_, err = f.svc.PutShare(ctx, ws, mine.ID, "n-bob", ngac.CalendarShareManage)
	require.NoError(t, err)
	assert.True(t, f.w.Allowed("n-bob", oa, ngac.OpManage))
	assert.True(t, f.w.Allowed("n-bob", oa, ngac.OpShare))

	shares, err := f.svc.ListShares(ctx, ws, mine.ID)
	require.NoError(t, err)
	require.Len(t, shares, 1, "one share per person")
	assert.Equal(t, ngac.CalendarShareManage, shares[0].Level)
	assert.Equal(t, "Bob", shares[0].Grantee.DisplayName)

	// The grantee now sees the calendar on their list at the level given, and
	// has it subscribed.
	bobs := kinds(mustList(t, f, "n-bob"))
	var found *CalendarView
	for _, c := range mustList(t, f, "n-bob") {
		if c.ID == mine.ID {
			found = c
		}
	}
	require.NotNil(t, found, "a shared calendar is on the grantee's list")
	assert.Equal(t, AccessManage, found.Access)
	assert.False(t, found.IsMine)
	assert.NotNil(t, bobs[store.KindPersonal])
}

func TestSharingIsRefusedToWhoLacksShareAndManage(t *testing.T) {
	f := newFixture(t)
	mine := kinds(mustList(t, f, "n-alice"))[store.KindPersonal]
	ctx := callerCtx("n-alice")
	_, err := f.svc.PutShare(ctx, ws, mine.ID, "n-bob", ngac.CalendarShareEdit)
	require.NoError(t, err)
	logged := len(f.w.Log)

	// A stranger of the same workspace.
	_, err = f.svc.PutShare(callerCtx("n-carol"), ws, mine.ID, "n-carol", ngac.CalendarShareView)
	requireKind(t, err, ErrAccessDenied)
	_, err = f.svc.ListShares(callerCtx("n-carol"), ws, mine.ID)
	requireKind(t, err, ErrAccessDenied)
	requireKind(t, f.svc.DeleteShare(callerCtx("n-carol"), ws, mine.ID, "n-bob"), ErrAccessDenied)

	// A grantee at "edit" changes events but does not share or revoke.
	_, err = f.svc.PutShare(callerCtx("n-bob"), ws, mine.ID, "n-carol", ngac.CalendarShareView)
	requireKind(t, err, ErrAccessDenied)
	requireKind(t, f.svc.DeleteShare(callerCtx("n-bob"), ws, mine.ID, "n-bob"), ErrAccessDenied)

	// Sharing takes both operations: one of them alone is not enough.
	oa := f.w.NodeNamed(ngac.TypeOA, ngac.CalendarOAName(ngac.CalendarID(mine.ID)))
	f.w.Allow("n-carol", oa, ngac.OpManage)
	_, err = f.svc.PutShare(callerCtx("n-carol"), ws, mine.ID, "n-bob", ngac.CalendarShareView)
	requireKind(t, err, ErrAccessDenied)
	f.w.Allow("n-olga", oa, ngac.OpShare)
	_, err = f.svc.PutShare(callerCtx("n-olga"), ws, mine.ID, "n-bob", ngac.CalendarShareView)
	requireKind(t, err, ErrAccessDenied)

	assert.Len(t, f.w.Log, logged, "a refused share writes nothing to the graph")
	shares, err := f.svc.ListShares(ctx, ws, mine.ID)
	require.NoError(t, err)
	assert.Len(t, shares, 1)
}

func TestShareInputIsValidated(t *testing.T) {
	f := newFixture(t)
	mine := kinds(mustList(t, f, "n-alice"))[store.KindPersonal]
	ctx := callerCtx("n-alice")
	logged := len(f.w.Log)

	_, err := f.svc.PutShare(ctx, ws, mine.ID, "n-bob", "owner")
	requireKind(t, err, ErrInvalidInput)
	_, err = f.svc.PutShare(ctx, ws, mine.ID, "n-bob", "read write share manage")
	requireKind(t, err, ErrInvalidInput)
	_, err = f.svc.PutShare(ctx, ws, mine.ID, "n-alice", ngac.CalendarShareView)
	requireKind(t, err, ErrInvalidInput) // never with oneself
	_, err = f.svc.PutShare(ctx, ws, mine.ID, "n-gus", ngac.CalendarShareView)
	requireKind(t, err, ErrAccessDenied) // not a person of this workspace
	_, err = f.svc.PutShare(ctx, ws, mine.ID, "", ngac.CalendarShareView)
	requireKind(t, err, ErrInvalidInput)

	assert.Len(t, f.w.Log, logged, "a refused share writes nothing to the graph")
	assert.Empty(t, f.st.shares)

	// Only personal calendars are shared.
	wsCal := kinds(mustList(t, f, "n-olga"))[store.KindWorkspace]
	_, err = f.svc.PutShare(callerCtx("n-olga"), ws, wsCal.ID, "n-bob", ngac.CalendarShareView)
	requireKind(t, err, ErrInvalidInput)
}

func TestRevokedShareDeniesTheNextDecision(t *testing.T) {
	f := newFixture(t)
	mine := kinds(mustList(t, f, "n-alice"))[store.KindPersonal]
	ctx := callerCtx("n-alice")
	oa := f.w.NodeNamed(ngac.TypeOA, ngac.CalendarOAName(ngac.CalendarID(mine.ID)))
	_, err := f.svc.PutShare(ctx, ws, mine.ID, "n-bob", ngac.CalendarShareEdit)
	require.NoError(t, err)
	require.True(t, f.w.Allowed("n-bob", oa, ngac.OpRead))

	require.NoError(t, f.svc.DeleteShare(ctx, ws, mine.ID, "n-bob"))
	assert.False(t, f.w.Allowed("n-bob", oa, ngac.OpRead))
	assert.False(t, f.w.Allowed("n-bob", oa, ngac.OpWrite))
	shares, err := f.svc.ListShares(ctx, ws, mine.ID)
	require.NoError(t, err)
	assert.Empty(t, shares)

	// A colleague still sees when Alice is busy, nothing more.
	var seen *CalendarView
	for _, c := range mustList(t, f, "n-bob") {
		if c.ID == mine.ID {
			seen = c
		}
	}
	if seen != nil {
		assert.Equal(t, AccessFreeBusy, seen.Access)
	}
}

func TestShareAnnouncesToTheOwnerAndTheGranteeOnly(t *testing.T) {
	f := newFixture(t)
	mine := kinds(mustList(t, f, "n-alice"))[store.KindPersonal]
	_, err := f.svc.PutShare(callerCtx("n-alice"), ws, mine.ID, "n-bob", ngac.CalendarShareView)
	require.NoError(t, err)
	evs := f.rec.Events()
	require.NotEmpty(t, evs)
	e := evs[len(evs)-1]
	assert.Equal(t, realtime.DomainCalendar, e.Domain)
	assert.Equal(t, realtime.KindShareCreated, e.Kind)
	assert.Equal(t, ws, e.WorkspaceID)
	assert.Equal(t, "ws1", e.TenantID)
	assert.ElementsMatch(t, []string{"n-alice", "n-bob"}, e.UserNodeIDs)
	assert.Equal(t, []string{mine.ID}, e.IDs)
	assert.NoError(t, e.Validate())
}

func TestSubscribeToAColleagueNeedsTheSameWorkspace(t *testing.T) {
	f := newFixture(t)
	got, err := f.svc.Subscribe(callerCtx("n-alice"), ws, "n-bob")
	require.NoError(t, err)
	assert.Equal(t, "Bob", got.Name)
	assert.Equal(t, AccessFreeBusy, got.Access, "without a share a colleague shows busy ranges only")
	assert.False(t, got.IsMine)

	// On the list from now on, and not twice.
	n := 0
	for _, c := range mustList(t, f, "n-alice") {
		if c.ID == got.ID {
			n++
		}
	}
	assert.Equal(t, 1, n)

	// Deny: someone who is not of the workspace.
	before := f.w.Mutations()
	_, err = f.svc.Subscribe(callerCtx("n-alice"), ws, "n-gus")
	requireKind(t, err, ErrAccessDenied)
	assert.Equal(t, before, f.w.Mutations())
	_, err = f.svc.Subscribe(callerCtx("n-gus"), ws, "n-bob")
	requireKind(t, err, ErrAccessDenied)
	_, err = f.svc.Subscribe(callerCtx("n-alice"), ws, "")
	requireKind(t, err, ErrInvalidInput)
}

func TestUnsubscribeOnlyRemovesAnotherPersonsCalendar(t *testing.T) {
	f := newFixture(t)
	bobs, err := f.svc.Subscribe(callerCtx("n-alice"), ws, "n-bob")
	require.NoError(t, err)
	require.NoError(t, f.svc.Unsubscribe(callerCtx("n-alice"), ws, bobs.ID))
	for _, c := range mustList(t, f, "n-alice") {
		assert.NotEqual(t, bobs.ID, c.ID)
	}
	mine := kinds(mustList(t, f, "n-alice"))[store.KindPersonal]
	requireKind(t, f.svc.Unsubscribe(callerCtx("n-alice"), ws, mine.ID), ErrInvalidInput)
}

func TestSubscriptionSettings(t *testing.T) {
	f := newFixture(t)
	mine := kinds(mustList(t, f, "n-alice"))[store.KindPersonal]
	ctx := callerCtx("n-alice")
	color, hidden := 5, false
	got, err := f.svc.SetSubscription(ctx, ws, mine.ID, SubscriptionInput{Color: &color, Visible: &hidden})
	require.NoError(t, err)
	assert.Equal(t, 5, got.Color)
	assert.False(t, got.Visible)

	// Only the fields given change.
	visible := true
	got, err = f.svc.SetSubscription(ctx, ws, mine.ID, SubscriptionInput{Visible: &visible})
	require.NoError(t, err)
	assert.Equal(t, 5, got.Color)
	assert.True(t, got.Visible)

	bad := 999
	_, err = f.svc.SetSubscription(ctx, ws, mine.ID, SubscriptionInput{Color: &bad})
	requireKind(t, err, ErrInvalidInput)
	neg := -2
	_, err = f.svc.SetSubscription(ctx, ws, mine.ID, SubscriptionInput{Color: &neg})
	requireKind(t, err, ErrInvalidInput)
	accent := -1 // the accent colour is a choice too
	got, err = f.svc.SetSubscription(ctx, ws, mine.ID, SubscriptionInput{Color: &accent})
	require.NoError(t, err)
	assert.Equal(t, -1, got.Color)

	// Deny: a calendar that does not exist.
	_, err = f.svc.SetSubscription(ctx, ws, "missing", SubscriptionInput{Color: &color})
	requireKind(t, err, ErrNotFound)
}

func TestGroupCalendarIsCreatedBeneathTheContentOAForMembers(t *testing.T) {
	f := newFixture(t)
	got, err := f.svc.EnsureChannelCalendar(callerCtx("n-alice"), ws, "ch-team")
	require.NoError(t, err)
	assert.Equal(t, store.KindChannel, got.Kind)
	assert.Equal(t, "Team", got.Name)
	assert.Equal(t, AccessWrite, got.Access)

	oa := ngac.CalendarOAName(ngac.CalendarID(got.ID))
	assert.Contains(t, f.w.Log, "assign "+oa+">"+ngac.ChannelContentOAName("ch-team"))
	for _, l := range f.w.Log {
		assert.NotContains(t, l, "associate", "members inherit from the content OA; the group calendar needs no association of its own")
	}

	// Again: the same calendar, nothing written.
	before := f.w.Mutations()
	again, err := f.svc.EnsureChannelCalendar(callerCtx("n-olga"), ws, "ch-team")
	require.NoError(t, err)
	assert.Equal(t, got.ID, again.ID)
	assert.Equal(t, before, f.w.Mutations())

	// It joins the lists of the group's members, and only theirs.
	has := func(node string) bool {
		for _, c := range mustList(t, f, node) {
			if c.ID == got.ID {
				return true
			}
		}
		return false
	}
	assert.True(t, has("n-alice"))
	assert.True(t, has("n-olga"))
	assert.False(t, has("n-carol"))
}

func TestGroupCalendarRefusals(t *testing.T) {
	f := newFixture(t)

	// Deny: a workspace member who is not in the group.
	_, err := f.svc.EnsureChannelCalendar(callerCtx("n-carol"), ws, "ch-team")
	requireKind(t, err, ErrAccessDenied)
	assert.Empty(t, f.w.Log)
	assert.Empty(t, f.st.calendars)

	// Direct messages have no calendar, and nothing is created for one.
	f.w.Allow("n-alice", "oa-ch-dm", ngac.OpWrite)
	_, err = f.svc.EnsureChannelCalendar(callerCtx("n-alice"), ws, "ch-dm")
	requireKind(t, err, ErrInvalidInput)
	assert.Empty(t, f.w.Created())

	// A conversation of another workspace is not found from this one.
	f.st.channels["ch-far"] = &store.Channel{ID: "ch-far", WorkspaceID: "ws9", Type: "private", Name: "Far", ContentOAID: "x"}
	_, err = f.svc.EnsureChannelCalendar(callerCtx("n-alice"), ws, "ch-far")
	requireKind(t, err, ErrNotFound)
	_, err = f.svc.EnsureChannelCalendar(callerCtx("n-alice"), ws, "nope")
	requireKind(t, err, ErrNotFound)
}

func TestGroupCalendarLeavesTheListOfWhoLeftTheGroup(t *testing.T) {
	f := newFixture(t)
	_, err := f.svc.EnsureChannelCalendar(callerCtx("n-alice"), ws, "ch-team")
	require.NoError(t, err)
	// The membership cache still says Alice is in the group, but the graph no
	// longer agrees: the policy decides.
	f.w.Parents["n-alice"] = []string{"ua-members"}
	for _, c := range mustList(t, f, "n-alice") {
		assert.NotEqual(t, store.KindChannel, c.Kind, "no read on the group's OA, no calendar")
	}
}

func TestACalendarOfAnotherWorkspaceIsNotFound(t *testing.T) {
	f := newFixture(t)
	f.st.calendars["foreign"] = &store.Calendar{ID: "foreign", WorkspaceID: "ws9", Kind: store.KindWorkspace, NgacOAID: "x"}
	_, err := f.svc.ListShares(callerCtx("n-alice"), ws, "foreign")
	requireKind(t, err, ErrNotFound)
	bad := 1
	_, err = f.svc.SetSubscription(callerCtx("n-alice"), ws, "foreign", SubscriptionInput{Color: &bad})
	requireKind(t, err, ErrNotFound)
}

func mustList(t *testing.T, f *fixture, node string) []*CalendarView {
	t.Helper()
	got, err := f.svc.ListCalendars(callerCtx(node), ws)
	require.NoError(t, err)
	return got
}
