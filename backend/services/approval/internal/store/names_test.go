package store_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"ngac-platform/ngac"
	"ngac-platform/services/approval/internal/store"
	"ngac-platform/testutil"
)

// tenantRows builds one tenant's worth of rows in the shared tables: a
// workspace, a member with a display name, a member without one, a role UA under
// the workspace PC (named Role_<id>, shown by its display_name property), a role
// with no display name, and a department. It returns the keys a request would
// hold, and removes everything when the test ends.
type tenantRows struct {
	ws              string
	person, bare    string // user node ids
	role, plainRole string // UA ids
	dept, deptUA    string
	stranger        string // a user node that is NOT a member of this tenant
}

func seedTenant(t *testing.T, pool *pgxpool.Pool, label string) tenantRows {
	t.Helper()
	ctx := context.Background()
	sfx := uuid.NewString()
	r := tenantRows{
		ws: "names-ws-" + sfx, person: "names-person-" + sfx, bare: "names-bare-" + sfx,
		role: "names-role-" + sfx, plainRole: "names-plain-" + sfx, dept: "names-dept-" + sfx,
		deptUA: "names-dept-ua-" + sfx, stranger: "names-stranger-" + sfx,
	}
	pc := "names-pc-" + sfx
	owner, personUser, bareUser, strangerUser := "o-"+sfx, "p-"+sfx, "b-"+sfx, "s-"+sfx
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("fixture %q: %v", sql, err)
		}
	}
	exec(`INSERT INTO ngac_nodes (id, name, node_type) VALUES ($1, $2, 'PC')`, pc, ngac.PCName(ngac.WorkspaceID(r.ws)))
	exec(`INSERT INTO ngac_nodes (id, name, node_type) VALUES ($1, $1, 'U'), ($2, $2, 'U'), ($3, $3, 'U')`, r.person, r.bare, r.stranger)
	exec(`INSERT INTO ngac_nodes (id, name, node_type, properties) VALUES ($1, $2, 'UA', jsonb_build_object('display_name', $3::text))`, r.role, "Role_"+sfx, "Kế toán trưởng "+label)
	exec(`INSERT INTO ngac_nodes (id, name, node_type) VALUES ($1, $2, 'UA')`, r.plainRole, "Plain "+label)
	exec(`INSERT INTO ngac_assignments (id, child_id, parent_id) VALUES ($1, $2, $3), ($4, $5, $3)`, "a1-"+sfx, r.role, pc, "a2-"+sfx, r.plainRole)
	exec(`INSERT INTO users (id, username, password) VALUES ($1, $1, '')`, owner)
	exec(`INSERT INTO workspaces (id, name, owner_id) VALUES ($1, 'ws', $2)`, r.ws, owner)
	exec(`INSERT INTO users (id, username, password, ngac_node, display_name) VALUES ($1, $1, '', $2, 'Lê Thị Hoa '||$3::text)`, personUser, r.person, label)
	exec(`INSERT INTO users (id, username, password, ngac_node, display_name) VALUES ($1, 'bare-'||$3::text, '', $2, '')`, bareUser, r.bare, sfx)
	exec(`INSERT INTO users (id, username, password, ngac_node, display_name) VALUES ($1, $1, '', $2, 'Người ngoài')`, strangerUser, r.stranger)
	// Members of this tenant: the person by tenant_users.ngac_node_id, the bare one only by users.ngac_node.
	exec(`INSERT INTO tenant_users (tenant_id, user_id, ngac_node_id) VALUES ($1, $2, $3)`, r.ws, personUser, r.person)
	exec(`INSERT INTO tenant_users (tenant_id, user_id) VALUES ($1, $2)`, r.ws, bareUser)
	exec(`INSERT INTO departments (id, workspace_id, name, ngac_ua_id) VALUES ($1, $2, 'Vận hành thanh toán', $3)`, r.dept, r.ws, r.deptUA)
	t.Cleanup(func() {
		c := context.Background()
		_, _ = pool.Exec(c, `DELETE FROM departments WHERE workspace_id = $1`, r.ws)
		_, _ = pool.Exec(c, `DELETE FROM tenant_users WHERE tenant_id = $1`, r.ws)
		_, _ = pool.Exec(c, `DELETE FROM workspaces WHERE id = $1`, r.ws)
		_, _ = pool.Exec(c, `DELETE FROM users WHERE id = ANY($1)`, []string{owner, personUser, bareUser, strangerUser})
		_, _ = pool.Exec(c, `DELETE FROM ngac_assignments WHERE id = ANY($1)`, []string{"a1-" + sfx, "a2-" + sfx})
		_, _ = pool.Exec(c, `DELETE FROM ngac_nodes WHERE id = ANY($1)`, []string{pc, r.person, r.bare, r.stranger, r.role, r.plainRole})
	})
	return r
}

