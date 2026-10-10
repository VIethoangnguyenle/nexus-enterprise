package grpc_test

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"ngac-platform/ngac"
	pb "ngac-platform/proto/drive"
	policypb "ngac-platform/proto/policy"
	"ngac-platform/services/drive/internal/store"
)

// Everything the drive provisions in the graph is named by an ID, and what it
// provisions in several steps is undone when a later step fails.

func nodesCreated(w *recWrite) []string {
	var out []string
	for _, c := range w.snapshot() {
		if rest, ok := strings.CutPrefix(c, "node "); ok {
			out = append(out, rest) // "<type> <name>"
		}
	}
	return out
}

func channelDrive(wsID, chID, name string) *pb.CreateDriveForChannelRequest {
	return &pb.CreateDriveForChannelRequest{
		WorkspaceId: wsID, ChannelId: chID, ChannelName: name,
		ChannelNgacOaId: "oa-" + chID, ChannelNgacUaId: "ua-" + chID,
	}
}

func cleanChannelDrives(t *testing.T, pool *pgxpool.Pool, ids ...string) {
	t.Helper()
	t.Cleanup(func() {
		for _, id := range ids {
			pool.Exec(context.Background(), "DELETE FROM drive_items WHERE drive_context_id = $1", id)
		}
	})
}

func TestCreateDriveForChannel_SameNameInTwoChannelsDoesNotCollide(t *testing.T) {
	pr, pw := newFakePolicyRead(), &recWrite{}
	srv, pool := newServerWith(t, pr, pw)
	wsID := getTestWorkspaceID(t, pool)
	a, b := insertTestChannel(t, pool, "general", wsID), insertTestChannel(t, pool, "general", wsID)
	cleanChannelDrives(t, pool, a, b)

	da, err := srv.CreateDriveForChannel(context.Background(), channelDrive(wsID, a, "general"))
	require.NoError(t, err)
	db, err := srv.CreateDriveForChannel(context.Background(), channelDrive(wsID, b, "general"))
	require.NoError(t, err)

	assert.Equal(t, []string{
		"OA " + ngac.ChannelDriveName(ngac.ChannelID(a)),
		"OA " + ngac.ChannelDriveName(ngac.ChannelID(b)),
	}, nodesCreated(pw), "two channels called general are two nodes, each named by its channel's ID")
	assert.Equal(t, "general", da.Name)
	assert.Equal(t, "general", db.Name, "what the drive shows is the channel's name, not a node name")
}

func TestCreateDriveForChannel_IsIdempotent(t *testing.T) {
	pr, pw := newFakePolicyRead(), &recWrite{}
	srv, pool := newServerWith(t, pr, pw)
	wsID := getTestWorkspaceID(t, pool)
	ch := insertTestChannel(t, pool, "general", wsID)
	cleanChannelDrives(t, pool, ch)

	first, err := srv.CreateDriveForChannel(context.Background(), channelDrive(wsID, ch, "general"))
	require.NoError(t, err)
	callsAfterFirst := len(pw.snapshot())
	second, err := srv.CreateDriveForChannel(context.Background(), channelDrive(wsID, ch, "general"))
	require.NoError(t, err)

	assert.Equal(t, first.Id, second.Id, "the same drive comes back")
	assert.Len(t, pw.snapshot(), callsAfterFirst, "and nothing more is written")
}

func TestCreateDriveForChannel_RollsBackItsNodeWhereverItFails(t *testing.T) {
	for name, set := range map[string]func(*recWrite){
		"assignment under the content OA": func(w *recWrite) { w.failAssign = func(string, string) bool { return true } },
		"association for the members":     func(w *recWrite) { w.failAssoc = true },
	} {
		t.Run(name, func(t *testing.T) {
			pr, pw := newFakePolicyRead(), &recWrite{}
			set(pw)
			srv, pool := newServerWith(t, pr, pw)
			wsID := getTestWorkspaceID(t, pool)
			ch := insertTestChannel(t, pool, "general", wsID)
			cleanChannelDrives(t, pool, ch)

			_, err := srv.CreateDriveForChannel(context.Background(), channelDrive(wsID, ch, "general"))

			require.Error(t, err)
			assert.True(t, pw.has("delete id:"+ngac.ChannelDriveName(ngac.ChannelID(ch))), "calls: %v", pw.snapshot())
			got, gerr := srv.GetChannelDrive(context.Background(), &pb.GetChannelDriveRequest{ChannelId: ch})
			if gerr == nil {
				assert.Empty(t, got.GetId(), "no drive row for a failed provisioning")
			}
		})
	}
}

