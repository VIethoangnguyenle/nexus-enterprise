package rest

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"

	"ngac-platform/ngac"
	"ngac-platform/pkg/httputil"
	"ngac-platform/services/approval/internal/domain"
	"ngac-platform/services/approval/internal/store"
	"ngac-platform/testutil"
)

// A personal workspace has no approvals: every route of the service answers 403,
// whoever asks and whether or not the workspace has an approval schema. The
// routes are read from the router, so one added later is walked without a new test.

const personalTestSecret = "personal-workspace-test-secret-0123456789"

type workspaceEnv struct {
	e    *echo.Echo
	ws   string
	user string
	node string
	pool *pgxpool.Pool
}

// dropSchema removes the approval schema the service provisioned during a test.
func (w *workspaceEnv) dropSchema(t *testing.T) {
	t.Helper()
	bg := context.Background()
	var schema string
	if err := w.pool.QueryRow(bg, `SELECT schema_name FROM tenant_schemas WHERE tenant_id = $1`, w.ws).Scan(&schema); err != nil {
		return
	}
	w.pool.Exec(bg, fmt.Sprintf(`DROP SCHEMA IF EXISTS %q CASCADE`, schema))
	w.pool.Exec(bg, `DELETE FROM tenant_schemas WHERE tenant_id = $1`, w.ws)
}

// newWorkspaceEnv inserts a workspace of the given type (with an approval
// schema if asked) and returns the real router over the real store.
func newWorkspaceEnv(t *testing.T, kind string, withSchema bool) *workspaceEnv {
	t.Helper()
	pool := testutil.SetupTestDB(t)
	user, node := testutil.CreateUser(t, pool)
	bg := context.Background()
	ws := "z" + strings.ReplaceAll(uuid.NewString(), "-", "")[:7]
	if _, err := pool.Exec(bg, `INSERT INTO workspaces (id, name, owner_id, type) VALUES ($1, 'walk', $2, $3)`, ws, user, kind); err != nil {
		t.Fatalf("workspace: %v", err)
	}
	t.Cleanup(func() { pool.Exec(bg, `DELETE FROM workspaces WHERE id = $1`, ws) })
	if withSchema {
		var schema string
		if err := pool.QueryRow(bg, `SELECT provision_tenant_schema($1)`, ws).Scan(&schema); err != nil {
			t.Fatalf("provision schema: %v", err)
		}
		t.Cleanup(func() {
			pool.Exec(bg, fmt.Sprintf(`DROP SCHEMA IF EXISTS %q CASCADE`, schema))
			pool.Exec(bg, `DELETE FROM tenant_schemas WHERE tenant_id = $1`, ws)
		})
	}
	return &workspaceEnv{e: routerOver(pool), ws: ws, user: user, node: node, pool: pool}
}

func routerOver(pool *pgxpool.Pool) *echo.Echo {
	return routerWith(pool, noScopes{})
}

// manageAnywhere lets every caller manage every object, so a test can reach what
// follows the authorization check.
type manageAnywhere struct{ noScopes }

func (manageAnywhere) CheckAccess(context.Context, string, string, string) (bool, error) {
	return true, nil
}

func routerWith(pool *pgxpool.Pool, policy domain.PolicyClient) *echo.Echo {
	svc := domain.NewService(store.NewStore(pool), policy)
	e := httputil.NewEcho("approval-test")
	NewHandler(svc, httputil.NewTenantSchemaResolver(pool), nil).RegisterRoutes(e, personalTestSecret)
	return e
}

type walkedRoute struct{ method, route, url string }

func (w *workspaceEnv) routes() []walkedRoute {
	fill := strings.NewReplacer(":id", w.ws)
	var out []walkedRoute
	for _, r := range w.e.Routes() {
		if !strings.HasPrefix(r.Path, "/api/") || r.Method == echo.RouteNotFound {
			continue
		}
		path := fill.Replace(r.Path)
		if strings.Contains(r.Path, "/requests/:id") || strings.Contains(r.Path, "/templates/:id") {
			path = strings.Replace(r.Path, ":id", "11111111-1111-4111-8111-111111111111", 1)
		}
		out = append(out, walkedRoute{r.Method, r.Method + " " + r.Path, path})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].route < out[j].route })
	return out
}

