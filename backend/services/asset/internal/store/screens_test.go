package store_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/services/asset/internal/store"
)

// member creates a user with a display name who belongs to the workspace.
// An empty wsID creates a user who belongs to none.
func member(t *testing.T, pool *pgxpool.Pool, wsID, display string) string {
	t.Helper()
	ctx := context.Background()
	id := "screens-user-" + uuid.NewString()
	_, err := pool.Exec(ctx, `INSERT INTO users (id, username, password, display_name) VALUES ($1, $1, '', $2)`, id, display)
	require.NoError(t, err)
	if wsID != "" {
		_, err = pool.Exec(ctx, `INSERT INTO tenant_users (tenant_id, user_id) VALUES ($1, $2)`, wsID, id)
		require.NoError(t, err)
	}
	t.Cleanup(func() {
		c := context.Background()
		pool.Exec(c, `DELETE FROM asset_requests WHERE requester_id = $1 OR approver_id = $1`, id)
		pool.Exec(c, `DELETE FROM asset_transitions WHERE actor_id = $1 OR subject_user_id = $1`, id)
		pool.Exec(c, `DELETE FROM assets WHERE assigned_to = $1 OR created_by = $1`, id)
		pool.Exec(c, `DELETE FROM tenant_users WHERE user_id = $1`, id)
		pool.Exec(c, `DELETE FROM users WHERE id = $1`, id)
	})
	return id
}

// otherWorkspace is a second workspace, owned by owner, for tenant-scoping tests.
func otherWorkspace(t *testing.T, pool *pgxpool.Pool, owner string) string {
	t.Helper()
	id := "screens-ws-" + uuid.NewString()
	_, err := pool.Exec(context.Background(), `INSERT INTO workspaces (id, name, owner_id) VALUES ($1, $1, $2)`, id, owner)
	require.NoError(t, err)
	t.Cleanup(func() {
		c := context.Background()
		pool.Exec(c, `DELETE FROM asset_requests WHERE workspace_id = $1`, id)
		pool.Exec(c, `DELETE FROM asset_transitions WHERE asset_id IN (SELECT id FROM assets WHERE workspace_id = $1)`, id)
		pool.Exec(c, `DELETE FROM assets WHERE workspace_id = $1`, id)
		pool.Exec(c, `DELETE FROM asset_types WHERE workspace_id = $1`, id)
		pool.Exec(c, `DELETE FROM tenant_users WHERE tenant_id = $1`, id)
		pool.Exec(c, `DELETE FROM workspaces WHERE id = $1`, id)
	})
	return id
}

func newAsset(t *testing.T, s *store.Store, typeID, wsID, creator, name, state string) *store.Asset {
	t.Helper()
	a := &store.Asset{Name: name, TypeID: typeID, WorkspaceID: wsID, State: state, CustomFields: json.RawMessage(`{}`), CreatedBy: creator}
	require.NoError(t, s.CreateAsset(context.Background(), a))
	t.Cleanup(func() {
		s.DB().Exec(context.Background(), "DELETE FROM asset_transitions WHERE asset_id = $1", a.ID)
		s.DB().Exec(context.Background(), "DELETE FROM assets WHERE id = $1", a.ID)
	})
	return a
}

func newRequest(t *testing.T, s *store.Store, typeID, wsID, requester, status string) *store.AssetRequest {
	t.Helper()
	r := &store.AssetRequest{TypeID: typeID, WorkspaceID: wsID, RequesterID: requester, Status: status, Justification: "need one", Quantity: 1, Urgency: "high"}
	require.NoError(t, s.CreateRequest(context.Background(), r))
	t.Cleanup(func() { s.DB().Exec(context.Background(), "DELETE FROM asset_requests WHERE id = $1", r.ID) })
	return r
}

// ---------------------------------------------------------------------------
// Names are read inside the asset's own workspace
// ---------------------------------------------------------------------------

func TestNames_ShownForMembersOfTheAssetsWorkspaceOnly(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()
	wsID := getTestWorkspaceID(t, s.DB())
	owner := getTestUserID(t, s.DB())
	at := createTestType(t, s, wsID)

	lan := member(t, s.DB(), wsID, "Nguyễn Thu Lan")
	stranger := member(t, s.DB(), "", "Người ngoài")

	mine := newAsset(t, s, at.ID, wsID, owner, "MacBook máy số 7", "assigned")
	theirs := newAsset(t, s, at.ID, wsID, owner, "MacBook máy số 8", "assigned")
	_, err := s.DB().Exec(ctx, `UPDATE assets SET assigned_to = $2 WHERE id = $1`, mine.ID, lan)
	require.NoError(t, err)
	_, err = s.DB().Exec(ctx, `UPDATE assets SET assigned_to = $2 WHERE id = $1`, theirs.ID, stranger)
	require.NoError(t, err)

	got, err := s.GetAsset(ctx, mine.ID)
	require.NoError(t, err)
	assert.Equal(t, "Nguyễn Thu Lan", got.AssignedToName)

	got, err = s.GetAsset(ctx, theirs.ID)
	require.NoError(t, err)
	assert.Empty(t, got.AssignedToName, "a person outside the workspace is not named")
	assert.Empty(t, got.AssignedToUsername, "nor is their login")
	require.NotNil(t, got.AssignedTo)

	list, _, err := s.ListAssets(ctx, store.ListAssetsFilter{WorkspaceID: wsID})
	require.NoError(t, err)
	names := map[string]string{}
	for _, a := range list {
		names[a.ID] = a.AssignedToName
	}
	assert.Equal(t, "Nguyễn Thu Lan", names[mine.ID])
	assert.Empty(t, names[theirs.ID])
}

