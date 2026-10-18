package store_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/services/drive/internal/store"
)

// A pending file published next to an active item of the same name is refused
// with ErrNameTaken, so the caller can answer 409 instead of a generic failure,
// and nothing is published or charged.
func TestActivateFile_NameTakenIsItsOwnRefusal(t *testing.T) {
	s, ws, fileID := pendingFile(t)
	ctx := context.Background()
	item, err := s.GetItem(ctx, fileID)
	require.NoError(t, err)

	twin := "f-" + uuid.NewString()
	key := "drive/" + twin + "/a.pdf"
	size := int64(10)
	require.NoError(t, s.InsertItem(ctx, &store.DriveItem{
		ID: twin, WorkspaceID: ws, DriveContext: "workspace", ParentID: item.ParentID, ItemType: "file",
		Name: item.Name, SizeBytes: &size, ObjectKey: &key, NGACNodeID: item.NGACNodeID, OwnerID: item.OwnerID, Status: "active",
	}))
	t.Cleanup(func() { _ = s.DeleteItem(context.Background(), twin) })

	before, err := s.GetOrCreateQuota(ctx, ws)
	require.NoError(t, err)
	err = s.ActivateFile(ctx, fileID, ws, 10)
	assert.ErrorIs(t, err, store.ErrNameTaken)

	after, err := s.GetOrCreateQuota(ctx, ws)
	require.NoError(t, err)
	assert.Equal(t, before.UsedFiles, after.UsedFiles, "nothing charged")
	still, err := s.GetItem(ctx, fileID)
	require.NoError(t, err)
	assert.Equal(t, "pending", still.Status)
}

func TestNameInUse_ChecksTheFolderOrTheTopLevel(t *testing.T) {
	s, ws, fileID := pendingFile(t)
	ctx := context.Background()
	item, err := s.GetItem(ctx, fileID)
	require.NoError(t, err)

	// A pending file does not hold its name.
	used, err := s.NameInUse(ctx, ws, "workspace", "", item.ParentID, item.Name)
	require.NoError(t, err)
	assert.False(t, used)

	require.NoError(t, s.ActivateFile(ctx, fileID, ws, 10))
	used, err = s.NameInUse(ctx, ws, "workspace", "", item.ParentID, item.Name)
	require.NoError(t, err)
	assert.True(t, used, "an active file holds its name in its folder")

	// The same name at the top level of the drive is free.
	used, err = s.NameInUse(ctx, ws, "workspace", "", nil, item.Name)
	require.NoError(t, err)
	assert.False(t, used)
}
