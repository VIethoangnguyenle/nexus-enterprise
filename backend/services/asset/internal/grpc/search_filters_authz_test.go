package grpc_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"

	"ngac-platform/ngac"
	pb "ngac-platform/proto/asset"
	"ngac-platform/services/asset/internal/domain"
	agrpc "ngac-platform/services/asset/internal/grpc"
)

// The unified search reaches assets through the asset list with its search
// words and filters. These pin that the words never widen what the caller may
// read: a match of an unreadable type is neither returned nor counted.

// Both fixture assets are named "authz-a?-<suffix>", so "authz-a" matches both.
const bothAssetsMatch = "authz-a"

func TestListAssets_SearchSkipsMatchesOfUnreadableType(t *testing.T) {
	f := newFixture(t)
	p := f.policy()
	p.grant("n-reader", f.oaA, ngac.OpRead)
	srv := agrpc.NewAssetServer(f.st, p, nil)

	list, err := srv.ListAssets(asCaller("", "n-reader"), &pb.ListAssetsRequest{
		WorkspaceId: f.wsID, Search: bothAssetsMatch,
	})

	require.NoError(t, err)
	assert.Equal(t, []string{f.assetA}, assetIDs(list))
	assert.Equal(t, int32(1), list.Total, "a hidden match must not be counted")
}

func TestListAssets_SearchWithoutReadSeesNothing(t *testing.T) {
	f := newFixture(t)
	p := f.policy()
	p.grant("n-member", f.oaA, ngac.OpWrite)
	srv := agrpc.NewAssetServer(f.st, p, nil)

	list, err := srv.ListAssets(asCaller("", "n-member"), &pb.ListAssetsRequest{
		WorkspaceId: f.wsID, Search: bothAssetsMatch,
	})

	require.NoError(t, err)
	assert.Empty(t, list.Assets)
	assert.Zero(t, list.Total)
}

// A filter naming an unreadable type combined with words matching an asset of
// that type still returns nothing.
func TestListAssets_SearchWithUnreadableTypeFilterIsEmpty(t *testing.T) {
	f := newFixture(t)
	p := f.policy()
	p.grant("n-reader", f.oaA, ngac.OpRead)
	srv := agrpc.NewAssetServer(f.st, p, nil)

	list, err := srv.ListAssets(asCaller("", "n-reader"), &pb.ListAssetsRequest{
		WorkspaceId: f.wsID, TypeId: f.typeB, Search: bothAssetsMatch,
	})

	require.NoError(t, err)
	assert.Empty(t, list.Assets)
	assert.Zero(t, list.Total)
}

// Words, state and holder filters narrow together.
func TestListAssets_SearchAndFiltersNarrowTogether(t *testing.T) {
	f := newFixture(t)
	_, err := f.pool.Exec(context.Background(),
		`UPDATE assets SET state = 'assigned', assigned_to = $1 WHERE id = $2`, f.userY, f.assetB)
	require.NoError(t, err)
	p := f.policy()
	p.grant("n-owner", f.oaA, ngac.OpRead)
	p.grant("n-owner", f.oaB, ngac.OpRead)
	srv := agrpc.NewAssetServer(f.st, p, nil)

	byState, err := srv.ListAssets(asCaller("", "n-owner"), &pb.ListAssetsRequest{
		WorkspaceId: f.wsID, Search: bothAssetsMatch, State: "assigned",
	})
	require.NoError(t, err)
	assert.Equal(t, []string{f.assetB}, assetIDs(byState))

	byHolder, err := srv.ListAssets(asCaller("", "n-owner"), &pb.ListAssetsRequest{
		WorkspaceId: f.wsID, Search: bothAssetsMatch, AssignedTo: f.userY,
	})
	require.NoError(t, err)
	assert.Equal(t, []string{f.assetB}, assetIDs(byHolder))
	assert.Equal(t, int32(1), byHolder.Total)
}

// A search in a personal workspace is refused by rule, whatever the policy says.
func TestListAssets_SearchInPersonalWorkspaceIsRefused(t *testing.T) {
	f := newFixture(t)
	_, err := f.pool.Exec(context.Background(), `UPDATE workspaces SET type = 'personal' WHERE id = $1`, f.wsID)
	require.NoError(t, err)
	p := f.policy()
	p.grant("n-owner", f.oaA, ngac.OpRead)
	p.grant("n-owner", f.oaB, ngac.OpRead)

	srv := agrpc.NewAssetServer(f.st, p, nil)

	list, err := srv.ListAssets(asCaller("", "n-owner"), &pb.ListAssetsRequest{
		WorkspaceId: f.wsID, Search: bothAssetsMatch,
	})
	asserted(t, err, codes.PermissionDenied)
	assert.Empty(t, list.GetAssets())
}

// Sorting never widens what the caller may read: an asset of an unreadable type
// is neither returned nor counted, even when it would sort first.
func TestListAssets_SortSkipsMatchesOfUnreadableType(t *testing.T) {
	f := newFixture(t)
	p := f.policy()
	// Reader may only read type A, not type B.
	p.grant("n-reader", f.oaA, ngac.OpRead)

	// Make assetB sort BEFORE assetA across every sort key:
	// name: "0-unreadable" < "z-readable"
	// created: assetB newer than assetA
	// updated: assetB newer than assetA
	// state: "assigned" < "maintenance"
	now := time.Now()
	_, err := f.pool.Exec(context.Background(),
		`UPDATE assets SET name = '0-unreadable', state = 'assigned', created_at = $1, updated_at = $1 WHERE id = $2`,
		now, f.assetB)
	require.NoError(t, err)
	_, err = f.pool.Exec(context.Background(),
		`UPDATE assets SET name = 'z-readable', state = 'maintenance', created_at = $1, updated_at = $1 WHERE id = $2`,
		now.Add(-1*time.Hour), f.assetA)
	require.NoError(t, err)

	svc := domain.NewAssetService(f.st, p, nil)

	for _, sort := range []string{"name", "created", "updated", "status", ""} {
		t.Run("sort="+sort, func(t *testing.T) {
			list, err := svc.ListAssets(asCaller("", "n-reader"), &pb.ListAssetsRequest{
				WorkspaceId: f.wsID,
			}, sort)
			require.NoError(t, err)
			assert.Equal(t, []string{f.assetA}, assetIDs(list), "unreadable asset must never be returned")
			assert.Equal(t, int32(1), list.Total, "unreadable asset must never be counted")
		})
	}
}