func TestNames_RequestAndHistoryAreScopedToTheWorkspace(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()
	wsID := getTestWorkspaceID(t, s.DB())
	owner := getTestUserID(t, s.DB())
	at := createTestType(t, s, wsID)
	yen := member(t, s.DB(), wsID, "Phạm Hải Yến")
	duc := member(t, s.DB(), "", "Trần Minh Đức") // approved from elsewhere; not a member here

	req := newRequest(t, s, at.ID, wsID, yen, "pending")
	require.NoError(t, s.UpdateRequestStatus(ctx, req.ID, "approved", duc, "ok"))
	got, err := s.GetRequest(ctx, req.ID)
	require.NoError(t, err)
	assert.Equal(t, "Phạm Hải Yến", got.RequesterName)
	assert.Empty(t, got.ApproverName, "an approver outside the workspace is not named")
	assert.Equal(t, "high", got.Urgency)

	asset := newAsset(t, s, at.ID, wsID, owner, "Laptop", "available")
	require.NoError(t, s.InsertTransition(ctx, &store.TransitionRecord{AssetID: asset.ID, FromState: "requested", ToState: "available", Action: "approve", ActorID: yen, SubjectUserID: duc}))
	hist, err := s.GetAssetHistory(ctx, asset.ID)
	require.NoError(t, err)
	require.Len(t, hist, 1)
	assert.Equal(t, "Phạm Hải Yến", hist[0].ActorName)
	assert.Empty(t, hist[0].SubjectName)
	assert.Equal(t, duc, hist[0].SubjectUserID)
}

// ---------------------------------------------------------------------------
// List: search and counts
// ---------------------------------------------------------------------------

func TestListAssets_SearchMatchesNameOrHolder(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()
	wsID := getTestWorkspaceID(t, s.DB())
	owner := getTestUserID(t, s.DB())
	at := createTestType(t, s, wsID)
	lan := member(t, s.DB(), wsID, "Nguyễn Thu Lan")
	dell := newAsset(t, s, at.ID, wsID, owner, "Màn hình Dell 27 inch", "available")
	mac := newAsset(t, s, at.ID, wsID, owner, "MacBook Pro", "assigned")
	_, err := s.DB().Exec(ctx, `UPDATE assets SET assigned_to = $2 WHERE id = $1`, mac.ID, lan)
	require.NoError(t, err)

	ids := func(f store.ListAssetsFilter) []string {
		f.WorkspaceID = wsID
		list, _, err := s.ListAssets(ctx, f)
		require.NoError(t, err)
		var out []string
		for _, a := range list {
			out = append(out, a.ID)
		}
		return out
	}
	assert.Equal(t, []string{dell.ID}, ids(store.ListAssetsFilter{Search: "dell"}), "by name, any case")
	assert.Equal(t, []string{mac.ID}, ids(store.ListAssetsFilter{Search: "thu lan"}), "by holder")
	assert.Empty(t, ids(store.ListAssetsFilter{Search: "%"}), "wildcards are text, not patterns")
	assert.ElementsMatch(t, []string{dell.ID, mac.ID}, ids(store.ListAssetsFilter{Search: ""}))
}

func TestListTypes_CountsAvailableAssets(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()
	wsID := getTestWorkspaceID(t, s.DB())
	owner := getTestUserID(t, s.DB())
	at := createTestType(t, s, wsID)
	newAsset(t, s, at.ID, wsID, owner, "a", "available")
	newAsset(t, s, at.ID, wsID, owner, "b", "available")
	newAsset(t, s, at.ID, wsID, owner, "c", "assigned")

	got, err := s.GetType(ctx, at.ID)
	require.NoError(t, err)
	assert.Equal(t, int32(3), got.AssetCount)
	assert.Equal(t, int32(2), got.AvailableCount)

	types, err := s.ListTypes(ctx, wsID)
	require.NoError(t, err)
	require.Len(t, types, 1)
	assert.Equal(t, int32(2), types[0].AvailableCount)
}

// ---------------------------------------------------------------------------
// Summary and activity
// ---------------------------------------------------------------------------

