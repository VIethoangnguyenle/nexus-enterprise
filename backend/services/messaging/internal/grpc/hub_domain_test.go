package grpc_test

import (
	"context"
	"fmt"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/pkg/realtime"
	pb "ngac-platform/proto/messaging"
	"ngac-platform/services/messaging/internal/domain"
	grpcserver "ngac-platform/services/messaging/internal/grpc"
)

// fakeWorkspaceAccess grants read on exactly the (workspace, user node) pairs
// it is told to, and records what it was asked.
type fakeWorkspaceAccess struct {
	mu     sync.Mutex
	allow  map[[2]string]bool
	called [][4]string
}

func (f *fakeWorkspaceAccess) AuthorizeWorkspaceAccess(_ context.Context, workspaceID, userNodeID, tenantID, operation string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.called = append(f.called, [4]string{workspaceID, userNodeID, tenantID, operation})
	if f.allow[[2]string{workspaceID, userNodeID}] {
		return nil
	}
	return domain.ErrAccessDenied
}

func (f *fakeWorkspaceAccess) asked() [][4]string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([][4]string(nil), f.called...)
}

func allowAll(pairs ...[2]string) *fakeWorkspaceAccess {
	m := map[[2]string]bool{}
	for _, p := range pairs {
		m[p] = true
	}
	return &fakeWorkspaceAccess{allow: m}
}

func (c *wsTestClient) subscribeWorkspace(t *testing.T, workspaceID string) {
	t.Helper()
	c.send(t, &pb.ClientEnvelope{Payload: &pb.ClientEnvelope_Subscribe{
		Subscribe: &pb.SubscribeRequest{WorkspaceId: workspaceID},
	}})
}

func isSubscribed(workspaceID string) func(*pb.ServerEnvelope) bool {
	return func(e *pb.ServerEnvelope) bool {
		return e.GetWorkspaceSubscribed() != nil && e.GetWorkspaceSubscribed().WorkspaceId == workspaceID
	}
}

func isDomain(domainName, kind string) func(*pb.ServerEnvelope) bool {
	return func(e *pb.ServerEnvelope) bool {
		d := e.GetDomainEvent()
		return d != nil && d.Domain == domainName && d.Kind == kind
	}
}

// joinWorkspace subscribes c and waits for the acknowledgement.
func joinWorkspace(t *testing.T, c *wsTestClient, workspaceID string) *pb.WorkspaceSubscribed {
	t.Helper()
	c.subscribeWorkspace(t, workspaceID)
	ack := c.next(t, isSubscribed(workspaceID), 2*time.Second)
	require.NotNil(t, ack, "workspace subscription was not acknowledged")
	return ack.GetWorkspaceSubscribed()
}

func startDomainHub(t *testing.T, ws *fakeWorkspaceAccess) (*grpcserver.Hub, string) {
	t.Helper()
	hub, url := startHub(t, &fakeAccess{allow: map[[3]string]bool{
		{"ch-1", "n-a1", "read"}: true, {"ch-1", "n-a2", "read"}: true,
	}})
	hub.SetWorkspaceAccess(ws)
	return hub, url
}

func driveEvent(tenant, workspace string, ids ...string) realtime.Event {
	return realtime.Event{
		Domain: realtime.DomainDrive, Kind: realtime.KindUpdated,
		TenantID: tenant, WorkspaceID: workspace, IDs: ids, ActorUserID: "u-actor",
	}
}

func TestWorkspaceSubscribe_DeniedWithoutRead(t *testing.T) {
	access := allowAll([2]string{"ws-1", "n-a1"})
	hub, url := startDomainHub(t, access)
	outsider := connect(t, url, identity{"u-x", "xavier", "n-x", "tenant-a"})

	outsider.subscribeWorkspace(t, "ws-1")

	assert.NotNil(t, outsider.next(t, isError(403), 2*time.Second), "a subscribe without read must be refused")
	hub.PublishDomain(driveEvent("tenant-a", "ws-1", "f1"))
	assert.Nil(t, outsider.next(t, isDomain("drive", "updated"), quiet), "a refused subscription must receive nothing")
	require.Len(t, access.asked(), 1)
	assert.Equal(t, [4]string{"ws-1", "n-x", "tenant-a", "read"}, access.asked()[0], "the check is a read, as the session's tenant")
}

