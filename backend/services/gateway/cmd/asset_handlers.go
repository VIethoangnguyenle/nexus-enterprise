package main

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"google.golang.org/protobuf/types/known/structpb"

	assetpb "ngac-platform/proto/asset"
	msgpb "ngac-platform/proto/messaging"
)

// ============================================
// Asset Type Handlers
// ============================================

func (gw *Gateway) handleCreateAssetType(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name         string `json:"name"`
		Description  string `json:"description"`
		Category     string `json:"category"`
		FieldsSchema string `json:"fields_schema"`
	}
	json.NewDecoder(r.Body).Decode(&body)
	resp, err := gw.assetTypeClient.CreateType(r.Context(), &assetpb.CreateTypeRequest{
		Name:         body.Name,
		Description:  body.Description,
		Category:     body.Category,
		WorkspaceId:  chi.URLParam(r, "id"),
		FieldsSchema: body.FieldsSchema,
	})
	writeResponse(w, resp, err)
}

func (gw *Gateway) handleListAssetTypes(w http.ResponseWriter, r *http.Request) {
	resp, err := gw.assetTypeClient.ListTypes(r.Context(), &assetpb.ListTypesRequest{
		WorkspaceId: chi.URLParam(r, "id"),
	})
	writeResponse(w, resp, err)
}

func (gw *Gateway) handleGetAssetType(w http.ResponseWriter, r *http.Request) {
	resp, err := gw.assetTypeClient.GetType(r.Context(), &assetpb.GetTypeRequest{
		TypeId: chi.URLParam(r, "typeId"),
	})
	writeResponse(w, resp, err)
}

func (gw *Gateway) handleUpdateAssetTypeSchema(w http.ResponseWriter, r *http.Request) {
	var body struct {
		FieldsSchema string `json:"fields_schema"`
	}
	json.NewDecoder(r.Body).Decode(&body)
	resp, err := gw.assetTypeClient.UpdateTypeSchema(r.Context(), &assetpb.UpdateTypeSchemaRequest{
		TypeId:       chi.URLParam(r, "typeId"),
		FieldsSchema: body.FieldsSchema,
	})
	writeResponse(w, resp, err)
}

// ============================================
// Asset Handlers
// ============================================

func (gw *Gateway) handleCreateAsset(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name         string                 `json:"name"`
		TypeID       string                 `json:"type_id"`
		CustomFields map[string]interface{} `json:"custom_fields"`
	}
	json.NewDecoder(r.Body).Decode(&body)

	var fields *structpb.Struct
	if body.CustomFields != nil {
		fields, _ = structpb.NewStruct(body.CustomFields)
	}

	resp, err := gw.assetClient.CreateAsset(r.Context(), &assetpb.CreateAssetRequest{
		Name:           body.Name,
		TypeId:         body.TypeID,
		WorkspaceId:    chi.URLParam(r, "id"),
		UserId:         r.Context().Value("user_id").(string),
		UserNgacNodeId: r.Context().Value("ngac_node_id").(string),
		CustomFields:   fields,
	})
	writeResponse(w, resp, err)
}

func (gw *Gateway) handleListAssets(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, _ := strconv.ParseInt(q.Get("limit"), 10, 32)
	offset, _ := strconv.ParseInt(q.Get("offset"), 10, 32)

	resp, err := gw.assetClient.ListAssets(r.Context(), &assetpb.ListAssetsRequest{
		WorkspaceId: chi.URLParam(r, "id"),
		TypeId:      q.Get("type_id"),
		State:       q.Get("state"),
		AssignedTo:  q.Get("assigned_to"),
		Limit:       int32(limit),
		Offset:      int32(offset),
	})
	writeResponse(w, resp, err)
}

func (gw *Gateway) handleGetAsset(w http.ResponseWriter, r *http.Request) {
	resp, err := gw.assetClient.GetAsset(r.Context(), &assetpb.GetAssetRequest{
		AssetId:        chi.URLParam(r, "assetId"),
		UserNgacNodeId: r.Context().Value("ngac_node_id").(string),
	})
	writeResponse(w, resp, err)
}

