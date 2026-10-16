package rest

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ngac-platform/pkg/httputil"
	"ngac-platform/services/messaging/internal/domain"
)

func avatarKeyFor(ip string) string {
	sum := sha256.Sum256([]byte(ip))
	return "channel_avatar_pub_rl:" + hex.EncodeToString(sum[:8])
}

func fetchFrom(e *echo.Echo, remote, xff string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/api/channels/avatars/"+strings.Repeat("0", 32), nil)
	req.RemoteAddr = remote
	if xff != "" {
		req.Header.Set(echo.HeaderXForwardedFor, xff)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func TestChannelAvatarRoutes_RateLimitCountsTheClientBehindATrustedProxy(t *testing.T) {
	rdb := testRedis(t)
	c := newChat(t)
	ctx := context.Background()
	exhausted, fresh, direct := "198.51.100.77", "198.51.100.78", "203.0.113.9"
	t.Cleanup(func() { rdb.Del(ctx, avatarKeyFor(exhausted), avatarKeyFor(fresh), avatarKeyFor(direct)) })
	require.NoError(t, rdb.Set(ctx, avatarKeyFor(exhausted), domain.AvatarPublicLimit, 0).Err())

	extract, err := httputil.NewIPExtractor([]string{"172.16.0.0/12"})
	require.NoError(t, err)
	e := c.avatarEcho(true, rdb)
	e.IPExtractor = extract

	t.Run("each forwarded client has its own bucket", func(t *testing.T) {
		assert.Equal(t, http.StatusTooManyRequests, fetchFrom(e, "172.18.0.4:5000", exhausted).Code)
		assert.NotEqual(t, http.StatusTooManyRequests, fetchFrom(e, "172.18.0.4:5000", fresh).Code,
			"one client's used-up allowance must not limit another behind the same proxy")
	})

	t.Run("a direct caller cannot choose its bucket with a header", func(t *testing.T) {
		require.NoError(t, rdb.Set(ctx, avatarKeyFor(direct), domain.AvatarPublicLimit, 0).Err())
		// Claiming to be a fresh client does not escape its own used-up bucket.
		assert.Equal(t, http.StatusTooManyRequests, fetchFrom(e, direct+":4000", fresh).Code)
	})
}
