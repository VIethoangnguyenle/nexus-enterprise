package store_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"

	"ngac-platform/pkg/httputil"
	"ngac-platform/services/approval/internal/domain"
	"ngac-platform/services/approval/internal/store"
	"ngac-platform/testutil"
)

// Each person's saved switches are read back by their user node; a person who
// never saved settings, and a tenant whose settings table does not exist yet,
// have both notifications on.
func TestNotifyOffByNode_ReadsEachPersonsOwnSwitches(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	st := store.NewStore(pool)
	id := "ntf-" + uuid.NewString()
	newOrg(t, pool, id)
	schema, err := st.EnsureSchema(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	ctx := httputil.WithTenantSchema(context.Background(), schema)
	quiet, quietNode := testutil.CreateUser(t, pool)
	noResult, noResultNode := testutil.CreateUser(t, pool)
	_, untouchedNode := testutil.CreateUser(t, pool)
	nodes := []string{quietNode, noResultNode, untouchedNode}

	pending, result, err := st.NotifyOffByNode(ctx, nodes)
	if err != nil {
		t.Fatalf("before any settings: %v", err)
	}
	if len(pending) != 0 || len(result) != 0 {
		t.Fatalf("before any settings: pending off %v, result off %v; want nobody", pending, result)
	}
	// A schema provisioned before the settings table existed has nobody opted out.
	if _, err := pool.Exec(context.Background(), fmt.Sprintf(`DROP TABLE %q.approval_user_settings`, schema)); err != nil {
		t.Fatal(err)
	}
	if pending, result, err := st.NotifyOffByNode(ctx, nodes); err != nil || len(pending) != 0 || len(result) != 0 {
		t.Fatalf("without the table: pending off %v, result off %v, err %v; want nobody, no error", pending, result, err)
	}
	if err := st.EnsureTenantTables(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, row := range []*domain.SettingsRow{
		{UserID: quiet, DefaultTab: "pending", Sort: "newest", NotifyPending: false, NotifyResult: true},
		{UserID: noResult, DefaultTab: "pending", Sort: "newest", NotifyPending: true, NotifyResult: false},
	} {
		if err := st.UpsertApprovalUserSettings(ctx, row); err != nil {
			t.Fatal(err)
		}
	}

	pending, result, err = st.NotifyOffByNode(ctx, nodes)
	if err != nil {
		t.Fatal(err)
	}
	if !pending[quietNode] || pending[noResultNode] || pending[untouchedNode] {
		t.Errorf("pending off = %v; want only %s", pending, quietNode)
	}
	if !result[noResultNode] || result[quietNode] || result[untouchedNode] {
		t.Errorf("result off = %v; want only %s", result, noResultNode)
	}
}