func (gw *Gateway) handleUpdateAsset(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name         string                 `json:"name"`
		CustomFields map[string]interface{} `json:"custom_fields"`
	}
	json.NewDecoder(r.Body).Decode(&body)

	var fields *structpb.Struct
	if body.CustomFields != nil {
		fields, _ = structpb.NewStruct(body.CustomFields)
	}

	resp, err := gw.assetClient.UpdateAsset(r.Context(), &assetpb.UpdateAssetRequest{
		AssetId:        chi.URLParam(r, "assetId"),
		Name:           body.Name,
		UserNgacNodeId: r.Context().Value("ngac_node_id").(string),
		CustomFields:   fields,
	})
	writeResponse(w, resp, err)
}

func (gw *Gateway) handleDeleteAsset(w http.ResponseWriter, r *http.Request) {
	resp, err := gw.assetClient.DeleteAsset(r.Context(), &assetpb.DeleteAssetRequest{
		AssetId:        chi.URLParam(r, "assetId"),
		UserNgacNodeId: r.Context().Value("ngac_node_id").(string),
	})
	writeResponse(w, resp, err)
}

// ============================================
// Asset Lifecycle Handlers
// ============================================

func (gw *Gateway) handleTransitionAsset(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Action  string `json:"action"`
		Comment string `json:"comment"`
	}
	json.NewDecoder(r.Body).Decode(&body)
	resp, err := gw.assetClient.TransitionAsset(r.Context(), &assetpb.TransitionRequest{
		AssetId:        chi.URLParam(r, "assetId"),
		Action:         body.Action,
		Comment:        body.Comment,
		UserId:         r.Context().Value("user_id").(string),
		UserNgacNodeId: r.Context().Value("ngac_node_id").(string),
	})
	writeResponse(w, resp, err)
}

func (gw *Gateway) handleGetAvailableTransitions(w http.ResponseWriter, r *http.Request) {
	resp, err := gw.assetClient.GetAvailableTransitions(r.Context(), &assetpb.GetTransitionsRequest{
		AssetId:        chi.URLParam(r, "assetId"),
		UserNgacNodeId: r.Context().Value("ngac_node_id").(string),
	})
	writeResponse(w, resp, err)
}

func (gw *Gateway) handleGetAssetHistory(w http.ResponseWriter, r *http.Request) {
	resp, err := gw.assetClient.GetAssetHistory(r.Context(), &assetpb.GetHistoryRequest{
		AssetId:        chi.URLParam(r, "assetId"),
		UserNgacNodeId: r.Context().Value("ngac_node_id").(string),
	})
	writeResponse(w, resp, err)
}

// ============================================
// Asset Request Handlers
// ============================================

func (gw *Gateway) handleCreateAssetRequest(w http.ResponseWriter, r *http.Request) {
	var body struct {
		TypeID        string `json:"type_id"`
		Justification string `json:"justification"`
		Quantity      int32  `json:"quantity"`
	}
	json.NewDecoder(r.Body).Decode(&body)
	resp, err := gw.assetRequestClient.CreateRequest(r.Context(), &assetpb.CreateAssetRequestReq{
		TypeId:         body.TypeID,
		WorkspaceId:    chi.URLParam(r, "id"),
		UserId:         r.Context().Value("user_id").(string),
		UserNgacNodeId: r.Context().Value("ngac_node_id").(string),
		Justification:  body.Justification,
		Quantity:       body.Quantity,
	})
	writeResponse(w, resp, err)
}

func (gw *Gateway) handleListAssetRequests(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, _ := strconv.ParseInt(q.Get("limit"), 10, 32)
	offset, _ := strconv.ParseInt(q.Get("offset"), 10, 32)
	mineOnly := q.Get("mine") == "true"

	resp, err := gw.assetRequestClient.ListRequests(r.Context(), &assetpb.ListRequestsReq{
		WorkspaceId: chi.URLParam(r, "id"),
		UserId:      r.Context().Value("user_id").(string),
		Status:      q.Get("status"),
		MineOnly:    mineOnly,
		Limit:       int32(limit),
		Offset:      int32(offset),
	})
	writeResponse(w, resp, err)
}

func (gw *Gateway) handleGetAssetRequest(w http.ResponseWriter, r *http.Request) {
	resp, err := gw.assetRequestClient.GetRequest(r.Context(), &assetpb.GetRequestReq{
		RequestId: chi.URLParam(r, "reqId"),
	})
	writeResponse(w, resp, err)
}

