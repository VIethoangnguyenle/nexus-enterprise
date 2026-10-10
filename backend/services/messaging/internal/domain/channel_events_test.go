package domain_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/ngac"
	"ngac-platform/pkg/grpcauth"
	"ngac-platform/pkg/realtime"
	"ngac-platform/services/messaging/internal/domain"
	"ngac-platform/testutil"
)

func (f *fixture) callerCtx() context.Context {
	return grpcauth.WithCaller(context.Background(), grpcauth.Caller{UserID: f.userID, NGACNodeID: userNode, TenantID: f.wsID})
}

// A channel row visible in the database at the moment the event is emitted
// proves the announcement follows the commit.
type visibleOnEmit struct {
	realtime.Recorder
	check func() bool
	seen  []bool
}

func (v *visibleOnEmit) Emit(e realtime.Event) {
	v.seen = append(v.seen, v.check())
	v.Recorder.Emit(e)
}

func TestCreateChannel_AnnouncesOnlyAfterTheRowIsCommitted(t *testing.T) {
	f := newFixture(t)
	channelsOA := f.withChannelsOA()
	f.read.allow[grant{userNode, channelsOA, ngac.OpCreateChannel}] = true
	name := fmt.Sprintf("rt-create-%d", time.Now().UnixNano())
	rec := &visibleOnEmit{check: func() bool { return f.channelCount(t, name) == 1 }}
	f.svc.SetEmitter(rec)

	ch, err := f.svc.CreateChannel(f.callerCtx(), f.createInput(name))

	require.NoError(t, err)
	evts := rec.Events()
	require.Len(t, evts, 1)
	assert.Equal(t, realtime.DomainChannel, evts[0].Domain)
	assert.Equal(t, realtime.KindCreated, evts[0].Kind)
	assert.Equal(t, []string{ch.Id}, evts[0].IDs)
	assert.Equal(t, f.wsID, evts[0].WorkspaceID)
	assert.Equal(t, f.wsID, evts[0].TenantID)
	assert.Equal(t, f.userID, evts[0].ActorUserID)
	assert.Equal(t, realtime.LevelUser, evts[0].Level(), "a new channel is announced to the people who can read it, not the workspace")
	assert.Equal(t, []string{userNode}, evts[0].UserNodeIDs)
	assert.Equal(t, []bool{true}, rec.seen, "the row must be readable when the event goes out")
}

func TestCreateChannel_AnnouncesNothingWhenDeniedOrRolledBack(t *testing.T) {
	t.Run("denied", func(t *testing.T) {
		f := newFixture(t)
		f.withChannelsOA()
		rec := &realtime.Recorder{}
		f.svc.SetEmitter(rec)
		_, err := f.svc.CreateChannel(f.callerCtx(), f.createInput(fmt.Sprintf("rt-deny-%d", time.Now().UnixNano())))
		require.ErrorIs(t, err, domain.ErrAccessDenied)
		assert.Empty(t, rec.Events())
	})
	t.Run("graph write fails and the nodes are removed", func(t *testing.T) {
		f := newFixture(t)
		channelsOA := f.withChannelsOA()
		f.read.allow[grant{userNode, channelsOA, ngac.OpCreateChannel}] = true
		rec := &realtime.Recorder{}
		f.svc.SetEmitter(rec)
		w := testutil.NewFakePolicyWrite()
		w.FailAt = 3 // the first assignment, after both nodes exist
		f.withFakeWrite(w)
		f.svc.SetEmitter(rec)
		name := fmt.Sprintf("rt-rollback-%d", time.Now().UnixNano())

		_, err := f.svc.CreateChannel(f.callerCtx(), f.createInput(name))

		require.Error(t, err)
		assert.Equal(t, 0, f.channelCount(t, name))
		assert.Empty(t, rec.Events(), "a creation that was undone must not be announced")
	})
}

