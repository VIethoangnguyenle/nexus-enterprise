package grpc_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"ngac-platform/pkg/realtime"
	pb "ngac-platform/proto/messaging"
	grpcserver "ngac-platform/services/messaging/internal/grpc"
)

// frame is one thing a session received: a protobuf envelope (raw keeps the
// exact bytes) or a JSON control message.
type frame struct {
	raw []byte
	env *pb.ServerEnvelope
	ctl map[string]any
}

type syncClient struct {
	conn *websocket.Conn
	in   chan frame
}

func dialSync(t *testing.T, url string, id identity) *syncClient {
	t.Helper()
	claims := jwt.MapClaims{
		"user_id": id.userID, "username": id.username, "ngac_node_id": id.nodeID,
		"tenant_id": id.tenantID, "exp": time.Now().Add(time.Hour).Unix(),
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(hubTestSecret))
	require.NoError(t, err)
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })
	c := &syncClient{conn: conn, in: make(chan frame, 4096)}
	go func() {
		defer close(c.in)
		for {
			kind, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			if kind == websocket.TextMessage {
				var m map[string]any
				if json.Unmarshal(data, &m) == nil {
					c.in <- frame{ctl: m}
				}
				continue
			}
			var env pb.ServerEnvelope
			if proto.Unmarshal(data, &env) == nil {
				c.in <- frame{raw: data, env: &env}
			}
		}
	}()
	auth, err := proto.Marshal(&pb.ClientEnvelope{Payload: &pb.ClientEnvelope_Auth{Auth: &pb.AuthRequest{Token: token}}})
	require.NoError(t, err)
	require.NoError(t, conn.WriteMessage(websocket.BinaryMessage, auth))
	got := c.wait(t, func(f frame) bool { return f.env.GetAuthResponse() != nil }, 2*time.Second)
	require.NotNil(t, got)
	require.True(t, got.env.GetAuthResponse().Ok)
	return c
}

func (c *syncClient) wait(t *testing.T, match func(frame) bool, within time.Duration) *frame {
	t.Helper()
	deadline := time.After(within)
	for {
		select {
		case f, ok := <-c.in:
			if !ok {
				return nil
			}
			if match(f) {
				return &f
			}
		case <-deadline:
			return nil
		}
	}
}

func (c *syncClient) sendJSON(t *testing.T, v any) {
	t.Helper()
	data, err := json.Marshal(v)
	require.NoError(t, err)
	require.NoError(t, c.conn.WriteMessage(websocket.TextMessage, data))
}

func (c *syncClient) syncWorkspace(t *testing.T, ws string, since uint64) {
	c.sendJSON(t, map[string]any{"type": "sync", "workspace_id": ws, "since": since})
}

func (c *syncClient) syncUser(t *testing.T, since uint64) {
	c.sendJSON(t, map[string]any{"type": "user_sync", "since": since})
}

func isCtl(typ string) func(frame) bool {
	return func(f frame) bool { return f.ctl != nil && f.ctl["type"] == typ }
}

func isAnyOutcome(f frame) bool { return isCtl("synced")(f) || isCtl("too_far")(f) }

// collectUntilOutcome returns the domain frames received before the sync outcome
// and the outcome itself.
func (c *syncClient) collectUntilOutcome(t *testing.T) (frames []frame, outcome map[string]any) {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		select {
		case f, ok := <-c.in:
			require.True(t, ok, "connection closed before a sync outcome")
			if f.ctl != nil && (f.ctl["type"] == "synced" || f.ctl["type"] == "too_far") {
				return frames, f.ctl
			}
			if f.env.GetDomainEvent() != nil {
				frames = append(frames, f)
			}
		case <-deadline:
			t.Fatal("no sync outcome")
		}
	}
}

func seqsOf(frames []frame) []uint64 {
	out := make([]uint64, 0, len(frames))
	for _, f := range frames {
		out = append(out, f.env.GetDomainEvent().Seq)
	}
	return out
}

