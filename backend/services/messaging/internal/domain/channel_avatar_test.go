package domain_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/ngac"
	"ngac-platform/pkg/realtime"
	"ngac-platform/services/messaging/internal/domain"
)

func pngOf(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: 90, A: 255})
		}
	}
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))
	return buf.Bytes()
}

// avatarState reads what the database holds of the channel's picture.
func (f *fixture) avatarState(t *testing.T, chID string) (url string, rows int) {
	t.Helper()
	ctx := context.Background()
	require.NoError(t, f.pool.QueryRow(ctx, `SELECT avatar_url FROM channels WHERE id = $1`, chID).Scan(&url))
	require.NoError(t, f.pool.QueryRow(ctx, `SELECT count(*) FROM channel_avatars WHERE channel_id = $1`, chID).Scan(&rows))
	return url, rows
}

func (f *fixture) allowManage(oaID string) { f.read.allow[grant{userNode, oaID, ngac.OpManage}] = true }

func TestSetChannelAvatar_ManagerChangesThePictureAndTheChannelSaysSo(t *testing.T) {
	f := newFixture(t)
	chID, oaID := f.insertChannel(t, "av-group")
	f.allowManage(oaID)
	f.read.allow[grant{userNode, oaID, ngac.OpRead}] = true
	rec := &realtime.Recorder{}
	f.svc.SetEmitter(rec)
	ctx := f.callerCtx()

	url, err := f.svc.SetChannelAvatar(ctx, chID, f.userID, userNode, pngOf(t, 300, 200))

	require.NoError(t, err)
	token := strings.TrimPrefix(url, domain.AvatarURLPrefix)
	require.Len(t, token, 32)
	stored, rows := f.avatarState(t, chID)
	assert.Equal(t, url, stored)
	assert.Equal(t, 1, rows)
	for _, size := range []int{64, 192, 1024} {
		b, err := f.svc.ChannelAvatar(ctx, token, size)
		require.NoError(t, err, "size %d", size)
		assert.NotEmpty(t, b)
	}
	got, err := f.svc.GetChannel(ctx, chID, userNode)
	require.NoError(t, err)
	assert.Equal(t, url, got.AvatarUrl, "the channel carries its picture address")
	list, err := f.svc.ListChannels(ctx, f.wsID, userNode)
	require.NoError(t, err)
	var listed string
	for _, c := range list {
		if c.Id == chID {
			listed = c.AvatarUrl
		}
	}
	assert.Equal(t, url, listed, "the list carries it too")

	evts := rec.Events()
	require.Len(t, evts, 1)
	assert.Equal(t, realtime.DomainChannel, evts[0].Domain)
	assert.Equal(t, realtime.KindRenamed, evts[0].Kind)
	assert.Equal(t, chID, evts[0].ChannelID)
	assert.Equal(t, []string{chID}, evts[0].IDs)
	assert.Equal(t, realtime.LevelChannel, evts[0].Level())

	again, err := f.svc.SetChannelAvatar(ctx, chID, f.userID, userNode, pngOf(t, 300, 200))
	require.NoError(t, err)
	assert.NotEqual(t, url, again, "the same picture twice gets a new address")
	_, err = f.svc.ChannelAvatar(ctx, token, 192)
	assert.ErrorIs(t, err, domain.ErrNotFound, "the replaced picture is gone")
}

func TestSetChannelAvatar_DeniedWithoutManage(t *testing.T) {
	cases := map[string]func(f *fixture, oaID string){
		"no grant at all": func(*fixture, string) {},
		"read and write only": func(f *fixture, oaID string) {
			f.read.allow[grant{userNode, oaID, ngac.OpRead}] = true
			f.read.allow[grant{userNode, oaID, ngac.OpWrite}] = true
		},
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			chID, oaID := f.insertChannel(t, "av-denied")
			setup(f, oaID)
			rec := &realtime.Recorder{}
			f.svc.SetEmitter(rec)

			_, err := f.svc.SetChannelAvatar(f.callerCtx(), chID, f.userID, userNode, pngOf(t, 80, 80))
			require.ErrorIs(t, err, domain.ErrAccessDenied)
			err = f.svc.ClearChannelAvatar(f.callerCtx(), chID, userNode)
			require.ErrorIs(t, err, domain.ErrAccessDenied)

			url, rows := f.avatarState(t, chID)
			assert.Empty(t, url)
			assert.Zero(t, rows, "a refused upload writes nothing")
			assert.Empty(t, rec.Events(), "a refused change is not announced")
		})
	}
}

func TestClearChannelAvatar_DeniedKeepsThePicture(t *testing.T) {
	f := newFixture(t)
	chID, oaID := f.insertChannel(t, "av-keep")
	f.allowManage(oaID)
	url, err := f.svc.SetChannelAvatar(f.callerCtx(), chID, f.userID, userNode, pngOf(t, 80, 80))
	require.NoError(t, err)
	delete(f.read.allow, grant{userNode, oaID, ngac.OpManage})

	err = f.svc.ClearChannelAvatar(f.callerCtx(), chID, userNode)

	require.ErrorIs(t, err, domain.ErrAccessDenied)
	stored, rows := f.avatarState(t, chID)
	assert.Equal(t, url, stored)
	assert.Equal(t, 1, rows)
}

func TestSetChannelAvatar_FailsClosedWhenPolicyIsUnreachable(t *testing.T) {
	f := newFixture(t)
	chID, oaID := f.insertChannel(t, "av-closed")
	f.allowManage(oaID)
	f.read.err = errors.New("policy down")

	_, err := f.svc.SetChannelAvatar(f.callerCtx(), chID, f.userID, userNode, pngOf(t, 80, 80))

	require.Error(t, err)
	_, rows := f.avatarState(t, chID)
	assert.Zero(t, rows)
}

