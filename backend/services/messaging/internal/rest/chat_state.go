// Package rest — the chat navigator's state in one answer.
package rest

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"ngac-platform/pkg/httputil"
	"ngac-platform/services/messaging/internal/domain"
)

// orEmpty keeps a list that is empty an empty JSON array, not null.
func orEmpty[T any](list []T) []T {
	if list == nil {
		return []T{}
	}
	return list
}

// ChatState handles GET /api/workspaces/:id/chat-state: the spaces, direct
// messages, unread counts, previews, the caller's sections and the unread
// mention and thread counts, each as its own endpoint answers it. The answer
// carries an ETag; a client that sends it back in If-None-Match is told 304
// without anything being read.
func (h *Handler) ChatState(c echo.Context) error {
	who, err := callerOf(c)
	if err != nil {
		return err
	}
	st, err := h.svc.ChatState(c.Request().Context(), c.Param("id"), who)
	if err != nil {
		return httputil.MapDomainError(err)
	}
	sections := make([]sectionJSON, 0, len(st.Sections))
	for _, s := range st.Sections {
		sections = append(sections, toSectionJSON(s))
	}
	return c.JSON(http.StatusOK, map[string]any{
		"channels":             orEmpty(st.Channels),
		"dms":                  orEmpty(st.DMs),
		"unread":               orEmpty(st.Unread),
		"activity":             orEmpty(st.Activity),
		"sections":             sections,
		"mention_unread_count": st.UnreadMentions,
		"unread_thread_count":  len(st.UnreadThreads),
		"unread_threads":       orEmpty(st.UnreadThreads),
	})
}

// chatStateETag answers 304 to a chat state the client already holds. The
// counters are the caller's tenant and the caller; a request for a workspace
// that is not the caller's tenant carries no ETag, so it is always judged by the
// handler, never waved through.
func (h *Handler) chatStateETag() echo.MiddlewareFunc {
	return httputil.ConditionalGET(httputil.ETagConfig{
		Versions: h.svc.Versions(),
		Keys: func(c echo.Context) ([]string, bool) {
			claims := httputil.GetClaims(c)
			if claims == nil || claims.UserID == "" || claims.TenantID == "" || c.Param("id") != claims.TenantID {
				return nil, false
			}
			return domain.ChatVersionKeys(claims.TenantID, claims.UserID), true
		},
	})
}

// bumpChatOnWrite moves the chat state's counters after any request that
// changed something and succeeded: the caller's own state (read marks, stars,
// levels, sections, follows) and, for what everyone sees (messages, reactions,
// channels), the tenant's. Writes are never ETagged, so a request that only
// reads is untouched. It sits on the whole /api group, which is what makes
// "every write path bumps" true by construction rather than by memory.
func (h *Handler) bumpChatOnWrite() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			switch c.Request().Method {
			case http.MethodGet, http.MethodHead, http.MethodOptions:
				return next(c)
			}
			// The bump goes out just before the status line does, not after
			// the handler returns: the client's very next read must already
			// see the new version.
			res := c.Response()
			res.Before(func() {
				if res.Status >= http.StatusBadRequest {
					return
				}
				if claims := httputil.GetClaims(c); claims != nil {
					h.svc.BumpChat(claims.TenantID, claims.UserID)
				}
			})
			return next(c)
		}
	}
}
