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

// recordingWriter is a policy writer that knows no Managers UA and records the
// graph writes asked of it.
type recordingWriter struct {
	policypb.PolicyWriteServiceClient
	created  []string
	assigned [][2]string
}

func (w *recordingWriter) FindNodeByName(_ context.Context, r *policypb.FindNodeByNameRequest, _ ...grpc.CallOption) (*policypb.NGACNode, error) {
	return nil, status.Errorf(codes.NotFound, "node %s not found", r.Name)
}

func (w *recordingWriter) CreateNode(_ context.Context, r *policypb.CreateNodeRequest, _ ...grpc.CallOption) (*policypb.NGACNode, error) {
	w.created = append(w.created, r.Name)
	return &policypb.NGACNode{Id: "new-" + r.Name, Name: r.Name, NodeType: r.NodeType}, nil
}

func (w *recordingWriter) CreateAssignment(_ context.Context, r *policypb.CreateAssignmentRequest, _ ...grpc.CallOption) (*policypb.Assignment, error) {
	w.assigned = append(w.assigned, [2]string{r.ChildId, r.ParentId})
	return &policypb.Assignment{}, nil
}

func (w *recordingWriter) CreateAssociation(context.Context, *policypb.CreateAssociationRequest, ...grpc.CallOption) (*policypb.Association, error) {
	return &policypb.Association{}, nil
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
	assert.Equal(t, 1, s.OwnersMoved, "the dry run still reports the owner it would choose")
}
