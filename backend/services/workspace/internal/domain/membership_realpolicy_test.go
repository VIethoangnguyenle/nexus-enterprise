//go:build realpolicy

package domain_test

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/ngac"
	"ngac-platform/services/workspace/internal/domain"
	"ngac-platform/services/workspace/internal/store"
	"ngac-platform/testutil"
)

func TestMain(m *testing.M) { os.Exit(testutil.RunMain(m)) }

// These tests run the workspace service against the real policy service. A person
// is a member of a workspace when they sit under its Members UA. A channel's UA
// and an event's guest UA sit under the workspace's policy class too (the
// intersection rule needs them there), so reaching the class through one of them
// must not read as membership.

type wsPerson struct{ id, node string }

type wsWorld struct {
	t    *testing.T
	pool *pgxpool.Pool
	svc  *domain.Service
	g    testutil.WorkspaceGraph

	owner, member, tenantMember wsPerson
	// reach the policy class and nothing else: through a channel's UA, through an
	// event's guest UA.
	viaChannel, viaGuests wsPerson
	outsider              wsPerson
	pc                    string
}

func (w *wsWorld) person(parents ...string) wsPerson {
	w.t.Helper()
	_, write := testutil.StartPolicy(w.t)
	node := testutil.NewUser(w.t, write, parents...)
	id := "ws-rp-" + uuid.NewString()
	_, err := w.pool.Exec(context.Background(),
		`INSERT INTO users (id, username, password, ngac_node, display_name) VALUES ($1, $1, '', $2, $1)`, id, node)
	require.NoError(w.t, err)
	w.t.Cleanup(func() { w.pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, id) })
	return wsPerson{id: id, node: node}
}

func newWSWorld(t *testing.T) *wsWorld {
	t.Helper()
	ctx := context.Background()
	read, write := testutil.StartPolicy(t)
	pool := testutil.SetupTestDB(t)
	w := &wsWorld{t: t, pool: pool}
	w.g = testutil.SeedWorkspaceGraph(t, write)
	other := testutil.SeedWorkspaceGraph(t, write)
	id := ngac.WorkspaceID(w.g.WorkspaceID)

	tenantUA := testutil.NewNode(t, write, ngac.TenantMemberUAName(id), ngac.TypeUA, w.g.Members)
	chanUA := testutil.NewNode(t, write, ngac.ChannelMembersUAName("rp-ch"), ngac.TypeUA, w.g.PC)
	guestsUA := testutil.NewNode(t, write, ngac.CalendarEventGuestsUAName("rp-ev"), ngac.TypeUA, w.g.PC)

	w.owner = w.person(w.g.Owners)
	w.member = w.person(w.g.Members)
	w.tenantMember = w.person(tenantUA)
	w.viaChannel = w.person(chanUA)
	w.viaGuests = w.person(guestsUA)
	w.outsider = w.person(other.Members)

	_, err := pool.Exec(ctx, `INSERT INTO workspaces (id, name, owner_id, ngac_pc_id, type) VALUES ($1, 'ws-rp', $2, $3, 'organization')`,
		w.g.WorkspaceID, w.owner.id, w.g.PC)
	require.NoError(t, err)
	t.Cleanup(func() { pool.Exec(ctx, `DELETE FROM workspaces WHERE id = $1`, w.g.WorkspaceID) })

	st := store.New(pool)
	w.svc = domain.NewService(st, st, read, write, nil, nil)
	return w
}

func (w *wsWorld) as(p wsPerson) context.Context {
	return testutil.PolicyCaller(context.Background(), p.node)
}

func nodesOf(ms []*domain.Member) []string {
	var out []string
	for _, m := range ms {
		out = append(out, m.NGACNodeID)
	}
	return out
}

// Owners, ordinary members and tenant members are let in; a person who reaches
// the policy class through a group or through an event's guest list is not, and
// neither is a person of another workspace.
func TestRealPolicy_WorkspaceMembershipIsTheMembersUA(t *testing.T) {
	w := newWSWorld(t)
	ws := w.g.WorkspaceID

	for name, p := range map[string]wsPerson{"owner": w.owner, "member": w.member, "tenant member": w.tenantMember} {
		got, err := w.svc.ViewWorkspace(w.as(p), p.node, ws)
		require.NoError(t, err, name+": ViewWorkspace")
		assert.Equal(t, ws, got.ID)
		_, err = w.svc.ListDepartments(w.as(p), p.node, ws)
		assert.NoError(t, err, name+": ListDepartments")
		_, err = w.svc.ListMembers(w.as(p), p.node, ws)
		assert.NoError(t, err, name+": ListMembers")
		list, err := w.svc.ListAccessibleWorkspaces(w.as(p), p.node)
		require.NoError(t, err)
		assert.Contains(t, workspaceIDs(list), ws, name+": the workspace is listed")
	}

	for name, p := range map[string]wsPerson{"reaching it through a group": w.viaChannel, "reaching it through an event's guests": w.viaGuests, "another workspace's member": w.outsider} {
		_, err := w.svc.ViewWorkspace(w.as(p), p.node, ws)
		assert.ErrorIs(t, err, domain.ErrAccessDenied, name+": ViewWorkspace")
		_, err = w.svc.ListMembers(w.as(p), p.node, ws)
		assert.ErrorIs(t, err, domain.ErrAccessDenied, name+": ListMembers")
		_, err = w.svc.ListDepartments(w.as(p), p.node, ws)
		assert.ErrorIs(t, err, domain.ErrAccessDenied, name+": ListDepartments")
		list, err := w.svc.ListAccessibleWorkspaces(w.as(p), p.node)
		require.NoError(t, err)
		assert.NotContains(t, workspaceIDs(list), ws, name+": the workspace is not listed")
	}
}

// The people of a workspace are who sits under its Members UA: a group's or an
// event's UA beneath the policy class adds nobody, in the member list or in the
// admin directory.
func TestRealPolicy_PeopleListsHoldOnlyMembers(t *testing.T) {
	w := newWSWorld(t)
	ws := w.g.WorkspaceID

	members, err := w.svc.ListMembers(w.as(w.member), w.member.node, ws)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{w.owner.node, w.member.node, w.tenantMember.node}, nodesOf(members))

	dir, err := w.svc.ListMemberDirectory(w.as(w.owner), w.owner.node, ws)
	require.NoError(t, err)
	var got []string
	for _, v := range dir {
		got = append(got, v.NodeID)
	}
	assert.ElementsMatch(t, []string{w.owner.node, w.member.node, w.tenantMember.node}, got)
}

// Handing over ownership needs the new owner to be a member already: a member is
// accepted, a person who only reaches the policy class through a group is not.
func TestRealPolicy_OwnershipGoesOnlyToMembers(t *testing.T) {
	w := newWSWorld(t)
	ws := w.g.WorkspaceID

	err := w.svc.TransferOwnership(w.as(w.owner), w.owner.node, ws, w.viaChannel.node)
	assert.ErrorIs(t, err, domain.ErrNotFound, "a person in a group only")
	err = w.svc.TransferOwnership(w.as(w.owner), w.owner.node, ws, w.viaGuests.node)
	assert.ErrorIs(t, err, domain.ErrNotFound, "a person on an event's guest list only")
	require.NoError(t, w.svc.TransferOwnership(w.as(w.owner), w.owner.node, ws, w.member.node), "a member")
}

func workspaceIDs(list []*domain.WorkspaceResult) []string {
	var out []string
	for _, r := range list {
		out = append(out, r.ID)
	}
	return out
}
