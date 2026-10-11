package store_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/services/messaging/internal/store"
	"ngac-platform/testutil"
)

// driveWorld is a channel with a drive root, a sub-folder, and files placed in
// it and elsewhere.
type driveWorld struct {
	*threadWorld
	wsID string
}

func newDriveWorld(t *testing.T) *driveWorld {
	t.Helper()
	w := newThreadWorld(t, 1)
	wsID, _ := testutil.CreateWorkspace(t, w.pool, w.users[0])
	t.Cleanup(func() {
		w.pool.Exec(context.Background(), `DELETE FROM drive_items WHERE workspace_id = $1 AND parent_id IS NOT NULL`, wsID)
		w.pool.Exec(context.Background(), `DELETE FROM drive_items WHERE workspace_id = $1`, wsID)
	})
	return &driveWorld{threadWorld: w, wsID: wsID}
}

// item writes a drive item and returns its id.
func (d *driveWorld) item(t *testing.T, kind, parent, ctxKind, ctxID, status string, root bool) string {
	t.Helper()
	id := "di-" + uuid.NewString()
	var p any
	if parent != "" {
		p = parent
	}
	_, err := d.pool.Exec(context.Background(),
		`INSERT INTO drive_items (id, workspace_id, drive_context, drive_context_id, parent_id, item_type, name,
		   mime_type, size_bytes, ngac_node_id, owner_id, status, is_root)
		 VALUES ($1, $2, $3, NULLIF($4, ''), $5, $6, $7, 'image/png', 2048, 'node-x', $8, $9, $10)`,
		id, d.wsID, ctxKind, ctxID, p, kind, kind+"-"+id[:8]+".png", d.users[0], status, root)
	require.NoError(t, err)
	return id
}

func TestChannelDriveFiles_OnlyConfirmedFilesUnderTheChannelsOwnDrive(t *testing.T) {
	d := newDriveWorld(t)
	ctx := context.Background()
	root := d.item(t, "folder", "", "channel", d.chID, "active", true)
	sub := d.item(t, "folder", root, "channel", d.chID, "active", false)
	atRoot := d.item(t, "file", root, "channel", d.chID, "active", false)
	nested := d.item(t, "file", sub, "channel", d.chID, "active", false)
	pending := d.item(t, "file", sub, "channel", d.chID, "pending", false)
	otherRoot := d.item(t, "folder", "", "channel", "another-channel", "active", true)
	otherChannel := d.item(t, "file", otherRoot, "channel", "another-channel", "active", false)
	wsRoot := d.item(t, "folder", "", "workspace", "", "active", true)
	// A file that claims the channel's context but sits in the workspace drive.
	liar := d.item(t, "file", wsRoot, "channel", d.chID, "active", false)

	got, err := d.st.ChannelDriveFiles(ctx, d.chID, []string{atRoot, nested, pending, otherChannel, liar, sub, "no-such-file"})
	require.NoError(t, err)
	assert.Contains(t, got, atRoot)
	assert.Contains(t, got, nested)
	assert.Len(t, got, 2, "pending, other channel's, mislabelled, folder and unknown ids are all refused")
	assert.Equal(t, "image/png", got[atRoot].MimeType)
	assert.EqualValues(t, 2048, got[atRoot].SizeBytes)
}

func TestInsertMessage_StoresAttachmentsInOrderWithTheMessage(t *testing.T) {
	d := newDriveWorld(t)
	ctx := context.Background()
	m := &store.Message{ID: "m-" + uuid.NewString(), ChannelID: d.chID, SenderID: d.users[0], Content: "ba ảnh",
		MessageType: "user", CreatedAt: d.clock,
		Attachments: []store.Attachment{
			{DriveFileID: "f-2", FileName: "b.png", MimeType: "image/png", SizeBytes: 10, Width: 800, Height: 600},
			{DriveFileID: "f-1", FileName: "a.pdf", MimeType: "application/pdf", SizeBytes: 20},
		}}
	require.NoError(t, d.st.InsertMessage(ctx, m))

	got, err := d.st.ListAttachmentsForMessages(ctx, []string{m.ID})
	require.NoError(t, err)
	require.Len(t, got[m.ID], 2)
	assert.Equal(t, "f-2", got[m.ID][0].DriveFileID, "the order they were attached")
	assert.EqualValues(t, 800, got[m.ID][0].Width)
	assert.Equal(t, "a.pdf", got[m.ID][1].FileName)
}

func TestListAttachmentsForMessages_ReadsAnOlderLinkedDriveFileAsOneAttachment(t *testing.T) {
	d := newDriveWorld(t)
	ctx := context.Background()
	root := d.item(t, "folder", "", "channel", d.chID, "active", true)
	file := d.item(t, "file", root, "channel", d.chID, "active", false)
	m := &store.Message{ID: "m-" + uuid.NewString(), ChannelID: d.chID, SenderID: d.users[0], Content: "📎 old",
		MessageType: "user", LinkedEntityType: "drive_file", LinkedEntityID: file, CreatedAt: d.clock}
	require.NoError(t, d.st.InsertMessage(ctx, m))

	got, err := d.st.ListAttachmentsForMessages(ctx, []string{m.ID})
	require.NoError(t, err)
	require.Len(t, got[m.ID], 1)
	assert.Equal(t, file, got[m.ID][0].DriveFileID)
	assert.Equal(t, "image/png", got[m.ID][0].MimeType)
}
