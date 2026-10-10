package rest

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"ngac-platform/ngac"
	"ngac-platform/pkg/httputil"
	policypb "ngac-platform/proto/policy"
	"ngac-platform/services/asset/internal/domain"
	agrpc "ngac-platform/services/asset/internal/grpc"
	"ngac-platform/services/asset/internal/store"
)

// End-to-end through the REST handlers and the real gRPC servers, against the
// test DB. Only the policy service is faked. These pin that the identity the
// guards and the audit columns see is the one in the token, whatever the body
// says.

type grantPolicy struct {
	policypb.PolicyReadServiceClient
	grants map[[3]string]bool // {user node, object, op}
}

func (g *grantPolicy) CheckAccess(_ context.Context, req *policypb.CheckAccessRequest, _ ...grpc.CallOption) (*policypb.AccessDecision, error) {
	if g.grants[[3]string{req.UserNodeId, req.ObjectNodeId, req.Operation}] {
		return &policypb.AccessDecision{Decision: ngac.DecisionAllow}, nil
	}
	return &policypb.AccessDecision{Decision: ngac.DecisionDeny}, nil
}

func (g *grantPolicy) FindNodeByName(_ context.Context, req *policypb.FindNodeByNameRequest, _ ...grpc.CallOption) (*policypb.NGACNode, error) {
	return nil, status.Errorf(codes.NotFound, "node %s not found", req.Name)
}

type flowFixture struct {
	pool                *pgxpool.Pool
	h                   *Handler
	policy              *grantPolicy
	wsID, typeID, oaID  string
	assetID, requestID  string
	requester, approver string // users.id; NGAC nodes are "n-" + id
}

func flowDBURL() string {
	if url := os.Getenv("TEST_DATABASE_URL"); url != "" {
		return url
	}
	return "postgres://ngac:ngac_secret@localhost:5432/ngac?sslmode=disable"
}

func nodeOf(userID string) string { return "n-" + userID }

func newFlowFixture(t *testing.T) *flowFixture {
	t.Helper()
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, flowDBURL())
	if err != nil {
		t.Fatalf("connect to test DB: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		t.Skipf("test DB not available: %v", err)
	}
	t.Cleanup(pool.Close)

	f := &flowFixture{pool: pool, policy: &grantPolicy{grants: map[[3]string]bool{}}}
	if err := pool.QueryRow(ctx, "SELECT id FROM ngac_nodes WHERE node_type = 'OA' ORDER BY id LIMIT 1").Scan(&f.oaID); err != nil {
		t.Skipf("no OA node in test DB: %v", err)
	}
	sfx := uuid.New().String()[:8]
	f.requester, f.approver = "flow-req-"+sfx, "flow-appr-"+sfx
	f.wsID, f.typeID = "flow-ws-"+sfx, "flow-type-"+sfx
	f.assetID, f.requestID = "flow-asset-"+sfx, "flow-request-"+sfx
	lifecycle, err := json.Marshal(domain.DefaultLifecycle())
	require.NoError(t, err)

	exec := func(q string, args ...any) {
		_, err := pool.Exec(ctx, q, args...)
		require.NoError(t, err, q)
	}
	exec(`INSERT INTO users (id, username, password) VALUES ($1, $1, ''), ($2, $2, '')`, f.requester, f.approver)
	exec(`INSERT INTO workspaces (id, name, owner_id) VALUES ($1, $1, $2)`, f.wsID, f.approver)
	exec(`INSERT INTO asset_types (id, name, category, workspace_id, ngac_oa_id, lifecycle)
	      VALUES ($1, $1, 'hardware', $2, $3, $4)`, f.typeID, f.wsID, f.oaID, lifecycle)
	exec(`INSERT INTO assets (id, name, type_id, workspace_id, state, created_by)
	      VALUES ($1, $1, $2, $3, 'requested', $4)`, f.assetID, f.typeID, f.wsID, f.approver)
	exec(`INSERT INTO asset_requests (id, type_id, workspace_id, requester_id, justification)
	      VALUES ($1, $2, $3, $4, 'need one')`, f.requestID, f.typeID, f.wsID, f.requester)
	t.Cleanup(func() {
		c := context.Background()
		pool.Exec(c, `DELETE FROM asset_requests WHERE workspace_id = $1`, f.wsID)
		pool.Exec(c, `DELETE FROM asset_transitions WHERE asset_id = $1`, f.assetID)
		pool.Exec(c, `DELETE FROM assets WHERE workspace_id = $1`, f.wsID)
		pool.Exec(c, `DELETE FROM asset_types WHERE workspace_id = $1`, f.wsID)
		pool.Exec(c, `DELETE FROM workspaces WHERE id = $1`, f.wsID)
		pool.Exec(c, `DELETE FROM users WHERE id IN ($1, $2)`, f.requester, f.approver)
	})

	st := store.New(pool)
	f.h = NewHandler(
		agrpc.NewAssetServer(st, f.policy, nil),
		agrpc.NewAssetTypeServer(st, f.policy, nil),
		agrpc.NewAssetRequestServer(st, f.policy, nil, nil),
	)
	// Both users may approve this type and act on this asset, so only the
	// identity rules — not missing grants — decide the outcomes below.
	for _, u := range []string{f.requester, f.approver} {
		f.policy.grants[[3]string{nodeOf(u), f.oaID, ngac.OpApprove}] = true
	}
	return f
}

