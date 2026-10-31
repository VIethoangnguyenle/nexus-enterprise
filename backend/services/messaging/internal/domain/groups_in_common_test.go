package domain_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/ngac"
	"ngac-platform/pkg/grpcauth"
	pb "ngac-platform/proto/messaging"
	"ngac-platform/services/messaging/internal/domain"
	"ngac-platform/testutil"
)

// The people of the "groups in common" world: the viewer asking, the person
// whose profile they opened, and a third member.
const (
	commonViewer = "ngac-common-viewer"
	commonOther  = "ngac-common-other"
)

// commonChannel writes a channel of the given type in workspace ws, with a
// content OA and a members UA in ngac_nodes, and returns its id, OA and UA.
func (f *fixture) commonChannel(t *testing.T, ws, name, kind string) (chID, oaID, uaID string) {
	t.Helper()
	ctx := context.Background()
	time.Sleep(time.Microsecond)
	n := time.Now().UnixNano()
	chID = fmt.Sprintf("test-common-ch-%d", n)
	oaID = fmt.Sprintf("test-common-oa-%d", n)
	uaID = fmt.Sprintf("test-common-ua-%d", n)
	_, err := f.pool.Exec(ctx, `INSERT INTO ngac_nodes (id, name, node_type) VALUES ($1, $2, $3), ($4, $5, $6)`,
		oaID, ngac.ChannelContentOAName(ngac.ChannelID(chID)), ngac.TypeOA,
		uaID, ngac.ChannelMembersUAName(ngac.ChannelID(chID)), ngac.TypeUA)
	require.NoError(t, err)
	_, err = f.pool.Exec(ctx,
		`INSERT INTO channels (id, name, channel_type, workspace_id, ngac_oa_id, ngac_ua_id, created_at) VALUES ($1, $2, $3, $4, $5, $6, now())`,
		chID, name, kind, ws, oaID, uaID)
	require.NoError(t, err)
	t.Cleanup(func() {
		f.pool.Exec(ctx, `DELETE FROM channels WHERE id = $1`, chID)
		f.pool.Exec(ctx, `DELETE FROM ngac_nodes WHERE id IN ($1, $2)`, oaID, uaID)
	})
	return chID, oaID, uaID
}

// seat puts each node directly under the channel's members UA, as the graph
// holds a group's membership.
func (f *fixture) seat(uaID string, nodes ...string) {
	f.read.mu.Lock()
	defer f.read.mu.Unlock()
	if f.read.edges == nil {
		f.read.edges = map[[2]string]bool{}
	}
	for _, n := range nodes {
		f.read.edges[[2]string{n, uaID}] = true
	}
}

// readable lets the viewer read the channel's content OA.
func (f *fixture) readable(oaID string) {
	f.read.mu.Lock()
	defer f.read.mu.Unlock()
	f.read.allow[grant{commonViewer, oaID, ngac.OpRead}] = true
}

func (f *fixture) asViewer() context.Context {
	return grpcauth.WithCaller(context.Background(), grpcauth.Caller{UserID: "viewer", NGACNodeID: commonViewer, TenantID: f.wsID})
}

func channelNames(list []*pb.Channel) []string {
	out := make([]string, 0, len(list))
	for _, c := range list {
		out = append(out, c.Name)
	}
	return out
}

// A group is in common only when the viewer is in it, may read it, and the
// other person is in it too, in the workspace asked about. Every other channel
// is left out, the one the viewer could read but is not a member of included.
func TestGroupsInCommon_ListsOnlyGroupsBothAreIn(t *testing.T) {
	f := newFixture(t)
	f.read.addMember(f.wsID, commonViewer, commonOther)

	// Both in it, viewer reads it: listed (#general and a private group alike).
	_, generalOA, generalUA := f.commonChannel(t, f.wsID, "common-general", "workspace")
	f.seat(generalUA, commonViewer, commonOther)
	f.readable(generalOA)
	_, bothOA, bothUA := f.commonChannel(t, f.wsID, "common-both", "private")
	f.seat(bothUA, commonViewer, commonOther)
	f.readable(bothOA)

	// Deny: the other person's group the viewer is not in, even though the policy lets the viewer read it.
	_, theirsOA, theirsUA := f.commonChannel(t, f.wsID, "common-theirs-only", "private")
	f.seat(theirsUA, commonOther)
	f.readable(theirsOA)

	// Deny: the viewer's group the other person is not in.
	_, mineOA, mineUA := f.commonChannel(t, f.wsID, "common-mine-only", "private")
	f.seat(mineUA, commonViewer)
	f.readable(mineOA)

	// Deny: both seated, but the policy refuses the viewer read (the read is re-checked).
	_, _, unreadableUA := f.commonChannel(t, f.wsID, "common-unreadable", "private")
	f.seat(unreadableUA, commonViewer, commonOther)

	// Deny: a direct message between the two is not a group.
	_, dmOA, dmUA := f.commonChannel(t, f.wsID, "common-dm", "dm")
	f.seat(dmUA, commonViewer, commonOther)
	f.readable(dmOA)

	got, err := f.svc.GroupsInCommon(f.asViewer(), f.wsID, commonViewer, commonOther)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"common-general", "common-both"}, channelNames(got))
}

