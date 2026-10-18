package grpc_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"ngac-platform/ngac"
	"ngac-platform/pkg/grpcauth"
	docpb "ngac-platform/proto/document"
	policypb "ngac-platform/proto/policy"
	"ngac-platform/services/drive/internal/domain"
	"ngac-platform/services/drive/internal/store"
)

// nodePolicy grants read on exactly the OA nodes in readable, for single and
// batch checks alike; everything else is denied. failErr makes every call fail.
type nodePolicy struct {
	mockPolicyRead
	mu       sync.Mutex
	readable map[string]bool
	failErr  error
}

func (p *nodePolicy) allows(node, op string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return op == ngac.OpRead && p.readable[node]
}

func (p *nodePolicy) set(readable ...string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.readable = map[string]bool{}
	for _, n := range readable {
		p.readable[n] = true
	}
}

func (p *nodePolicy) CheckAccess(_ context.Context, req *policypb.CheckAccessRequest, _ ...grpc.CallOption) (*policypb.AccessDecision, error) {
	if p.failErr != nil {
		return nil, p.failErr
	}
	if p.allows(req.ObjectNodeId, req.Operation) {
		return &policypb.AccessDecision{Decision: "ALLOW"}, nil
	}
	return &policypb.AccessDecision{Decision: "DENY"}, nil
}

func (p *nodePolicy) BatchCheckAccess(_ context.Context, req *policypb.BatchCheckAccessRequest, _ ...grpc.CallOption) (*policypb.BatchAccessResult, error) {
	if p.failErr != nil {
		return nil, p.failErr
	}
	results := make(map[string]*policypb.ObjectPermissions, len(req.ObjectIds))
	for _, id := range req.ObjectIds {
		perms := make(map[string]bool, len(req.Operations))
		for _, op := range req.Operations {
			perms[op] = p.allows(id, op)
		}
		results[id] = &policypb.ObjectPermissions{Permissions: perms}
	}
	return &policypb.BatchAccessResult{Results: results}, nil
}

// The OA nodes items hang under in this fixture. Only the policy decides what
// they grant; the nodes themselves need not exist.
const (
	oaOpen   = "stars-oa-open"
	oaClosed = "stars-oa-closed"
)

// starsFixture: a workspace the caller belongs to (wsA) and one they do not
// (wsB). Items are written straight into drive_items so each one sits under
// the OA the test needs.
type starsFixture struct {
	pool                *pgxpool.Pool
	wsA, wsB            string
	caller, callerNode  string
	sharer, sharerNode  string
	policy              *nodePolicy
	docs                *recordingDocs
	svc                 *domain.Service
}

func newStarsFixture(t *testing.T) *starsFixture {
	t.Helper()
	_, pool := setupServer(t)
	ctx := context.Background()
	tt := tenantFor(t)
	sfx := uuid.NewString()
	f := &starsFixture{
		pool: pool, wsA: tt.wsID, wsB: "drive-stars-ws-" + sfx,
		caller: "drive-stars-caller-" + sfx, callerNode: "drive-stars-caller-node-" + sfx,
		sharer: "drive-stars-sharer-" + sfx, sharerNode: "drive-stars-sharer-node-" + sfx,
		policy: &nodePolicy{readable: map[string]bool{oaOpen: true}},
		docs:   &recordingDocs{},
	}
	exec := func(q string, args ...any) {
		t.Helper()
		_, err := pool.Exec(ctx, q, args...)
		require.NoError(t, err, q)
	}
	for _, u := range []struct{ id, node, name string }{{f.caller, f.callerNode, "Minh Đức"}, {f.sharer, f.sharerNode, "Thu Lan"}} {
		exec(`INSERT INTO ngac_nodes (id, name, node_type) VALUES ($1, $1, 'U')`, u.node)
		exec(`INSERT INTO users (id, username, password, ngac_node, display_name) VALUES ($1, $1, '', $2, $3)`, u.id, u.node, u.name)
		exec(`INSERT INTO tenant_users (tenant_id, user_id, role, status) VALUES ($1, $2, 'member', 'active')`, f.wsA, u.id)
	}
	exec(`INSERT INTO workspaces (id, name, owner_id, type) VALUES ($1, $1, $2, 'organization')`, f.wsB, tt.userID)
	t.Cleanup(func() {
		c := context.Background()
		for _, st := range []struct{ q, arg string }{
			{`DELETE FROM drive_items WHERE workspace_id = $1`, f.wsB},
			{`DELETE FROM drive_items WHERE workspace_id = $1`, f.wsA},
			{`DELETE FROM workspaces WHERE id = $1`, f.wsB},
			{`DELETE FROM tenant_users WHERE user_id = $1`, f.caller},
			{`DELETE FROM tenant_users WHERE user_id = $1`, f.sharer},
			{`DELETE FROM users WHERE id = $1`, f.caller},
			{`DELETE FROM users WHERE id = $1`, f.sharer},
			{`DELETE FROM ngac_nodes WHERE id = $1`, f.callerNode},
			{`DELETE FROM ngac_nodes WHERE id = $1`, f.sharerNode},
		} {
			if _, err := pool.Exec(c, st.q, st.arg); err != nil {
				t.Errorf("fixture cleanup %q: %v", st.q, err)
			}
		}
	})
	f.svc = domain.NewService(store.NewStore(pool), f.policy, &mockPolicyWrite{}, f.docs)
	return f
}

