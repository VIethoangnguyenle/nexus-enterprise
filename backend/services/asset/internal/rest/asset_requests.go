// Package rest provides Echo REST handlers for the asset service. They adapt
// HTTP to the asset domain services and hold no business logic.
package rest

import (
	"net/http"

	"github.com/labstack/echo/v4"

	pb "ngac-platform/proto/asset"
)

func (h *Handler) CreateAssetRequest(c echo.Context) error {
	// The requester is taken from the verified token only. The body has no
	// user field, and none may be added: the request is recorded as theirs and
	// ApproveRequest refuses to let a requester approve their own.
	var body struct {
		TypeID   string `json:"type_id"`
		Reason   string `json:"reason"`
		Urgency  string `json:"urgency"`
		Quantity int32  `json:"quantity"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	resp, err := h.requestSvc.CreateRequest(c.Request().Context(), &pb.CreateAssetRequestReq{
		WorkspaceId:   c.Param("id"),
		TypeId:        body.TypeID,
		Justification: body.Reason,
		Urgency:       body.Urgency,
		Quantity:      body.Quantity,
	})
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusCreated, resp)
}

// ListAssetRequests lists the requests the caller may see: their own, and those
// they may decide. status may name several, separated by commas.
func (h *Handler) ListAssetRequests(c echo.Context) error {
	limit, err := intQuery(c, "limit")
	if err != nil {
		return err
	}
	offset, err := intQuery(c, "offset")
	if err != nil {
		return err
	}
	resp, err := h.requestSvc.ListRequests(c.Request().Context(), &pb.ListRequestsReq{
		WorkspaceId: c.Param("id"),
		Status:      c.QueryParam("status"),
		MineOnly:    c.QueryParam("mine") == "true",
		Limit:       limit,
		Offset:      offset,
	})
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusOK, resp)
}

func (h *Handler) GetAssetRequest(c echo.Context) error {
	resp, err := h.requestSvc.GetRequest(c.Request().Context(), &pb.GetRequestReq{
		RequestId: c.Param("reqId"),
	})
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusOK, resp)
}

// ApproveAssetRequest approves a request as the authenticated caller. The
// user ID matters as much as the NGAC node: ApproveRequest compares it with
// the requester to refuse self-approval, and records it as the approver. Both
// come from the token only.
//
// With asset_id the request is approved and that asset handed to the requester
// in one step; without it the request is only approved.
func (h *Handler) ApproveAssetRequest(c echo.Context) error {
	var body struct {
		AssetID string `json:"asset_id"`
		Comment string `json:"comment"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	resp, err := h.requestSvc.ApproveRequest(c.Request().Context(), &pb.ApproveRequestReq{
		RequestId: c.Param("reqId"),
		AssetId:   body.AssetID,
		Comment:   body.Comment,
	})
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusOK, resp)
}

// RejectAssetRequest rejects a request as the authenticated caller, recorded
// as the approver. Identity comes from the token only.
func (h *Handler) RejectAssetRequest(c echo.Context) error {
	var body struct {
		Reason string `json:"reason"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	resp, err := h.requestSvc.RejectRequest(c.Request().Context(), &pb.RejectRequestReq{
		RequestId: c.Param("reqId"),
		Reason:    body.Reason,
	})
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusOK, resp)
}

// AssignAsset gives an asset to a request that was approved without one.
func (h *Handler) AssignAsset(c echo.Context) error {
	var body struct {
		AssetID string `json:"asset_id"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	resp, err := h.requestSvc.AssignAsset(c.Request().Context(), &pb.AssignAssetReq{
		RequestId: c.Param("reqId"),
		AssetId:   body.AssetID,
	})
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusOK, resp)
}

// ReturnAsset takes an assigned asset back into stock.
func (h *Handler) ReturnAsset(c echo.Context) error {
	if _, err := h.requestSvc.ReturnAsset(c.Request().Context(), &pb.ReturnAssetReq{
		AssetId: c.Param("assetId"),
	}); err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "returned"})
}