// An OA a previous attempt left behind is picked up, not duplicated — and not
// deleted if this attempt then fails, since this attempt did not create it.
func TestCreateDriveForChannel_ReusesAnExistingOAAndNeverDeletesIt(t *testing.T) {
	pr, pw := newFakePolicyRead(), &recWrite{}
	srv, pool := newServerWith(t, pr, pw)
	wsID := getTestWorkspaceID(t, pool)
	ch := insertTestChannel(t, pool, "general", wsID)
	cleanChannelDrives(t, pool, ch)
	pr.add("oa-left-behind", ngac.ChannelDriveName(ngac.ChannelID(ch)), ngac.TypeOA)
	pw.failAssoc = true

	_, err := srv.CreateDriveForChannel(context.Background(), channelDrive(wsID, ch, "general"))

	require.Error(t, err)
	assert.Empty(t, nodesCreated(pw), "the existing OA is reused")
	assert.False(t, pw.has("delete "), "and it is not ours to delete: %v", pw.snapshot())
}

func TestCreateDriveForChannel_RequiresAChannelAndWorkspace(t *testing.T) {
	pw := &recWrite{}
	srv, _ := newServerWith(t, newFakePolicyRead(), pw)

	for _, req := range []*pb.CreateDriveForChannelRequest{
		{WorkspaceId: "ws", ChannelName: "general"},
		{ChannelId: "ch", ChannelName: "general"},
	} {
		_, err := srv.CreateDriveForChannel(context.Background(), req)
		require.Error(t, err)
	}
	assert.Empty(t, pw.snapshot(), "a drive keyed by nothing is never created")
}

func TestCreateFolder_NodeIsKeyedByTheFolderIDNotItsName(t *testing.T) {
	pr, pw := newFakePolicyRead(), &recWrite{}
	srv, pool := newServerWith(t, pr, pw)
	wsID := getTestWorkspaceID(t, pool)

	var items []*pb.DriveItem
	for i := 0; i < 2; i++ {
		it, err := srv.CreateFolder(asCaller("", actor), &pb.CreateFolderRequest{WorkspaceId: wsID, Name: "Reports"})
		require.NoError(t, err)
		items = append(items, it)
	}
	t.Cleanup(func() { cleanDriveItems(t, pool, items[0].Id, items[1].Id) })

	created := nodesCreated(pw)
	require.Contains(t, created, "OA "+ngac.FolderNodeName(ngac.FolderID(items[0].Id)))
	require.Contains(t, created, "OA "+ngac.FolderNodeName(ngac.FolderID(items[1].Id)))
	for _, c := range created {
		if strings.HasPrefix(c, "OA Folder_") {
			assert.NotContains(t, c, "Reports", "the folder's name is not part of its node name")
		}
	}
	assert.Equal(t, "Reports", items[0].Name)
	assert.Equal(t, "Reports", pw.nodeProps["id:"+ngac.FolderNodeName(ngac.FolderID(items[0].Id))][ngac.PropDisplayName])
}

func TestCreateFolder_RollsBackItsNodeWhenItCannotBeAssigned(t *testing.T) {
	pr, pw := newFakePolicyRead(), &recWrite{}
	pw.failAssign = func(string, string) bool { return true }
	srv, pool := newServerWith(t, pr, pw)
	wsID := getTestWorkspaceID(t, pool)

	_, err := srv.CreateFolder(asCaller("", actor), &pb.CreateFolderRequest{WorkspaceId: wsID, Name: "Reports"})

	require.Error(t, err)
	assert.True(t, pw.has("delete id:Folder_"), "calls: %v", pw.snapshot())
	var rows int
	require.NoError(t, pool.QueryRow(context.Background(), `SELECT count(*) FROM drive_items WHERE workspace_id = $1 AND name = 'Reports'`, wsID).Scan(&rows))
	assert.Zero(t, rows, "no row for the folder that could not be provisioned")
}

