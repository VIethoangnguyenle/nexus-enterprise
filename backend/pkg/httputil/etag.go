package httputil

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
)

// Versions is a set of counters that move whenever the data behind a response
// changes. A write path bumps the counters of what it changed, in the same
// place that announces the change to live clients; ConditionalGET reads them to
// decide whether a client's copy is still current.
//
// A Versions that cannot be reached must be treated as "unknown": callers get
// an error and answer in full, never a stale 304.
type Versions interface {
	// Get returns the current value of each key, in order. A key never bumped is 0.
	Get(ctx context.Context, keys ...string) ([]int64, error)
	// Bump advances every key.
	Bump(ctx context.Context, keys ...string) error
}

// VersionKey joins the parts of a counter's name. Every scope that must not
// share a counter (tenant, workspace, user) belongs in the name.
func VersionKey(parts ...string) string { return "ver:" + strings.Join(parts, ":") }

// DefaultETagLifetime is how long an ETag stays valid at most. It is the safety
// net under the bumps: a write path that forgot to bump costs a stale screen for
// this long, never forever.
const DefaultETagLifetime = 5 * time.Minute

// ETagConfig configures ConditionalGET.
type ETagConfig struct {
	// Versions holds the counters. Nil disables the middleware.
	Versions Versions
	// Keys names the counters the response depends on, from the verified
	// caller and the route. They must include everything the payload varies by:
	// tenant, workspace and, when the data is per user, the user. ok=false
	// (no claims, no scope) serves the request without an ETag.
	Keys func(c echo.Context) (keys []string, ok bool)
	// Lifetime caps how long one ETag stays valid; zero is DefaultETagLifetime.
	Lifetime time.Duration
	// Now is the clock; nil is time.Now.
	Now func() time.Time
}

// ConditionalGET answers 304 Not Modified, without running the handler, when
// the client's If-None-Match names the current version of the data.
//
// The ETag is computed before the handler runs and sent with its 200: data read
// after a concurrent write is then labelled with the older version, so the next
// request refetches instead of trusting a copy that missed the write. It is
// derived from the counter names as well as their values, so two tenants or two
// users never produce the same tag. Any failure to read the counters, and any
// response but a 200, goes out with no ETag.
func ConditionalGET(cfg ETagConfig) echo.MiddlewareFunc {
	lifetime := cfg.Lifetime
	if lifetime <= 0 {
		lifetime = DefaultETagLifetime
	}
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if cfg.Versions == nil || cfg.Keys == nil || c.Request().Method != http.MethodGet {
				return next(c)
			}
			keys, ok := cfg.Keys(c)
			if !ok || len(keys) == 0 {
				return next(c)
			}
			vals, err := cfg.Versions.Get(c.Request().Context(), keys...)
			if err != nil || len(vals) != len(keys) {
				return next(c)
			}
			tag := etagOf(keys, vals, now().UnixNano()/int64(lifetime), c.Request().URL.RequestURI())

			res := c.Response()
			res.Header().Set(echo.HeaderCacheControl, "private, no-cache")
			if matches(c.Request().Header.Get("If-None-Match"), tag) {
				res.Header().Set("ETag", tag)
				return c.NoContent(http.StatusNotModified)
			}
			res.Header().Set("ETag", tag)
			res.Before(func() {
				if res.Status != http.StatusOK {
					res.Header().Del("ETag")
				}
			})
			return next(c)
		}
	}
}

// etagOf hashes the counters, the validity window and the request URI: the same
// data asked for at another path or with another filter is another resource, and
// a tag earned by one must never answer for the other.
func etagOf(keys []string, vals []int64, window int64, uri string) string {
	h := sha256.New()
	for i, k := range keys {
		h.Write([]byte(k))
		h.Write([]byte{0})
		h.Write([]byte(strconv.FormatInt(vals[i], 10)))
		h.Write([]byte{0})
	}
	h.Write([]byte(strconv.FormatInt(window, 10)))
	h.Write([]byte{0})
	h.Write([]byte(uri))
	return `"` + hex.EncodeToString(h.Sum(nil)[:16]) + `"`
}

// matches reports whether an If-None-Match header lists tag. Weak validators
// (W/"…") compare equal to strong ones for a GET; "*" is not honoured, since a
// client that sends it has no copy to keep.
func matches(header, tag string) bool {
	for _, part := range strings.Split(header, ",") {
		part = strings.TrimPrefix(strings.TrimSpace(part), "W/")
		if part == tag {
			return true
		}
	}
	return false
}
