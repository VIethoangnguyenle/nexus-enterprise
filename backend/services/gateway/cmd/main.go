package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/timestamppb"

	assetpb "ngac-platform/proto/asset"
	authpb "ngac-platform/proto/auth"
	docpb "ngac-platform/proto/document"
	msgpb "ngac-platform/proto/messaging"
	wspb "ngac-platform/proto/workspace"
)

type Gateway struct {
	authClient          authpb.AuthServiceClient
	workspaceClient     wspb.WorkspaceServiceClient
	documentClient      docpb.DocumentServiceClient
	messagingClient     msgpb.MessagingServiceClient
	assetTypeClient     assetpb.AssetTypeServiceClient
	assetClient         assetpb.AssetServiceClient
	assetRequestClient  assetpb.AssetRequestServiceClient
	notificationClient  msgpb.NotificationServiceClient
	jwtSecret           string
	wsAddr              string
}

type Claims struct {
	UserID     string `json:"user_id"`
	Username   string `json:"username"`
	NGACNodeID string `json:"ngac_node_id"`
	jwt.RegisteredClaims
}

func main() {
	authAddr := envOr("AUTH_SERVICE_ADDR", "localhost:50052")
	wsAddr := envOr("WORKSPACE_SERVICE_ADDR", "localhost:50053")
	docAddr := envOr("DOCUMENT_SERVICE_ADDR", "localhost:50054")
	msgAddr := envOr("MESSAGING_SERVICE_ADDR", "localhost:50055")
	assetAddr := envOr("ASSET_SERVICE_ADDR", "localhost:50056")
	msgWSAddr := envOr("MESSAGING_WS_ADDR", "localhost:8081")
	jwtSecret := envOr("JWT_SECRET", "ngac-super-secret-key-change-in-production")
	port := envOr("PORT", "8080")

	authConn := dial(authAddr)
	wsConn := dial(wsAddr)
	docConn := dial(docAddr)
	msgConn := dial(msgAddr)
	assetConn := dial(assetAddr)

	gw := &Gateway{
		authClient:          authpb.NewAuthServiceClient(authConn),
		workspaceClient:     wspb.NewWorkspaceServiceClient(wsConn),
		documentClient:      docpb.NewDocumentServiceClient(docConn),
		messagingClient:     msgpb.NewMessagingServiceClient(msgConn),
		assetTypeClient:     assetpb.NewAssetTypeServiceClient(assetConn),
		assetClient:         assetpb.NewAssetServiceClient(assetConn),
		assetRequestClient:  assetpb.NewAssetRequestServiceClient(assetConn),
		notificationClient:  msgpb.NewNotificationServiceClient(msgConn),
		jwtSecret:           jwtSecret,
		wsAddr:              msgWSAddr,
	}

	r := chi.NewRouter()
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true,
	}))

	// Public routes
	r.Post("/api/auth/register", gw.handleRegister)
	r.Post("/api/auth/login", gw.handleLogin)

	// Protected routes
	r.Group(func(r chi.Router) {
		r.Use(gw.authMiddleware)

		// Users
		r.Get("/api/users", gw.handleListUsers)

		// Workspaces
		r.Post("/api/workspaces", gw.handleCreateWorkspace)
		r.Get("/api/workspaces", gw.handleListWorkspaces)
		r.Get("/api/workspaces/{id}", gw.handleGetWorkspace)
		r.Post("/api/workspaces/{id}/invite", gw.handleInviteMember)
		r.Delete("/api/workspaces/{id}/members/{nodeId}", gw.handleRemoveMember)
		r.Get("/api/workspaces/{id}/members", gw.handleListMembers)
		r.Post("/api/workspaces/{id}/roles", gw.handleCreateRole)
		r.Get("/api/workspaces/{id}/roles", gw.handleListRoles)
		r.Post("/api/workspaces/{id}/folders", gw.handleCreateFolder)
		r.Post("/api/workspaces/{id}/permissions", gw.handleCreatePermission)

		// Documents
		r.Post("/api/workspaces/{id}/documents", gw.handleUploadDocument)
		r.Get("/api/workspaces/{id}/documents", gw.handleListDocuments)
		r.Post("/api/documents/{docId}/approve", gw.handleApproveDocument)
		r.Post("/api/documents/{docId}/share", gw.handleShareDocument)
		r.Post("/api/documents/{docId}/publish", gw.handlePublishDocument)

		// Messaging
		r.Post("/api/workspaces/{id}/channels", gw.handleCreateChannel)
		r.Get("/api/workspaces/{id}/channels", gw.handleListChannels)
		r.Post("/api/channels/{chId}/messages", gw.handleSendMessage)
		r.Get("/api/channels/{chId}/messages", gw.handleGetMessages)
		r.Post("/api/channels/{chId}/members", gw.handleAddChannelMember)
		r.Get("/api/channels/{chId}/members", gw.handleListChannelMembers)
		r.Post("/api/dms", gw.handleCreateDM)
		r.Get("/api/dms", gw.handleListDMs)

		// Threads
		r.Get("/api/messages/{msgId}/thread", gw.handleGetThread)
		r.Get("/api/threads/entity/{entityType}/{entityId}", gw.handleFindThreadsByEntity)

		// Notifications
		r.Get("/api/notifications", gw.handleListNotifications)
		r.Post("/api/notifications/{notifId}/read", gw.handleMarkRead)
		r.Post("/api/notifications/read-all", gw.handleMarkAllRead)
		r.Get("/api/notifications/unread-count", gw.handleUnreadCount)

		// Asset Types
		r.Post("/api/workspaces/{id}/asset-types", gw.handleCreateAssetType)
		r.Get("/api/workspaces/{id}/asset-types", gw.handleListAssetTypes)
		r.Get("/api/asset-types/{typeId}", gw.handleGetAssetType)
		r.Put("/api/asset-types/{typeId}/schema", gw.handleUpdateAssetTypeSchema)

		// Assets
		r.Post("/api/workspaces/{id}/assets", gw.handleCreateAsset)
		r.Get("/api/workspaces/{id}/assets", gw.handleListAssets)
		r.Get("/api/assets/{assetId}", gw.handleGetAsset)
		r.Put("/api/assets/{assetId}", gw.handleUpdateAsset)
		r.Delete("/api/assets/{assetId}", gw.handleDeleteAsset)

		// Asset Lifecycle
		r.Post("/api/assets/{assetId}/transition", gw.handleTransitionAsset)
		r.Get("/api/assets/{assetId}/transitions", gw.handleGetAvailableTransitions)
		r.Get("/api/assets/{assetId}/history", gw.handleGetAssetHistory)

		// Asset Requests
		r.Post("/api/workspaces/{id}/asset-requests", gw.handleCreateAssetRequest)
		r.Get("/api/workspaces/{id}/asset-requests", gw.handleListAssetRequests)
		r.Get("/api/asset-requests/{reqId}", gw.handleGetAssetRequest)
		r.Post("/api/asset-requests/{reqId}/approve", gw.handleApproveAssetRequest)
		r.Post("/api/asset-requests/{reqId}/reject", gw.handleRejectAssetRequest)
		r.Post("/api/asset-requests/{reqId}/assign", gw.handleAssignAsset)
		r.Post("/api/assets/{assetId}/return", gw.handleReturnAsset)

		// WebSocket
		r.Get("/api/ws", gw.handleWebSocket)
	})

	r.Get("/api/health", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	log.Printf("Gateway listening on :%s", port)
	if err := http.ListenAndServe(fmt.Sprintf(":%s", port), r); err != nil {
		log.Fatal(err)
	}
}

