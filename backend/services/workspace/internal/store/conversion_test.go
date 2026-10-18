package store_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/services/workspace/internal/store"
	"ngac-platform/testutil"
)

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
