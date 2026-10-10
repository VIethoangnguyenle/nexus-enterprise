package rest

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"

	"ngac-platform/pkg/httputil"
	"ngac-platform/services/messaging/internal/domain"
	"ngac-platform/services/messaging/internal/store"
	"ngac-platform/testutil"
)

// A channel that does not exist answers 404 with a plain message, not the
// generic 500 an unclassified error gets.
func TestGetUnknownChannelIs404(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	svc := domain.NewService(store.NewStore(pool), nil, nil, nil, nil)
	h := NewHandler(svc, nil, nil)

	e := httputil.NewEcho("messaging")
	e.GET("/api/channels/:chId", h.GetChannel, func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			httputil.SetClaims(c, &httputil.Claims{UserID: "u", NGACNodeID: "n"})
			return next(c)
		}
	})
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/channels/no-such-channel", nil))

	require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	require.True(t, strings.Contains(rec.Body.String(), "channel not found"), rec.Body.String())
	require.NotContains(t, rec.Body.String(), "request_id")
}