type redisRig struct {
	hub       *grpcserver.Hub
	url       string
	rdb       *redis.Client
	tenant    string
	workspace string
	t         *testing.T
	access    grpcserver.WorkspaceAccessChecker

	mu    sync.Mutex
	probe *syncClient // follows the workspace; proves delivery of what was published
}

// newRedisRig starts a hub on the dev Redis under tenant and workspace ids
// unique to this run, so streams of other runs never interfere.
func newRedisRig(t *testing.T, access grpcserver.WorkspaceAccessChecker) *redisRig {
	t.Helper()
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}
	rdb := redis.NewClient(&redis.Options{Addr: addr})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		t.Skipf("redis not available: %v", err)
	}
	t.Cleanup(func() { rdb.Close() })
	run := fmt.Sprintf("%d", time.Now().UnixNano())
	rig := &redisRig{rdb: rdb, tenant: "tenant-" + run, workspace: "tenant-" + run, t: t, access: access}
	hub := grpcserver.NewHub(rdb, &fakeAccess{})
	hub.SetWorkspaceAccess(access)
	t.Cleanup(hub.Close)
	srv := httptest.NewServer(hub.HandleWebSocket(hubTestSecret))
	t.Cleanup(srv.Close)
	rig.hub, rig.url = hub, "ws"+strings.TrimPrefix(srv.URL, "http")
	t.Cleanup(func() {
		ctx := context.Background()
		keys, _ := rdb.Keys(ctx, "*"+run+"*").Result()
		if len(keys) > 0 {
			rdb.Del(ctx, keys...)
		}
	})
	time.Sleep(300 * time.Millisecond) // pattern subscriptions are established in the background
	return rig
}

// ensureProbe dials, once, a session that follows the workspace. Events reach
// sessions only through Redis pub/sub, after the hub has numbered them, so the
// counter alone proves nothing about delivery; a probe that has received a frame
// proves the hub finished delivering it, and a session dialled afterwards sees it
// only through replay. Call it before anything is published.
func (r *redisRig) ensureProbe() *syncClient {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.probe != nil {
		return r.probe
	}
	if fa, ok := r.access.(*fakeWorkspaceAccess); ok {
		fa.mu.Lock()
		fa.allow[[2]string{r.workspace, "n-probe"}] = true
		fa.mu.Unlock()
	}
	p := dialSync(r.t, r.url, r.id("n-probe"))
	p.syncWorkspace(r.t, r.workspace, 0)
	_, outcome := p.collectUntilOutcome(r.t)
	require.Equal(r.t, "synced", outcome["type"])
	r.probe = p
	return p
}

// awaitFrames waits until the probe has received n more workspace frames.
func (r *redisRig) awaitFrames(n int) {
	r.t.Helper()
	p := r.ensureProbe()
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := 0; i < n; i++ {
		f := p.wait(r.t, func(f frame) bool { return f.env != nil && f.env.GetDomainEvent() != nil }, 10*time.Second)
		require.NotNil(r.t, f, "probe received %d of %d published frames", i, n)
	}
}

// publish emits n workspace events and returns once they have all been
// delivered on this hub.
func (r *redisRig) publish(n int) {
	r.t.Helper()
	r.ensureProbe()
	for i := 0; i < n; i++ {
		r.hub.PublishDomain(driveEvent(r.tenant, r.workspace, fmt.Sprintf("f%d", i)))
	}
	r.awaitFrames(n)
}

// publishToUsers emits events addressed to users and returns once each
// recipient's own probe session has received all of its frames.
func (r *redisRig) publishToUsers(events ...realtime.Event) {
	r.t.Helper()
	want := map[string]int{}
	for _, e := range events {
		seen := map[string]bool{}
		for _, node := range e.UserNodeIDs {
			if !seen[node] {
				seen[node] = true
				want[node]++
			}
		}
	}
	probes := map[string]*syncClient{}
	for node := range want {
		probes[node] = dialSync(r.t, r.url, r.id(node))
	}
	for _, e := range events {
		r.hub.PublishDomain(e)
	}
	for node, n := range want {
		for i := 0; i < n; i++ {
			f := probes[node].wait(r.t, func(f frame) bool { return f.env != nil && f.env.GetDomainEvent() != nil }, 10*time.Second)
			require.NotNil(r.t, f, "probe of %s received %d of %d frames", node, i, n)
		}
	}
}

