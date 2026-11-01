package grpc_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"ngac-platform/ngac"
	docpb "ngac-platform/proto/document"
	pb "ngac-platform/proto/drive"
	policypb "ngac-platform/proto/policy"
)

// The boundaries of a drive: its root is not an item anyone edits, nothing is
// created in a drive before the caller is known to be allowed to, a file share
// reaches the file and not its folder, the quota holds for what was actually
// uploaded, and a trashed file is not downloadable.

// linkedRead is the fake policy reader that also knows every node recWrite
// created: recWrite names a node's ID "id:" + its name, so GetNode can answer
// for it the way the policy service would.
type linkedRead struct{ *fakePolicyRead }

func (l linkedRead) GetNode(ctx context.Context, req *policypb.GetNodeRequest, opts ...grpc.CallOption) (*policypb.NGACNode, error) {
	if name, ok := strings.CutPrefix(req.NodeId, "id:"); ok {
		return &policypb.NGACNode{Id: req.NodeId, Name: name, NodeType: ngac.TypeOA}, nil
	}
	return l.fakePolicyRead.GetNode(ctx, req, opts...)
}

type boundaryFixture struct {
	pr   *fakePolicyRead
	pw   *recWrite
	srv  pb.DriveServiceServer
	pool *pgxpool.Pool
	wsID string
}

func newBoundaryFixture(t *testing.T) *boundaryFixture {
	t.Helper()
	pr, pw := newFakePolicyRead(), &recWrite{}
	srv, pool := newServerWith(t, linkedRead{pr}, pw)
	return &boundaryFixture{pr: pr, pw: pw, srv: srv, pool: pool, wsID: getTestWorkspaceID(t, pool)}
}

func (f *boundaryFixture) folder(t *testing.T, name, parent string) *pb.DriveItem {
	t.Helper()
	it, err := f.srv.CreateFolder(asCaller("u", actor), &pb.CreateFolderRequest{WorkspaceId: f.wsID, Name: name, ParentId: parent})
	require.NoError(t, err)
	t.Cleanup(func() { cleanDriveItems(t, f.pool, it.Id) })
	return it
}

// file uploads and confirms a file in parent.
func (f *boundaryFixture) file(t *testing.T, name, parent string) *pb.DriveItem {
	t.Helper()
	created, err := f.srv.CreateFile(asCaller("u", actor), &pb.CreateFileRequest{
		WorkspaceId: f.wsID, Name: name, MimeType: "text/plain", SizeBytes: 1, ParentId: parent,
	})
	require.NoError(t, err)
	t.Cleanup(func() { cleanDriveItems(t, f.pool, created.FileId) })
	it, err := f.srv.ConfirmFile(asCaller("u", actor), &pb.ConfirmFileRequest{FileId: created.FileId})
	require.NoError(t, err)
	return it
}

func (f *boundaryFixture) rootID(t *testing.T) string {
	t.Helper()
	var id string
	require.NoError(t, f.pool.QueryRow(context.Background(),
		`SELECT id FROM drive_items WHERE workspace_id = $1 AND drive_context = 'workspace' AND is_root AND status = 'active'`,
		f.wsID).Scan(&id))
	return id
}

