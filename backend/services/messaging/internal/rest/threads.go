// Package rest — in-line thread REST handlers.
package rest

import (
	"context"
	"net/http"

	"github.com/labstack/echo/v4"

	"ngac-platform/pkg/httputil"
	pb "ngac-platform/proto/messaging"
)

// announceMessage tells the live sessions about a message that was just
// stored (fire-and-forget). A root message goes to the channel as it is; a
// reply goes to the channel as the thread's new summary, and to each follower
// as their own unread count.
func (h *Handler) announceMessage(ctx context.Context, tenantID string, msg *pb.Message) {
	h.svc.PushLater(ctx, msg)
	if h.hub == nil {
		return
	}
	if msg.ParentMessageId == "" {
		h.hub.BroadcastToChannel(msg.ChannelId, msg)
		return
	}
	h.svc.AnnounceReply(ctx, msg, tenantID, h.hub)
}

// threadStateAnswer sends the caller's new thread state to their other
// sessions and returns it as the response.
func (h *Handler) threadStateAnswer(c echo.Context, claims *httputil.Claims, st *pb.ThreadStateEvent, err error) error {
	if err != nil {
		return httputil.MapDomainError(err)
	}
	if h.hub != nil {
		h.hub.SendThreadStates(claims.TenantID, map[string]*pb.ThreadStateEvent{claims.NGACNodeID: st})
	}
	return c.JSON(http.StatusOK, st)
}

// FollowThread handles POST /api/messages/:msgId/follow.
func (h *Handler) FollowThread(c echo.Context) error {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	st, err := h.svc.FollowThread(c.Request().Context(), c.Param("msgId"), claims.UserID, claims.NGACNodeID)
	return h.threadStateAnswer(c, claims, st, err)
}

// UnfollowThread handles DELETE /api/messages/:msgId/follow.
func (h *Handler) UnfollowThread(c echo.Context) error {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	st, err := h.svc.UnfollowThread(c.Request().Context(), c.Param("msgId"), claims.UserID, claims.NGACNodeID)
	return h.threadStateAnswer(c, claims, st, err)
}

// MarkThreadRead handles POST /api/messages/:msgId/thread/read.
func (h *Handler) MarkThreadRead(c echo.Context) error {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	st, err := h.svc.MarkThreadRead(c.Request().Context(), c.Param("msgId"), claims.UserID, claims.NGACNodeID)
	return h.threadStateAnswer(c, claims, st, err)
}

// ListThreads handles GET /api/channels/:chId/threads?filter=all|following|unread&before=&before_id=&limit=.
func (h *Handler) ListThreads(c echo.Context) error {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	limit, err := intQuery(c, "limit")
	if err != nil {
		return err
	}
	list, err := h.svc.ListThreads(c.Request().Context(), c.Param("chId"), claims.UserID, claims.NGACNodeID,
		c.QueryParam("filter"), c.QueryParam("before"), c.QueryParam("before_id"), limit)
	if err != nil {
		return httputil.MapDomainError(err)
	}
	return c.JSON(http.StatusOK, list)
}

// UnreadThreads handles GET /api/threads/unread?limit=: the caller's followed
// threads with unread replies across the conversations they read in the
// workspace they are signed in to, newest reply first.
func (h *Handler) UnreadThreads(c echo.Context) error {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	limit, err := intQuery(c, "limit")
	if err != nil {
		return err
	}
	list, err := h.svc.UnreadThreads(c.Request().Context(), claims.UserID, claims.NGACNodeID, claims.TenantID, limit)
	if err != nil {
		return httputil.MapDomainError(err)
	}
	return c.JSON(http.StatusOK, list)
}