func (r *redisRig) id(node string) identity {
	return identity{"u-" + node, node, node, r.tenant}
}

func TestReplay_SendsMissedFramesInOrderThenSynced(t *testing.T) {
	access := &fakeWorkspaceAccess{allow: map[[2]string]bool{}}
	rig := newRedisRig(t, access)
	access.allow[[2]string{rig.workspace, "n-a1"}] = true
	rig.publish(5)

	c := dialSync(t, rig.url, rig.id("n-a1"))
	c.syncWorkspace(t, rig.workspace, 2)
	frames, outcome := c.collectUntilOutcome(t)

	assert.Equal(t, []uint64{3, 4, 5}, seqsOf(frames), "exactly what the client missed, in order")
	assert.Equal(t, "synced", outcome["type"])
	assert.Equal(t, "workspace", outcome["scope"])
	assert.EqualValues(t, 5, outcome["seq"])
	assert.EqualValues(t, 3, outcome["replayed"])
	assert.Equal(t, []string{"f2"}, frames[0].env.GetDomainEvent().Ids, "frames are hints with ids only")

	// After the sync the next live event is the next number.
	rig.publish(1)
	live := c.wait(t, func(f frame) bool { return f.env.GetDomainEvent() != nil }, 3*time.Second)
	require.NotNil(t, live)
	assert.EqualValues(t, 6, live.env.GetDomainEvent().Seq)
}

func TestReplay_UpToDateClientGetsSyncedWithNothing(t *testing.T) {
	access := &fakeWorkspaceAccess{allow: map[[2]string]bool{}}
	rig := newRedisRig(t, access)
	access.allow[[2]string{rig.workspace, "n-a1"}] = true
	rig.publish(3)

	c := dialSync(t, rig.url, rig.id("n-a1"))
	c.syncWorkspace(t, rig.workspace, 3)
	frames, outcome := c.collectUntilOutcome(t)

	assert.Empty(t, frames)
	assert.Equal(t, "synced", outcome["type"])
	assert.EqualValues(t, 3, outcome["seq"])
}

func TestReplay_ReplayedFrameIsTheLiveFrame(t *testing.T) {
	access := &fakeWorkspaceAccess{allow: map[[2]string]bool{}}
	rig := newRedisRig(t, access)
	access.allow[[2]string{rig.workspace, "n-a1"}] = true
	access.allow[[2]string{rig.workspace, "n-a2"}] = true

	live := dialSync(t, rig.url, rig.id("n-a1"))
	live.sendJSON(t, map[string]any{"type": "sync", "workspace_id": rig.workspace, "since": 0})
	_, first := live.collectUntilOutcome(t)
	require.Equal(t, "synced", first["type"])
	rig.publish(3)
	var liveFrames []frame
	for len(liveFrames) < 3 {
		f := live.wait(t, func(f frame) bool { return f.env.GetDomainEvent() != nil }, 3*time.Second)
		require.NotNil(t, f, "live frame %d never arrived", len(liveFrames))
		liveFrames = append(liveFrames, *f)
	}

	late := dialSync(t, rig.url, rig.id("n-a2"))
	late.syncWorkspace(t, rig.workspace, 0)
	replayed, outcome := late.collectUntilOutcome(t)

	require.Equal(t, "synced", outcome["type"])
	require.Len(t, replayed, 3)
	for i := range replayed {
		assert.Equal(t, liveFrames[i].raw, replayed[i].raw, "frame %d: replay and live must be byte-identical", i)
	}
}

