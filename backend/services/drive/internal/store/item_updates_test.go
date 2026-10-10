package store_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/services/drive/internal/store"
)

func TestQuota_IncrementDecrementNeverGoBelowZero(t *testing.T) {
	s, ws, _ := pendingFile(t)
	ctx := context.Background()
	_, err := s.GetOrCreateQuota(ctx, ws)
	require.NoError(t, err)

	require.NoError(t, s.IncrementQuota(ctx, ws, 500, 2))
	q, _ := s.GetOrCreateQuota(ctx, ws)
	assert.EqualValues(t, 500, q.UsedBytes)
	assert.EqualValues(t, 2, q.UsedFiles)

	require.NoError(t, s.DecrementQuota(ctx, ws, 200, 1))
	q, _ = s.GetOrCreateQuota(ctx, ws)
	assert.EqualValues(t, 300, q.UsedBytes)
	assert.EqualValues(t, 1, q.UsedFiles)

	require.NoError(t, s.DecrementQuota(ctx, ws, 10_000, 10))
	q, _ = s.GetOrCreateQuota(ctx, ws)
	assert.EqualValues(t, 0, q.UsedBytes, "usage is floored at zero")
	assert.EqualValues(t, 0, q.UsedFiles)
}

func TestCheckQuota_RefusesWhatDoesNotFit(t *testing.T) {
	s, ws, _ := pendingFile(t)
	ctx := context.Background()
	_, err := s.GetOrCreateQuota(ctx, ws)
	require.NoError(t, err)

	ok, err := s.CheckQuota(ctx, ws, 1<<40)
	require.NoError(t, err)
	assert.True(t, ok, "no limit by default")

	require.NoError(t, s.UpdateQuotaLimits(ctx, ws, 1000, 5))
	require.NoError(t, s.IncrementQuota(ctx, ws, 900, 1))
	ok, _ = s.CheckQuota(ctx, ws, 100)
	assert.True(t, ok, "exactly at the limit still fits")
	ok, _ = s.CheckQuota(ctx, ws, 101)
	assert.False(t, ok, "one byte over does not")

	require.NoError(t, s.UpdateQuotaLimits(ctx, ws, 1000, 1))
	ok, _ = s.CheckQuota(ctx, ws, 1)
	assert.False(t, ok, "the file-count limit holds even with room in bytes")
}

func TestItemUpdates_StatusNodeSizeAndChildFiles(t *testing.T) {
	s, pool := newStore(t)
	top, mid, leaf := treeFixture(t, s, pool)
	ctx := context.Background()

	require.NoError(t, s.UpdateStatus(ctx, mid, "trashed"))
	got, err := s.GetItem(ctx, mid)
	require.NoError(t, err)
	assert.Equal(t, "trashed", got.Status)
	assert.NotNil(t, got.TrashedAt)
	require.NoError(t, s.UpdateStatus(ctx, mid, "active"))
	got, _ = s.GetItem(ctx, mid)
	assert.Equal(t, "active", got.Status)
	assert.Nil(t, got.TrashedAt, "restoring clears the trash time")

	require.NoError(t, s.UpdateNGACNodeID(ctx, leaf, "node-elsewhere"))
	got, _ = s.GetItem(ctx, leaf)
	assert.Equal(t, "node-elsewhere", got.NGACNodeID)

	// A file below the top folder: found by the recursive child query.
	size := int64(7)
	key := "k/file"
	require.NoError(t, s.InsertItem(ctx, &store.DriveItem{
		ID: "file-" + top, WorkspaceID: got.WorkspaceID, DriveContext: "workspace", ParentID: ptr(leaf), ItemType: "file",
		Name: "f", SizeBytes: &size, ObjectKey: &key, NGACNodeID: "n", OwnerID: got.OwnerID, Status: "active",
	}))
	files, err := s.GetChildFiles(ctx, top)
	require.NoError(t, err)
	require.Len(t, files, 1, "folders are not files and depth does not hide one")
	assert.Equal(t, "file-"+top, files[0].ID)
	none, err := s.GetChildFiles(ctx, leaf+"-nope")
	require.NoError(t, err)
	assert.Empty(t, none)

	require.NoError(t, s.UpdateFileSize(ctx, "file-"+top, 99))
	f, _ := s.GetItem(ctx, "file-"+top)
	assert.EqualValues(t, 99, *f.SizeBytes)
}

func TestSharesByTarget_FindsTheTargetsAndPublicOnesOfActiveItems(t *testing.T) {
	s, pool := newStore(t)
	top, mid, _ := treeFixture(t, s, pool)
	ctx := context.Background()
	target, label := "target-node-"+top, "Phòng Kế toán"

	for _, sh := range []*store.DriveShare{
		{DriveItemID: top, ShareType: "user", TargetNGACID: &target, TargetLabel: &label, Operations: []string{"read"}, NGACShareOA: "oa-1", CreatedBy: "u"},
		{DriveItemID: mid, ShareType: "public", Operations: []string{"read"}, NGACShareOA: "oa-2", CreatedBy: "u"},
	} {
		require.NoError(t, s.InsertShare(ctx, sh))
	}
	got, err := s.ListSharesByTarget(ctx, []string{target})
	require.NoError(t, err)
	ids := map[string]bool{}
	for _, g := range got {
		ids[g.DriveItemID] = true
	}
	assert.True(t, ids[top], "the share aimed at the target")
	assert.True(t, ids[mid], "a public share reaches everyone")

	other, err := s.ListSharesByTarget(ctx, []string{"someone-else"})
	require.NoError(t, err)
	for _, g := range other {
		assert.NotEqual(t, top, g.DriveItemID, "a share aimed elsewhere is not shown")
	}

	// A trashed item's shares are not listed.
	require.NoError(t, s.UpdateStatus(ctx, top, "trashed"))
	got, _ = s.ListSharesByTarget(ctx, []string{target})
	for _, g := range got {
		assert.NotEqual(t, top, g.DriveItemID)
	}
}