type itemSpec struct {
	ws, node, name, owner, status, kind string
	updated                             time.Time
}

// item writes one drive item and returns its id.
func (f *starsFixture) item(t *testing.T, s itemSpec) string {
	t.Helper()
	if s.status == "" {
		s.status = "active"
	}
	if s.kind == "" {
		s.kind = "file"
	}
	if s.owner == "" {
		s.owner = f.sharer
	}
	if s.updated.IsZero() {
		s.updated = time.Now()
	}
	id := uuid.NewString()
	key := "drive/" + id + "/" + s.name
	_, err := f.pool.Exec(context.Background(),
		`INSERT INTO drive_items (id, workspace_id, drive_context, item_type, name, object_key, ngac_node_id,
			owner_id, status, created_at, updated_at)
		 VALUES ($1, $2, 'workspace', $3, $4, $5, $6, $7, $8, $9, $9)`,
		id, s.ws, s.kind, s.name, key, s.node, s.owner, s.status, s.updated)
	require.NoError(t, err)
	return id
}

// star writes a star row directly, the way an earlier, then-allowed star left it.
func (f *starsFixture) star(t *testing.T, userID, itemID string) {
	t.Helper()
	_, err := f.pool.Exec(context.Background(), `INSERT INTO drive_stars (user_id, item_id) VALUES ($1, $2)`, userID, itemID)
	require.NoError(t, err)
}

// share records that the sharer shared itemID with the caller.
func (f *starsFixture) share(t *testing.T, itemID string) {
	t.Helper()
	_, err := f.pool.Exec(context.Background(),
		`INSERT INTO drive_shares (id, drive_item_id, share_type, target_ngac_id, operations, ngac_share_oa, created_by)
		 VALUES ($1, $2, 'user', $3, ARRAY['read'], $4, $5)`,
		uuid.NewString(), itemID, f.callerNode, "share-oa-"+uuid.NewString(), f.sharerNode)
	require.NoError(t, err)
}

func (f *starsFixture) starred(t *testing.T, userID, itemID string) bool {
	t.Helper()
	var ok bool
	require.NoError(t, f.pool.QueryRow(context.Background(),
		`SELECT EXISTS (SELECT 1 FROM drive_stars WHERE user_id = $1 AND item_id = $2)`, userID, itemID).Scan(&ok))
	return ok
}

func (f *starsFixture) asCaller() context.Context {
	return grpcauth.WithCaller(context.Background(), grpcauth.Caller{UserID: f.caller, NGACNodeID: f.callerNode})
}

func denied(t *testing.T, err error) {
	t.Helper()
	require.Error(t, err)
	assert.True(t, errors.Is(err, domain.ErrAccessDenied), "want access denied, got %v", err)
}

// ---- star / unstar ----------------------------------------------------------

func TestStarItem_UnreadableItemIsDenied(t *testing.T) {
	f := newStarsFixture(t)
	id := f.item(t, itemSpec{ws: f.wsA, node: oaClosed, name: "Lương 2026.xlsx"})

	denied(t, f.svc.StarItem(f.asCaller(), id))
	assert.False(t, f.starred(t, f.caller, id), "a refused star must not be written")
}