func TestCreateShare_ShareOAIsKeyedByTheShareID(t *testing.T) {
	pr, pw := newFakePolicyRead(), &recWrite{}
	pr.add("pc-global", ngac.NodePCGlobal, ngac.TypePC)
	srv, pool := newServerWith(t, pr, pw)
	wsID := getTestWorkspaceID(t, pool)
	folder, err := srv.CreateFolder(asCaller("", actor), &pb.CreateFolderRequest{WorkspaceId: wsID, Name: "Contract"})
	require.NoError(t, err)
	t.Cleanup(func() { cleanDriveItems(t, pool, folder.Id) })
	pr.add("ua-team", "Team", ngac.TypeUA)

	var shares []*pb.ShareInfo
	for i := 0; i < 2; i++ {
		sh, err := srv.CreateShare(asCaller("", actor), &pb.CreateShareRequest{
			ItemId: folder.Id, ShareType: "role", TargetNgacNodeId: "ua-team", Operations: []string{ngac.SharePermissionRead},
		})
		require.NoError(t, err)
		shares = append(shares, sh)
	}

	created := nodesCreated(pw)
	for _, sh := range shares {
		assert.Contains(t, created, "OA "+ngac.ShareOAName(ngac.ShareID(sh.Id)))
	}
	assert.NotEqual(t, shares[0].Id, shares[1].Id)
	for _, c := range created {
		if strings.HasPrefix(c, "OA Share_") {
			assert.NotContains(t, c, "Contract", "the item's name is not part of the share OA's name")
		}
	}
}

// A share's label reaches the screen. The target's node name is an ID-keyed
// platform name for roles and users now, so the label comes from its display
// name.
func TestCreateShare_LabelIsTheTargetsDisplayName(t *testing.T) {
	pr, pw := newFakePolicyRead(), &recWrite{}
	pr.add("pc-global", ngac.NodePCGlobal, ngac.TypePC)
	pr.nodes["ua-role"] = &policypb.NGACNode{
		Id: "ua-role", Name: "Role_6f1e", NodeType: ngac.TypeUA,
		Properties: map[string]string{ngac.PropDisplayName: "Reviewers"},
	}
	srv, pool := newServerWith(t, pr, pw)
	wsID := getTestWorkspaceID(t, pool)
	folder, err := srv.CreateFolder(asCaller("", actor), &pb.CreateFolderRequest{WorkspaceId: wsID, Name: "Doc"})
	require.NoError(t, err)
	t.Cleanup(func() { cleanDriveItems(t, pool, folder.Id) })

	sh, err := srv.CreateShare(asCaller("", actor), &pb.CreateShareRequest{
		ItemId: folder.Id, ShareType: "role", TargetNgacNodeId: "ua-role", Operations: []string{ngac.SharePermissionRead},
	})

	require.NoError(t, err)
	assert.Equal(t, "Reviewers", sh.TargetLabel)
}

// ---------------------------------------------------------------------------
// Drive root
// ---------------------------------------------------------------------------

// decoyRead lists a "Docs" folder and an unrelated OA first among the
// workspace PC's children. The drive root must not look at them.
type decoyRead struct{ *fakePolicyRead }

func (d decoyRead) GetChildren(context.Context, *policypb.GetChildrenRequest, ...grpc.CallOption) (*policypb.NodeList, error) {
	return &policypb.NodeList{Nodes: []*policypb.NGACNode{
		{Id: "oa-first", Name: "Aardvark", NodeType: ngac.TypeOA},
		{Id: "oa-docs-decoy", Name: "Docs", NodeType: ngac.TypeOA},
		{Id: "oa-documents-decoy", Name: "Documents", NodeType: ngac.TypeOA},
	}}, nil
}

func anyOAID(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(), `SELECT id FROM ngac_nodes WHERE node_type = 'OA' ORDER BY id LIMIT 1`).Scan(&id); err != nil {
		t.Skipf("no OA node in the test DB: %v", err)
	}
	return id
}

func parentOfFolder(w *recWrite, folderItemID string) string {
	prefix := "assign id:" + ngac.FolderNodeName(ngac.FolderID(folderItemID)) + ">"
	for _, c := range w.snapshot() {
		if rest, ok := strings.CutPrefix(c, prefix); ok {
			return rest
		}
	}
	return ""
}

func TestDriveRoot_IsTheWorkspacesRecordedDocumentsOA(t *testing.T) {
	pw := &recWrite{}
	pr := decoyRead{newFakePolicyRead()}
	srv, pool := newServerWith(t, pr, pw)
	wsID := getTestWorkspaceID(t, pool)
	docs := anyOAID(t, pool)
	_, err := pool.Exec(context.Background(), `UPDATE workspaces SET documents_oa_id = $1 WHERE id = $2`, docs, wsID)
	require.NoError(t, err)

	it, err := srv.CreateFolder(asCaller("", actor), &pb.CreateFolderRequest{WorkspaceId: wsID, Name: "Top"})
	require.NoError(t, err)
	t.Cleanup(func() { cleanDriveItems(t, pool, it.Id) })

	assert.Equal(t, docs, parentOfFolder(pw, it.Id), "a top-level folder hangs under the recorded Documents OA")
	for _, decoy := range []string{"oa-first", "oa-docs-decoy", "oa-documents-decoy"} {
		assert.NotContains(t, pw.snapshot(), "assign id:"+ngac.FolderNodeName(ngac.FolderID(it.Id))+">"+decoy)
	}
	assert.Empty(t, nodesCreatedExcept(pw, "Folder_"), "no root node is created when the OA is recorded")
}

