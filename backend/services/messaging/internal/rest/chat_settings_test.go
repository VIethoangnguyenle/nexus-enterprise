package rest

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/ngac"
	"ngac-platform/pkg/httputil"
	"ngac-platform/services/messaging/internal/domain"
	"ngac-platform/services/messaging/internal/store"
)

// settingsWorld is a workspace whose "being in it" attribute exists, so the
// policy answer (allow or deny) decides membership.
type settingsWorld struct {
	c *chat
}

func newSettingsWorld(t *testing.T) *settingsWorld {
	t.Helper()
	c := newChat(t)
	ctx := context.Background()
	oa := "rest-docs-oa-" + uuid.NewString()
	_, err := c.pool.Exec(ctx, `INSERT INTO ngac_nodes (id, name, node_type) VALUES ($1, $1, $2)`, oa, ngac.TypeOA)
	require.NoError(t, err)
	_, err = c.pool.Exec(ctx, `UPDATE workspaces SET documents_oa_id = $2 WHERE id = $1`, c.ws, oa)
	require.NoError(t, err)
	t.Cleanup(func() {
		c.pool.Exec(ctx, `DELETE FROM chat_user_settings WHERE workspace_id = $1`, c.ws)
		c.pool.Exec(ctx, `UPDATE workspaces SET documents_oa_id = NULL WHERE id = $1`, c.ws)
		c.pool.Exec(ctx, `DELETE FROM ngac_nodes WHERE id = $1`, oa)
	})
	return &settingsWorld{c: c}
}

func (w *settingsWorld) echo(allow bool, tenant string) *echo.Echo {
	svc := domain.NewService(store.NewStore(w.c.pool), answerPolicy{allow: allow}, nil, namedUser{}, nil)
	h := NewHandler(svc, nil, nil)
	e := httputil.NewEcho("messaging")
	as := func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(ctx echo.Context) error {
			httputil.SetClaims(ctx, &httputil.Claims{UserID: w.c.user, NGACNodeID: w.c.node, TenantID: tenant})
			return next(ctx)
		}
	}
	e.GET("/api/workspaces/:id/chat-settings", h.GetChatSettings, as)
	e.PUT("/api/workspaces/:id/chat-settings", h.PutChatSettings, as)
	e.GET("/api/me/push-preferences", h.GetPushPreferences, as)
	e.PUT("/api/me/push-preferences", h.PutPushPreferences, as)
	return e
}

func TestChatSettingsRoutes_ForbiddenToNonMembers(t *testing.T) {
	w := newSettingsWorld(t)
	path := "/api/workspaces/" + w.c.ws + "/chat-settings"

	refused := w.echo(false, w.c.ws)
	assert.Equal(t, http.StatusForbidden, call(refused, http.MethodGet, path, "").Code, "no read on the workspace")
	assert.Equal(t, http.StatusForbidden, call(refused, http.MethodPut, path, `{"sort":"name"}`).Code)

	wrongTenant := w.echo(true, "another-workspace")
	assert.Equal(t, http.StatusForbidden, call(wrongTenant, http.MethodGet, path, "").Code, "the token's tenant is not this workspace")
	assert.Equal(t, http.StatusForbidden, call(wrongTenant, http.MethodPut, path, `{"sort":"name"}`).Code)

	var n int
	require.NoError(t, w.c.pool.QueryRow(context.Background(), `SELECT count(*) FROM chat_user_settings WHERE workspace_id = $1`, w.c.ws).Scan(&n))
	assert.Zero(t, n, "nothing written")
}

func TestChatSettingsRoutes_DefaultsRoundTripAndValidation(t *testing.T) {
	w := newSettingsWorld(t)
	e := w.echo(true, w.c.ws)
	path := "/api/workspaces/" + w.c.ws + "/chat-settings"

	rec := call(e, http.MethodGet, path, "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	got := decode(t, rec)
	assert.Equal(t, map[string]any{
		"enter_to_send": false, "link_preview": true, "open_at": "all", "sort": "activity", "double_tap": "👍", "default_level": nil,
	}, got)

	rec = call(e, http.MethodPut, path, `{"enter_to_send":true,"sort":"name","double_tap":"","default_level":"off"}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, map[string]any{
		"enter_to_send": true, "link_preview": true, "open_at": "all", "sort": "name", "double_tap": "", "default_level": "off",
	}, decode(t, rec))

	rec = call(e, http.MethodPut, path, `{"default_level":null}`)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Nil(t, decode(t, rec)["default_level"], "null clears it")
	assert.Equal(t, "name", decode(t, call(e, http.MethodGet, path, ""))["sort"], "a field left out stays")

	for _, bad := range []string{
		`{"open_at":"x"}`, `{"sort":"x"}`, `{"default_level":"loud"}`, `{"double_tap":"hello"}`,
	} {
		assert.Equal(t, http.StatusBadRequest, call(e, http.MethodPut, path, bad).Code, bad)
	}
	assert.Equal(t, http.StatusBadRequest, call(e, http.MethodPut, path, `{"sort":`).Code, "malformed body")
	assert.Equal(t, "name", decode(t, call(e, http.MethodGet, path, ""))["sort"], "refused writes change nothing")
}

func TestPushPreferencesRoutes_PauseUntilIsSetClearedAndKept(t *testing.T) {
	w := newSettingsWorld(t)
	t.Cleanup(func() {
		w.c.pool.Exec(context.Background(), `DELETE FROM push_preferences WHERE user_id = $1`, w.c.user)
	})
	e := w.echo(true, w.c.ws)
	const path = "/api/me/push-preferences"
	base := `"kinds":{"direct":true,"mention":true,"thread_reply":true},"quiet_hours":{"enabled":false,"start":"22:00","end":"07:00","time_zone":"Asia/Ho_Chi_Minh"}`

	until := time.Now().Add(time.Hour).UTC().Truncate(time.Second)
	rec := call(e, http.MethodPut, path, `{`+base+`,"pause_until":"`+until.Format(time.RFC3339)+`"}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, until.Format(time.RFC3339), decode(t, rec)["pause_until"])
	assert.Equal(t, until.Format(time.RFC3339), decode(t, call(e, http.MethodGet, path, ""))["pause_until"])

	// A client that does not know the field leaves the pause alone.
	rec = call(e, http.MethodPut, path, `{`+base+`}`)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, until.Format(time.RFC3339), decode(t, call(e, http.MethodGet, path, ""))["pause_until"], "absent keeps it")

	rec = call(e, http.MethodPut, path, `{`+base+`,"pause_until":null}`)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Nil(t, decode(t, call(e, http.MethodGet, path, ""))["pause_until"], "null resumes")

	assert.Equal(t, http.StatusBadRequest, call(e, http.MethodPut, path, `{`+base+`,"pause_until":"tomorrow"}`).Code)
}
