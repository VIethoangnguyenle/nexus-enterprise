package rest

import (
	"fmt"
	"log/slog"
	"net"
	"strings"

	"github.com/labstack/echo/v4"

	"ngac-platform/services/auth/internal/domain"
)

// PublicLimit is the allowance of one address for one public route.
type PublicLimit = domain.PublicLimit

// SetPublicLimit sets how often one address may call a public route (sign-in
// requests, code checks, refresh, logout, the start of Google sign-in). Zero Max
// switches the limit off. Call before RegisterRoutes.
func (h *Handler) SetPublicLimit(l PublicLimit) {
	h.limit = l
	h.limitSet = true
}

func (h *Handler) publicLimit() PublicLimit {
	if h.limitSet {
		return h.limit
	}
	return domain.DefaultPublicLimit
}

// publicLimiter is the middleware for one public route. It counts per address
// and per route, so a script hammering code checks cannot lock the same address
// out of refreshing its session. A counter that cannot be reached never blocks a
// person: the limit is a flood guard, and the sign-in paths have their own limits.
func (h *Handler) publicLimiter(route string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			err := h.svc.TakePublicRequest(c.Request().Context(), route, c.RealIP(), h.publicLimit())
			if err == nil {
				return next(c)
			}
			if _, limited := domain.RetryAfter(err); limited {
				return fail(c, err)
			}
			slog.Error("public rate limit unavailable; request allowed", "route", route, "error", err)
			return next(c)
		}
	}
}

// NewIPExtractor says whose address a request comes from.
//
// With no trusted proxies (the default) it is the connection's own address and
// X-Forwarded-For is ignored: a header any client can write must never choose
// which counter a request lands in. Behind a reverse proxy, list the proxy's
// networks (CIDR) and the client is read from X-Forwarded-For right to left,
// skipping only entries those networks added, so a forged leading entry does
// not count either.
func NewIPExtractor(trustedProxies []string) (echo.IPExtractor, error) {
	var ranges []echo.TrustOption
	for _, raw := range trustedProxies {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		_, n, err := net.ParseCIDR(raw)
		if err != nil {
			return nil, fmt.Errorf("trusted proxy %q is not a CIDR range: %w", raw, err)
		}
		ranges = append(ranges, echo.TrustIPRange(n))
	}
	if len(ranges) == 0 {
		return echo.ExtractIPDirect(), nil
	}
	// Nothing is trusted but what is listed: not loopback, not link-local, not
	// "any private address".
	opts := append([]echo.TrustOption{
		echo.TrustLoopback(false), echo.TrustLinkLocal(false), echo.TrustPrivateNet(false),
	}, ranges...)
	return echo.ExtractIPFromXFFHeader(opts...), nil
}
