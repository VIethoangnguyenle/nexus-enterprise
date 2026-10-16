package rest

import (
	"log/slog"

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
	return h.limiterAt(route, h.publicLimit())
}

// avatarLimiter guards the public picture route. A page of the directory
// shows dozens of avatars at once, so the allowance is ten times the
// sign-in routes'; it stops a script walking addresses, not a busy page.
func (h *Handler) avatarLimiter() echo.MiddlewareFunc {
	lim := h.publicLimit()
	lim.Max *= 10
	return h.limiterAt("avatar", lim)
}

func (h *Handler) limiterAt(route string, lim PublicLimit) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			err := h.svc.TakePublicRequest(c.Request().Context(), route, c.RealIP(), lim)
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