func TestStarItem_ItemOfAnotherWorkspaceIsDenied(t *testing.T) {
	f := newStarsFixture(t)
	// The policy would allow the node; the caller is simply not in that workspace.
	id := f.item(t, itemSpec{ws: f.wsB, node: oaOpen, name: "Ngoài.pdf"})

	denied(t, f.svc.StarItem(f.asCaller(), id))
	assert.False(t, f.starred(t, f.caller, id))
}

func TestStarItem_AnonymousIsDenied(t *testing.T) {
	f := newStarsFixture(t)
	id := f.item(t, itemSpec{ws: f.wsA, node: oaOpen, name: "Mở.pdf"})

	denied(t, f.svc.StarItem(context.Background(), id))
}

func TestStarItem_PolicyFailureDenies(t *testing.T) {
	f := newStarsFixture(t)
	id := f.item(t, itemSpec{ws: f.wsA, node: oaOpen, name: "Mở.pdf"})
	f.policy.failErr = errors.New("policy unreachable")

	require.Error(t, f.svc.StarItem(f.asCaller(), id))
	assert.False(t, f.starred(t, f.caller, id))
}

func TestStarItem_TrashedPendingOrMissingIsNotFound(t *testing.T) {
	f := newStarsFixture(t)
	for _, id := range []string{
		f.item(t, itemSpec{ws: f.wsA, node: oaOpen, name: "Rác.pdf", status: "trashed"}),
		f.item(t, itemSpec{ws: f.wsA, node: oaOpen, name: "Đang tải.pdf", status: "pending"}),
		uuid.NewString(),
	} {
		err := f.svc.StarItem(f.asCaller(), id)
		require.Error(t, err)
		assert.True(t, errors.Is(err, domain.ErrNotFound), "want not found, got %v", err)
		assert.False(t, f.starred(t, f.caller, id))
	}
}

func TestStarItem_ReadableItemIsStarredOnceForTheCallerOnly(t *testing.T) {
	f := newStarsFixture(t)
	id := f.item(t, itemSpec{ws: f.wsA, node: oaOpen, name: "Mở.pdf"})

	require.NoError(t, f.svc.StarItem(f.asCaller(), id))
	require.NoError(t, f.svc.StarItem(f.asCaller(), id), "starring twice is not an error")
	assert.True(t, f.starred(t, f.caller, id))
	assert.False(t, f.starred(t, f.sharer, id), "a star belongs to the person who set it")
}

func TestUnstarItem_RemovesOnlyTheCallersOwnStar(t *testing.T) {
	f := newStarsFixture(t)
	id := f.item(t, itemSpec{ws: f.wsA, node: oaOpen, name: "Mở.pdf"})
	f.star(t, f.caller, id)
	f.star(t, f.sharer, id)

	require.NoError(t, f.svc.UnstarItem(f.asCaller(), id))
	assert.False(t, f.starred(t, f.caller, id))
	assert.True(t, f.starred(t, f.sharer, id), "someone else's star is never touched")
}

// A star on an item the person can no longer read can still be taken off.
func TestUnstarItem_WorksAfterReadIsLost(t *testing.T) {
	f := newStarsFixture(t)
	id := f.item(t, itemSpec{ws: f.wsA, node: oaClosed, name: "Đã khoá.pdf"})
	f.star(t, f.caller, id)

	require.NoError(t, f.svc.UnstarItem(f.asCaller(), id))
	assert.False(t, f.starred(t, f.caller, id))
}

func TestUnstarItem_AnonymousIsDenied(t *testing.T) {
	f := newStarsFixture(t)
	id := f.item(t, itemSpec{ws: f.wsA, node: oaOpen, name: "Mở.pdf"})
	f.star(t, f.caller, id)

	denied(t, f.svc.UnstarItem(context.Background(), id))
	assert.True(t, f.starred(t, f.caller, id))
}

// ---- starred list -----------------------------------------------------------

func TestListStarred_NonMemberIsDenied(t *testing.T) {
	f := newStarsFixture(t)
	id := f.item(t, itemSpec{ws: f.wsB, node: oaOpen, name: "Ngoài.pdf"})
	f.star(t, f.caller, id)

	_, err := f.svc.ListStarredItems(f.asCaller(), f.wsB)
	denied(t, err)
}

