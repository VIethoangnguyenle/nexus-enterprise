package domain_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/services/auth/internal/domain"
)

func sessionStore(t *testing.T) *domain.RefreshStore {
	t.Helper()
	rs := domain.NewRefreshStore(testRedisClient(t))
	rs.SetGraceForTest(0)
	return rs
}

// issue starts a session for user on device and ends it when the test does.
func issue(t *testing.T, rs *domain.RefreshStore, user, device string) (token, sid string) {
	t.Helper()
	token, sid, err := rs.Issue(context.Background(), domain.RefreshIdentity{UserID: user, Username: user, Device: device})
	require.NoError(t, err)
	t.Cleanup(func() { _ = rs.RevokeSession(context.Background(), sid) })
	return token, sid
}

func sessionIDs(list []domain.SessionInfo) []string {
	out := make([]string, 0, len(list))
	for _, s := range list {
		out = append(out, s.SessionID)
	}
	return out
}

func TestListUserSessions_NeverReturnsAnotherUsersSessions(t *testing.T) {
	rs := sessionStore(t)
	alice, bob := "alice-"+uuid.NewString(), "bob-"+uuid.NewString()
	_, mine := issue(t, rs, alice, "Safari · iPhone")
	_, theirs := issue(t, rs, bob, "Chrome · Windows")

	list, err := rs.ListUserSessions(context.Background(), alice)
	require.NoError(t, err)
	assert.Equal(t, []string{mine}, sessionIDs(list))
	assert.NotContains(t, sessionIDs(list), theirs)
}

func TestListUserSessions_UnknownUserHasNone(t *testing.T) {
	rs := sessionStore(t)
	list, err := rs.ListUserSessions(context.Background(), "nobody-"+uuid.NewString())
	require.NoError(t, err)
	assert.Empty(t, list)
}

func TestListUserSessions_KeepsDeviceAndStartAcrossRotation(t *testing.T) {
	rs := sessionStore(t)
	user := "rot-" + uuid.NewString()
	token, sid := issue(t, rs, user, "Safari · iPhone")
	before, err := rs.ListUserSessions(context.Background(), user)
	require.NoError(t, err)
	require.Len(t, before, 1)

	_, _, err = rs.Rotate(context.Background(), token)
	require.NoError(t, err)

	after, err := rs.ListUserSessions(context.Background(), user)
	require.NoError(t, err)
	require.Len(t, after, 1)
	assert.Equal(t, sid, after[0].SessionID)
	assert.Equal(t, "Safari · iPhone", after[0].Device)
	assert.True(t, before[0].StartedAt.Equal(after[0].StartedAt), "the start of a session survives rotation")
	assert.False(t, after[0].StartedAt.IsZero())
}

func TestListUserSessions_RevokedSessionDisappears(t *testing.T) {
	rs := sessionStore(t)
	user := "rev-" + uuid.NewString()
	_, gone := issue(t, rs, user, "a")
	_, kept := issue(t, rs, user, "b")

	require.NoError(t, rs.RevokeSession(context.Background(), gone))

	list, err := rs.ListUserSessions(context.Background(), user)
	require.NoError(t, err)
	assert.Equal(t, []string{kept}, sessionIDs(list))
}

func TestListUserSessions_RevokeAllEndsEveryListedSession(t *testing.T) {
	rs := sessionStore(t)
	user := "all-" + uuid.NewString()
	issue(t, rs, user, "a")
	issue(t, rs, user, "b")

	require.NoError(t, rs.RevokeAllForUser(context.Background(), user))

	list, err := rs.ListUserSessions(context.Background(), user)
	require.NoError(t, err)
	assert.Empty(t, list)
}

func TestListUserSessions_RevokeAllExceptKeepsTheSparedSession(t *testing.T) {
	rs := sessionStore(t)
	user := "keep-" + uuid.NewString()
	issue(t, rs, user, "a")
	_, spared := issue(t, rs, user, "b")

	require.NoError(t, rs.RevokeAllForUserExcept(context.Background(), user, spared))

	list, err := rs.ListUserSessions(context.Background(), user)
	require.NoError(t, err)
	assert.Equal(t, []string{spared}, sessionIDs(list))
}

func TestDeviceLabel(t *testing.T) {
	cases := map[string]string{
		"Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.0 Mobile/15E148 Safari/604.1": "Safari · iPhone",
		"Mozilla/5.0 (iPad; CPU OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1":           "Safari · iPad",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36":                         "Chrome · Windows",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36 Edg/130.0.0.0":           "Edge · Windows",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.0 Safari/605.1.15":                  "Safari · macOS",
		"Mozilla/5.0 (X11; Linux x86_64; rv:131.0) Gecko/20100101 Firefox/131.0":                                                                  "Firefox · Linux",
		"Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Mobile Safari/537.36":                   "Chrome · Android",
		"Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) CriOS/130.0 Mobile/15E148 Safari/604.1":   "Chrome · iPhone",
		"curl/8.5.0": "",
		"":           "",
	}
	for ua, want := range cases {
		assert.Equal(t, want, domain.DeviceLabel(ua), ua)
	}
}
