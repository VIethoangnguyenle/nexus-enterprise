package grpc_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "ngac-platform/proto/drive"
)

// A copy writes a new file into the destination folder, so it needs the same
// right on that folder as creating or moving a file there, it is charged to the
// quota like an upload, and it lands in the destination's own workspace and
// drive whatever the request claims.

// copyFixture is the move fixture plus one confirmed file in B.
type copyFixture struct {
	*moveFixture
	file *pb.DriveItem
}

func newCopyFixture(t *testing.T) *copyFixture {
	t.Helper()
	f := newMoveFixture(t)
	created, err := f.srv.CreateFile(asCaller("u", actor), &pb.CreateFileRequest{
		WorkspaceId: f.wsID, Name: "report.txt", MimeType: "text/plain", SizeBytes: 1, ParentId: f.b.Id,
	})
	require.NoError(t, err)
	t.Cleanup(func() { cleanDriveItems(t, f.pool, created.FileId) })
	file, err := f.srv.ConfirmFile(asCaller("u", actor), &pb.ConfirmFileRequest{FileId: created.FileId})
	require.NoError(t, err)
	f.pw.reset()
	return &copyFixture{moveFixture: f, file: file}
}

// copyAs copies the fixture's file into dest the way the REST handler asks:
// only the item and the destination folder, nothing about where it lands.
func (f *copyFixture) copyAs(t *testing.T, dest string) (*pb.DriveItem, error) {
	t.Helper()
	out, err := f.srv.CopyItem(asCaller("u", actor), &pb.CopyItemRequest{ItemId: f.file.Id, DestParentId: dest})
	if out != nil {
		t.Cleanup(func() { cleanDriveItems(t, f.pool, out.Id) })
	}
	return out, err
}

// usage reads the workspace's quota counters.
func (f *copyFixture) usage(t *testing.T) (bytes int64, files int) {
	t.Helper()
	require.NoError(t, f.pool.QueryRow(context.Background(),
		`SELECT used_bytes, used_files FROM drive_quotas WHERE workspace_id = $1`, f.wsID).Scan(&bytes, &files))
	return bytes, files
}

func TestCopyItem_DeniedWithoutWriteOnDestination(t *testing.T) {
	f := newCopyFixture(t)
	f.pr.allow = func(_, object, op string) bool { return object != oaOf(f.other) }
	out, err := f.copyAs(t, f.other.Id)
	assert.Equal(t, codes.PermissionDenied, status.Code(err))
	assert.Nil(t, out, "no copy may be created")
}

