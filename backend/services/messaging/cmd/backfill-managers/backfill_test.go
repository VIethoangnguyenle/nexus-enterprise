package main

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"ngac-platform/ngac"
	policypb "ngac-platform/proto/policy"
	"ngac-platform/testutil"
)

// recordingWriter is a policy writer that records the graph writes asked of it.
// It knows the nodes it created (and any put in known), so a second run finds
// them by name; failAssociations makes that many CreateAssociation calls fail.
type recordingWriter struct {
	policypb.PolicyWriteServiceClient
	known            map[string]string // name -> id
	created          []string
	assigned         [][2]string
	associations     []grantWrite
	failAssociations int
}

// grantWrite is one CreateAssociation: UA, OA and the operations.
type grantWrite struct {
	ua, oa string
	ops    []string
}

func (w *recordingWriter) FindNodeByName(_ context.Context, r *policypb.FindNodeByNameRequest, _ ...grpc.CallOption) (*policypb.NGACNode, error) {
	if id, ok := w.known[r.Name]; ok {
		return &policypb.NGACNode{Id: id, Name: r.Name, NodeType: r.NodeType}, nil
	}
	return nil, status.Errorf(codes.NotFound, "node %s not found", r.Name)
}

func (w *recordingWriter) CreateNode(_ context.Context, r *policypb.CreateNodeRequest, _ ...grpc.CallOption) (*policypb.NGACNode, error) {
	w.created = append(w.created, r.Name)
	if w.known == nil {
		w.known = map[string]string{}
	}
	w.known[r.Name] = "new-" + r.Name
	return &policypb.NGACNode{Id: "new-" + r.Name, Name: r.Name, NodeType: r.NodeType}, nil
}

func (w *recordingWriter) CreateAssignment(_ context.Context, r *policypb.CreateAssignmentRequest, _ ...grpc.CallOption) (*policypb.Assignment, error) {
	w.assigned = append(w.assigned, [2]string{r.ChildId, r.ParentId})
	return &policypb.Assignment{}, nil
}

func (w *recordingWriter) CreateAssociation(_ context.Context, r *policypb.CreateAssociationRequest, _ ...grpc.CallOption) (*policypb.Association, error) {
	if w.failAssociations > 0 {
		w.failAssociations--
		return nil, status.Error(codes.Unavailable, "policy unavailable")
	}
	w.associations = append(w.associations, grantWrite{ua: r.UaId, oa: r.OaId, ops: r.Operations})
	return &policypb.Association{}, nil
}

// grantOn returns the operations last granted ua on oa, and whether any were.
func (w *recordingWriter) grantOn(ua, oa string) ([]string, bool) {
	for i := len(w.associations) - 1; i >= 0; i-- {
		if a := w.associations[i]; a.ua == ua && a.oa == oa {
			return a.ops, true
		}
	}
	return nil, false
}

type person struct{ id, node string }

// legacyGroup is a group from before roles existed: a row, a Members UA and a
// content OA, people assigned to Members, and no Managers UA.
type legacyGroup struct {
	pool *pgxpool.Pool
	g    group
}

