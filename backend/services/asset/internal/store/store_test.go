package store_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/services/asset/internal/store"
)

// ---------------------------------------------------------------------------
// Test setup helpers
// ---------------------------------------------------------------------------

func testDBURL() string {
	if url := os.Getenv("TEST_DATABASE_URL"); url != "" {
		return url
	}
	return "postgres://ngac:ngac_secret@localhost:5432/ngac?sslmode=disable"
}

func setupStore(t *testing.T) *store.Store {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), testDBURL())
	if err != nil {
		t.Fatalf("connect to test DB: %v", err)
	}
	if err := pool.Ping(context.Background()); err != nil {
		t.Skipf("test DB not available: %v", err)
	}
	t.Cleanup(func() { pool.Close() })
	return store.New(pool)
}

// testTenant is a user and a workspace owned by one test.
//
// Test packages run in parallel against one shared database, so a test must
// never borrow a row it did not create: another package may delete it
// mid-test (this used to surface as an assets_created_by_fkey violation), and
// its own cleanup could delete rows another test is using. Each test gets its
// own uniquely named user and workspace, created on first use and removed —
// with everything this package hangs off them — when the test ends.
type testTenant struct{ wsID, userID string }

var tenants sync.Map // *testing.T -> *testTenant

func tenantFor(t *testing.T, pool *pgxpool.Pool) *testTenant {
	t.Helper()
	if v, ok := tenants.Load(t); ok {
		return v.(*testTenant)
	}
	sfx := uuid.New().String()
	tt := &testTenant{wsID: "store-test-ws-" + sfx, userID: "store-test-user-" + sfx}
	ctx := context.Background()
	_, err := pool.Exec(ctx, `INSERT INTO users (id, username, password) VALUES ($1, $1, '')`, tt.userID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO workspaces (id, name, owner_id) VALUES ($1, $1, $2)`, tt.wsID, tt.userID)
	require.NoError(t, err)
	tenants.Store(t, tt)

	t.Cleanup(func() {
		tenants.Delete(t)
		c := context.Background()
		for _, st := range []struct {
			q    string
			args []any
		}{
			{`DELETE FROM asset_requests WHERE workspace_id = $1 OR requester_id = $2 OR approver_id = $2`, []any{tt.wsID, tt.userID}},
			{`DELETE FROM asset_transitions WHERE actor_id = $1`, []any{tt.userID}},
			{`DELETE FROM assets WHERE workspace_id = $1 OR created_by = $2 OR assigned_to = $2`, []any{tt.wsID, tt.userID}},
			{`DELETE FROM asset_types WHERE workspace_id = $1`, []any{tt.wsID}},
			{`DELETE FROM workspaces WHERE id = $1`, []any{tt.wsID}},
			{`DELETE FROM users WHERE id = $1`, []any{tt.userID}},
		} {
			if _, err := pool.Exec(c, st.q, st.args...); err != nil {
				t.Errorf("fixture cleanup %q: %v", st.q, err)
			}
		}
	})
	return tt
}

// getTestWorkspaceID returns a workspace created for, and owned by, this test.
func getTestWorkspaceID(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	return tenantFor(t, pool).wsID
}

// getTestUserID returns a user created for, and owned by, this test.
func getTestUserID(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	return tenantFor(t, pool).userID
}

// getTestNGACNodeID returns an existing NGAC OA node for FK compliance.
func getTestNGACNodeID(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	var nodeID string
	err := pool.QueryRow(context.Background(), "SELECT id FROM ngac_nodes WHERE node_type = 'OA' LIMIT 1").Scan(&nodeID)
	if err != nil {
		t.Skipf("no NGAC OA node in test DB: %v", err)
	}
	return nodeID
}

// createTestType inserts a test asset type via direct SQL to avoid FK issues with ngac_oa_id.
func createTestType(t *testing.T, s *store.Store, wsID string) *store.AssetType {
	t.Helper()
	at := &store.AssetType{
		ID:           fmt.Sprintf("tt-%d", time.Now().UnixNano()),
		Name:         fmt.Sprintf("test-type-%d", time.Now().UnixNano()),
		Description:  "test description",
		Category:     "hardware",
		WorkspaceID:  wsID,
		FieldsSchema: json.RawMessage(`{}`),
		Lifecycle:    json.RawMessage(`{}`),
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	_, err := s.DB().Exec(context.Background(),
		`INSERT INTO asset_types (id, name, description, category, workspace_id, fields_schema, lifecycle, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		at.ID, at.Name, at.Description, at.Category, at.WorkspaceID,
		at.FieldsSchema, at.Lifecycle, at.CreatedAt, at.UpdatedAt,
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		s.DB().Exec(context.Background(), "DELETE FROM asset_types WHERE id = $1", at.ID)
	})
	return at
}

