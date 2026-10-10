package grpc_test

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/pkg/grpcauth"
	"ngac-platform/pkg/realtime"
	pb "ngac-platform/proto/drive"
	grpcserver "ngac-platform/services/drive/internal/grpc"
)

// probe is an Emitter that records each event together with whether the
// database already showed the change at the moment it was announced. An event
// sent before the change is visible would tell a browser to refetch the old
// state.
type probe struct {
	mu      sync.Mutex
	events  []realtime.Event
	visible []error
	check   func(realtime.Event) error
}

func (p *probe) Emit(e realtime.Event) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.events = append(p.events, e)
	if p.check != nil {
		p.visible = append(p.visible, p.check(e))
	}
}

func (p *probe) got() []realtime.Event {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]realtime.Event(nil), p.events...)
}

func (p *probe) assertVisibleAtEmit(t *testing.T) {
	t.Helper()
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, err := range p.visible {
		assert.NoError(t, err, "the change must be committed before it is announced")
	}
}

// rtFixture is a server with the probe wired in and a caller that has a tenant.
type rtFixture struct {
	srv    *grpcserver.DriveServer
	pool   *pgxpool.Pool
	probe  *probe
	wsID   string
	tenant string
	ctx    context.Context
}

func newRTFixture(t *testing.T) *rtFixture {
	t.Helper()
	srv, pool := setupServer(t)
	return newRTFixtureOn(t, srv, pool)
}

func newRTFixtureOn(t *testing.T, srv *grpcserver.DriveServer, pool *pgxpool.Pool) *rtFixture {
	t.Helper()
	wsID := getTestWorkspaceID(t, pool)
	f := &rtFixture{srv: srv, pool: pool, probe: &probe{}, wsID: wsID, tenant: "tenant-" + wsID}
	f.ctx = grpcauth.WithCaller(context.Background(), grpcauth.Caller{
		UserID: getTestUserID(t, pool), NGACNodeID: "ngac-user-1", TenantID: f.tenant,
	})
	srv.SetEmitter(f.probe)
	return f
}

// setup runs fn with the emitter muted, so arrangement is not counted.
func (f *rtFixture) setup(fn func()) {
	f.srv.SetEmitter(nil)
	fn()
	f.srv.SetEmitter(f.probe)
}

func (f *rtFixture) folder(t *testing.T, name, parent string) *pb.DriveItem {
	t.Helper()
	var it *pb.DriveItem
	f.setup(func() {
		var err error
		it, err = f.srv.CreateFolder(f.ctx, &pb.CreateFolderRequest{WorkspaceId: f.wsID, Name: name, ParentId: parent})
		require.NoError(t, err)
	})
	t.Cleanup(func() { cleanDriveItems(t, f.pool, it.Id) })
	return it
}

func (f *rtFixture) confirmedFile(t *testing.T, name string) string {
	t.Helper()
	var id string
	f.setup(func() {
		resp, err := f.srv.CreateFile(f.ctx, &pb.CreateFileRequest{
			WorkspaceId: f.wsID, Name: name, MimeType: "application/pdf", SizeBytes: 1024,
		})
		require.NoError(t, err)
		id = resp.FileId
		_, err = f.srv.ConfirmFile(f.ctx, &pb.ConfirmFileRequest{FileId: id})
		require.NoError(t, err)
	})
	t.Cleanup(func() { cleanDriveItems(t, f.pool, id) })
	return id
}

// column reads one column of a drive item; "" when the row is gone.
func (f *rtFixture) rowState(id string) (name, status, parent string, found bool) {
	var p *string
	err := f.pool.QueryRow(context.Background(),
		`SELECT name, status, parent_id FROM drive_items WHERE id = $1`, id).Scan(&name, &status, &p)
	if err != nil {
		return "", "", "", false
	}
	if p != nil {
		parent = *p
	}
	return name, status, parent, true
}

func (f *rtFixture) only(t *testing.T, kind string, ids ...string) realtime.Event {
	t.Helper()
	evs := f.probe.got()
	require.Len(t, evs, 1, "exactly one event, got %+v", evs)
	e := evs[0]
	assert.Equal(t, realtime.DomainDrive, e.Domain)
	assert.Equal(t, kind, e.Kind)
	assert.Equal(t, ids, e.IDs)
	assert.Equal(t, f.wsID, e.WorkspaceID)
	assert.Equal(t, f.tenant, e.TenantID)
	assert.Equal(t, getTestUserID(t, f.pool), e.ActorUserID)
	return e
}

