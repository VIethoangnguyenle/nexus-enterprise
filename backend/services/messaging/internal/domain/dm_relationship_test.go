package domain_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/ngac"
	"ngac-platform/services/messaging/internal/domain"
)

// A direct message is written to only while its two people share a workspace, and
// the question is asked of the policy each time: when it cannot be answered, or
// the conversation records nobody else, the answer is no.
func TestDMWrite_FailsClosedWhenTheRelationshipIsUnknown(t *testing.T) {
	send := func(f *fixture, chID string) error {
		_, err := f.svc.SendMessage(f.callerCtx(), domain.SendMessageInput{
			ChannelID: chID, SenderID: f.userID, SenderNodeID: userNode, Content: "hello",
		})
		return err
	}

	t.Run("the policy cannot say who belongs where", func(t *testing.T) {
		f := newFixture(t)
		chID, oaID := f.insertChannel(t, "dm-unknown")
		f.read.allow[grant{userNode, oaID, ngac.OpWrite}] = true
		f.withDMPeer(t, chID)
		require.NoError(t, send(f, chID), "allowed while the policy answers")

		f.read.ancErr = errors.New("policy down")
		assert.ErrorIs(t, send(f, chID), domain.ErrAccessDenied)
	})

	t.Run("the conversation records nobody else", func(t *testing.T) {
		f := newFixture(t)
		chID, oaID := f.insertChannel(t, "dm-alone")
		f.read.allow[grant{userNode, oaID, ngac.OpWrite}] = true
		peer := f.withDMPeer(t, chID)
		require.NoError(t, send(f, chID))

		_, err := f.pool.Exec(t.Context(), `DELETE FROM channel_members WHERE channel_id = $1 AND ngac_node_id = $2`, chID, peer)
		require.NoError(t, err)
		assert.ErrorIs(t, send(f, chID), domain.ErrAccessDenied)
	})

	t.Run("a group is not held to it", func(t *testing.T) {
		f := newFixture(t)
		chID, oaID := f.insertChannel(t, "group-not-dm")
		f.read.allow[grant{userNode, oaID, ngac.OpWrite}] = true
		f.read.ancErr = errors.New("policy down")
		require.NoError(t, send(f, chID), "membership of the workspace is not asked of a group's write")
	})
}

// The same for an event's call: a policy that cannot say whether the person
// belongs to the event's workspace lets nobody into it.
func TestEventCall_FailsClosedWhenMembershipIsUnknown(t *testing.T) {
	w := newEventWorld(t)
	_, _, err := w.start(w.guest, w.guestN)
	require.NoError(t, err, "allowed while the policy answers")

	w.f.read.ancErr = errors.New("policy down")
	_, _, err = w.start(w.guest2, w.guest2N)
	assert.ErrorIs(t, err, domain.ErrAccessDenied)
}
