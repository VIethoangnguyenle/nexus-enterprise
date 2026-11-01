package grpc_test

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"ngac-platform/pkg/realtime"
	pb "ngac-platform/proto/messaging"
	grpcserver "ngac-platform/services/messaging/internal/grpc"
)

// startHardenedHub serves a hub with the given limits and origin policy and
// returns the hub, its ws:// URL and its http:// URL.
func startHardenedHub(t *testing.T, access *fakeAccess, limits *grpcserver.ConnLimits, origins *grpcserver.OriginPolicy) (*grpcserver.Hub, string, string) {
	t.Helper()
	hub := grpcserver.NewHub(nil, access)
	hub.SetMembership(everyoneIsAMember{})
	if limits != nil {
		hub.SetConnLimits(*limits)
	}
	if origins != nil {
		hub.SetOriginPolicy(*origins)
	}
	t.Cleanup(hub.Close)
	srv := httptest.NewServer(hub.HandleWebSocket(hubTestSecret))
	t.Cleanup(srv.Close)
	return hub, "ws" + strings.TrimPrefix(srv.URL, "http"), srv.URL
}

func authFrame(t *testing.T, id identity) []byte {
	t.Helper()
	data, err := proto.Marshal(&pb.ClientEnvelope{Payload: &pb.ClientEnvelope_Auth{Auth: &pb.AuthRequest{Token: tokenFor(t, id)}}})
	require.NoError(t, err)
	return data
}

// closeCodeOf waits for conn to end and returns the close code the server sent,
// -1 when it ended without one.
func closeCodeOf(t *testing.T, conn *websocket.Conn) int {
	t.Helper()
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(3*time.Second)))
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			var ce *websocket.CloseError
			if errors.As(err, &ce) {
				return ce.Code
			}
			var ne interface{ Timeout() bool }
			if errors.As(err, &ne) && ne.Timeout() {
				t.Fatal("the server did not close the connection")
			}
			return -1
		}
	}
}

// drain discards what is already queued for c (the broadcasts subscribeAndWait
// used to see a subscription go live), so a later assertion sees only new frames.
func drain(c *wsTestClient) {
	for {
		select {
		case _, ok := <-c.in:
			if !ok {
				return
			}
		case <-time.After(100 * time.Millisecond):
			return
		}
	}
}

// ---------------------------------------------------------------------------
// One identity per socket: a second Auth frame ends the connection
// ---------------------------------------------------------------------------

func TestHubAuth_ASecondAuthFrameClosesTheSocket(t *testing.T) {
	_, url := hubWith(t, everyoneIsAMember{})
	s := openSession(t, url, identity{"u-1", "one", "n-1", "tenant-a"})
	require.True(t, s.authAnswer().GetOk())

	require.NoError(t, s.conn.WriteMessage(websocket.BinaryMessage, authFrame(t, identity{"u-1", "one", "n-1", "tenant-b"})))

	assert.Equal(t, websocket.ClosePolicyViolation, s.closeCode(t))
}

// Deny side of the same boundary: a socket that authenticated once keeps
// working, so the refusal above is about the second frame and nothing else.
func TestHubAuth_ASingleAuthFrameKeepsTheSocketOpen(t *testing.T) {
	_, url := hubWith(t, everyoneIsAMember{})
	s := openSession(t, url, identity{"u-1", "one", "n-1", "tenant-a"})
	require.True(t, s.authAnswer().GetOk())
	assert.True(t, s.staysOpen(quiet))
}

// The attack: subscribe to tenant A's channel, re-authenticate the socket as
// tenant B, get removed from A. The re-auth closes the socket, so nothing of A
// reaches it afterwards.
func TestHubRevoke_ARemovedMemberStopsReceivingAfterAReAuthAttempt(t *testing.T) {
	access := &fakeAccess{
		allow:     map[[3]string]bool{{"ch-a", "n-1", "read"}: true, {"ch-a", "n-2", "read"}: true},
		workspace: map[string]string{"ch-a": "tenant-a"},
	}
	hub, url := startHub(t, access)
	victim := connect(t, url, identity{"u-1", "one", "n-1", "tenant-a"})
	colleague := connect(t, url, identity{"u-2", "two", "n-2", "tenant-a"})
	subscribeAndWait(t, hub, victim, "ch-a")
	subscribeAndWait(t, hub, colleague, "ch-a")
	drain(victim)
	drain(colleague)

	victim.send(t, &pb.ClientEnvelope{Payload: &pb.ClientEnvelope_Auth{Auth: &pb.AuthRequest{Token: tokenFor(t, identity{"u-1", "one", "n-1", "tenant-b"})}}})
	// Whatever the hub makes of the frame, it has made it before the removal.
	victim.next(t, func(e *pb.ServerEnvelope) bool { return e.GetAuthResponse() != nil || e.GetError() != nil }, 2*time.Second)
	hub.RevokeWorkspaceSubscriptions("tenant-a", "n-1")
	hub.BroadcastToChannel("ch-a", secretMessage("ch-a"))

	assert.Nil(t, victim.next(t, isChat("ch-a"), quiet), "the removed member receives nothing of tenant A")
	assert.NotNil(t, colleague.next(t, isChat("ch-a"), 2*time.Second), "a colleague who stays keeps receiving")
}