func TestUpdateChannel_AnnouncesRenameToTheChannelOnly(t *testing.T) {
	f := newFixture(t)
	chID, oaID := f.insertChannel(t, "rt-before")
	f.read.allow[grant{userNode, oaID, ngac.OpManage}] = true
	rec := &visibleOnEmit{}
	rec.check = func() bool {
		var name string
		err := f.pool.QueryRow(context.Background(), `SELECT name FROM channels WHERE id = $1`, chID).Scan(&name)
		return err == nil && name == "rt-after"
	}
	f.svc.SetEmitter(rec)

	_, err := f.svc.UpdateChannel(f.callerCtx(), chID, userNode, "rt-after")

	require.NoError(t, err)
	evts := rec.Events()
	require.Len(t, evts, 1)
	assert.Equal(t, realtime.KindRenamed, evts[0].Kind)
	assert.Equal(t, chID, evts[0].ChannelID)
	assert.Equal(t, realtime.LevelChannel, evts[0].Level())
	assert.Equal(t, []bool{true}, rec.seen)
}

func TestUpdateChannel_AnnouncesNothingWhenRefused(t *testing.T) {
	f := newFixture(t)
	chID, _ := f.insertChannel(t, "rt-keep")
	rec := &realtime.Recorder{}
	f.svc.SetEmitter(rec)

	_, err := f.svc.UpdateChannel(f.callerCtx(), chID, userNode, "rt-nope")
	require.ErrorIs(t, err, domain.ErrAccessDenied)
	_, err = f.svc.UpdateChannel(f.callerCtx(), chID, userNode, "")
	require.Error(t, err)

	assert.Empty(t, rec.Events())
}

func TestMembership_AnnouncesToTheRosterAndThePerson(t *testing.T) {
	f := newFixture(t)
	chID, oaID := f.insertChannel(t, "rt-members")
	f.read.allow[grant{userNode, oaID, ngac.OpInvite}] = true
	rec := &realtime.Recorder{}
	f.withFakeWrite(testutil.NewFakePolicyWrite())
	f.svc.SetEmitter(rec)

	require.NoError(t, f.svc.AddMember(f.callerCtx(), chID, userNode, "ngac-user-newcomer"))
	require.NoError(t, f.svc.RemoveMember(f.callerCtx(), chID, userNode, "ngac-user-newcomer"))

	evts := rec.Events()
	require.Len(t, evts, 4)
	for i, kind := range []string{realtime.KindMemberAdded, realtime.KindMemberAdded, realtime.KindMemberRemoved, realtime.KindMemberRemoved} {
		assert.Equal(t, kind, evts[i].Kind)
		assert.Equal(t, chID, evts[i].ChannelID)
	}
	assert.Equal(t, realtime.LevelChannel, evts[0].Level(), "the roster hears it through the channel")
	assert.Equal(t, realtime.LevelUser, evts[1].Level(), "the person hears it directly")
	assert.Equal(t, []string{"ngac-user-newcomer"}, evts[1].UserNodeIDs)
	assert.Equal(t, []string{"ngac-user-newcomer"}, evts[3].UserNodeIDs)
}

func TestMembership_AnnouncesNothingWhenRefusedOrFailed(t *testing.T) {
	f := newFixture(t)
	chID, oaID := f.insertChannel(t, "rt-members-denied")
	rec := &realtime.Recorder{}
	f.svc.SetEmitter(rec)

	require.ErrorIs(t, f.svc.AddMember(f.callerCtx(), chID, userNode, "ngac-user-x"), domain.ErrAccessDenied)
	require.ErrorIs(t, f.svc.RemoveMember(f.callerCtx(), chID, userNode, "ngac-user-x"), domain.ErrAccessDenied)

	f.read.allow[grant{userNode, oaID, ngac.OpInvite}] = true
	w := testutil.NewFakePolicyWrite()
	w.FailAt = 1
	f.withFakeWrite(w)
	f.svc.SetEmitter(rec)
	require.Error(t, f.svc.AddMember(f.callerCtx(), chID, userNode, "ngac-user-x"))

	assert.Empty(t, rec.Events())
}