func TestWorkspaceSubscribe_DeniedWhenUnauthenticatedOrNoChecker(t *testing.T) {
	hub, url := startHub(t, &fakeAccess{}) // no workspace checker installed
	c := connect(t, url, identity{"u-a1", "alice", "n-a1", "tenant-a"})
	c.subscribeWorkspace(t, "ws-1")
	assert.NotNil(t, c.next(t, isError(403), 2*time.Second), "no checker means no subscription")
	hub.PublishDomain(driveEvent("tenant-a", "ws-1", "f1"))
	assert.Nil(t, c.next(t, isDomain("drive", "updated"), quiet))
}

func TestDomainEvent_NeverCrossesTenants(t *testing.T) {
	access := allowAll([2]string{"ws-1", "n-a1"}, [2]string{"ws-1", "n-b1"})
	hub, url := startDomainHub(t, access)
	inTenant := connect(t, url, identity{"u-a1", "alice", "n-a1", "tenant-a"})
	foreign := connect(t, url, identity{"u-b1", "bella", "n-b1", "tenant-b"})
	joinWorkspace(t, inTenant, "ws-1")
	joinWorkspace(t, foreign, "ws-1") // even holding a subscription to the same workspace id

	hub.PublishDomain(driveEvent("tenant-a", "ws-1", "f1"))

	got := inTenant.next(t, isDomain("drive", "updated"), 2*time.Second)
	require.NotNil(t, got)
	assert.Equal(t, []string{"f1"}, got.GetDomainEvent().Ids)
	assert.Equal(t, "tenant-a", got.GetDomainEvent().TenantId)
	assert.Equal(t, "ws-1", got.GetDomainEvent().WorkspaceId)
	assert.Nil(t, foreign.next(t, isDomain("drive", "updated"), quiet), "a tenant-B session must never see a tenant-A event")
}

func TestDomainEvent_OnlyForSubscribedWorkspace(t *testing.T) {
	access := allowAll([2]string{"ws-1", "n-a1"}, [2]string{"ws-2", "n-a2"})
	hub, url := startDomainHub(t, access)
	one := connect(t, url, identity{"u-a1", "alice", "n-a1", "tenant-a"})
	two := connect(t, url, identity{"u-a2", "bob", "n-a2", "tenant-a"})
	idle := connect(t, url, identity{"u-a3", "cara", "n-a3", "tenant-a"})
	joinWorkspace(t, one, "ws-1")
	joinWorkspace(t, two, "ws-2")

	hub.PublishDomain(driveEvent("tenant-a", "ws-1", "f1"))

	assert.NotNil(t, one.next(t, isDomain("drive", "updated"), 2*time.Second))
	assert.Nil(t, two.next(t, isDomain("drive", "updated"), quiet), "subscribed to another workspace")
	assert.Nil(t, idle.next(t, isDomain("drive", "updated"), quiet), "subscribed to nothing")
}

func TestWorkspaceUnsubscribe_StopsDelivery(t *testing.T) {
	hub, url := startDomainHub(t, allowAll([2]string{"ws-1", "n-a1"}))
	c := connect(t, url, identity{"u-a1", "alice", "n-a1", "tenant-a"})
	joinWorkspace(t, c, "ws-1")
	c.send(t, &pb.ClientEnvelope{Payload: &pb.ClientEnvelope_Unsubscribe{
		Unsubscribe: &pb.UnsubscribeRequest{WorkspaceId: "ws-1"},
	}})
	time.Sleep(100 * time.Millisecond)

	hub.PublishDomain(driveEvent("tenant-a", "ws-1", "f1"))
	assert.Nil(t, c.next(t, isDomain("drive", "updated"), quiet))
}