// The sweep goes by the channel's workspace, not by the tenant of the socket:
// a socket of tenant B that follows a channel of workspace A loses it when the
// person is removed from A, while the socket itself (B) stays.
func TestHubRevoke_DropsChannelsOfTheWorkspaceHeldOnAnotherTenantsSocket(t *testing.T) {
	access := &fakeAccess{
		allow: map[[3]string]bool{
			{"ch-a", "n-1", "read"}: true, {"ch-b", "n-1", "read"}: true, {"ch-a", "n-2", "read"}: true,
		},
		workspace: map[string]string{"ch-a": "tenant-a", "ch-b": "tenant-b"},
	}
	hub, url := startHub(t, access)
	inB := connect(t, url, identity{"u-1", "one", "n-1", "tenant-b"})
	colleague := connect(t, url, identity{"u-2", "two", "n-2", "tenant-a"})
	subscribeAndWait(t, hub, inB, "ch-a")
	subscribeAndWait(t, hub, inB, "ch-b")
	subscribeAndWait(t, hub, colleague, "ch-a")
	drain(inB)
	drain(colleague)

	hub.RevokeWorkspaceSubscriptions("tenant-a", "n-1")
	hub.BroadcastToChannel("ch-b", secretMessage("ch-b"))
	assert.NotNil(t, inB.next(t, isChat("ch-b"), 2*time.Second), "workspace B's channel stays")
	hub.BroadcastToChannel("ch-a", secretMessage("ch-a"))
	assert.Nil(t, inB.next(t, isChat("ch-a"), quiet), "workspace A's channel is gone from the B socket")
	assert.NotNil(t, colleague.next(t, isChat("ch-a"), 2*time.Second), "a colleague who stays keeps receiving")
}

// A permission event re-checks the person's live channel subscriptions, not only
// the workspace stream: read taken away ends the feed, read kept keeps it.
func TestPermissionEvent_DropsChannelSubscriptionsThatNoLongerPass(t *testing.T) {
	access := &fakeAccess{
		allow:     map[[3]string]bool{{"ch-lost", "n-1", "read"}: true, {"ch-kept", "n-1", "read"}: true},
		workspace: map[string]string{"ch-lost": "tenant-a", "ch-kept": "tenant-a"},
	}
	hub, url := startHub(t, access)
	c := connect(t, url, identity{"u-1", "one", "n-1", "tenant-a"})
	subscribeAndWait(t, hub, c, "ch-lost")
	subscribeAndWait(t, hub, c, "ch-kept")
	drain(c)

	access.revoke("ch-lost", "n-1")
	hub.PublishDomain(realtime.Event{
		Domain: realtime.DomainPermission, Kind: realtime.KindChanged,
		TenantID: "tenant-a", WorkspaceID: "tenant-a", UserNodeIDs: []string{"n-1"},
	})
	require.NotNil(t, c.next(t, isDomain("permission", "changed"), 2*time.Second))
	time.Sleep(300 * time.Millisecond) // the re-check runs off the publisher's path

	hub.BroadcastToChannel("ch-kept", secretMessage("ch-kept"))
	assert.NotNil(t, c.next(t, isChat("ch-kept"), 2*time.Second), "read kept, feed kept")
	hub.BroadcastToChannel("ch-lost", secretMessage("ch-lost"))
	assert.Nil(t, c.next(t, isChat("ch-lost"), quiet), "read taken away, feed ended")
}

// ---------------------------------------------------------------------------
// Frame size, keepalive, per-address caps, frame rate
// ---------------------------------------------------------------------------

