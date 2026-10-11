package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/services/workspace/internal/store"
	"ngac-platform/testutil"
)

func personalWorkspace(t *testing.T, st *store.Store) (ws, owner, ownerNode string) {
	t.Helper()
	pool := testutil.SetupTestDB(t)
	owner, ownerNode = testutil.CreateUser(t, pool)
	ws, _ = testutil.CreateWorkspace(t, pool, owner)
	_, err := pool.Exec(context.Background(), `UPDATE workspaces SET type = 'personal' WHERE id = $1`, ws)
	require.NoError(t, err)
	return ws, owner, ownerNode
}

func TestConvertToOrganization_ChangesTypeOnceAndRecordsWho(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	st := store.New(pool)
	ctx := context.Background()
	owner, ownerNode := testutil.CreateUser(t, pool)
	ws, _ := testutil.CreateWorkspace(t, pool, owner)
	other, _ := testutil.CreateWorkspace(t, pool, owner)
	_, err := pool.Exec(ctx, `UPDATE workspaces SET type = 'personal' WHERE id IN ($1, $2)`, ws, other)
	require.NoError(t, err)

	before := time.Now().Add(-time.Second)
	c, err := st.ConvertToOrganization(ctx, ws, ownerNode)

	require.NoError(t, err)
	assert.Equal(t, "testuser", c.ByName)
	assert.True(t, c.At.After(before))
	got, err := st.GetByID(ctx, ws)
	require.NoError(t, err)
	assert.Equal(t, "organization", got.Type)
	untouched, err := st.GetByID(ctx, other)
	require.NoError(t, err)
	assert.Equal(t, "personal", untouched.Type, "another workspace is untouched")
	var by string
	require.NoError(t, pool.QueryRow(ctx, `SELECT converted_by FROM workspaces WHERE id = $1`, ws).Scan(&by))
	assert.Equal(t, owner, by)
}

func TestConvertToOrganization_RefusesWhatIsNotPersonal(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	st := store.New(pool)
	ctx := context.Background()
	owner, ownerNode := testutil.CreateUser(t, pool)
	org, _ := testutil.CreateWorkspace(t, pool, owner) // made an organization

	_, err := st.ConvertToOrganization(ctx, org, ownerNode)
	assert.ErrorIs(t, err, store.ErrNotPersonal)
	var by *string
	var at *time.Time
	require.NoError(t, pool.QueryRow(ctx, `SELECT converted_by, converted_at FROM workspaces WHERE id = $1`, org).Scan(&by, &at))
	assert.Nil(t, by, "a workspace made an organization was never converted")
	assert.Nil(t, at)

	_, err = st.ConvertToOrganization(ctx, "no-such-workspace", ownerNode)
	assert.ErrorIs(t, err, store.ErrWorkspaceNotFound)
}

func TestConvertToOrganization_OnlyOneOfTwoAtOnceWins(t *testing.T) {
	st := store.New(testutil.SetupTestDB(t))
	ws, _, node := personalWorkspace(t, st)

	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() { _, err := st.ConvertToOrganization(context.Background(), ws, node); results <- err }()
	}
	errs := []error{<-results, <-results}

	won := 0
	for _, err := range errs {
		if err == nil {
			won++
		} else {
			assert.ErrorIs(t, err, store.ErrNotPersonal)
		}
	}
	assert.Equal(t, 1, won)
}

func TestConversionRecord_NamesWhoConvertedAndWhen(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	st := store.New(pool)
	ctx := context.Background()
	owner, _ := testutil.CreateUser(t, pool)
	ws, _ := testutil.CreateWorkspace(t, pool, owner)
	_, err := pool.Exec(ctx, `UPDATE users SET display_name = 'Lan Anh' WHERE id = $1`, owner)
	require.NoError(t, err)

	r, err := st.ConversionRecord(ctx, ws)
	require.NoError(t, err)
	assert.Empty(t, r.ByName)
	assert.Nil(t, r.At, "never converted")

	_, err = pool.Exec(ctx, `UPDATE workspaces SET converted_by = $2, converted_at = NOW() WHERE id = $1`, ws, owner)
	require.NoError(t, err)
	r, err = st.ConversionRecord(ctx, ws)
	require.NoError(t, err)
	assert.Equal(t, "Lan Anh", r.ByName)
	require.NotNil(t, r.At)

	_, err = st.ConversionRecord(ctx, "no-such-workspace")
	assert.ErrorIs(t, err, store.ErrWorkspaceNotFound)
}
