package api

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"

	"ngac-document-platform/internal/auth"
	"ngac-document-platform/internal/models"
	"ngac-document-platform/internal/ngac"
)

type AuthHandler struct {
	store *ngac.Store
}

func NewAuthHandler(store *ngac.Store) *AuthHandler {
	return &AuthHandler{store: store}
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req models.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.Username == "" || req.Password == "" || req.Company == "" || req.Department == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "username, password, company, and department are required"})
		return
	}

	// Check if username exists
	existing, _ := h.store.GetUserByUsername(r.Context(), req.Username)
	if existing != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "username already exists"})
		return
	}

	// Hash password
	hashedPw, err := auth.HashPassword(req.Password)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to hash password"})
		return
	}

	// Create NGAC user node
	userNode, err := h.store.CreateNode(r.Context(), req.Username, models.NodeTypeUser, map[string]string{
		"company":    req.Company,
		"department": req.Department,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create NGAC node"})
		return
	}

	// Find department UA
	deptUA := h.store.FindNodeByName(req.Department, models.NodeTypeUserAttribute)
	if deptUA == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "department not found: " + req.Department})
		return
	}

	// Assign user to department UA
	if _, err := h.store.CreateAssignment(r.Context(), userNode.ID, deptUA.ID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to assign user to department"})
		return
	}

	// Assign user to PublicUsers UA
	publicUA := h.store.FindNodeByName("PublicUsers", models.NodeTypeUserAttribute)
	if publicUA != nil {
		h.store.CreateAssignment(r.Context(), userNode.ID, publicUA.ID)
	}

	// Create user record
	userID := uuid.New().String()
	if err := h.store.CreateUser(r.Context(), userID, req.Username, hashedPw, userNode.ID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create user"})
		return
	}

	// Generate token
	token, _ := auth.GenerateToken(userID, req.Username)

	writeJSON(w, http.StatusCreated, models.LoginResponse{
		Token: token,
		User: models.UserInfo{
			ID:         userID,
			Username:   req.Username,
			Company:    req.Company,
			Department: req.Department,
			NGACNodeID: userNode.ID,
		},
	})
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req models.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	user, err := h.store.GetUserByUsername(r.Context(), req.Username)
	if err != nil || user == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
		return
	}

	if !auth.CheckPassword(req.Password, user.Password) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
		return
	}

	token, _ := auth.GenerateToken(user.ID, user.Username)

	// Get user info from graph
	info := models.UserInfo{
		ID:         user.ID,
		Username:   user.Username,
		NGACNodeID: user.NGACNodeID,
	}

	// Resolve company/department from graph
	parents := h.store.GetNodesByType(models.NodeTypeUserAttribute)
	userNode := h.store.FindNodeByName(user.Username, models.NodeTypeUser)
	if userNode != nil {
		for _, p := range parents {
			if h.store.IsAssigned(userNode.ID, p.ID) && p.Name != "PublicUsers" {
				info.Department = p.Name
				// Get PC
				grandparents := h.store.GetParentsOfNode(p.ID)
				for _, gp := range grandparents {
					if gp.NodeType == models.NodeTypePolicyClass && gp.Name != "PC_Global" {
						info.Company = gp.Name
					}
				}
			}
		}
	}

	writeJSON(w, http.StatusOK, models.LoginResponse{Token: token, User: info})
}
