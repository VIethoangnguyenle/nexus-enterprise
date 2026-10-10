package domain_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"ngac-platform/ngac"
	policypb "ngac-platform/proto/policy"
	"ngac-platform/services/auth/internal/auth"
	"ngac-platform/services/auth/internal/domain"
	"ngac-platform/testutil"
)

// scriptedRead answers FindNodeByName from a fixed set and reports NotFound for
// everything else, as the real policy service does.
type scriptedRead struct {
	policypb.PolicyReadServiceClient
	known map[string]*policypb.NGACNode // "<type>|<name>"
	err   error
}

func (r *scriptedRead) FindNodeByName(_ context.Context, req *policypb.FindNodeByNameRequest, _ ...grpc.CallOption) (*policypb.NGACNode, error) {
	if r.err != nil {
		return nil, r.err
	}
	if n, ok := r.known[req.NodeType+"|"+req.Name]; ok {
		return n, nil
	}
	return nil, status.Error(codes.NotFound, "node not found")
}

func publicUsersOnly() *scriptedRead {
	return &scriptedRead{known: map[string]*policypb.NGACNode{
		ngac.TypeUA + "|" + ngac.NodePublicUsers: {Id: "ua-public", Name: ngac.NodePublicUsers, NodeType: ngac.TypeUA},
	}}
}

type failingCreateUser struct {
	*fakeWorld
	err error
}

func (f *failingCreateUser) CreateUser(context.Context, string, string, string, string, string, string, string, string) error {
	return f.err
}

func (w *fakeWorld) serviceOn(t *testing.T, st domain.AuthStore, read *scriptedRead, write *testutil.FakePolicyWrite) *domain.Service {
	t.Helper()
	auth.SetJWTSecret("test-secret-key-for-testing-only")
	return domain.NewService(st, nil, read, write, &fakeWorkspace{w: w}, &fakeMessaging{w: w})
}

func hasPrefix(live []string, prefix string) bool {
	for _, l := range live {
		if strings.HasPrefix(l, prefix) {
			return true
		}
	}
	return false
}

// The user's node is named by users.id and carries the username as its display
// name. A node named by username is raw input in the graph's name index.
func TestSignup_UserNodeIsKeyedByUserID(t *testing.T) {
	w := newFakeWorld()
	write := testutil.NewFakePolicyWrite()
	svc := w.serviceOn(t, w, publicUsersOnly(), write)

	res, err := svc.Signup(context.Background(), "alice@example.com", "pw-123456", "Alice", "")

	require.NoError(t, err)
	assert.Contains(t, write.LiveNodes(), "U "+ngac.UserNodeName(ngac.UserID(res.UserID)))
	assert.NotContains(t, write.LiveNodes(), "U "+res.Username, "no node is named by the username")
	n, ok := write.Node(res.NGACNodeID)
	require.True(t, ok)
	assert.Equal(t, res.Username, n.Properties[ngac.PropDisplayName])
	assert.Equal(t, res.UserID, n.Properties["user_id"])
}

// Two accounts whose usernames normalise to the same string get distinct nodes
// (the username is unique in the table, but nothing in the graph depends on it).
func TestSignup_UserNodeNamesDifferPerUser(t *testing.T) {
	w := newFakeWorld()
	write := testutil.NewFakePolicyWrite()
	svc := w.serviceOn(t, w, publicUsersOnly(), write)

	a, err := svc.Signup(context.Background(), "bob@one.com", "pw-123456", "Bob", "")
	require.NoError(t, err)
	b, err := svc.Signup(context.Background(), "bob@two.com", "pw-123456", "Bob", "")
	require.NoError(t, err)

	assert.NotEqual(t, ngac.UserNodeName(ngac.UserID(a.UserID)), ngac.UserNodeName(ngac.UserID(b.UserID)))
	assert.Contains(t, write.LiveNodes(), "U "+ngac.UserNodeName(ngac.UserID(a.UserID)))
	assert.Contains(t, write.LiveNodes(), "U "+ngac.UserNodeName(ngac.UserID(b.UserID)))
}

func TestSignup_RemovesTheUserNodeWhenTheRowCannotBeWritten(t *testing.T) {
	w := newFakeWorld()
	write := testutil.NewFakePolicyWrite()
	svc := w.serviceOn(t, &failingCreateUser{fakeWorld: w, err: errors.New("db down")}, publicUsersOnly(), write)

	_, err := svc.Signup(context.Background(), "carol@example.com", "pw-123456", "Carol", "")

	require.ErrorContains(t, err, "db down")
	assert.Empty(t, write.LiveNodes(), "a failed signup leaves no user node behind")
	log := write.Log()
	assert.True(t, strings.HasPrefix(log[len(log)-1], "delete U_"), "the node is removed again: %v", log)
}

