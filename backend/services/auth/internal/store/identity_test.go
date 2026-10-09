package store_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// insertTestWorkspace creates a bare workspace row owned by ownerID.
func insertTestWorkspace(t *testing.T, pool *pgxpool.Pool, ownerID string) string {
	t.Helper()
	id := fmt.Sprintf("test-ws-%d", time.Now().UnixNano())
	_, err := pool.Exec(context.Background(),
		"INSERT INTO workspaces (id, name, owner_id) VALUES ($1, $2, $3)", id, "Test "+id, ownerID)
	require.NoError(t, err)
	t.Cleanup(func() {
		pool.Exec(context.Background(), "DELETE FROM workspaces WHERE id = $1", id)
	})
	return id
}

func uniqueDomain() string {
	return fmt.Sprintf("t%d.example", time.Now().UnixNano())
}

func TestLinkIdentity_ThenFindUserByIdentity(t *testing.T) {
	s := setupStore(t)
	pool := getPool(t, s)
	ctx := context.Background()
	userID, username, _ := insertTestUser(t, s, pool)
	subject := fmt.Sprintf("sub-%d", time.Now().UnixNano())
	t.Cleanup(func() { pool.Exec(ctx, "DELETE FROM user_identities WHERE subject = $1", subject) })

	require.NoError(t, s.LinkIdentity(ctx, "google", subject, userID, "a@example.com"))

	got, err := s.FindUserByIdentity(ctx, "google", subject)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, userID, got.ID)
	assert.Equal(t, username, got.Username)

	linked, err := s.GetIdentitySubject(ctx, "google", userID)
	require.NoError(t, err)
	assert.Equal(t, subject, linked)
}

func TestFindUserByIdentity_UnknownSubject(t *testing.T) {
	s := setupStore(t)
	got, err := s.FindUserByIdentity(context.Background(), "google", "no-such-subject")
	require.NoError(t, err)
	assert.Nil(t, got)

	subj, err := s.GetIdentitySubject(context.Background(), "google", "no-such-user")
	require.NoError(t, err)
	assert.Empty(t, subj)
}

// A subject is bound to one account for good: re-linking it must not move it.
func TestLinkIdentity_DoesNotRebindAnExistingSubject(t *testing.T) {
	s := setupStore(t)
	pool := getPool(t, s)
	ctx := context.Background()
	first, _, _ := insertTestUser(t, s, pool)
	second, _, _ := insertTestUser(t, s, pool)
	subject := fmt.Sprintf("sub-%d", time.Now().UnixNano())
	t.Cleanup(func() { pool.Exec(ctx, "DELETE FROM user_identities WHERE subject = $1", subject) })

	require.NoError(t, s.LinkIdentity(ctx, "google", subject, first, "a@example.com"))
	require.NoError(t, s.LinkIdentity(ctx, "google", subject, second, "b@example.com"))

	got, err := s.FindUserByIdentity(ctx, "google", subject)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, first, got.ID)
}

func TestClaimTenantDomain(t *testing.T) {
	s := setupStore(t)
	pool := getPool(t, s)
	ctx := context.Background()
	owner, _, _ := insertTestUser(t, s, pool)
	wsA := insertTestWorkspace(t, pool, owner)
	wsB := insertTestWorkspace(t, pool, owner)
	d := uniqueDomain()

	ok, err := s.ClaimTenantDomain(ctx, wsA, strings.ToUpper(d))
	require.NoError(t, err)
	assert.True(t, ok, "first claim must succeed")

	found, err := s.FindTenantByDomain(ctx, d)
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, wsA, found.ID)
	assert.Equal(t, d, found.Domain, "domain is stored lowercased")

	var wsType string
	require.NoError(t, pool.QueryRow(ctx, "SELECT type FROM workspaces WHERE id = $1", wsA).Scan(&wsType))
	assert.Equal(t, "organization", wsType, "a tenant that owns a company domain is an organization")

	ok, err = s.ClaimTenantDomain(ctx, wsB, d)
	require.NoError(t, err)
	assert.False(t, ok, "a domain already owned by another tenant must not be claimed again")

	ok, err = s.ClaimTenantDomain(ctx, wsA, uniqueDomain())
	require.NoError(t, err)
	assert.False(t, ok, "a tenant that already has a domain keeps it")
}

func TestClearPassword(t *testing.T) {
	s := setupStore(t)
	pool := getPool(t, s)
	ctx := context.Background()
	userID, username, _ := insertTestUser(t, s, pool)

	require.NoError(t, s.ClearPassword(ctx, userID))

	got, err := s.GetUserByUsername(ctx, username)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Empty(t, got.Password, "password must be cleared")
}

func TestFindTenantByDomain_IsCaseInsensitive(t *testing.T) {
	s := setupStore(t)
	pool := getPool(t, s)
	ctx := context.Background()
	owner, _, _ := insertTestUser(t, s, pool)
	ws := insertTestWorkspace(t, pool, owner)
	d := uniqueDomain()
	_, err := pool.Exec(ctx, "UPDATE workspaces SET domain = $2 WHERE id = $1", ws, strings.ToUpper(d))
	require.NoError(t, err)

	found, err := s.FindTenantByDomain(ctx, d)
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, ws, found.ID)
}