func TestListStarred_AnonymousIsDenied(t *testing.T) {
	f := newStarsFixture(t)
	_, err := f.svc.ListStarredItems(context.Background(), f.wsA)
	denied(t, err)
}

func TestListStarred_ShowsOnlyReadableItemsOfTheWorkspace(t *testing.T) {
	f := newStarsFixture(t)
	open := f.item(t, itemSpec{ws: f.wsA, node: oaOpen, name: "Mở.pdf"})
	closed := f.item(t, itemSpec{ws: f.wsA, node: oaClosed, name: "Đã khoá.pdf"})
	other := f.item(t, itemSpec{ws: f.wsB, node: oaOpen, name: "Ngoài.pdf"})
	trashed := f.item(t, itemSpec{ws: f.wsA, node: oaOpen, name: "Rác.pdf", status: "trashed"})
	for _, id := range []string{open, closed, other, trashed} {
		f.star(t, f.caller, id)
	}
	sharersOwn := f.item(t, itemSpec{ws: f.wsA, node: oaOpen, name: "Của Lan.pdf"})
	f.star(t, f.sharer, sharersOwn)

	list, err := f.svc.ListStarredItems(f.asCaller(), f.wsA)
	require.NoError(t, err)
	assert.Equal(t, []string{"Mở.pdf"}, names(list))
}

func TestListStarred_ItemNoLongerReadableDisappears(t *testing.T) {
	f := newStarsFixture(t)
	id := f.item(t, itemSpec{ws: f.wsA, node: oaOpen, name: "Mở.pdf"})
	require.NoError(t, f.svc.StarItem(f.asCaller(), id))

	f.policy.set() // read on the folder is taken away
	list, err := f.svc.ListStarredItems(f.asCaller(), f.wsA)
	require.NoError(t, err)
	assert.Empty(t, list.GetItems())
}

func TestListStarred_PolicyFailureFailsClosed(t *testing.T) {
	f := newStarsFixture(t)
	f.star(t, f.caller, f.item(t, itemSpec{ws: f.wsA, node: oaOpen, name: "Mở.pdf"}))
	f.policy.failErr = errors.New("policy unreachable")

	list, err := f.svc.ListStarredItems(f.asCaller(), f.wsA)
	require.Error(t, err)
	assert.Nil(t, list)
}

// ---- quick access -----------------------------------------------------------

func entryNames(entries []*domain.QuickAccessEntry) []string {
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.Item.Name)
	}
	return out
}

func TestQuickAccess_NonMemberIsDenied(t *testing.T) {
	f := newStarsFixture(t)
	f.item(t, itemSpec{ws: f.wsB, node: oaOpen, name: "Ngoài.pdf", owner: f.caller})

	_, err := f.svc.QuickAccessItems(f.asCaller(), f.wsB)
	denied(t, err)
}

func TestQuickAccess_AnonymousIsDenied(t *testing.T) {
	f := newStarsFixture(t)
	_, err := f.svc.QuickAccessItems(context.Background(), f.wsA)
	denied(t, err)
}