func TestSummary_CountsOnlyTheVisibleTypes(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()
	wsID := getTestWorkspaceID(t, s.DB())
	owner := getTestUserID(t, s.DB())
	laptop := createTestType(t, s, wsID)
	secret := createTestType(t, s, wsID)
	lan := member(t, s.DB(), wsID, "Lan")
	a1 := newAsset(t, s, laptop.ID, wsID, owner, "a1", "assigned")
	a2 := newAsset(t, s, laptop.ID, wsID, owner, "a2", "assigned")
	newAsset(t, s, laptop.ID, wsID, owner, "a3", "available")
	newAsset(t, s, secret.ID, wsID, owner, "hidden", "available")
	for _, id := range []string{a1.ID, a2.ID} {
		_, err := s.DB().Exec(ctx, `UPDATE assets SET assigned_to = $2 WHERE id = $1`, id, lan)
		require.NoError(t, err)
	}
	// A deleted asset is nobody's.
	gone := newAsset(t, s, laptop.ID, wsID, owner, "gone", "available")
	require.NoError(t, s.SoftDeleteAsset(ctx, gone.ID))

	sum, err := s.Summary(ctx, wsID, []string{laptop.ID})
	require.NoError(t, err)
	assert.Equal(t, int32(3), sum.Total)
	assert.Equal(t, map[string]int32{"assigned": 2, "available": 1}, sum.ByState)
	assert.Equal(t, int32(1), sum.Holders, "two assets held by one person")
	require.Len(t, sum.ByType, 1)
	assert.Equal(t, laptop.ID, sum.ByType[0].TypeID)
	assert.Equal(t, laptop.Name, sum.ByType[0].TypeName)
	assert.Equal(t, int32(3), sum.ByType[0].Count)

	none, err := s.Summary(ctx, wsID, []string{})
	require.NoError(t, err)
	assert.Equal(t, int32(0), none.Total, "no readable type, nothing counted")
	assert.Empty(t, none.ByState)
}

func TestListActivity_NewestFirstWithNamesOfThePeopleInvolved(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()
	wsID := getTestWorkspaceID(t, s.DB())
	owner := getTestUserID(t, s.DB())
	at := createTestType(t, s, wsID)
	hidden := createTestType(t, s, wsID)
	duc := member(t, s.DB(), wsID, "Trần Minh Đức")
	lan := member(t, s.DB(), wsID, "Nguyễn Thu Lan")
	mac := newAsset(t, s, at.ID, wsID, owner, "MacBook máy số 3", "assigned")
	secret := newAsset(t, s, hidden.ID, wsID, owner, "Máy chủ", "available")

	require.NoError(t, s.InsertTransition(ctx, &store.TransitionRecord{AssetID: mac.ID, FromState: "available", ToState: "assigned", Action: "assign", ActorID: duc, SubjectUserID: lan}))
	require.NoError(t, s.InsertTransition(ctx, &store.TransitionRecord{AssetID: mac.ID, FromState: "assigned", ToState: "available", Action: "return", ActorID: duc, SubjectUserID: lan}))
	require.NoError(t, s.InsertTransition(ctx, &store.TransitionRecord{AssetID: secret.ID, FromState: "available", ToState: "maintenance", Action: "flag_maintenance", ActorID: duc}))

	got, err := s.ListActivity(ctx, wsID, []string{at.ID}, 10)
	require.NoError(t, err)
	require.Len(t, got, 2, "the type the caller cannot read is left out")
	assert.Equal(t, "return", got[0].Action, "newest first")
	assert.Equal(t, "MacBook máy số 3", got[0].AssetName)
	assert.Equal(t, at.Name, got[0].TypeName)
	assert.Equal(t, "Trần Minh Đức", got[0].ActorName)
	assert.Equal(t, "Nguyễn Thu Lan", got[0].SubjectName)

	limited, err := s.ListActivity(ctx, wsID, []string{at.ID}, 1)
	require.NoError(t, err)
	assert.Len(t, limited, 1)

	none, err := s.ListActivity(ctx, wsID, []string{}, 10)
	require.NoError(t, err)
	assert.Empty(t, none)
}

// ---------------------------------------------------------------------------
// Transitions keep the holder honest
// ---------------------------------------------------------------------------

func TestApplyTransition_ClearsTheHolderWhenTheAssetGoesBackToStock(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()
	wsID := getTestWorkspaceID(t, s.DB())
	owner := getTestUserID(t, s.DB())
	at := createTestType(t, s, wsID)
	lan := member(t, s.DB(), wsID, "Lan")
	a := newAsset(t, s, at.ID, wsID, owner, "Laptop", "assigned")
	_, err := s.DB().Exec(ctx, `UPDATE assets SET assigned_to = $2 WHERE id = $1`, a.ID, lan)
	require.NoError(t, err)

	// Maintenance keeps the holder: it is still their laptop.
	require.NoError(t, s.ApplyTransition(ctx, &store.TransitionRecord{AssetID: a.ID, FromState: "assigned", ToState: "maintenance", Action: "flag_maintenance", ActorID: owner}, nil))
	got, err := s.GetAsset(ctx, a.ID)
	require.NoError(t, err)
	require.NotNil(t, got.AssignedTo)
	assert.Equal(t, lan, *got.AssignedTo)

	// Back to available: the holder is cleared and the step names who it was.
	tr := &store.TransitionRecord{AssetID: a.ID, FromState: "maintenance", ToState: "available", Action: "complete_maintenance", ActorID: owner}
	require.NoError(t, s.ApplyTransition(ctx, tr, nil))
	got, err = s.GetAsset(ctx, a.ID)
	require.NoError(t, err)
	assert.Nil(t, got.AssignedTo)
	hist, err := s.GetAssetHistory(ctx, a.ID)
	require.NoError(t, err)
	require.Len(t, hist, 2)
	assert.Equal(t, lan, hist[1].SubjectUserID)
}

// ---------------------------------------------------------------------------
// Hand over
// ---------------------------------------------------------------------------

