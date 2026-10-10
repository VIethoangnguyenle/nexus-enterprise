package domain

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"ngac-platform/ngac"
	policypb "ngac-platform/proto/policy"
	"ngac-platform/services/messaging/internal/store"
	"ngac-platform/testutil"
)

type allowAll struct {
	policypb.PolicyReadServiceClient
	assigned bool // the target is already in the channel's members UA
}

func (a allowAll) IsAssigned(context.Context, *policypb.IsAssignedRequest, ...grpc.CallOption) (*policypb.BoolResponse, error) {
	return &policypb.BoolResponse{Value: a.assigned}, nil
}

func (allowAll) CheckAccess(context.Context, *policypb.CheckAccessRequest, ...grpc.CallOption) (*policypb.AccessDecision, error) {
	return &policypb.AccessDecision{Decision: ngac.DecisionAllow}, nil
}

type graphLog struct {
	policypb.PolicyWriteServiceClient
	assigned     []string
	withdrawn    []string
	failWithdraw bool
}

func (g *graphLog) CreateAssignment(_ context.Context, r *policypb.CreateAssignmentRequest, _ ...grpc.CallOption) (*policypb.Assignment, error) {
	g.assigned = append(g.assigned, r.ChildId+">"+r.ParentId)
	return &policypb.Assignment{}, nil
}
func (g *graphLog) RemoveAssignment(_ context.Context, r *policypb.RemoveAssignmentRequest, _ ...grpc.CallOption) (*policypb.Empty, error) {
	g.withdrawn = append(g.withdrawn, r.ChildId+">"+r.ParentId)
	if g.failWithdraw {
		return nil, errors.New("policy down")
	}
	return &policypb.Empty{}, nil
}

type failingCache struct{ calls int }

func (f *failingCache) InsertChannelMember(context.Context, string, string) error {
	f.calls++
	return errors.New(`ERROR: deadlock detected (SQLSTATE 40P01)`)
}

// seedChannel stores a channel whose OA and UA exist, so AddMember has something to act on.
func seedChannel(t *testing.T) (*store.Store, *store.Channel) {
	t.Helper()
	pool := testutil.SetupTestDB(t)
	st := store.NewStore(pool)
	oa, ua := "oa-"+uuid.NewString(), "ua-"+uuid.NewString()
	for _, n := range [][3]string{{oa, "OA", oa}, {ua, "UA", ua}} {
		_, err := pool.Exec(context.Background(), `INSERT INTO ngac_nodes (id, name, node_type) VALUES ($1, $3, $2)`, n[0], n[1], n[2])
		require.NoError(t, err)
	}
	user, _ := testutil.CreateUser(t, pool)
	ch := &store.Channel{ID: "ch-" + uuid.NewString(), Name: "general", ChannelType: "workspace", NGACOaID: oa, NGACUaID: ua, CreatedBy: user}
	require.NoError(t, st.InsertChannelWithMembers(context.Background(), ch, "creator-node"))
	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM channels WHERE id = $1`, ch.ID)
		pool.Exec(context.Background(), `DELETE FROM ngac_nodes WHERE id IN ($1, $2)`, oa, ua)
	})
	return st, ch
}

// The graph holds the membership and the cache copies it. When the copy cannot
// be written, the assignment is withdrawn and the caller is told: the two never
// disagree about who is in the channel.
func TestAddMember_WithdrawsTheAssignmentWhenTheCacheCannotBeWritten(t *testing.T) {
	st, ch := seedChannel(t)
	g := &graphLog{}
	svc := NewService(st, allowAll{}, g, nil, nil)
	cache := &failingCache{}
	svc.members = cache

	err := svc.AddMember(context.Background(), ch.ID, "requester", "new-member")

	require.Error(t, err)
	assert.Equal(t, 1, cache.calls)
	assert.Equal(t, []string{"new-member>" + ch.NGACUaID}, g.assigned)
	assert.Equal(t, []string{"new-member>" + ch.NGACUaID}, g.withdrawn, "the assignment made for the cache write is withdrawn")
}

func TestAddMember_SaysSoWhenTheWithdrawalAlsoFails(t *testing.T) {
	st, ch := seedChannel(t)
	g := &graphLog{failWithdraw: true}
	svc := NewService(st, allowAll{}, g, nil, nil)
	svc.members = &failingCache{}

	err := svc.AddMember(context.Background(), ch.ID, "requester", "new-member")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "disagree", "a half state must be reported, not hidden")
}

func TestAddMember_RecordsTheMemberOnSuccess(t *testing.T) {
	st, ch := seedChannel(t)
	g := &graphLog{}
	svc := NewService(st, allowAll{}, g, nil, nil)

	require.NoError(t, svc.AddMember(context.Background(), ch.ID, "requester", "new-member"))
	assert.Empty(t, g.withdrawn)
}

// CreateAssignment is idempotent: re-adding someone who is already a member
// while the cache cannot be written must not delete the edge that was there.
func TestAddMember_NeverWithdrawsAnEdgeThatWasAlreadyThere(t *testing.T) {
	st, ch := seedChannel(t)
	g := &graphLog{}
	svc := NewService(st, allowAll{assigned: true}, g, nil, nil)
	svc.members = &failingCache{}

	err := svc.AddMember(context.Background(), ch.ID, "requester", "existing-member")

	require.Error(t, err)
	assert.Empty(t, g.withdrawn, "the pre-existing membership edge is left alone")
}