func TestRealtime_CreateFolder(t *testing.T) {
	f := newRTFixture(t)
	parent := f.folder(t, "RtParent", "")
	f.probe.check = func(e realtime.Event) error {
		if _, st, _, ok := f.rowState(e.IDs[0]); !ok || st != "active" {
			return fmt.Errorf("folder %s not committed", e.IDs[0])
		}
		return nil
	}

	child, err := f.srv.CreateFolder(f.ctx, &pb.CreateFolderRequest{WorkspaceId: f.wsID, Name: "RtChild", ParentId: parent.Id})
	require.NoError(t, err)
	t.Cleanup(func() { cleanDriveItems(t, f.pool, child.Id) })

	e := f.only(t, realtime.KindCreated, child.Id)
	assert.Equal(t, parent.Id, e.ParentID)
	f.probe.assertVisibleAtEmit(t)
}

func TestRealtime_CreateFolderAtRootHasNoParent(t *testing.T) {
	f := newRTFixture(t)
	root, err := f.srv.CreateFolder(f.ctx, &pb.CreateFolderRequest{WorkspaceId: f.wsID, Name: "RtTop"})
	require.NoError(t, err)
	t.Cleanup(func() { cleanDriveItems(t, f.pool, root.Id) })
	assert.Equal(t, "", f.only(t, realtime.KindCreated, root.Id).ParentID)
}

func TestRealtime_FileAppearsOnConfirmNotOnCreate(t *testing.T) {
	f := newRTFixture(t)
	parent := f.folder(t, "RtUploads", "")

	resp, err := f.srv.CreateFile(f.ctx, &pb.CreateFileRequest{
		WorkspaceId: f.wsID, Name: "a.pdf", MimeType: "application/pdf", SizeBytes: 10, ParentId: parent.Id,
	})
	require.NoError(t, err)
	t.Cleanup(func() { cleanDriveItems(t, f.pool, resp.FileId) })
	assert.Empty(t, f.probe.got(), "a pending upload is not listed, so nothing changed for anyone else")

	f.probe.check = func(e realtime.Event) error {
		if _, st, _, _ := f.rowState(e.IDs[0]); st != "active" {
			return fmt.Errorf("file still %q", st)
		}
		return nil
	}
	_, err = f.srv.ConfirmFile(f.ctx, &pb.ConfirmFileRequest{FileId: resp.FileId})
	require.NoError(t, err)

	e := f.only(t, realtime.KindCreated, resp.FileId)
	assert.Equal(t, parent.Id, e.ParentID)
	f.probe.assertVisibleAtEmit(t)
}

func TestRealtime_ConfirmFileThatIsNotPendingEmitsNothing(t *testing.T) {
	f := newRTFixture(t)
	id := f.confirmedFile(t, "done.pdf")
	_, err := f.srv.ConfirmFile(f.ctx, &pb.ConfirmFileRequest{FileId: id})
	require.Error(t, err)
	assert.Empty(t, f.probe.got())
}

func TestRealtime_CopyItem(t *testing.T) {
	f := newRTFixture(t)
	dest := f.folder(t, "RtCopyDest", "")
	src := f.confirmedFile(t, "copyme.pdf")

	copied, err := f.srv.CopyItem(f.ctx, &pb.CopyItemRequest{ItemId: src, DestParentId: dest.Id, DestWorkspaceId: f.wsID})
	require.NoError(t, err)
	t.Cleanup(func() { cleanDriveItems(t, f.pool, copied.Id) })

	e := f.only(t, realtime.KindCreated, copied.Id)
	assert.Equal(t, dest.Id, e.ParentID)
}

func TestRealtime_RenameItem(t *testing.T) {
	f := newRTFixture(t)
	parent := f.folder(t, "RtRenameParent", "")
	item := f.folder(t, "Before", parent.Id)
	f.probe.check = func(e realtime.Event) error {
		if name, _, _, _ := f.rowState(e.IDs[0]); name != "After" {
			return fmt.Errorf("name is still %q", name)
		}
		return nil
	}

	_, err := f.srv.RenameItem(f.ctx, &pb.RenameItemRequest{ItemId: item.Id, NewName: "After"})
	require.NoError(t, err)

	assert.Equal(t, parent.Id, f.only(t, realtime.KindUpdated, item.Id).ParentID)
	f.probe.assertVisibleAtEmit(t)
}