func (gw *Gateway) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		tokenStr := strings.TrimPrefix(auth, "Bearer ")
		token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (interface{}, error) {
			return []byte(gw.jwtSecret), nil
		})
		if err != nil {
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}
		claims := token.Claims.(*Claims)
		ctx := context.WithValue(r.Context(), "user_id", claims.UserID)
		ctx = context.WithValue(ctx, "username", claims.Username)
		ctx = context.WithValue(ctx, "ngac_node_id", claims.NGACNodeID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Auth handlers
func (gw *Gateway) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req authpb.RegisterRequest
	json.NewDecoder(r.Body).Decode(&req)
	resp, err := gw.authClient.Register(r.Context(), &req)
	writeResponse(w, resp, err)
}

func (gw *Gateway) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req authpb.LoginRequest
	json.NewDecoder(r.Body).Decode(&req)
	resp, err := gw.authClient.Login(r.Context(), &req)
	writeResponse(w, resp, err)
}

func (gw *Gateway) handleListUsers(w http.ResponseWriter, r *http.Request) {
	resp, err := gw.authClient.ListUsers(r.Context(), &authpb.ListUsersRequest{})
	writeResponse(w, resp, err)
}

// Workspace handlers
func (gw *Gateway) handleCreateWorkspace(w http.ResponseWriter, r *http.Request) {
	var body struct{ Name string `json:"name"` }
	json.NewDecoder(r.Body).Decode(&body)
	resp, err := gw.workspaceClient.CreateWorkspace(r.Context(), &wspb.CreateWorkspaceRequest{
		Name: body.Name, UserId: r.Context().Value("user_id").(string),
		UserNgacNodeId: r.Context().Value("ngac_node_id").(string),
	})
	writeResponse(w, resp, err)
}

