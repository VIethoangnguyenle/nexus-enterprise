package store_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"ngac-platform/services/approval/internal/domain"
	"ngac-platform/services/approval/internal/store"
	"ngac-platform/testutil"
)

// newOrg inserts an organization workspace with the given ID and removes it, its
// registry row and its schema afterwards.
func newOrg(t *testing.T, pool *pgxpool.Pool, id string) {
	t.Helper()
	bg := context.Background()
	user, _ := testutil.CreateUser(t, pool)
	if _, err := pool.Exec(bg, `INSERT INTO workspaces (id, name, owner_id, type) VALUES ($1, 'org', $2, 'organization')`, id, user); err != nil {
		t.Fatalf("workspace: %v", err)
	}
	t.Cleanup(func() {
		var schema string
		if pool.QueryRow(bg, `SELECT schema_name FROM tenant_schemas WHERE tenant_id = $1`, id).Scan(&schema) == nil {
			pool.Exec(bg, fmt.Sprintf(`DROP SCHEMA IF EXISTS %q CASCADE`, schema))
		}
		pool.Exec(bg, `DELETE FROM tenant_schemas WHERE tenant_id = $1`, id)
		pool.Exec(bg, `DELETE FROM workspaces WHERE id = $1`, id)
	})
}

func status(t *testing.T, pool *pgxpool.Pool, id string) string {
	t.Helper()
	var s string
	if err := pool.QueryRow(context.Background(), `SELECT status FROM tenant_schemas WHERE tenant_id = $1`, id).Scan(&s); err != nil {
		return ""
	}
	return s
}

func TestEnsureSchema_CreatesWhenAbsentAndKeepsIt(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	st := store.NewStore(pool)
	id := "ens-" + uuid.NewString()
	newOrg(t, pool, id)

	first, err := st.EnsureSchema(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	again, err := st.EnsureSchema(context.Background(), id)
	if err != nil || again != first {
		t.Fatalf("second call = %q, %v; want %q", again, err, first)
	}
	if status(t, pool, id) != "active" {
		t.Errorf("status = %q; want active", status(t, pool, id))
	}
}

func TestEnsureSchema_NeverRevivesAStatusItDidNotSet(t *testing.T) {
	for _, st := range []string{"disabled", "migrating", "provisioning"} {
		t.Run(st, func(t *testing.T) {
			pool := testutil.SetupTestDB(t)
			s := store.NewStore(pool)
			id := "ens-" + uuid.NewString()
			newOrg(t, pool, id)
			name := "tenant_" + strings.ReplaceAll(id, "-", "_")
			if _, err := pool.Exec(context.Background(), `INSERT INTO tenant_schemas (tenant_id, schema_name, status) VALUES ($1, $2, $3)`, id, name, st); err != nil {
				t.Fatal(err)
			}

			_, err := s.EnsureSchema(context.Background(), id)

			if !errors.Is(err, domain.ErrSchemaNotActive) {
				t.Fatalf("err = %v; want ErrSchemaNotActive", err)
			}
			if got := status(t, pool, id); got != st {
				t.Errorf("status = %q; want it left as %q", got, st)
			}
		})
	}
}

// Two workspaces whose IDs begin alike each get a schema of their own.
func TestProvisionSchema_NamesComeFromTheWholeID(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	st := store.NewStore(pool)
	stem := strings.ReplaceAll(uuid.NewString(), "-", "")[:8]
	a, b := stem+"-aaaa-4000-8000-000000000001", stem+"-bbbb-4000-8000-000000000002"
	newOrg(t, pool, a)
	newOrg(t, pool, b)

	sa, err := st.EnsureSchema(context.Background(), a)
	if err != nil {
		t.Fatal(err)
	}
	sb, err := st.EnsureSchema(context.Background(), b)
	if err != nil {
		t.Fatalf("second workspace with the same first 8 characters: %v", err)
	}
	if sa == sb {
		t.Fatalf("both got %q", sa)
	}
	if want := "tenant_" + strings.ReplaceAll(strings.ToLower(a), "-", "_"); sa != want {
		t.Errorf("schema = %q; want %q", sa, want)
	}
}

// A tenant registered before the naming changed keeps its schema.
func TestProvisionSchema_ARegisteredTenantKeepsItsSchemaName(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	st := store.NewStore(pool)
	id := "ens-" + uuid.NewString()
	newOrg(t, pool, id)
	legacy := "tenant_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:8]
	if _, err := pool.Exec(context.Background(), `INSERT INTO tenant_schemas (tenant_id, schema_name, status) VALUES ($1, $2, 'active')`, id, legacy); err != nil {
		t.Fatal(err)
	}

	got, err := st.ProvisionSchema(context.Background(), id)

	if err != nil || got != legacy {
		t.Fatalf("schema = %q, %v; want %q", got, err, legacy)
	}
}
