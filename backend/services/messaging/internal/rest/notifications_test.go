package rest

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/pkg/httputil"
	"ngac-platform/services/messaging/internal/domain"
	"ngac-platform/services/messaging/internal/store"
)

type fakeNotifs struct {
	rows       []*store.Notification
	err        error
	markedBy   string
	markedID   string
	markedWS   string
	markedAll  [2]string
	about      [4]string
	limit, off int
	listedWS   string
}

func (f *fakeNotifs) InsertNotification(context.Context, *store.Notification) error { return f.err }
func (f *fakeNotifs) ListNotifications(_ context.Context, _, ws string, limit, offset int) ([]*store.Notification, error) {
	f.limit, f.off, f.listedWS = limit, offset, ws
	return f.rows, f.err
}
func (f *fakeNotifs) NotificationCounts(context.Context, string, string) (int, int, error) {
	return len(f.rows), 1, f.err
}
func (f *fakeNotifs) MarkNotificationRead(_ context.Context, id, user, ws string) error {
	f.markedID, f.markedBy, f.markedWS = id, user, ws
	return f.err
}
func (f *fakeNotifs) MarkAllNotificationsRead(_ context.Context, user, ws string) error {
	f.markedAll = [2]string{user, ws}
	return f.err
}
func (f *fakeNotifs) FindMember(context.Context, string, string) (store.Member, bool, error) {
	return store.Member{}, false, f.err
}
func (f *fakeNotifs) InviteesOf(context.Context, string, time.Time) ([]store.Invitee, error) {
	return nil, f.err
}
func (f *fakeNotifs) DeleteNotificationsAbout(context.Context, string, string, string) error {
	return f.err
}
func (f *fakeNotifs) MarkNotificationsAboutRead(_ context.Context, user, ws, typ, id string) error {
	f.about = [4]string{user, ws, typ, id}
	return f.err
}

func notificationsEcho(f *fakeNotifs) *echo.Echo {
	e := httputil.NewEcho("messaging")
	h := NewHandler(nil, domain.NewNotificationService(f, nil), nil)
	signedIn := func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			httputil.SetClaims(c, &httputil.Claims{UserID: "alice", NGACNodeID: "u-alice", TenantID: "ws-1"})
			return next(c)
		}
	}
	e.GET("/api/notifications", h.ListNotifications, signedIn)
	e.POST("/api/notifications/:notifId/read", h.MarkRead, signedIn)
	e.POST("/api/notifications/read-all", h.MarkAllRead, signedIn)
	e.GET("/api/notifications/unread-count", h.UnreadCount, signedIn)
	e.POST("/api/notifications/read-about", h.MarkAboutRead, signedIn)
	return e
}

func do(e *echo.Echo, method, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(method, path, nil))
	return rec
}

