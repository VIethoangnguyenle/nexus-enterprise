package grpc_test

import (
	"context"
	"errors"
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

	"ngac-platform/ngac"
	pb "ngac-platform/proto/messaging"
	"ngac-platform/services/messaging/internal/domain"
	"ngac-platform/services/messaging/internal/events"
	grpcserver "ngac-platform/services/messaging/internal/grpc"
)

const hubTestSecret = "hub-test-secret-hub-test-secret-0123456789"

// quiet is how long a test waits to be satisfied that something did NOT
// arrive. Deliveries in these tests are in-process and land in microseconds.
const quiet = 300 * time.Millisecond

// fakeAccess stands in for the domain's channel authorizer. It grants exactly
// the (channel, user node, operation) triples it is told to and records every
// question it is asked.
type fakeAccess struct {
	mu     sync.Mutex
	allow  map[[3]string]bool
	err    error
	called [][3]string
}

func (f *fakeAccess) AuthorizeChannelAccess(_ context.Context, channelID, userNodeID, operation string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := [3]string{channelID, userNodeID, operation}
	f.called = append(f.called, key)
	if f.err != nil {
		return f.err
	}
	if f.allow[key] {
		return nil
	}
	return domain.ErrAccessDenied
}

func (f *fakeAccess) calls() [][3]string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([][3]string(nil), f.called...)
}

// wsTestClient is one authenticated WebSocket connection. A single goroutine
// owns the read side and forwards every server envelope to in.
type wsTestClient struct {
	conn *websocket.Conn
	in   chan *pb.ServerEnvelope
}

type identity struct{ userID, username, nodeID, tenantID string }

func startHub(t *testing.T, access grpcserver.ChannelAccessChecker) (*grpcserver.Hub, string) {
	t.Helper()
	hub := grpcserver.NewHub(nil, access)
	t.Cleanup(hub.Close)
	srv := httptest.NewServer(hub.HandleWebSocket(hubTestSecret))
	t.Cleanup(srv.Close)
	return hub, "ws" + strings.TrimPrefix(srv.URL, "http")
}

func connect(t *testing.T, url string, id identity) *wsTestClient {
	t.Helper()
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })

	c := &wsTestClient{conn: conn, in: make(chan *pb.ServerEnvelope, 64)}
	go func() {
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				close(c.in)
				return
			}
			var env pb.ServerEnvelope
			if proto.Unmarshal(data, &env) == nil {
				c.in <- &env
			}
		}
	}()

	claims := jwt.MapClaims{
		"user_id": id.userID, "username": id.username, "ngac_node_id": id.nodeID,
		"exp": time.Now().Add(time.Hour).Unix(),
	}
	if id.tenantID != "" {
		claims["tenant_id"] = id.tenantID
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(hubTestSecret))
	require.NoError(t, err)
	c.send(t, &pb.ClientEnvelope{Payload: &pb.ClientEnvelope_Auth{Auth: &pb.AuthRequest{Token: token}}})

	auth := c.next(t, func(e *pb.ServerEnvelope) bool { return e.GetAuthResponse() != nil }, 2*time.Second)
	require.NotNil(t, auth, "no auth response")
	require.True(t, auth.GetAuthResponse().Ok, "auth rejected: %s", auth.GetAuthResponse().Reason)
	return c
}

func (c *wsTestClient) send(t *testing.T, env *pb.ClientEnvelope) {
	t.Helper()
	data, err := proto.Marshal(env)
	require.NoError(t, err)
	require.NoError(t, c.conn.WriteMessage(websocket.BinaryMessage, data))
}

func (c *wsTestClient) subscribe(t *testing.T, channelID string) {
	t.Helper()
	c.send(t, &pb.ClientEnvelope{Payload: &pb.ClientEnvelope_Subscribe{
		Subscribe: &pb.SubscribeRequest{ChannelId: channelID},
	}})
}

// next waits for the first envelope matching match, discarding others.
func (c *wsTestClient) next(t *testing.T, match func(*pb.ServerEnvelope) bool, within time.Duration) *pb.ServerEnvelope {
	t.Helper()
	deadline := time.After(within)
	for {
		select {
		case env, ok := <-c.in:
			if !ok {
				return nil
			}
			if match(env) {
				return env
			}
		case <-deadline:
			return nil
		}
	}
}

func isChat(channelID string) func(*pb.ServerEnvelope) bool {
	return func(e *pb.ServerEnvelope) bool {
		return e.GetChatMessage() != nil && e.GetChatMessage().ChannelId == channelID
	}
}

func isError(code int32) func(*pb.ServerEnvelope) bool {
	return func(e *pb.ServerEnvelope) bool { return e.GetError() != nil && e.GetError().Code == code }
}