func driveRows(t *testing.T, pool *pgxpool.Pool, wsID string) int {
	t.Helper()
	var n int
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT count(*) FROM drive_items WHERE workspace_id = $1`, wsID).Scan(&n))
	return n
}

func itemStatus(t *testing.T, pool *pgxpool.Pool, id string) string {
	t.Helper()
	var st string
	require.NoError(t, pool.QueryRow(context.Background(), `SELECT status FROM drive_items WHERE id = $1`, id).Scan(&st))
	return st
}

// withoutDocumentsOA makes the test's workspace one that records no Documents OA
// (one created outside the workspace service).
func withoutDocumentsOA(t *testing.T, pool *pgxpool.Pool, wsID string) {
	t.Helper()
	_, err := pool.Exec(context.Background(), `UPDATE workspaces SET documents_oa_id = NULL WHERE id = $1`, wsID)
	require.NoError(t, err)
}

// asMemberOf makes the test's user an active member of wsID and returns a
// context calling as them.
func asMemberOf(t *testing.T, pool *pgxpool.Pool, wsID string) context.Context {
	t.Helper()
	userID := getTestUserID(t, pool)
	_, err := pool.Exec(context.Background(),
		`INSERT INTO tenant_users (tenant_id, user_id, role, status) VALUES ($1, $2, 'member', 'active') ON CONFLICT DO NOTHING`,
		wsID, userID)
	require.NoError(t, err)
	return asCaller(userID, actor)
}

// ---------------------------------------------------------------------------
// The drive root
// ---------------------------------------------------------------------------

func TestDriveRoot_CannotBeRenamedTrashedOrDeleted(t *testing.T) {
	f := newBoundaryFixture(t)
	top := f.folder(t, "Top", "")
	root := f.rootID(t)
	f.pw.reset()
	ctx := asCaller("u", actor)

	_, err := f.srv.RenameItem(ctx, &pb.RenameItemRequest{ItemId: root, NewName: "Mine"})
	assert.Equal(t, codes.InvalidArgument, status.Code(err), "rename")
	_, err = f.srv.TrashItem(ctx, &pb.TrashItemRequest{ItemId: root})
	assert.Equal(t, codes.InvalidArgument, status.Code(err), "trash")
	_, err = f.srv.DeleteItem(ctx, &pb.DeleteItemRequest{ItemId: root})
	assert.Equal(t, codes.InvalidArgument, status.Code(err), "permanent delete")
	_, err = f.srv.MoveItem(ctx, &pb.MoveItemRequest{ItemId: root, NewParentId: top.Id})
	assert.Equal(t, codes.InvalidArgument, status.Code(err), "move into a folder")

	assert.Equal(t, "active", itemStatus(t, f.pool, root))
	assert.Equal(t, "active", itemStatus(t, f.pool, top.Id), "nothing under the root went with it")
	got, err := f.srv.GetItem(ctx, &pb.GetItemRequest{ItemId: root})
	require.NoError(t, err)
	assert.Equal(t, "Root", got.Name)
	assert.Empty(t, f.pw.snapshot(), "no policy write, least of all deleting the Documents OA")
}

func TestChannelDriveRoot_IsTrashedOnlyByTheGroupsCreator(t *testing.T) {
	f := newBoundaryFixture(t)
	creator := getTestUserID(t, f.pool)
	chID := insertTestChannel(t, f.pool, "group", f.wsID)
	_, err := f.pool.Exec(context.Background(), `UPDATE channels SET created_by = $1 WHERE id = $2`, creator, chID)
	require.NoError(t, err)
	cleanChannelDrives(t, f.pool, chID)
	drive, err := f.srv.CreateDriveForChannel(context.Background(), channelDrive(f.wsID, chID, "group"))
	require.NoError(t, err)

	// Deny: another member with write on the drive.
	_, err = f.srv.TrashItem(asCaller("another-member", actor), &pb.TrashItemRequest{ItemId: drive.Id})
	assert.Equal(t, codes.PermissionDenied, status.Code(err))
	assert.Equal(t, "active", itemStatus(t, f.pool, drive.Id))
	_, err = f.srv.RenameItem(asCaller(creator, actor), &pb.RenameItemRequest{ItemId: drive.Id, NewName: "x"})
	assert.Equal(t, codes.InvalidArgument, status.Code(err), "not even the creator renames a drive root")
	_, err = f.srv.DeleteItem(asCaller(creator, actor), &pb.DeleteItemRequest{ItemId: drive.Id})
	assert.Equal(t, codes.InvalidArgument, status.Code(err), "nor deletes it for good")

	// Allow: the creator disbanding their group moves its drive to the trash.
	_, err = f.srv.TrashItem(asCaller(creator, actor), &pb.TrashItemRequest{ItemId: drive.Id})
	require.NoError(t, err)
	assert.Equal(t, "trashed", itemStatus(t, f.pool, drive.Id))
}

func TestDeleteFolder_NeverDeletesAnOAItDoesNotOwn(t *testing.T) {
	f := newBoundaryFixture(t)
	docs := getTestDocumentsOA(t)
	// A folder row that is not a root but points at the workspace's Documents OA
	// (as a bad write or an old bug could leave one).
	id := uuid.New().String()
	_, err := f.pool.Exec(context.Background(),
		`INSERT INTO drive_items (id, workspace_id, drive_context, item_type, name, ngac_node_id, owner_id, status)
		 VALUES ($1, $2, 'workspace', 'folder', 'Docs', $3, 'u', 'active')`, id, f.wsID, docs)
	require.NoError(t, err)

	_, err = f.srv.DeleteItem(asCaller("u", actor), &pb.DeleteItemRequest{ItemId: id})
	require.NoError(t, err)
	assert.False(t, f.pw.has("delete "+docs), "an OA another row or the workspace still uses is never deleted: %v", f.pw.snapshot())
}

func TestDeleteFolder_StillDeletesItsOwnOA(t *testing.T) {
	f := newBoundaryFixture(t)
	a := f.folder(t, "A", "")
	f.pw.reset()
	_, err := f.srv.DeleteItem(asCaller("u", actor), &pb.DeleteItemRequest{ItemId: a.Id})
	require.NoError(t, err)
	assert.Equal(t, []string{"delete " + oaOf(a)}, f.pw.snapshot())
}

// The workspace drive's root is the Documents OA every member writes into; a
// listing must not hand out its ID. (A channel drive's root is listed: clients
// upload into it, and it cannot be renamed, trashed by a member or deleted.)
func TestListRoot_WorkspaceListingIgnoresTheContextIDAndNeverListsTheRoot(t *testing.T) {
	f := newBoundaryFixture(t)
	top := f.folder(t, "Top", "")
	root := f.rootID(t)

	for _, ctxID := range []string{"", f.wsID, "anything"} {
		list, err := f.srv.ListFolder(asCaller("u", actor), &pb.ListFolderRequest{
			WorkspaceId: f.wsID, DriveContext: "workspace", DriveContextId: ctxID,
		})
		require.NoError(t, err)
		var ids []string
		for _, it := range list.Items {
			ids = append(ids, it.Id)
		}
		assert.NotContains(t, ids, root, "drive_context_id=%q", ctxID)
		assert.Contains(t, ids, top.Id, "drive_context_id=%q", ctxID)
	}
}

// ---------------------------------------------------------------------------
// Nothing is created in a drive before the caller is authorized
// ---------------------------------------------------------------------------

func TestTopLevelCreation_WithoutRightsOnTheDriveCreatesNothing(t *testing.T) {
	f := newBoundaryFixture(t)
	docs := getTestDocumentsOA(t)
	// The caller holds nothing on this workspace's Documents OA (another
	// tenant's user, say).
	f.pr.allow = func(_, object, _ string) bool { return object != docs }
	ctx := asCaller("outsider", "ngac-outsider")

	_, err := f.srv.CreateFolder(ctx, &pb.CreateFolderRequest{WorkspaceId: f.wsID, Name: "Spam"})
	assert.Equal(t, codes.PermissionDenied, status.Code(err), "folder")
	_, err = f.srv.CreateFile(ctx, &pb.CreateFileRequest{WorkspaceId: f.wsID, Name: "x.html", SizeBytes: 1})
	assert.Equal(t, codes.PermissionDenied, status.Code(err), "file")
	_, err = f.srv.GetQuota(ctx, &pb.GetQuotaRequest{WorkspaceId: f.wsID})
	assert.Equal(t, codes.PermissionDenied, status.Code(err), "quota")

	assert.Zero(t, driveRows(t, f.pool, f.wsID), "no root, folder or file row")
	assert.Empty(t, f.pw.snapshot(), "no node or edge")
}

func TestCreateFolder_TopLevelNeedsWriteOnAnExistingRoot(t *testing.T) {
	f := newBoundaryFixture(t)
	f.folder(t, "First", "") // the root exists from here on
	docs := getTestDocumentsOA(t)
	f.pr.allow = func(_, object, op string) bool { return !(object == docs && op == ngac.OpWrite) }
	f.pw.reset()

	_, err := f.srv.CreateFolder(asCaller("reader", "ngac-reader"), &pb.CreateFolderRequest{WorkspaceId: f.wsID, Name: "Second"})
	assert.Equal(t, codes.PermissionDenied, status.Code(err))
	assert.Empty(t, f.pw.snapshot())
}

func TestDriveRoot_AChannelDriveIsNeverMadeOnTheFly(t *testing.T) {
	f := newBoundaryFixture(t)
	chID := insertTestChannel(t, f.pool, "nodrive", f.wsID)
	cleanChannelDrives(t, f.pool, chID)

	_, err := f.srv.CreateFolder(asCaller("u", actor), &pb.CreateFolderRequest{
		WorkspaceId: f.wsID, Name: "X", DriveContext: "channel", DriveContextId: chID,
	})
	assert.Equal(t, codes.NotFound, status.Code(err))
	assert.Zero(t, driveRows(t, f.pool, f.wsID), "no root bound to the Documents OA for CreateDriveForChannel to adopt")
	assert.Empty(t, f.pw.snapshot())
}

func TestDriveRoot_WithoutDocumentsOAANonMemberCreatesNothing(t *testing.T) {
	f := newBoundaryFixture(t)
	withoutDocumentsOA(t, f.pool, f.wsID)

	_, err := f.srv.CreateFolder(asCaller("not-a-member", actor), &pb.CreateFolderRequest{WorkspaceId: f.wsID, Name: "X"})
	assert.Equal(t, codes.PermissionDenied, status.Code(err))
	assert.Zero(t, driveRows(t, f.pool, f.wsID))
	assert.Empty(t, f.pw.snapshot(), "no DriveRoot OA made for a stranger")
}

// ---------------------------------------------------------------------------
// A file share reaches the file only
// ---------------------------------------------------------------------------

func fileOA(it *pb.DriveItem) string { return "id:" + ngac.FileOAName(ngac.FileID(it.Id)) }

func (f *boundaryFixture) shareWithRole(t *testing.T, itemID string) {
	t.Helper()
	if _, ok := f.pr.nodes["role-ua"]; !ok {
		f.pr.add("role-ua", "Role_reviewers", ngac.TypeUA)
	}
	_, err := f.srv.CreateShare(asCaller("u", actor), &pb.CreateShareRequest{
		ItemId: itemID, ShareType: "role", TargetNgacNodeId: "role-ua", Operations: []string{ngac.SharePermissionRead},
	})
	require.NoError(t, err)
}

func TestCreateShare_AFileGetsItsOwnOAAndOnlyThatGoesUnderTheShare(t *testing.T) {
	f := newBoundaryFixture(t)
	folder := f.folder(t, "Private", "")
	file := f.file(t, "report.pdf", folder.Id)
	f.pw.reset()

	f.shareWithRole(t, file.Id)

	calls := f.pw.snapshot()
	assert.Contains(t, calls, "node OA "+ngac.FileOAName(ngac.FileID(file.Id)))
	assert.Contains(t, calls, fmt.Sprintf("assign %s>%s", fileOA(file), oaOf(folder)), "the file OA inherits its folder")
	assert.True(t, f.pw.has(fmt.Sprintf("assign %s>id:Share_", fileOA(file))), "the file OA is what is shared: %v", calls)
	// Deny: the folder's OA is never put under the share, or the target would
	// reach every file in it.
	assert.False(t, f.pw.has(fmt.Sprintf("assign %s>id:Share_", oaOf(folder))), "%v", calls)

	got, err := f.srv.GetItem(asCaller("u", actor), &pb.GetItemRequest{ItemId: file.Id})
	require.NoError(t, err)
	assert.Equal(t, fileOA(file), got.NgacNodeId, "the file is authorized on its own OA from now on")
}

func TestCreateShare_ASecondShareOfAFileReusesItsOA(t *testing.T) {
	f := newBoundaryFixture(t)
	folder := f.folder(t, "Private", "")
	file := f.file(t, "report.pdf", folder.Id)
	f.shareWithRole(t, file.Id)
	f.pw.reset()

	f.shareWithRole(t, file.Id)

	assert.False(t, f.pw.has("node OA "+ngac.FileOAName(ngac.FileID(file.Id))), "%v", f.pw.snapshot())
	assert.False(t, f.pw.has(fmt.Sprintf("assign %s>%s", fileOA(file), oaOf(folder))))
	assert.True(t, f.pw.has(fmt.Sprintf("assign %s>id:Share_", fileOA(file))))
}

func TestCreateShare_AFolderShareStillWrapsTheFolder(t *testing.T) {
	f := newBoundaryFixture(t)
	folder := f.folder(t, "Shared", "")
	f.pw.reset()

	f.shareWithRole(t, folder.Id)

	assert.True(t, f.pw.has(fmt.Sprintf("assign %s>id:Share_", oaOf(folder))))
	assert.False(t, f.pw.has("node OA File_"))
}

func TestMoveItem_ASharedFileTakesItsOAAlong(t *testing.T) {
	f := newBoundaryFixture(t)
	from, to := f.folder(t, "From", ""), f.folder(t, "To", "")
	file := f.file(t, "report.pdf", from.Id)
	f.shareWithRole(t, file.Id)
	f.pw.reset()

	moved, err := f.srv.MoveItem(asCaller("u", actor), &pb.MoveItemRequest{ItemId: file.Id, NewParentId: to.Id})
	require.NoError(t, err)
	assert.Equal(t, fileOA(file), moved.NgacNodeId)
	assert.Equal(t, []string{
		fmt.Sprintf("assign %s>%s", fileOA(file), oaOf(to)),
		fmt.Sprintf("unassign %s>%s", fileOA(file), oaOf(from)),
	}, f.pw.snapshot())
}

func TestMoveItem_AnUnsharedFileStillTakesTheDestinationsOA(t *testing.T) {
	f := newBoundaryFixture(t)
	from, to := f.folder(t, "From", ""), f.folder(t, "To", "")
	file := f.file(t, "report.pdf", from.Id)
	f.pw.reset()

	moved, err := f.srv.MoveItem(asCaller("u", actor), &pb.MoveItemRequest{ItemId: file.Id, NewParentId: to.Id})
	require.NoError(t, err)
	assert.Equal(t, oaOf(to), moved.NgacNodeId)
	assert.Empty(t, f.pw.snapshot())
}

func TestDeleteItem_ASharedFileTakesOnlyItsOwnOA(t *testing.T) {
	f := newBoundaryFixture(t)
	folder := f.folder(t, "Private", "")
	shared, plain := f.file(t, "a.pdf", folder.Id), f.file(t, "b.pdf", folder.Id)
	f.shareWithRole(t, shared.Id)
	f.pw.reset()

	_, err := f.srv.DeleteItem(asCaller("u", actor), &pb.DeleteItemRequest{ItemId: plain.Id})
	require.NoError(t, err)
	assert.Empty(t, f.pw.snapshot(), "an unshared file has no node to delete; its folder's stays")

	_, err = f.srv.DeleteItem(asCaller("u", actor), &pb.DeleteItemRequest{ItemId: shared.Id})
	require.NoError(t, err)
	assert.Equal(t, []string{"delete " + fileOA(shared)}, f.pw.snapshot())
}

func TestDeleteFolder_RemovesTheOAsOfSharedFilesInIt(t *testing.T) {
	f := newBoundaryFixture(t)
	folder := f.folder(t, "Private", "")
	shared := f.file(t, "a.pdf", folder.Id)
	f.shareWithRole(t, shared.Id)
	f.pw.reset()

	_, err := f.srv.DeleteItem(asCaller("u", actor), &pb.DeleteItemRequest{ItemId: folder.Id})
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"delete " + fileOA(shared), "delete " + oaOf(folder)}, f.pw.snapshot())
}

// ---------------------------------------------------------------------------
// The quota holds for the bytes actually stored
// ---------------------------------------------------------------------------

func TestConfirmFile_OverTheQuotaIsRefusedAndTheUploadRemoved(t *testing.T) {
	for _, tc := range []struct {
		name, limit string
	}{
		{"bytes", `max_bytes = used_bytes + 10`},
		{"files", `max_files = used_files`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, pool := setupServer(t)
			storage := &recordingDocStorage{}
			srv := newDrive(pool, &mockPolicyRead{}, &mockPolicyWrite{}, storage)
			wsID := getTestWorkspaceID(t, pool)
			ctx := asCaller("u", actor)

			// Declares one byte; the store will report 1024 (mockDocStorage).
			created, err := srv.CreateFile(ctx, &pb.CreateFileRequest{WorkspaceId: wsID, Name: "big.bin", SizeBytes: 1})
			require.NoError(t, err)
			t.Cleanup(func() { cleanDriveItems(t, pool, created.FileId) })
			_, err = pool.Exec(context.Background(), `UPDATE drive_quotas SET `+tc.limit+` WHERE workspace_id = $1`, wsID)
			require.NoError(t, err)
			var usedBefore int64
			require.NoError(t, pool.QueryRow(context.Background(),
				`SELECT used_bytes FROM drive_quotas WHERE workspace_id = $1`, wsID).Scan(&usedBefore))

			_, err = srv.ConfirmFile(ctx, &pb.ConfirmFileRequest{FileId: created.FileId})
			assert.Equal(t, codes.ResourceExhausted, status.Code(err))

			var rows int
			require.NoError(t, pool.QueryRow(context.Background(), `SELECT count(*) FROM drive_items WHERE id = $1`, created.FileId).Scan(&rows))
			assert.Zero(t, rows, "the upload is not left pending")
			assert.Contains(t, storage.removedKeys(), created.ObjectKey, "the stored bytes are removed")
			var usedAfter int64
			require.NoError(t, pool.QueryRow(context.Background(),
				`SELECT used_bytes FROM drive_quotas WHERE workspace_id = $1`, wsID).Scan(&usedAfter))
			assert.Equal(t, usedBefore, usedAfter, "nothing is charged")
		})
	}
}

// ---------------------------------------------------------------------------
// Downloads and copies
// ---------------------------------------------------------------------------

func TestGetDownloadURL_TrashedFileOrFolderIsNotFound(t *testing.T) {
	f := newBoundaryFixture(t)
	folder := f.folder(t, "F", "")
	file := f.file(t, "a.pdf", folder.Id)
	ctx := asCaller("u", actor)

	_, err := f.srv.GetDownloadURL(ctx, &pb.GetDownloadURLRequest{FileId: file.Id})
	require.NoError(t, err, "precondition: an active file downloads")

	_, err = f.srv.TrashItem(ctx, &pb.TrashItemRequest{ItemId: file.Id})
	require.NoError(t, err)
	_, err = f.srv.GetDownloadURL(ctx, &pb.GetDownloadURLRequest{FileId: file.Id})
	assert.Equal(t, codes.NotFound, status.Code(err), "trashed")

	_, err = f.srv.GetDownloadURL(ctx, &pb.GetDownloadURLRequest{FileId: folder.Id})
	assert.Equal(t, codes.NotFound, status.Code(err), "a folder has nothing to download")
}

func TestCopyItem_ToTheTopLevelNeedsWriteOnTheRoot(t *testing.T) {
	f := newCopyFixture(t)
	docs := getTestDocumentsOA(t)
	f.pr.allow = func(_, object, op string) bool { return !(object == docs && op == ngac.OpWrite) }

	out, err := f.copyAs(t, "")
	assert.Equal(t, codes.PermissionDenied, status.Code(err))
	assert.Nil(t, out)
}

// uploadSlots remembers the size each upload URL was asked for.
type uploadSlots struct {
	mockDocStorage
	sizes []*int64
}

func (u *uploadSlots) GetUploadURL(ctx context.Context, req *docpb.GetUploadURLRequest, opts ...grpc.CallOption) (*docpb.GetUploadURLResponse, error) {
	u.sizes = append(u.sizes, req.SizeBytes)
	return u.mockDocStorage.GetUploadURL(ctx, req, opts...)
}

func TestCreateFile_TheUploadURLIsBoundToTheDeclaredSize(t *testing.T) {
	_, pool := setupServer(t)
	slots := &uploadSlots{}
	srv := newDrive(pool, &mockPolicyRead{}, &mockPolicyWrite{}, slots)
	wsID := getTestWorkspaceID(t, pool)

	for _, size := range []int64{0, 4096} {
		created, err := srv.CreateFile(asCaller("u", actor), &pb.CreateFileRequest{WorkspaceId: wsID, Name: fmt.Sprintf("f%d.bin", size), SizeBytes: size})
		require.NoError(t, err)
		t.Cleanup(func() { cleanDriveItems(t, pool, created.FileId) })
	}
	require.Len(t, slots.sizes, 2)
	for i, want := range []int64{0, 4096} {
		require.NotNil(t, slots.sizes[i], "a person's upload always names its size, 0 included")
		assert.Equal(t, want, *slots.sizes[i])
	}

	// Deny: a negative size is refused before anything is stored or signed.
	_, err := srv.CreateFile(asCaller("u", actor), &pb.CreateFileRequest{WorkspaceId: wsID, Name: "neg.bin", SizeBytes: -1})
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
	assert.Len(t, slots.sizes, 2)
}

// Deny: a drive root is the whole drive. Sharing it, with anyone and with any
// permission, would hand over everything in it, so it is refused before
// anything is written, for a workspace root and a channel root alike.
func TestCreateShare_ADriveRootIsNeverShared(t *testing.T) {
	f := newBoundaryFixture(t)
	f.folder(t, "Top", "")
	wsRoot := f.rootID(t)
	chID := insertTestChannel(t, f.pool, "group", f.wsID)
	cleanChannelDrives(t, f.pool, chID)
	chRoot, err := f.srv.CreateDriveForChannel(context.Background(), channelDrive(f.wsID, chID, "group"))
	require.NoError(t, err)
	f.pr.add(ngac.NodePublicUsers, ngac.NodePublicUsers, ngac.TypeUA)
	f.pr.add("role-ua", "Role_reviewers", ngac.TypeUA)
	f.pr.add("person", "U_person", ngac.TypeU)
	f.pw.reset()

	for _, root := range []string{wsRoot, chRoot.Id} {
		for _, req := range []*pb.CreateShareRequest{
			{ShareType: "public", Operations: []string{ngac.SharePermissionWrite}},
			{ShareType: "public", Operations: []string{ngac.SharePermissionRead}},
			{ShareType: "user", TargetNgacNodeId: "person", Operations: []string{ngac.SharePermissionWrite}},
			{ShareType: "role", TargetNgacNodeId: "role-ua", Operations: []string{ngac.SharePermissionRead}},
		} {
			req.ItemId = root
			_, err := f.srv.CreateShare(asCaller("u", actor), req)
			assert.Equal(t, codes.InvalidArgument, status.Code(err), "%s share of %s", req.ShareType, root)
		}
	}
	assert.Empty(t, f.pw.snapshot(), "no share OA, edge or association")
	var n int
	require.NoError(t, f.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM drive_shares WHERE drive_item_id IN ($1, $2)`, wsRoot, chRoot.Id).Scan(&n))
	assert.Zero(t, n)
}

// A file's ownership of its OA is read from the policy writer, which is never
// behind: a reader that has not yet seen the file's new node must not make a
// move treat the shared file as one that inherits its folder's OA.
func TestMoveItem_ASharedFileKeepsItsOAWhenTheReaderLags(t *testing.T) {
	pr, pw := newFakePolicyRead(), &recWrite{} // pr knows none of pw's nodes
	srv, pool := newServerWith(t, pr, pw)
	f := &boundaryFixture{pr: pr, pw: pw, srv: srv, pool: pool, wsID: getTestWorkspaceID(t, pool)}
	from, to := f.folder(t, "From", ""), f.folder(t, "To", "")
	file := f.file(t, "a.pdf", from.Id)
	f.shareWithRole(t, file.Id)
	f.pw.reset()

	moved, err := f.srv.MoveItem(asCaller("u", actor), &pb.MoveItemRequest{ItemId: file.Id, NewParentId: to.Id})
	require.NoError(t, err)
	assert.Equal(t, fileOA(file), moved.NgacNodeId, "the share keeps covering the file")
	assert.Contains(t, f.pw.snapshot(), fmt.Sprintf("assign %s>%s", fileOA(file), oaOf(to)))
}
