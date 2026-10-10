package store_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/services/drive/internal/store"
)

// pendingFile inserts a pending upload of a file in a workspace of its own.
func pendingFile(t *testing.T) (s *store.Store, ws, fileID string) {
	t.Helper()
	s, pool := newStore(t)
	top, _, _ := treeFixture(t, s, pool)
	item, err := s.GetItem(context.Background(), top)
	require.NoError(t, err)
	ws = item.WorkspaceID

	fileID = "f-" + uuid.NewString()
	key := "drive/" + fileID + "/a.pdf"
	size := int64(10)
	require.NoError(t, s.InsertItem(context.Background(), &store.DriveItem{
		ID: fileID, WorkspaceID: ws, DriveContext: "workspace", ParentID: ptr(top), ItemType: "file",
		Name: "a.pdf", SizeBytes: &size, ObjectKey: &key, NGACNodeID: "node-" + top, OwnerID: item.OwnerID, Status: "pending",
	}))
	t.Cleanup(func() { pool.Exec(context.Background(), `DELETE FROM drive_quotas WHERE workspace_id = $1`, ws) })
	return s, ws, fileID
}

func TestGetOrCreateQuota_CreatesTheDefaultOnceAndReturnsIt(t *testing.T) {
	s, ws, _ := pendingFile(t)
	ctx := context.Background()

	q, err := s.GetOrCreateQuota(ctx, ws)
	require.NoError(t, err)
	assert.Equal(t, ws, q.WorkspaceID)
	assert.EqualValues(t, 0, q.UsedBytes)
	assert.EqualValues(t, -1, q.MaxBytes, "no limit by default")

	again, err := s.GetOrCreateQuota(ctx, ws)
	require.NoError(t, err)
	assert.Equal(t, q.WorkspaceID, again.WorkspaceID)
}

func TestGetOrCreateQuota_ReportsADatabaseFailure(t *testing.T) {
	s, pool := newStore(t)
	pool.Close()
	_, err := s.GetOrCreateQuota(context.Background(), "any")
	assert.Error(t, err, "a failure must be returned, not retried into silence")
}

// A file becomes active and is charged to the quota in one step: never one
// without the other.
func TestActivateFile_PublishesAndChargesTogether(t *testing.T) {
	s, ws, id := pendingFile(t)
	ctx := context.Background()
	_, err := s.GetOrCreateQuota(ctx, ws)
	require.NoError(t, err)

	require.NoError(t, s.ActivateFile(ctx, id, ws, 2048))

	item, err := s.GetItem(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, "active", item.Status)
	require.NotNil(t, item.SizeBytes)
	assert.EqualValues(t, 2048, *item.SizeBytes, "the stored size replaces the declared one")
	q, err := s.GetOrCreateQuota(ctx, ws)
	require.NoError(t, err)
	assert.EqualValues(t, 2048, q.UsedBytes)
	assert.EqualValues(t, 1, q.UsedFiles)
}

// Confirming twice must not charge twice: the second finds nothing pending.
func TestActivateFile_ASecondConfirmChargesNothing(t *testing.T) {
	s, ws, id := pendingFile(t)
	ctx := context.Background()
	require.NoError(t, s.ActivateFile(ctx, id, ws, 100))

	err := s.ActivateFile(ctx, id, ws, 100)

	assert.True(t, errors.Is(err, store.ErrNotPending), "got %v", err)
	q, _ := s.GetOrCreateQuota(ctx, ws)
	assert.EqualValues(t, 100, q.UsedBytes)
	assert.EqualValues(t, 1, q.UsedFiles)
}

// A quota that cannot be charged leaves the file pending: the transaction rolls back.
func TestActivateFile_LeavesTheFilePendingWhenTheQuotaCannotBeCharged(t *testing.T) {
	s, _, id := pendingFile(t)
	ctx := context.Background()

	// A workspace with no row in workspaces cannot have a quota row: the
	// charge fails on the foreign key after the item update has run.
	err := s.ActivateFile(ctx, id, "no-such-workspace", 100)

	require.Error(t, err)
	item, gerr := s.GetItem(ctx, id)
	require.NoError(t, gerr)
	assert.Equal(t, "pending", item.Status, "the half-done publish was rolled back")
}