// A group of another workspace that both people are in is never listed when
// the viewer asks about this one.
func TestGroupsInCommon_OtherWorkspaceGroupIsNotListed(t *testing.T) {
	f := newFixture(t)
	f.read.addMember(f.wsID, commonViewer, commonOther)
	wsB, _ := testutil.CreateWorkspace(t, f.pool, f.userID)

	_, oaB, uaB := f.commonChannel(t, wsB, "common-in-b", "private")
	f.seat(uaB, commonViewer, commonOther)
	f.readable(oaB)

	got, err := f.svc.GroupsInCommon(f.asViewer(), f.wsID, commonViewer, commonOther)
	require.NoError(t, err)
	assert.Empty(t, got)
}

// Someone who is not a member of the workspace is refused, and nothing about
// its groups is asked of the policy.
func TestGroupsInCommon_NonMemberIsDenied(t *testing.T) {
	f := newFixture(t)
	f.read.addMember(f.wsID, commonOther) // only the other person belongs here
	_, oa, ua := f.commonChannel(t, f.wsID, "common-closed", "private")
	f.seat(ua, commonViewer, commonOther)
	f.readable(oa)

	got, err := f.svc.GroupsInCommon(f.asViewer(), f.wsID, commonViewer, commonOther)
	require.ErrorIs(t, err, domain.ErrAccessDenied)
	assert.Nil(t, got)
	for _, g := range f.read.asked() {
		assert.NotEqual(t, oa, g.object, "no decision on a group may be asked for a non-member")
	}
}

// A policy failure is an error, never a shorter (or longer) list.
func TestGroupsInCommon_PolicyFailureFailsClosed(t *testing.T) {
	f := newFixture(t)
	f.read.addMember(f.wsID, commonViewer, commonOther)
	_, oa, ua := f.commonChannel(t, f.wsID, "common-outage", "private")
	f.seat(ua, commonViewer, commonOther)
	f.readable(oa)
	f.read.err = errors.New("policy unreachable")

	got, err := f.svc.GroupsInCommon(f.asViewer(), f.wsID, commonViewer, commonOther)
	require.Error(t, err)
	assert.Nil(t, got)
}

// The membership of both people is read once each, whatever the number of
// groups: two parent lookups and no per-group assignment check, so a profile in
// a workspace with many groups costs the policy the same as one with a few.
func TestGroupsInCommon_ReadsMembershipOnceForEveryGroup(t *testing.T) {
	f := newFixture(t)
	f.read.addMember(f.wsID, commonViewer, commonOther)
	var uas []string
	for i := 0; i < 6; i++ {
		_, oa, ua := f.commonChannel(t, f.wsID, fmt.Sprintf("common-many-%d", i), "private")
		f.seat(ua, commonViewer, commonOther)
		f.readable(oa)
		uas = append(uas, ua)
	}
	// Deny inside the batch: one more group only the viewer is in.
	_, soloOA, soloUA := f.commonChannel(t, f.wsID, "common-many-solo", "private")
	f.seat(soloUA, commonViewer)
	f.readable(soloOA)

	got, err := f.svc.GroupsInCommon(f.asViewer(), f.wsID, commonViewer, commonOther)

	require.NoError(t, err)
	assert.Len(t, got, 6)
	assert.NotContains(t, channelNames(got), "common-many-solo")
	f.read.mu.Lock()
	defer f.read.mu.Unlock()
	assert.ElementsMatch(t, []string{commonViewer, commonOther}, f.read.parentsAsk, "one parent lookup per person")
	for _, a := range f.read.assignedAsk {
		assert.NotContains(t, append(uas, soloUA), a[1], "no per-group assignment check")
	}
}

// Deny: a membership lookup that fails fails the whole answer.
func TestGroupsInCommon_MembershipLookupFailureFailsClosed(t *testing.T) {
	f := newFixture(t)
	f.read.addMember(f.wsID, commonViewer, commonOther)
	_, oa, ua := f.commonChannel(t, f.wsID, "common-parents-down", "private")
	f.seat(ua, commonViewer, commonOther)
	f.readable(oa)
	f.read.parentsErr = errors.New("policy unreachable")

	got, err := f.svc.GroupsInCommon(f.asViewer(), f.wsID, commonViewer, commonOther)
	require.Error(t, err)
	assert.Nil(t, got)
}

// The question needs a workspace and another person; asking about oneself is not one.
func TestGroupsInCommon_RejectsAMalformedQuestion(t *testing.T) {
	f := newFixture(t)
	f.read.addMember(f.wsID, commonViewer, commonOther)
	for name, in := range map[string][3]string{
		"no workspace": {"", commonViewer, commonOther},
		"no person":    {f.wsID, commonViewer, ""},
		"oneself":      {f.wsID, commonViewer, commonViewer},
		"no caller":    {f.wsID, "", commonOther},
	} {
		_, err := f.svc.GroupsInCommon(f.asViewer(), in[0], in[1], in[2])
		assert.True(t, errors.Is(err, domain.ErrInvalidInput) || errors.Is(err, domain.ErrAccessDenied), name+": got %v", err)
	}
}