func TestClearChannelAvatar_RemovesThePictureAndItsAddress(t *testing.T) {
	f := newFixture(t)
	chID, oaID := f.insertChannel(t, "av-clear")
	f.allowManage(oaID)
	ctx := f.callerCtx()
	url, err := f.svc.SetChannelAvatar(ctx, chID, f.userID, userNode, pngOf(t, 80, 80))
	require.NoError(t, err)
	rec := &realtime.Recorder{}
	f.svc.SetEmitter(rec)

	require.NoError(t, f.svc.ClearChannelAvatar(ctx, chID, userNode))
	require.NoError(t, f.svc.ClearChannelAvatar(ctx, chID, userNode), "having none is not an error")

	stored, rows := f.avatarState(t, chID)
	assert.Empty(t, stored)
	assert.Zero(t, rows)
	_, err = f.svc.ChannelAvatar(ctx, strings.TrimPrefix(url, domain.AvatarURLPrefix), 192)
	assert.ErrorIs(t, err, domain.ErrNotFound, "the old address answers not found")
	require.NotEmpty(t, rec.Events())
	assert.Equal(t, chID, rec.Events()[0].ChannelID)
}

func TestSetChannelAvatar_DirectMessageHasNoGroupPicture(t *testing.T) {
	f := newFixture(t)
	chID, oaID := f.insertChannel(t, "av-dm")
	_, err := f.pool.Exec(context.Background(), `UPDATE channels SET channel_type = 'dm' WHERE id = $1`, chID)
	require.NoError(t, err)
	f.allowManage(oaID)

	_, err = f.svc.SetChannelAvatar(f.callerCtx(), chID, f.userID, userNode, pngOf(t, 80, 80))
	require.ErrorIs(t, err, domain.ErrInvalidInput)
	require.ErrorIs(t, f.svc.ClearChannelAvatar(f.callerCtx(), chID, userNode), domain.ErrInvalidInput)

	_, rows := f.avatarState(t, chID)
	assert.Zero(t, rows)
}

func TestSetChannelAvatar_UnknownChannelIsNotFound(t *testing.T) {
	f := newFixture(t)
	_, err := f.svc.SetChannelAvatar(f.callerCtx(), "no-such-channel", f.userID, userNode, pngOf(t, 80, 80))
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestSetChannelAvatar_RefusesWhatIsNotAPicture(t *testing.T) {
	f := newFixture(t)
	chID, oaID := f.insertChannel(t, "av-bad")
	f.allowManage(oaID)
	ctx := f.callerCtx()

	_, err := f.svc.SetChannelAvatar(ctx, chID, f.userID, userNode, []byte("<svg onload=alert(1)>"))
	assert.ErrorIs(t, err, domain.ErrAvatarNotImage)
	_, err = f.svc.SetChannelAvatar(ctx, chID, f.userID, userNode, domain.PNGHeaderForTest(10000, 10000, 8, 6))
	assert.ErrorIs(t, err, domain.ErrAvatarTooManyPixels)

	stored, rows := f.avatarState(t, chID)
	assert.Empty(t, stored)
	assert.Zero(t, rows)
}

func TestChannelAvatar_OnlyAStoredTokenAndSizeAreServed(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	for _, tc := range []struct {
		token string
		size  int
	}{
		{"", 192}, {"../etc/passwd", 192}, {strings.Repeat("a", 32), 192}, {strings.Repeat("A", 32), 192}, {strings.Repeat("a", 32), 100},
	} {
		_, err := f.svc.ChannelAvatar(ctx, tc.token, tc.size)
		assert.ErrorIs(t, err, domain.ErrNotFound, "%q/%d", tc.token, tc.size)
	}
}

func TestSetChannelAvatar_BusyWhenAnotherPictureIsBeingProcessed(t *testing.T) {
	f := newFixture(t)
	chID, oaID := f.insertChannel(t, "av-busy")
	f.allowManage(oaID)
	release := f.svc.HoldAvatarGate(20 * time.Millisecond)
	defer release()

	_, err := f.svc.SetChannelAvatar(f.callerCtx(), chID, f.userID, userNode, pngOf(t, 80, 80))

	require.ErrorIs(t, err, domain.ErrAvatarBusy)
	_, rows := f.avatarState(t, chID)
	assert.Zero(t, rows)
}

func TestSetChannelAvatar_UploadsPerHourAreLimitedPerPerson(t *testing.T) {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		t.Skip("REDIS_ADDR not set")
	}
	rdb := redis.NewClient(&redis.Options{Addr: addr})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		t.Skipf("redis not available: %v", err)
	}
	t.Cleanup(func() { rdb.Close() })
	f := newFixture(t)
	chID, oaID := f.insertChannel(t, "av-rate")
	f.allowManage(oaID)
	f.svc.SetRedis(rdb)
	key := "channel_avatar_rl:" + f.userID
	rdb.Del(context.Background(), key)
	t.Cleanup(func() { rdb.Del(context.Background(), key) })
	img := pngOf(t, 40, 40)

	for i := 0; i < domain.AvatarUploadLimit; i++ {
		_, err := f.svc.SetChannelAvatar(f.callerCtx(), chID, f.userID, userNode, img)
		require.NoError(t, err, fmt.Sprintf("upload %d", i+1))
	}
	_, err := f.svc.SetChannelAvatar(f.callerCtx(), chID, f.userID, userNode, img)

	require.ErrorIs(t, err, domain.ErrRateLimited)
	wait, ok := domain.RetryAfter(err)
	assert.True(t, ok)
	assert.Positive(t, wait)
}
