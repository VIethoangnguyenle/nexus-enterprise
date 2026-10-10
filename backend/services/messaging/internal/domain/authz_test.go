package domain_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"ngac-platform/ngac"
	authpb "ngac-platform/proto/auth"
	policypb "ngac-platform/proto/policy"
	"ngac-platform/services/messaging/internal/domain"
	"ngac-platform/services/messaging/internal/store"
	"ngac-platform/testutil"
)

// ---------------------------------------------------------------------------
// Policy doubles that answer per (user, object, operation) and record what was
// asked. An allow-everything mock cannot tell a check on the right OA from a
// check on the wrong one, so these tests grant exactly one triple and assert
// that the decision hinged on it.
// ---------------------------------------------------------------------------

type grant struct{ user, object, op string }

type scriptedPolicyRead struct {
	policypb.PolicyReadServiceClient

	mu       sync.Mutex
	allow    map[grant]bool
	err      error
	children map[string][]*policypb.NGACNode
	checks   []grant
	noGlobal bool // PC_Global cannot be found
}

func (p *scriptedPolicyRead) CheckAccess(_ context.Context, req *policypb.CheckAccessRequest, _ ...grpc.CallOption) (*policypb.AccessDecision, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	g := grant{req.UserNodeId, req.ObjectNodeId, req.Operation}
	p.checks = append(p.checks, g)
	if p.err != nil {
		return nil, p.err
	}
	if p.allow[g] {
		return &policypb.AccessDecision{Decision: ngac.DecisionAllow}, nil
	}
	return &policypb.AccessDecision{Decision: ngac.DecisionDeny}, nil
}

func (p *scriptedPolicyRead) GetChildren(_ context.Context, req *policypb.GetChildrenRequest, _ ...grpc.CallOption) (*policypb.NodeList, error) {
	return &policypb.NodeList{Nodes: p.children[req.NodeId]}, nil
}

func (p *scriptedPolicyRead) FindNodeByName(_ context.Context, req *policypb.FindNodeByNameRequest, _ ...grpc.CallOption) (*policypb.NGACNode, error) {
	if req.Name == ngac.NodePCGlobal && req.NodeType == ngac.TypePC && !p.noGlobal {
		return &policypb.NGACNode{Id: "pc-global", Name: req.Name, NodeType: req.NodeType}, nil
	}
	return nil, errors.New("not found")
}

func (p *scriptedPolicyRead) asked() []grant {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]grant(nil), p.checks...)
}

// recordingPolicyWrite persists the nodes it creates (channels carry foreign
// keys onto ngac_nodes) and counts every graph write, so a denial can be shown
// to have left the graph untouched.
type recordingPolicyWrite struct {
	policypb.PolicyWriteServiceClient
	pool *pgxpool.Pool

	mu     sync.Mutex
	nodes  []string
	writes int
}

func (w *recordingPolicyWrite) CreateNode(ctx context.Context, req *policypb.CreateNodeRequest, _ ...grpc.CallOption) (*policypb.NGACNode, error) {
	id := fmt.Sprintf("test-node-%s-%d", req.Name, time.Now().UnixNano())
	if _, err := w.pool.Exec(ctx,
		`INSERT INTO ngac_nodes (id, name, node_type) VALUES ($1, $2, $3)`,
		id, req.Name, req.NodeType,
	); err != nil {
		return nil, err
	}
	w.mu.Lock()
	w.nodes = append(w.nodes, id)
	w.writes++
	w.mu.Unlock()
	return &policypb.NGACNode{Id: id, Name: req.Name, NodeType: req.NodeType}, nil
}

func (w *recordingPolicyWrite) CreateAssignment(_ context.Context, _ *policypb.CreateAssignmentRequest, _ ...grpc.CallOption) (*policypb.Assignment, error) {
	w.mu.Lock()
	w.writes++
	w.mu.Unlock()
	return &policypb.Assignment{Id: "assign"}, nil
}