func TestHubConn_AnOversizedFrameClosesTheConnection(t *testing.T) {
	limits := grpcserver.DefaultConnLimits()
	_, url, _ := startHardenedHub(t, &fakeAccess{}, &limits, nil)

	// Allow: a frame within the limit is read (and answered as bad protobuf).
	ok, _, err := websocket.DefaultDialer.Dial(url, nil)
	require.NoError(t, err)
	t.Cleanup(func() { ok.Close() })
	require.NoError(t, ok.WriteMessage(websocket.BinaryMessage, bytes.Repeat([]byte{0xff}, int(limits.MaxFrameBytes)-16)))
	require.NoError(t, ok.SetReadDeadline(time.Now().Add(2*time.Second)))
	_, data, err := ok.ReadMessage()
	require.NoError(t, err, "a frame within the limit does not end the connection")
	var env pb.ServerEnvelope
	require.NoError(t, proto.Unmarshal(data, &env))
	assert.EqualValues(t, 400, env.GetError().GetCode())

	// Deny: a frame over the limit ends it, before authentication.
	big, _, err := websocket.DefaultDialer.Dial(url, nil)
	require.NoError(t, err)
	t.Cleanup(func() { big.Close() })
	require.NoError(t, big.WriteMessage(websocket.BinaryMessage, make([]byte, limits.MaxFrameBytes+1)))
	assert.Equal(t, websocket.CloseMessageTooBig, closeCodeOf(t, big))
}

func TestHubConn_APeerThatStopsAnsweringPingsIsDropped(t *testing.T) {
	limits := grpcserver.DefaultConnLimits()
	limits.PongWait = 300 * time.Millisecond
	_, url, _ := startHardenedHub(t, &fakeAccess{}, &limits, nil)

	// Deny: pings ignored, so the read deadline runs out.
	dead, _, err := websocket.DefaultDialer.Dial(url, nil)
	require.NoError(t, err)
	t.Cleanup(func() { dead.Close() })
	dead.SetPingHandler(func(string) error { return nil })
	require.NoError(t, dead.WriteMessage(websocket.BinaryMessage, authFrame(t, identity{"u-1", "one", "n-1", ""})))
	ended := make(chan struct{})
	go func() {
		defer close(ended)
		for {
			if _, _, err := dead.ReadMessage(); err != nil {
				return
			}
		}
	}()

	// Allow: the default ping handler answers, so the connection lives on.
	alive, _, err := websocket.DefaultDialer.Dial(url, nil)
	require.NoError(t, err)
	t.Cleanup(func() { alive.Close() })
	require.NoError(t, alive.WriteMessage(websocket.BinaryMessage, authFrame(t, identity{"u-2", "two", "n-2", ""})))
	var aliveEnded atomic.Bool
	go func() {
		for {
			if _, _, err := alive.ReadMessage(); err != nil {
				aliveEnded.Store(true)
				return
			}
		}
	}()

	select {
	case <-ended:
	case <-time.After(3 * time.Second):
		t.Fatal("a peer that never answers pings was kept")
	}
	time.Sleep(4 * limits.PongWait)
	assert.False(t, aliveEnded.Load(), "a peer that answers pings is kept")
}

func TestHubConn_CapsUnauthenticatedConnectionsPerAddress(t *testing.T) {
	limits := grpcserver.DefaultConnLimits()
	limits.PreAuthPerIP = 2
	_, url, _ := startHardenedHub(t, &fakeAccess{}, &limits, nil)

	dial := func() (*websocket.Conn, *http.Response, error) {
		conn, resp, err := websocket.DefaultDialer.Dial(url, nil)
		if conn != nil {
			t.Cleanup(func() { conn.Close() })
		}
		return conn, resp, err
	}
	first, _, err := dial()
	require.NoError(t, err)
	_, _, err = dial()
	require.NoError(t, err)

	_, resp, err := dial()
	require.Error(t, err, "a third unauthenticated socket from the same address is refused")
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode)

	// Authenticating frees the slot.
	require.NoError(t, first.WriteMessage(websocket.BinaryMessage, authFrame(t, identity{"u-1", "one", "n-1", ""})))
	require.NoError(t, first.SetReadDeadline(time.Now().Add(2*time.Second)))
	_, _, err = first.ReadMessage()
	require.NoError(t, err)
	_, _, err = dial()
	assert.NoError(t, err, "a slot freed by authentication is available again")
}