func nodesCreatedExcept(w *recWrite, prefix string) []string {
	var out []string
	for _, n := range nodesCreated(w) {
		if !strings.Contains(n, " "+prefix) {
			out = append(out, n)
		}
	}
	return out
}

// Deny: a workspace that records no Documents OA must never be rooted on
// whichever OA happens to be listed first or happens to be called "Docs".
func TestDriveRoot_WithoutARecordedOAGetsItsOwnNodeNotAGuess(t *testing.T) {
	pw := &recWrite{}
	pr := decoyRead{newFakePolicyRead()}
	srv, pool := newServerWith(t, pr, pw)
	wsID := getTestWorkspaceID(t, pool)

	it, err := srv.CreateFolder(asCaller("", actor), &pb.CreateFolderRequest{WorkspaceId: wsID, Name: "Top"})
	require.NoError(t, err)
	t.Cleanup(func() { cleanDriveItems(t, pool, it.Id) })

	root := "OA " + ngac.DriveRootName(ngac.WorkspaceID(wsID))
	assert.Contains(t, nodesCreated(pw), root, "the workspace's own root node, named by its ID")
	parent := parentOfFolder(pw, it.Id)
	assert.Equal(t, "id:"+ngac.DriveRootName(ngac.WorkspaceID(wsID)), parent)
	for _, decoy := range []string{"oa-first", "oa-docs-decoy", "oa-documents-decoy"} {
		assert.NotEqual(t, decoy, parent)
	}
}

// The root node is found, not created again, when it exists but the root row
// does not (a first attempt that failed after creating the node).
func TestDriveRoot_ReusesItsNodeWhenOnlyTheRowIsMissing(t *testing.T) {
	pw := &recWrite{}
	pr := newFakePolicyRead()
	srv, pool := newServerWith(t, pr, pw)
	wsID := getTestWorkspaceID(t, pool)
	pr.add("oa-existing-root", ngac.DriveRootName(ngac.WorkspaceID(wsID)), ngac.TypeOA)

	it, err := srv.CreateFolder(asCaller("", actor), &pb.CreateFolderRequest{WorkspaceId: wsID, Name: "Top"})
	require.NoError(t, err)
	t.Cleanup(func() { cleanDriveItems(t, pool, it.Id) })

	assert.Equal(t, "oa-existing-root", parentOfFolder(pw, it.Id))
	assert.Empty(t, nodesCreatedExcept(pw, "Folder_"))
}

// ---------------------------------------------------------------------------
// Roots: one per context, and a drive is only ever built for a channel of the
// workspace that asks.
// ---------------------------------------------------------------------------

