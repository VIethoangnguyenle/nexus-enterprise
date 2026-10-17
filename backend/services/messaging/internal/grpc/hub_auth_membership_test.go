package grpc_test

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	pb "ngac-platform/proto/messaging"
	grpcserver "ngac-platform/services/messaging/internal/grpc"
)

// everyoneIsAMember is the membership check for tests that are about something
// else: it lets every session in.
type everyoneIsAMember struct{}

func (everyoneIsAMember) IsWorkspaceMember(context.Context, string, string) (bool, error) {
	return true, nil
}

// fakeMembers answers by "node|workspace" and remembers what it was asked.
type fakeMembers struct {
	mu     sync.Mutex
	member map[string]bool
	err    error
	asked  []string
}

func (f *fakeMembers) IsWorkspaceMember(_ context.Context, node, ws string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.asked = append(f.asked, node+"|"+ws)
	if f.err != nil {
		return false, f.err
	}
	return f.member[node+"|"+ws], nil
}

func (f *fakeMembers) questions() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.asked...)
}

// rawSession is a WebSocket connection that keeps how it was closed: the tests below
// are about the close code, which the shared helper discards.
type rawSession struct {
	conn   *websocket.Conn
	in     chan *pb.ServerEnvelope
	closed chan struct{} // closed once the connection has ended; code is then set
	code   int
}

func tokenFor(t *testing.T, id identity) string {
	t.Helper()
	claims := jwt.MapClaims{
		"user_id": id.userID, "username": id.username, "ngac_node_id": id.nodeID,
		"exp": time.Now().Add(time.Hour).Unix(),
	}
	if id.tenantID != "" {
		claims["tenant_id"] = id.tenantID
	}
	tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(hubTestSecret))
	require.NoError(t, err)
	return tok
}

func openSession(t *testing.T, url string, id identity) *rawSession {
	t.Helper()
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })
	s := &rawSession{conn: conn, in: make(chan *pb.ServerEnvelope, 64), closed: make(chan struct{})}
	go func() {
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				code := -1
				var ce *websocket.CloseError
				if errors.As(err, &ce) {
					code = ce.Code
				}
				s.code = code
				close(s.closed)
				return
			}
			var env pb.ServerEnvelope
			if proto.Unmarshal(data, &env) == nil {
				s.in <- &env
			}
		}
	}()
	data, err := proto.Marshal(&pb.ClientEnvelope{Payload: &pb.ClientEnvelope_Auth{Auth: &pb.AuthRequest{Token: tokenFor(t, id)}}})
	require.NoError(t, err)
	require.NoError(t, conn.WriteMessage(websocket.BinaryMessage, data))
	return s
}

// authAnswer waits for the server's answer to the auth message, nil if it closed first.
func (s *rawSession) authAnswer() *pb.AuthResponse {
	timeout := time.After(3 * time.Second)
	for {
		select {
		case env := <-s.in:
			if r := env.GetAuthResponse(); r != nil {
				return r
			}
		case <-s.closed:
			return nil
		case <-timeout:
			return nil
		}
	}
}

// closeCode waits for the server to close the connection and returns its code.
func (s *rawSession) closeCode(t *testing.T) int {
	t.Helper()
	select {
	case <-s.closed:
		return s.code
	case <-time.After(3 * time.Second):
		t.Fatal("the server did not close the connection")
		return 0
	}
}

func (s *rawSession) staysOpen(within time.Duration) bool {
	select {
	case <-s.closed:
		return false
	case <-time.After(within):
		return true
	}
}

func hubWith(t *testing.T, m grpcserver.MembershipChecker) (*grpcserver.Hub, string) {
	t.Helper()
	hub := grpcserver.NewHub(nil, &fakeAccess{})
	if m != nil {
		hub.SetMembership(m)
	}
	t.Cleanup(hub.Close)
	srv := httptest.NewServer(hub.HandleWebSocket(hubTestSecret))
	t.Cleanup(srv.Close)
	return hub, "ws" + strings.TrimPrefix(srv.URL, "http")
}