func TestHubConn_CapsAllConnectionsPerAddress(t *testing.T) {
	limits := grpcserver.DefaultConnLimits()
	limits.PerIP = 1
	_, url, _ := startHardenedHub(t, &fakeAccess{}, &limits, nil)

	first, _, err := websocket.DefaultDialer.Dial(url, nil)
	require.NoError(t, err)
	_, resp, err := websocket.DefaultDialer.Dial(url, nil)
	require.Error(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode)

	first.Close()
	require.Eventually(t, func() bool {
		c, _, err := websocket.DefaultDialer.Dial(url, nil)
		if err != nil {
			return false
		}
		c.Close()
		return true
	}, 2*time.Second, 20*time.Millisecond, "a closed connection frees its slot")
}

// A user's frames are throttled: a typing flood reaches colleagues at the
// configured rate, not at the rate it is sent.
func TestHubConn_AUsersFramesAreRateLimited(t *testing.T) {
	limits := grpcserver.DefaultConnLimits()
	limits.FramesPerSecond = 2
	limits.FrameBurst = 3
	access := &fakeAccess{allow: map[[3]string]bool{{"ch-1", "n-1", "read"}: true, {"ch-1", "n-2", "read"}: true}}
	hub, url, _ := startHardenedHub(t, access, &limits, nil)
	flooder := connect(t, url, identity{"u-1", "one", "n-1", ""})
	peer := connect(t, url, identity{"u-2", "two", "n-2", ""})
	subscribeAndWait(t, hub, flooder, "ch-1")
	subscribeAndWait(t, hub, peer, "ch-1")
	time.Sleep(time.Second) // let the bucket refill after the subscribes

	for i := 0; i < 20; i++ {
		flooder.typing(t, "ch-1")
	}
	got := 0
	for peer.next(t, isTyping("ch-1"), 400*time.Millisecond) != nil {
		got++
		if got > 10 {
			break
		}
	}
	assert.GreaterOrEqual(t, got, 1, "frames within the budget are delivered")
	assert.LessOrEqual(t, got, limits.FrameBurst+3, "the flood is throttled")
}

// ---------------------------------------------------------------------------
// Origin
// ---------------------------------------------------------------------------

func TestHubConn_OriginPolicy(t *testing.T) {
	policy, err := grpcserver.NewOriginPolicy("https://nexus.example.com", false)
	require.NoError(t, err)
	_, url, httpURL := startHardenedHub(t, &fakeAccess{}, nil, &policy)

	dial := func(origin string) (int, error) {
		h := http.Header{}
		if origin != "" {
			h.Set("Origin", origin)
		}
		conn, resp, err := websocket.DefaultDialer.Dial(url, h)
		if conn != nil {
			conn.Close()
		}
		if resp != nil {
			return resp.StatusCode, err
		}
		return 0, err
	}

	for _, origin := range []string{"https://evil.example", "http://nexus.example.com", "https://nexus.example.com.evil.example", "null"} {
		code, err := dial(origin)
		assert.Error(t, err, "origin %s must be refused", origin)
		assert.Equal(t, http.StatusForbidden, code, "origin %s", origin)
	}
	// Deny in production: a dev server on loopback is not the app.
	code, err := dial("http://localhost:5173")
	assert.Error(t, err)
	assert.Equal(t, http.StatusForbidden, code)

	for _, origin := range []string{"https://nexus.example.com", httpURL, ""} {
		code, err := dial(origin)
		assert.NoError(t, err, "origin %q must be admitted", origin)
		assert.Equal(t, http.StatusSwitchingProtocols, code)
	}
}

func TestHubConn_DevAdmitsLoopbackOrigins(t *testing.T) {
	policy, err := grpcserver.NewOriginPolicy("http://localhost:5173", true)
	require.NoError(t, err)
	_, url, _ := startHardenedHub(t, &fakeAccess{}, nil, &policy)

	for origin, admitted := range map[string]bool{
		"http://localhost:5173": true, "http://127.0.0.1:5174": true, "http://[::1]:3000": true,
		"https://evil.example": false, "http://localhost.evil.example": false,
	} {
		h := http.Header{"Origin": []string{origin}}
		conn, _, err := websocket.DefaultDialer.Dial(url, h)
		if conn != nil {
			conn.Close()
		}
		assert.Equal(t, admitted, err == nil, "origin %s", origin)
	}
}

func TestNewOriginPolicy_RejectsAnAppURLThatIsNotAnOrigin(t *testing.T) {
	for _, raw := range []string{"nexus.example.com", "ftp://nexus.example.com", "https://"} {
		_, err := grpcserver.NewOriginPolicy(raw, false)
		assert.Error(t, err, raw)
	}
	_, err := grpcserver.NewOriginPolicy("", false)
	assert.NoError(t, err, "no app URL: same-origin only")
}
