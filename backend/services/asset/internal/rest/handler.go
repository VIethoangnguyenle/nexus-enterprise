// Package rest provides Echo REST handlers for the asset service. They adapt
// HTTP to the asset domain services and hold no business logic.
package rest

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
	"google.golang.org/protobuf/types/known/structpb"

	"ngac-platform/pkg/httputil"
	pb "ngac-platform/proto/asset"
	"ngac-platform/services/asset/internal/domain"
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
	HandOverAsset(ctx context.Context, req *pb.HandOverRequest) (*pb.Asset, error)
	GetSummary(ctx context.Context, req *pb.GetSummaryRequest) (*pb.AssetSummary, error)
	ListActivity(ctx context.Context, req *pb.ListActivityRequest) (*pb.ActivityList, error)
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
	AssignAsset(ctx context.Context, req *pb.AssignAssetReq) (*pb.AssetRequest, error)
	ReturnAsset(ctx context.Context, req *pb.ReturnAssetReq) (*pb.Empty, error)
	ListRequests(ctx context.Context, req *pb.ListRequestsReq) (*pb.AssetRequestList, error)
	GetRequest(ctx context.Context, req *pb.GetRequestReq) (*pb.AssetRequest, error)
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
	api.GET("/workspaces/:id/assets/activity", h.ListAssetActivity)
	api.POST("/workspaces/:id/assets", h.CreateAsset)
	api.GET("/workspaces/:id/assets", h.ListAssets)
	api.GET("/assets/:assetId", h.GetAsset)
	api.PUT("/assets/:assetId", h.UpdateAsset)
	api.DELETE("/assets/:assetId", h.DeleteAsset)

	// Asset Lifecycle
	api.POST("/assets/:assetId/transition", h.TransitionAsset)
	api.POST("/assets/:assetId/assign", h.HandOverAsset)
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

// GetAssetSummary returns the dashboard's counts over the assets the caller may read.
func (h *Handler) GetAssetSummary(c echo.Context) error {
	resp, err := h.assetSvc.GetSummary(c.Request().Context(), &pb.GetSummaryRequest{
		WorkspaceId: c.Param("id"),
	})
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusOK, resp)
}

// ListAssetActivity returns the newest lifecycle steps on assets the caller may
// read, with the people involved named.
func (h *Handler) ListAssetActivity(c echo.Context) error {
	limit, err := intQuery(c, "limit")
	if err != nil {
		return err
	}
	resp, err := h.assetSvc.ListActivity(c.Request().Context(), &pb.ListActivityRequest{
		WorkspaceId: c.Param("id"),
		Limit:       limit,
	})
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusOK, resp)
}

// intQuery reads an optional integer query parameter. A value that is not a
// number is the client's mistake and is refused, not read as zero.
func intQuery(c echo.Context, name string) (int32, error) {
	raw := c.QueryParam(name)
	if raw == "" {
		return 0, nil
	}
	n, err := strconv.ParseInt(raw, 10, 32)
	if err != nil || n < 0 {
		return 0, echo.NewHTTPError(http.StatusBadRequest, name+" must be a non-negative number")
	}
	return int32(n), nil
}

// customFields turns the JSON object a client sent into the proto struct.
func customFields(raw map[string]any) (*structpb.Struct, error) {
	if raw == nil {
		return nil, nil
	}
	st, err := structpb.NewStruct(raw)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "custom_fields is not valid")
	}
	return st, nil
}

// --- Asset Types ---

func (h *Handler) CreateAssetType(c echo.Context) error {
	var body struct {
		Name         string `json:"name"`
		Category     string `json:"category"`
		Description  string `json:"description"`
		FieldsSchema string `json:"fields_schema"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	resp, err := h.typeSvc.CreateType(c.Request().Context(), &pb.CreateTypeRequest{
		WorkspaceId:  c.Param("id"),
		Name:         body.Name,
		Category:     body.Category,
		Description:  body.Description,
		FieldsSchema: body.FieldsSchema,
	})
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusCreated, resp)
}

func (h *Handler) ListAssetTypes(c echo.Context) error {
	resp, err := h.typeSvc.ListTypes(c.Request().Context(), &pb.ListTypesRequest{
		WorkspaceId: c.Param("id"),
	})
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusOK, resp)
}

func (h *Handler) GetAssetType(c echo.Context) error {
	resp, err := h.typeSvc.GetType(c.Request().Context(), &pb.GetTypeRequest{
		TypeId: c.Param("typeId"),
	})
	if err != nil {
		return mapError(err)
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
		return mapError(err)
	}
	return c.JSON(http.StatusOK, resp)
}

// --- Assets ---

func (h *Handler) CreateAsset(c echo.Context) error {
	var body struct {
		TypeID       string         `json:"type_id"`
		Name         string         `json:"name"`
		CustomFields map[string]any `json:"custom_fields"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	fields, err := customFields(body.CustomFields)
	if err != nil {
		return err
	}
	resp, err := h.assetSvc.CreateAsset(c.Request().Context(), &pb.CreateAssetRequest{
		WorkspaceId:  c.Param("id"),
		TypeId:       body.TypeID,
		Name:         body.Name,
		CustomFields: fields,
	})
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusCreated, resp)
}