func TestReplay_TooFarWhenOutsideTheWindow(t *testing.T) {
	access := &fakeWorkspaceAccess{allow: map[[2]string]bool{}}
	rig := newRedisRig(t, access)
	access.allow[[2]string{rig.workspace, "n-a1"}] = true
	rig.publish(5)

	t.Run("cursor ahead of the counter", func(t *testing.T) {
		c := dialSync(t, rig.url, rig.id("n-a1"))
		c.syncWorkspace(t, rig.workspace, 99)
		frames, outcome := c.collectUntilOutcome(t)
		assert.Empty(t, frames)
		assert.Equal(t, "too_far", outcome["type"], "a counter that went backwards cannot be replayed from")
		assert.EqualValues(t, 5, outcome["seq"], "the outcome carries the number to restart from")
	})

	t.Run("entries already trimmed", func(t *testing.T) {
		ctx := context.Background()
		require.NoError(t, rig.rdb.XTrimMinID(ctx, "rt:ws:"+rig.workspace, "4-0").Err())
		c := dialSync(t, rig.url, rig.id("n-a1"))
		c.syncWorkspace(t, rig.workspace, 1)
		frames, outcome := c.collectUntilOutcome(t)
		assert.Empty(t, frames, "a partial replay would hide the hole")
		assert.Equal(t, "too_far", outcome["type"])
	})
}

func TestReplay_TooFarWhenMoreThanTheLimitBehind(t *testing.T) {
	access := &fakeWorkspaceAccess{allow: map[[2]string]bool{}}
	rig := newRedisRig(t, access)
	access.allow[[2]string{rig.workspace, "n-a1"}] = true
	rig.publish(2001)

	c := dialSync(t, rig.url, rig.id("n-a1"))
	c.syncWorkspace(t, rig.workspace, 0)
	frames, outcome := c.collectUntilOutcome(t)
	assert.Empty(t, frames)
	assert.Equal(t, "too_far", outcome["type"])
	assert.EqualValues(t, 2001, outcome["seq"])

	c.syncWorkspace(t, rig.workspace, 1) // exactly 2000 behind
	frames, outcome = c.collectUntilOutcome(t)
	assert.Equal(t, "synced", outcome["type"])
	assert.Len(t, frames, 2000)
}

func TestReplay_DeniedSubscriptionReplaysNothing(t *testing.T) {
	access := &fakeWorkspaceAccess{allow: map[[2]string]bool{}}
	rig := newRedisRig(t, access)
	rig.publish(4) // the stream holds frames; the session has no right to them

	c := dialSync(t, rig.url, rig.id("n-x"))
	c.syncWorkspace(t, rig.workspace, 0)

	assert.NotNil(t, c.wait(t, func(f frame) bool { return f.env.GetError() != nil && f.env.GetError().Code == 403 }, 2*time.Second))
	assert.Nil(t, c.wait(t, func(f frame) bool { return f.env.GetDomainEvent() != nil || isAnyOutcome(f) }, quiet),
		"a refused session gets no frame and no sync outcome")
	rig.publish(1)
	assert.Nil(t, c.wait(t, func(f frame) bool { return f.env.GetDomainEvent() != nil }, quiet), "and is not subscribed")
}

// Even if a checker wrongly let a session of another tenant through, the log
// holds the frame's own tenant and a mismatch is never replayed.
func TestReplay_NeverReplaysAnotherTenantsFrames(t *testing.T) {
	access := &fakeWorkspaceAccess{allow: map[[2]string]bool{}}
	rig := newRedisRig(t, access)
	access.allow[[2]string{rig.workspace, "n-b1"}] = true // granted, but in the wrong tenant
	rig.publish(2)

	foreign := dialSync(t, rig.url, identity{"u-b1", "bella", "n-b1", "other-" + rig.tenant})
	foreign.syncWorkspace(t, rig.workspace, 0)

	got := foreign.wait(t, func(f frame) bool { return f.env.GetDomainEvent() != nil }, quiet)
	assert.Nil(t, got, "frames of tenant A never reach a session of tenant B, whatever the checker says")
}