func (gw *Gateway) handleListWorkspaces(w http.ResponseWriter, r *http.Request) {
	resp, err := gw.workspaceClient.ListWorkspaces(r.Context(), &wspb.ListWorkspacesRequest{
		UserNgacNodeId: r.Context().Value("ngac_node_id").(string),
	})
	writeResponse(w, resp, err)
}

func (gw *Gateway) handleGetWorkspace(w http.ResponseWriter, r *http.Request) {
	resp, err := gw.workspaceClient.GetWorkspace(r.Context(), &wspb.GetWorkspaceRequest{
		WorkspaceId: chi.URLParam(r, "id"),
	})
	writeResponse(w, resp, err)
}

func (gw *Gateway) handleInviteMember(w http.ResponseWriter, r *http.Request) {
	var body struct{ NGACNodeID string `json:"ngac_node_id"` }
	json.NewDecoder(r.Body).Decode(&body)
	resp, err := gw.workspaceClient.InviteMember(r.Context(), &wspb.InviteMemberRequest{
		WorkspaceId:     chi.URLParam(r, "id"),
		InviterNgacNodeId: r.Context().Value("ngac_node_id").(string),
		TargetNgacNodeId:  body.NGACNodeID,
	})
	writeResponse(w, resp, err)
}

func (gw *Gateway) handleRemoveMember(w http.ResponseWriter, r *http.Request) {
	resp, err := gw.workspaceClient.RemoveMember(r.Context(), &wspb.RemoveMemberRequest{
		WorkspaceId:      chi.URLParam(r, "id"),
		TargetNgacNodeId: chi.URLParam(r, "nodeId"),
	})
	writeResponse(w, resp, err)
}

func (gw *Gateway) handleListMembers(w http.ResponseWriter, r *http.Request) {
	resp, err := gw.workspaceClient.ListMembers(r.Context(), &wspb.ListMembersRequest{
		WorkspaceId: chi.URLParam(r, "id"),
	})
	writeResponse(w, resp, err)
}

func (gw *Gateway) handleCreateRole(w http.ResponseWriter, r *http.Request) {
	var body struct{ Name string `json:"name"` }
	json.NewDecoder(r.Body).Decode(&body)
	resp, err := gw.workspaceClient.CreateRole(r.Context(), &wspb.CreateRoleRequest{
		WorkspaceId: chi.URLParam(r, "id"), Name: body.Name,
	})
	writeResponse(w, resp, err)
}

func (gw *Gateway) handleListRoles(w http.ResponseWriter, r *http.Request) {
	resp, err := gw.workspaceClient.ListRoles(r.Context(), &wspb.ListRolesRequest{
		WorkspaceId: chi.URLParam(r, "id"),
	})
	writeResponse(w, resp, err)
}

func (gw *Gateway) handleCreateFolder(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name       string `json:"name"`
		ParentOAID string `json:"parent_oa_id"`
	}
	json.NewDecoder(r.Body).Decode(&body)
	resp, err := gw.workspaceClient.CreateFolder(r.Context(), &wspb.CreateFolderRequest{
		WorkspaceId: chi.URLParam(r, "id"), Name: body.Name, ParentOaId: body.ParentOAID,
	})
	writeResponse(w, resp, err)
}

