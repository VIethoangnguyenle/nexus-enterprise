package domain_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"ngac-platform/ngac"
	authpb "ngac-platform/proto/auth"
	"ngac-platform/services/messaging/internal/domain"
	"ngac-platform/services/messaging/internal/store"
	"ngac-platform/testutil"
)

// A channel is a Content OA, a Members UA, four edges and a row. These tests
// break the run at every write and require that nothing it created is left in
// the graph — wherever it stops, and newest node first.

func (f *fixture) withFakeWrite(w *testutil.FakePolicyWrite) {
	f.svc = domain.NewService(store.NewStore(f.pool), f.read, w, stubAuth{}, nil)
}

func liveChannelNodes(w *testutil.FakePolicyWrite) []string {
	var out []string
	for _, n := range w.LiveNodes() {
		if strings.Contains(n, "Ch_") {
			out = append(out, n)
		}
	}
	return out
}

func TestCreateChannel_RollsBackWhereverTheGraphWritesFail(t *testing.T) {
	// Content OA, Members UA, assign content, assign members, associate, assign creator.
	const writes = 6
	for k := 1; k <= writes; k++ {
		f := newFixture(t)
		channelsOA := f.withChannelsOA()
		f.read.allow[grant{userNode, channelsOA, ngac.OpCreateChannel}] = true
		w := testutil.NewFakePolicyWrite()
		w.FailAt = k
		f.withFakeWrite(w)
		name := fmt.Sprintf("prov-fail-%d-%d", k, time.Now().UnixNano())

		_, err := f.svc.CreateChannel(context.Background(), f.createInput(name))

		require.Errorf(t, err, "failure injected at write %d must surface", k)
		assert.Emptyf(t, w.LiveNodes(), "failure at write %d left nodes behind: %v", k, w.LiveNodes())
		assert.Equal(t, 0, f.channelCount(t, name), "and no channel row")
		assert.Equal(t, reversed(created(w.Log())), deleted(w.Log()), "newest first")
	}
}

func created(log []string) []string {
	var out []string
	for _, l := range log {
		if rest, ok := strings.CutPrefix(l, "create "); ok {
			out = append(out, rest[strings.Index(rest, " ")+1:])
		}
	}
	return out
}

func deleted(log []string) []string {
	var out []string
	for _, l := range log {
		if name, ok := strings.CutPrefix(l, "delete "); ok {
			out = append(out, name)
		}
	}
	return out
}

func reversed(in []string) []string {
	out := append([]string(nil), in...)
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}

// The graph writes all succeed and the row insert is refused (a creator the users
// table does not know). The nodes written before it must go.
func TestCreateChannel_RollsBackWhenTheRowIsRefused(t *testing.T) {
	f := newFixture(t)
	channelsOA := f.withChannelsOA()
	f.read.allow[grant{userNode, channelsOA, ngac.OpCreateChannel}] = true
	w := testutil.NewFakePolicyWrite()
	f.withFakeWrite(w)
	in := f.createInput(fmt.Sprintf("prov-row-%d", time.Now().UnixNano()))
	in.UserID = "no-such-user"

	_, err := f.svc.CreateChannel(context.Background(), in)

	require.Error(t, err)
	// Content OA, Members UA, two assignments, the members association, the
	// creator's edge; then the Managers UA, its PC edge, its association and the
	// owner's Managers edge.
	assert.Equal(t, 10, w.Calls(), "every graph write happened before the row was refused")
	assert.Empty(t, w.LiveNodes())
	assert.Equal(t, reversed(created(w.Log())), deleted(w.Log()))
}

