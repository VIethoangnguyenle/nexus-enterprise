package rest

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/ngac"
	"ngac-platform/pkg/httputil"
	"ngac-platform/services/messaging/internal/domain"
	"ngac-platform/services/messaging/internal/store"
	"ngac-platform/testutil"
)

// GET /api/dms lists the DMs of the workspace the session is signed in to, and a
// DM's own routes answer "not found" from any other workspace.
func TestDMRoutes_AreScopedToTheSessionsWorkspace(t *testing.T) {
	c := newChat(t) // the caller, and workspace A
	ctx := context.Background()
	wsB, _ := testutil.CreateWorkspace(t, c.pool, c.user)
	_, peerNode := testutil.CreateUser(t, c.pool)

	makeDM := func(ws string) string {
		time.Sleep(time.Microsecond)
		n := time.Now().UnixNano()
		id, oa := fmt.Sprintf("rest-dm-%d", n), fmt.Sprintf("rest-dm-oa-%d", n)
		_, err := c.pool.Exec(ctx, `INSERT INTO ngac_nodes (id, name, node_type) VALUES ($1, $1, $2)`, oa, ngac.TypeOA)
		require.NoError(t, err)
		_, err = c.pool.Exec(ctx, `INSERT INTO channels (id, name, channel_type, workspace_id, ngac_oa_id, created_at) VALUES ($1, 'dm', 'dm', NULLIF($2, ''), $3, now())`, id, ws, oa)
		require.NoError(t, err)
		for _, node := range []string{c.node, peerNode} {
			_, err = c.pool.Exec(ctx, `INSERT INTO channel_members (channel_id, ngac_node_id) VALUES ($1, $2)`, id, node)
			require.NoError(t, err)
		}
		t.Cleanup(func() {
			c.pool.Exec(ctx, `DELETE FROM channel_members WHERE channel_id = $1`, id)
			c.pool.Exec(ctx, `DELETE FROM channels WHERE id = $1`, id)
			c.pool.Exec(ctx, `DELETE FROM ngac_nodes WHERE id = $1`, oa)
		})
		return id
	}
	inA, inB, orphan := makeDM(c.ws), makeDM(wsB), makeDM("")

	serve := func(tenant, method, path string) *httptest.ResponseRecorder {
		svc := domain.NewService(store.NewStore(c.pool), answerPolicy{allow: true}, nil, namedUser{}, nil)
		h := NewHandler(svc, nil, nil)
		e := httputil.NewEcho("messaging")
		as := func(next echo.HandlerFunc) echo.HandlerFunc {
			return func(ctx echo.Context) error {
				httputil.SetClaims(ctx, &httputil.Claims{UserID: c.user, NGACNodeID: c.node, TenantID: tenant})
				return next(ctx)
			}
		}
		e.GET("/api/dms", h.ListDMs, as)
		e.GET("/api/channels/:chId/messages", h.GetMessages, as)
		e.GET("/api/channels/:chId", h.GetChannel, as)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, httptest.NewRequest(method, path, nil))
		return rec
	}
	listed := func(tenant string) []string {
		rec := serve(tenant, http.MethodGet, "/api/dms")
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		var body struct {
			Channels []struct {
				ID string `json:"id"`
			} `json:"channels"`
		}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		var ids []string
		for _, ch := range body.Channels {
			ids = append(ids, ch.ID)
		}
		return ids
	}

	assert.Equal(t, []string{inA}, listed(c.ws), "signed in to A: A's DM only")
	assert.Equal(t, []string{inB}, listed(wsB), "signed in to B: B's DM only")
	assert.NotContains(t, append(listed(c.ws), listed(wsB)...), orphan, "a DM with no workspace is in no list")

	assert.Equal(t, http.StatusOK, serve(c.ws, http.MethodGet, "/api/channels/"+inA+"/messages").Code, "allow: A's DM from A")
	assert.Equal(t, http.StatusNotFound, serve(wsB, http.MethodGet, "/api/channels/"+inA+"/messages").Code, "deny: A's DM from B")
	assert.Equal(t, http.StatusNotFound, serve(wsB, http.MethodGet, "/api/channels/"+inA).Code, "deny: A's DM opened from B")
	assert.Equal(t, http.StatusNotFound, serve(c.ws, http.MethodGet, "/api/channels/"+orphan+"/messages").Code, "deny: a DM with no workspace")
}