func (h *Handler) ListAssets(c echo.Context) error {
	limit, err := intQuery(c, "limit")
	if err != nil {
		return err
	}
	offset, err := intQuery(c, "offset")
	if err != nil {
		return err
	}
	resp, err := h.assetSvc.ListAssets(c.Request().Context(), &pb.ListAssetsRequest{
		WorkspaceId: c.Param("id"),
		TypeId:      c.QueryParam("type_id"),
		State:       c.QueryParam("state"),
		AssignedTo:  c.QueryParam("assigned_to"),
		Search:      c.QueryParam("search"),
		Limit:       limit,
		Offset:      offset,
	})
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusOK, resp)
}

func (h *Handler) GetAsset(c echo.Context) error {
	resp, err := h.assetSvc.GetAsset(c.Request().Context(), &pb.GetAssetRequest{
		AssetId: c.Param("assetId"),
	})
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusOK, resp)
}

func (h *Handler) UpdateAsset(c echo.Context) error {
	var body struct {
		Name         string         `json:"name"`
		CustomFields map[string]any `json:"custom_fields"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	fields, err := customFields(body.CustomFields)
	if err != nil {
		return err
	}
	resp, err := h.assetSvc.UpdateAsset(c.Request().Context(), &pb.UpdateAssetRequest{
		AssetId:      c.Param("assetId"),
		Name:         body.Name,
		CustomFields: fields,
	})
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusOK, resp)
}

func (h *Handler) DeleteAsset(c echo.Context) error {
	_, err := h.assetSvc.DeleteAsset(c.Request().Context(), &pb.DeleteAssetRequest{
		AssetId: c.Param("assetId"),
	})
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "deleted"})
}

// --- Asset Lifecycle ---

func (h *Handler) TransitionAsset(c echo.Context) error {
	// The actor recorded in the asset's history is taken from the token only.
	var body struct {
		Action  string `json:"action"`
		ToState string `json:"to_state"` // the older spelling of action
		Comment string `json:"comment"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	action := body.Action
	if action == "" {
		action = body.ToState
	}
	resp, err := h.assetSvc.TransitionAsset(c.Request().Context(), &pb.TransitionRequest{
		AssetId: c.Param("assetId"),
		Action:  action,
		Comment: body.Comment,
	})
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusOK, resp)
}

// HandOverAsset gives an available or assigned asset to a person. The person is
// named by user ID, picked from the workspace's people; who is acting comes
// from the token only.
func (h *Handler) HandOverAsset(c echo.Context) error {
	var body struct {
		AssigneeID string `json:"assignee_id"`
		Comment    string `json:"comment"`
	}
	if err := c.Bind(&body); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	resp, err := h.assetSvc.HandOverAsset(c.Request().Context(), &pb.HandOverRequest{
		AssetId:    c.Param("assetId"),
		AssigneeId: body.AssigneeID,
		Comment:    body.Comment,
	})
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusOK, resp)
}

func (h *Handler) GetAvailableTransitions(c echo.Context) error {
	resp, err := h.assetSvc.GetAvailableTransitions(c.Request().Context(), &pb.GetTransitionsRequest{
		AssetId: c.Param("assetId"),
	})
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusOK, resp)
}

func (h *Handler) GetAssetHistory(c echo.Context) error {
	resp, err := h.assetSvc.GetAssetHistory(c.Request().Context(), &pb.GetHistoryRequest{
		AssetId: c.Param("assetId"),
	})
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusOK, resp)
}

// --- Asset Requests ---

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

// mapError answers a domain error with its HTTP status. A refusal that carries a
// reason (to tell apart refusals that share a status) answers
// {"message", "reason"}. Anything the domain did not classify is a failure of
// ours and becomes a generic 500 (httputil.Internal).
func mapError(err error) *echo.HTTPError {
	var code int
	switch {
	case errors.Is(err, domain.ErrNotFound):
		code = http.StatusNotFound
	case errors.Is(err, domain.ErrAccessDenied):
		code = http.StatusForbidden
	case errors.Is(err, domain.ErrAlreadyExists), errors.Is(err, domain.ErrConflict):
		code = http.StatusConflict
	case errors.Is(err, domain.ErrInvalidInput):
		code = http.StatusBadRequest
	case errors.Is(err, domain.ErrUnauthenticated):
		code = http.StatusUnauthorized
	default:
		return httputil.Internal(err)
	}
	if reason := domain.Reason(err); reason != "" {
		return echo.NewHTTPError(code, map[string]any{"message": err.Error(), "reason": reason})
	}
	return echo.NewHTTPError(code, err.Error())
}