func TestCopyItem_DeniedWithReadOnlyOnDestination(t *testing.T) {
	f := newCopyFixture(t)
	f.pr.allow = func(_, object, op string) bool { return object != oaOf(f.other) || op == "read" }
	_, err := f.copyAs(t, f.other.Id)
	assert.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestCopyItem_DeniedWithoutReadOnSource(t *testing.T) {
	f := newCopyFixture(t)
	f.pr.allow = func(_, object, op string) bool { return object != oaOf(f.b) }
	_, err := f.copyAs(t, f.other.Id)
	assert.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestCopyItem_IntoAnotherWorkspaceIsRefused(t *testing.T) {
	f := newCopyFixture(t)
	foreign := "drive-test-copy-foreign-" + f.wsID
	_, err := f.pool.Exec(context.Background(),
		`INSERT INTO workspaces (id, name, owner_id, type) VALUES ($1, $1, $2, 'organization') ON CONFLICT DO NOTHING`,
		foreign, getTestUserID(t, f.pool))
	require.NoError(t, err)
	t.Cleanup(func() {
		f.pool.Exec(context.Background(), `DELETE FROM drive_items WHERE workspace_id = $1`, foreign)
		f.pool.Exec(context.Background(), `DELETE FROM drive_quotas WHERE workspace_id = $1`, foreign)
		f.pool.Exec(context.Background(), `DELETE FROM workspaces WHERE id = $1`, foreign)
	})
	theirs, err := f.srv.CreateFolder(asMemberOf(t, f.pool, foreign), &pb.CreateFolderRequest{WorkspaceId: foreign, Name: "Theirs"})
	require.NoError(t, err)

	// Even with every right granted, a file never crosses into another workspace.
	out, err := f.copyAs(t, theirs.Id)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
	assert.Nil(t, out)
	var n int
	require.NoError(t, f.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM drive_items WHERE parent_id = $1`, theirs.Id).Scan(&n))
	assert.Zero(t, n, "nothing may land in the foreign folder")
}

func TestCopyItem_RequestCannotChooseWhereTheCopyLands(t *testing.T) {
	f := newCopyFixture(t)
	out, err := f.srv.CopyItem(asCaller("u", actor), &pb.CopyItemRequest{
		ItemId: f.file.Id, DestParentId: f.other.Id,
		DestWorkspaceId: "some-other-workspace", DestDriveContext: "channel", DestDriveContextId: "some-channel",
	})
	require.NoError(t, err)
	t.Cleanup(func() { cleanDriveItems(t, f.pool, out.Id) })
	assert.Equal(t, f.wsID, out.WorkspaceId, "the copy belongs to the destination folder's workspace")
	assert.Equal(t, f.other.DriveContext, out.DriveContext)
}

func TestCopyItem_LandsInTheDestinationsWorkspace(t *testing.T) {
	f := newCopyFixture(t)
	out, err := f.copyAs(t, f.other.Id)
	require.NoError(t, err)
	assert.Equal(t, f.wsID, out.WorkspaceId, "REST sends no workspace; the copy must not be orphaned")
	assert.Equal(t, f.other.Id, out.ParentId)
}

func TestCopyItem_RefusedDestinations(t *testing.T) {
	f := newCopyFixture(t)
	ctx := asCaller("", actor)

	// A file is not a folder.
	_, err := f.copyAs(t, f.file.Id)
	assert.Equal(t, codes.InvalidArgument, status.Code(err), "into a file")

	// A trashed folder is not a place to copy into.
	_, err = f.srv.TrashItem(ctx, &pb.TrashItemRequest{ItemId: f.other.Id})
	require.NoError(t, err)
	_, err = f.copyAs(t, f.other.Id)
	assert.Equal(t, codes.NotFound, status.Code(err), "into a trashed folder")

	// A trashed source is not there to copy.
	_, err = f.srv.RestoreItem(ctx, &pb.RestoreItemRequest{ItemId: f.other.Id})
	require.NoError(t, err)
	_, err = f.srv.TrashItem(ctx, &pb.TrashItemRequest{ItemId: f.file.Id})
	require.NoError(t, err)
	_, err = f.copyAs(t, f.other.Id)
	assert.Equal(t, codes.NotFound, status.Code(err), "a trashed source")
}

func TestCopyItem_IsChargedToTheQuota(t *testing.T) {
	f := newCopyFixture(t)
	beforeBytes, beforeFiles := f.usage(t)
	out, err := f.copyAs(t, f.other.Id)
	require.NoError(t, err)
	afterBytes, afterFiles := f.usage(t)
	assert.Equal(t, beforeFiles+1, afterFiles, "a copy is one more file")
	assert.Equal(t, beforeBytes+out.SizeBytes, afterBytes, "a copy counts its size")
	assert.Positive(t, out.SizeBytes)
}

func TestCopyItem_RefusedWhenTheQuotaIsFull(t *testing.T) {
	f := newCopyFixture(t)
	ctx := context.Background()
	var maxBytes int64
	require.NoError(t, f.pool.QueryRow(ctx, `SELECT max_bytes FROM drive_quotas WHERE workspace_id = $1`, f.wsID).Scan(&maxBytes))
	t.Cleanup(func() {
		f.pool.Exec(ctx, `UPDATE drive_quotas SET max_bytes = $2 WHERE workspace_id = $1`, f.wsID, maxBytes)
	})
	_, err := f.pool.Exec(ctx, `UPDATE drive_quotas SET max_bytes = used_bytes WHERE workspace_id = $1`, f.wsID)
	require.NoError(t, err)

	out, err := f.copyAs(t, f.other.Id)
	assert.Equal(t, codes.ResourceExhausted, status.Code(err))
	assert.Nil(t, out)
}

func TestCopyItem_AllowedWithReadOnSourceAndWriteOnDestination(t *testing.T) {
	f := newCopyFixture(t)
	f.pr.allow = func(_, object, op string) bool {
		if object == oaOf(f.b) {
			return op == "read"
		}
		return true
	}
	out, err := f.copyAs(t, f.other.Id)
	require.NoError(t, err)
	assert.Equal(t, f.file.Name, out.Name)
	assert.Equal(t, oaOf(f.other), out.NgacNodeId, "a file inherits the destination folder's node")
}

func TestCopyItem_PendingSourceIsNotFound(t *testing.T) {
	f := newCopyFixture(t)
	pending, err := f.srv.CreateFile(asCaller("u", actor), &pb.CreateFileRequest{
		WorkspaceId: f.wsID, Name: "half.txt", MimeType: "text/plain", SizeBytes: 1, ParentId: f.b.Id,
	})
	require.NoError(t, err)
	t.Cleanup(func() { cleanDriveItems(t, f.pool, pending.FileId) })
	_, err = f.srv.CopyItem(asCaller("u", actor), &pb.CopyItemRequest{ItemId: pending.FileId, DestParentId: f.other.Id})
	assert.Equal(t, codes.NotFound, status.Code(err), "an upload that is not confirmed is not there to copy")
}

// workspaceRoot is the drive root row of the fixture's workspace drive.
func (f *copyFixture) workspaceRoot(t *testing.T) string {
	t.Helper()
	var id string
	require.NoError(t, f.pool.QueryRow(context.Background(),
		`SELECT id FROM drive_items WHERE workspace_id = $1 AND drive_context = 'workspace' AND is_root AND status = 'active'`,
		f.wsID).Scan(&id))
	return id
}

func TestCopyItem_EmptyDestinationIsTheTopOfTheSourcesDrive(t *testing.T) {
	f := newCopyFixture(t)
	out, err := f.copyAs(t, "")
	require.NoError(t, err)
	assert.Empty(t, out.ParentId, "top-level items have no parent row")
	assert.Equal(t, f.file.DriveContext, out.DriveContext)
	assert.Equal(t, f.file.DriveContextId, out.DriveContextId, "a top-level file carries its drive's context like an upload does")
	assert.Equal(t, f.wsID, out.WorkspaceId)
}

func TestCopyItem_IntoTheDriveRootLandsAtTheTopLevel(t *testing.T) {
	f := newCopyFixture(t)
	out, err := f.copyAs(t, f.workspaceRoot(t))
	require.NoError(t, err)
	assert.Empty(t, out.ParentId, "the root is the top level, not a folder to sit under")
	assert.Equal(t, f.file.DriveContextId, out.DriveContextId, "not the root's own context id")
}

func TestCopyItem_DuplicateNameGetsACopyName(t *testing.T) {
	f := newCopyFixture(t)
	first, err := f.copyAs(t, f.b.Id)
	require.NoError(t, err)
	assert.Equal(t, "Bản sao của report.txt", first.Name)
	second, err := f.copyAs(t, f.b.Id)
	require.NoError(t, err)
	assert.Equal(t, "Bản sao của report.txt (2)", second.Name)

	// Into a folder where the name is free, the copy keeps it.
	third, err := f.copyAs(t, f.other.Id)
	require.NoError(t, err)
	assert.Equal(t, "report.txt", third.Name)
}

func TestCopyItem_DuplicateNameAtTheTopLevelGetsACopyName(t *testing.T) {
	f := newCopyFixture(t)
	first, err := f.copyAs(t, "")
	require.NoError(t, err)
	second, err := f.copyAs(t, "")
	require.NoError(t, err)
	assert.Equal(t, "report.txt", first.Name)
	assert.Equal(t, "Bản sao của report.txt", second.Name, "top-level siblings are siblings too")
}