func TestCreateChannel_NodesAreKeyedByTheChannelID(t *testing.T) {
	f := newFixture(t)
	channelsOA := f.withChannelsOA()
	f.read.allow[grant{userNode, channelsOA, ngac.OpCreateChannel}] = true
	name := fmt.Sprintf("prov-ok-%d", time.Now().UnixNano())

	ch, err := f.svc.CreateChannel(context.Background(), f.createInput(name))

	require.NoError(t, err)
	var oaName, uaName string
	require.NoError(t, f.pool.QueryRow(context.Background(), `SELECT name FROM ngac_nodes WHERE id = $1`, ch.NgacOaId).Scan(&oaName))
	require.NoError(t, f.pool.QueryRow(context.Background(), `SELECT name FROM ngac_nodes WHERE id = $1`, ch.NgacUaId).Scan(&uaName))
	assert.Equal(t, ngac.ChannelContentOAName(ngac.ChannelID(ch.Id)), oaName)
	assert.Equal(t, ngac.ChannelMembersUAName(ngac.ChannelID(ch.Id)), uaName)
	assert.NotContains(t, oaName+uaName, name, "the channel's display name is not part of its node names")
}

// dmAuthStub resolves users with their NGAC node id, so FindOrCreateDM gets
// past the "person" gate and reaches the provisioning it is meant to test.
type dmAuthStub struct {
	authpb.AuthServiceClient
	nodes map[string]string // user id → NGAC node id
}

func (a dmAuthStub) GetUserByID(_ context.Context, req *authpb.GetUserByIDRequest, _ ...grpc.CallOption) (*authpb.UserInfo, error) {
	node, ok := a.nodes[req.UserId]
	if !ok {
		return nil, status.Error(codes.NotFound, "no such user")
	}
	return &authpb.UserInfo{Id: req.UserId, Username: "testuser-" + req.UserId, NgacNodeId: node}, nil
}

// withFakeWriteAndAuth builds the service with a FakePolicyWrite whose reads
// are delegated to f.read, and a dmAuthStub that resolves both the caller and
// the target user.
func (f *fixture) withFakeWriteAndAuth(w *testutil.FakePolicyWrite, otherUserID, otherNode string) {
	w.Reads = f.read
	auth := dmAuthStub{nodes: map[string]string{
		f.userID:    userNode,
		otherUserID: otherNode,
	}}
	f.svc = domain.NewService(store.NewStore(f.pool), f.read, w, auth, nil)
}

func TestFindOrCreateDM_RollsBackWhenTheOtherParticipantCannotBeAdded(t *testing.T) {
	// Content OA, Members UA, assign content, assign members (both under PC_Global),
	// associate, assign creator, assign the other participant.
	const writes = 7
	const otherUserID = "other-user"
	const otherNode = "ngac-other-user"
	for k := 1; k <= writes; k++ {
		k := k
		t.Run(fmt.Sprintf("fail_at_%d", k), func(t *testing.T) {
			f := newFixture(t)
			w := testutil.NewFakePolicyWrite()
			w.FailAt = k
			f.withFakeWriteAndAuth(w, otherUserID, otherNode)
			// Both users must be members of the caller's workspace so
			// SharedWorkspaces includes it.
			f.read.addMember(f.wsID, userNode, otherNode)
			ctx := f.inWorkspace(f.wsID)

			_, err := f.svc.FindOrCreateDM(ctx, f.userID, userNode, otherUserID, otherNode)

			require.Errorf(t, err, "failure injected at write %d must surface", k)
			assert.Emptyf(t, w.LiveNodes(), "failure at write %d left nodes behind: %v", k, w.LiveNodes())
		})
	}
}

func TestFindOrCreateDM_FailsWithoutGlobalPCAndWritesNothingLasting(t *testing.T) {
	const otherUserID = "other-user"
	const otherNode = "ngac-other-user"
	f := newFixture(t)
	w := testutil.NewFakePolicyWrite()
	f.withFakeWriteAndAuth(w, otherUserID, otherNode)
	f.read.addMember(f.wsID, userNode, otherNode)
	f.read.noGlobal = true
	ctx := f.inWorkspace(f.wsID)

	_, err := f.svc.FindOrCreateDM(ctx, f.userID, userNode, otherUserID, otherNode)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "PC_Global", "the error must be about PC_Global, not an early refusal")
	assert.Empty(t, liveChannelNodes(w), "the DM nodes created before the lookup failed are removed")
}