func TestDomainEvent_SeqIsMonotonicPerWorkspace(t *testing.T) {
	access := allowAll([2]string{"ws-1", "n-a1"}, [2]string{"ws-2", "n-a1"})
	hub, url := startDomainHub(t, access)
	c := connect(t, url, identity{"u-a1", "alice", "n-a1", "tenant-a"})
	ack1 := joinWorkspace(t, c, "ws-1")
	ack2 := joinWorkspace(t, c, "ws-2")
	assert.Zero(t, ack1.Seq, "nothing issued yet")
	assert.Zero(t, ack2.Seq)

	for i := 0; i < 5; i++ {
		hub.PublishDomain(driveEvent("tenant-a", "ws-1", fmt.Sprintf("f%d", i)))
	}
	hub.PublishDomain(driveEvent("tenant-a", "ws-2", "g0"))

	var seqs1, seqs2 []uint64
	for i := 0; i < 6; i++ {
		e := c.next(t, func(e *pb.ServerEnvelope) bool { return e.GetDomainEvent() != nil }, 2*time.Second)
		require.NotNil(t, e)
		if e.GetDomainEvent().WorkspaceId == "ws-1" {
			seqs1 = append(seqs1, e.GetDomainEvent().Seq)
		} else {
			seqs2 = append(seqs2, e.GetDomainEvent().Seq)
		}
	}
	assert.Equal(t, []uint64{1, 2, 3, 4, 5}, seqs1)
	assert.Equal(t, []uint64{1}, seqs2, "each workspace counts on its own")

	late := connect(t, url, identity{"u-a1b", "alice", "n-a1", "tenant-a"})
	assert.Equal(t, uint64(5), joinWorkspace(t, late, "ws-1").Seq, "the ack reports the last sequence issued")
}

func TestDomainEvent_UserAddressedReachesOnlyThatUser(t *testing.T) {
	access := allowAll([2]string{"ws-1", "n-a1"}, [2]string{"ws-1", "n-a2"})
	hub, url := startDomainHub(t, access)
	target := connect(t, url, identity{"u-a1", "alice", "n-a1", "tenant-a"})
	targetTab2 := connect(t, url, identity{"u-a1", "alice", "n-a1", "tenant-a"})
	colleague := connect(t, url, identity{"u-a2", "bob", "n-a2", "tenant-a"})
	sameNodeOtherTenant := connect(t, url, identity{"u-a1", "alice", "n-a1", "tenant-b"})
	for _, c := range []*wsTestClient{target, colleague} {
		joinWorkspace(t, c, "ws-1")
	}

	hub.PublishDomain(realtime.Event{
		Domain: realtime.DomainPermission, Kind: realtime.KindChanged,
		TenantID: "tenant-a", WorkspaceID: "ws-1", UserNodeIDs: []string{"n-a1"},
	})

	got := target.next(t, isDomain("permission", "changed"), 2*time.Second)
	require.NotNil(t, got)
	assert.Zero(t, got.GetDomainEvent().Seq, "user-addressed events are outside the workspace stream")
	assert.NotNil(t, targetTab2.next(t, isDomain("permission", "changed"), 2*time.Second), "every session of the user")
	assert.Nil(t, colleague.next(t, isDomain("permission", "changed"), quiet), "a colleague in the same workspace gets nothing")
	assert.Nil(t, sameNodeOtherTenant.next(t, isDomain("permission", "changed"), quiet), "the node id in another tenant is someone else")
}

func TestDomainEvent_ChannelAddressedReachesOnlySubscribers(t *testing.T) {
	hub, url := startDomainHub(t, allowAll([2]string{"ws-1", "n-a1"}, [2]string{"ws-1", "n-a2"}))
	member := connect(t, url, identity{"u-a1", "alice", "n-a1", "tenant-a"})
	other := connect(t, url, identity{"u-a2", "bob", "n-a2", "tenant-a"})
	foreign := connect(t, url, identity{"u-b1", "bella", "n-b1", "tenant-b"})
	subscribeAndWait(t, hub, member, "ch-1")
	joinWorkspace(t, other, "ws-1") // in the workspace, not in the channel

	hub.PublishDomain(realtime.Event{
		Domain: realtime.DomainChannel, Kind: realtime.KindRenamed,
		TenantID: "tenant-a", WorkspaceID: "ws-1", ChannelID: "ch-1", IDs: []string{"ch-1"},
	})

	got := member.next(t, isDomain("channel", "renamed"), 2*time.Second)
	require.NotNil(t, got)
	assert.Equal(t, "ch-1", got.GetDomainEvent().ChannelId)
	assert.Zero(t, got.GetDomainEvent().Seq)
	assert.Nil(t, other.next(t, isDomain("channel", "renamed"), quiet), "workspace membership is not channel membership")
	assert.Nil(t, foreign.next(t, isDomain("channel", "renamed"), quiet))
}

