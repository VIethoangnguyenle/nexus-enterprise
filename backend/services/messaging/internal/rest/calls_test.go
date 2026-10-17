package rest

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/livekit/protocol/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/pkg/httputil"
	"ngac-platform/services/messaging/internal/domain"
	"ngac-platform/services/messaging/internal/livekit"
	"ngac-platform/services/messaging/internal/store"
)

const (
	webhookKey    = "test-key"
	webhookSecret = "test-secret-that-is-long-enough-0123456789"
)

// callsEcho serves the call routes as c's user, with a LiveKit client that is
// only ever asked to verify webhooks.
func (c *chat) callsEcho(t *testing.T, withMedia bool) *echo.Echo {
	t.Helper()
	svc := domain.NewService(store.NewStore(c.pool), answerPolicy{allow: true}, nil, namedUser{}, nil)
	h := NewHandler(svc, nil, nil)
	if withMedia {
		h.SetCallWebhooks(livekit.New(livekit.Config{
			URL: "http://livekit.invalid", PublicURL: "ws://livekit.invalid", APIKey: webhookKey, APISecret: webhookSecret,
		}))
	}
	e := httputil.NewEcho("messaging")
	as := func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(ctx echo.Context) error {
			httputil.SetClaims(ctx, &httputil.Claims{UserID: c.user, NGACNodeID: c.node, TenantID: c.ws})
			return next(ctx)
		}
	}
	e.POST("/api/channels/:chId/calls", h.StartCall, as)
	e.POST("/api/calendar-events/:eventId/call", h.StartEventCall, as)
	e.POST("/internal/livekit/webhook", h.LiveKitWebhook)
	return e
}

// signedWebhook is a webhook request as LiveKit sends it, signed with secret.
func signedWebhook(t *testing.T, secret, body string) *http.Request {
	t.Helper()
	sum := sha256.Sum256([]byte(body))
	token, err := auth.NewAccessToken(webhookKey, secret).
		SetSha256(base64.StdEncoding.EncodeToString(sum[:])).
		SetValidFor(time.Minute).
		ToJWT()
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/internal/livekit/webhook", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/webhook+json")
	req.Header.Set("Authorization", token)
	return req
}

func serve(e *echo.Echo, req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

// Deny: a webhook not signed with this server's secret is refused, whatever it
// claims; one that is signed is accepted.
func TestLiveKitWebhook_OnlySignedRequestsAreRead(t *testing.T) {
	c := newChat(t)
	e := c.callsEcho(t, true)
	body := `{"event":"room_finished","room":{"name":"no-such-call"}}`

	unsigned := httptest.NewRequest(http.MethodPost, "/internal/livekit/webhook", strings.NewReader(body))
	assert.Equal(t, http.StatusUnauthorized, serve(e, unsigned).Code, "no signature")
	assert.Equal(t, http.StatusUnauthorized, serve(e, signedWebhook(t, "another-secret-that-is-long-enough-012345", body)).Code, "wrong secret")

	tampered := signedWebhook(t, webhookSecret, body)
	tampered.Body = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"event":"room_finished","room":{"name":"other"}}`)).Body
	assert.Equal(t, http.StatusUnauthorized, serve(e, tampered).Code, "a body that is not the one signed")

	assert.Equal(t, http.StatusOK, serve(e, signedWebhook(t, webhookSecret, body)).Code)
}

func TestLiveKitWebhook_WithoutCallsRefuses(t *testing.T) {
	c := newChat(t)
	e := c.callsEcho(t, false)
	body := `{"event":"room_finished","room":{"name":"x"}}`
	assert.Equal(t, http.StatusServiceUnavailable, serve(e, signedWebhook(t, webhookSecret, body)).Code)
}

func TestStartCall_WithoutMediaAnswersUnavailable(t *testing.T) {
	c := newChat(t)
	rec := call(c.callsEcho(t, false), http.MethodPost, "/api/channels/"+c.chID+"/calls", `{"kind":"video"}`)
	require.Equal(t, http.StatusServiceUnavailable, rec.Code, rec.Body.String())
	assert.Equal(t, "calls_unavailable", decode(t, rec)["code"])
	var n int
	require.NoError(t, c.pool.QueryRow(context.Background(), `SELECT count(*) FROM calls WHERE channel_id = $1`, c.chID).Scan(&n))
	assert.Zero(t, n)
}

func TestStartEventCall_WithoutMediaAnswersUnavailable(t *testing.T) {
	c := newChat(t)
	rec := call(c.callsEcho(t, false), http.MethodPost, "/api/calendar-events/some-event/call", "")
	require.Equal(t, http.StatusServiceUnavailable, rec.Code, rec.Body.String())
	assert.Equal(t, "calls_unavailable", decode(t, rec)["code"])
}

// A client that does not know incoming_call yet leaves it as it was; one that
// sends it sets it.
func TestPushPreferences_IncomingCallSurvivesAnOlderClient(t *testing.T) {
	c := newChat(t)
	svc := domain.NewService(store.NewStore(c.pool), answerPolicy{allow: true}, nil, namedUser{}, nil)
	h := NewHandler(svc, nil, nil)
	e := httputil.NewEcho("messaging")
	as := func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(ctx echo.Context) error {
			httputil.SetClaims(ctx, &httputil.Claims{UserID: c.user, NGACNodeID: c.node, TenantID: c.ws})
			return next(ctx)
		}
	}
	e.GET("/api/me/push-preferences", h.GetPushPreferences, as)
	e.PUT("/api/me/push-preferences", h.PutPushPreferences, as)
	t.Cleanup(func() { c.pool.Exec(context.Background(), `DELETE FROM push_preferences WHERE user_id = $1`, c.user) })

	quiet := `"quiet_hours":{"enabled":false,"start":"22:00","end":"07:00","time_zone":"UTC"}`
	rec := call(e, http.MethodPut, "/api/me/push-preferences",
		`{"kinds":{"direct":true,"mention":true,"thread_reply":true,"incoming_call":false},`+quiet+`}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	rec = call(e, http.MethodPut, "/api/me/push-preferences", `{"kinds":{"direct":true,"mention":true,"thread_reply":false},`+quiet+`}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	got := decode(t, call(e, http.MethodGet, "/api/me/push-preferences", ""))["kinds"].(map[string]any)
	assert.Equal(t, false, got["incoming_call"], "an older client did not turn it back on")
	assert.Equal(t, false, got["thread_reply"])
}
