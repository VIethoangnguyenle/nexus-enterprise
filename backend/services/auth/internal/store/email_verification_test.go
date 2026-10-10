package store_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/services/auth/internal/store"
)

func TestEmailVerificationAndCaseInsensitiveAddresses(t *testing.T) {
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, testDBURL())
	require.NoError(t, err)
	if err := pool.Ping(ctx); err != nil {
		t.Skipf("test DB not available: %v", err)
	}
	t.Cleanup(pool.Close)
	st := store.New(pool)

	n := time.Now().UnixNano()
	addr := fmt.Sprintf("Case.%d@Example.vn", n)
	id := fmt.Sprintf("ev-user-%d", n)
	node := func(name string) string {
		nid := fmt.Sprintf("evn-%s-%d", name, n)
		_, err := pool.Exec(ctx, `INSERT INTO ngac_nodes (id, name, node_type, properties) VALUES ($1, $1, 'U', '{}')`, nid)
		require.NoError(t, err)
		t.Cleanup(func() {
			pool.Exec(context.Background(), `DELETE FROM users WHERE ngac_node = $1`, nid)
			pool.Exec(context.Background(), `DELETE FROM ngac_nodes WHERE id = $1`, nid)
		})
		return nid
	}
	require.NoError(t, st.CreateUser(ctx, id, fmt.Sprintf("ev%d", n), "", node("a"), addr, fmt.Sprintf("u-%d", n), "Eve", ""))
	t.Cleanup(func() { pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, id) })

	t.Run("an address is found whatever its case", func(t *testing.T) {
		for _, in := range []string{addr, fmt.Sprintf("case.%d@example.vn", n), fmt.Sprintf("CASE.%d@EXAMPLE.VN", n)} {
			u, err := st.GetUserByEmail(ctx, in)
			require.NoError(t, err)
			require.NotNil(t, u, in)
			assert.Equal(t, id, u.ID)
		}
	})

	t.Run("a case variant cannot be registered beside it", func(t *testing.T) {
		err := st.CreateUser(ctx, fmt.Sprintf("ev-dup-%d", n), fmt.Sprintf("evdup%d", n), "", node("b"), fmt.Sprintf("case.%d@example.vn", n), fmt.Sprintf("ud-%d", n), "Dup", "")
		require.Error(t, err, "the unique index on lower(email) refuses it")
		pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, fmt.Sprintf("ev-dup-%d", n))
	})

	t.Run("a new account is unverified; the first proof sets it, the second does nothing", func(t *testing.T) {
		var at *time.Time
		require.NoError(t, pool.QueryRow(ctx, `SELECT email_verified_at FROM users WHERE id = $1`, id).Scan(&at))
		assert.Nil(t, at)

		changed, err := st.MarkEmailVerified(ctx, id)
		require.NoError(t, err)
		assert.True(t, changed)
		changed, err = st.MarkEmailVerified(ctx, id)
		require.NoError(t, err)
		assert.False(t, changed, "already proved")
		require.NoError(t, pool.QueryRow(ctx, `SELECT email_verified_at FROM users WHERE id = $1`, id).Scan(&at))
		assert.NotNil(t, at)
	})

	t.Run("an account with no address has nothing to verify", func(t *testing.T) {
		pid := fmt.Sprintf("ev-phone-%d", n)
		require.NoError(t, st.CreateUser(ctx, pid, fmt.Sprintf("evp%d", n), "", node("c"), "", fmt.Sprintf("up-%d", n), "P", fmt.Sprintf("09%08d", n%100000000)))
		t.Cleanup(func() { pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, pid) })
		changed, err := st.MarkEmailVerified(ctx, pid)
		require.NoError(t, err)
		assert.False(t, changed)
	})
}

// A person whose address is already proved is created verified in one statement.
func TestCreateUserWithVerifiedEmail_IsBornVerified(t *testing.T) {
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, testDBURL())
	require.NoError(t, err)
	if err := pool.Ping(ctx); err != nil {
		t.Skipf("test DB not available: %v", err)
	}
	t.Cleanup(pool.Close)
	st := store.New(pool)

	n := time.Now().UnixNano()
	node := func(name string) string {
		nid := fmt.Sprintf("evv-%s-%d", name, n)
		_, err := pool.Exec(ctx, `INSERT INTO ngac_nodes (id, name, node_type, properties) VALUES ($1, $1, 'U', '{}')`, nid)
		require.NoError(t, err)
		t.Cleanup(func() {
			pool.Exec(context.Background(), `DELETE FROM users WHERE ngac_node = $1`, nid)
			pool.Exec(context.Background(), `DELETE FROM ngac_nodes WHERE id = $1`, nid)
		})
		return nid
	}

	id := fmt.Sprintf("evv-user-%d", n)
	require.NoError(t, st.CreateUserWithVerifiedEmail(ctx, id, fmt.Sprintf("evv%d", n), "", node("a"), fmt.Sprintf("v.%d@example.vn", n), fmt.Sprintf("uv-%d", n), "Vee", ""))
	u, err := st.GetUserByID(ctx, id)
	require.NoError(t, err)
	assert.True(t, u.EmailVerified, "created verified")

	noAddr := fmt.Sprintf("evv-none-%d", n)
	require.NoError(t, st.CreateUserWithVerifiedEmail(ctx, noAddr, fmt.Sprintf("evn%d", n), "", node("b"), "", fmt.Sprintf("un-%d", n), "Nil", ""))
	u, err = st.GetUserByID(ctx, noAddr)
	require.NoError(t, err)
	assert.False(t, u.EmailVerified, "no address, nothing to verify")
}