func TestRealtime_MoveItemNamesBothFolders(t *testing.T) {
	f := newRTFixture(t)
	from := f.folder(t, "RtFrom", "")
	to := f.folder(t, "RtTo", "")
	item := f.folder(t, "RtMoving", from.Id)
	f.probe.check = func(e realtime.Event) error {
		if _, _, parent, _ := f.rowState(e.IDs[0]); parent != to.Id {
			return fmt.Errorf("parent is still %q", parent)
		}
		return nil
	}

	_, err := f.srv.MoveItem(f.ctx, &pb.MoveItemRequest{ItemId: item.Id, NewParentId: to.Id})
	require.NoError(t, err)

	e := f.only(t, realtime.KindMoved, item.Id)
	assert.Equal(t, to.Id, e.ParentID)
	assert.Equal(t, from.Id, e.OldParentID)
	f.probe.assertVisibleAtEmit(t)
}

func TestRealtime_MoveToRootHasEmptyParent(t *testing.T) {
	f := newRTFixture(t)
	from := f.folder(t, "RtFromTop", "")
	item := f.folder(t, "RtUp", from.Id)

	_, err := f.srv.MoveItem(f.ctx, &pb.MoveItemRequest{ItemId: item.Id})
	require.NoError(t, err)

	e := f.only(t, realtime.KindMoved, item.Id)
	assert.Equal(t, "", e.ParentID)
	assert.Equal(t, from.Id, e.OldParentID)
}

func TestRealtime_RefusedMovesEmitNothing(t *testing.T) {
	f := newRTFixture(t)
	outer := f.folder(t, "RtOuter", "")
	inner := f.folder(t, "RtInner", outer.Id)
	file := f.confirmedFile(t, "notafolder.pdf")

	_, err := f.srv.MoveItem(f.ctx, &pb.MoveItemRequest{ItemId: outer.Id, NewParentId: inner.Id})
	require.Error(t, err, "into its own subfolder")
	_, err = f.srv.MoveItem(f.ctx, &pb.MoveItemRequest{ItemId: inner.Id, NewParentId: file})
	require.Error(t, err, "destination is a file")
	_, err = f.srv.MoveItem(f.ctx, &pb.MoveItemRequest{ItemId: "no-such-item", NewParentId: outer.Id})
	require.Error(t, err, "unknown item")

	assert.Empty(t, f.probe.got())
}

func TestRealtime_TrashRestoreAndDelete(t *testing.T) {
	f := newRTFixture(t)
	parent := f.folder(t, "RtTrashParent", "")
	item := f.folder(t, "RtTrashMe", parent.Id)

	f.probe.check = func(e realtime.Event) error {
		if _, st, _, _ := f.rowState(e.IDs[0]); st != "trashed" {
			return fmt.Errorf("status %q", st)
		}
		return nil
	}
	_, err := f.srv.TrashItem(f.ctx, &pb.TrashItemRequest{ItemId: item.Id})
	require.NoError(t, err)
	assert.Equal(t, parent.Id, f.only(t, realtime.KindDeleted, item.Id).ParentID)
	f.probe.assertVisibleAtEmit(t)

	f.probe = &probe{check: func(e realtime.Event) error {
		if _, st, _, _ := f.rowState(e.IDs[0]); st != "active" {
			return fmt.Errorf("status %q", st)
		}
		return nil
	}}
	f.srv.SetEmitter(f.probe)
	_, err = f.srv.RestoreItem(f.ctx, &pb.RestoreItemRequest{ItemId: item.Id})
	require.NoError(t, err)
	assert.Equal(t, parent.Id, f.only(t, realtime.KindUpdated, item.Id).ParentID)
	f.probe.assertVisibleAtEmit(t)

	f.probe = &probe{check: func(e realtime.Event) error {
		if _, _, _, found := f.rowState(e.IDs[0]); found {
			return fmt.Errorf("row still present")
		}
		return nil
	}}
	f.srv.SetEmitter(f.probe)
	_, err = f.srv.DeleteItem(f.ctx, &pb.DeleteItemRequest{ItemId: item.Id})
	require.NoError(t, err)
	assert.Equal(t, parent.Id, f.only(t, realtime.KindDeleted, item.Id).ParentID)
	f.probe.assertVisibleAtEmit(t)
}

