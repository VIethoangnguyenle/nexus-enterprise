package texts_test

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"ngac-platform/ngac"
	policypb "ngac-platform/proto/policy"
	"ngac-platform/services/document/internal/texts"
	"ngac-platform/testutil"
)

// fakePolicy answers access checks from a table of {user node, OA, op} grants.
type fakePolicy struct {
	policypb.PolicyReadServiceClient
	mu     sync.Mutex
	grants map[[3]string]bool
	fail   error
	checks [][3]string
	// onCheck runs on every CheckAccess, to stage something between two checks.
	onCheck func(user, oa, op string)
}

func newFakePolicy() *fakePolicy { return &fakePolicy{grants: map[[3]string]bool{}} }

func (f *fakePolicy) grant(user, oa string, ops ...string) {
	for _, op := range ops {
		f.grants[[3]string{user, oa, op}] = true
	}
}

func (f *fakePolicy) CheckAccess(_ context.Context, r *policypb.CheckAccessRequest, _ ...grpc.CallOption) (*policypb.AccessDecision, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := [3]string{r.UserNodeId, r.ObjectNodeId, r.Operation}
	f.checks = append(f.checks, key)
	if f.onCheck != nil {
		f.onCheck(r.UserNodeId, r.ObjectNodeId, r.Operation)
	}
	if f.fail != nil {
		return nil, f.fail
	}
	if f.grants[key] {
		return &policypb.AccessDecision{Decision: ngac.DecisionAllow}, nil
	}
	return &policypb.AccessDecision{Decision: ngac.DecisionDeny}, nil
}

func (f *fakePolicy) BatchCheckAccess(_ context.Context, r *policypb.BatchCheckAccessRequest, _ ...grpc.CallOption) (*policypb.BatchAccessResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fail != nil {
		return nil, f.fail
	}
	res := map[string]*policypb.ObjectPermissions{}
	for _, oa := range r.ObjectIds {
		perms := map[string]bool{}
		for _, op := range r.Operations {
			perms[op] = f.grants[[3]string{r.UserNodeId, oa, op}]
		}
		res[oa] = &policypb.ObjectPermissions{Permissions: perms}
	}
	return &policypb.BatchAccessResult{Results: res}, nil
}

// fixture is one workspace with a Documents OA, two folders and three people:
// an owner, a reader and an outsider who belongs to nothing.
type fixture struct {
	t                    *testing.T
	pool                 *pgxpool.Pool
	wsID                 string
	docsOA               string
	folderA, folderB     string // drive folder ids
	oaA, oaB             string // their OAs
	owner, reader, other texts.Caller
	policy               *fakePolicy
	svc                  *texts.Service
}

func (f *fixture) exec(sql string, args ...any) {
	f.t.Helper()
	_, err := f.pool.Exec(context.Background(), sql, args...)
	require.NoError(f.t, err, sql)
}

func (f *fixture) oaNode(label string) string {
	id := "oa-" + label + "-" + uuid.NewString()[:8]
	f.exec(`INSERT INTO ngac_nodes (id, name, node_type, properties) VALUES ($1, $1, 'OA', '{}')`, id)
	f.t.Cleanup(func() { _, _ = f.pool.Exec(context.Background(), `DELETE FROM ngac_nodes WHERE id = $1`, id) })
	return id
}

func (f *fixture) folder(ws, oa, name string) string {
	id := uuid.NewString()
	f.exec(`INSERT INTO drive_items (id, workspace_id, item_type, name, ngac_node_id, owner_id, status)
	        VALUES ($1, $2, 'folder', $3, $4, 'system', 'active')`, id, ws, name, oa)
	return id
}

func (f *fixture) member(ws string, u texts.Caller) {
	f.exec(`INSERT INTO tenant_users (tenant_id, user_id, role, status) VALUES ($1, $2, 'member', 'active')`, ws, u.UserID)
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	pool := testutil.SetupTestDB(t)
	f := &fixture{t: t, pool: pool, policy: newFakePolicy()}

	person := func(name string) texts.Caller {
		uid, node := testutil.CreateUser(t, pool)
		f.exec(`UPDATE users SET display_name = $2 WHERE id = $1`, uid, name)
		return texts.Caller{UserID: uid, NGACNodeID: node}
	}
	f.owner, f.reader, f.other = person("Lê Thị Hoa"), person("Trần Minh Đức"), person("Người Ngoài")

	f.wsID, _ = testutil.CreateWorkspace(t, pool, f.owner.UserID)
	f.docsOA = f.oaNode("docs")
	f.exec(`UPDATE workspaces SET documents_oa_id = $1 WHERE id = $2`, f.docsOA, f.wsID)
	f.oaA, f.oaB = f.oaNode("a"), f.oaNode("b")
	f.folderA, f.folderB = f.folder(f.wsID, f.oaA, "A"), f.folder(f.wsID, f.oaB, "B")
	f.member(f.wsID, f.owner)
	f.member(f.wsID, f.reader)

	f.svc = texts.NewService(texts.NewStore(pool), f.policy)
	// Folders hold the documents; they go first so the workspace can be removed.
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM text_documents WHERE workspace_id = $1`, f.wsID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM drive_items WHERE workspace_id = $1`, f.wsID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM tenant_users WHERE tenant_id = $1`, f.wsID)
	})
	return f
}

// doc creates a document as the owner, granting write on its folder first.
func (f *fixture) doc(folder, oa, title string) *texts.Doc {
	f.t.Helper()
	f.policy.grant(f.owner.NGACNodeID, oa, ngac.OpRead, ngac.OpWrite)
	d, err := f.svc.Create(context.Background(), f.owner, f.wsID, folder, title)
	require.NoError(f.t, err)
	return d
}

// list is one default-sized page of a listing, as the screens first ask for it.
func (f *fixture) list(who texts.Caller, ws, scope string) ([]*texts.Opened, error) {
	page, err := f.svc.List(context.Background(), who, ws, scope, "", 0)
	if err != nil {
		return nil, err
	}
	return page.Docs, nil
}

func ptr[T any](v T) *T { return &v }

var _ = fmt.Sprintf

// testutilWorkspace makes a second workspace owned by the fixture's owner.
func testutilWorkspace(t *testing.T, f *fixture) (string, string) {
	t.Helper()
	return testutil.CreateWorkspace(t, f.pool, f.owner.UserID)
}