func isApproval(requestID string) func(*pb.ServerEnvelope) bool {
	return func(e *pb.ServerEnvelope) bool {
		return e.GetApprovalEvent() != nil && e.GetApprovalEvent().RequestId == requestID
	}
}

func isPresenceOf(userID string) func(*pb.ServerEnvelope) bool {
	return func(e *pb.ServerEnvelope) bool {
		return e.GetPresenceEvent() != nil && e.GetPresenceEvent().UserId == userID
	}
}

func secretMessage(channelID string) *pb.Message {
	return &pb.Message{Id: "m-" + channelID, ChannelId: channelID, Content: "for members only"}
}

// ---------------------------------------------------------------------------
// Subscribe — read on the channel's content OA
// ---------------------------------------------------------------------------

func TestHubSubscribe_DeniedWithoutRead(t *testing.T) {
	access := &fakeAccess{allow: map[[3]string]bool{}}
	hub, url := startHub(t, access)
	outsider := connect(t, url, identity{"u-out", "outsider", "n-out", "tenant-a"})

	outsider.subscribe(t, "ch-secret")

	require.NotNil(t, outsider.next(t, isError(403), 2*time.Second), "a denied subscribe must answer 403")
	hub.BroadcastToChannel("ch-secret", secretMessage("ch-secret"))
	assert.Nil(t, outsider.next(t, isChat("ch-secret"), quiet), "a denied client must not receive the channel's messages")
	assert.Contains(t, access.calls(), [3]string{"ch-secret", "n-out", ngac.OpRead})
}

func TestHubSubscribe_DeniedWhenCheckFails(t *testing.T) {
	access := &fakeAccess{err: errors.New("policy service unreachable")}
	hub, url := startHub(t, access)
	c := connect(t, url, identity{"u-1", "member", "n-1", "tenant-a"})

	c.subscribe(t, "ch-1")

	require.NotNil(t, c.next(t, isError(403), 2*time.Second), "a failed check must deny, not subscribe")
	hub.BroadcastToChannel("ch-1", secretMessage("ch-1"))
	assert.Nil(t, c.next(t, isChat("ch-1"), quiet))
}

func TestHubSubscribe_DeniedWithoutChecker(t *testing.T) {
	hub, url := startHub(t, nil)
	c := connect(t, url, identity{"u-1", "member", "n-1", "tenant-a"})

	c.subscribe(t, "ch-1")

	require.NotNil(t, c.next(t, isError(403), 2*time.Second), "a hub with no authorizer must deny every subscribe")
	hub.BroadcastToChannel("ch-1", secretMessage("ch-1"))
	assert.Nil(t, c.next(t, isChat("ch-1"), quiet))
}

func TestHubSubscribe_AllowedWithRead(t *testing.T) {
	access := &fakeAccess{allow: map[[3]string]bool{{"ch-1", "n-member", ngac.OpRead}: true}}
	hub, url := startHub(t, access)
	member := connect(t, url, identity{"u-member", "member", "n-member", "tenant-a"})
	outsider := connect(t, url, identity{"u-out", "outsider", "n-out", "tenant-a"})

	member.subscribe(t, "ch-1")
	outsider.subscribe(t, "ch-1")
	require.NotNil(t, outsider.next(t, isError(403), 2*time.Second))

	// A successful subscribe sends no acknowledgement, so broadcast until the
	// member's subscription has been processed and the message lands.
	var got *pb.ServerEnvelope
	for deadline := time.Now().Add(2 * time.Second); got == nil && time.Now().Before(deadline); {
		hub.BroadcastToChannel("ch-1", secretMessage("ch-1"))
		got = member.next(t, isChat("ch-1"), 50*time.Millisecond)
	}
	require.NotNil(t, got, "an authorized member must receive the channel's messages")
	assert.Nil(t, member.next(t, isError(403), quiet), "an authorized subscribe must not error")
	assert.Nil(t, outsider.next(t, isChat("ch-1"), quiet), "the outsider on the same hub must still receive nothing")
}

// ---------------------------------------------------------------------------
// Approval events — only to the people named in them, never across tenants
// ---------------------------------------------------------------------------

func TestApprovalEvent_DeliveredOnlyToNamedUsers(t *testing.T) {
	hub, url := startHub(t, &fakeAccess{})
	requester := connect(t, url, identity{"u-req", "requester", "n-req", "tenant-a"})
	bystander := connect(t, url, identity{"u-by", "bystander", "n-by", "tenant-a"})
	foreign := connect(t, url, identity{"u-b", "foreigner", "n-b", "tenant-b"})

	hub.BroadcastApprovalEvent(events.ApprovalNotice{
		RequestID: "req-1", Status: "approved", Action: "approved",
		ActorNodeID: "n-req", TemplateName: "Leave", RecipientNodeIDs: []string{"n-req"},
	})

	require.NotNil(t, requester.next(t, isApproval("req-1"), 2*time.Second), "the named user must receive the event")
	assert.Nil(t, foreign.next(t, isApproval("req-1"), quiet), "a tenant-B client must not receive a tenant-A approval event")
	assert.Nil(t, bystander.next(t, isApproval("req-1"), quiet), "an unnamed user in the same tenant must not receive it")
}

