package rest

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"

	"ngac-platform/pkg/httputil"
)

// newContextWithClaims builds an Echo context carrying the given claims, as
// JWTMiddleware would have left it.
func newContextWithClaims(t *testing.T, tenantID, pathTenantID string) (echo.Context, *httptest.ResponseRecorder) {
	t.Helper()
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/admin/tenants/"+pathTenantID+"/provision", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(pathTenantID)
	httputil.SetClaims(c, &httputil.Claims{
		UserID:   "user-1",
		TenantID: tenantID,
	})
	return c, rec
}

// Provisioning builds a Postgres schema. Signing in to tenant A must not let a
// caller create schemas for tenant B.
func TestProvisionTenant_RejectsOtherTenant(t *testing.T) {
	h := &Handler{} // the tenant check runs before any dependency is touched
	c, _ := newContextWithClaims(t, "tenant-a", "tenant-b")

	err := h.ProvisionTenant(c)

	he, ok := err.(*echo.HTTPError)
	if !ok {
		t.Fatalf("err = %v (%T), want *echo.HTTPError", err, err)
	}
	if he.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", he.Code, http.StatusForbidden)
	}
}

// A request with no tenant context at all must not fall through to provisioning.
func TestProvisionTenant_RejectsMissingTenantContext(t *testing.T) {
	h := &Handler{}
	c, _ := newContextWithClaims(t, "", "tenant-b")

	err := h.ProvisionTenant(c)

	he, ok := err.(*echo.HTTPError)
	if !ok {
		t.Fatalf("err = %v (%T), want *echo.HTTPError", err, err)
	}
	if he.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", he.Code, http.StatusForbidden)
	}
}
