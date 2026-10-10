package httputil

import (
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"

	"ngac-platform/pkg/grpcauth"
)

func TestSetClaimsPutsCallerOnRequestContext(t *testing.T) {
	c := echo.New().NewContext(httptest.NewRequest("GET", "/", nil), httptest.NewRecorder())
	SetClaims(c, &Claims{UserID: "u1", NGACNodeID: "n1", TenantID: "t1"})
	got := grpcauth.CallerFrom(c.Request().Context())
	if got != (grpcauth.Caller{UserID: "u1", NGACNodeID: "n1", TenantID: "t1"}) {
		t.Fatalf("got %+v", got)
	}
}