func newLegacyGroup(t *testing.T, creator person) *legacyGroup {
	t.Helper()
	pool := testutil.SetupTestDB(t)
	ctx := context.Background()
	ws, _ := testutil.CreateWorkspace(t, pool, creator.id)
	g := group{ID: "ch-" + uuid.NewString(), CreatedBy: creator.id, WorkspaceID: ws,
		ContentOA: "oa-" + uuid.NewString(), MembersUA: "ua-" + uuid.NewString()}
	for _, n := range [][2]string{{g.ContentOA, "OA"}, {g.MembersUA, "UA"}} {
		_, err := pool.Exec(ctx, `INSERT INTO ngac_nodes (id, name, node_type) VALUES ($1, $1, $2)`, n[0], n[1])
		require.NoError(t, err)
	}
	_, err := pool.Exec(ctx, `INSERT INTO channels (id, name, channel_type, workspace_id, ngac_oa_id, ngac_ua_id, created_by, created_at)
		VALUES ($1, 'cũ', 'private', $2, $3, $4, $5, now())`, g.ID, ws, g.ContentOA, g.MembersUA, creator.id)
	require.NoError(t, err)
	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM channels WHERE id = $1`, g.ID)
		pool.Exec(ctx, `DELETE FROM ngac_nodes WHERE id IN ($1, $2)`, g.ContentOA, g.MembersUA)
	})
	return &legacyGroup{pool: pool, g: g}
}

// join assigns p to the group's Members UA, having joined at the given time.
func (l *legacyGroup) join(t *testing.T, p person, at time.Time) {
	t.Helper()
	ctx := context.Background()
	_, err := l.pool.Exec(ctx, `INSERT INTO ngac_assignments (id, child_id, parent_id) VALUES ($1, $2, $3)`, uuid.NewString(), p.node, l.g.MembersUA)
	require.NoError(t, err)
	_, err = l.pool.Exec(ctx, `INSERT INTO channel_members (channel_id, ngac_node_id, joined_at) VALUES ($1, $2, $3)`, l.g.ID, p.node, at)
	require.NoError(t, err)
}

func newPerson(t *testing.T) person {
	t.Helper()
	id, node := testutil.CreateUser(t, testutil.SetupTestDB(t))
	return person{id: id, node: node}
}

func TestChooseOwner_CreatorStillAMemberStaysOwner(t *testing.T) {
	creator, other := newPerson(t), newPerson(t)
	l := newLegacyGroup(t, creator)
	l.join(t, other, time.Now().Add(-2*time.Hour))
	l.join(t, creator, time.Now().Add(-time.Hour))
	b := &backfiller{db: l.pool, out: io.Discard}

	id, node, err := b.chooseOwner(context.Background(), l.g)

	require.NoError(t, err)
	assert.Equal(t, creator.id, id)
	assert.Equal(t, creator.node, node)
}

func TestChooseOwner_CreatorWhoLeftIsReplacedByTheEarliestCurrentMember(t *testing.T) {
	creator, first, second := newPerson(t), newPerson(t), newPerson(t)
	l := newLegacyGroup(t, creator)
	l.join(t, second, time.Now().Add(-time.Hour))
	l.join(t, first, time.Now().Add(-3*time.Hour))
	b := &backfiller{db: l.pool, out: io.Discard}

	id, node, err := b.chooseOwner(context.Background(), l.g)

	require.NoError(t, err)
	assert.Equal(t, first.id, id)
	assert.Equal(t, first.node, node)
}

// The creator who left before roles arrived does not get back in: the new
// owner is moved into created_by and the Managers UA, and no write puts the
// creator under Members or Managers, so the creator's read stays denied (see the
// policy vector for the same graph).
func TestBackfill_CreatorWhoLeftGetsNoEdgeBack(t *testing.T) {
	creator, member := newPerson(t), newPerson(t)
	l := newLegacyGroup(t, creator)
	l.join(t, member, time.Now().Add(-time.Hour))
	w := &recordingWriter{}
	b := &backfiller{db: l.pool, write: w, out: io.Discard}
	var s summary

	require.NoError(t, b.backfill(context.Background(), l.g, &s))

	var owner string
	require.NoError(t, l.pool.QueryRow(context.Background(), `SELECT created_by FROM channels WHERE id = $1`, l.g.ID).Scan(&owner))
	assert.Equal(t, member.id, owner)
	managers := "new-" + ngac.ChannelManagersUAName(ngac.ChannelID(l.g.ID))
	assert.Equal(t, []string{ngac.ChannelManagersUAName(ngac.ChannelID(l.g.ID))}, w.created)
	assert.Contains(t, w.assigned, [2]string{member.node, managers}, "the new owner is a manager")
	for _, a := range w.assigned {
		assert.NotEqual(t, creator.node, a[0], "nothing puts the creator back into the group")
	}
	assert.Equal(t, 1, s.OwnersMoved)
}

func TestBackfill_DryRunWritesNothing(t *testing.T) {
	creator, member := newPerson(t), newPerson(t)
	l := newLegacyGroup(t, creator)
	l.join(t, member, time.Now())
	w := &recordingWriter{}
	b := &backfiller{db: l.pool, write: w, dryRun: true, out: io.Discard}
	var s summary

	require.NoError(t, b.backfill(context.Background(), l.g, &s))

	var owner string
	require.NoError(t, l.pool.QueryRow(context.Background(), `SELECT created_by FROM channels WHERE id = $1`, l.g.ID).Scan(&owner))
	assert.Equal(t, creator.id, owner)
	assert.Empty(t, w.created)
	assert.Empty(t, w.assigned)
	assert.Empty(t, w.associations)
	assert.Equal(t, 1, s.OwnersMoved, "the dry run still reports the owner it would choose")
}

// pc is the workspace PC the legacy group's workspace was made with.
func (l *legacyGroup) pc(t *testing.T) string {
	t.Helper()
	var pc string
	require.NoError(t, l.pool.QueryRow(context.Background(), `SELECT ngac_pc_id FROM workspaces WHERE id = $1`, l.g.WorkspaceID).Scan(&pc))
	return pc
}

// A run that created the Managers UA and then failed left it without its PC
// edge or its grant. The rerun finds the UA by name and still writes the PC
// edge, the managers' invite grant, the members' grant and the owner's edge:
// each is an upsert, so writing them again costs nothing.
func TestBackfill_RerunAfterAPartialFailureReassertsEveryEdge(t *testing.T) {
	creator := newPerson(t)
	l := newLegacyGroup(t, creator)
	l.join(t, creator, time.Now())
	w := &recordingWriter{failAssociations: 1}
	b := &backfiller{db: l.pool, write: w, out: io.Discard}

	var first summary
	require.Error(t, b.backfill(context.Background(), l.g, &first), "the grant failed")
	managers := "new-" + ngac.ChannelManagersUAName(ngac.ChannelID(l.g.ID))
	_, granted := w.grantOn(managers, l.g.ContentOA)
	require.False(t, granted, "the fixture is meaningful: the first run left the UA without its grant")

	w.assigned = nil
	var second summary
	require.NoError(t, b.backfill(context.Background(), l.g, &second))

	assert.Len(t, w.created, 1, "the UA is found, not made twice")
	assert.Equal(t, 0, second.ManagersUAsMade)
	assert.Contains(t, w.assigned, [2]string{managers, l.pc(t)}, "the PC edge is written again")
	assert.Contains(t, w.assigned, [2]string{creator.node, managers}, "the owner is a manager")
	ops, ok := w.grantOn(managers, l.g.ContentOA)
	require.True(t, ok, "the managers' grant is written again")
	assert.ElementsMatch(t, ngac.ChannelManagerOps(), ops)
	ops, ok = w.grantOn(l.g.MembersUA, l.g.ContentOA)
	require.True(t, ok, "the members' grant follows the invite rule")
	assert.ElementsMatch(t, ngac.ChannelMemberOps(), ops, "everyone may add people: members keep invite")
}

// A group whose adding is restricted to managers keeps its members without
// invite: the backfill writes the members' grant from the group's rule, never
// the full member operations.
func TestBackfill_RestrictedInviteGroupGrantsMembersNoInvite(t *testing.T) {
	creator := newPerson(t)
	l := newLegacyGroup(t, creator)
	l.join(t, creator, time.Now())
	_, err := l.pool.Exec(context.Background(), `UPDATE channels SET invite_policy = $2 WHERE id = $1`, l.g.ID, ngac.GroupSettingManagers)
	require.NoError(t, err)
	l.g.InvitePolicy = ngac.GroupSettingManagers
	w := &recordingWriter{known: map[string]string{ngac.ChannelManagersUAName(ngac.ChannelID(l.g.ID)): "managers-ua"}}
	b := &backfiller{db: l.pool, write: w, out: io.Discard}

	require.NoError(t, b.backfill(context.Background(), l.g, &summary{}))

	ops, ok := w.grantOn(l.g.MembersUA, l.g.ContentOA)
	require.True(t, ok)
	assert.NotContains(t, ops, ngac.OpInvite)
	assert.ElementsMatch(t, ngac.ChannelMemberOpsRestricted(), ops)
	ops, ok = w.grantOn("managers-ua", l.g.ContentOA)
	require.True(t, ok)
	assert.Contains(t, ops, ngac.OpInvite, "managers keep invite")
}

// The full run reads each group's invite rule from its row.
func TestBackfill_RunReadsTheInviteRuleFromTheRow(t *testing.T) {
	creator := newPerson(t)
	l := newLegacyGroup(t, creator)
	l.join(t, creator, time.Now())
	_, err := l.pool.Exec(context.Background(), `UPDATE channels SET invite_policy = $2 WHERE id = $1`, l.g.ID, ngac.GroupSettingManagers)
	require.NoError(t, err)
	b := &backfiller{db: l.pool, out: io.Discard}

	groups, err := b.groups(context.Background())
	require.NoError(t, err)
	for _, g := range groups {
		if g.ID == l.g.ID {
			assert.Equal(t, ngac.GroupSettingManagers, g.InvitePolicy)
			return
		}
	}
	t.Fatal("the group is listed")
}