func TestHandOver_AssignsAndRecordsTheStep(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()
	wsID := getTestWorkspaceID(t, s.DB())
	owner := getTestUserID(t, s.DB())
	at := createTestType(t, s, wsID)
	lan := member(t, s.DB(), wsID, "Nguyễn Thu Lan")
	vinh := member(t, s.DB(), wsID, "Lê Quang Vinh")
	a := newAsset(t, s, at.ID, wsID, owner, "Laptop", "available")

	require.NoError(t, s.HandOver(ctx, a.ID, lan, owner, "Thay máy cũ"))
	got, err := s.GetAsset(ctx, a.ID)
	require.NoError(t, err)
	assert.Equal(t, "assigned", got.State)
	require.NotNil(t, got.AssignedTo)
	assert.Equal(t, lan, *got.AssignedTo)

	// Handing an assigned asset to someone else is allowed: it moves.
	require.NoError(t, s.HandOver(ctx, a.ID, vinh, owner, ""))
	got, err = s.GetAsset(ctx, a.ID)
	require.NoError(t, err)
	assert.Equal(t, vinh, *got.AssignedTo)

	hist, err := s.GetAssetHistory(ctx, a.ID)
	require.NoError(t, err)
	require.Len(t, hist, 2)
	assert.Equal(t, "assign", hist[0].Action)
	assert.Equal(t, lan, hist[0].SubjectUserID)
	assert.Equal(t, "Thay máy cũ", hist[0].Comment)
	assert.Equal(t, vinh, hist[1].SubjectUserID)
}

func TestHandOver_Refusals(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()
	wsID := getTestWorkspaceID(t, s.DB())
	owner := getTestUserID(t, s.DB())
	at := createTestType(t, s, wsID)
	lan := member(t, s.DB(), wsID, "Lan")
	stranger := member(t, s.DB(), "", "Người ngoài")

	maint := newAsset(t, s, at.ID, wsID, owner, "In repair", "maintenance")
	assert.ErrorIs(t, s.HandOver(ctx, maint.ID, lan, owner, ""), store.ErrAssetUnavailable, "only available or assigned assets move")

	avail := newAsset(t, s, at.ID, wsID, owner, "Free", "available")
	assert.ErrorIs(t, s.HandOver(ctx, avail.ID, stranger, owner, ""), store.ErrNotAMember, "to a person outside the workspace")
	assert.ErrorIs(t, s.HandOver(ctx, avail.ID, "nobody-"+uuid.NewString(), owner, ""), store.ErrNotAMember)

	gone := newAsset(t, s, at.ID, wsID, owner, "Gone", "available")
	require.NoError(t, s.SoftDeleteAsset(ctx, gone.ID))
	assert.ErrorIs(t, s.HandOver(ctx, gone.ID, lan, owner, ""), store.ErrNotFound)
	assert.ErrorIs(t, s.HandOver(ctx, "missing-"+uuid.NewString(), lan, owner, ""), store.ErrNotFound)

	got, err := s.GetAsset(ctx, avail.ID)
	require.NoError(t, err)
	assert.Equal(t, "available", got.State, "a refused hand-over changes nothing")
	assert.Nil(t, got.AssignedTo)
}

// ---------------------------------------------------------------------------
// Approve and give an asset, in one step
// ---------------------------------------------------------------------------

func TestApproveAndAssign_RecordsEverythingTogether(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()
	wsID := getTestWorkspaceID(t, s.DB())
	owner := getTestUserID(t, s.DB())
	at := createTestType(t, s, wsID)
	yen := member(t, s.DB(), wsID, "Phạm Hải Yến")
	duc := member(t, s.DB(), wsID, "Trần Minh Đức")
	asset := newAsset(t, s, at.ID, wsID, owner, "Laptop số 4", "available")
	req := newRequest(t, s, at.ID, wsID, yen, "pending")

	require.NoError(t, s.ApproveAndAssign(ctx, store.FulfilParams{RequestID: req.ID, AssetID: asset.ID, ActorID: duc, Comment: "Giao máy mới"}))

	r, err := s.GetRequest(ctx, req.ID)
	require.NoError(t, err)
	assert.Equal(t, "fulfilled", r.Status)
	require.NotNil(t, r.AssignedAssetID)
	assert.Equal(t, asset.ID, *r.AssignedAssetID)
	assert.Equal(t, "Trần Minh Đức", r.ApproverName)
	assert.Equal(t, "Laptop số 4", r.AssignedAssetName)

	a, err := s.GetAsset(ctx, asset.ID)
	require.NoError(t, err)
	assert.Equal(t, "assigned", a.State)
	require.NotNil(t, a.AssignedTo)
	assert.Equal(t, yen, *a.AssignedTo)

	hist, err := s.GetAssetHistory(ctx, asset.ID)
	require.NoError(t, err)
	require.Len(t, hist, 1)
	assert.Equal(t, duc, hist[0].ActorID)
	assert.Equal(t, yen, hist[0].SubjectUserID)
}

