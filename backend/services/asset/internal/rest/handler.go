// Package rest provides Echo REST handlers for the asset service.
// Delegates to gRPC servers (AssetServer, AssetTypeServer, AssetRequestServer).
package rest

import (
	"context"
	"net/http"

	"github.com/labstack/echo/v4"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"ngac-platform/pkg/httputil"
	pb "ngac-platform/proto/asset"
)

// AssetService defines the operations the REST handler needs for assets.
type AssetService interface {
	CreateAsset(ctx context.Context, req *pb.CreateAssetRequest) (*pb.Asset, error)
	GetAsset(ctx context.Context, req *pb.GetAssetRequest) (*pb.Asset, error)
	ListAssets(ctx context.Context, req *pb.ListAssetsRequest) (*pb.AssetList, error)
	UpdateAsset(ctx context.Context, req *pb.UpdateAssetRequest) (*pb.Asset, error)
	DeleteAsset(ctx context.Context, req *pb.DeleteAssetRequest) (*pb.Empty, error)
	TransitionAsset(ctx context.Context, req *pb.TransitionRequest) (*pb.Asset, error)
	GetAvailableTransitions(ctx context.Context, req *pb.GetTransitionsRequest) (*pb.TransitionList, error)
	GetAssetHistory(ctx context.Context, req *pb.GetHistoryRequest) (*pb.TransitionHistoryList, error)
}

// AssetTypeService defines asset type operations.
type AssetTypeService interface {
	CreateType(ctx context.Context, req *pb.CreateTypeRequest) (*pb.AssetType, error)
	GetType(ctx context.Context, req *pb.GetTypeRequest) (*pb.AssetType, error)
	ListTypes(ctx context.Context, req *pb.ListTypesRequest) (*pb.AssetTypeList, error)
	UpdateTypeSchema(ctx context.Context, req *pb.UpdateTypeSchemaRequest) (*pb.AssetType, error)
}

// AssetRequestService defines asset request operations.
type AssetRequestService interface {
	CreateRequest(ctx context.Context, req *pb.CreateAssetRequestReq) (*pb.AssetRequest, error)
	ApproveRequest(ctx context.Context, req *pb.ApproveRequestReq) (*pb.AssetRequest, error)
	RejectRequest(ctx context.Context, req *pb.RejectRequestReq) (*pb.AssetRequest, error)
}

// Handler serves asset REST endpoints.
type Handler struct {
	assetSvc   AssetService
	typeSvc    AssetTypeService
	requestSvc AssetRequestService
}

// NewHandler creates an asset REST handler.
func NewHandler(a AssetService, t AssetTypeService, r AssetRequestService) *Handler {
	return &Handler{assetSvc: a, typeSvc: t, requestSvc: r}
}

// RegisterRoutes mounts asset endpoints on the Echo instance.
func (h *Handler) RegisterRoutes(e *echo.Echo, jwtSecret string) {
	api := e.Group("/api", httputil.JWTMiddleware(jwtSecret))

	// Asset Types
	api.POST("/workspaces/:id/asset-types", h.CreateAssetType)
	api.GET("/workspaces/:id/asset-types", h.ListAssetTypes)
	api.GET("/asset-types/:typeId", h.GetAssetType)
	api.PUT("/asset-types/:typeId/schema", h.UpdateAssetTypeSchema)

	// Assets
	api.GET("/workspaces/:id/assets/summary", h.GetAssetSummary)
	api.POST("/workspaces/:id/assets", h.CreateAsset)
	api.GET("/workspaces/:id/assets", h.ListAssets)
	api.GET("/assets/:assetId", h.GetAsset)
	api.PUT("/assets/:assetId", h.UpdateAsset)
	api.DELETE("/assets/:assetId", h.DeleteAsset)

	// Asset Lifecycle
	api.POST("/assets/:assetId/transition", h.TransitionAsset)
	api.GET("/assets/:assetId/transitions", h.GetAvailableTransitions)
	api.GET("/assets/:assetId/history", h.GetAssetHistory)

	// Asset Requests
	api.POST("/workspaces/:id/asset-requests", h.CreateAssetRequest)
	api.GET("/workspaces/:id/asset-requests", h.ListAssetRequests)
	api.GET("/asset-requests/:reqId", h.GetAssetRequest)
	api.POST("/asset-requests/:reqId/approve", h.ApproveAssetRequest)
	api.POST("/asset-requests/:reqId/reject", h.RejectAssetRequest)
	api.POST("/asset-requests/:reqId/assign", h.AssignAsset)
	api.POST("/assets/:assetId/return", h.ReturnAsset)
}

