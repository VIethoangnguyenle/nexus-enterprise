package grpc_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"ngac-platform/pkg/grpcauth"
	pb "ngac-platform/proto/drive"
	policypb "ngac-platform/proto/policy"
	"ngac-platform/services/drive/internal/domain"
	"ngac-platform/services/drive/internal/store"
)

// readablePolicy answers a batch check with read on exactly the objects in
// readable; everything else is denied. failErr makes every batch call fail.
type readablePolicy struct {
	mockPolicyRead
	readable map[string]bool
	allowAll bool
	failErr  error
}

func (p *readablePolicy) BatchCheckAccess(_ context.Context, req *policypb.BatchCheckAccessRequest, _ ...grpc.CallOption) (*policypb.BatchAccessResult, error) {
	if p.failErr != nil {
		return nil, p.failErr
	}
	results := make(map[string]*policypb.ObjectPermissions, len(req.ObjectIds))
	for _, id := range req.ObjectIds {
		perms := make(map[string]bool, len(req.Operations))
		for _, op := range req.Operations {
			perms[op] = p.allowAll || p.readable[id]
		}
		results[id] = &policypb.ObjectPermissions{Permissions: perms}
	}
	return &policypb.BatchAccessResult{Results: results}, nil
}

// searchFixture is a workspace the searcher belongs to, holding a folder the
// searcher may read and one they may not, plus a second workspace the searcher
// does not belong to, holding a folder whose node the policy would allow.
type searchFixture struct {
	pool                        *pgxpool.Pool
	wsA, wsB, searcher          string
	readable, hidden, otherWSID *pb.DriveItem
}

func newSearchFixture(t *testing.T) *searchFixture {
	t.Helper()
	srv, pool := setupServer(t)
	ctx := context.Background()
	tt := tenantFor(t)

	searcher := "drive-search-user-" + uuid.NewString()
	wsB := "drive-search-ws-" + uuid.NewString()
	_, err := pool.Exec(ctx, `INSERT INTO users (id, username, password) VALUES ($1, $1, '')`, searcher)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO workspaces (id, name, owner_id, type) VALUES ($1, $1, $2, 'organization')`, wsB, tt.userID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO tenant_users (tenant_id, user_id, role, status) VALUES ($1, $2, 'member', 'active')`, tt.wsID, searcher)
	require.NoError(t, err)
	t.Cleanup(func() {
		c := context.Background()
		for _, st := range []struct{ q, arg string }{
			{`DELETE FROM tenant_users WHERE user_id = $1`, searcher},
			{`DELETE FROM drive_items WHERE workspace_id = $1`, wsB},
			{`DELETE FROM drive_quotas WHERE workspace_id = $1`, wsB},
			{`DELETE FROM workspaces WHERE id = $1`, wsB},
			{`DELETE FROM users WHERE id = $1`, searcher},
		} {
			if _, err := pool.Exec(c, st.q, st.arg); err != nil {
				t.Errorf("fixture cleanup %q: %v", st.q, err)
			}
		}
	})

	owner := asCaller(tt.userID, "ngac-owner")
	mk := func(ws, name string) *pb.DriveItem {
		f, err := srv.CreateFolder(owner, &pb.CreateFolderRequest{WorkspaceId: ws, Name: name})
		require.NoError(t, err)
		return f
	}
	return &searchFixture{
		pool: pool, wsA: tt.wsID, wsB: wsB, searcher: searcher,
		readable:  mk(tt.wsID, "Budget Alpha"),
		hidden:    mk(tt.wsID, "Budget Beta"),
		otherWSID: mk(wsB, "Budget Gamma"),
	}
}

func (f *searchFixture) service(p *readablePolicy) *domain.Service {
	return domain.NewService(store.NewStore(f.pool), p, &mockPolicyWrite{}, &mockDocStorage{})
}

func (f *searchFixture) policyAllowingAll() *readablePolicy {
	return &readablePolicy{readable: map[string]bool{
		f.readable.NgacNodeId: true, f.hidden.NgacNodeId: true, f.otherWSID.NgacNodeId: true,
	}}
}

func (f *searchFixture) asSearcher() context.Context {
	return grpcauth.WithCaller(context.Background(), grpcauth.Caller{UserID: f.searcher, NGACNodeID: "ngac-searcher"})
}