func TestQuickAccess_NeverShowsUnreadableOrForeignItems(t *testing.T) {
	f := newStarsFixture(t)
	f.item(t, itemSpec{ws: f.wsA, node: oaClosed, name: "Của tôi nhưng đã khoá.pdf", owner: f.caller})
	sharedClosed := f.item(t, itemSpec{ws: f.wsA, node: oaClosed, name: "Chia sẻ đã thu hồi.pdf"})
	f.share(t, sharedClosed)
	starredClosed := f.item(t, itemSpec{ws: f.wsA, node: oaClosed, name: "Sao đã khoá.pdf"})
	f.star(t, f.caller, starredClosed)
	foreign := f.item(t, itemSpec{ws: f.wsB, node: oaOpen, name: "Ngoài.pdf", owner: f.caller})
	f.share(t, foreign)
	f.star(t, f.caller, foreign)

	entries, err := f.svc.QuickAccessItems(f.asCaller(), f.wsA)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

// Someone else's file that was neither shared with, starred by nor put there by
// the caller is not "quick access", even when readable.
func TestQuickAccess_UnrelatedReadableFileIsNotSuggested(t *testing.T) {
	f := newStarsFixture(t)
	f.item(t, itemSpec{ws: f.wsA, node: oaOpen, name: "Của Lan.pdf"})

	entries, err := f.svc.QuickAccessItems(f.asCaller(), f.wsA)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestQuickAccess_GivesTheReasonAndWhoShared(t *testing.T) {
	f := newStarsFixture(t)
	now := time.Now()
	mine := f.item(t, itemSpec{ws: f.wsA, node: oaOpen, name: "Đối soát tháng 9.xlsx", owner: f.caller, updated: now.Add(-3 * time.Hour)})
	shared := f.item(t, itemSpec{ws: f.wsA, node: oaOpen, name: "Ảnh khai trương.jpg", updated: now.Add(-48 * time.Hour)})
	f.share(t, shared)
	starred := f.item(t, itemSpec{ws: f.wsA, node: oaOpen, name: "Quy trình mua sắm.docx", updated: now.Add(-72 * time.Hour)})
	f.star(t, f.caller, starred)

	entries, err := f.svc.QuickAccessItems(f.asCaller(), f.wsA)
	require.NoError(t, err)
	require.Len(t, entries, 3)
	byID := map[string]*domain.QuickAccessEntry{}
	for _, e := range entries {
		byID[e.Item.Id] = e
	}
	assert.Equal(t, domain.ReasonYours, byID[mine].Reason)
	assert.Equal(t, domain.ReasonShared, byID[shared].Reason)
	assert.Equal(t, f.sharerNode, byID[shared].SharedBy)
	assert.Equal(t, domain.ReasonStarred, byID[starred].Reason)
	assert.Empty(t, byID[mine].SharedBy)
}

// A share loses its reason the moment read on the item is gone: the item is
// not shown at all, so neither is who shared it.
func TestQuickAccess_ReasonHiddenOnceReadIsLost(t *testing.T) {
	f := newStarsFixture(t)
	shared := f.item(t, itemSpec{ws: f.wsA, node: oaOpen, name: "Ảnh khai trương.jpg"})
	f.share(t, shared)

	entries, err := f.svc.QuickAccessItems(f.asCaller(), f.wsA)
	require.NoError(t, err)
	require.Len(t, entries, 1)

	f.policy.set()
	entries, err = f.svc.QuickAccessItems(f.asCaller(), f.wsA)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

// What I share with myself, or a file I shared out, is mine, not "shared".
func TestQuickAccess_OwnShareIsNotASharedReason(t *testing.T) {
	f := newStarsFixture(t)
	id := f.item(t, itemSpec{ws: f.wsA, node: oaOpen, name: "Của tôi.pdf", owner: f.caller})
	_, err := f.pool.Exec(context.Background(),
		`INSERT INTO drive_shares (id, drive_item_id, share_type, target_ngac_id, operations, ngac_share_oa, created_by)
		 VALUES ($1, $2, 'user', $3, ARRAY['read'], $4, $3)`, uuid.NewString(), id, f.callerNode, "share-oa-"+uuid.NewString())
	require.NoError(t, err)

	entries, err := f.svc.QuickAccessItems(f.asCaller(), f.wsA)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, domain.ReasonYours, entries[0].Reason)
}

func TestQuickAccess_AtMostSixNewestFirst(t *testing.T) {
	f := newStarsFixture(t)
	now := time.Now()
	for i := 0; i < 8; i++ {
		f.item(t, itemSpec{ws: f.wsA, node: oaOpen, name: string(rune('A'+i)) + ".pdf", owner: f.caller, updated: now.Add(-time.Duration(i) * time.Hour)})
	}

	entries, err := f.svc.QuickAccessItems(f.asCaller(), f.wsA)
	require.NoError(t, err)
	assert.Equal(t, []string{"A.pdf", "B.pdf", "C.pdf", "D.pdf", "E.pdf", "F.pdf"}, entryNames(entries))
}

func TestQuickAccess_PolicyFailureFailsClosed(t *testing.T) {
	f := newStarsFixture(t)
	f.item(t, itemSpec{ws: f.wsA, node: oaOpen, name: "Của tôi.pdf", owner: f.caller})
	f.policy.failErr = errors.New("policy unreachable")

	entries, err := f.svc.QuickAccessItems(f.asCaller(), f.wsA)
	require.Error(t, err)
	assert.Nil(t, entries)
}

// ---- recent ----------------------------------------------------------------

func TestRecentFiles_NonMemberIsDenied(t *testing.T) {
	f := newStarsFixture(t)
	_, err := f.svc.RecentFiles(f.asCaller(), f.wsB)
	denied(t, err)
}

func TestRecentFiles_OnlyReadableActiveFilesOfTheWorkspaceNewestFirst(t *testing.T) {
	f := newStarsFixture(t)
	now := time.Now()
	f.item(t, itemSpec{ws: f.wsA, node: oaOpen, name: "Cũ.pdf", updated: now.Add(-48 * time.Hour)})
	f.item(t, itemSpec{ws: f.wsA, node: oaOpen, name: "Mới.pdf", updated: now})
	f.item(t, itemSpec{ws: f.wsA, node: oaClosed, name: "Đã khoá.pdf"})
	f.item(t, itemSpec{ws: f.wsA, node: oaOpen, name: "Rác.pdf", status: "trashed"})
	f.item(t, itemSpec{ws: f.wsA, node: oaOpen, name: "Đang tải.pdf", status: "pending"})
	f.item(t, itemSpec{ws: f.wsA, node: oaOpen, name: "Thư mục", kind: "folder"})
	f.item(t, itemSpec{ws: f.wsB, node: oaOpen, name: "Ngoài.pdf"})

	list, err := f.svc.RecentFiles(f.asCaller(), f.wsA)
	require.NoError(t, err)
	assert.Equal(t, []string{"Mới.pdf", "Cũ.pdf"}, names(list))
}

func TestRecentFiles_PolicyFailureFailsClosed(t *testing.T) {
	f := newStarsFixture(t)
	f.item(t, itemSpec{ws: f.wsA, node: oaOpen, name: "Mới.pdf"})
	f.policy.failErr = errors.New("policy unreachable")

	list, err := f.svc.RecentFiles(f.asCaller(), f.wsA)
	require.Error(t, err)
	assert.Nil(t, list)
}

// ---- stale uploads ---------------------------------------------------------

// recordingDocs records each stored object the drive asks to delete, and as whom.
type recordingDocs struct {
	mockDocStorage
	mu      sync.Mutex
	deleted map[string]grpcauth.Caller
}

func (d *recordingDocs) DeleteObject(ctx context.Context, req *docpb.DeleteObjectRequest, _ ...grpc.CallOption) (*docpb.Empty, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.deleted == nil {
		d.deleted = map[string]grpcauth.Caller{}
	}
	d.deleted[req.ObjectKey] = grpcauth.CallerFrom(ctx)
	return &docpb.Empty{}, nil
}

func (d *recordingDocs) caller(key string) (grpcauth.Caller, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	c, ok := d.deleted[key]
	return c, ok
}

func (f *starsFixture) status(t *testing.T, id string) string {
	t.Helper()
	var st string
	err := f.pool.QueryRow(context.Background(), `SELECT status FROM drive_items WHERE id = $1`, id).Scan(&st)
	if err != nil {
		return "gone"
	}
	return st
}

func TestCleanupPendingUploads_RemovesOnlyStalePendingRowsAndTheirObjects(t *testing.T) {
	f := newStarsFixture(t)
	old := time.Now().Add(-25 * time.Hour)
	stale := f.item(t, itemSpec{ws: f.wsA, node: oaOpen, name: "Bỏ dở.pdf", status: "pending", owner: f.caller, updated: old})
	fresh := f.item(t, itemSpec{ws: f.wsA, node: oaOpen, name: "Đang tải.pdf", status: "pending", owner: f.caller})
	activeOld := f.item(t, itemSpec{ws: f.wsA, node: oaOpen, name: "Đã xong.pdf", owner: f.caller, updated: old})

	n, err := f.svc.CleanupPendingUploads(context.Background(), 24*time.Hour)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, n, 1)

	assert.Equal(t, "gone", f.status(t, stale))
	assert.Equal(t, "pending", f.status(t, fresh))
	assert.Equal(t, "active", f.status(t, activeOld))

	// The stored object is removed on behalf of the person who started the upload.
	who, ok := f.docs.caller("drive/" + stale + "/Bỏ dở.pdf")
	require.True(t, ok, "the abandoned upload's object must be removed")
	assert.Equal(t, grpcauth.Caller{UserID: f.caller, NGACNodeID: f.callerNode}, who)
	_, ok = f.docs.caller("drive/" + fresh + "/Đang tải.pdf")
	assert.False(t, ok)
}