func TestReplay_WithoutRedisThereIsNoReplayAndNoError(t *testing.T) {
	access := &fakeWorkspaceAccess{allow: map[[2]string]bool{{"ws-1", "n-a1"}: true}}
	hub, url := startDomainHub(t, access)
	hub.PublishDomain(driveEvent("tenant-a", "ws-1", "f1"))

	c := dialSync(t, url, identity{"u-a1", "alice", "n-a1", "tenant-a"})
	c.syncWorkspace(t, "ws-1", 0)
	frames, outcome := c.collectUntilOutcome(t)

	assert.Empty(t, frames)
	assert.Equal(t, "too_far", outcome["type"], "no log means the client refreshes as it always did")
	assert.Nil(t, c.wait(t, func(f frame) bool { return f.env.GetError() != nil }, quiet), "and nothing is an error")

	hub.PublishDomain(driveEvent("tenant-a", "ws-1", "f2"))
	live := c.wait(t, func(f frame) bool { return f.env.GetDomainEvent() != nil }, 2*time.Second)
	require.NotNil(t, live, "the session still follows the workspace live")
	assert.EqualValues(t, 2, live.env.GetDomainEvent().Seq)

	c.syncUser(t, 0)
	_, userOutcome := c.collectUntilOutcome(t)
	assert.Equal(t, "too_far", userOutcome["type"])
	assert.Equal(t, "user", userOutcome["scope"])
}

func TestReplay_LiveEventsDuringASyncStayInOrder(t *testing.T) {
	access := &fakeWorkspaceAccess{allow: map[[2]string]bool{}}
	rig := newRedisRig(t, access)
	access.allow[[2]string{rig.workspace, "n-a1"}] = true
	rig.publish(20)

	c := dialSync(t, rig.url, rig.id("n-a1"))
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 30; i++ {
			rig.publish(1)
			time.Sleep(time.Millisecond)
		}
	}()
	c.syncWorkspace(t, rig.workspace, 0)
	wg.Wait()

	var seqs []uint64
	deadline := time.After(5 * time.Second)
	for len(seqs) < 50 {
		select {
		case f := <-c.in:
			if d := f.env.GetDomainEvent(); d != nil {
				seqs = append(seqs, d.Seq)
			}
		case <-deadline:
			t.Fatalf("only %v arrived", seqs)
		}
	}
	for i, s := range seqs {
		assert.EqualValues(t, i+1, s, "replay then live, one sequence, no repeat, no hole")
	}
}

func TestEventLog_NumberAndFrameAppendTogether(t *testing.T) {
	access := &fakeWorkspaceAccess{allow: map[[2]string]bool{}}
	rig := newRedisRig(t, access)
	ctx := context.Background()

	rig.ensureProbe()
	var wg sync.WaitGroup
	for i := 0; i < 40; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); rig.hub.PublishDomain(driveEvent(rig.tenant, rig.workspace, "f")) }()
	}
	wg.Wait()
	rig.awaitFrames(40)

	counter, err := rig.rdb.Get(ctx, "wsseq:"+rig.workspace).Uint64()
	require.NoError(t, err)
	entries, err := rig.rdb.XRange(ctx, "rt:ws:"+rig.workspace, "-", "+").Result()
	require.NoError(t, err)
	require.EqualValues(t, 40, counter)
	require.Len(t, entries, 40, "every number has its frame")
	for i, e := range entries {
		assert.Equal(t, fmt.Sprintf("%d-0", i+1), e.ID, "entry %d is stored under its own number", i)
	}

	// A counter that was lost while a stream survived: the stale stream is
	// dropped and the log restarts with the new numbering, never diverging.
	require.NoError(t, rig.rdb.Del(ctx, "wsseq:"+rig.workspace).Err())
	rig.publish(1)
	entries, err = rig.rdb.XRange(ctx, "rt:ws:"+rig.workspace, "-", "+").Result()
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "1-0", entries[0].ID)
	ttl, err := rig.rdb.TTL(ctx, "rt:ws:"+rig.workspace).Result()
	require.NoError(t, err)
	assert.Greater(t, ttl, 23*time.Hour, "an idle stream expires after the retention")
	assert.LessOrEqual(t, ttl, 24*time.Hour)
}

