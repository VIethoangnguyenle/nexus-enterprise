//go:build realpolicy

package grpc_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	grpcserver "ngac-platform/services/messaging/internal/grpc"
	"ngac-platform/testutil"
)

func TestMain(m *testing.M) { os.Exit(testutil.RunMain(m)) }

// The hub's membership check against the real policy service, through the writer:
// a person detached a moment ago is refused at once, and their open sockets for
// that workspace close with the terminal code.
func TestRealPolicy_HubAuthFollowsTheGraph(t *testing.T) {
	_, write := testutil.StartPolicy(t)
	w := testutil.SeedWorkspaceGraph(t, write)
	other := testutil.SeedWorkspaceGraph(t, write)
	stays := testutil.NewUser(t, write, w.Members, other.Members)
	leaves := testutil.NewUser(t, write, w.Members, other.Members)
	owner := testutil.NewUser(t, write, w.Owners)

	hub, url := hubWith(t, grpcserver.NewPolicyMembership(write))

	// Allow: the three people of the fixture authenticate for the workspace.
	sessions := map[string]*rawSession{
		"stays":  openSession(t, url, identity{"u-stays", "stays", stays, w.WorkspaceID}),
		"leaves": openSession(t, url, identity{"u-leaves", "leaves", leaves, w.WorkspaceID}),
		"owner":  openSession(t, url, identity{"u-owner", "owner", owner, w.WorkspaceID}),
	}
	leavesElsewhere := openSession(t, url, identity{"u-leaves", "leaves", leaves, other.WorkspaceID})
	for name, s := range sessions {
		r := s.authAnswer()
		require.NotNil(t, r, name)
		assert.True(t, r.Ok, name)
	}
	require.NotNil(t, leavesElsewhere.authAnswer())

	// The person is detached from W; the removal's announcement reaches the hub.
	testutil.Unassign(t, write, leaves, w.Members)
	hub.RevokeWorkspaceSubscriptions(w.WorkspaceID, leaves)

	assert.Equal(t, grpcserver.CloseNotMember, sessions["leaves"].closeCode(t), "their socket for W closes")
	assert.True(t, sessions["stays"].staysOpen(quiet))
	assert.True(t, sessions["owner"].staysOpen(quiet))
	assert.True(t, leavesElsewhere.staysOpen(quiet), "their socket for the other workspace stays")

	// Deny: reconnecting with the token they still hold.
	again := openSession(t, url, identity{"u-leaves", "leaves", leaves, w.WorkspaceID})
	if r := again.authAnswer(); r != nil && r.Ok {
		t.Fatal("a removed member reconnected with a still-valid token")
	}
	assert.Equal(t, grpcserver.CloseNotMember, again.closeCode(t))

	// Allow: the same person, same hub, the workspace they still belong to.
	elsewhere := openSession(t, url, identity{"u-leaves", "leaves", leaves, other.WorkspaceID})
	r := elsewhere.authAnswer()
	require.NotNil(t, r)
	assert.True(t, r.Ok)

	// A UA passed as the person's node does not authenticate as a member.
	ua := openSession(t, url, identity{"u-ua", "ua", w.Members, w.WorkspaceID})
	if r := ua.authAnswer(); r != nil && r.Ok {
		t.Fatal("a UA node authenticated as a workspace member")
	}
}