// walkBody carries every field any route reads, so a request fails on the
// workspace type and never earlier on a missing field.
const walkBody = `{"template_id":"11111111-1111-4111-8111-111111111111","entity_type":"expense","name":"T","request_id":"11111111-1111-4111-8111-111111111111",` +
	`"request_ids":["11111111-1111-4111-8111-111111111111"],"comment":"x","expected_updated_at":"2026-10-01T08:00:00Z",` +
	`"steps":[{"step_order":1,"approver_type":"specific_user","approver_value":"x","required_count":1}]}`

func (w *workspaceEnv) do(t *testing.T, r walkedRoute) *httptest.ResponseRecorder {
	t.Helper()
	return w.doBody(t, r, walkBody)
}

func (w *workspaceEnv) doBody(t *testing.T, r walkedRoute, body string) *httptest.ResponseRecorder {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, &httputil.Claims{
		UserID: w.user, NGACNodeID: w.node, TenantID: w.ws,
		RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))},
	})
	signed, err := tok.SignedString([]byte(personalTestSecret))
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(r.method, r.url, strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.Header.Set(echo.HeaderAuthorization, "Bearer "+signed)
	rec := httptest.NewRecorder()
	w.e.ServeHTTP(rec, req)
	return rec
}

func TestPersonalWorkspace_EveryRouteAnswers403(t *testing.T) {
	for _, withSchema := range []bool{false, true} {
		t.Run(fmt.Sprintf("schema=%v", withSchema), func(t *testing.T) {
			env := newWorkspaceEnv(t, "personal", withSchema)
			routes := env.routes()
			if len(routes) != 16 {
				t.Fatalf("the approval service has 16 routes, found %d; update this when it changes", len(routes))
			}
			for _, r := range routes {
				t.Run(r.route, func(t *testing.T) {
					rec := env.do(t, r)

					if rec.Code != http.StatusForbidden {
						t.Fatalf("status %d body %s; want 403", rec.Code, rec.Body)
					}
					if !strings.Contains(rec.Body.String(), "personal workspace") {
						t.Errorf("body %s should say why", rec.Body)
					}
					if strings.Contains(rec.Body.String(), env.ws) {
						t.Errorf("body %s names the workspace ID", rec.Body)
					}
				})
			}
		})
	}
}

// An organization is untouched: with a schema no route is refused for its type,
// and without one the routes still answer 404, as they did before.
func TestOrganizationWorkspace_RoutesBehaveAsBefore(t *testing.T) {
	t.Run("with a schema", func(t *testing.T) {
		env := newWorkspaceEnv(t, "organization", true)
		for _, r := range env.routes() {
			if strings.Contains(r.route, "/provision") {
				continue // would provision a schema for the workspace
			}
			t.Run(r.route, func(t *testing.T) {
				rec := env.do(t, r)
				if strings.Contains(rec.Body.String(), "personal workspace") || strings.Contains(rec.Body.String(), "not available") {
					t.Fatalf("refused for its workspace type: %d %s", rec.Code, rec.Body)
				}
			})
		}
		if rec := env.do(t, walkedRoute{http.MethodGet, "", "/api/approval/pending"}); rec.Code != http.StatusOK {
			t.Errorf("GET pending status %d body %s; want 200", rec.Code, rec.Body)
		}
	})
	// An organization that has no schema yet (a personal workspace just converted)
	// gets one on its first approval call, instead of a 404.
	t.Run("without a schema", func(t *testing.T) {
		env := newWorkspaceEnv(t, "organization", false)
		t.Cleanup(func() { env.dropSchema(t) })
		rec := env.do(t, walkedRoute{http.MethodGet, "", "/api/approval/pending"})
		if rec.Code != http.StatusOK {
			t.Errorf("status %d body %s; want 200 after provisioning on first use", rec.Code, rec.Body)
		}
		if again := env.do(t, walkedRoute{http.MethodGet, "", "/api/approval/pending"}); again.Code != http.StatusOK {
			t.Errorf("second call status %d body %s; want 200", again.Code, again.Body)
		}
	})
}