func (w *recordingPolicyWrite) CreateAssociation(_ context.Context, _ *policypb.CreateAssociationRequest, _ ...grpc.CallOption) (*policypb.Association, error) {
	w.mu.Lock()
	w.writes++
	w.mu.Unlock()
	return &policypb.Association{Id: "assoc"}, nil
}

func (w *recordingPolicyWrite) graphWrites() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.writes
}

type stubAuth struct{ authpb.AuthServiceClient }

func (stubAuth) GetUserByID(_ context.Context, req *authpb.GetUserByIDRequest, _ ...grpc.CallOption) (*authpb.UserInfo, error) {
	return &authpb.UserInfo{Id: req.UserId, Username: "testuser"}, nil
}

// ---------------------------------------------------------------------------
// Fixture
// ---------------------------------------------------------------------------

type fixture struct {
	pool   *pgxpool.Pool
	read   *scriptedPolicyRead
	write  *recordingPolicyWrite
	svc    *domain.Service
	wsID   string
	wsName string
	pcID   string
	userID string
}

const userNode = "ngac-user-caller"

func newFixture(t *testing.T) *fixture {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		url = "postgres://ngac:ngac_secret@localhost:5432/ngac?sslmode=disable"
	}
	pool, err := pgxpool.New(context.Background(), url)
	require.NoError(t, err)
	if err := pool.Ping(context.Background()); err != nil {
		t.Skipf("test DB not available: %v", err)
	}
	t.Cleanup(pool.Close)

	f := &fixture{pool: pool}
	f.userID, _ = testutil.CreateUser(t, pool)
	f.wsID, f.pcID = testutil.CreateWorkspace(t, pool, f.userID)
	require.NoError(t, pool.QueryRow(context.Background(), `SELECT name FROM workspaces WHERE id = $1`, f.wsID).Scan(&f.wsName))

	f.read = &scriptedPolicyRead{allow: map[grant]bool{}, children: map[string][]*policypb.NGACNode{}}
	f.write = &recordingPolicyWrite{pool: pool}
	f.svc = domain.NewService(store.NewStore(pool), f.read, f.write, stubAuth{}, nil)

	t.Cleanup(func() {
		ctx := context.Background()
		for _, id := range f.write.nodes {
			pool.Exec(ctx, `DELETE FROM channel_members WHERE channel_id IN (SELECT id FROM channels WHERE ngac_oa_id = $1 OR ngac_ua_id = $1)`, id)
			pool.Exec(ctx, `DELETE FROM channels WHERE ngac_oa_id = $1 OR ngac_ua_id = $1`, id)
		}
		for _, id := range f.write.nodes {
			pool.Exec(ctx, `DELETE FROM ngac_nodes WHERE id = $1`, id)
		}
	})
	return f
}

// withChannelsOA places the workspace's ID-keyed Channels OA under its PC.
func (f *fixture) withChannelsOA() string {
	id := fmt.Sprintf("test-channels-oa-%d", time.Now().UnixNano())
	f.read.children[f.pcID] = append(f.read.children[f.pcID], &policypb.NGACNode{
		Id: id, Name: ngac.ChannelsOAName(ngac.WorkspaceID(f.wsID)), NodeType: ngac.TypeOA,
	})
	return id
}

// insertChannel writes a channel row whose content OA exists in ngac_nodes.
func (f *fixture) insertChannel(t *testing.T, name string) (chID, oaID string) {
	t.Helper()
	ctx := context.Background()
	n := time.Now().UnixNano()
	chID = fmt.Sprintf("test-authz-ch-%d", n)
	oaID = fmt.Sprintf("test-authz-oa-%d", n)
	_, err := f.pool.Exec(ctx, `INSERT INTO ngac_nodes (id, name, node_type) VALUES ($1, $2, $3)`,
		oaID, ngac.ChannelContentOAName(ngac.ChannelID(chID)), ngac.TypeOA)
	require.NoError(t, err)
	_, err = f.pool.Exec(ctx,
		`INSERT INTO channels (id, name, channel_type, workspace_id, ngac_oa_id, created_at) VALUES ($1, $2, 'workspace', $3, $4, now())`,
		chID, name, f.wsID, oaID)
	require.NoError(t, err)
	t.Cleanup(func() {
		f.pool.Exec(ctx, `DELETE FROM channels WHERE id = $1`, chID)
		f.pool.Exec(ctx, `DELETE FROM ngac_nodes WHERE id = $1`, oaID)
	})
	return chID, oaID
}