// serve runs a handler as userID (claims), with the given body and path param.
func (f *flowFixture) serve(t *testing.T, userID, param, value, body string, h echo.HandlerFunc) int {
	t.Helper()
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames(param)
	c.SetParamValues(value)
	httputil.SetClaims(c, &httputil.Claims{UserID: userID, NGACNodeID: nodeOf(userID)})
	if err := h(c); err != nil {
		if he, ok := err.(*echo.HTTPError); ok {
			return he.Code
		}
		return http.StatusInternalServerError
	}
	return rec.Code
}

func (f *flowFixture) requestState(t *testing.T) (statusStr string, approver *string) {
	t.Helper()
	require.NoError(t, f.pool.QueryRow(context.Background(),
		`SELECT status, approver_id FROM asset_requests WHERE id = $1`, f.requestID).Scan(&statusStr, &approver))
	return statusStr, approver
}

// forged returns a body claiming to be someone else, in every spelling a
// binder might pick up.
func forged(userID string, extra string) string {
	return `{` + extra + `"user_id":"` + userID + `","UserId":"` + userID +
		`","user_ngac_node_id":"` + nodeOf(userID) + `","UserNgacNodeId":"` + nodeOf(userID) + `"}`
}

func TestApproveOverREST_SelfApprovalRejected(t *testing.T) {
	f := newFlowFixture(t)

	code := f.serve(t, f.requester, "reqId", f.requestID, `{}`, f.h.ApproveAssetRequest)

	assert.Equal(t, http.StatusForbidden, code)
	st, approver := f.requestState(t)
	assert.Equal(t, "pending", st)
	assert.Nil(t, approver)
}

func TestApproveOverREST_SelfApprovalWithForgedBodyUserRejected(t *testing.T) {
	f := newFlowFixture(t)

	code := f.serve(t, f.requester, "reqId", f.requestID, forged(f.approver, ""), f.h.ApproveAssetRequest)

	assert.Equal(t, http.StatusForbidden, code)
	st, approver := f.requestState(t)
	assert.Equal(t, "pending", st)
	assert.Nil(t, approver)
}

func TestApproveOverREST_RecordsApproverFromToken(t *testing.T) {
	f := newFlowFixture(t)

	// The body claims to be the requester; the token is the approver.
	code := f.serve(t, f.approver, "reqId", f.requestID, forged(f.requester, ""), f.h.ApproveAssetRequest)

	require.Equal(t, http.StatusOK, code)
	st, approver := f.requestState(t)
	assert.Equal(t, "approved", st)
	require.NotNil(t, approver)
	assert.Equal(t, f.approver, *approver)
}

func TestRejectOverREST_RecordsApproverFromToken(t *testing.T) {
	f := newFlowFixture(t)

	code := f.serve(t, f.approver, "reqId", f.requestID, forged(f.requester, `"reason":"not now",`), f.h.RejectAssetRequest)

	require.Equal(t, http.StatusOK, code)
	st, approver := f.requestState(t)
	assert.Equal(t, "rejected", st)
	require.NotNil(t, approver)
	assert.Equal(t, f.approver, *approver)
}

func TestRejectOverREST_DeniedWithoutApprove(t *testing.T) {
	f := newFlowFixture(t)
	outsider := "flow-out-" + uuid.New().String()[:8]

	code := f.serve(t, outsider, "reqId", f.requestID, forged(f.approver, `"reason":"x",`), f.h.RejectAssetRequest)

	assert.Equal(t, http.StatusForbidden, code)
	st, approver := f.requestState(t)
	assert.Equal(t, "pending", st)
	assert.Nil(t, approver)
}