// --- Asset Summary ---

// GetAssetSummary returns aggregate counts for the asset dashboard.
func (h *Handler) GetAssetSummary(c echo.Context) error {
	resp, err := h.assetSvc.ListAssets(c.Request().Context(), &pb.ListAssetsRequest{
		WorkspaceId: c.Param("id"),
	})
	if err != nil {
		return mapGRPCError(err)
	}
	total := len(resp.GetAssets())
	var inUse, pending int
	for _, a := range resp.GetAssets() {
		switch a.GetState() {
		case "in_use", "assigned":
			inUse++
		case "pending", "requested":
			pending++
		}
	}
	return c.JSON(http.StatusOK, map[string]int{
		"total": total, "in_use": inUse, "pending": pending,
	})
}

// --- Asset Types ---

func (h *Handler) CreateAssetType(c echo.Context) error {
	claims := httputil.GetClaims(c)
	var body struct {
		Name     string `json:"name"`
		Category string `json:"category"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	resp, err := h.typeSvc.CreateType(c.Request().Context(), &pb.CreateTypeRequest{
		WorkspaceId:    c.Param("id"),
		Name:           body.Name,
		Category:       body.Category,
		UserNgacNodeId: claims.NGACNodeID,
	})
	if err != nil {
		return mapGRPCError(err)
	}
	return c.JSON(http.StatusCreated, resp)
}

func (h *Handler) ListAssetTypes(c echo.Context) error {
	resp, err := h.typeSvc.ListTypes(c.Request().Context(), &pb.ListTypesRequest{
		WorkspaceId: c.Param("id"),
	})
	if err != nil {
		return mapGRPCError(err)
	}
	return c.JSON(http.StatusOK, resp)
}

func (h *Handler) GetAssetType(c echo.Context) error {
	resp, err := h.typeSvc.GetType(c.Request().Context(), &pb.GetTypeRequest{
		TypeId: c.Param("typeId"),
	})
	if err != nil {
		return mapGRPCError(err)
	}
	return c.JSON(http.StatusOK, resp)
}

func (h *Handler) UpdateAssetTypeSchema(c echo.Context) error {
	var body struct {
		FieldsSchema string `json:"fields_schema"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	resp, err := h.typeSvc.UpdateTypeSchema(c.Request().Context(), &pb.UpdateTypeSchemaRequest{
		TypeId:       c.Param("typeId"),
		FieldsSchema: body.FieldsSchema,
	})
	if err != nil {
		return mapGRPCError(err)
	}
	return c.JSON(http.StatusOK, resp)
}

// --- Assets ---

func (h *Handler) CreateAsset(c echo.Context) error {
	claims := httputil.GetClaims(c)
	var body struct {
		TypeID string `json:"type_id"`
		Name   string `json:"name"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	resp, err := h.assetSvc.CreateAsset(c.Request().Context(), &pb.CreateAssetRequest{
		WorkspaceId:    c.Param("id"),
		TypeId:         body.TypeID,
		Name:           body.Name,
		UserNgacNodeId: claims.NGACNodeID,
	})
	if err != nil {
		return mapGRPCError(err)
	}
	return c.JSON(http.StatusCreated, resp)
}

func (h *Handler) ListAssets(c echo.Context) error {
	resp, err := h.assetSvc.ListAssets(c.Request().Context(), &pb.ListAssetsRequest{
		WorkspaceId: c.Param("id"),
		TypeId:      c.QueryParam("type_id"),
	})
	if err != nil {
		return mapGRPCError(err)
	}
	return c.JSON(http.StatusOK, resp)
}

func (h *Handler) GetAsset(c echo.Context) error {
	resp, err := h.assetSvc.GetAsset(c.Request().Context(), &pb.GetAssetRequest{
		AssetId: c.Param("assetId"),
	})
	if err != nil {
		return mapGRPCError(err)
	}
	return c.JSON(http.StatusOK, resp)
}

func (h *Handler) UpdateAsset(c echo.Context) error {
	claims := httputil.GetClaims(c)
	var body struct {
		Name string `json:"name"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	resp, err := h.assetSvc.UpdateAsset(c.Request().Context(), &pb.UpdateAssetRequest{
		AssetId:        c.Param("assetId"),
		Name:           body.Name,
		UserNgacNodeId: claims.NGACNodeID,
	})
	if err != nil {
		return mapGRPCError(err)
	}
	return c.JSON(http.StatusOK, resp)
}

func (h *Handler) DeleteAsset(c echo.Context) error {
	claims := httputil.GetClaims(c)
	_, err := h.assetSvc.DeleteAsset(c.Request().Context(), &pb.DeleteAssetRequest{
		AssetId:        c.Param("assetId"),
		UserNgacNodeId: claims.NGACNodeID,
	})
	if err != nil {
		return mapGRPCError(err)
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "deleted"})
}

// --- Asset Lifecycle ---

func (h *Handler) TransitionAsset(c echo.Context) error {
	claims := httputil.GetClaims(c)
	var body struct {
		ToState string `json:"to_state"`
		Comment string `json:"comment"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	resp, err := h.assetSvc.TransitionAsset(c.Request().Context(), &pb.TransitionRequest{
		AssetId:        c.Param("assetId"),
		Action:         body.ToState,
		Comment:        body.Comment,
		UserNgacNodeId: claims.NGACNodeID,
	})
	if err != nil {
		return mapGRPCError(err)
	}
	return c.JSON(http.StatusOK, resp)
}

func (h *Handler) GetAvailableTransitions(c echo.Context) error {
	resp, err := h.assetSvc.GetAvailableTransitions(c.Request().Context(), &pb.GetTransitionsRequest{
		AssetId: c.Param("assetId"),
	})
	if err != nil {
		return mapGRPCError(err)
	}
	return c.JSON(http.StatusOK, resp)
}

func (h *Handler) GetAssetHistory(c echo.Context) error {
	resp, err := h.assetSvc.GetAssetHistory(c.Request().Context(), &pb.GetHistoryRequest{
		AssetId: c.Param("assetId"),
	})
	if err != nil {
		return mapGRPCError(err)
	}
	return c.JSON(http.StatusOK, resp)
}

// --- Asset Requests ---

func (h *Handler) CreateAssetRequest(c echo.Context) error {
	claims := httputil.GetClaims(c)
	var body struct {
		TypeID   string `json:"type_id"`
		Reason   string `json:"reason"`
		Quantity int32  `json:"quantity"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	resp, err := h.requestSvc.CreateRequest(c.Request().Context(), &pb.CreateAssetRequestReq{
		WorkspaceId:    c.Param("id"),
		TypeId:         body.TypeID,
		Justification:  body.Reason,
		Quantity:       body.Quantity,
		UserNgacNodeId: claims.NGACNodeID,
	})
	if err != nil {
		return mapGRPCError(err)
	}
	return c.JSON(http.StatusCreated, resp)
}

func (h *Handler) ListAssetRequests(c echo.Context) error {
	// ListRequests is not in proto, return placeholder
	return c.JSON(http.StatusOK, map[string]any{"requests": []any{}})
}

func (h *Handler) GetAssetRequest(c echo.Context) error {
	// GetRequest is not in proto, return placeholder
	return c.JSON(http.StatusOK, map[string]string{"id": c.Param("reqId")})
}

func (h *Handler) ApproveAssetRequest(c echo.Context) error {
	claims := httputil.GetClaims(c)
	resp, err := h.requestSvc.ApproveRequest(c.Request().Context(), &pb.ApproveRequestReq{
		RequestId:      c.Param("reqId"),
		UserNgacNodeId: claims.NGACNodeID,
	})
	if err != nil {
		return mapGRPCError(err)
	}
	return c.JSON(http.StatusOK, resp)
}

func (h *Handler) RejectAssetRequest(c echo.Context) error {
	claims := httputil.GetClaims(c)
	var body struct {
		Reason string `json:"reason"`
	}
	c.Bind(&body)
	resp, err := h.requestSvc.RejectRequest(c.Request().Context(), &pb.RejectRequestReq{
		RequestId:      c.Param("reqId"),
		Reason:         body.Reason,
		UserNgacNodeId: claims.NGACNodeID,
	})
	if err != nil {
		return mapGRPCError(err)
	}
	return c.JSON(http.StatusOK, resp)
}

func (h *Handler) AssignAsset(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) ReturnAsset(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

func mapGRPCError(err error) *echo.HTTPError {
	st, ok := status.FromError(err)
	if !ok {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	switch st.Code() {
	case codes.NotFound:
		return echo.NewHTTPError(http.StatusNotFound, st.Message())
	case codes.PermissionDenied:
		return echo.NewHTTPError(http.StatusForbidden, st.Message())
	case codes.AlreadyExists:
		return echo.NewHTTPError(http.StatusConflict, st.Message())
	case codes.InvalidArgument:
		return echo.NewHTTPError(http.StatusBadRequest, st.Message())
	default:
		return echo.NewHTTPError(http.StatusInternalServerError, st.Message())
	}
}