// createTestAsset inserts a test asset via direct SQL to avoid FK issues with ngac_node_id.
func createTestAsset(t *testing.T, s *store.Store, typeID, wsID, userID string) *store.Asset {
	t.Helper()
	a := &store.Asset{
		ID:           fmt.Sprintf("ta-%d", time.Now().UnixNano()),
		Name:         fmt.Sprintf("test-asset-%d", time.Now().UnixNano()),
		TypeID:       typeID,
		WorkspaceID:  wsID,
		State:        "requested",
		CustomFields: json.RawMessage(`{}`),
		CreatedBy:    userID,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	_, err := s.DB().Exec(context.Background(),
		`INSERT INTO assets (id, name, type_id, workspace_id, state, custom_fields, created_by, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		a.ID, a.Name, a.TypeID, a.WorkspaceID, a.State,
		a.CustomFields, a.CreatedBy, a.CreatedAt, a.UpdatedAt,
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		s.DB().Exec(context.Background(), "DELETE FROM assets WHERE id = $1", a.ID)
	})
	return a
}

// ---------------------------------------------------------------------------
// 4.1: TestCreateType + TestGetType + TestListTypes
// ---------------------------------------------------------------------------

func TestCreateType(t *testing.T) {
	s := setupStore(t)
	wsID := getTestWorkspaceID(t, s.DB())
	ngacOA := getTestNGACNodeID(t, s.DB())

	at := &store.AssetType{
		Name:         fmt.Sprintf("create-type-%d", time.Now().UnixNano()),
		Description:  "A laptop",
		Category:     "hardware",
		WorkspaceID:  wsID,
		FieldsSchema: json.RawMessage(`{"type":"object"}`),
		Lifecycle:    json.RawMessage(`{"states":["new","active"]}`),
		NgacOAID:     ngacOA,
	}

	err := s.CreateType(context.Background(), at)
	require.NoError(t, err)
	t.Cleanup(func() {
		s.DB().Exec(context.Background(), "DELETE FROM asset_types WHERE id = $1", at.ID)
	})

	assert.NotEmpty(t, at.ID, "ID should be generated")
	assert.False(t, at.CreatedAt.IsZero(), "CreatedAt should be set")
}

func TestGetType(t *testing.T) {
	s := setupStore(t)
	wsID := getTestWorkspaceID(t, s.DB())
	created := createTestType(t, s, wsID)

	got, err := s.GetType(context.Background(), created.ID)
	require.NoError(t, err)

	assert.Equal(t, created.ID, got.ID)
	assert.Equal(t, created.Name, got.Name)
	assert.Equal(t, "hardware", got.Category)
	assert.Equal(t, wsID, got.WorkspaceID)
	assert.Equal(t, int32(0), got.AssetCount, "no assets yet")
}

func TestGetType_NotFound(t *testing.T) {
	s := setupStore(t)
	_, err := s.GetType(context.Background(), "nonexistent-type-id")
	require.Error(t, err)
}

func TestListTypes(t *testing.T) {
	s := setupStore(t)
	wsID := getTestWorkspaceID(t, s.DB())
	t1 := createTestType(t, s, wsID)

	types, err := s.ListTypes(context.Background(), wsID)
	require.NoError(t, err)

	found := false
	for _, at := range types {
		if at.ID == t1.ID {
			found = true
			assert.Equal(t, t1.Name, at.Name)
		}
	}
	assert.True(t, found, "created type should appear in list")
}

func TestListTypes_EmptyWorkspace(t *testing.T) {
	s := setupStore(t)
	types, err := s.ListTypes(context.Background(), "nonexistent-ws")
	require.NoError(t, err)
	assert.Empty(t, types)
}

// ---------------------------------------------------------------------------
// 4.2: TestCreateAsset + TestGetAsset (nullable assigned_to)
// ---------------------------------------------------------------------------

func TestCreateAsset(t *testing.T) {
	s := setupStore(t)
	wsID := getTestWorkspaceID(t, s.DB())
	userID := getTestUserID(t, s.DB())
	ngacOA := getTestNGACNodeID(t, s.DB())
	at := createTestType(t, s, wsID)

	a := &store.Asset{
		Name:         "test-laptop-01",
		TypeID:       at.ID,
		WorkspaceID:  wsID,
		State:        "requested",
		CustomFields: json.RawMessage(`{"serial":"ABC123"}`),
		NgacNodeID:   ngacOA,
		CreatedBy:    userID,
	}
	err := s.CreateAsset(context.Background(), a)
	require.NoError(t, err)
	t.Cleanup(func() {
		s.DB().Exec(context.Background(), "DELETE FROM assets WHERE id = $1", a.ID)
	})

	assert.NotEmpty(t, a.ID)
	assert.False(t, a.CreatedAt.IsZero())
}

func TestGetAsset_NullAssignedTo(t *testing.T) {
	s := setupStore(t)
	wsID := getTestWorkspaceID(t, s.DB())
	userID := getTestUserID(t, s.DB())
	at := createTestType(t, s, wsID)
	created := createTestAsset(t, s, at.ID, wsID, userID)

	got, err := s.GetAsset(context.Background(), created.ID)
	require.NoError(t, err, "GetAsset must handle NULL assigned_to")

	assert.Equal(t, created.ID, got.ID)
	assert.Nil(t, got.AssignedTo, "should be nil when not assigned")
	assert.Equal(t, "", got.AssignedToUsername)
	assert.Equal(t, at.Name, got.TypeName)
}

func TestGetAsset_NotFound(t *testing.T) {
	s := setupStore(t)
	_, err := s.GetAsset(context.Background(), "nonexistent-asset-id")
	require.Error(t, err)
}

// ---------------------------------------------------------------------------
// 4.3: TestListAssets (filters: type, state, assigned_to)
// ---------------------------------------------------------------------------

func TestListAssets_NoFilter(t *testing.T) {
	s := setupStore(t)
	wsID := getTestWorkspaceID(t, s.DB())
	userID := getTestUserID(t, s.DB())
	at := createTestType(t, s, wsID)
	a := createTestAsset(t, s, at.ID, wsID, userID)

	assets, total, err := s.ListAssets(context.Background(), store.ListAssetsFilter{
		WorkspaceID: wsID,
	})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, total, int32(1))

	found := false
	for _, asset := range assets {
		if asset.ID == a.ID {
			found = true
		}
	}
	assert.True(t, found, "created asset should appear in unfiltered list")
}

func TestListAssets_FilterByType(t *testing.T) {
	s := setupStore(t)
	wsID := getTestWorkspaceID(t, s.DB())
	userID := getTestUserID(t, s.DB())
	at := createTestType(t, s, wsID)
	createTestAsset(t, s, at.ID, wsID, userID)

	assets, _, err := s.ListAssets(context.Background(), store.ListAssetsFilter{
		WorkspaceID: wsID,
		TypeID:      at.ID,
	})
	require.NoError(t, err)
	for _, asset := range assets {
		assert.Equal(t, at.ID, asset.TypeID)
	}
}

func TestListAssets_FilterByState(t *testing.T) {
	s := setupStore(t)
	wsID := getTestWorkspaceID(t, s.DB())
	userID := getTestUserID(t, s.DB())
	at := createTestType(t, s, wsID)
	createTestAsset(t, s, at.ID, wsID, userID)

	assets, _, err := s.ListAssets(context.Background(), store.ListAssetsFilter{
		WorkspaceID: wsID,
		State:       "requested",
	})
	require.NoError(t, err)
	for _, asset := range assets {
		assert.Equal(t, "requested", asset.State)
	}
}

func TestListAssets_EmptyResult(t *testing.T) {
	s := setupStore(t)
	assets, total, err := s.ListAssets(context.Background(), store.ListAssetsFilter{
		WorkspaceID: "nonexistent-ws",
	})
	require.NoError(t, err)
	assert.Empty(t, assets)
	assert.Equal(t, int32(0), total)
}

// ---------------------------------------------------------------------------
// 4.4: TestUpdateAssetState + TestClearAssignment
// ---------------------------------------------------------------------------

func TestUpdateAssetState(t *testing.T) {
	s := setupStore(t)
	wsID := getTestWorkspaceID(t, s.DB())
	userID := getTestUserID(t, s.DB())
	at := createTestType(t, s, wsID)
	a := createTestAsset(t, s, at.ID, wsID, userID)

	err := s.UpdateAssetState(context.Background(), a.ID, "active", nil)
	require.NoError(t, err)

	got, err := s.GetAsset(context.Background(), a.ID)
	require.NoError(t, err)
	assert.Equal(t, "active", got.State)
	assert.Nil(t, got.AssignedTo)
}

func TestUpdateAssetState_WithAssignment(t *testing.T) {
	s := setupStore(t)
	wsID := getTestWorkspaceID(t, s.DB())
	userID := getTestUserID(t, s.DB())
	at := createTestType(t, s, wsID)
	a := createTestAsset(t, s, at.ID, wsID, userID)

	err := s.UpdateAssetState(context.Background(), a.ID, "in_use", &userID)
	require.NoError(t, err)

	got, err := s.GetAsset(context.Background(), a.ID)
	require.NoError(t, err)
	assert.Equal(t, "in_use", got.State)
	require.NotNil(t, got.AssignedTo)
	assert.Equal(t, userID, *got.AssignedTo)
}

func TestClearAssignment(t *testing.T) {
	s := setupStore(t)
	wsID := getTestWorkspaceID(t, s.DB())
	userID := getTestUserID(t, s.DB())
	at := createTestType(t, s, wsID)
	a := createTestAsset(t, s, at.ID, wsID, userID)

	// Assign first
	err := s.UpdateAssetState(context.Background(), a.ID, "in_use", &userID)
	require.NoError(t, err)

	// Clear
	err = s.ClearAssignment(context.Background(), a.ID)
	require.NoError(t, err)

	got, err := s.GetAsset(context.Background(), a.ID)
	require.NoError(t, err)
	assert.Nil(t, got.AssignedTo, "assigned_to should be NULL after clear")
}

// ---------------------------------------------------------------------------
// 4.5: TestSoftDeleteAsset (idempotent, already deleted)
// ---------------------------------------------------------------------------

func TestSoftDeleteAsset(t *testing.T) {
	s := setupStore(t)
	wsID := getTestWorkspaceID(t, s.DB())
	userID := getTestUserID(t, s.DB())
	at := createTestType(t, s, wsID)
	a := createTestAsset(t, s, at.ID, wsID, userID)

	err := s.SoftDeleteAsset(context.Background(), a.ID)
	require.NoError(t, err)

	got, err := s.GetAsset(context.Background(), a.ID)
	require.NoError(t, err)
	assert.True(t, got.Deleted)
}

func TestSoftDeleteAsset_AlreadyDeleted(t *testing.T) {
	s := setupStore(t)
	wsID := getTestWorkspaceID(t, s.DB())
	userID := getTestUserID(t, s.DB())
	at := createTestType(t, s, wsID)
	a := createTestAsset(t, s, at.ID, wsID, userID)

	err := s.SoftDeleteAsset(context.Background(), a.ID)
	require.NoError(t, err)

	// Second delete should fail (already deleted)
	err = s.SoftDeleteAsset(context.Background(), a.ID)
	require.Error(t, err, "should error when already deleted")
}

func TestSoftDeleteAsset_NotFound(t *testing.T) {
	s := setupStore(t)
	err := s.SoftDeleteAsset(context.Background(), "nonexistent-asset")
	require.Error(t, err)
}

// ---------------------------------------------------------------------------
// 4.6: TestCreateRequest + TestGetRequest (nullable approver)
// ---------------------------------------------------------------------------

func TestCreateRequest(t *testing.T) {
	s := setupStore(t)
	wsID := getTestWorkspaceID(t, s.DB())
	userID := getTestUserID(t, s.DB())
	at := createTestType(t, s, wsID)

	req := &store.AssetRequest{
		TypeID:        at.ID,
		WorkspaceID:   wsID,
		RequesterID:   userID,
		Status:        "pending",
		Justification: "Need for development",
		Quantity:      2,
	}
	err := s.CreateRequest(context.Background(), req)
	require.NoError(t, err)
	t.Cleanup(func() {
		s.DB().Exec(context.Background(), "DELETE FROM asset_requests WHERE id = $1", req.ID)
	})

	assert.NotEmpty(t, req.ID)
	assert.False(t, req.CreatedAt.IsZero())
}

func TestGetRequest_NullableApprover(t *testing.T) {
	s := setupStore(t)
	wsID := getTestWorkspaceID(t, s.DB())
	userID := getTestUserID(t, s.DB())
	at := createTestType(t, s, wsID)

	req := &store.AssetRequest{
		TypeID:        at.ID,
		WorkspaceID:   wsID,
		RequesterID:   userID,
		Status:        "pending",
		Justification: "Testing",
		Quantity:      1,
	}
	err := s.CreateRequest(context.Background(), req)
	require.NoError(t, err)
	t.Cleanup(func() {
		s.DB().Exec(context.Background(), "DELETE FROM asset_requests WHERE id = $1", req.ID)
	})

	got, err := s.GetRequest(context.Background(), req.ID)
	require.NoError(t, err, "GetRequest must handle NULL approver")

	assert.Equal(t, req.ID, got.ID)
	assert.Nil(t, got.ApproverID, "approver should be nil for pending")
	assert.Nil(t, got.AssignedAssetID, "no asset assigned yet")
	assert.Equal(t, at.Name, got.TypeName)
}

// ---------------------------------------------------------------------------
// 4.7: TestListRequests (filters: status, mine_only)
// ---------------------------------------------------------------------------

func TestListRequests_NoFilter(t *testing.T) {
	s := setupStore(t)
	wsID := getTestWorkspaceID(t, s.DB())
	userID := getTestUserID(t, s.DB())
	at := createTestType(t, s, wsID)

	req := &store.AssetRequest{
		TypeID: at.ID, WorkspaceID: wsID, RequesterID: userID,
		Status: "pending", Justification: "test", Quantity: 1,
	}
	err := s.CreateRequest(context.Background(), req)
	require.NoError(t, err)
	t.Cleanup(func() {
		s.DB().Exec(context.Background(), "DELETE FROM asset_requests WHERE id = $1", req.ID)
	})

	requests, total, err := s.ListRequests(context.Background(), store.ListRequestsFilter{
		WorkspaceID: wsID,
	})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, total, int32(1))
	assert.NotEmpty(t, requests)
}

func TestListRequests_FilterByStatus(t *testing.T) {
	s := setupStore(t)
	wsID := getTestWorkspaceID(t, s.DB())
	userID := getTestUserID(t, s.DB())
	at := createTestType(t, s, wsID)

	req := &store.AssetRequest{
		TypeID: at.ID, WorkspaceID: wsID, RequesterID: userID,
		Status: "pending", Justification: "test", Quantity: 1,
	}
	err := s.CreateRequest(context.Background(), req)
	require.NoError(t, err)
	t.Cleanup(func() {
		s.DB().Exec(context.Background(), "DELETE FROM asset_requests WHERE id = $1", req.ID)
	})

	requests, _, err := s.ListRequests(context.Background(), store.ListRequestsFilter{
		WorkspaceID: wsID, Status: "pending",
	})
	require.NoError(t, err)
	for _, r := range requests {
		assert.Equal(t, "pending", r.Status)
	}
}

func TestListRequests_MineOnly(t *testing.T) {
	s := setupStore(t)
	wsID := getTestWorkspaceID(t, s.DB())
	userID := getTestUserID(t, s.DB())
	at := createTestType(t, s, wsID)

	req := &store.AssetRequest{
		TypeID: at.ID, WorkspaceID: wsID, RequesterID: userID,
		Status: "pending", Justification: "mine", Quantity: 1,
	}
	err := s.CreateRequest(context.Background(), req)
	require.NoError(t, err)
	t.Cleanup(func() {
		s.DB().Exec(context.Background(), "DELETE FROM asset_requests WHERE id = $1", req.ID)
	})

	requests, _, err := s.ListRequests(context.Background(), store.ListRequestsFilter{
		WorkspaceID: wsID, UserID: userID, MineOnly: true,
	})
	require.NoError(t, err)
	for _, r := range requests {
		assert.Equal(t, userID, r.RequesterID)
	}
}

// ---------------------------------------------------------------------------
// 4.8: TestFulfillRequest + TestUpdateRequestStatus
// ---------------------------------------------------------------------------

func TestUpdateRequestStatus(t *testing.T) {
	s := setupStore(t)
	wsID := getTestWorkspaceID(t, s.DB())
	userID := getTestUserID(t, s.DB())
	at := createTestType(t, s, wsID)

	req := &store.AssetRequest{
		TypeID: at.ID, WorkspaceID: wsID, RequesterID: userID,
		Status: "pending", Justification: "test", Quantity: 1,
	}
	err := s.CreateRequest(context.Background(), req)
	require.NoError(t, err)
	t.Cleanup(func() {
		s.DB().Exec(context.Background(), "DELETE FROM asset_requests WHERE id = $1", req.ID)
	})

	err = s.UpdateRequestStatus(context.Background(), req.ID, "approved", userID, "Looks good")
	require.NoError(t, err)

	got, err := s.GetRequest(context.Background(), req.ID)
	require.NoError(t, err)
	assert.Equal(t, "approved", got.Status)
	require.NotNil(t, got.ApproverID)
	assert.Equal(t, userID, *got.ApproverID)
	assert.Equal(t, "Looks good", got.ApproverComment)
}

func TestFulfillRequest(t *testing.T) {
	s := setupStore(t)
	wsID := getTestWorkspaceID(t, s.DB())
	userID := getTestUserID(t, s.DB())
	at := createTestType(t, s, wsID)
	asset := createTestAsset(t, s, at.ID, wsID, userID)

	req := &store.AssetRequest{
		TypeID: at.ID, WorkspaceID: wsID, RequesterID: userID,
		Status: "approved", Justification: "test", Quantity: 1,
	}
	err := s.CreateRequest(context.Background(), req)
	require.NoError(t, err)
	t.Cleanup(func() {
		s.DB().Exec(context.Background(), "DELETE FROM asset_requests WHERE id = $1", req.ID)
	})

	err = s.FulfillRequest(context.Background(), req.ID, asset.ID)
	require.NoError(t, err)

	got, err := s.GetRequest(context.Background(), req.ID)
	require.NoError(t, err)
	assert.Equal(t, "fulfilled", got.Status)
	require.NotNil(t, got.AssignedAssetID)
	assert.Equal(t, asset.ID, *got.AssignedAssetID)
}

// ---------------------------------------------------------------------------
// 4.9: TestInsertTransition + TestGetAssetHistory
// ---------------------------------------------------------------------------

func TestInsertTransition(t *testing.T) {
	s := setupStore(t)
	wsID := getTestWorkspaceID(t, s.DB())
	userID := getTestUserID(t, s.DB())
	at := createTestType(t, s, wsID)
	asset := createTestAsset(t, s, at.ID, wsID, userID)

	tr := &store.TransitionRecord{
		AssetID:   asset.ID,
		FromState: "requested",
		ToState:   "active",
		Action:    "approve",
		ActorID:   userID,
		Comment:   "Approved for use",
	}
	err := s.InsertTransition(context.Background(), tr)
	require.NoError(t, err)
	t.Cleanup(func() {
		s.DB().Exec(context.Background(), "DELETE FROM asset_transitions WHERE id = $1", tr.ID)
	})

	assert.NotEmpty(t, tr.ID)
	assert.False(t, tr.CreatedAt.IsZero())
}

func TestGetAssetHistory(t *testing.T) {
	s := setupStore(t)
	wsID := getTestWorkspaceID(t, s.DB())
	userID := getTestUserID(t, s.DB())
	at := createTestType(t, s, wsID)
	asset := createTestAsset(t, s, at.ID, wsID, userID)

	transitions := []struct{ from, to, action string }{
		{"requested", "active", "approve"},
		{"active", "in_use", "assign"},
	}
	for _, tr := range transitions {
		rec := &store.TransitionRecord{
			AssetID: asset.ID, FromState: tr.from, ToState: tr.to,
			Action: tr.action, ActorID: userID, Comment: "test",
		}
		err := s.InsertTransition(context.Background(), rec)
		require.NoError(t, err)
		t.Cleanup(func() {
			s.DB().Exec(context.Background(), "DELETE FROM asset_transitions WHERE id = $1", rec.ID)
		})
	}

	history, err := s.GetAssetHistory(context.Background(), asset.ID)
	require.NoError(t, err)
	assert.Len(t, history, 2)
	assert.Equal(t, "requested", history[0].FromState)
	assert.Equal(t, "active", history[0].ToState)
	assert.Equal(t, "active", history[1].FromState)
	assert.Equal(t, "in_use", history[1].ToState)
}

func TestGetAssetHistory_Empty(t *testing.T) {
	s := setupStore(t)
	wsID := getTestWorkspaceID(t, s.DB())
	userID := getTestUserID(t, s.DB())
	at := createTestType(t, s, wsID)
	asset := createTestAsset(t, s, at.ID, wsID, userID)

	history, err := s.GetAssetHistory(context.Background(), asset.ID)
	require.NoError(t, err)
	assert.Empty(t, history)
}

// ---------------------------------------------------------------------------
// ApplyTransition — the state change and its history row commit together.
// ---------------------------------------------------------------------------

func assetState(t *testing.T, s *store.Store, assetID string) string {
	t.Helper()
	var st string
	require.NoError(t, s.DB().QueryRow(context.Background(),
		`SELECT state FROM assets WHERE id = $1`, assetID).Scan(&st))
	return st
}

func TestApplyTransition_WritesStateAndHistory(t *testing.T) {
	s := setupStore(t)
	wsID := getTestWorkspaceID(t, s.DB())
	userID := getTestUserID(t, s.DB())
	at := createTestType(t, s, wsID)
	a := createTestAsset(t, s, at.ID, wsID, userID)

	err := s.ApplyTransition(context.Background(), &store.TransitionRecord{
		AssetID: a.ID, FromState: "requested", ToState: "available", Action: "approve", ActorID: userID,
	}, nil)

	require.NoError(t, err)
	assert.Equal(t, "available", assetState(t, s, a.ID))
	history, err := s.GetAssetHistory(context.Background(), a.ID)
	require.NoError(t, err)
	require.Len(t, history, 1)
	assert.Equal(t, userID, history[0].ActorID)
}

func TestApplyTransition_HistoryFailureRollsBackState(t *testing.T) {
	s := setupStore(t)
	wsID := getTestWorkspaceID(t, s.DB())
	userID := getTestUserID(t, s.DB())
	at := createTestType(t, s, wsID)
	a := createTestAsset(t, s, at.ID, wsID, userID)

	// actor_id references users: an unknown actor makes the history insert fail.
	err := s.ApplyTransition(context.Background(), &store.TransitionRecord{
		AssetID: a.ID, FromState: "requested", ToState: "available", Action: "approve", ActorID: "no-such-user",
	}, nil)

	require.Error(t, err)
	assert.Equal(t, "requested", assetState(t, s, a.ID), "a transition with no audit record must not take effect")
	history, err := s.GetAssetHistory(context.Background(), a.ID)
	require.NoError(t, err)
	assert.Empty(t, history)
}

// ---------------------------------------------------------------------------
// Visibility filters — the authorization layer narrows the query, so pages and
// totals count only rows the caller may see.
// ---------------------------------------------------------------------------

func TestListAssets_VisibleTypeIDsRestrictsRowsAndTotal(t *testing.T) {
	s := setupStore(t)
	wsID := getTestWorkspaceID(t, s.DB())
	userID := getTestUserID(t, s.DB())
	visible := createTestType(t, s, wsID)
	hidden := createTestType(t, s, wsID)
	a := createTestAsset(t, s, visible.ID, wsID, userID)
	createTestAsset(t, s, hidden.ID, wsID, userID)

	assets, total, err := s.ListAssets(context.Background(), store.ListAssetsFilter{
		WorkspaceID: wsID, VisibleTypeIDs: []string{visible.ID},
	})
	require.NoError(t, err)
	require.Len(t, assets, 1)
	assert.Equal(t, a.ID, assets[0].ID)
	assert.Equal(t, int32(1), total)
}

func TestListAssets_EmptyVisibleTypeIDsMatchesNothing(t *testing.T) {
	s := setupStore(t)
	wsID := getTestWorkspaceID(t, s.DB())
	userID := getTestUserID(t, s.DB())
	at := createTestType(t, s, wsID)
	createTestAsset(t, s, at.ID, wsID, userID)

	assets, total, err := s.ListAssets(context.Background(), store.ListAssetsFilter{
		WorkspaceID: wsID, VisibleTypeIDs: []string{},
	})
	require.NoError(t, err)
	assert.Empty(t, assets)
	assert.Zero(t, total)
}

func TestListRequests_VisibilityRestrictsRowsAndTotal(t *testing.T) {
	s := setupStore(t)
	wsID := getTestWorkspaceID(t, s.DB())
	userID := getTestUserID(t, s.DB())
	approvable := createTestType(t, s, wsID)
	other := createTestType(t, s, wsID)

	mk := func(typeID string) string {
		r := &store.AssetRequest{
			TypeID: typeID, WorkspaceID: wsID, RequesterID: userID,
			Status: "pending", Justification: "vis", Quantity: 1,
		}
		require.NoError(t, s.CreateRequest(context.Background(), r))
		t.Cleanup(func() {
			s.DB().Exec(context.Background(), "DELETE FROM asset_requests WHERE id = $1", r.ID)
		})
		return r.ID
	}
	onApprovable := mk(approvable.ID)
	mk(other.ID)

	// Not the requester; may approve one type only.
	requests, total, err := s.ListRequests(context.Background(), store.ListRequestsFilter{
		WorkspaceID: wsID,
		Visibility:  &store.RequestVisibility{RequesterID: "not-a-requester", TypeIDs: []string{approvable.ID}},
	})
	require.NoError(t, err)
	require.Len(t, requests, 1)
	assert.Equal(t, onApprovable, requests[0].ID)
	assert.Equal(t, int32(1), total)

	// The requester sees their own requests of any type.
	requests, total, err = s.ListRequests(context.Background(), store.ListRequestsFilter{
		WorkspaceID: wsID,
		Visibility:  &store.RequestVisibility{RequesterID: userID},
	})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, total, int32(2))
	for _, r := range requests {
		assert.Equal(t, userID, r.RequesterID)
	}

	// Nobody and nothing: matches no rows.
	requests, total, err = s.ListRequests(context.Background(), store.ListRequestsFilter{
		WorkspaceID: wsID, Visibility: &store.RequestVisibility{},
	})
	require.NoError(t, err)
	assert.Empty(t, requests)
	assert.Zero(t, total)
}
