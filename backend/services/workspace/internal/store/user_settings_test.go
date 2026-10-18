package store_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/services/workspace/internal/store"
	"ngac-platform/testutil"
)

func TestUserSettingsStore(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	ctx := context.Background()
	st := store.New(pool)

	ownerID, _ := testutil.CreateUser(t, pool)
	wsA, _ := testutil.CreateWorkspace(t, pool, ownerID)
	wsB, _ := testutil.CreateWorkspace(t, pool, ownerID)
	other, _ := testutil.CreateUser(t, pool)
	bar := func(ids ...string) map[string]json.RawMessage {
		b, _ := json.Marshal(ids)
		return map[string]json.RawMessage{"mobile.bar": b}
	}

	got, err := st.GetUserSettings(ctx, ownerID, wsA)
	require.NoError(t, err)
	assert.Empty(t, got, "a person who never saved has no rows")

	require.NoError(t, st.PutUserSettings(ctx, ownerID, wsA, bar("chat", "drive", "approval", "assets")))
	require.NoError(t, st.PutUserSettings(ctx, ownerID, wsA, bar("chat", "calendar", "drive", "contacts")), "a second write replaces the first")

	got, err = st.GetUserSettings(ctx, ownerID, wsA)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.JSONEq(t, `["chat","calendar","drive","contacts"]`, string(got["mobile.bar"]))

	other1, err := st.GetUserSettings(ctx, other, wsA)
	require.NoError(t, err)
	assert.Empty(t, other1, "another person's rows are not mine")
	otherWs, err := st.GetUserSettings(ctx, ownerID, wsB)
	require.NoError(t, err)
	assert.Empty(t, otherWs, "settings are per workspace")

	var n int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM user_settings WHERE user_id = $1`, ownerID).Scan(&n))
	assert.Equal(t, 1, n, "one row per key, not one per write")
}