func names(list *pb.DriveItemList) []string {
	out := make([]string, 0, len(list.GetItems()))
	for _, it := range list.GetItems() {
		out = append(out, it.Name)
	}
	return out
}

// A non-member naming a workspace is refused, even when the policy would let
// them read every item in it.
func TestSearchItems_NonMemberIsDenied(t *testing.T) {
	f := newSearchFixture(t)
	svc := f.service(f.policyAllowingAll())

	_, err := svc.SearchItems(f.asSearcher(), f.wsB, "Budget", 20)
	require.Error(t, err)
	assert.True(t, errors.Is(err, domain.ErrAccessDenied), "want access denied, got %v", err)
}

// A caller with no identity is refused before anything is looked up.
func TestSearchItems_AnonymousIsDenied(t *testing.T) {
	f := newSearchFixture(t)
	svc := f.service(f.policyAllowingAll())

	_, err := svc.SearchItems(context.Background(), f.wsA, "Budget", 20)
	require.Error(t, err)
	assert.True(t, errors.Is(err, domain.ErrAccessDenied), "want access denied, got %v", err)
}

// An item that matches the words but that the caller cannot read is neither
// returned nor counted.
func TestSearchItems_UnreadableItemIsNotReturned(t *testing.T) {
	f := newSearchFixture(t)
	svc := f.service(&readablePolicy{readable: map[string]bool{
		f.readable.NgacNodeId: true, f.otherWSID.NgacNodeId: true,
	}})

	list, err := svc.SearchItems(f.asSearcher(), f.wsA, "budget", 20)
	require.NoError(t, err)
	assert.Equal(t, []string{"Budget Alpha"}, names(list))
}

// An item of another workspace is never returned, even when readable.
func TestSearchItems_OtherWorkspaceItemIsNotReturned(t *testing.T) {
	f := newSearchFixture(t)
	svc := f.service(f.policyAllowingAll())

	list, err := svc.SearchItems(f.asSearcher(), f.wsA, "Budget", 20)
	require.NoError(t, err)
	got := names(list)
	assert.ElementsMatch(t, []string{"Budget Alpha", "Budget Beta"}, got)
	for _, it := range list.GetItems() {
		assert.Equal(t, f.wsA, it.WorkspaceId)
	}
}

// A policy failure fails closed: no partial list.
func TestSearchItems_PolicyFailureFailsClosed(t *testing.T) {
	f := newSearchFixture(t)
	p := f.policyAllowingAll()
	p.failErr = errors.New("policy unreachable")
	svc := f.service(p)

	list, err := svc.SearchItems(f.asSearcher(), f.wsA, "Budget", 20)
	require.Error(t, err)
	assert.Nil(t, list)
}

// A trashed item is not found.
func TestSearchItems_TrashedItemIsNotReturned(t *testing.T) {
	f := newSearchFixture(t)
	_, err := f.pool.Exec(context.Background(), `UPDATE drive_items SET status = 'trashed', trashed_at = NOW() WHERE id = $1`, f.hidden.Id)
	require.NoError(t, err)
	svc := f.service(f.policyAllowingAll())

	list, err := svc.SearchItems(f.asSearcher(), f.wsA, "Budget", 20)
	require.NoError(t, err)
	assert.Equal(t, []string{"Budget Alpha"}, names(list))
}

// A drive's root is where its top level hangs, not an item anyone looks for.
func TestSearchItems_DriveRootIsNotReturned(t *testing.T) {
	f := newSearchFixture(t)
	svc := f.service(&readablePolicy{allowAll: true})

	list, err := svc.SearchItems(f.asSearcher(), f.wsA, "Root", 20)
	require.NoError(t, err)
	assert.Empty(t, list.GetItems())
}

// LIKE wildcards in the words are taken literally.
func TestSearchItems_WildcardsAreLiteral(t *testing.T) {
	f := newSearchFixture(t)
	svc := f.service(f.policyAllowingAll())

	list, err := svc.SearchItems(f.asSearcher(), f.wsA, "%", 20)
	require.NoError(t, err)
	assert.Empty(t, list.GetItems())
}

// The limit caps what comes back.
func TestSearchItems_LimitIsHonoured(t *testing.T) {
	f := newSearchFixture(t)
	svc := f.service(f.policyAllowingAll())

	list, err := svc.SearchItems(f.asSearcher(), f.wsA, "Budget", 1)
	require.NoError(t, err)
	assert.Len(t, list.GetItems(), 1)
}