func TestNotifications_ListAnswersTheShapeTheScreenReads(t *testing.T) {
	f := &fakeNotifs{rows: []*store.Notification{
		{ID: "n1", Type: "approval_rejected", WorkspaceID: "ws-1", ActorUserID: "u-boss", ActorName: "Vinh",
			TargetType: "approval", TargetID: "r1", TargetName: "Tạm ứng", Params: map[string]string{"reason": "no budget"},
			CreatedAt: time.Unix(1700000000, 0).UTC()},
	}}
	rec := do(notificationsEcho(f), http.MethodGet, "/api/notifications?limit=10&offset=2")

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var body struct {
		Notifications []map[string]any `json:"notifications"`
		Total         int              `json:"total"`
		Unread        int              `json:"unread_count"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Len(t, body.Notifications, 1)
	n := body.Notifications[0]
	assert.Equal(t, "n1", n["id"])
	assert.Equal(t, false, n["read"])
	assert.Equal(t, "approval", n["target_type"])
	assert.Equal(t, "Tạm ứng", n["target_name"])
	assert.Equal(t, "Vinh", n["actor_name"])
	assert.Equal(t, "u-boss", n["actor_user_id"])
	assert.Equal(t, map[string]any{"reason": "no budget"}, n["params"])
	_, hasTitle := n["title"]
	_, hasBody := n["body"]
	assert.False(t, hasTitle || hasBody, "pre-built text is not served: the screen words the notification")
	assert.Equal(t, 1, body.Total)
	assert.Equal(t, 1, body.Unread)
	assert.Equal(t, 10, f.limit)
	assert.Equal(t, 2, f.off)
	assert.Equal(t, "ws-1", f.listedWS, "the list is for the workspace in the caller's token")
}

func TestNotifications_ANonNumericPageIsRefused(t *testing.T) {
	for _, q := range []string{"limit=ten", "offset=x"} {
		f := &fakeNotifs{}
		rec := do(notificationsEcho(f), http.MethodGet, "/api/notifications?"+q)
		assert.Equal(t, http.StatusBadRequest, rec.Code, q)
		assert.Zero(t, f.limit, "the store is not asked")
	}
}

func TestNotifications_EmptyListIsAnArrayNotNull(t *testing.T) {
	rec := do(notificationsEcho(&fakeNotifs{}), http.MethodGet, "/api/notifications")
	assert.Contains(t, rec.Body.String(), `"notifications":[]`)
}

func TestNotifications_MarkingIsScopedToTheSignedInUserAndWorkspace(t *testing.T) {
	f := &fakeNotifs{}
	e := notificationsEcho(f)

	require.Equal(t, http.StatusOK, do(e, http.MethodPost, "/api/notifications/n9/read").Code)
	assert.Equal(t, "n9", f.markedID)
	assert.Equal(t, "alice", f.markedBy, "the user comes from the verified claims")
	assert.Equal(t, "ws-1", f.markedWS, "so does the workspace")

	require.Equal(t, http.StatusOK, do(e, http.MethodPost, "/api/notifications/read-all").Code)
	assert.Equal(t, [2]string{"alice", "ws-1"}, f.markedAll)

	rec := do(e, http.MethodGet, "/api/notifications/unread-count")
	assert.Contains(t, rec.Body.String(), `"count":1`)
}

func TestNotifications_MarkAboutReadIsScopedAndValidated(t *testing.T) {
	f := &fakeNotifs{}
	e := notificationsEcho(f)
	post := func(body string) int {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/notifications/read-about", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		e.ServeHTTP(rec, req)
		return rec.Code
	}
	require.Equal(t, http.StatusOK, post(`{"type":"workspace_invitation","id":"i1"}`))
	assert.Equal(t, [4]string{"alice", "ws-1", "workspace_invitation", "i1"}, f.about)
	assert.Equal(t, http.StatusBadRequest, post(`{"type":"workspace_invitation"}`), "an id is required")
}

// Without a workspace in the token nothing is listed, counted or marked.
func TestNotifications_NoWorkspaceInTheTokenIsRefused(t *testing.T) {
	f := &fakeNotifs{}
	e := httputil.NewEcho("messaging")
	h := NewHandler(nil, domain.NewNotificationService(f, nil), nil)
	noTenant := func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			httputil.SetClaims(c, &httputil.Claims{UserID: "alice", NGACNodeID: "u-alice"})
			return next(c)
		}
	}
	e.GET("/api/notifications", h.ListNotifications, noTenant)
	e.POST("/api/notifications/read-all", h.MarkAllRead, noTenant)
	assert.Equal(t, http.StatusForbidden, do(e, http.MethodGet, "/api/notifications").Code)
	assert.Equal(t, http.StatusForbidden, do(e, http.MethodPost, "/api/notifications/read-all").Code)
	assert.Zero(t, f.limit, "the store is not asked")
	assert.Empty(t, f.markedAll)
}

func TestNotifications_NoClaimsIs401(t *testing.T) {
	e := httputil.NewEcho("messaging")
	h := NewHandler(nil, domain.NewNotificationService(&fakeNotifs{}, nil), nil)
	e.GET("/api/notifications", h.ListNotifications)
	e.POST("/api/notifications/:notifId/read", h.MarkRead)
	assert.Equal(t, http.StatusUnauthorized, do(e, http.MethodGet, "/api/notifications").Code)
	assert.Equal(t, http.StatusUnauthorized, do(e, http.MethodPost, "/api/notifications/n1/read").Code)
}

// A store failure never shows its text: a generic message and a request ID.
func TestNotifications_InternalFailuresAreSanitised(t *testing.T) {
	f := &fakeNotifs{err: errors.New(`ERROR: relation "notifications" does not exist (SQLSTATE 42P01)`)}
	e := notificationsEcho(f)
	for _, path := range []string{"/api/notifications", "/api/notifications/unread-count"} {
		rec := do(e, http.MethodGet, path)
		require.Equal(t, http.StatusInternalServerError, rec.Code, path)
		body := rec.Body.String()
		assert.False(t, strings.Contains(body, "SQLSTATE") || strings.Contains(body, "relation"), "body leaks: %s", body)
		assert.Contains(t, body, `"request_id"`)
		assert.Contains(t, body, `"internal error"`)
	}
	assert.Equal(t, http.StatusInternalServerError, do(e, http.MethodPost, "/api/notifications/read-all").Code)
}
