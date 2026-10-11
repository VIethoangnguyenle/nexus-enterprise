package rest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"ngac-platform/pkg/httputil"
	policypb "ngac-platform/proto/policy"
)

// A personal workspace has no assets and no asset requests: every route of the
// service answers 403, whoever asks. Walking the route table, rather than
// listing routes by hand, means a route added later is covered without a new test.

// BatchCheckAccess answers the listings the walk below reaches, from the same
// grants as CheckAccess (there are none for the routes it visits).
func (g *grantPolicy) BatchCheckAccess(_ context.Context, req *policypb.BatchCheckAccessRequest, _ ...grpc.CallOption) (*policypb.BatchAccessResult, error) {
	results := map[string]*policypb.ObjectPermissions{}
	for _, obj := range req.ObjectIds {
		perms := map[string]bool{}
		for _, op := range req.Operations {
			perms[op] = g.grants[[3]string{req.UserNodeId, obj, op}]
		}
		results[obj] = &policypb.ObjectPermissions{Permissions: perms}
	}
	return &policypb.BatchAccessResult{Results: results}, nil
}

const restTestSecret = "personal-workspace-test-secret-0123456789"

func (f *flowFixture) bearer(t *testing.T) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, &httputil.Claims{
		UserID: f.approver, NGACNodeID: nodeOf(f.approver),
		RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))},
	})
	s, err := tok.SignedString([]byte(restTestSecret))
	require.NoError(t, err)
	return "Bearer " + s
}

// routeCall is one registered route with its path parameters filled in.
type routeCall struct{ method, route, url string }

func (f *flowFixture) routes(e *echo.Echo) []routeCall {
	fill := strings.NewReplacer(":id", f.wsID, ":typeId", f.typeID, ":assetId", f.assetID, ":reqId", f.requestID)
	var out []routeCall
	for _, r := range e.Routes() {
		if strings.HasPrefix(r.Path, "/api/") && !strings.Contains(r.Path, "*") {
			out = append(out, routeCall{r.Method, r.Method + " " + r.Path, fill.Replace(r.Path)})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].route < out[j].route })
	return out
}

// anyBody carries every field any route reads, so a request fails on the
// workspace type and never earlier on a missing field.
func (f *flowFixture) anyBody() string {
	return `{"type_id":"` + f.typeID + `","name":"x","category":"hardware","fields_schema":"{}",` +
		`"reason":"x","justification":"x","asset_id":"` + f.assetID + `","assignee_id":"` + f.requester + `","action":"retire"}`
}

func (f *flowFixture) do(e *echo.Echo, t *testing.T, c routeCall) *httptest.ResponseRecorder {
	req := httptest.NewRequest(c.method, c.url, strings.NewReader(f.anyBody()))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.Header.Set(echo.HeaderAuthorization, f.bearer(t))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func (f *flowFixture) echo() *echo.Echo {
	e := echo.New()
	e.HTTPErrorHandler = httputil.ErrorHandler("asset-test")
	f.h.RegisterRoutes(e, restTestSecret)
	return e
}

func TestPersonalWorkspace_EveryRouteAnswers403(t *testing.T) {
	f := newFlowFixture(t)
	_, err := f.pool.Exec(context.Background(), `UPDATE workspaces SET type = 'personal' WHERE id = $1`, f.wsID)
	require.NoError(t, err)
	e := f.echo()

	calls := f.routes(e)
	require.Len(t, calls, 22, "the asset service has 22 routes; update this when it changes")
	for _, c := range calls {
		t.Run(c.route, func(t *testing.T) {
			rec := f.do(e, t, c)

			assert.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
			assert.Contains(t, rec.Body.String(), "personal_workspace")
			assert.NotContains(t, rec.Body.String(), f.wsID)
		})
	}
}

func TestOrganizationWorkspace_RoutesAreNotRefusedAsPersonal(t *testing.T) {
	f := newFlowFixture(t)
	e := f.echo()

	for _, c := range f.routes(e) {
		t.Run(c.route, func(t *testing.T) {
			rec := f.do(e, t, c)

			assert.NotContains(t, rec.Body.String(), "personal_workspace", rec.Body.String())
		})
	}
	// A listing with no grants is not an error: nothing about the guard changes it.
	rec := f.do(e, t, routeCall{http.MethodGet, "", "/api/workspaces/" + f.wsID + "/assets"})
	assert.Equal(t, http.StatusOK, rec.Code)
}
