//go:build realpolicy

package domain_test

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	policypb "ngac-platform/proto/policy"
	"ngac-platform/services/workspace/internal/domain"
	"ngac-platform/services/workspace/internal/store"
	"ngac-platform/testutil"
)

// Accepting an invitation writes the graph and then lists the person in the workspace
// (tenant_users), against the real policy service. The edge is the truth; the listing
// follows it. Each case has its allow on the same fixture.

type admitWorld struct {
	*wsWorld
	dir *fakeDirectory
	inv *fakeInvites
}

func newAdmitWorld(t *testing.T) *admitWorld {
	t.Helper()
	w := &admitWorld{wsWorld: newWSWorld(t), dir: &fakeDirectory{}, inv: newFakeInvites()}
	w.svc = w.svc.WithDirectory(w.dir).WithInvitations(w.inv)
	return w
}

// offerTo makes an open invitation from the owner to a new person and returns the
// person and the invitation's id. The person is under the given UAs of the graph
// (none: a stranger to the workspace).
func (w *admitWorld) offerTo(parents ...string) (wsPerson, string) {
	w.t.Helper()
	p := w.person(parents...)
	id := "adm-inv-" + p.id
	email := p.id + "@example.test"
	w.inv.emails[p.id] = email
	w.inv.byID[id] = &store.Invitation{
		ID: id, WorkspaceID: w.g.WorkspaceID, Email: email, InvitedBy: w.owner.node,
		Status: store.InvitationPending, ExpiresAt: time.Now().Add(time.Hour),
	}
	return p, id
}

func (w *admitWorld) accept(p wsPerson, invID string) error {
	_, err := w.svc.AcceptInvitation(w.as(p), p.id, p.node, invID)
	return err
}

func (w *admitWorld) underMembers(p wsPerson) bool {
	w.t.Helper()
	_, write := testutil.StartPolicy(w.t)
	r, err := write.IsAssigned(w.as(p), &policypb.IsAssignedRequest{ChildId: p.node, ParentId: w.g.Members})
	require.NoError(w.t, err)
	return r.GetValue()
}

func (w *admitWorld) listedNodes() []string { return w.dir.listed }

// A stranger is put under Members and listed; a person the graph already holds, whose
// row was never written, is listed too, which is how the table catches up.
func TestRealPolicy_AcceptListsTheNewMemberAndTheOneTheTableLost(t *testing.T) {
	w := newAdmitWorld(t)
	ws := w.g.WorkspaceID

	fresh, id := w.offerTo()
	require.NoError(t, w.accept(fresh, id))
	assert.True(t, w.underMembers(fresh), "a stranger is under Members after accepting")
	assert.Contains(t, w.listedNodes(), ws+"/"+fresh.node)

	lost, id := w.offerTo(w.g.Members) // in the graph, no row
	require.NoError(t, w.accept(lost, id))
	assert.True(t, w.underMembers(lost))
	assert.Contains(t, w.listedNodes(), ws+"/"+lost.node, "a member whose row was missing is listed again")
}

// When the listing fails the invitation is reopened. The edge this call made is taken
// back; an edge the person already had is not.
func TestRealPolicy_AcceptUndoesOnlyTheEdgeItCreated(t *testing.T) {
	w := newAdmitWorld(t)
	w.dir.ensure = errors.New("tenant_users down")

	fresh, freshInv := w.offerTo()
	require.Error(t, w.accept(fresh, freshInv))
	assert.False(t, w.underMembers(fresh), "the edge this call made is taken back")
	assert.Equal(t, []string{store.InvitationPending}, w.inv.status(w.inv.emails[fresh.id]))

	held, heldInv := w.offerTo(w.g.Members)
	require.Error(t, w.accept(held, heldInv))
	assert.True(t, w.underMembers(held), "an edge the person already had is not this call's to remove")
	assert.Equal(t, []string{store.InvitationPending}, w.inv.status(w.inv.emails[held.id]))

	// Allow, same fixture: with the directory back both are admitted.
	w.dir.ensure = nil
	require.NoError(t, w.accept(fresh, freshInv))
	require.NoError(t, w.accept(held, heldInv))
	assert.True(t, w.underMembers(fresh))
}

// A person an administrator disabled is refused: not put under Members, the offer stays
// open. The same fixture admits a removed person and a stranger.
func TestRealPolicy_AcceptRefusesADisabledMember(t *testing.T) {
	w := newAdmitWorld(t)

	disabled, disabledInv := w.offerTo()
	removed, removedInv := w.offerTo()
	w.dir.status = map[string]string{disabled.id: "disabled", removed.id: "removed"}

	assert.ErrorIs(t, w.accept(disabled, disabledInv), domain.ErrAccessDenied)
	assert.False(t, w.underMembers(disabled), "a suspended person is not put under Members")
	assert.Equal(t, []string{store.InvitationPending}, w.inv.status(w.inv.emails[disabled.id]))

	require.NoError(t, w.accept(removed, removedInv))
	assert.True(t, w.underMembers(removed))
}