func TestApproveAndAssign_Refusals(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()
	wsID := getTestWorkspaceID(t, s.DB())
	owner := getTestUserID(t, s.DB())
	laptop := createTestType(t, s, wsID)
	monitor := createTestType(t, s, wsID)
	yen := member(t, s.DB(), wsID, "Yến")
	duc := member(t, s.DB(), wsID, "Đức")

	cases := []struct {
		name  string
		asset func() string
		req   func() string
		want  error
	}{
		{"asset already assigned", func() string { return newAsset(t, s, laptop.ID, wsID, owner, "taken", "assigned").ID }, func() string { return newRequest(t, s, laptop.ID, wsID, yen, "pending").ID }, store.ErrAssetUnavailable},
		{"asset in maintenance", func() string { return newAsset(t, s, laptop.ID, wsID, owner, "repair", "maintenance").ID }, func() string { return newRequest(t, s, laptop.ID, wsID, yen, "pending").ID }, store.ErrAssetUnavailable},
		{"asset of another type", func() string { return newAsset(t, s, monitor.ID, wsID, owner, "screen", "available").ID }, func() string { return newRequest(t, s, laptop.ID, wsID, yen, "pending").ID }, store.ErrWrongType},
		{"asset not found", func() string { return "missing-" + uuid.NewString() }, func() string { return newRequest(t, s, laptop.ID, wsID, yen, "pending").ID }, store.ErrNotFound},
		{"request already decided", func() string { return newAsset(t, s, laptop.ID, wsID, owner, "free", "available").ID }, func() string { return newRequest(t, s, laptop.ID, wsID, yen, "rejected").ID }, store.ErrRequestNotOpen},
		{"request not found", func() string { return newAsset(t, s, laptop.ID, wsID, owner, "free2", "available").ID }, func() string { return "missing-" + uuid.NewString() }, store.ErrNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assetID, reqID := tc.asset(), tc.req()
			err := s.ApproveAndAssign(ctx, store.FulfilParams{RequestID: reqID, AssetID: assetID, ActorID: duc})
			assert.ErrorIs(t, err, tc.want)
			if r, gerr := s.GetRequest(ctx, reqID); gerr == nil {
				assert.NotEqual(t, "fulfilled", r.Status)
				assert.Nil(t, r.AssignedAssetID, "a refused approval gives nothing")
			}
		})
	}
}

func TestApproveAndAssign_AWriteThatFailsMidwayLeavesNothingBehind(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()
	wsID := getTestWorkspaceID(t, s.DB())
	owner := getTestUserID(t, s.DB())
	at := createTestType(t, s, wsID)
	yen := member(t, s.DB(), wsID, "Yến")
	asset := newAsset(t, s, at.ID, wsID, owner, "Laptop", "available")
	req := newRequest(t, s, at.ID, wsID, yen, "pending")

	// An actor who is no user fails the history insert, after the asset row
	// was already changed inside the same transaction.
	err := s.ApproveAndAssign(ctx, store.FulfilParams{RequestID: req.ID, AssetID: asset.ID, ActorID: "no-such-user-" + uuid.NewString()})
	require.Error(t, err)

	a, gerr := s.GetAsset(ctx, asset.ID)
	require.NoError(t, gerr)
	assert.Equal(t, "available", a.State)
	assert.Nil(t, a.AssignedTo)
	r, gerr := s.GetRequest(ctx, req.ID)
	require.NoError(t, gerr)
	assert.Equal(t, "pending", r.Status)
	hist, gerr := s.GetAssetHistory(ctx, asset.ID)
	require.NoError(t, gerr)
	assert.Empty(t, hist)
}

func TestApproveAndAssign_TwoRequestsCannotTakeTheSameAsset(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()
	wsID := getTestWorkspaceID(t, s.DB())
	owner := getTestUserID(t, s.DB())
	at := createTestType(t, s, wsID)
	yen := member(t, s.DB(), wsID, "Yến")
	vinh := member(t, s.DB(), wsID, "Vinh")
	duc := member(t, s.DB(), wsID, "Đức")
	asset := newAsset(t, s, at.ID, wsID, owner, "Laptop", "available")
	r1 := newRequest(t, s, at.ID, wsID, yen, "pending")
	r2 := newRequest(t, s, at.ID, wsID, vinh, "pending")

	errs := make(chan error, 2)
	for _, id := range []string{r1.ID, r2.ID} {
		go func(id string) {
			errs <- s.ApproveAndAssign(ctx, store.FulfilParams{RequestID: id, AssetID: asset.ID, ActorID: duc})
		}(id)
	}
	e1, e2 := <-errs, <-errs
	assert.True(t, (e1 == nil) != (e2 == nil), "exactly one wins: %v / %v", e1, e2)
	for _, e := range []error{e1, e2} {
		if e != nil {
			assert.ErrorIs(t, e, store.ErrAssetUnavailable)
		}
	}
}

func TestAssignApproved_FulfilsAnApprovedRequestOnly(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()
	wsID := getTestWorkspaceID(t, s.DB())
	owner := getTestUserID(t, s.DB())
	at := createTestType(t, s, wsID)
	yen := member(t, s.DB(), wsID, "Yến")
	duc := member(t, s.DB(), wsID, "Đức")
	asset := newAsset(t, s, at.ID, wsID, owner, "Laptop", "available")
	pending := newRequest(t, s, at.ID, wsID, yen, "pending")
	approved := newRequest(t, s, at.ID, wsID, yen, "approved")

	assert.ErrorIs(t, s.AssignApproved(ctx, store.FulfilParams{RequestID: pending.ID, AssetID: asset.ID, ActorID: duc}), store.ErrRequestNotOpen)
	require.NoError(t, s.AssignApproved(ctx, store.FulfilParams{RequestID: approved.ID, AssetID: asset.ID, ActorID: duc}))
	r, err := s.GetRequest(ctx, approved.ID)
	require.NoError(t, err)
	assert.Equal(t, "fulfilled", r.Status)
	assert.Empty(t, r.ApproverName, "giving the asset does not make the giver the approver")
}