// A workspace converted from personal can create an approval template at once,
// with no separate provisioning step.
func TestConvertedWorkspace_CanCreateTemplateRightAway(t *testing.T) {
	env := newWorkspaceEnv(t, "personal", false)
	t.Cleanup(func() { env.dropSchema(t) })
	bg := context.Background()
	mgmt := "oa-mgmt-" + env.ws
	for _, q := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO ngac_nodes (id, name, node_type, properties) VALUES ($1, $2, 'OA', '{}')`, []any{mgmt, ngac.MgmtOAName(ngac.WorkspaceID(env.ws))}},
		{`INSERT INTO tenant_users (tenant_id, user_id, role, status, ngac_node_id) VALUES ($1, $2, 'member', 'active', $3)`, []any{env.ws, env.user, env.node}},
	} {
		if _, err := env.pool.Exec(bg, q.sql, q.args...); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	t.Cleanup(func() {
		env.pool.Exec(bg, `DELETE FROM tenant_users WHERE tenant_id = $1`, env.ws)
		env.pool.Exec(bg, `DELETE FROM ngac_nodes WHERE id = $1`, mgmt)
	})
	env.e = routerWith(env.pool, manageAnywhere{})
	create := walkedRoute{http.MethodPost, "", "/api/approval/templates"}
	body := fmt.Sprintf(`{"name":"Chi phí","entity_type":"expense","steps":[{"step_order":1,"approver_type":"specific_user","approver_value":%q,"required_count":1}]}`, env.node)

	if rec := env.doBody(t, create, body); rec.Code != http.StatusForbidden {
		t.Fatalf("personal: status %d body %s; want 403", rec.Code, rec.Body)
	}
	if _, err := env.pool.Exec(bg, `UPDATE workspaces SET type = 'organization' WHERE id = $1`, env.ws); err != nil {
		t.Fatalf("convert: %v", err)
	}

	rec := env.doBody(t, create, body)

	if rec.Code != http.StatusCreated {
		t.Fatalf("converted: status %d body %s; want 201", rec.Code, rec.Body)
	}
	var schemas int
	if err := env.pool.QueryRow(bg, `SELECT count(*) FROM tenant_schemas WHERE tenant_id = $1 AND status = 'active'`, env.ws).Scan(&schemas); err != nil || schemas != 1 {
		t.Errorf("active schemas = %d, err %v; want exactly 1", schemas, err)
	}
}

// A database failure while looking for the schema is ours: 500, not a 404 that
// would send the caller to provision something.
func TestSchemaLookupFailureIs500(t *testing.T) {
	env := newWorkspaceEnv(t, "organization", true)
	broken, err := pgxpool.New(context.Background(), testutil.DatabaseURL())
	if err != nil {
		t.Fatal(err)
	}
	broken.Close()
	env.e = routerOver(broken)
	rec := env.do(t, walkedRoute{http.MethodGet, "", "/api/approval/pending"})
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status %d body %s; want 500", rec.Code, rec.Body)
	}
}

// An organization whose registry row is not active is not provisioned over: the
// answer stays 404 and the row keeps its status.
func TestOrganizationWithADisabledSchemaIsNotRevived(t *testing.T) {
	env := newWorkspaceEnv(t, "organization", false)
	bg := context.Background()
	if _, err := env.pool.Exec(bg, `INSERT INTO tenant_schemas (tenant_id, schema_name, status) VALUES ($1, $2, 'disabled')`, env.ws, "tenant_off_"+env.ws); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { env.pool.Exec(bg, `DELETE FROM tenant_schemas WHERE tenant_id = $1`, env.ws) })

	rec := env.do(t, walkedRoute{http.MethodGet, "", "/api/approval/pending"})

	if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), "tenant schema not provisioned") {
		t.Errorf("status %d body %s; want the 404", rec.Code, rec.Body)
	}
	var status string
	if err := env.pool.QueryRow(bg, `SELECT status FROM tenant_schemas WHERE tenant_id = $1`, env.ws).Scan(&status); err != nil || status != "disabled" {
		t.Errorf("status = %q, err %v; want it left disabled", status, err)
	}
}
