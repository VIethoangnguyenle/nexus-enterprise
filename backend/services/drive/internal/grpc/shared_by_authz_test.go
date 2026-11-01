package grpc_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/ngac"
	"ngac-platform/pkg/grpcauth"
	pb "ngac-platform/proto/drive"
	"ngac-platform/services/drive/internal/domain"
	"ngac-platform/services/drive/internal/store"
)

// sharedByFixture: a viewer who belongs to workspace A, a person (the sharer)
// whose profile the viewer opened, and items of every kind the list must sort
// out: shared with the viewer or not, owned by the sharer or by someone else,
// readable or not, in A or in another workspace B, live or trashed.
type sharedByFixture struct {
	svcFor                 func(p *readablePolicy) *domain.Service
	wsA, wsB               string
	viewer, sharer         string // user ids
	viewerNode, sharerNode string
	sharedFolder, sharedFile, unreadable, otherWS, notShared, someoneElses, trashed *pb.DriveItem
}

func newSharedByFixture(t *testing.T) *sharedByFixture {
	t.Helper()
	srv, pool := setupServer(t)
	ctx := context.Background()
	tt := tenantFor(t)

	f := &sharedByFixture{
		wsA:        tt.wsID,
		wsB:        "drive-sharedby-ws-" + uuid.NewString(),
		viewer:     "drive-sharedby-viewer-" + uuid.NewString(),
		sharer:     "drive-sharedby-sharer-" + uuid.NewString(),
		viewerNode: "ngac-sharedby-viewer-" + uuid.NewString(),
		sharerNode: "ngac-sharedby-sharer-" + uuid.NewString(),
	}
	f.svcFor = func(p *readablePolicy) *domain.Service {
		return domain.NewService(store.NewStore(pool), p, &mockPolicyWrite{}, &mockDocStorage{})
	}
	exec := func(q string, args ...any) {
		t.Helper()
		_, err := pool.Exec(ctx, q, args...)
		require.NoError(t, err, q)
	}
	exec(`INSERT INTO ngac_nodes (id, name, node_type) VALUES ($1, $1, $2), ($3, $3, $2)`, f.viewerNode, ngac.TypeU, f.sharerNode)
	exec(`INSERT INTO users (id, username, password, ngac_node) VALUES ($1, $1, '', $2), ($3, $3, '', $4)`,
		f.viewer, f.viewerNode, f.sharer, f.sharerNode)
	exec(`INSERT INTO workspaces (id, name, owner_id, type) VALUES ($1, $1, $2, 'organization')`, f.wsB, tt.userID)
	exec(`INSERT INTO tenant_users (tenant_id, user_id, role, status) VALUES ($1, $2, 'member', 'active'), ($1, $3, 'member', 'active')`,
		f.wsA, f.viewer, f.sharer)
	// wsB records no Documents OA, so only its members may make its drive root.
	exec(`INSERT INTO tenant_users (tenant_id, user_id, role, status) VALUES ($1, $2, 'member', 'active')`, f.wsB, f.sharer)
	t.Cleanup(func() {
		c := context.Background()
		for _, st := range []struct{ q, arg string }{
			{`DELETE FROM tenant_users WHERE user_id = $1`, f.viewer},
			{`DELETE FROM tenant_users WHERE user_id = $1`, f.sharer},
			{`DELETE FROM drive_items WHERE workspace_id = $1`, f.wsB},
			{`DELETE FROM drive_quotas WHERE workspace_id = $1`, f.wsB},
			{`DELETE FROM workspaces WHERE id = $1`, f.wsB},
			{`DELETE FROM drive_items WHERE owner_id = $1`, f.sharer},
			{`DELETE FROM drive_items WHERE owner_id = $1`, f.sharerNode},
			{`DELETE FROM users WHERE id = $1`, f.viewer},
			{`DELETE FROM users WHERE id = $1`, f.sharer},
			{`DELETE FROM ngac_nodes WHERE id = $1`, f.viewerNode},
			{`DELETE FROM ngac_nodes WHERE id = $1`, f.sharerNode},
		} {
			if _, err := pool.Exec(c, st.q, st.arg); err != nil {
				t.Errorf("fixture cleanup %q: %v", st.q, err)
			}
		}
	})

	asSharer := asCaller(f.sharer, f.sharerNode)
	mk := func(as context.Context, ws, name string) *pb.DriveItem {
		t.Helper()
		item, err := srv.CreateFolder(as, &pb.CreateFolderRequest{WorkspaceId: ws, Name: name})
		require.NoError(t, err)
		return item
	}
	share := func(item *pb.DriveItem) {
		t.Helper()
		exec(`INSERT INTO drive_shares (id, drive_item_id, share_type, target_ngac_id, target_label, operations, ngac_share_oa, created_by)
			VALUES ($1, $2, 'user', $3, 'viewer', ARRAY['read'], 'share-oa', $4)`, uuid.NewString(), item.Id, f.viewerNode, f.sharerNode)
	}

	f.sharedFolder = mk(asSharer, f.wsA, "Shared folder")
	share(f.sharedFolder)
	// A file's owner is kept as the user id, a folder's as the NGAC node: both are the sharer's.
	f.sharedFile = mk(asSharer, f.wsA, "Shared file")
	exec(`UPDATE drive_items SET owner_id = $1 WHERE id = $2`, f.sharer, f.sharedFile.Id)
	share(f.sharedFile)
	f.unreadable = mk(asSharer, f.wsA, "Unreadable")
	share(f.unreadable)
	f.otherWS = mk(asSharer, f.wsB, "Other workspace")
	share(f.otherWS)
	f.notShared = mk(asSharer, f.wsA, "Not shared")
	f.someoneElses = mk(asCaller(tt.userID, "ngac-owner"), f.wsA, "Someone else's")
	share(f.someoneElses)
	f.trashed = mk(asSharer, f.wsA, "Trashed")
	share(f.trashed)
	exec(`UPDATE drive_items SET status = 'trashed', trashed_at = NOW() WHERE id = $1`, f.trashed.Id)
	return f
}