func TestApprovalEvent_TenantFilterAppliesToNamedUsers(t *testing.T) {
	hub, url := startHub(t, &fakeAccess{})
	requester := connect(t, url, identity{"u-req", "requester", "n-req", "tenant-a"})
	// Named in the event but connected under another tenant: once the event
	// carries a tenant, that session must not see it.
	otherTenantSession := connect(t, url, identity{"u-x", "approver", "n-x", "tenant-b"})

	hub.BroadcastApprovalEvent(events.ApprovalNotice{
		RequestID: "req-2", Action: "created", TenantID: "tenant-a",
		RecipientNodeIDs: []string{"n-req", "n-x"},
	})

	require.NotNil(t, requester.next(t, isApproval("req-2"), 2*time.Second))
	assert.Nil(t, otherTenantSession.next(t, isApproval("req-2"), quiet))
}

func TestApprovalEvent_NoRecipientsDeliversToNoOne(t *testing.T) {
	hub, url := startHub(t, &fakeAccess{})
	c := connect(t, url, identity{"u-1", "someone", "n-1", "tenant-a"})

	hub.BroadcastApprovalEvent(events.ApprovalNotice{RequestID: "req-3", Action: "approved"})

	assert.Nil(t, c.next(t, isApproval("req-3"), quiet))
}

// ---------------------------------------------------------------------------
// Presence — scoped to the connection's tenant
// ---------------------------------------------------------------------------

func TestPresence_ScopedToTenant(t *testing.T) {
	_, url := startHub(t, &fakeAccess{})
	foreign := connect(t, url, identity{"u-b", "foreigner", "n-b", "tenant-b"})
	noTenant := connect(t, url, identity{"u-none", "loner", "n-none", ""})
	colleague := connect(t, url, identity{"u-a1", "alice", "n-a1", "tenant-a"})

	connect(t, url, identity{"u-a2", "bob", "n-a2", "tenant-a"})

	require.NotNil(t, colleague.next(t, isPresenceOf("u-a2"), 2*time.Second), "same-tenant users must see each other come online")
	assert.Nil(t, foreign.next(t, isPresenceOf("u-a2"), quiet), "presence must not cross tenants")
	assert.Nil(t, noTenant.next(t, isPresenceOf("u-a2"), quiet), "a tenantless session belongs to no tenant's presence")
}

// With Redis, presence crosses instances on a per-tenant channel. The scoping
// must survive that hop: one instance's tenant-A announcement must not reach
// tenant-B sessions on another instance.
func TestPresence_ScopedToTenantAcrossInstancesViaRedis(t *testing.T) {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}
	rdb := redis.NewClient(&redis.Options{Addr: addr})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		t.Skipf("redis not available: %v", err)
	}
	t.Cleanup(func() { rdb.Close() })

	startInstance := func() string {
		hub := grpcserver.NewHub(rdb, &fakeAccess{})
		t.Cleanup(hub.Close)
		srv := httptest.NewServer(hub.HandleWebSocket(hubTestSecret))
		t.Cleanup(srv.Close)
		return "ws" + strings.TrimPrefix(srv.URL, "http")
	}
	urlOne, urlTwo := startInstance(), startInstance()

	run := fmt.Sprintf("%d", time.Now().UnixNano())
	tenantA, tenantB := "tenant-a-"+run, "tenant-b-"+run
	colleague := connect(t, urlTwo, identity{"u-a1-" + run, "alice", "n-a1", tenantA})
	foreign := connect(t, urlTwo, identity{"u-b-" + run, "foreigner", "n-b", tenantB})

	// Each hub pattern-subscribes in the background, so the first
	// announcements may be published before instance two listens. Keep
	// bringing tenant-A users online on instance one until one is seen.
	announced := "u-a2-" + run
	isAnnounced := func(e *pb.ServerEnvelope) bool {
		return e.GetPresenceEvent() != nil && strings.HasPrefix(e.GetPresenceEvent().UserId, announced)
	}
	var got *pb.ServerEnvelope
	for i := 0; got == nil && i < 40; i++ {
		connect(t, urlOne, identity{fmt.Sprintf("%s-%d", announced, i), "bob", "n-a2", tenantA})
		got = colleague.next(t, isAnnounced, 50*time.Millisecond)
	}

	require.NotNil(t, got, "presence must reach the same tenant on another instance")
	assert.Nil(t, foreign.next(t, isAnnounced, quiet), "presence must not reach another tenant on another instance")
}