func TestEventLog_AgeTrimDropsExpiredEntries(t *testing.T) {
	access := &fakeWorkspaceAccess{allow: map[[2]string]bool{}}
	rig := newRedisRig(t, access)
	rig.ensureProbe()
	ctx := context.Background()
	key := "rt:ws:" + rig.workspace
	old := time.Now().Add(-48 * time.Hour).UnixMilli()
	for i := 1; i <= 49; i++ {
		require.NoError(t, rig.rdb.XAdd(ctx, &redis.XAddArgs{Stream: key, ID: fmt.Sprintf("%d-0", i),
			Values: []any{"f", "x", "t", old}}).Err())
	}
	require.NoError(t, rig.rdb.Set(ctx, "wsseq:"+rig.workspace, 49, 0).Err())

	rig.publish(2) // numbers 50 and 51: the 50th triggers the age trim

	entries, err := rig.rdb.XRange(ctx, key, "-", "+").Result()
	require.NoError(t, err)
	require.Len(t, entries, 2, "entries older than the retention are gone, the fresh ones stay")
	assert.Equal(t, "50-0", entries[0].ID)
}

func TestUserStream_CounterAdvancesByOnePerUserEvent(t *testing.T) {
	access := &fakeWorkspaceAccess{allow: map[[2]string]bool{}}
	rig := newRedisRig(t, access)
	ctx := context.Background()
	a1 := dialSync(t, rig.url, rig.id("n-a1"))

	perm := func(nodes ...string) realtime.Event {
		return realtime.Event{Domain: realtime.DomainPermission, Kind: realtime.KindChanged,
			TenantID: rig.tenant, WorkspaceID: rig.workspace, UserNodeIDs: nodes}
	}
	rig.hub.PublishDomain(perm("n-a1"))
	rig.hub.PublishDomain(perm("n-a1", "n-a2")) // one event, two users: one number each
	rig.hub.PublishDomain(perm("n-a1"))

	n1, err := rig.rdb.Get(ctx, "useq:"+rig.tenant+":n-a1").Uint64()
	require.NoError(t, err)
	n2, err := rig.rdb.Get(ctx, "useq:"+rig.tenant+":n-a2").Uint64()
	require.NoError(t, err)
	assert.EqualValues(t, 3, n1)
	assert.EqualValues(t, 1, n2)
	l1, _ := rig.rdb.XLen(ctx, "rt:user:"+rig.tenant+":n-a1").Result()
	assert.EqualValues(t, 3, l1, "each number has its frame")

	// The live session learns each number right after the frame, and the
	// frame itself is unchanged (seq 0: it is not part of the workspace stream).
	var cursors []float64
	for len(cursors) < 3 {
		f := a1.wait(t, func(f frame) bool { return isCtl("user_seq")(f) }, 3*time.Second)
		require.NotNil(t, f)
		cursors = append(cursors, f.ctl["seq"].(float64))
	}
	assert.Equal(t, []float64{1, 2, 3}, cursors)
}

