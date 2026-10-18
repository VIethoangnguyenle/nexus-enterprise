package domain_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/ngac"
	"ngac-platform/services/messaging/internal/domain"
	"ngac-platform/testutil"
)

func pushedTo(w *pushWorld) []string {
	var ids []string
	for _, m := range w.sender.all() {
		ids = append(ids, m.ExternalIDs...)
	}
	return ids
}

// "@Tất cả" mentions every current member as if by name: it reaches a member
// in quiet hours and one who set the group to "off", as a name mention does,
// and still reaches nobody who cannot read the group, nor the sender.
func TestPush_MentionAllReachesEveryMemberAsAMention(t *testing.T) {
	w := newGroupWithMember(t)
	ctx := context.Background()
	require.NoError(t, w.f.svc.SavePushPreferences(w.f.callerCtx(), w.other, quietNow()))
	_, err := w.f.pool.Exec(ctx, `INSERT INTO channel_notification_levels (user_id, channel_id, level) VALUES ($1, $2, 'off')`, w.other, w.chID)
	require.NoError(t, err)
	t.Cleanup(func() { w.f.pool.Exec(ctx, `DELETE FROM channel_notification_levels WHERE channel_id = $1`, w.chID) })

	third, thirdNode := testutil.CreateUser(t, w.f.pool)
	w.f.read.allow[grant{thirdNode, w.oaID, ngac.OpRead}] = true
	_, goneNode := testutil.CreateUser(t, w.f.pool) // still in the roster cache, no longer reads the group
	for _, n := range []string{thirdNode, goneNode, "u-" + w.f.userID} {
		_, err = w.f.pool.Exec(ctx, `INSERT INTO channel_members (channel_id, ngac_node_id) VALUES ($1, $2)`, w.chID, n)
		require.NoError(t, err)
	}

	w.send(t, "không nhắc ai")
	require.Equal(t, []string{third}, pushedTo(w), "a plain message: quiet hours and off hold the other member back")

	w.send(t, "họp gấp", domain.MentionAll)

	// Messages close together collapse into one notification per person, so
	// the third member's two show as one; what matters is who is reached.
	got := pushedTo(w)
	assert.Contains(t, got, w.other, "@Tất cả gets through quiet hours and off, as a name mention does")
	assert.Contains(t, got, third)
	assert.NotContains(t, got, w.f.userID, "not the sender")
	assert.Len(t, got, 2, "nobody who cannot read the group")
}

// The members behind "@Tất cả" are not cut by the cap on typed mentions.
func TestPush_MentionAllIsNotCappedLikeTypedMentions(t *testing.T) {
	w := newGroupWithMember(t)
	ctx := context.Background()
	want := []string{w.other}
	for i := 0; i < domain.MaxMentionPushes+2; i++ {
		id, node := testutil.CreateUser(t, w.f.pool)
		w.f.read.allow[grant{node, w.oaID, ngac.OpRead}] = true
		_, err := w.f.pool.Exec(ctx, `INSERT INTO channel_members (channel_id, ngac_node_id) VALUES ($1, $2)`, w.chID, node)
		require.NoError(t, err)
		want = append(want, id)
	}

	w.send(t, "cả nhóm", domain.MentionAll)

	assert.ElementsMatch(t, want, pushedTo(w))
}
