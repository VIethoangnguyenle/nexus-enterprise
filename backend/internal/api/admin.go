package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"ngac-document-platform/internal/models"
	"ngac-document-platform/internal/ngac"
)

type AdminHandler struct {
	store *ngac.Store
}

func NewAdminHandler(store *ngac.Store) *AdminHandler {
	return &AdminHandler{store: store}
}

func (h *AdminHandler) ListCompanies(w http.ResponseWriter, r *http.Request) {
	pcs := h.store.GetNodesByType(models.NodeTypePolicyClass)

	type Company struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}

	var companies []Company
	for _, pc := range pcs {
		if pc.Name != "PC_Global" {
			companies = append(companies, Company{ID: pc.ID, Name: pc.Name})
		}
	}
	if companies == nil {
		companies = []Company{}
	}
	writeJSON(w, http.StatusOK, companies)
}

func (h *AdminHandler) ListDepartments(w http.ResponseWriter, r *http.Request) {
	companyID := chi.URLParam(r, "id")

	children := h.store.GetChildrenOfNode(companyID)

	type Department struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Type string `json:"type"`
	}

	var departments []Department
	for _, child := range children {
		if child.NodeType == models.NodeTypeUserAttribute {
			departments = append(departments, Department{ID: child.ID, Name: child.Name, Type: "UA"})
		}
	}
	if departments == nil {
		departments = []Department{}
	}
	writeJSON(w, http.StatusOK, departments)
}

func (h *AdminHandler) ListAllDepartments(w http.ResponseWriter, r *http.Request) {
	uas := h.store.GetNodesByType(models.NodeTypeUserAttribute)

	type Department struct {
		ID      string `json:"id"`
		Name    string `json:"name"`
		Company string `json:"company"`
	}

	var departments []Department
	for _, ua := range uas {
		if ua.Name == "PublicUsers" {
			continue
		}
		// Find parent PC
		parents := h.store.GetParentsOfNode(ua.ID)
		company := ""
		for _, p := range parents {
			if p.NodeType == models.NodeTypePolicyClass {
				company = p.Name
			}
		}
		departments = append(departments, Department{ID: ua.ID, Name: ua.Name, Company: company})
	}
	if departments == nil {
		departments = []Department{}
	}
	writeJSON(w, http.StatusOK, departments)
}

func (h *AdminHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.store.ListUsers(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list users"})
		return
	}
	if users == nil {
		users = []models.UserInfo{}
	}
	writeJSON(w, http.StatusOK, users)
}