// ---------------------------------------------------------------------------
// Requests: status lists
// ---------------------------------------------------------------------------

func TestListRequests_StatusMayBeSeveralSeparatedByCommas(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()
	wsID := getTestWorkspaceID(t, s.DB())
	at := createTestType(t, s, wsID)
	yen := member(t, s.DB(), wsID, "Yến")
	for _, st := range []string{"pending", "approved", "fulfilled", "rejected"} {
		newRequest(t, s, at.ID, wsID, yen, st)
	}
	statuses := func(filter string) []string {
		list, total, err := s.ListRequests(ctx, store.ListRequestsFilter{WorkspaceID: wsID, Status: filter})
		require.NoError(t, err)
		assert.Equal(t, int32(len(list)), total)
		var out []string
		for _, r := range list {
			out = append(out, r.Status)
		}
		return out
	}
	assert.ElementsMatch(t, []string{"approved", "fulfilled"}, statuses("approved,fulfilled"))
	assert.ElementsMatch(t, []string{"pending"}, statuses("pending"))
	assert.Len(t, statuses(""), 4)
}

// ---------------------------------------------------------------------------
// A decision is made once: nothing overwrites a request that already has one
// ---------------------------------------------------------------------------

func TestUpdateRequestStatus_OnlyWhilePending(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()
	wsID := getTestWorkspaceID(t, s.DB())
	owner := getTestUserID(t, s.DB())
	at := createTestType(t, s, wsID)
	yen := member(t, s.DB(), wsID, "Yến")
	duc := member(t, s.DB(), wsID, "Đức")
	asset := newAsset(t, s, at.ID, wsID, owner, "Laptop", "available")
	req := newRequest(t, s, at.ID, wsID, yen, "pending")

	require.NoError(t, s.ApproveAndAssign(ctx, store.FulfilParams{RequestID: req.ID, AssetID: asset.ID, ActorID: duc}))

	// A plain approve or a reject that read the request while it was pending lands late.
	assert.ErrorIs(t, s.UpdateRequestStatus(ctx, req.ID, "approved", duc, "late"), store.ErrRequestNotOpen)
	assert.ErrorIs(t, s.UpdateRequestStatus(ctx, req.ID, "rejected", duc, "late"), store.ErrRequestNotOpen)
	r, err := s.GetRequest(ctx, req.ID)
	require.NoError(t, err)
	assert.Equal(t, "fulfilled", r.Status, "the fulfilled request keeps its asset")
	require.NotNil(t, r.AssignedAssetID)
	assert.Equal(t, asset.ID, *r.AssignedAssetID)

	assert.ErrorIs(t, s.UpdateRequestStatus(ctx, "missing-"+uuid.NewString(), "approved", duc, ""), store.ErrRequestNotOpen)
}

func TestRequestDecisions_ExactlyOneWins(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()
	wsID := getTestWorkspaceID(t, s.DB())
	owner := getTestUserID(t, s.DB())
	at := createTestType(t, s, wsID)
	yen := member(t, s.DB(), wsID, "Yến")
	duc := member(t, s.DB(), wsID, "Đức")

	race := func(t *testing.T, name string, a, b func(reqID, assetID string) error) {
		t.Run(name, func(t *testing.T) {
			for i := 0; i < 8; i++ {
				asset := newAsset(t, s, at.ID, wsID, owner, "Laptop", "available")
				req := newRequest(t, s, at.ID, wsID, yen, "pending")
				errs := make(chan error, 2)
				for _, f := range []func(string, string) error{a, b} {
					go func(f func(string, string) error) { errs <- f(req.ID, asset.ID) }(f)
				}
				e1, e2 := <-errs, <-errs
				assert.True(t, (e1 == nil) != (e2 == nil), "exactly one decision stands: %v / %v", e1, e2)

				r, err := s.GetRequest(ctx, req.ID)
				require.NoError(t, err)
				a, err := s.GetAsset(ctx, asset.ID)
				require.NoError(t, err)
				switch r.Status {
				case "fulfilled":
					require.NotNil(t, r.AssignedAssetID)
					assert.Equal(t, "assigned", a.State)
				default:
					assert.Nil(t, r.AssignedAssetID, "a request that was not fulfilled keeps no asset")
					assert.Equal(t, "available", a.State, "and its asset was not given away")
				}
			}
		})
	}
	assign := func(reqID, assetID string) error {
		return s.ApproveAndAssign(ctx, store.FulfilParams{RequestID: reqID, AssetID: assetID, ActorID: duc})
	}
	approve := func(reqID, _ string) error { return s.UpdateRequestStatus(ctx, reqID, "approved", duc, "") }
	reject := func(reqID, _ string) error { return s.UpdateRequestStatus(ctx, reqID, "rejected", duc, "không") }
	race(t, "approve and assign vs plain approve", assign, approve)
	race(t, "approve and assign vs reject", assign, reject)
	race(t, "approve vs reject", approve, reject)
}

// ---------------------------------------------------------------------------
// A step is taken on the state it was chosen from
// ---------------------------------------------------------------------------