func TestDomainEvent_InvalidEventIsDropped(t *testing.T) {
	hub, url := startDomainHub(t, allowAll([2]string{"ws-1", "n-a1"}))
	c := connect(t, url, identity{"u-a1", "alice", "n-a1", "tenant-a"})
	joinWorkspace(t, c, "ws-1")

	hub.PublishDomain(driveEvent("", "ws-1", "f1"))     // no tenant
	hub.PublishDomain(driveEvent("tenant-a", "", "f1")) // no workspace
	assert.Nil(t, c.next(t, isDomain("drive", "updated"), quiet), "an event without a boundary is never delivered")

	hub.PublishDomain(driveEvent("tenant-a", "ws-1", "f1"))
	got := c.next(t, isDomain("drive", "updated"), 2*time.Second)
	require.NotNil(t, got)
	assert.Equal(t, uint64(1), got.GetDomainEvent().Seq, "dropped events do not consume sequence numbers")
}

func TestPresence_OfflineWaitsOutAQuickReconnect(t *testing.T) {
	hub, url := startHub(t, &fakeAccess{})
	hub.SetPresenceGrace(400 * time.Millisecond)
	watcher := connect(t, url, identity{"u-a1", "alice", "n-a1", "tenant-a"})

	flaky := connect(t, url, identity{"u-a2", "bob", "n-a2", "tenant-a"})
	require.NotNil(t, watcher.next(t, isPresenceOf("u-a2"), 2*time.Second))
	_ = flaky.conn.Close()
	time.Sleep(100 * time.Millisecond)
	connect(t, url, identity{"u-a2", "bob", "n-a2", "tenant-a"}) // back inside the grace period

	offline := func(e *pb.ServerEnvelope) bool {
		return e.GetPresenceEvent() != nil && e.GetPresenceEvent().Status == "offline"
	}
	assert.Nil(t, watcher.next(t, offline, time.Second), "a reconnect inside the grace period is not an absence")
}

func TestPresence_OfflineAnnouncedAfterGrace(t *testing.T) {
	hub, url := startHub(t, &fakeAccess{})
	hub.SetPresenceGrace(150 * time.Millisecond)
	watcher := connect(t, url, identity{"u-a1", "alice", "n-a1", "tenant-a"})
	gone := connect(t, url, identity{"u-a2", "bob", "n-a2", "tenant-a"})
	require.NotNil(t, watcher.next(t, isPresenceOf("u-a2"), 2*time.Second))
	_ = gone.conn.Close()

	got := watcher.next(t, func(e *pb.ServerEnvelope) bool {
		return e.GetPresenceEvent() != nil && e.GetPresenceEvent().UserId == "u-a2" && e.GetPresenceEvent().Status == "offline"
	}, 2*time.Second)
	assert.NotNil(t, got, "a real absence is announced once the grace period passes")
}

// Sequence numbers come from Redis when it is there, so two hub instances
// issue one monotonic stream per workspace, and a subscriber on either one
// sees events published through the other.
func TestDomainEvent_SeqAndDeliveryAcrossInstancesViaRedis(t *testing.T) {
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
	tenant, workspace := "tenant-"+run, "ws-"+run
	access := allowAll([2]string{workspace, "n-a1"})
	start := func() (*grpcserver.Hub, string) {
		hub := grpcserver.NewHub(rdb, &fakeAccess{})
		hub.SetWorkspaceAccess(access)
		t.Cleanup(hub.Close)
		srv := httptest.NewServer(hub.HandleWebSocket(hubTestSecret))
		t.Cleanup(srv.Close)
		return hub, "ws" + strings.TrimPrefix(srv.URL, "http")
	}
	hubOne, _ := start()
	hubTwo, urlTwo := start()
	c := connect(t, urlTwo, identity{"u-a1", "alice", "n-a1", tenant})
	joinWorkspace(t, c, workspace)
	time.Sleep(200 * time.Millisecond) // pattern subscriptions are established in the background

	hubOne.PublishDomain(driveEvent(tenant, workspace, "f1"))
	hubTwo.PublishDomain(driveEvent(tenant, workspace, "f2"))
	hubOne.PublishDomain(driveEvent(tenant, workspace, "f3"))

	var seqs []uint64
	for i := 0; i < 3; i++ {
		e := c.next(t, isDomain("drive", "updated"), 3*time.Second)
		require.NotNil(t, e, "event %d never arrived", i)
		seqs = append(seqs, e.GetDomainEvent().Seq)
	}
	assert.ElementsMatch(t, []uint64{seqs[0], seqs[0] + 1, seqs[0] + 2}, seqs, "one stream, no repeats, no holes")
}