func TestSignup_RemovesTheUserNodeWhenItCannotBeAssigned(t *testing.T) {
	w := newFakeWorld()
	write := testutil.NewFakePolicyWrite()
	write.FailAt = 2 // the assignment under PublicUsers
	svc := w.serviceOn(t, w, publicUsersOnly(), write)

	_, err := svc.Signup(context.Background(), "dave@example.com", "pw-123456", "Dave", "")

	require.Error(t, err)
	assert.Empty(t, write.LiveNodes())
	assert.Zero(t, w.userCount(), "and no users row was written")
}

func initTenant(svc *domain.Service) error {
	return svc.InitTenantNGAC(context.Background(), "ws-t1", "pc-t1", "owners-t1", "members-t1")
}

func TestInitTenantNGAC_CreatesTheTenantUAsKeyedByTenantID(t *testing.T) {
	w := newFakeWorld()
	write := testutil.NewFakePolicyWrite()
	svc := w.serviceOn(t, w, &scriptedRead{}, write)

	require.NoError(t, initTenant(svc))

	assert.ElementsMatch(t, []string{
		"UA " + ngac.TenantMemberUAName("ws-t1"), "UA " + ngac.TenantOwnerUAName("ws-t1"),
	}, write.LiveNodes())
}

// Run it again, as after a partial failure: what exists is reused, nothing is
// duplicated, and the edges are made again.
func TestInitTenantNGAC_IsIdempotent(t *testing.T) {
	w := newFakeWorld()
	write := testutil.NewFakePolicyWrite()
	read := &scriptedRead{known: map[string]*policypb.NGACNode{
		ngac.TypeUA + "|" + ngac.TenantMemberUAName("ws-t1"): {Id: "ua-m", Name: ngac.TenantMemberUAName("ws-t1"), NodeType: ngac.TypeUA},
		ngac.TypeUA + "|" + ngac.TenantOwnerUAName("ws-t1"):  {Id: "ua-o", Name: ngac.TenantOwnerUAName("ws-t1"), NodeType: ngac.TypeUA},
	}}
	svc := w.serviceOn(t, w, read, write)

	require.NoError(t, initTenant(svc))

	assert.Empty(t, write.LiveNodes(), "existing UAs are reused, not created again")
	assert.Equal(t, 4, write.Calls(), "the four assignments are still made")
}

// The UAs are created and then attached with four assignments. A failure at any
// of the six writes leaves neither UA behind.
func TestInitTenantNGAC_RollsBackWhereverItFails(t *testing.T) {
	for k := 1; k <= 6; k++ {
		w := newFakeWorld()
		write := testutil.NewFakePolicyWrite()
		write.FailAt = k
		svc := w.serviceOn(t, w, &scriptedRead{}, write)

		err := initTenant(svc)

		require.Errorf(t, err, "failure injected at write %d must surface", k)
		assert.Emptyf(t, write.LiveNodes(), "failure at write %d left nodes behind", k)
	}
}

// Deny: a lookup that fails for a reason other than "not found" must not be read
// as "absent" and answered with a second copy of the UA.
func TestInitTenantNGAC_DoesNotCreateWhenTheLookupFails(t *testing.T) {
	w := newFakeWorld()
	write := testutil.NewFakePolicyWrite()
	svc := w.serviceOn(t, w, &scriptedRead{err: status.Error(codes.Unavailable, "policy down")}, write)

	require.Error(t, initTenant(svc))
	assert.Zero(t, write.Calls())
}

// An incomplete tenant init is logged and tolerated, as it always was: the
// owner is in the workspace. What changed is that it cleans up after itself.
func TestSignup_ToleratesATenantInitFailureAndLeavesNoHalfBuiltUAs(t *testing.T) {
	w := newFakeWorld()
	write := testutil.NewFakePolicyWrite()
	// 1 user node, 2 public assignment, 3 TenantMember, 4 TenantOwner, 5 first tenant assignment.
	write.FailAt = 5
	svc := w.serviceOn(t, w, publicUsersOnly(), write)

	res, err := svc.Signup(context.Background(), "erin@example.com", "pw-123456", "Erin", "")

	require.NoError(t, err)
	assert.False(t, hasPrefix(write.LiveNodes(), "UA TenantMember_"), "live: %v", write.LiveNodes())
	assert.False(t, hasPrefix(write.LiveNodes(), "UA TenantOwner_"), "live: %v", write.LiveNodes())
	assert.Contains(t, write.LiveNodes(), fmt.Sprintf("U %s", ngac.UserNodeName(ngac.UserID(res.UserID))), "the user's own node stays")
}