func rootRows(t *testing.T, pool *pgxpool.Pool, wsID string) int {
	t.Helper()
	var n int
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT count(*) FROM drive_items WHERE workspace_id = $1 AND is_root`, wsID).Scan(&n))
	return n
}

// Top-level user folders are parent-less like the root. Any number of them may
// exist in a workspace; only the root is unique.
func TestInsertItem_OneRootPerContextButAnyNumberOfTopLevelFolders(t *testing.T) {
	_, pool := newServerWith(t, newFakePolicyRead(), &recWrite{})
	wsID := getTestWorkspaceID(t, pool)
	st := store.NewStore(pool)
	mk := func(root bool, name string) error {
		return st.InsertItem(context.Background(), &store.DriveItem{
			WorkspaceID: wsID, DriveContext: "workspace", DriveContextID: wsID, ItemType: "folder",
			Name: name, NGACNodeID: "oa-x", OwnerID: "o", Status: "active", IsRoot: root,
		})
	}

	require.NoError(t, mk(true, "Root"))
	assert.ErrorIs(t, mk(true, "Root again"), store.ErrRootExists, "a second root for the context is refused")
	for _, name := range []string{"A", "B", "C"} {
		require.NoError(t, mk(false, name), "top-level folders are not roots")
	}
	assert.Equal(t, 1, rootRows(t, pool, wsID))
}

// Many first requests at once, each needing the root, end with one root and
// every request served.
func TestEnsureRoot_ConcurrentFirstRequestsCreateOneRoot(t *testing.T) {
	pw := &recWrite{}
	srv, pool := newServerWith(t, newFakePolicyRead(), pw)
	wsID := getTestWorkspaceID(t, pool)

	const n = 8
	var wg sync.WaitGroup
	items := make([]*pb.DriveItem, n)
	errs := make([]error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			items[i], errs[i] = srv.CreateFolder(asCaller("", actor), &pb.CreateFolderRequest{WorkspaceId: wsID, Name: fmt.Sprintf("F%d", i)})
		}()
	}
	wg.Wait()

	for i := range errs {
		require.NoError(t, errs[i], "request %d", i)
	}
	assert.Equal(t, 1, rootRows(t, pool, wsID))
}

func TestCreateDriveForChannel_RefusesAChannelOfAnotherWorkspace(t *testing.T) {
	pr, pw := newFakePolicyRead(), &recWrite{}
	srv, pool := newServerWith(t, pr, pw)
	wsA := getTestWorkspaceID(t, pool)
	ch := insertTestChannel(t, pool, "a-only", wsA)
	t.Cleanup(func() { pool.Exec(context.Background(), "DELETE FROM channels WHERE id = $1", ch) })

	_, err := srv.CreateDriveForChannel(context.Background(), channelDrive("ws-of-b", ch, "a-only"))
	assert.Equal(t, codes.PermissionDenied, status.Code(err), "workspace B naming A's channel")
	_, err = srv.CreateDriveForChannel(context.Background(), channelDrive("ws-of-b", "no-such-channel", "x"))
	assert.Equal(t, codes.PermissionDenied, status.Code(err), "a channel nobody has")

	assert.Empty(t, pw.snapshot(), "nothing is written for a refused request")
	assert.Zero(t, rootRows(t, pool, "ws-of-b"))
}

func TestCreateDriveForChannel_AcceptsOwnChannelAndTheWorkspaceItself(t *testing.T) {
	pr, pw := newFakePolicyRead(), &recWrite{}
	srv, pool := newServerWith(t, pr, pw)
	ws := getTestWorkspaceID(t, pool)
	ch := insertTestChannel(t, pool, "mine", ws)
	t.Cleanup(func() { pool.Exec(context.Background(), "DELETE FROM channels WHERE id = $1", ch) })

	_, err := srv.CreateDriveForChannel(context.Background(), channelDrive(ws, ch, "mine"))
	require.NoError(t, err)
	_, err = srv.CreateDriveForChannel(context.Background(), channelDrive(ws, ws, "Workspace"))
	require.NoError(t, err, "the workspace's own root drive uses its id as the context")
}

// Even for a channel of the right workspace, an OA that is another workspace's
// drive root is not adopted.
func TestCreateDriveForChannel_DoesNotAdoptAnotherWorkspacesRootOA(t *testing.T) {
	pr, pw := newFakePolicyRead(), &recWrite{}
	srv, pool := newServerWith(t, pr, pw)
	ws := getTestWorkspaceID(t, pool)
	ch := insertTestChannel(t, pool, "mine", ws)
	other := "drive-test-ws-other-" + uuid.NewString()
	ctx := context.Background()
	_, err := pool.Exec(ctx, `INSERT INTO workspaces (id, name, owner_id) SELECT $1, $1, owner_id FROM workspaces WHERE id = $2`, other, ws)
	require.NoError(t, err)
	t.Cleanup(func() {
		pool.Exec(ctx, "DELETE FROM drive_items WHERE workspace_id = $1", other)
		pool.Exec(ctx, "DELETE FROM channels WHERE id = $1", ch)
		pool.Exec(ctx, "DELETE FROM workspaces WHERE id = $1", other)
	})
	oa := ngac.ChannelDriveName(ngac.ChannelID(ch))
	pr.add("oa-foreign", oa, ngac.TypeOA)
	_, err = pool.Exec(ctx, `INSERT INTO drive_items (id, workspace_id, drive_context, drive_context_id, item_type, name, ngac_node_id, owner_id, status, is_root)
		VALUES ($1, $2, 'channel', 'elsewhere', 'folder', 'x', 'oa-foreign', 'system', 'active', TRUE)`, uuid.NewString(), other)
	if err != nil {
		t.Skipf("cannot fabricate a foreign root: %v", err) // ngac_node_id has a foreign key in some schemas
	}

	_, err = srv.CreateDriveForChannel(ctx, channelDrive(ws, ch, "mine"))

	assert.Equal(t, codes.PermissionDenied, status.Code(err))
	assert.False(t, pw.has("assoc "), "no association is made onto another workspace's OA: %v", pw.snapshot())
	assert.False(t, pw.has("delete "), "and the foreign OA is not deleted")
}
