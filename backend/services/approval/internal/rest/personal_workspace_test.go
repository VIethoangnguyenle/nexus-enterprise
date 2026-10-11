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
	return &workspaceEnv{e: routerOver(pool), ws: ws, user: user, node: node}
}

func routerOver(pool *pgxpool.Pool) *echo.Echo {
	svc := domain.NewService(store.NewStore(pool), noScopes{})
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
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, &httputil.Claims{
		UserID: w.user, NGACNodeID: w.node, TenantID: w.ws,
		RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))},
	})
	signed, err := tok.SignedString([]byte(personalTestSecret))
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(r.method, r.url, strings.NewReader(walkBody))
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
	t.Run("without a schema", func(t *testing.T) {
		env := newWorkspaceEnv(t, "organization", false)
		rec := env.do(t, walkedRoute{http.MethodGet, "", "/api/approval/pending"})
		if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), "tenant schema not provisioned") {
			t.Errorf("status %d body %s; want the unchanged 404", rec.Code, rec.Body)
		}
	})
}