func TestRealtime_Shares(t *testing.T) {
	f := newRTFixture(t)
	item := f.folder(t, "RtShared", "")
	f.probe.check = func(e realtime.Event) error {
		var n int
		err := f.pool.QueryRow(context.Background(), `SELECT count(*) FROM drive_shares WHERE drive_item_id = $1`, e.IDs[0]).Scan(&n)
		if err != nil {
			return err
		}
		if e.Kind == realtime.KindShareCreated && n != 1 {
			return fmt.Errorf("share row missing")
		}
		if e.Kind == realtime.KindShareRevoked && n != 0 {
			return fmt.Errorf("share row still present")
		}
		return nil
	}

	share, err := f.srv.CreateShare(f.ctx, &pb.CreateShareRequest{
		ItemId: item.Id, ShareType: "user", TargetNgacNodeId: "ngac-user-2", Operations: []string{"read"},
	})
	require.NoError(t, err)
	f.only(t, realtime.KindShareCreated, item.Id)

	f.probe.mu.Lock()
	f.probe.events = nil
	f.probe.mu.Unlock()
	_, err = f.srv.RevokeShare(f.ctx, &pb.RevokeShareRequest{ShareId: share.Id})
	require.NoError(t, err)
	f.only(t, realtime.KindShareRevoked, item.Id)
	f.probe.assertVisibleAtEmit(t)
}

func TestRealtime_RefusedShareChangesEmitNothing(t *testing.T) {
	f := newRTFixture(t)
	item := f.folder(t, "RtShareRefused", "")

	_, err := f.srv.CreateShare(f.ctx, &pb.CreateShareRequest{
		ItemId: item.Id, ShareType: "user", TargetNgacNodeId: "ngac-user-2", Operations: []string{"manage"},
	})
	require.Error(t, err, "an operation a share may not grant")
	_, err = f.srv.CreateShare(f.ctx, &pb.CreateShareRequest{ItemId: "missing", ShareType: "user", Operations: []string{"read"}})
	require.Error(t, err)
	_, err = f.srv.RevokeShare(f.ctx, &pb.RevokeShareRequest{ShareId: "missing"})
	require.Error(t, err)

	assert.Empty(t, f.probe.got())
}

func TestRealtime_DeniedCallerEmitsNothing(t *testing.T) {
	allow, pool := setupServer(t)
	allowed := newRTFixtureOn(t, allow, pool)
	item := allowed.folder(t, "RtDenied", "")
	file := allowed.confirmedFile(t, "denied.pdf")

	deny, _ := setupServerDeny(t)
	denied := &rtFixture{srv: deny, pool: pool, probe: &probe{}, wsID: allowed.wsID, tenant: allowed.tenant, ctx: allowed.ctx}
	deny.SetEmitter(denied.probe)

	_, err := deny.CreateFolder(denied.ctx, &pb.CreateFolderRequest{WorkspaceId: denied.wsID, Name: "x", ParentId: item.Id})
	require.Error(t, err)
	_, err = deny.RenameItem(denied.ctx, &pb.RenameItemRequest{ItemId: item.Id, NewName: "y"})
	require.Error(t, err)
	_, err = deny.MoveItem(denied.ctx, &pb.MoveItemRequest{ItemId: item.Id})
	require.Error(t, err)
	_, err = deny.TrashItem(denied.ctx, &pb.TrashItemRequest{ItemId: item.Id})
	require.Error(t, err)
	_, err = deny.RestoreItem(denied.ctx, &pb.RestoreItemRequest{ItemId: item.Id})
	require.Error(t, err)
	_, err = deny.DeleteItem(denied.ctx, &pb.DeleteItemRequest{ItemId: item.Id})
	require.Error(t, err)
	_, err = deny.CopyItem(denied.ctx, &pb.CopyItemRequest{ItemId: file, DestParentId: item.Id, DestWorkspaceId: denied.wsID})
	require.Error(t, err)
	_, err = deny.CreateShare(denied.ctx, &pb.CreateShareRequest{
		ItemId: item.Id, ShareType: "user", TargetNgacNodeId: "ngac-user-2", Operations: []string{"read"},
	})
	require.Error(t, err)

	assert.Empty(t, denied.probe.got())
	_, status, _, found := allowed.rowState(item.Id)
	assert.True(t, found)
	assert.Equal(t, "active", status, "and nothing changed")
}

