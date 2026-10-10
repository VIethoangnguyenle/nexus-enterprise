package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/services/workspace/internal/store"
	"ngac-platform/testutil"
)

func TestInvitationStore(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	ctx := context.Background()
	st := store.New(pool)
	ownerID, _ := testutil.CreateUser(t, pool)
	wsA, _ := testutil.CreateWorkspace(t, pool, ownerID)
	wsB, _ := testutil.CreateWorkspace(t, pool, ownerID)
	now := time.Now().UTC().Truncate(time.Second)
	addr := "inv." + uuid.NewString()[:8] + "@example.vn"
	other := "other." + uuid.NewString()[:8] + "@example.vn"
	week := now.Add(7 * 24 * time.Hour)
	mk := func(ws, email string, exp time.Time) *store.Invitation {
		return &store.Invitation{WorkspaceID: ws, Email: email, InvitedBy: "node-inviter", ExpiresAt: exp}
	}

	t.Run("inviting twice is one open offer, refreshed", func(t *testing.T) {
		require.NoError(t, st.UpsertInvitation(ctx, mk(wsA, addr, now.Add(time.Hour))))
		require.NoError(t, st.UpsertInvitation(ctx, mk(wsA, addr, week)))
		list, err := st.ListPendingForWorkspace(ctx, wsA, now)
		require.NoError(t, err)
		require.Len(t, list, 1)
		assert.True(t, list[0].ExpiresAt.Equal(week), "the expiry was refreshed")
		assert.Equal(t, "node-inviter", list[0].InvitedBy)
	})

	t.Run("an offer is per workspace and per address", func(t *testing.T) {
		require.NoError(t, st.UpsertInvitation(ctx, mk(wsB, addr, week)))
		require.NoError(t, st.UpsertInvitation(ctx, mk(wsA, other, week)))
		mine, err := st.ListPendingForEmail(ctx, addr, now)
		require.NoError(t, err)
		assert.Len(t, mine, 2)
		inA, _ := st.ListPendingForWorkspace(ctx, wsA, now)
		assert.Len(t, inA, 2)
	})

	t.Run("an expired offer is not listed", func(t *testing.T) {
		list, err := st.ListPendingForEmail(ctx, addr, week.Add(time.Minute))
		require.NoError(t, err)
		assert.Empty(t, list)
	})

	var id string
	t.Run("only the addressee answers, once", func(t *testing.T) {
		mine, _ := st.ListPendingForEmail(ctx, addr, now)
		for _, m := range mine {
			if m.WorkspaceID == wsA {
				id = m.ID
			}
		}
		require.NotEmpty(t, id)
		got, err := st.RespondInvitation(ctx, id, other, store.InvitationAccepted, now)
		require.NoError(t, err)
		assert.Nil(t, got, "another address cannot answer it")

		got, err = st.RespondInvitation(ctx, id, addr, store.InvitationAccepted, week.Add(time.Hour))
		require.NoError(t, err)
		assert.Nil(t, got, "not after it has expired")

		got, err = st.RespondInvitation(ctx, id, addr, store.InvitationAccepted, now)
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, store.InvitationAccepted, got.Status)

		got, err = st.RespondInvitation(ctx, id, addr, store.InvitationDeclined, now)
		require.NoError(t, err)
		assert.Nil(t, got, "an answered offer cannot be answered again")
	})

	t.Run("an accepted offer can be reopened, and only an accepted one", func(t *testing.T) {
		require.NoError(t, st.ReopenInvitation(ctx, id))
		got, _ := st.GetInvitation(ctx, id)
		assert.Equal(t, store.InvitationPending, got.Status)
		require.NoError(t, st.ReopenInvitation(ctx, id))
		got, _ = st.GetInvitation(ctx, id)
		assert.Equal(t, store.InvitationPending, got.Status)
	})

	t.Run("revoking stays inside the workspace and only while pending", func(t *testing.T) {
		ok, err := st.RevokeInvitation(ctx, wsB, id, now)
		require.NoError(t, err)
		assert.False(t, ok, "wsB cannot revoke wsA's offer")
		ok, err = st.RevokeInvitation(ctx, wsA, id, now)
		require.NoError(t, err)
		assert.True(t, ok)
		ok, _ = st.RevokeInvitation(ctx, wsA, id, now)
		assert.False(t, ok)
		got, _ := st.GetInvitation(ctx, id)
		assert.Equal(t, store.InvitationRevoked, got.Status)
		// After a revoke the address can be invited afresh.
		require.NoError(t, st.UpsertInvitation(ctx, mk(wsA, addr, week)))
	})

	t.Run("unknown ID is nil, not an error", func(t *testing.T) {
		got, err := st.GetInvitation(ctx, uuid.NewString())
		require.NoError(t, err)
		assert.Nil(t, got)
	})

	t.Run("only a proved address is the account's address", func(t *testing.T) {
		uid, node := testutil.CreateUser(t, pool)
		mixed := "Mixed.Case." + uuid.NewString()[:6] + "@Example.VN"
		_, err := pool.Exec(ctx, `UPDATE users SET email = $2 WHERE id = $1`, uid, mixed)
		require.NoError(t, err)

		// Typed at signup, not proved: it matches nothing.
		e, err := st.UserEmail(ctx, uid)
		require.NoError(t, err)
		assert.Empty(t, e, "an unverified address is only a claim")
		e, err = st.VerifiedEmailByNode(ctx, node)
		require.NoError(t, err)
		assert.Empty(t, e)

		_, err = pool.Exec(ctx, `UPDATE users SET email_verified_at = NOW() WHERE id = $1`, uid)
		require.NoError(t, err)
		e, err = st.UserEmail(ctx, uid)
		require.NoError(t, err)
		assert.Equal(t, e, lowerASCII(e))
		assert.Contains(t, e, "mixed.case.")
		e2, err := st.VerifiedEmailByNode(ctx, node)
		require.NoError(t, err)
		assert.Equal(t, e, e2)

		none, _ := testutil.CreateUser(t, pool)
		e, err = st.UserEmail(ctx, none)
		require.NoError(t, err)
		assert.Empty(t, e)
		e, err = st.UserEmail(ctx, "no-such-user")
		require.NoError(t, err)
		assert.Empty(t, e)
	})

	t.Run("withdrawing everything open for an address in one workspace", func(t *testing.T) {
		target := "gone." + uuid.NewString()[:8] + "@example.vn"
		require.NoError(t, st.UpsertInvitation(ctx, mk(wsA, target, week)))
		require.NoError(t, st.UpsertInvitation(ctx, mk(wsB, target, week)))
		n, err := st.RevokePendingForEmail(ctx, wsA, target, now)
		require.NoError(t, err)
		assert.Equal(t, 1, n)
		left, _ := st.ListPendingForEmail(ctx, target, now)
		require.Len(t, left, 1)
		assert.Equal(t, wsB, left[0].WorkspaceID, "another workspace's offer is untouched")
		n, _ = st.RevokePendingForEmail(ctx, wsA, target, now)
		assert.Zero(t, n)
	})
}

func lowerASCII(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 32
		}
	}
	return string(b)
}
