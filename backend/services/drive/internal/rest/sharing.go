// Package rest provides Echo REST handlers for the drive service. They adapt
// HTTP to the domain service and hold no business logic.
package rest

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"ngac-platform/ngac"
	"ngac-platform/pkg/httputil"
	"ngac-platform/pkg/policyclient"
	pb "ngac-platform/proto/drive"
)

// CreateShare handles POST /api/drive/items/:itemId/share.
func (h *Handler) CreateShare(c echo.Context) error {
	_, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	var body struct {
		TargetNodeID string `json:"target_node_id"`
		ShareType    string `json:"share_type"`
		Permission   string `json:"permission"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	resp, err := h.svc.CreateShare(c.Request().Context(), &pb.CreateShareRequest{
		ItemId:           c.Param("itemId"),
		TargetNgacNodeId: body.TargetNodeID,
		ShareType:        body.ShareType,
		Operations:       []string{body.Permission},
	})
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusCreated, resp)
}

// RevokeShare handles DELETE /api/drive/shares/:shareId.
func (h *Handler) RevokeShare(c echo.Context) error {
	_, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	_, err = h.svc.RevokeShare(c.Request().Context(), &pb.RevokeShareRequest{
		ShareId: c.Param("shareId"),
	})
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "revoked"})
}

// ListShares handles GET /api/drive/items/:itemId/shares.
func (h *Handler) ListShares(c echo.Context) error {
	_, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	resp, err := h.svc.ListShares(c.Request().Context(), &pb.ListSharesRequest{
		ItemId: c.Param("itemId"),
	})
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusOK, resp)
}

// SharedWithMe handles GET /api/drive/shared-with-me.
func (h *Handler) SharedWithMe(c echo.Context) error {
	_, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	resp, err := h.svc.GetSharedWithMe(c.Request().Context(), &pb.GetSharedWithMeRequest{})
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusOK, resp)
}

// GetQuota handles GET /api/workspaces/:id/drive/quota.
func (h *Handler) GetQuota(c echo.Context) error {
	_, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	resp, err := h.svc.GetQuota(c.Request().Context(), &pb.GetQuotaRequest{
		WorkspaceId: c.Param("id"),
	})
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusOK, resp)
}

// BatchAccess handles POST /api/drive/batch-access.
// Resolves NGAC permissions for a batch of drive object IDs.
func (h *Handler) BatchAccess(c echo.Context) error {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}

	var body struct {
		ObjectIDs  []string `json:"object_ids"`
		Operations []string `json:"operations"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if len(body.ObjectIDs) == 0 {
		return c.JSON(http.StatusOK, map[string]any{"results": map[string]any{}})
	}
	if len(body.Operations) == 0 {
		body.Operations = []string{ngac.OpRead, ngac.OpWrite, ngac.OpShare}
	}

	results, err := policyclient.New(h.policyRead).BatchCheck(c.Request().Context(), claims.NGACNodeID, body.ObjectIDs, body.Operations)
	if err != nil {
		return httputil.MapGRPCError(err)
	}

	return c.JSON(http.StatusOK, map[string]any{"results": results})
}
