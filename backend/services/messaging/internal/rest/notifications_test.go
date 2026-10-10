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
	markedAll  string
	limit, off int
}

func (f *fakeNotifs) InsertNotification(context.Context, *store.Notification) error { return f.err }
func (f *fakeNotifs) ListNotifications(_ context.Context, _ string, limit, offset int) ([]*store.Notification, error) {
	f.limit, f.off = limit, offset
	return f.rows, f.err
}
func (f *fakeNotifs) NotificationCounts(context.Context, string) (int, int, error) {
	return len(f.rows), 1, f.err
}
func (f *fakeNotifs) MarkNotificationRead(_ context.Context, id, user string) error {
	f.markedID, f.markedBy = id, user
	return f.err
}
func (f *fakeNotifs) MarkAllNotificationsRead(_ context.Context, user string) error {
	f.markedAll = user
	return f.err
}

func notificationsEcho(f *fakeNotifs) *echo.Echo {
	e := httputil.NewEcho("messaging")
	h := NewHandler(nil, domain.NewNotificationService(f, nil), nil)
	signedIn := func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			httputil.SetClaims(c, &httputil.Claims{UserID: "alice", NGACNodeID: "u-alice"})
			return next(c)
		}
	}
	e.GET("/api/notifications", h.ListNotifications, signedIn)
	e.POST("/api/notifications/:notifId/read", h.MarkRead, signedIn)
	e.POST("/api/notifications/read-all", h.MarkAllRead, signedIn)
	e.GET("/api/notifications/unread-count", h.UnreadCount, signedIn)
	return e
}

func do(e *echo.Echo, method, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(method, path, nil))
	return rec
}

func TestNotifications_ListAnswersTheShapeTheScreenReads(t *testing.T) {
	f := &fakeNotifs{rows: []*store.Notification{
		{ID: "n1", Type: "mention", Title: "Hi", Body: "b", EntityType: "asset", EntityID: "a1", CreatedAt: time.Unix(1700000000, 0).UTC()},
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
	assert.Equal(t, "n1", body.Notifications[0]["id"])
	assert.Equal(t, false, body.Notifications[0]["read"])
	assert.Equal(t, "asset", body.Notifications[0]["entity_type"])
	assert.Equal(t, 1, body.Total)
	assert.Equal(t, 1, body.Unread)
	assert.Equal(t, 10, f.limit)
	assert.Equal(t, 2, f.off)
}

func TestNotifications_EmptyListIsAnArrayNotNull(t *testing.T) {
	rec := do(notificationsEcho(&fakeNotifs{}), http.MethodGet, "/api/notifications")
	assert.Contains(t, rec.Body.String(), `"notifications":[]`)
}

func TestNotifications_MarkingIsScopedToTheSignedInUser(t *testing.T) {
	f := &fakeNotifs{}
	e := notificationsEcho(f)

	require.Equal(t, http.StatusOK, do(e, http.MethodPost, "/api/notifications/n9/read").Code)
	assert.Equal(t, "n9", f.markedID)
	assert.Equal(t, "alice", f.markedBy, "the user comes from the verified claims")

	require.Equal(t, http.StatusOK, do(e, http.MethodPost, "/api/notifications/read-all").Code)
	assert.Equal(t, "alice", f.markedAll)

	rec := do(e, http.MethodGet, "/api/notifications/unread-count")
	assert.Contains(t, rec.Body.String(), `"count":1`)
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
