package domain_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/ngac"
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

// The graph writes all succeed and the row insert is refused (a channel type
// the table does not allow). The nodes written before it must go.
func TestCreateChannel_RollsBackWhenTheRowIsRefused(t *testing.T) {
	f := newFixture(t)
	channelsOA := f.withChannelsOA()
	f.read.allow[grant{userNode, channelsOA, ngac.OpCreateChannel}] = true
	w := testutil.NewFakePolicyWrite()
	f.withFakeWrite(w)
	in := f.createInput(fmt.Sprintf("prov-row-%d", time.Now().UnixNano()))
	in.ChannelType = "not-a-channel-type"

	_, err := f.svc.CreateChannel(context.Background(), in)

	require.Error(t, err)
	assert.Equal(t, 6, w.Calls(), "every graph write happened before the row was refused")
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

func TestFindOrCreateDM_RollsBackWhenTheOtherParticipantCannotBeAdded(t *testing.T) {
	// Content OA, Members UA, assign content, assign members (both under PC_Global),
	// associate, assign creator, assign the other participant.
	const writes = 7
	for k := 1; k <= writes; k++ {
		f := newFixture(t)
		w := testutil.NewFakePolicyWrite()
		w.FailAt = k
		f.withFakeWrite(w)

		_, err := f.svc.FindOrCreateDM(context.Background(), f.userID, userNode, "other-user", "ngac-other-user")

		require.Errorf(t, err, "failure injected at write %d must surface", k)
		assert.Emptyf(t, w.LiveNodes(), "failure at write %d left nodes behind: %v", k, w.LiveNodes())
	}
}

func TestFindOrCreateDM_FailsWithoutGlobalPCAndWritesNothingLasting(t *testing.T) {
	f := newFixture(t)
	w := testutil.NewFakePolicyWrite()
	f.withFakeWrite(w)
	f.read.noGlobal = true

	_, err := f.svc.FindOrCreateDM(context.Background(), f.userID, userNode, "other-user", "ngac-other-user")

	require.Error(t, err)
	assert.Empty(t, liveChannelNodes(w), "the DM nodes created before the lookup failed are removed")
}
