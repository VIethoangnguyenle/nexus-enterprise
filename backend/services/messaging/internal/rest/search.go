// Package rest — searching messages across conversations.
package rest

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"ngac-platform/pkg/httputil"
	"ngac-platform/services/messaging/internal/domain"
)

// SearchAllMessages handles GET /api/messages/search?q=&limit=&before=&before_id=&has_file=:
// the messages, main and thread replies, whose text matches q, in the
// conversations the caller reads in the workspace they are signed in to,
// newest first. A q shorter than two characters answers an empty list.
func (h *Handler) SearchAllMessages(c echo.Context) error {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	limit, err := intQuery(c, "limit")
	if err != nil {
		return err
	}
	list, err := h.svc.SearchAll(c.Request().Context(), claims.UserID, claims.NGACNodeID, claims.TenantID, domain.SearchQuery{
		Text:     c.QueryParam("q"),
		HasFile:  c.QueryParam("has_file") == "true",
		Before:   c.QueryParam("before"),
		BeforeID: c.QueryParam("before_id"),
		Limit:    limit,
	})
	if err != nil {
		return httputil.MapDomainError(err)
	}
	return c.JSON(http.StatusOK, list)
}
