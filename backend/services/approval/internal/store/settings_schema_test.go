package store_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"ngac-platform/pkg/httputil"
	"ngac-platform/services/approval/internal/domain"
	"ngac-platform/services/approval/internal/store"
	"ngac-platform/testutil"
)

// settingsTableIn reports whether the schema has its approval_user_settings table.
func settingsTableIn(t *testing.T, pool *pgxpool.Pool, schema string) bool {
	t.Helper()
	var ok bool
	if err := pool.QueryRow(context.Background(),
		`SELECT to_regclass(format('%I.approval_user_settings', $1::text)) IS NOT NULL`, schema).Scan(&ok); err != nil {
		t.Fatal(err)
	}
	return ok
}

// The settings table is part of a tenant's schema: provisioning makes it, with
// the rest of the schema, and a settings request never does.
func TestEnsureSchema_ProvisionsTheSettingsTable(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	st := store.NewStore(pool)
	id := "set-" + uuid.NewString()
	newOrg(t, pool, id)

	schema, err := st.EnsureSchema(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if !settingsTableIn(t, pool, schema) {
		t.Fatal("a new tenant schema has no approval_user_settings")
	}
	ctx := httputil.WithTenantSchema(context.Background(), schema)
	user, _ := testutil.CreateUser(t, pool)
	if err := st.UpsertApprovalUserSettings(ctx, &domain.SettingsRow{
		UserID: user, DefaultTab: "mine", Sort: "oldest", NotifyPending: false, NotifyResult: true,
	}); err != nil {
		t.Fatalf("save settings into the provisioned table: %v", err)
	}

	// Idempotent: ensuring again keeps the table and what is in it.
	if _, err := st.EnsureSchema(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	row, err := st.GetApprovalUserSettings(ctx, user)
	if err != nil || row == nil || row.DefaultTab != "mine" {
		t.Fatalf("settings after a second ensure = %+v, %v", row, err)
	}
}

// The admin provisioning endpoint's path makes the table too.
func TestProvisionSchema_ProvisionsTheSettingsTable(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	st := store.NewStore(pool)
	id := "set-" + uuid.NewString()
	newOrg(t, pool, id)

	schema, err := st.ProvisionSchema(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if !settingsTableIn(t, pool, schema) {
		t.Fatal("ProvisionSchema left the schema without approval_user_settings")
	}
}

// A schema provisioned before the table existed gets it from
// EnsureTenantTables, which the service runs when it starts; running it again
// changes nothing. Until then a settings read fails rather than creating the
// table itself.
func TestEnsureTenantTables_BringsAnExistingSchemaUpToDate(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	st := store.NewStore(pool)
	id := "set-" + uuid.NewString()
	newOrg(t, pool, id)
	schema, err := st.EnsureSchema(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(context.Background(), fmt.Sprintf(`DROP TABLE %q.approval_user_settings`, schema)); err != nil {
		t.Fatal(err)
	}
	ctx := httputil.WithTenantSchema(context.Background(), schema)

	if _, err := st.GetApprovalUserSettings(ctx, "anyone"); err == nil {
		t.Fatal("a settings read on a schema without the table succeeded: it must not create the table itself")
	}
	if settingsTableIn(t, pool, schema) {
		t.Fatal("a settings read created the table")
	}

	for i := 0; i < 2; i++ {
		if err := st.EnsureTenantTables(context.Background()); err != nil {
			t.Fatalf("run %d: %v", i+1, err)
		}
	}
	if !settingsTableIn(t, pool, schema) {
		t.Fatal("EnsureTenantTables left the schema without approval_user_settings")
	}
	if row, err := st.GetApprovalUserSettings(ctx, "anyone"); err != nil || row != nil {
		t.Fatalf("read after the table exists = %+v, %v; want no row, no error", row, err)
	}
}