func (gw *Gateway) handleCreatePermission(w http.ResponseWriter, r *http.Request) {
	var body struct {
		UAID       string   `json:"ua_id"`
		OAID       string   `json:"oa_id"`
		Operations []string `json:"operations"`
	}
	json.NewDecoder(r.Body).Decode(&body)
	resp, err := gw.workspaceClient.CreatePermission(r.Context(), &wspb.CreatePermissionRequest{
		WorkspaceId: chi.URLParam(r, "id"), UaId: body.UAID, OaId: body.OAID, Operations: body.Operations,
	})
	writeResponse(w, resp, err)
}

// Document handlers
func (gw *Gateway) handleUploadDocument(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Title    string `json:"title"`
		Filename string `json:"filename"`
		MimeType string `json:"mime_type"`
		Content  string `json:"content"` // base64
	}
	json.NewDecoder(r.Body).Decode(&body)
	resp, err := gw.documentClient.Upload(r.Context(), &docpb.UploadRequest{
		Title: body.Title, Filename: body.Filename, MimeType: body.MimeType,
		Content: []byte(body.Content),
		UserId: r.Context().Value("user_id").(string),
		UserNgacNodeId: r.Context().Value("ngac_node_id").(string),
		WorkspaceId: chi.URLParam(r, "id"),
	})
	writeResponse(w, resp, err)
}

func (gw *Gateway) handleListDocuments(w http.ResponseWriter, r *http.Request) {
	resp, err := gw.documentClient.List(r.Context(), &docpb.ListDocumentsRequest{
		WorkspaceId:    chi.URLParam(r, "id"),
		UserNgacNodeId: r.Context().Value("ngac_node_id").(string),
	})
	writeResponse(w, resp, err)
}

func (gw *Gateway) handleApproveDocument(w http.ResponseWriter, r *http.Request) {
	resp, err := gw.documentClient.Approve(r.Context(), &docpb.ApproveDocumentRequest{
		DocumentId:     chi.URLParam(r, "docId"),
		UserNgacNodeId: r.Context().Value("ngac_node_id").(string),
	})
	writeResponse(w, resp, err)
}

func (gw *Gateway) handleShareDocument(w http.ResponseWriter, r *http.Request) {
	var body struct {
		TargetUAID string   `json:"target_ua_id"`
		Operations []string `json:"operations"`
	}
	json.NewDecoder(r.Body).Decode(&body)
	resp, err := gw.documentClient.Share(r.Context(), &docpb.ShareDocumentRequest{
		DocumentId:     chi.URLParam(r, "docId"),
		UserNgacNodeId: r.Context().Value("ngac_node_id").(string),
		TargetUaId:     body.TargetUAID,
		Operations:     body.Operations,
	})
	writeResponse(w, resp, err)
}

func (gw *Gateway) handlePublishDocument(w http.ResponseWriter, r *http.Request) {
	resp, err := gw.documentClient.Publish(r.Context(), &docpb.PublishDocumentRequest{
		DocumentId:     chi.URLParam(r, "docId"),
		UserNgacNodeId: r.Context().Value("ngac_node_id").(string),
	})
	writeResponse(w, resp, err)
}

// Messaging handlers
func (gw *Gateway) handleCreateChannel(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name        string `json:"name"`
		ChannelType string `json:"channel_type"`
	}
	json.NewDecoder(r.Body).Decode(&body)
	if body.ChannelType == "" {
		body.ChannelType = "workspace"
	}
	resp, err := gw.messagingClient.CreateChannel(r.Context(), &msgpb.CreateChannelRequest{
		Name: body.Name, WorkspaceId: chi.URLParam(r, "id"),
		UserId: r.Context().Value("user_id").(string),
		UserNgacNodeId: r.Context().Value("ngac_node_id").(string),
		ChannelType: body.ChannelType,
	})
	writeResponse(w, resp, err)
}

func (gw *Gateway) handleListChannels(w http.ResponseWriter, r *http.Request) {
	resp, err := gw.messagingClient.ListChannels(r.Context(), &msgpb.ListChannelsRequest{
		WorkspaceId:    chi.URLParam(r, "id"),
		UserNgacNodeId: r.Context().Value("ngac_node_id").(string),
	})
	writeResponse(w, resp, err)
}