func (f *sharedByFixture) asViewer() context.Context {
	return grpcauth.WithCaller(context.Background(), grpcauth.Caller{UserID: f.viewer, NGACNodeID: f.viewerNode})
}

// readsAllBut lets the viewer read every item of the fixture except the
// unreadable one.
func (f *sharedByFixture) readsAllBut() *readablePolicy {
	readable := map[string]bool{}
	for _, it := range []*pb.DriveItem{f.sharedFolder, f.sharedFile, f.otherWS, f.notShared, f.someoneElses, f.trashed} {
		readable[it.NgacNodeId] = true
	}
	return &readablePolicy{readable: readable}
}

// Only the sharer's live items of this workspace, shared with the viewer and
// readable by them, are listed: an unreadable one, another workspace's, one not
// shared, someone else's and a trashed one are each left out.
func TestSharedBy_ListsOnlyReadableItemsOfThePersonInThisWorkspace(t *testing.T) {
	f := newSharedByFixture(t)
	list, err := f.svcFor(f.readsAllBut()).SharedWithMeBy(f.asViewer(), f.wsA, f.sharer)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"Shared folder", "Shared file"}, names(list))
	for _, it := range list.GetItems() {
		assert.Equal(t, f.wsA, it.WorkspaceId)
	}
}

// Deny: an item shared with the viewer that the policy no longer lets them read
// is neither listed nor counted.
func TestSharedBy_UnreadableItemIsNotListed(t *testing.T) {
	f := newSharedByFixture(t)
	list, err := f.svcFor(&readablePolicy{readable: map[string]bool{f.sharedFolder.NgacNodeId: true}}).
		SharedWithMeBy(f.asViewer(), f.wsA, f.sharer)
	require.NoError(t, err)
	assert.Equal(t, []string{"Shared folder"}, names(list))
}

// Deny: asking about a workspace the viewer is not a member of is refused, even
// though an item the sharer shared there would be readable.
func TestSharedBy_NonMemberIsDenied(t *testing.T) {
	f := newSharedByFixture(t)
	_, err := f.svcFor(&readablePolicy{allowAll: true}).SharedWithMeBy(f.asViewer(), f.wsB, f.sharer)
	require.Error(t, err)
	assert.True(t, errors.Is(err, domain.ErrAccessDenied), "want access denied, got %v", err)
}

// Deny: no identity, no answer.
func TestSharedBy_AnonymousIsDenied(t *testing.T) {
	f := newSharedByFixture(t)
	_, err := f.svcFor(&readablePolicy{allowAll: true}).SharedWithMeBy(context.Background(), f.wsA, f.sharer)
	require.Error(t, err)
	assert.True(t, errors.Is(err, domain.ErrAccessDenied), "want access denied, got %v", err)
}

// A policy failure is an error, never a list.
func TestSharedBy_PolicyFailureFailsClosed(t *testing.T) {
	f := newSharedByFixture(t)
	p := f.readsAllBut()
	p.failErr = errors.New("policy unreachable")
	list, err := f.svcFor(p).SharedWithMeBy(f.asViewer(), f.wsA, f.sharer)
	require.Error(t, err)
	assert.Nil(t, list)
}

// The person is required, and is never the viewer themself.
func TestSharedBy_RejectsAMalformedQuestion(t *testing.T) {
	f := newSharedByFixture(t)
	svc := f.svcFor(&readablePolicy{allowAll: true})
	for name, in := range map[string][2]string{
		"no workspace": {"", f.sharer},
		"no person":    {f.wsA, ""},
		"oneself":      {f.wsA, f.viewer},
	} {
		_, err := svc.SharedWithMeBy(f.asViewer(), in[0], in[1])
		assert.True(t, errors.Is(err, domain.ErrInvalidInput), name+": got %v", err)
	}
}
