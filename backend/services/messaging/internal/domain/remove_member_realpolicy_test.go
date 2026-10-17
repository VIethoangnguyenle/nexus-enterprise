//go:build realpolicy

package domain_test

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"ngac-platform/ngac"
	policypb "ngac-platform/proto/policy"
	"ngac-platform/services/messaging/internal/domain"
	"ngac-platform/services/messaging/internal/store"
	"ngac-platform/testutil"
)

// Taking someone out of a group is for them (leaving), for the person who made
// the group, and for a holder of manage on it. Any member holds invite, which is
// not enough. A DM is a fixed pair and #general follows workspace membership, so
// neither loses people this way. The channel_members copy follows the graph.

func (w *rpWorld) inChannel(p rpPerson, ch interface{ GetNgacUaId() string }) bool {
	return w.assigned(p.node, ch.GetNgacUaId())
}

// cached reports whether channel_members still has the person's row.
func (w *rpWorld) cached(p rpPerson, chID string) bool {
	w.t.Helper()
	var n int
	require.NoError(w.t, w.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM channel_members WHERE channel_id = $1 AND ngac_node_id = $2`, chID, p.node).Scan(&n))
	return n == 1
}

func TestRealPolicy_RemovingAGroupMember(t *testing.T) {
	w := newRPWorld(t)
	g := w.createGroup(w.creator, "rp-remove")
	bystander := w.person(w.g.Members)
	ctx := w.as(w.creator)
	for _, p := range []rpPerson{w.member, w.latecomer} {
		require.NoError(t, w.svc.AddMember(ctx, g.Id, w.creator.node, p.node))
	}
	// The fixture is meaningful: the people are in, with their cache rows.
	for _, p := range []rpPerson{w.creator, w.member, w.latecomer} {
		require.True(t, w.inChannel(p, g), "in the group")
		require.True(t, w.cached(p, g.Id), "cached")
	}

	// Deny: invite (which every member holds) is not the right to remove others.
	stays := func(name string, p rpPerson) {
		t.Helper()
		assert.True(t, w.inChannel(p, g), name+": still in the group")
		assert.True(t, w.cached(p, g.Id), name+": cache row kept")
	}
	err := w.svc.RemoveMember(w.as(w.member), g.Id, w.member.node, w.latecomer.node)
	assert.ErrorIs(t, err, domain.ErrAccessDenied, "a member removes another member")
	stays("latecomer", w.latecomer)
	err = w.svc.RemoveMember(w.as(w.member), g.Id, w.member.node, w.creator.node)
	assert.ErrorIs(t, err, domain.ErrAccessDenied, "a member removes the person who made the group")
	stays("creator", w.creator)
	err = w.svc.RemoveMember(w.as(bystander), g.Id, bystander.node, w.latecomer.node)
	assert.ErrorIs(t, err, domain.ErrAccessDenied, "a workspace member outside the group")
	stays("latecomer", w.latecomer)
	err = w.svc.RemoveMember(w.as(w.outsider), g.Id, w.outsider.node, w.latecomer.node)
	assert.ErrorIs(t, err, domain.ErrAccessDenied, "a person of another workspace")
	stays("latecomer", w.latecomer)

	// Not a person who is in the group: nothing to remove.
	nested := testutil.NewNode(t, w.write, "UA_rp_nested_"+w.g.WorkspaceID, ngac.TypeUA, g.NgacUaId)
	err = w.svc.RemoveMember(ctx, g.Id, w.creator.node, nested)
	assert.ErrorIs(t, err, domain.ErrNotFound, "a UA under the group's UA")
	assert.True(t, w.assigned(nested, g.NgacUaId), "the UA stays")
	err = w.svc.RemoveMember(ctx, g.Id, w.creator.node, bystander.node)
	assert.ErrorIs(t, err, domain.ErrNotFound, "someone who is not in the group")
	err = w.svc.RemoveMember(ctx, g.Id, w.creator.node, "no-such-node")
	assert.ErrorIs(t, err, domain.ErrNotFound, "a node that does not exist")

	// Allow: leaving.
	require.NoError(t, w.svc.RemoveMember(w.as(w.member), g.Id, w.member.node, w.member.node), "leaving")
	assert.False(t, w.inChannel(w.member, g))
	assert.False(t, w.cached(w.member, g.Id), "the cache follows")
	_, err = w.svc.GetChannel(w.as(w.member), g.Id, w.member.node)
	assert.ErrorIs(t, err, domain.ErrAccessDenied, "a person who left reads nothing")

	// Allow: the person who made the group removes someone.
	require.NoError(t, w.svc.RemoveMember(ctx, g.Id, w.creator.node, w.latecomer.node), "the creator removes a member")
	assert.False(t, w.inChannel(w.latecomer, g))
	assert.False(t, w.cached(w.latecomer, g.Id))

	// Allow: an owner, who is not in the group, holds manage on it.
	require.NoError(t, w.svc.AddMember(ctx, g.Id, w.creator.node, w.latecomer.node))
	require.False(t, w.inChannel(w.owner, g))
	require.NoError(t, w.svc.RemoveMember(w.as(w.owner), g.Id, w.owner.node, w.latecomer.node), "an owner removes a member")
	assert.False(t, w.inChannel(w.latecomer, g))
	assert.False(t, w.cached(w.latecomer, g.Id))

	// Allow: the creator may leave their own group.
	require.NoError(t, w.svc.RemoveMember(ctx, g.Id, w.creator.node, w.creator.node))
	assert.False(t, w.inChannel(w.creator, g))
}

// A group's creator is protected from other members, not from a manage holder:
// the same fixture, the same target.
func TestRealPolicy_OnlyAManagerRemovesTheCreator(t *testing.T) {
	w := newRPWorld(t)
	g := w.createGroup(w.creator, "rp-remove-creator")
	require.NoError(t, w.svc.AddMember(w.as(w.creator), g.Id, w.creator.node, w.member.node))

	err := w.svc.RemoveMember(w.as(w.member), g.Id, w.member.node, w.creator.node)
	assert.ErrorIs(t, err, domain.ErrAccessDenied)
	assert.True(t, w.inChannel(w.creator, g))

	require.NoError(t, w.svc.RemoveMember(w.as(w.owner), g.Id, w.owner.node, w.creator.node))
	assert.False(t, w.inChannel(w.creator, g))
	assert.False(t, w.cached(w.creator, g.Id))
}

// #general's people are the workspace's: nobody is taken out of it, not even by an
// owner with manage or by themselves. A DM is a pair: neither side removes the other
// or leaves. The cache row stays with the graph edge in both.
func TestRealPolicy_NoRemovalFromGeneralOrADM(t *testing.T) {
	w := newDMWorld(t)
	general := w.provisionGeneral()
	require.NoError(t, w.join(w.member))
	dm := w.mustDM(w.creator, w.member)
	group := w.createGroup(w.creator, "rp-remove-pair")
	require.NoError(t, w.svc.AddMember(w.as(w.creator), group.Id, w.creator.node, w.member.node))

	for name, c := range map[string]struct {
		ch      interface{ GetNgacUaId() string }
		id      string
		by, who rpPerson
	}{
		"an owner removes a member of #general": {general, general.Id, w.owner, w.member},
		"a member leaves #general":              {general, general.Id, w.member, w.member},
		"one side removes the other of a DM":    {dm, dm.Id, w.creator, w.member},
		"a DM side leaves":                      {dm, dm.Id, w.member, w.member},
	} {
		err := w.svc.RemoveMember(w.as(c.by), c.id, c.by.node, c.who.node)
		assert.ErrorIs(t, err, domain.ErrInvalidInput, name)
		assert.True(t, w.inChannel(c.who, c.ch), name+": still in")
		assert.True(t, w.cached(c.who, c.id), name+": cache row kept")
	}
	found, err := w.svc.FindOrCreateDM(w.as(w.creator), w.creator.id, w.creator.node, w.member.id, w.member.node)
	require.NoError(t, err)
	assert.Equal(t, dm.Id, found.Id, "the DM is still the pair's")

	// The same two people, in an ordinary group, can be taken out.
	require.NoError(t, w.svc.RemoveMember(w.as(w.creator), group.Id, w.creator.node, w.member.node))
	assert.False(t, w.inChannel(w.member, group))
}

// batchRecorder notes which objects every batch check asked about.
type batchRecorder struct {
	policypb.PolicyReadServiceClient
	mu    sync.Mutex
	asked [][]string
}

func (b *batchRecorder) BatchCheckAccess(ctx context.Context, in *policypb.BatchCheckAccessRequest, opts ...grpc.CallOption) (*policypb.BatchAccessResult, error) {
	b.mu.Lock()
	b.asked = append(b.asked, in.ObjectIds)
	b.mu.Unlock()
	return b.PolicyReadServiceClient.BatchCheckAccess(ctx, in, opts...)
}

// Listing DMs asks the policy about the caller's DMs only, not about every DM in
// the database: the two people's lists are their own, and the second person's DM
// with a third is not looked at when the first lists.
func TestRealPolicy_ListDMsLooksAtTheCallersDMsOnly(t *testing.T) {
	w := newDMWorld(t)
	mine := w.mustDM(w.creator, w.member)
	theirs := w.mustDM(w.owner, w.latecomer)

	rec := &batchRecorder{PolicyReadServiceClient: w.read}
	svc := domain.NewService(store.NewStore(w.pool), rec, w.write, rpDBAuth{pool: w.pool}, nil)

	list, err := svc.ListDMs(w.as(w.creator), w.creator.node)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, mine.Id, list[0].Id)
	require.Len(t, rec.asked, 1)
	assert.Equal(t, []string{mine.NgacOaId}, rec.asked[0], "only the caller's DM was put to the policy")

	list, err = svc.ListDMs(w.as(w.owner), w.owner.node)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, theirs.Id, list[0].Id, "and the other pair's list is theirs")
}

// The roster tells each reader whether they may remove others, so a screen shows
// the action only to them: the group's creator and a manager, in a group; nobody in
// a DM or in #general.
func TestRealPolicy_RosterSaysWhoMayRemove(t *testing.T) {
	w := newDMWorld(t)
	g := w.createGroup(w.creator, "rp-roster")
	require.NoError(t, w.svc.AddMember(w.as(w.creator), g.Id, w.creator.node, w.member.node))
	dm := w.mustDM(w.creator, w.member)
	general := w.provisionGeneral()
	require.NoError(t, w.join(w.member))

	may := func(p rpPerson, chID string) (bool, error) {
		r, err := w.svc.MemberRoster(w.as(p), chID, p.node)
		if err != nil {
			return false, err
		}
		require.NotEmpty(t, r.Members)
		return r.CanRemove, nil
	}
	got, err := w.svc.MemberRoster(w.as(w.creator), g.Id, w.creator.node)
	require.NoError(t, err)
	assert.Len(t, got.Members, 2, "the roster is the same list ListMembers gives")

	// Deny: a plain member of the group; both sides of a DM; everyone in #general.
	ok, err := may(w.member, g.Id)
	require.NoError(t, err)
	assert.False(t, ok, "a plain member")
	for name, p := range map[string]rpPerson{"DM creator": w.creator, "DM other side": w.member} {
		ok, err = may(p, dm.Id)
		require.NoError(t, err)
		assert.False(t, ok, name)
	}
	for name, p := range map[string]rpPerson{"owner": w.owner, "member": w.member} {
		ok, err = may(p, general.Id)
		require.NoError(t, err)
		assert.False(t, ok, "#general: "+name)
	}
	// Deny: someone outside the group gets no roster at all.
	_, err = w.svc.MemberRoster(w.as(w.latecomer), g.Id, w.latecomer.node)
	assert.ErrorIs(t, err, domain.ErrAccessDenied)

	// Allow, same group: the creator, and an owner who is not in it.
	ok, err = may(w.creator, g.Id)
	require.NoError(t, err)
	assert.True(t, ok, "the group's creator")
	_, err = w.svc.ListMembers(w.as(w.owner), g.Id, w.owner.node)
	assert.ErrorIs(t, err, domain.ErrAccessDenied, "an owner outside a private group cannot read its roster, so has no flag to read")
}