// A valid token for a tenant is not enough: the person must still belong to it.
// A removed person reconnecting with the token they still hold is turned away with
// a code the client reads as "stop", and is never registered with the hub.
func TestHubAuth_ARemovedMemberIsRefusedWith4403(t *testing.T) {
	members := &fakeMembers{member: map[string]bool{"n-stays|tenant-a": true}}
	_, url := hubWith(t, members)

	// Allow: a member of the tenant authenticates and stays connected.
	ok := openSession(t, url, identity{"u-stays", "stays", "n-stays", "tenant-a"})
	if r := ok.authAnswer(); r == nil || !r.Ok {
		t.Fatalf("a member was refused: %v", r)
	}
	assert.True(t, ok.staysOpen(quiet), "a member's connection stays open")

	// Deny: same tenant, same token shape, not a member.
	gone := openSession(t, url, identity{"u-gone", "gone", "n-gone", "tenant-a"})
	if r := gone.authAnswer(); r != nil && r.Ok {
		t.Fatal("a removed member authenticated")
	}
	assert.Equal(t, 4403, gone.closeCode(t))
	assert.Contains(t, members.questions(), "n-gone|tenant-a", "the question was about this person and this tenant")
}

// A hub that cannot tell whether the person belongs refuses them: not as a ban
// (the client may try again), but never as a yes.
func TestHubAuth_AnUnansweredMembershipQuestionIsNotAnAllow(t *testing.T) {
	_, url := hubWith(t, &fakeMembers{err: errors.New("policy: connection refused")})

	s := openSession(t, url, identity{"u-1", "one", "n-1", "tenant-a"})

	if r := s.authAnswer(); r != nil && r.Ok {
		t.Fatal("authenticated though the policy could not be asked")
	}
	assert.Equal(t, websocket.CloseTryAgainLater, s.closeCode(t), "an outage is a try-again, not the terminal code")
}

// A workspace that no longer exists (or has no Members UA) holds nobody: that is an answer,
// the terminal one, not an outage to retry for ever.
func TestHubAuth_ADeletedWorkspaceIsNotAMemberNotAnOutage(t *testing.T) {
	_, url := hubWith(t, &fakeMembers{err: status.Error(codes.NotFound, "no such node")})

	s := openSession(t, url, identity{"u-1", "one", "n-1", "tenant-gone"})

	if r := s.authAnswer(); r != nil && r.Ok {
		t.Fatal("authenticated for a workspace that does not exist")
	}
	assert.Equal(t, 4403, s.closeCode(t))
}

func TestHubAuth_WithNoMembershipCheckerATenantSessionIsRefused(t *testing.T) {
	_, url := hubWith(t, nil)

	s := openSession(t, url, identity{"u-1", "one", "n-1", "tenant-a"})

	if r := s.authAnswer(); r != nil && r.Ok {
		t.Fatal("a hub with no membership check let a tenant session in")
	}
	assert.NotEqual(t, 0, s.closeCode(t))
}

// A session of no tenant has nothing to be a member of.
func TestHubAuth_ATenantlessSessionIsNotAskedAboutMembership(t *testing.T) {
	members := &fakeMembers{}
	_, url := hubWith(t, members)

	s := openSession(t, url, identity{"u-1", "one", "n-1", ""})

	if r := s.authAnswer(); r == nil || !r.Ok {
		t.Fatalf("a tenant-less session was refused: %v", r)
	}
	assert.Empty(t, members.questions())
}

// Detaching a person closes their sockets of that workspace, with the terminal
// code; their socket of another tenant stays.
func TestHubRevoke_ClosesTheRemovedPersonsSocketsOfThatWorkspaceOnly(t *testing.T) {
	members := &fakeMembers{member: map[string]bool{"n-1|tenant-a": true, "n-1|tenant-b": true, "n-2|tenant-a": true}}
	hub, url := hubWith(t, members)

	inA := openSession(t, url, identity{"u-1", "one", "n-1", "tenant-a"})
	inA2 := openSession(t, url, identity{"u-1", "one", "n-1", "tenant-a"})
	inB := openSession(t, url, identity{"u-1", "one", "n-1", "tenant-b"})
	other := openSession(t, url, identity{"u-2", "two", "n-2", "tenant-a"})
	for _, s := range []*rawSession{inA, inA2, inB, other} {
		require.NotNil(t, s.authAnswer())
	}

	hub.RevokeWorkspaceSubscriptions("tenant-a", "n-1")

	assert.Equal(t, 4403, inA.closeCode(t))
	assert.Equal(t, 4403, inA2.closeCode(t))
	assert.True(t, inB.staysOpen(quiet), "the same person's socket for another tenant stays")
	assert.True(t, other.staysOpen(quiet), "another person's socket in the same tenant stays")
}