func TestApplyTransition_RefusesWhenTheStateMovedOrTheAssetIsGone(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()
	wsID := getTestWorkspaceID(t, s.DB())
	owner := getTestUserID(t, s.DB())
	at := createTestType(t, s, wsID)

	a := newAsset(t, s, at.ID, wsID, owner, "Laptop", "assigned")
	err := s.ApplyTransition(ctx, &store.TransitionRecord{AssetID: a.ID, FromState: "available", ToState: "retired", Action: "retire", ActorID: owner}, nil)
	assert.ErrorIs(t, err, store.ErrStateChanged, "retiring an asset read as available, now assigned")
	got, _ := s.GetAsset(ctx, a.ID)
	assert.Equal(t, "assigned", got.State)
	hist, _ := s.GetAssetHistory(ctx, a.ID)
	assert.Empty(t, hist)

	gone := newAsset(t, s, at.ID, wsID, owner, "Gone", "available")
	require.NoError(t, s.SoftDeleteAsset(ctx, gone.ID))
	assert.ErrorIs(t, s.ApplyTransition(ctx, &store.TransitionRecord{AssetID: gone.ID, FromState: "available", ToState: "retired", Action: "retire", ActorID: owner}, nil), store.ErrNotFound)
}

func TestApplyTransition_ReturnComparesTheHolderUnderTheLock(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()
	wsID := getTestWorkspaceID(t, s.DB())
	owner := getTestUserID(t, s.DB())
	at := createTestType(t, s, wsID)
	lan := member(t, s.DB(), wsID, "Lan")
	vinh := member(t, s.DB(), wsID, "Vinh")
	a := newAsset(t, s, at.ID, wsID, owner, "Laptop", "available")
	require.NoError(t, s.HandOver(ctx, a.ID, lan, owner, ""))
	// The caller read Lan as the holder, then the asset moved to Vinh.
	require.NoError(t, s.HandOver(ctx, a.ID, vinh, owner, ""))

	err := s.ApplyTransition(ctx, &store.TransitionRecord{AssetID: a.ID, FromState: "assigned", ToState: "available", Action: "return", ActorID: owner, ExpectHolder: lan}, nil)
	assert.ErrorIs(t, err, store.ErrStateChanged)
	got, _ := s.GetAsset(ctx, a.ID)
	require.NotNil(t, got.AssignedTo)
	assert.Equal(t, vinh, *got.AssignedTo, "Vinh keeps the asset")

	require.NoError(t, s.ApplyTransition(ctx, &store.TransitionRecord{AssetID: a.ID, FromState: "assigned", ToState: "available", Action: "return", ActorID: owner, ExpectHolder: vinh}, nil))
}

func TestTransitions_RaceWithApproveAndAssignAndHandOver(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()
	wsID := getTestWorkspaceID(t, s.DB())
	owner := getTestUserID(t, s.DB())
	at := createTestType(t, s, wsID)
	yen := member(t, s.DB(), wsID, "Yến")
	lan := member(t, s.DB(), wsID, "Lan")
	duc := member(t, s.DB(), wsID, "Đức")

	for i := 0; i < 8; i++ {
		// retire vs approve-and-assign on one available asset
		asset := newAsset(t, s, at.ID, wsID, owner, "Laptop", "available")
		req := newRequest(t, s, at.ID, wsID, yen, "pending")
		errs := make(chan error, 2)
		go func() {
			errs <- s.ApplyTransition(ctx, &store.TransitionRecord{AssetID: asset.ID, FromState: "available", ToState: "retired", Action: "retire", ActorID: duc}, nil)
		}()
		go func() {
			errs <- s.ApproveAndAssign(ctx, store.FulfilParams{RequestID: req.ID, AssetID: asset.ID, ActorID: duc})
		}()
		e1, e2 := <-errs, <-errs
		assert.True(t, (e1 == nil) != (e2 == nil), "exactly one wins: %v / %v", e1, e2)
		got, _ := s.GetAsset(ctx, asset.ID)
		r, _ := s.GetRequest(ctx, req.ID)
		if got.State == "retired" {
			assert.Nil(t, got.AssignedTo, "a retired asset has no holder")
			assert.Equal(t, "pending", r.Status)
		} else {
			assert.Equal(t, "assigned", got.State)
			assert.Equal(t, "fulfilled", r.Status)
		}

		// a return racing a hand-over of the same assigned asset
		held := newAsset(t, s, at.ID, wsID, owner, "Laptop 2", "available")
		require.NoError(t, s.HandOver(ctx, held.ID, yen, owner, ""))
		go func() {
			errs <- s.ApplyTransition(ctx, &store.TransitionRecord{AssetID: held.ID, FromState: "assigned", ToState: "available", Action: "return", ActorID: duc, ExpectHolder: yen}, nil)
		}()
		go func() { errs <- s.HandOver(ctx, held.ID, lan, owner, "") }()
		e1, e2 = <-errs, <-errs
		got, _ = s.GetAsset(ctx, held.ID)
		if e1 == nil && e2 == nil {
			// return first, then hand-over to Lan: Lan holds it.
			require.NotNil(t, got.AssignedTo)
			assert.Equal(t, lan, *got.AssignedTo)
		} else {
			assert.True(t, (e1 == nil) != (e2 == nil), "%v / %v", e1, e2)
			if got.State == "assigned" {
				require.NotNil(t, got.AssignedTo)
			} else {
				assert.Nil(t, got.AssignedTo, "an asset in stock has no holder")
			}
		}
	}
}