// mutableWorkspaceAccess is a checker whose grants can be taken away while a
// session is open.
type mutableWorkspaceAccess struct {
	mu    sync.Mutex
	allow map[[2]string]bool
}

func (m *mutableWorkspaceAccess) AuthorizeWorkspaceAccess(_ context.Context, ws, node, _, _ string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.allow[[2]string{ws, node}] {
		return nil
	}
	return domain.ErrAccessDenied
}

func (m *mutableWorkspaceAccess) revoke(ws, node string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.allow, [2]string{ws, node})
}

func TestRevokeWorkspaceSubscriptions_StopsDeliveryToThatUserOnly(t *testing.T) {
	hub, url := startDomainHub(t, allowAll([2]string{"tenant-a", "n-a1"}, [2]string{"tenant-a", "n-a2"}))
	removed := connect(t, url, identity{"u-a1", "alice", "n-a1", "tenant-a"})
	stays := connect(t, url, identity{"u-a2", "bob", "n-a2", "tenant-a"})
	joinWorkspace(t, removed, "tenant-a")
	joinWorkspace(t, stays, "tenant-a")
	subscribeAndWait(t, hub, removed, "ch-1")

	hub.RevokeWorkspaceSubscriptions("tenant-a", "n-a1")
	hub.PublishDomain(driveEvent("tenant-a", "tenant-a", "f1"))
	hub.BroadcastToChannel("ch-1", secretMessage("ch-1"))

	assert.NotNil(t, stays.next(t, isDomain("drive", "updated"), 2*time.Second), "a colleague keeps following")
	assert.Nil(t, removed.next(t, isDomain("drive", "updated"), quiet), "a removed member stops receiving workspace events")
	assert.Nil(t, removed.next(t, isChat("ch-1"), quiet), "and the channels of that workspace")
}

func TestRevokeWorkspaceSubscriptions_DoesNotTouchAnotherTenantSession(t *testing.T) {
	hub, url := startDomainHub(t, allowAll([2]string{"tenant-b", "n-a1"}))
	elsewhere := connect(t, url, identity{"u-a1", "alice", "n-a1", "tenant-b"})
	joinWorkspace(t, elsewhere, "tenant-b")

	hub.RevokeWorkspaceSubscriptions("tenant-a", "n-a1") // the same person, removed from a different workspace
	hub.PublishDomain(driveEvent("tenant-b", "tenant-b", "f1"))

	assert.NotNil(t, elsewhere.next(t, isDomain("drive", "updated"), 2*time.Second))
}

func TestRevokeWorkspaceSubscriptions_AcrossInstancesViaRedis(t *testing.T) {
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
	tenant := "tenant-" + run
	access := allowAll([2]string{tenant, "n-a1"})
	start := func() (*grpcserver.Hub, string) {
		hub := grpcserver.NewHub(rdb, &fakeAccess{})
		hub.SetWorkspaceAccess(access)
		t.Cleanup(hub.Close)
		srv := httptest.NewServer(hub.HandleWebSocket(hubTestSecret))
		t.Cleanup(srv.Close)
		return hub, "ws" + strings.TrimPrefix(srv.URL, "http")
	}
	hubOne, _ := start()
	_, urlTwo := start()
	c := connect(t, urlTwo, identity{"u-a1", "alice", "n-a1", tenant})
	joinWorkspace(t, c, tenant)
	time.Sleep(200 * time.Millisecond)

	hubOne.RevokeWorkspaceSubscriptions(tenant, "n-a1") // revoked through the other instance
	time.Sleep(200 * time.Millisecond)
	hubOne.PublishDomain(driveEvent(tenant, tenant, "f1"))

	assert.Nil(t, c.next(t, isDomain("drive", "updated"), quiet))
}

