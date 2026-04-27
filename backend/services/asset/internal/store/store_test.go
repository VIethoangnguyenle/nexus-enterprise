package store_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

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

// getTestWorkspaceID returns an existing workspace_id from DB for FK compliance.
func getTestWorkspaceID(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	var wsID string
	err := pool.QueryRow(context.Background(), "SELECT id FROM workspaces LIMIT 1").Scan(&wsID)
	if err != nil {
		t.Skipf("no workspace in test DB: %v", err)
	}
	return wsID
}

// getTestUserID returns an existing user ID from DB for FK compliance.
func getTestUserID(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	var uid string
	err := pool.QueryRow(context.Background(), "SELECT id FROM users LIMIT 1").Scan(&uid)
	if err != nil {
		t.Skipf("no user in test DB: %v", err)
	}
	return uid
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