func (gw *Gateway) handleApproveAssetRequest(w http.ResponseWriter, r *http.Request) {
	var body struct{ Comment string `json:"comment"` }
	json.NewDecoder(r.Body).Decode(&body)
	resp, err := gw.assetRequestClient.ApproveRequest(r.Context(), &assetpb.ApproveRequestReq{
		RequestId:      chi.URLParam(r, "reqId"),
		UserId:         r.Context().Value("user_id").(string),
		UserNgacNodeId: r.Context().Value("ngac_node_id").(string),
		Comment:        body.Comment,
	})
	writeResponse(w, resp, err)
}

func (gw *Gateway) handleRejectAssetRequest(w http.ResponseWriter, r *http.Request) {
	var body struct{ Reason string `json:"reason"` }
	json.NewDecoder(r.Body).Decode(&body)
	resp, err := gw.assetRequestClient.RejectRequest(r.Context(), &assetpb.RejectRequestReq{
		RequestId:      chi.URLParam(r, "reqId"),
		UserId:         r.Context().Value("user_id").(string),
		UserNgacNodeId: r.Context().Value("ngac_node_id").(string),
		Reason:         body.Reason,
	})
	writeResponse(w, resp, err)
}

func (gw *Gateway) handleAssignAsset(w http.ResponseWriter, r *http.Request) {
	var body struct{ AssetID string `json:"asset_id"` }
	json.NewDecoder(r.Body).Decode(&body)
	resp, err := gw.assetRequestClient.AssignAsset(r.Context(), &assetpb.AssignAssetReq{
		RequestId:      chi.URLParam(r, "reqId"),
		AssetId:        body.AssetID,
		UserId:         r.Context().Value("user_id").(string),
		UserNgacNodeId: r.Context().Value("ngac_node_id").(string),
	})
	writeResponse(w, resp, err)
}

func (gw *Gateway) handleReturnAsset(w http.ResponseWriter, r *http.Request) {
	resp, err := gw.assetRequestClient.ReturnAsset(r.Context(), &assetpb.ReturnAssetReq{
		AssetId:        chi.URLParam(r, "assetId"),
		UserId:         r.Context().Value("user_id").(string),
		UserNgacNodeId: r.Context().Value("ngac_node_id").(string),
	})
	writeResponse(w, resp, err)
}

// ============================================
// Thread Handlers
// ============================================

func (gw *Gateway) handleGetThread(w http.ResponseWriter, r *http.Request) {
	resp, err := gw.messagingClient.GetThread(r.Context(), &msgpb.GetThreadRequest{
		MessageId: chi.URLParam(r, "msgId"),
	})
	writeResponse(w, resp, err)
}

func (gw *Gateway) handleFindThreadsByEntity(w http.ResponseWriter, r *http.Request) {
	resp, err := gw.messagingClient.FindThreadsByEntity(r.Context(), &msgpb.FindThreadsByEntityRequest{
		EntityType: chi.URLParam(r, "entityType"),
		EntityId:   chi.URLParam(r, "entityId"),
	})
	writeResponse(w, resp, err)
}

// ============================================
// Notification Handlers
// ============================================

func (gw *Gateway) handleListNotifications(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, _ := strconv.ParseInt(q.Get("limit"), 10, 32)
	offset, _ := strconv.ParseInt(q.Get("offset"), 10, 32)

	resp, err := gw.notificationClient.ListNotifications(r.Context(), &msgpb.ListNotificationsRequest{
		UserId: r.Context().Value("user_id").(string),
		Limit:  int32(limit),
		Offset: int32(offset),
	})
	writeResponse(w, resp, err)
}

func (gw *Gateway) handleMarkRead(w http.ResponseWriter, r *http.Request) {
	resp, err := gw.notificationClient.MarkRead(r.Context(), &msgpb.MarkReadRequest{
		NotificationId: chi.URLParam(r, "notifId"),
		UserId:         r.Context().Value("user_id").(string),
	})
	writeResponse(w, resp, err)
}

func (gw *Gateway) handleMarkAllRead(w http.ResponseWriter, r *http.Request) {
	resp, err := gw.notificationClient.MarkAllRead(r.Context(), &msgpb.MarkAllReadRequest{
		UserId: r.Context().Value("user_id").(string),
	})
	writeResponse(w, resp, err)
}

func (gw *Gateway) handleUnreadCount(w http.ResponseWriter, r *http.Request) {
	resp, err := gw.notificationClient.GetUnreadCount(r.Context(), &msgpb.GetUnreadCountRequest{
		UserId: r.Context().Value("user_id").(string),
	})
	writeResponse(w, resp, err)
}

// Suppress unused import warnings for new packages.
var _ = strconv.Itoa