// A permission event is the moment to ask again: a user whose read was taken
// away keeps the socket, but not the subscription.
func TestPermissionEvent_DropsSubscriptionsThatNoLongerPassTheCheck(t *testing.T) {
	access := &mutableWorkspaceAccess{allow: map[[2]string]bool{{"tenant-a", "n-a1"}: true, {"tenant-a", "n-a2"}: true}}
	hub, url := startHub(t, &fakeAccess{})
	hub.SetWorkspaceAccess(access)
	demoted := connect(t, url, identity{"u-a1", "alice", "n-a1", "tenant-a"})
	other := connect(t, url, identity{"u-a2", "bob", "n-a2", "tenant-a"})
	joinWorkspace(t, demoted, "tenant-a")
	joinWorkspace(t, other, "tenant-a")

	access.revoke("tenant-a", "n-a1")
	hub.PublishDomain(realtime.Event{
		Domain: realtime.DomainPermission, Kind: realtime.KindChanged,
		TenantID: "tenant-a", WorkspaceID: "tenant-a", UserNodeIDs: []string{"n-a1"},
	})
	assert.NotNil(t, demoted.next(t, isDomain("permission", "changed"), 2*time.Second), "the person is told")
	time.Sleep(300 * time.Millisecond) // the re-check runs off the publisher's path

	hub.PublishDomain(driveEvent("tenant-a", "tenant-a", "f1"))
	assert.Nil(t, demoted.next(t, isDomain("drive", "updated"), quiet), "no longer allowed to follow, no longer followed")
	assert.NotNil(t, other.next(t, isDomain("drive", "updated"), 2*time.Second), "someone the event did not name is untouched")
}

func TestPermissionEvent_KeepsSubscriptionsThatStillPass(t *testing.T) {
	hub, url := startDomainHub(t, allowAll([2]string{"tenant-a", "n-a1"}))
	c := connect(t, url, identity{"u-a1", "alice", "n-a1", "tenant-a"})
	joinWorkspace(t, c, "tenant-a")

	hub.PublishDomain(realtime.Event{
		Domain: realtime.DomainPermission, Kind: realtime.KindChanged,
		TenantID: "tenant-a", WorkspaceID: "tenant-a", UserNodeIDs: []string{"n-a1"},
	})
	time.Sleep(300 * time.Millisecond)
	hub.PublishDomain(driveEvent("tenant-a", "tenant-a", "f1"))
	assert.NotNil(t, c.next(t, isDomain("drive", "updated"), 2*time.Second))
}

func connectExpiring(t *testing.T, url string, id identity, in time.Duration) *wsTestClient {
	t.Helper()
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": id.userID, "username": id.username, "ngac_node_id": id.nodeID, "tenant_id": id.tenantID,
		"exp": time.Now().Add(in).Unix(),
	}).SignedString([]byte(hubTestSecret))
	require.NoError(t, err)
	c, auth := dialAndAuth(t, url, token)
	require.True(t, auth.Ok)
	return c
}

// A session is good for as long as the token it proved itself with. The client
// reconnects with its refreshed token; one that cannot has lost its right.
func TestSession_ClosesWhenItsTokenExpires(t *testing.T) {
	hub, url := startDomainHub(t, allowAll([2]string{"tenant-a", "n-a1"}))
	c := connectExpiring(t, url, identity{"u-a1", "alice", "n-a1", "tenant-a"}, 2*time.Second)
	joinWorkspace(t, c, "tenant-a")
	hub.PublishDomain(driveEvent("tenant-a", "tenant-a", "f1"))
	require.NotNil(t, c.next(t, isDomain("drive", "updated"), time.Second), "works while the token is valid")

	closed := false
	deadline := time.After(5 * time.Second)
loop:
	for {
		select {
		case _, ok := <-c.in:
			if !ok {
				closed = true
				break loop
			}
		case <-deadline:
			break loop
		}
	}
	assert.True(t, closed, "the connection must be closed at the token's expiry")
}

func TestWorkspaceSubscribe_DenialIsAnsweredNotJustLogged(t *testing.T) {
	_, url := startDomainHub(t, allowAll())
	c := connect(t, url, identity{"u-x", "xavier", "n-x", "tenant-a"})
	c.subscribeWorkspace(t, "tenant-a")
	ack := c.next(t, isSubscribed("tenant-a"), 2*time.Second)
	require.NotNil(t, ack, "a refusal is answered so the client does not wait forever")
	assert.True(t, ack.GetWorkspaceSubscribed().Denied)
}
