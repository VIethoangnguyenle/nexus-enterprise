package grpc_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"ngac-platform/ngac"
	pb "ngac-platform/proto/asset"
	policypb "ngac-platform/proto/policy"
	agrpc "ngac-platform/services/asset/internal/grpc"
	"ngac-platform/testutil"
)

// The asset tree is a run of graph writes followed by the type's row. These
// tests check what it builds and that a failure anywhere in the run takes
// every node it created back out.

// freshRead is a graph with nothing in it but the workspace's Owners UA and its
// Mgmt OA, which is what the first asset type of a workspace finds.
func (f *fixture) freshRead() *fakePolicyRead {
	p := newFakePolicy()
	p.nodes[ngac.MgmtOAName(ngac.WorkspaceID(f.wsID))] = f.mgmtOA()
	p.nodes[ngac.OwnersUAName(ngac.WorkspaceID(f.wsID))] = "ua-owners:" + f.wsID
	p.grant("n-admin", f.mgmtOA(), ngac.OpManage)
	return p
}

func (f *fixture) newTypeReq(name, category string) *pb.CreateTypeRequest {
	return &pb.CreateTypeRequest{Name: name, Category: category, WorkspaceId: f.wsID}
}

func (f *fixture) typeCountNamed(t *testing.T, names ...string) int {
	t.Helper()
	var n int
	require.NoError(t, f.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM asset_types WHERE workspace_id = $1 AND name = ANY($2)`, f.wsID, names).Scan(&n))
	return n
}

func (f *fixture) cleanTypes(t *testing.T, names ...string) {
	t.Helper()
	t.Cleanup(func() {
		f.pool.Exec(context.Background(), `DELETE FROM asset_types WHERE workspace_id = $1 AND name = ANY($2)`, f.wsID, names)
	})
}

func TestCreateType_BuildsTheTreeUnderTheWorkspacePCWithIDKeyedNames(t *testing.T) {
	f := newFixture(t)
	p := f.freshRead()
	w := testutil.NewFakePolicyWrite()
	w.NextIDs = []string{"oa-assets", "oa-category", f.oaA}
	w.Seed(&policypb.NGACNode{Id: "ua-owners:" + f.wsID, Name: ngac.OwnersUAName(ngac.WorkspaceID(f.wsID)), NodeType: ngac.TypeUA})
	f.cleanTypes(t, "Laptop")
	srv := agrpc.NewAssetTypeServer(f.st, p, w)

	at, err := srv.CreateType(asCaller("admin", "n-admin"), f.newTypeReq("Laptop", "Hardware"))

	require.NoError(t, err)
	assert.Equal(t, f.oaA, at.NgacOaId)
	typeOA := ngac.AssetTypeOAName(ngac.WorkspaceID(f.wsID), ngac.AssetTypeID(at.Id))
	assert.ElementsMatch(t, []string{
		"OA " + ngac.AssetsOAName(ngac.WorkspaceID(f.wsID)),
		"OA " + ngac.AssetCategoryOAName(ngac.WorkspaceID(f.wsID), "Hardware"),
		"OA " + typeOA,
		"UA " + ngac.OwnersUAName(ngac.WorkspaceID(f.wsID)),
	}, w.LiveNodes())

	var pc string
	require.NoError(t, f.pool.QueryRow(context.Background(), `SELECT ngac_pc_id FROM workspaces WHERE id = $1`, f.wsID).Scan(&pc))
	log := w.Log()
	assert.Contains(t, log, fmt.Sprintf("assign %s>%s", ngac.AssetsOAName(ngac.WorkspaceID(f.wsID)), pc), "the tree hangs under the workspace PC")
	assert.Contains(t, log, fmt.Sprintf("assign %s>%s", ngac.AssetCategoryOAName(ngac.WorkspaceID(f.wsID), "Hardware"), ngac.AssetsOAName(ngac.WorkspaceID(f.wsID))))
	assert.Contains(t, log, fmt.Sprintf("assign %s>%s", typeOA, ngac.AssetCategoryOAName(ngac.WorkspaceID(f.wsID), "Hardware")))
	assert.Contains(t, log, fmt.Sprintf("associate %s>%s %v", ngac.OwnersUAName(ngac.WorkspaceID(f.wsID)), ngac.AssetsOAName(ngac.WorkspaceID(f.wsID)), ngac.AllOwnerOps()),
		"the Owners UA is granted the owner operations on the Assets OA")
	for _, l := range log {
		assert.NotContains(t, l, "AssetManagement", "no second policy class: an access must reach every class the object reaches")
	}
}

// Two types whose names sanitize to the same string used to share one OA name.
func TestCreateType_TwoTypesWithTheSameSanitizedNameGetDistinctOAs(t *testing.T) {
	f := newFixture(t)
	p := f.freshRead()
	w := testutil.NewFakePolicyWrite()
	w.NextIDs = []string{"oa-assets", "oa-category", f.oaA, f.oaB}
	f.cleanTypes(t, "Dell XPS", "Dell-XPS")
	srv := agrpc.NewAssetTypeServer(f.st, p, w)

	a, err := srv.CreateType(asCaller("admin", "n-admin"), f.newTypeReq("Dell XPS", "Hardware"))
	require.NoError(t, err)
	// The category and Assets OAs exist now, as they would in the graph.
	p.nodes[ngac.AssetsOAName(ngac.WorkspaceID(f.wsID))] = "oa-assets"
	p.nodes[ngac.AssetCategoryOAName(ngac.WorkspaceID(f.wsID), "Hardware")] = "oa-category"
	p.grant("n-admin", "oa-assets", ngac.OpManage) // from here on manage is checked on the Assets OA
	b, err := srv.CreateType(asCaller("admin", "n-admin"), f.newTypeReq("Dell-XPS", "Hardware"))
	require.NoError(t, err)

	assert.NotEqual(t, a.Id, b.Id)
	assert.Contains(t, w.LiveNodes(), "OA "+ngac.AssetTypeOAName(ngac.WorkspaceID(f.wsID), ngac.AssetTypeID(a.Id)))
	assert.Contains(t, w.LiveNodes(), "OA "+ngac.AssetTypeOAName(ngac.WorkspaceID(f.wsID), ngac.AssetTypeID(b.Id)))
	created := 0
	for _, l := range w.Log() {
		if strings.HasPrefix(l, "create OA ") && strings.Contains(l, "_Type_") {
			created++
		}
	}
	assert.Equal(t, 2, created, "each type has an OA of its own")
}

// The Assets and category OAs of a workspace that already has them are found,
// not created again.
func TestCreateType_ReusesTheAssetsAndCategoryOAs(t *testing.T) {
	f := newFixture(t)
	p := f.freshRead()
	p.nodes[ngac.AssetsOAName(ngac.WorkspaceID(f.wsID))] = "oa-assets"
	p.nodes[ngac.AssetCategoryOAName(ngac.WorkspaceID(f.wsID), "Hardware")] = "oa-category"
	p.grant("n-admin", "oa-assets", ngac.OpManage)
	w := testutil.NewFakePolicyWrite()
	w.NextIDs = []string{f.oaA}
	f.cleanTypes(t, "Laptop")

	_, err := agrpc.NewAssetTypeServer(f.st, p, w).CreateType(asCaller("admin", "n-admin"), f.newTypeReq("Laptop", "Hardware"))

	require.NoError(t, err)
	creates := 0
	for _, l := range w.Log() {
		if strings.HasPrefix(l, "create ") {
			creates++
		}
	}
	assert.Equal(t, 1, creates, "only the type's own OA is new")
}

func TestCreateType_RollsBackWhereverItFails(t *testing.T) {
	// 3 nodes, 3 assignments, 1 association.
	const writes = 7
	for k := 1; k <= writes; k++ {
		f := newFixture(t)
		p := f.freshRead()
		w := testutil.NewFakePolicyWrite()
		w.FailAt = k
		srv := agrpc.NewAssetTypeServer(f.st, p, w)

		_, err := srv.CreateType(asCaller("admin", "n-admin"), f.newTypeReq("Rollback", "Hardware"))

		require.Errorf(t, err, "failure injected at write %d must surface", k)
		var created, deleted []string
		for _, l := range w.Log() {
			if rest, ok := strings.CutPrefix(l, "create "); ok {
				created = append(created, rest[strings.Index(rest, " ")+1:])
			}
			if name, ok := strings.CutPrefix(l, "delete "); ok {
				deleted = append(deleted, name)
			}
		}
		for i, j := 0, len(created)-1; i < j; i, j = i+1, j-1 {
			created[i], created[j] = created[j], created[i]
		}
		assert.Equalf(t, created, deleted, "failure at write %d: newest first", k)
		assert.Emptyf(t, w.LiveNodes(), "failure at write %d left nodes behind", k)
		assert.Zero(t, f.typeCountNamed(t, "Rollback"), "and no type row")
	}
}

// The row is the last step. Its failure (here: an OA ID that is not in
// ngac_nodes) removes the whole tree.
func TestCreateType_RollsBackWhenTheRowIsRefused(t *testing.T) {
	f := newFixture(t)
	p := f.freshRead()
	w := testutil.NewFakePolicyWrite() // hands out IDs no foreign key will accept
	srv := agrpc.NewAssetTypeServer(f.st, p, w)

	_, err := srv.CreateType(asCaller("admin", "n-admin"), f.newTypeReq("Refused", "Hardware"))

	require.Error(t, err)
	assert.Equal(t, 7, w.Calls(), "every graph write happened before the row was refused")
	assert.Empty(t, w.LiveNodes())
}

// failingFind fails the lookup of one name, as an unavailable policy service
// would, and answers the rest from the wrapped fake.
type failingFind struct {
	*fakePolicyRead
	name string
}

func (r failingFind) FindNodeByName(ctx context.Context, req *policypb.FindNodeByNameRequest, opts ...grpc.CallOption) (*policypb.NGACNode, error) {
	if req.Name == r.name {
		return nil, errors.New("policy unavailable")
	}
	return r.fakePolicyRead.FindNodeByName(ctx, req, opts...)
}

// Deny: a lookup that fails is not an absent node. The category OA is not
// created a second time because the policy service did not answer.
func TestCreateType_DoesNotCreateWhenALookupFails(t *testing.T) {
	f := newFixture(t)
	p := failingFind{f.freshRead(), ngac.AssetCategoryOAName(ngac.WorkspaceID(f.wsID), "Hardware")}
	w := testutil.NewFakePolicyWrite()
	srv := agrpc.NewAssetTypeServer(f.st, p, w)

	_, err := srv.CreateType(asCaller("admin", "n-admin"), f.newTypeReq("Lookup", "Hardware"))

	require.Error(t, err)
	assert.Empty(t, w.LiveNodes())
	for _, l := range w.Log() {
		assert.NotContains(t, l, "Category", "the category OA must not be created on a failed lookup")
	}
}

// A workspace with no policy class cannot hold an asset tree: building one
// under nothing would leave OAs no user can ever reach.
func TestCreateType_RefusedForAWorkspaceWithoutAPolicyClass(t *testing.T) {
	f := newFixture(t)
	_, err := f.pool.Exec(context.Background(), `UPDATE workspaces SET ngac_pc_id = NULL WHERE id = $1`, f.wsID)
	require.NoError(t, err)
	w := testutil.NewFakePolicyWrite()

	_, err = agrpc.NewAssetTypeServer(f.st, f.freshRead(), w).CreateType(asCaller("admin", "n-admin"), f.newTypeReq("NoPC", "Hardware"))

	require.Error(t, err)
	assert.Zero(t, w.Calls())
}