func TestSummary_HoldersAreThoseHoldingAnAssignedAsset(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()
	wsID := getTestWorkspaceID(t, s.DB())
	owner := getTestUserID(t, s.DB())
	at := createTestType(t, s, wsID)
	lan := member(t, s.DB(), wsID, "Lan")
	yen := member(t, s.DB(), wsID, "Yến")
	a := newAsset(t, s, at.ID, wsID, owner, "a", "assigned")
	m := newAsset(t, s, at.ID, wsID, owner, "m", "maintenance")
	_, err := s.DB().Exec(ctx, `UPDATE assets SET assigned_to = $2 WHERE id = $1`, a.ID, lan)
	require.NoError(t, err)
	_, err = s.DB().Exec(ctx, `UPDATE assets SET assigned_to = $2 WHERE id = $1`, m.ID, yen)
	require.NoError(t, err)

	sum, err := s.Summary(ctx, wsID, []string{at.ID})
	require.NoError(t, err)
	assert.Equal(t, int32(1), sum.ByState["assigned"])
	assert.Equal(t, int32(1), sum.Holders, "the one in maintenance is not counted among 'Đang giao … cho N người'")
}

func TestAllActiveMembers(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()
	wsID := getTestWorkspaceID(t, s.DB())
	lan := member(t, s.DB(), wsID, "Lan")
	other := member(t, s.DB(), "", "Người ngoài")
	ok, err := s.AllActiveMembers(ctx, wsID, []string{lan, lan})
	require.NoError(t, err)
	assert.True(t, ok)
	ok, _ = s.AllActiveMembers(ctx, wsID, []string{lan, other})
	assert.False(t, ok)
	ok, _ = s.AllActiveMembers(ctx, wsID, nil)
	assert.True(t, ok)
}

func TestSummary_CountsMaintenanceOlderThanFourteenDays(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()
	wsID := getTestWorkspaceID(t, s.DB())
	owner := getTestUserID(t, s.DB())
	at := createTestType(t, s, wsID)
	old := newAsset(t, s, at.ID, wsID, owner, "old repair", "maintenance")
	fresh := newAsset(t, s, at.ID, wsID, owner, "new repair", "maintenance")
	other := newAsset(t, s, at.ID, wsID, owner, "available", "available")
	for _, a := range []*store.Asset{old, fresh} {
		require.NoError(t, s.InsertTransition(ctx, &store.TransitionRecord{AssetID: a.ID, FromState: "available", ToState: "maintenance", Action: "flag_maintenance", ActorID: owner}))
	}
	_, err := s.DB().Exec(ctx, `UPDATE asset_transitions SET created_at = NOW() - INTERVAL '15 days' WHERE asset_id = $1`, old.ID)
	require.NoError(t, err)
	_, err = s.DB().Exec(ctx, `UPDATE assets SET updated_at = NOW() - INTERVAL '40 days' WHERE id = $1`, other.ID)
	require.NoError(t, err)

	sum, err := s.Summary(ctx, wsID, []string{at.ID})
	require.NoError(t, err)
	assert.Equal(t, int32(2), sum.ByState["maintenance"])
	assert.Equal(t, int32(1), sum.MaintenanceOverdue)

	none, err := s.Summary(ctx, wsID, []string{})
	require.NoError(t, err)
	assert.Zero(t, none.MaintenanceOverdue)
}

func TestListRequestDecisions_OnlyDecidedOnesOfReadableTypes(t *testing.T) {
	s := setupStore(t)
	ctx := context.Background()
	wsID := getTestWorkspaceID(t, s.DB())
	at := createTestType(t, s, wsID)
	hidden := createTestType(t, s, wsID)
	yen := member(t, s.DB(), wsID, "Phạm Hải Yến")
	duc := member(t, s.DB(), wsID, "Trần Minh Đức")
	approved := newRequest(t, s, at.ID, wsID, yen, "pending")
	rejected := newRequest(t, s, at.ID, wsID, yen, "pending")
	newRequest(t, s, at.ID, wsID, yen, "pending")
	secret := newRequest(t, s, hidden.ID, wsID, yen, "pending")
	require.NoError(t, s.UpdateRequestStatus(ctx, approved.ID, "approved", duc, ""))
	require.NoError(t, s.UpdateRequestStatus(ctx, rejected.ID, "rejected", duc, "không"))
	require.NoError(t, s.UpdateRequestStatus(ctx, secret.ID, "approved", duc, ""))

	got, err := s.ListRequestDecisions(ctx, wsID, []string{at.ID}, 10)
	require.NoError(t, err)
	require.Len(t, got, 2, "pending ones and other types are left out")
	for _, e := range got {
		assert.Equal(t, "Trần Minh Đức", e.ActorName)
		assert.Equal(t, "Phạm Hải Yến", e.SubjectName)
		assert.Equal(t, at.Name, e.AssetName)
	}
	statuses := []string{got[0].RequestStatus, got[1].RequestStatus}
	assert.ElementsMatch(t, []string{"approved", "rejected"}, statuses)

	none, err := s.ListRequestDecisions(ctx, wsID, []string{}, 10)
	require.NoError(t, err)
	assert.Empty(t, none)
}