func TestRealtime_StorageFailureEmitsNothing(t *testing.T) {
	srv, pool := setupServer(t)
	f := newRTFixtureOn(t, srv, pool)
	item := f.folder(t, "RtClosed", "")
	// Once the pool is closed every store call fails.
	closed, err := pgxpool.New(context.Background(), testDBURL())
	require.NoError(t, err)
	closed.Close()
	broken := grpcserver.NewDriveServer(closed, &mockPolicyRead{}, &mockPolicyWrite{}, &mockDocStorage{})
	p := &probe{}
	broken.SetEmitter(p)

	_, err = broken.RenameItem(f.ctx, &pb.RenameItemRequest{ItemId: item.Id, NewName: "z"})
	require.Error(t, err)
	_, err = broken.TrashItem(f.ctx, &pb.TrashItemRequest{ItemId: item.Id})
	require.Error(t, err)
	_, err = broken.CreateFolder(f.ctx, &pb.CreateFolderRequest{WorkspaceId: f.wsID, Name: "n", ParentId: item.Id})
	require.Error(t, err)

	assert.Empty(t, p.got())
}

func TestRealtime_CallerWithoutTenantProducesAnEventTheProducerDrops(t *testing.T) {
	f := newRTFixture(t)
	ctx := grpcauth.WithCaller(context.Background(), grpcauth.Caller{UserID: "u", NGACNodeID: "ngac-user-1"})
	it, err := f.srv.CreateFolder(ctx, &pb.CreateFolderRequest{WorkspaceId: f.wsID, Name: "NoTenant"})
	require.NoError(t, err)
	t.Cleanup(func() { cleanDriveItems(t, f.pool, it.Id) })

	evs := f.probe.got()
	require.Len(t, evs, 1)
	assert.Empty(t, evs[0].TenantID, "no tenant is invented")
	assert.Error(t, evs[0].Validate(), "so the producer refuses it")
}

// Trashing a folder moves the folder and everything under it together: when
// the contents cannot be written, the folder is not left trashed with its
// children still live, and nothing is announced.
func TestRealtime_TrashAndRestoreAreAllOrNothing(t *testing.T) {
	f := newRTFixture(t)
	folder := f.folder(t, "RtAtomic", "")
	child := f.folder(t, "RtAtomicChild", folder.Id)
	ctx := context.Background()

	// A trigger that refuses to touch the child stands in for a failing write.
	_, err := f.pool.Exec(ctx, `CREATE OR REPLACE FUNCTION rt_refuse_child() RETURNS trigger AS $$
		BEGIN IF NEW.id = '`+child.Id+`' THEN RAISE EXCEPTION 'rt: child write refused'; END IF; RETURN NEW; END $$ LANGUAGE plpgsql`)
	require.NoError(t, err)
	_, err = f.pool.Exec(ctx, `CREATE TRIGGER rt_refuse_child BEFORE UPDATE ON drive_items FOR EACH ROW EXECUTE FUNCTION rt_refuse_child()`)
	require.NoError(t, err)
	dropped := false
	drop := func() {
		if !dropped {
			dropped = true
			f.pool.Exec(ctx, `DROP TRIGGER IF EXISTS rt_refuse_child ON drive_items`)
			f.pool.Exec(ctx, `DROP FUNCTION IF EXISTS rt_refuse_child()`)
		}
	}
	t.Cleanup(drop)

	before := len(f.probe.events)
	_, err = f.srv.TrashItem(f.ctx, &pb.TrashItemRequest{ItemId: folder.Id})
	require.Error(t, err)
	_, st, _, _ := f.rowState(folder.Id)
	assert.Equal(t, "active", st, "the folder must not be half trashed")
	assert.Len(t, f.probe.events, before, "a failed trash announces nothing")

	drop()
	_, err = f.srv.TrashItem(f.ctx, &pb.TrashItemRequest{ItemId: folder.Id})
	require.NoError(t, err)
	_, st, _, _ = f.rowState(child.Id)
	assert.Equal(t, "trashed", st, "and when it succeeds the contents go with it")
}