func (f *fixture) channelCount(t *testing.T, name string) int {
	t.Helper()
	var n int
	require.NoError(t, f.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM channels WHERE name = $1`, name).Scan(&n))
	return n
}

func (f *fixture) createInput(name string) domain.CreateChannelInput {
	return domain.CreateChannelInput{
		Name: name, WorkspaceID: f.wsID, UserID: f.userID,
		UserNodeID: userNode, ChannelType: "workspace",
	}
}

// ---------------------------------------------------------------------------
// CreateChannel — create_channel on the workspace's Channels OA
// ---------------------------------------------------------------------------

func TestCreateChannel_DeniedWithoutCreateChannelOnChannelsOA(t *testing.T) {
	f := newFixture(t)
	channelsOA := f.withChannelsOA()
	// Everything a plain channel member could hold — but not create_channel.
	for _, op := range []string{ngac.OpRead, ngac.OpWrite, ngac.OpInvite, ngac.OpManage} {
		f.read.allow[grant{userNode, channelsOA, op}] = true
	}
	name := fmt.Sprintf("authz-deny-%d", time.Now().UnixNano())

	_, err := f.svc.CreateChannel(context.Background(), f.createInput(name))

	require.ErrorIs(t, err, domain.ErrAccessDenied)
	assert.Equal(t, 0, f.channelCount(t, name), "a denied create must not persist a channel")
	assert.Equal(t, 0, f.write.graphWrites(), "a denied create must not write to the NGAC graph")
	assert.Contains(t, f.read.asked(), grant{userNode, channelsOA, ngac.OpCreateChannel})
}

func TestCreateChannel_AllowedWithCreateChannelOnChannelsOA(t *testing.T) {
	f := newFixture(t)
	channelsOA := f.withChannelsOA()
	f.read.allow[grant{userNode, channelsOA, ngac.OpCreateChannel}] = true
	name := fmt.Sprintf("authz-allow-%d", time.Now().UnixNano())

	ch, err := f.svc.CreateChannel(context.Background(), f.createInput(name))

	require.NoError(t, err)
	assert.Equal(t, name, ch.Name)
	assert.Equal(t, 1, f.channelCount(t, name))
}

// The grant must be on this workspace's Channels OA. Holding create_channel on
// some other node — say another workspace's Channels OA — is not enough.
func TestCreateChannel_DeniedWhenGrantIsOnAnotherOA(t *testing.T) {
	f := newFixture(t)
	f.withChannelsOA()
	f.read.allow[grant{userNode, "some-other-workspace-channels-oa", ngac.OpCreateChannel}] = true
	name := fmt.Sprintf("authz-otheroa-%d", time.Now().UnixNano())

	_, err := f.svc.CreateChannel(context.Background(), f.createInput(name))

	require.ErrorIs(t, err, domain.ErrAccessDenied)
	assert.Equal(t, 0, f.channelCount(t, name))
}

// Only the ID-keyed Channels OA is authoritative. A name-keyed node is what two
// workspaces with the same display name would share, so resolving onto it is a
// cross-tenant hazard — the create is denied rather than authorized against it.
func TestCreateChannel_DeniedWhenOnlyLegacyNameKeyedOAExists(t *testing.T) {
	f := newFixture(t)
	legacy := "test-legacy-channels-oa"
	f.read.children[f.pcID] = []*policypb.NGACNode{{Id: legacy, Name: ngac.ChannelsOAName(ngac.WorkspaceID(f.wsName)), NodeType: ngac.TypeOA}}
	f.read.allow[grant{userNode, legacy, ngac.OpCreateChannel}] = true
	name := fmt.Sprintf("authz-legacy-%d", time.Now().UnixNano())

	_, err := f.svc.CreateChannel(context.Background(), f.createInput(name))

	require.Error(t, err)
	assert.Equal(t, 0, f.channelCount(t, name))
	assert.Equal(t, 0, f.write.graphWrites())
}

func TestCreateChannel_DeniedWhenPolicyUnavailable(t *testing.T) {
	f := newFixture(t)
	f.withChannelsOA()
	f.read.err = errors.New("connection refused")
	name := fmt.Sprintf("authz-down-%d", time.Now().UnixNano())

	_, err := f.svc.CreateChannel(context.Background(), f.createInput(name))

	require.ErrorIs(t, err, domain.ErrAccessDenied)
	assert.Equal(t, 0, f.channelCount(t, name))
}

// A workspace-less channel has no Channels OA to authorize against. The public
// entry point must not become a way round the check; DMs have their own path.
func TestCreateChannel_RejectedWithoutWorkspace(t *testing.T) {
	f := newFixture(t)
	in := f.createInput(fmt.Sprintf("authz-nows-%d", time.Now().UnixNano()))
	in.WorkspaceID = ""

	_, err := f.svc.CreateChannel(context.Background(), in)

	require.Error(t, err)
	assert.Equal(t, 0, f.write.graphWrites())
}

// ---------------------------------------------------------------------------
// UpdateChannel — manage on the channel's content OA
// ---------------------------------------------------------------------------

func TestUpdateChannel_DeniedWithoutManage(t *testing.T) {
	f := newFixture(t)
	chID, oaID := f.insertChannel(t, "authz-original")
	// A channel member: read, write, invite — not manage.
	for _, op := range ngac.ChannelMemberOps() {
		f.read.allow[grant{userNode, oaID, op}] = true
	}

	_, err := f.svc.UpdateChannel(context.Background(), chID, userNode, "authz-renamed")

	require.ErrorIs(t, err, domain.ErrAccessDenied)
	var name string
	require.NoError(t, f.pool.QueryRow(context.Background(), `SELECT name FROM channels WHERE id = $1`, chID).Scan(&name))
	assert.Equal(t, "authz-original", name, "a denied rename must not change the channel")
	assert.Contains(t, f.read.asked(), grant{userNode, oaID, ngac.OpManage})
}

func TestUpdateChannel_DeniedWhenPolicyUnavailable(t *testing.T) {
	f := newFixture(t)
	chID, _ := f.insertChannel(t, "authz-original")
	f.read.err = errors.New("connection refused")

	_, err := f.svc.UpdateChannel(context.Background(), chID, userNode, "authz-renamed")

	require.ErrorIs(t, err, domain.ErrAccessDenied)
}

func TestUpdateChannel_AllowedWithManage(t *testing.T) {
	f := newFixture(t)
	chID, oaID := f.insertChannel(t, "authz-original")
	f.read.allow[grant{userNode, oaID, ngac.OpManage}] = true

	ch, err := f.svc.UpdateChannel(context.Background(), chID, userNode, "authz-renamed")

	require.NoError(t, err)
	assert.Equal(t, "authz-renamed", ch.Name)
}

// ---------------------------------------------------------------------------
// AuthorizeChannelAccess — the check the WebSocket hub delegates to
// ---------------------------------------------------------------------------

func TestAuthorizeChannelAccess(t *testing.T) {
	f := newFixture(t)
	chID, oaID := f.insertChannel(t, "authz-ws")
	f.read.allow[grant{userNode, oaID, ngac.OpRead}] = true

	require.NoError(t, f.svc.AuthorizeChannelAccess(context.Background(), chID, userNode, ngac.OpRead))
	require.ErrorIs(t, f.svc.AuthorizeChannelAccess(context.Background(), chID, "ngac-outsider", ngac.OpRead), domain.ErrAccessDenied)
	require.Error(t, f.svc.AuthorizeChannelAccess(context.Background(), "no-such-channel", userNode, ngac.OpRead))
}
