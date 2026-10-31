package rest

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/services/auth/internal/auth"
	"ngac-platform/services/auth/internal/domain"
)

// signedSession is one signed-in device of a person: its refresh token and an
// access token minted for the same session.
type signedSession struct {
	id      string
	refresh string
	access  string
}

// signIn starts one more session for user id (a new member of ws-1 the first
// time) on the device named by device.
func (a *refreshApp) signIn(t *testing.T, id, device string) signedSession {
	t.Helper()
	u, ok := a.st.users[id]
	if !ok {
		u = a.st.add(id, "")
	}
	sid := uniq("sess-" + id)
	tok, err := a.svc.AttachRefreshToken(context.Background(), domain.RefreshIdentity{
		UserID: id, Username: u.Username, NGACNodeID: u.NGACNodeID, TenantID: "ws-1", SessionID: sid, Device: device,
	})
	require.NoError(t, err)
	access, err := auth.GenerateTokenForSession(id, u.Username, u.NGACNodeID, "ws-1", sid)
	require.NoError(t, err)
	t.Cleanup(func() { _ = a.svc.EndSession(context.Background(), sid) })
	return signedSession{id: sid, refresh: tok, access: access}
}

func (a *refreshApp) do(method, path, access string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	if access != "" {
		req.Header.Set("Authorization", "Bearer "+access)
	}
	rec := httptest.NewRecorder()
	a.e.ServeHTTP(rec, req)
	return rec
}

type sessionsBody struct {
	Sessions []struct {
		SessionID string `json:"session_id"`
		Device    string `json:"device"`
		StartedAt string `json:"started_at"`
		Current   bool   `json:"current"`
	} `json:"sessions"`
}

func (a *refreshApp) listSessions(t *testing.T, access string) sessionsBody {
	t.Helper()
	rec := a.do(http.MethodGet, "/api/auth/sessions", access)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var body sessionsBody
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	return body
}

func TestSessions_ListNeverShowsAnotherUsersSessions(t *testing.T) {
	a := newRefreshApp(t)
	alice, bob := uniq("alice"), uniq("bob")
	mine := a.signIn(t, alice, "Safari · iPhone")
	theirs := a.signIn(t, bob, "Chrome · Windows")
	a.signIn(t, bob, "Firefox · Linux")

	body := a.listSessions(t, mine.access)

	require.Len(t, body.Sessions, 1, "only the caller's own session")
	assert.Equal(t, mine.id, body.Sessions[0].SessionID)
	for _, s := range body.Sessions {
		assert.NotEqual(t, theirs.id, s.SessionID)
	}
}

func TestSessions_ListShowsEveryOwnDeviceAndMarksThisOne(t *testing.T) {
	a := newRefreshApp(t)
	alice := uniq("alice")
	phone := a.signIn(t, alice, "Safari · iPhone")
	laptop := a.signIn(t, alice, "Chrome · macOS")

	body := a.listSessions(t, laptop.access)

	require.Len(t, body.Sessions, 2)
	assert.Equal(t, laptop.id, body.Sessions[0].SessionID, "the session in use comes first")
	assert.True(t, body.Sessions[0].Current)
	assert.Equal(t, "Chrome · macOS", body.Sessions[0].Device)
	assert.Equal(t, phone.id, body.Sessions[1].SessionID)
	assert.False(t, body.Sessions[1].Current)
	assert.Equal(t, "Safari · iPhone", body.Sessions[1].Device)
	assert.NotEmpty(t, body.Sessions[1].StartedAt)
}

func TestSessions_ListNeedsSignIn(t *testing.T) {
	a := newRefreshApp(t)
	rec := a.do(http.MethodGet, "/api/auth/sessions", "")
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

// Deny: a session id that belongs to someone else is answered exactly like one
// that does not exist, and the other person's session keeps working.
func TestSessions_RevokingAnotherUsersSessionIsRefusedAndChangesNothing(t *testing.T) {
	a := newRefreshApp(t)
	alice, bob := uniq("alice"), uniq("bob")
	mine := a.signIn(t, alice, "Safari · iPhone")
	theirs := a.signIn(t, bob, "Chrome · Windows")

	rec := a.do(http.MethodDelete, "/api/auth/sessions/"+theirs.id, mine.access)
	assert.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())

	unknown := a.do(http.MethodDelete, "/api/auth/sessions/"+uniq("no-such-session"), mine.access)
	assert.Equal(t, http.StatusNotFound, unknown.Code)
	assert.JSONEq(t, unknown.Body.String(), rec.Body.String(), "someone else's session must look like no session")

	assert.Len(t, a.listSessions(t, theirs.access).Sessions, 1, "the other person still has their session")
	refreshed := a.refresh(theirs.refresh)
	assert.Equal(t, http.StatusOK, refreshed.Code, "the other person's session still refreshes")
}

func TestSessions_RevokingTheCurrentSessionIsRefused(t *testing.T) {
	a := newRefreshApp(t)
	mine := a.signIn(t, uniq("alice"), "Safari · iPhone")

	rec := a.do(http.MethodDelete, "/api/auth/sessions/"+mine.id, mine.access)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Len(t, a.listSessions(t, mine.access).Sessions, 1)
}

func TestSessions_RevokingAnotherOwnSessionEndsIt(t *testing.T) {
	a := newRefreshApp(t)
	alice := uniq("alice")
	phone := a.signIn(t, alice, "Safari · iPhone")
	laptop := a.signIn(t, alice, "Chrome · macOS")

	rec := a.do(http.MethodDelete, "/api/auth/sessions/"+phone.id, laptop.access)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	body := a.listSessions(t, laptop.access)
	require.Len(t, body.Sessions, 1)
	assert.Equal(t, laptop.id, body.Sessions[0].SessionID)
	assert.Equal(t, http.StatusUnauthorized, a.refresh(phone.refresh).Code, "the ended session can no longer refresh")
}

// A session survives refreshes for longer than any single token, and keeps its
// place in the list as it rotates.
func TestSessions_ARotatedSessionIsStillListed(t *testing.T) {
	a := newRefreshApp(t)
	phone := a.signIn(t, uniq("alice"), "Safari · iPhone")

	rec := a.refresh(phone.refresh)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	body := a.listSessions(t, phone.access)
	require.Len(t, body.Sessions, 1)
	assert.Equal(t, phone.id, body.Sessions[0].SessionID)
	assert.Equal(t, "Safari · iPhone", body.Sessions[0].Device)
}