func TestUserStream_ReplaysOwnFramesOnly(t *testing.T) {
	access := &fakeWorkspaceAccess{allow: map[[2]string]bool{}}
	rig := newRedisRig(t, access)
	perm := func(nodes ...string) realtime.Event {
		return realtime.Event{Domain: realtime.DomainPermission, Kind: realtime.KindChanged,
			TenantID: rig.tenant, WorkspaceID: rig.workspace, UserNodeIDs: nodes}
	}
	rig.publishToUsers(perm("n-a1"), perm("n-a2"), perm("n-a1"), perm("n-a1"))

	own := dialSync(t, rig.url, rig.id("n-a1"))
	own.syncUser(t, 1)
	frames, outcome := own.collectUntilOutcome(t)
	assert.Len(t, frames, 2, "the frames after cursor 1, in order")
	assert.Equal(t, "synced", outcome["type"])
	assert.Equal(t, "user", outcome["scope"])
	assert.EqualValues(t, 3, outcome["seq"])
	assert.EqualValues(t, 2, outcome["replayed"])
	for _, f := range frames {
		assert.EqualValues(t, 0, f.env.GetDomainEvent().Seq, "a user frame is not a workspace-sequenced one")
	}

	other := dialSync(t, rig.url, rig.id("n-a2"))
	other.syncUser(t, 0)
	frames, outcome = other.collectUntilOutcome(t)
	assert.Len(t, frames, 1, "another user sees only their own stream")
	assert.EqualValues(t, 1, outcome["seq"])

	foreign := dialSync(t, rig.url, identity{"u-a1", "alice", "n-a1", "other-" + rig.tenant})
	foreign.syncUser(t, 0)
	frames, outcome = foreign.collectUntilOutcome(t)
	assert.Empty(t, frames, "the same node id in another tenant is another stream")
	assert.EqualValues(t, 0, outcome["seq"])

	own.sendJSON(t, map[string]any{"type": "user_sync", "since": 0, "user_node_id": "n-a2"})
	frames, outcome = own.collectUntilOutcome(t)
	assert.Len(t, frames, 3, "naming someone else's node changes nothing: the stream is the session's own")
	assert.EqualValues(t, 3, outcome["seq"])
}

func TestUserStream_TooFarOutsideTheWindow(t *testing.T) {
	access := &fakeWorkspaceAccess{allow: map[[2]string]bool{}}
	rig := newRedisRig(t, access)
	c := dialSync(t, rig.url, rig.id("n-a1"))
	rig.hub.PublishDomain(realtime.Event{Domain: realtime.DomainPermission, Kind: realtime.KindChanged,
		TenantID: rig.tenant, WorkspaceID: rig.workspace, UserNodeIDs: []string{"n-a1"}})
	// The session is already connected, so it receives the event live; once it has
	// its number the event is behind it and only the sync below can be answered.
	live := c.wait(t, isCtl("user_seq"), 5*time.Second)
	require.NotNil(t, live, "the live frame never arrived")
	require.EqualValues(t, 1, live.ctl["seq"])

	c.syncUser(t, 40)
	frames, outcome := c.collectUntilOutcome(t)
	assert.Empty(t, frames)
	assert.Equal(t, "too_far", outcome["type"])
	assert.EqualValues(t, 1, outcome["seq"])
}

func TestSync_RequiresAuthAndAWellFormedRequest(t *testing.T) {
	access := &fakeWorkspaceAccess{allow: map[[2]string]bool{{"ws-1", "n-a1"}: true}}
	_, url := startDomainHub(t, access)

	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })
	require.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"sync","workspace_id":"ws-1","since":0}`)))
	_, data, err := conn.ReadMessage()
	require.NoError(t, err)
	var env pb.ServerEnvelope
	require.NoError(t, proto.Unmarshal(data, &env))
	assert.EqualValues(t, 401, env.GetError().GetCode(), "an unauthenticated session cannot sync")

	c := dialSync(t, url, identity{"u-a1", "alice", "n-a1", "tenant-a"})
	c.sendJSON(t, map[string]any{"type": "sync", "workspace_id": "ws-1"}) // no cursor
	assert.NotNil(t, c.wait(t, func(f frame) bool { return f.env.GetError() != nil && f.env.GetError().Code == 400 }, 2*time.Second))
	c.sendJSON(t, map[string]any{"type": "bogus", "since": 0})
	assert.NotNil(t, c.wait(t, func(f frame) bool { return f.env.GetError() != nil && f.env.GetError().Code == 400 }, 2*time.Second))
}
