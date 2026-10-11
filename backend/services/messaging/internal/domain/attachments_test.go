package domain_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/services/messaging/internal/domain"
)

// driveItem writes a drive item in the fixture's workspace and returns its id.
func (k *talk) driveItem(t *testing.T, kind, parent, ctxKind, ctxID, status, mime string, root bool) string {
	t.Helper()
	id := "di-" + uuid.NewString()
	var p any
	if parent != "" {
		p = parent
	}
	_, err := k.f.pool.Exec(context.Background(),
		`INSERT INTO drive_items (id, workspace_id, drive_context, drive_context_id, parent_id, item_type, name,
		   mime_type, size_bytes, ngac_node_id, owner_id, status, is_root)
		 VALUES ($1, $2, $3, NULLIF($4, ''), $5, $6, $7, $8, 4096, 'node-x', $9, $10, $11)`,
		id, k.f.wsID, ctxKind, ctxID, p, kind, fmt.Sprintf("%s-%s", kind, id[:8]), mime, k.f.userID, status, root)
	require.NoError(t, err)
	t.Cleanup(func() { k.f.pool.Exec(context.Background(), `DELETE FROM drive_items WHERE id = $1`, id) })
	return id
}

func attach(ids ...string) []domain.AttachmentInput {
	out := make([]domain.AttachmentInput, 0, len(ids))
	for _, id := range ids {
		out = append(out, domain.AttachmentInput{DriveFileID: id, Width: 640, Height: 480})
	}
	return out
}

func TestSendMessage_TextAndSeveralFilesAreOneMessage(t *testing.T) {
	k := newTalk(t, "attach")
	ctx := context.Background()
	root := k.driveItem(t, "folder", "", "channel", k.chID, "active", "", true)
	a := k.driveItem(t, "file", root, "channel", k.chID, "active", "image/png", false)
	b := k.driveItem(t, "file", root, "channel", k.chID, "active", "image/jpeg", false)
	c := k.driveItem(t, "file", root, "channel", k.chID, "active", "application/pdf", false)
	t.Cleanup(func() { k.f.pool.Exec(ctx, `DELETE FROM drive_items WHERE parent_id = $1`, root) })

	msg, err := k.f.svc.SendMessage(ctx, domain.SendMessageInput{
		ChannelID: k.chID, SenderID: k.f.userID, SenderNodeID: userNode, Content: "báo cáo tuần",
		Attachments: attach(a, b, c),
	})
	require.NoError(t, err)
	require.Len(t, msg.Attachments, 3)
	assert.Equal(t, a, msg.Attachments[0].DriveFileId)
	assert.Equal(t, "image/png", msg.Attachments[0].MimeType)
	assert.EqualValues(t, 640, msg.Attachments[0].Width)
	assert.EqualValues(t, 4096, msg.Attachments[2].SizeBytes)

	list, err := k.f.svc.GetMessages(ctx, k.chID, k.f.userID, userNode, "", 20)
	require.NoError(t, err)
	var found bool
	for _, m := range list.Messages {
		if m.Id == msg.Id {
			found = true
			assert.Len(t, m.Attachments, 3, "listed with its files")
		}
	}
	assert.True(t, found)

	// Files alone, without text, are a message too.
	_, err = k.f.svc.SendMessage(ctx, domain.SendMessageInput{
		ChannelID: k.chID, SenderID: k.f.userID, SenderNodeID: userNode, Attachments: attach(a),
	})
	require.NoError(t, err)
}

// A file can be attached only from the channel's own drive, confirmed. A file
// of another channel or of the workspace drive would hand its contents to this
// channel's readers.
func TestSendMessage_RefusesFilesOutsideTheChannelsDrive(t *testing.T) {
	k := newTalk(t, "attach-deny")
	ctx := context.Background()
	root := k.driveItem(t, "folder", "", "channel", k.chID, "active", "", true)
	ok := k.driveItem(t, "file", root, "channel", k.chID, "active", "image/png", false)
	pending := k.driveItem(t, "file", root, "channel", k.chID, "pending", "image/png", false)
	otherRoot := k.driveItem(t, "folder", "", "channel", "another-channel", "active", "", true)
	foreign := k.driveItem(t, "file", otherRoot, "channel", "another-channel", "active", "image/png", false)
	wsRoot := k.driveItem(t, "folder", "", "workspace", "", "active", "", true)
	workspaceFile := k.driveItem(t, "file", wsRoot, "workspace", "", "active", "image/png", false)
	t.Cleanup(func() {
		k.f.pool.Exec(ctx, `DELETE FROM drive_items WHERE parent_id IN ($1, $2, $3)`, root, otherRoot, wsRoot)
	})

	count := func() int {
		var n int
		require.NoError(t, k.f.pool.QueryRow(ctx, `SELECT count(*) FROM messages WHERE channel_id = $1`, k.chID).Scan(&n))
		return n
	}
	before := count()
	for name, files := range map[string][]domain.AttachmentInput{
		"another channel's file": attach(ok, foreign),
		"a workspace file":       attach(workspaceFile),
		"an unconfirmed upload":  attach(pending),
		"an unknown id":          attach("no-such-file"),
		"the same file twice":    attach(ok, ok),
	} {
		_, err := k.f.svc.SendMessage(ctx, domain.SendMessageInput{
			ChannelID: k.chID, SenderID: k.f.userID, SenderNodeID: userNode, Content: "x", Attachments: files,
		})
		require.ErrorIs(t, err, domain.ErrInvalidInput, name)
	}
	assert.Equal(t, before, count(), "a refused message stores nothing")

	tooMany := make([]string, 21)
	for i := range tooMany {
		tooMany[i] = ok
	}
	_, err := k.f.svc.SendMessage(ctx, domain.SendMessageInput{
		ChannelID: k.chID, SenderID: k.f.userID, SenderNodeID: userNode, Attachments: attach(tooMany...),
	})
	require.ErrorIs(t, err, domain.ErrInvalidInput, "at most 20 files")

	_, err = k.f.svc.SendMessage(ctx, domain.SendMessageInput{
		ChannelID: k.chID, SenderID: k.f.userID, SenderNodeID: userNode, Content: "   ",
	})
	require.ErrorIs(t, err, domain.ErrInvalidInput, "neither text nor files")

	// Write is checked first: an outsider learns nothing about which files exist.
	_, err = k.f.svc.SendMessage(ctx, domain.SendMessageInput{
		ChannelID: k.chID, SenderID: k.f.userID, SenderNodeID: outsider, Content: "x", Attachments: attach(foreign),
	})
	require.ErrorIs(t, err, domain.ErrAccessDenied)
}
