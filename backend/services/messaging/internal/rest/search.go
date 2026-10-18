// Package rest — searching messages across conversations.
package rest

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"ngac-platform/pkg/httputil"
	pb "ngac-platform/proto/messaging"
	"ngac-platform/services/messaging/internal/domain"
)

// SearchAllMessages handles GET /api/messages/search?q=&limit=&before=&before_id=&has_file=
// &from=&channel_id=&after=&before_date=&tz=&has_link=&mentions_me=:
// the messages, main and thread replies, whose text matches q, in the
// conversations the caller reads in the workspace they are signed in to,
// newest first. A q shorter than two characters answers an empty list, unless
// at least one filter (from, channel_id, after, before_date, has_file,
// has_link, mentions_me) is set. Filters combine with AND.
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
		Text:       c.QueryParam("q"),
		HasFile:    c.QueryParam("has_file") == "true",
		Before:     c.QueryParam("before"),
		BeforeID:   c.QueryParam("before_id"),
		Limit:      limit,
		From:       c.QueryParam("from"),
		ChannelID:  c.QueryParam("channel_id"),
		After:      c.QueryParam("after"),
		BeforeDate: c.QueryParam("before_date"),
		TZ:         c.QueryParam("tz"),
		HasLink:    c.QueryParam("has_link") == "true",
		MentionsMe: c.QueryParam("mentions_me") == "true",
	})
	if err != nil {
		return httputil.MapDomainError(err)
	}
	// An empty answer is an empty list, never an empty object: the client reads `messages` as an array.
	msgs := list.GetMessages()
	if msgs == nil {
		msgs = []*pb.Message{}
	}
	return c.JSON(http.StatusOK, struct {
		Messages []*pb.Message `json:"messages"`
		HasMore  bool          `json:"has_more"`
	}{msgs, list.GetHasMore()})
}
