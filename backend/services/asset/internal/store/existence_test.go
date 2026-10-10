package store_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHasExistingAssets_IgnoresDeletedOnesAndOtherTypes(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()
	wsID := getTestWorkspaceID(t, s.DB())
	userID := getTestUserID(t, s.DB())
	used := createTestType(t, s, wsID)
	empty := createTestType(t, s, wsID)

	has, err := s.HasExistingAssets(ctx, used.ID)
	require.NoError(t, err)
	assert.False(t, has, "a type with no assets")

	a := createTestAsset(t, s, used.ID, wsID, userID)
	has, _ = s.HasExistingAssets(ctx, used.ID)
	assert.True(t, has)
	has, _ = s.HasExistingAssets(ctx, empty.ID)
	assert.False(t, has, "another type's asset does not count")

	require.NoError(t, s.SoftDeleteAsset(ctx, a.ID))
	has, _ = s.HasExistingAssets(ctx, used.ID)
	assert.False(t, has, "a deleted asset no longer holds its type")
}

func TestIsAssetAssigned_FollowsTheAssigneeAndRefusesAMissingAsset(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()
	wsID := getTestWorkspaceID(t, s.DB())
	userID := getTestUserID(t, s.DB())
	at := createTestType(t, s, wsID)
	a := createTestAsset(t, s, at.ID, wsID, userID)

	assigned, err := s.IsAssetAssigned(ctx, a.ID)
	require.NoError(t, err)
	assert.False(t, assigned)

	_, err = s.DB().Exec(ctx, `UPDATE assets SET assigned_to = $1 WHERE id = $2`, userID, a.ID)
	require.NoError(t, err)
	assigned, err = s.IsAssetAssigned(ctx, a.ID)
	require.NoError(t, err)
	assert.True(t, assigned)

	_, err = s.IsAssetAssigned(ctx, "no-such-asset")
	assert.Error(t, err)
	require.NoError(t, s.SoftDeleteAsset(ctx, a.ID))
	_, err = s.IsAssetAssigned(ctx, a.ID)
	assert.Error(t, err, "a deleted asset is treated as missing")
}