func (gw *Gateway) handleSendMessage(w http.ResponseWriter, r *http.Request) {
	var body struct{ Content string `json:"content"` }
	json.NewDecoder(r.Body).Decode(&body)
	resp, err := gw.messagingClient.SendMessage(r.Context(), &msgpb.SendMessageRequest{
		ChannelId:       chi.URLParam(r, "chId"),
		SenderId:        r.Context().Value("user_id").(string),
		SenderNgacNodeId: r.Context().Value("ngac_node_id").(string),
		Content:         body.Content,
	})
	writeResponse(w, resp, err)
}

func (gw *Gateway) handleGetMessages(w http.ResponseWriter, r *http.Request) {
	resp, err := gw.messagingClient.GetMessages(r.Context(), &msgpb.GetMessagesRequest{
		ChannelId:      chi.URLParam(r, "chId"),
		UserNgacNodeId: r.Context().Value("ngac_node_id").(string),
		Before:         r.URL.Query().Get("before"),
		Limit:          50,
	})
	writeResponse(w, resp, err)
}

func (gw *Gateway) handleAddChannelMember(w http.ResponseWriter, r *http.Request) {
	var body struct{ NGACNodeID string `json:"ngac_node_id"` }
	json.NewDecoder(r.Body).Decode(&body)
	resp, err := gw.messagingClient.AddChannelMember(r.Context(), &msgpb.AddChannelMemberRequest{
		ChannelId:            chi.URLParam(r, "chId"),
		RequesterNgacNodeId:  r.Context().Value("ngac_node_id").(string),
		TargetNgacNodeId:     body.NGACNodeID,
	})
	writeResponse(w, resp, err)
}

func (gw *Gateway) handleListChannelMembers(w http.ResponseWriter, r *http.Request) {
	resp, err := gw.messagingClient.ListChannelMembers(r.Context(), &msgpb.ListChannelMembersRequest{
		ChannelId: chi.URLParam(r, "chId"),
	})
	writeResponse(w, resp, err)
}

func (gw *Gateway) handleCreateDM(w http.ResponseWriter, r *http.Request) {
	var body struct {
		TargetUserID     string `json:"target_user_id"`
		TargetNGACNodeID string `json:"target_ngac_node_id"`
	}
	json.NewDecoder(r.Body).Decode(&body)
	resp, err := gw.messagingClient.CreateDM(r.Context(), &msgpb.CreateDMRequest{
		UserId:          r.Context().Value("user_id").(string),
		UserNgacNodeId:  r.Context().Value("ngac_node_id").(string),
		TargetUserId:    body.TargetUserID,
		TargetNgacNodeId: body.TargetNGACNodeID,
	})
	writeResponse(w, resp, err)
}

func (gw *Gateway) handleListDMs(w http.ResponseWriter, r *http.Request) {
	resp, err := gw.messagingClient.ListDMs(r.Context(), &msgpb.ListDMsRequest{
		UserNgacNodeId: r.Context().Value("ngac_node_id").(string),
	})
	writeResponse(w, resp, err)
}

var wsUpgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}

func (gw *Gateway) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Get the token from query param for the backend WS connection
	token := r.URL.Query().Get("token")
	if token == "" {
		// Try from auth middleware context
		ngacNodeID := r.Context().Value("ngac_node_id")
		if ngacNodeID == nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
	}

	// Proxy WebSocket to messaging service
	backendURL := url.URL{Scheme: "ws", Host: gw.wsAddr, Path: "/ws", RawQuery: r.URL.RawQuery}
	backendConn, _, err := websocket.DefaultDialer.Dial(backendURL.String(), nil)
	if err != nil {
		http.Error(w, "backend ws connect failed", http.StatusBadGateway)
		return
	}
	defer backendConn.Close()

	clientConn, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer clientConn.Close()

	// Bidirectional proxy
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			_, msg, err := backendConn.ReadMessage()
			if err != nil { return }
			if err := clientConn.WriteMessage(websocket.TextMessage, msg); err != nil { return }
		}
	}()
	for {
		_, msg, err := clientConn.ReadMessage()
		if err != nil { return }
		if err := backendConn.WriteMessage(websocket.TextMessage, msg); err != nil { return }
	}
}

func writeResponse(w http.ResponseWriter, resp interface{}, err error) {
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	json.NewEncoder(w).Encode(resp)
}

func dial(addr string) *grpc.ClientConn {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to dial %s: %v", addr, err)
	}
	return conn
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" { return v }
	return def
}

// suppress unused import warnings
var _ = timestamppb.Now
var _ = io.EOF