// Requests name people by NGAC node id, roles by their UA and departments by
// their own id. Each resolves to what a person would say, and a key that is
// nobody — or belongs to another tenant — is left out, so the client never has
// an id to fall back on.
func TestDisplayNames_ResolvesPeopleRolesAndDepartmentsOfTheTenant(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	a := seedTenant(t, pool, "A")

	got, err := store.NewStore(pool).DisplayNames(context.Background(), a.ws,
		[]string{a.person, a.bare, a.role, a.plainRole, a.dept, a.deptUA, "default", "nobody-" + uuid.NewString()})
	if err != nil {
		t.Fatalf("DisplayNames: %v", err)
	}
	if got[a.person] != "Lê Thị Hoa A" {
		t.Errorf("person (tenant_users.ngac_node_id) = %q", got[a.person])
	}
	if got[a.bare] == "" || got[a.bare][:5] != "bare-" {
		t.Errorf("a member without a display name falls back to the username, got %q", got[a.bare])
	}
	if got[a.role] != "Kế toán trưởng A" {
		t.Errorf("role = %q, want its display_name property, not the Role_<id> node name", got[a.role])
	}
	if got[a.plainRole] != "Plain A" {
		t.Errorf("role without the property falls back to its name, got %q", got[a.plainRole])
	}
	if got[a.dept] != "Vận hành thanh toán" || got[a.deptUA] != "Vận hành thanh toán" {
		t.Errorf("department by id %q / by UA %q", got[a.dept], got[a.deptUA])
	}
	if len(got) != 6 {
		t.Errorf("got %d names %v, want exactly the six that resolve", len(got), got)
	}
}

// Deny case: one tenant never reads another's names.
func TestDisplayNames_DoesNotCrossTenants(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	a := seedTenant(t, pool, "A")
	b := seedTenant(t, pool, "B")

	got, err := store.NewStore(pool).DisplayNames(context.Background(), a.ws,
		[]string{b.person, b.bare, b.role, b.plainRole, b.dept, b.deptUA, a.stranger})
	if err != nil {
		t.Fatalf("DisplayNames: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("tenant A resolved %v of tenant B / of a non-member; want nothing", got)
	}
	// And the other way round still works for the right tenant.
	own, _ := store.NewStore(pool).DisplayNames(context.Background(), b.ws, []string{b.person, b.role})
	if own[b.person] != "Lê Thị Hoa B" || own[b.role] != "Kế toán trưởng B" {
		t.Errorf("tenant B's own names = %v", own)
	}
}

func TestDisplayNames_EmptyInput(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	s := store.NewStore(pool)
	if got, err := s.DisplayNames(context.Background(), "ws", nil); err != nil || len(got) != 0 {
		t.Errorf("no keys: %v, %v", got, err)
	}
	if got, err := s.DisplayNames(context.Background(), "", []string{"x"}); err != nil || len(got) != 0 {
		t.Errorf("no tenant: %v, %v", got, err)
	}
}

func TestCanonicalApprover_OnlyThingsOfTheTenant(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	s := store.NewStore(pool)
	ctx := context.Background()
	a := seedTenant(t, pool, "A")
	b := seedTenant(t, pool, "B")

	ok := func(typ, val, want string) {
		t.Helper()
		got, err := s.CanonicalApprover(ctx, a.ws, typ, val)
		if err != nil || got != want {
			t.Errorf("%s %q = %q, %v; want %q", typ, val, got, err, want)
		}
	}
	bad := func(typ, val string) {
		t.Helper()
		if got, err := s.CanonicalApprover(ctx, a.ws, typ, val); err == nil {
			t.Errorf("%s %q accepted as %q; want ErrInvalidInput", typ, val, got)
		}
	}
	ok("specific_user", a.person, a.person)
	ok("role_in_dept", a.role, a.role)
	ok("department", a.dept, a.deptUA) // a department id becomes its UA
	ok("department", a.deptUA, a.deptUA)
	bad("specific_user", b.person)   // member of another tenant
	bad("specific_user", a.stranger) // exists, not a member
	bad("specific_user", a.role)     // a UA is not a person
	bad("role_in_dept", b.role)      // another tenant's role
	bad("role_in_dept", a.person)    // a person is not a role
	bad("department", b.dept)        // another tenant's department
	bad("department", "")
	bad("creator_manager", a.person)
}