func TestDeleteItemReleasingQuota_RemovesTheRowAndReleasesTogether(t *testing.T) {
	s, ws, id := pendingFile(t)
	ctx := context.Background()
	require.NoError(t, s.ActivateFile(ctx, id, ws, 500))
	_, err := s.GetItem(ctx, id)
	require.NoError(t, err)

	removed, derr := s.DeleteItemReleasingQuota(ctx, id)
	require.NoError(t, derr)
	require.Len(t, removed, 1, "the deleted file is handed back for its stored object")

	gone, err := s.GetItem(ctx, id)
	require.NoError(t, err)
	assert.Nil(t, gone)
	q, _ := s.GetOrCreateQuota(ctx, ws)
	assert.EqualValues(t, 0, q.UsedBytes)
	assert.EqualValues(t, 0, q.UsedFiles)
}

func TestDeleteItemReleasingQuota_ARefusedDeleteReleasesNothing(t *testing.T) {
	s, ws, id := pendingFile(t)
	ctx := context.Background()
	require.NoError(t, s.ActivateFile(ctx, id, ws, 500))
	// An item that is not there removes no row, so nothing is released.
	gone, err := s.DeleteItemReleasingQuota(ctx, "no-such-item")
	require.NoError(t, err)
	assert.Empty(t, gone)

	q, _ := s.GetOrCreateQuota(ctx, ws)
	assert.EqualValues(t, 500, q.UsedBytes, "nothing was deleted, nothing is released")
	still, _ := s.GetItem(ctx, id)
	assert.NotNil(t, still)
}

// Deleting a folder releases what the files beneath it held, as the database
// finds them in the same transaction: the active file's bytes, not the pending
// upload's, whatever the caller last looked at.
func TestDeleteItemReleasingQuota_ReadsTheFilesInsideTheTransaction(t *testing.T) {
	s, ws, activeID := pendingFile(t)
	ctx := context.Background()
	require.NoError(t, s.ActivateFile(ctx, activeID, ws, 700))
	active, err := s.GetItem(ctx, activeID)
	require.NoError(t, err)

	// A second file under the same folder stays pending (never charged).
	key, size := "drive/pending/b.pdf", int64(300)
	pendingID := "f-" + uuid.NewString()
	require.NoError(t, s.InsertItem(ctx, &store.DriveItem{
		ID: pendingID, WorkspaceID: ws, DriveContext: "workspace", ParentID: active.ParentID, ItemType: "file",
		Name: "b.pdf", SizeBytes: &size, ObjectKey: &key, NGACNodeID: active.NGACNodeID, OwnerID: active.OwnerID, Status: "pending",
	}))

	removed, err := s.DeleteItemReleasingQuota(ctx, *active.ParentID)

	require.NoError(t, err)
	assert.Len(t, removed, 2, "both files are handed back so their stored objects can go")
	q, err := s.GetOrCreateQuota(ctx, ws)
	require.NoError(t, err)
	assert.EqualValues(t, 0, q.UsedBytes, "700 released; the pending 300 was never charged")
	assert.EqualValues(t, 0, q.UsedFiles)
}

// Reading a quota that exists takes no row lock, so a screen polling it does not
// queue behind a transaction that is changing it.
func TestGetOrCreateQuota_AReadDoesNotWaitForAWriter(t *testing.T) {
	s, ws, _ := pendingFile(t)
	ctx := context.Background()
	_, err := s.GetOrCreateQuota(ctx, ws)
	require.NoError(t, err)

	_, pool := newStore(t)
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { tx.Rollback(context.Background()) })
	_, err = tx.Exec(ctx, `SELECT 1 FROM drive_quotas WHERE workspace_id = $1 FOR UPDATE`, ws)
	require.NoError(t, err)

	readCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	_, err = s.GetOrCreateQuota(readCtx, ws)
	assert.NoError(t, err, "a read must not block on the row another transaction holds")
}