func TestTransitionOverREST_RecordsActorFromToken(t *testing.T) {
	f := newFlowFixture(t)

	// "requested" -> "available" is the default lifecycle's "approve".
	code := f.serve(t, f.approver, "assetId", f.assetID,
		forged(f.requester, `"action":"approve","comment":"ok",`), f.h.TransitionAsset)

	require.Equal(t, http.StatusOK, code)
	var actor, toState string
	require.NoError(t, f.pool.QueryRow(context.Background(),
		`SELECT actor_id, to_state FROM asset_transitions WHERE asset_id = $1`, f.assetID).Scan(&actor, &toState))
	assert.Equal(t, f.approver, actor)
	assert.Equal(t, "available", toState)
}

// ---------------------------------------------------------------------------
// Approve and hand over an asset, end to end over REST
// ---------------------------------------------------------------------------

// freeAsset adds an available asset of the fixture's type and makes the
// requester a member of the workspace, which is what receiving one needs.
func (f *flowFixture) freeAsset(t *testing.T) string {
	t.Helper()
	ctx := context.Background()
	id := "flow-free-" + uuid.New().String()[:8]
	_, err := f.pool.Exec(ctx, `INSERT INTO tenant_users (tenant_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, f.wsID, f.requester)
	require.NoError(t, err)
	_, err = f.pool.Exec(ctx, `INSERT INTO assets (id, name, type_id, workspace_id, state, created_by) VALUES ($1, $1, $2, $3, 'available', $4)`,
		id, f.typeID, f.wsID, f.approver)
	require.NoError(t, err)
	t.Cleanup(func() { f.pool.Exec(context.Background(), `DELETE FROM asset_transitions WHERE asset_id = $1`, id) })
	return id
}

func (f *flowFixture) holder(t *testing.T, assetID string) (state string, holder *string) {
	t.Helper()
	require.NoError(t, f.pool.QueryRow(context.Background(),
		`SELECT state, assigned_to FROM assets WHERE id = $1`, assetID).Scan(&state, &holder))
	return state, holder
}

func TestApproveWithAssetOverREST_NeedsApproveAndManageAndChangesNothingOtherwise(t *testing.T) {
	f := newFlowFixture(t)
	asset := f.freeAsset(t)
	body := `{"asset_id":"` + asset + `","comment":"Máy mới"}`

	// Approve alone: the hand-over half is refused, so nothing happens.
	code := f.serve(t, f.approver, "reqId", f.requestID, body, f.h.ApproveAssetRequest)
	assert.Equal(t, http.StatusForbidden, code)
	st, _ := f.requestState(t)
	assert.Equal(t, "pending", st)
	state, holder := f.holder(t, asset)
	assert.Equal(t, "available", state)
	assert.Nil(t, holder)

	// Approve and manage: request and asset change together, recorded as the token's user.
	f.policy.grants[[3]string{nodeOf(f.approver), f.oaID, ngac.OpManage}] = true
	code = f.serve(t, f.approver, "reqId", f.requestID, forged(f.requester, `"asset_id":"`+asset+`",`), f.h.ApproveAssetRequest)
	require.Equal(t, http.StatusOK, code)
	st, approver := f.requestState(t)
	assert.Equal(t, "fulfilled", st)
	require.NotNil(t, approver)
	assert.Equal(t, f.approver, *approver)
	state, holder = f.holder(t, asset)
	assert.Equal(t, "assigned", state)
	require.NotNil(t, holder)
	assert.Equal(t, f.requester, *holder)

	// The same asset cannot be given to a second request.
	second := "flow-second-" + uuid.New().String()[:8]
	_, err := f.pool.Exec(context.Background(), `INSERT INTO asset_requests (id, type_id, workspace_id, requester_id, justification) VALUES ($1, $2, $3, $4, 'again')`,
		second, f.typeID, f.wsID, f.approver)
	require.NoError(t, err)
	f.policy.grants[[3]string{nodeOf(f.requester), f.oaID, ngac.OpManage}] = true
	code = f.serve(t, f.requester, "reqId", second, body, f.h.ApproveAssetRequest)
	assert.Equal(t, http.StatusConflict, code, "an asset that is no longer available is a conflict")
}

func TestRejectOverREST_RefusesAnEmptyReason(t *testing.T) {
	f := newFlowFixture(t)

	code := f.serve(t, f.approver, "reqId", f.requestID, `{"reason":"  "}`, f.h.RejectAssetRequest)

	assert.Equal(t, http.StatusBadRequest, code)
	st, _ := f.requestState(t)
	assert.Equal(t, "pending", st)
}
