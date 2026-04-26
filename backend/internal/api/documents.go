package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"ngac-document-platform/internal/models"
	"ngac-document-platform/internal/ngac"
)

type DocumentHandler struct {
	store      *ngac.Store
	constraints *ngac.ConstraintEngine
	dataDir    string
}

func NewDocumentHandler(store *ngac.Store, constraints *ngac.ConstraintEngine, dataDir string) *DocumentHandler {
	return &DocumentHandler{store: store, constraints: constraints, dataDir: dataDir}
}

func (h *DocumentHandler) Upload(w http.ResponseWriter, r *http.Request) {
	userID := GetUserID(r)
	user, _ := h.store.GetUserByID(r.Context(), userID)
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "user not found"})
		return
	}

	// Check upload constraint
	ctx := ngac.RequestContext{Time: time.Now(), UserID: userID, Operation: "upload"}
	if denied, name, msg, _ := h.constraints.Evaluate(ctx); denied {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": fmt.Sprintf("Constraint '%s' denied: %s", name, msg)})
		return
	}

	// Parse multipart form
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "failed to parse form"})
		return
	}

	title := r.FormValue("title")
	if title == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "title is required"})
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "file is required"})
		return
	}
	defer file.Close()

	docID := uuid.New().String()

	// Save file
	docDir := filepath.Join(h.dataDir, docID)
	os.MkdirAll(docDir, 0755)
	destPath := filepath.Join(docDir, header.Filename)
	dest, err := os.Create(destPath)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save file"})
		return
	}
	defer dest.Close()
	io.Copy(dest, file)

	// Create NGAC object node
	objNode, err := h.store.CreateNode(r.Context(), title, models.NodeTypeObject, map[string]string{
		"doc_id":   docID,
		"filename": header.Filename,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create NGAC node"})
		return
	}

	// Assign to DraftDocs
	draftOA := h.store.FindNodeByName("DraftDocs", models.NodeTypeObjectAttr)
	if draftOA != nil {
		h.store.CreateAssignment(r.Context(), objNode.ID, draftOA.ID)
	}

	// Also assign to the user's department docs OA
	userNode := h.store.FindNodeByName(user.Username, models.NodeTypeUser)
	if userNode != nil {
		parents := h.store.GetParentsOfNode(userNode.ID)
		for _, p := range parents {
			if p.NodeType == models.NodeTypeUserAttribute && p.Name != "PublicUsers" {
				// Find this UA's associated OAs to determine the company docs OA
				assocs := h.store.GetGraph().GetAssociationsFromUA(p.ID)
				for _, a := range assocs {
					oaNode := h.store.GetNode(a.OAID)
					if oaNode != nil && oaNode.NodeType == models.NodeTypeObjectAttr {
						// Assign document to this OA so department members can access it
						h.store.CreateAssignment(r.Context(), objNode.ID, oaNode.ID)
						break
					}
				}
			}
		}
	}

	doc := &models.Document{
		ID:         docID,
		Title:      title,
		Filename:   header.Filename,
		MimeType:   header.Header.Get("Content-Type"),
		OwnerID:    userID,
		NGACNodeID: objNode.ID,
	}

	if err := h.store.CreateDocument(r.Context(), doc); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save document"})
		return
	}

	doc.Status = "draft"
	doc.OwnerName = user.Username
	writeJSON(w, http.StatusCreated, doc)
}

func (h *DocumentHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := GetUserID(r)
	user, _ := h.store.GetUserByID(r.Context(), userID)
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "user not found"})
		return
	}

	allDocs, err := h.store.ListDocuments(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list documents"})
		return
	}

	// Filter by NGAC access
	graph := h.store.GetGraph()
	var accessible []models.Document
	for _, doc := range allDocs {
		decision := graph.CheckAccess(user.NGACNodeID, doc.NGACNodeID, "read")
		if decision.Decision == "ALLOW" {
			accessible = append(accessible, doc)
		}
	}

	if accessible == nil {
		accessible = []models.Document{}
	}
	writeJSON(w, http.StatusOK, accessible)
}

func (h *DocumentHandler) Get(w http.ResponseWriter, r *http.Request) {
	docID := chi.URLParam(r, "id")
	userID := GetUserID(r)
	user, _ := h.store.GetUserByID(r.Context(), userID)
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "user not found"})
		return
	}

	doc, err := h.store.GetDocument(r.Context(), docID)
	if err != nil || doc == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "document not found"})
		return
	}

	// NGAC access check
	graph := h.store.GetGraph()
	decision := graph.CheckAccess(user.NGACNodeID, doc.NGACNodeID, "read")
	if decision.Decision != "ALLOW" {
		writeJSON(w, http.StatusForbidden, decision)
		return
	}

	writeJSON(w, http.StatusOK, doc)
}

func (h *DocumentHandler) Delete(w http.ResponseWriter, r *http.Request) {
	docID := chi.URLParam(r, "id")
	userID := GetUserID(r)
	user, _ := h.store.GetUserByID(r.Context(), userID)
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "user not found"})
		return
	}

	doc, err := h.store.GetDocument(r.Context(), docID)
	if err != nil || doc == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "document not found"})
		return
	}

	// Check write access via NGAC
	graph := h.store.GetGraph()
	decision := graph.CheckAccess(user.NGACNodeID, doc.NGACNodeID, "write")
	if decision.Decision != "ALLOW" {
		writeJSON(w, http.StatusForbidden, decision)
		return
	}

	// Remove NGAC node (cascades assignments/associations)
	h.store.DeleteNode(r.Context(), doc.NGACNodeID)
	h.store.DeleteDocument(r.Context(), docID)

	// Remove file
	os.RemoveAll(filepath.Join(h.dataDir, docID))

	writeJSON(w, http.StatusOK, map[string]string{"message": "document deleted"})
}

func (h *DocumentHandler) Approve(w http.ResponseWriter, r *http.Request) {
	docID := chi.URLParam(r, "id")
	userID := GetUserID(r)
	user, _ := h.store.GetUserByID(r.Context(), userID)
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "user not found"})
		return
	}

	doc, err := h.store.GetDocument(r.Context(), docID)
	if err != nil || doc == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "document not found"})
		return
	}

	if doc.Status == "approved" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "document is already approved"})
		return
	}

	// Check approve access via NGAC
	graph := h.store.GetGraph()
	decision := graph.CheckAccess(user.NGACNodeID, doc.NGACNodeID, "approve")
	if decision.Decision != "ALLOW" {
		writeJSON(w, http.StatusForbidden, decision)
		return
	}

	// Move from DraftDocs to ApprovedDocs (graph mutation)
	draftOA := h.store.FindNodeByName("DraftDocs", models.NodeTypeObjectAttr)
	approvedOA := h.store.FindNodeByName("ApprovedDocs", models.NodeTypeObjectAttr)

	if draftOA != nil {
		h.store.RemoveAssignment(r.Context(), doc.NGACNodeID, draftOA.ID)
	}
	if approvedOA != nil {
		h.store.CreateAssignment(r.Context(), doc.NGACNodeID, approvedOA.ID)
	}

	doc.Status = "approved"
	writeJSON(w, http.StatusOK, doc)
}

func (h *DocumentHandler) Share(w http.ResponseWriter, r *http.Request) {
	docID := chi.URLParam(r, "id")
	userID := GetUserID(r)
	user, _ := h.store.GetUserByID(r.Context(), userID)
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "user not found"})
		return
	}

	var req models.ShareRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}

	doc, err := h.store.GetDocument(r.Context(), docID)
	if err != nil || doc == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "document not found"})
		return
	}

	// Check write access
	graph := h.store.GetGraph()
	decision := graph.CheckAccess(user.NGACNodeID, doc.NGACNodeID, "write")
	if decision.Decision != "ALLOW" {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "you don't have permission to share this document"})
		return
	}

	// Precondition: document must be approved
	if doc.Status != "approved" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Document must be approved before sharing"})
		return
	}

	// Verify target UA exists
	targetUA := h.store.GetNode(req.TargetUAID)
	if targetUA == nil || targetUA.NodeType != models.NodeTypeUserAttribute {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "target department not found"})
		return
	}

	if len(req.Operations) == 0 {
		req.Operations = []string{"read"}
	}

	// Create scoped share OA under SharedDocs
	sharedDocsOA := h.store.FindNodeByName("SharedDocs", models.NodeTypeObjectAttr)
	if sharedDocsOA == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "SharedDocs OA not found"})
		return
	}

	shareOAName := ngac.GetDocumentShareOAName(doc.NGACNodeID, req.TargetUAID)
	shareOA, err := h.store.CreateNode(r.Context(), shareOAName, models.NodeTypeObjectAttr, map[string]string{
		"share_type": "cross-company",
		"doc_id":     docID,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create share OA"})
		return
	}

	// Assign share OA under SharedDocs
	h.store.CreateAssignment(r.Context(), shareOA.ID, sharedDocsOA.ID)

	// Assign document to share OA
	h.store.CreateAssignment(r.Context(), doc.NGACNodeID, shareOA.ID)

	// Create association: target UA -> share OA with operations
	h.store.CreateAssociation(r.Context(), req.TargetUAID, shareOA.ID, req.Operations)

	writeJSON(w, http.StatusOK, map[string]string{
		"message":      "document shared successfully",
		"share_oa_id":  shareOA.ID,
		"target_ua":    targetUA.Name,
	})
}

func (h *DocumentHandler) RevokeShare(w http.ResponseWriter, r *http.Request) {
	docID := chi.URLParam(r, "id")
	shareOAID := chi.URLParam(r, "shareId")

	doc, err := h.store.GetDocument(r.Context(), docID)
	if err != nil || doc == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "document not found"})
		return
	}

	// Remove the share OA (cascades)
	h.store.DeleteNode(r.Context(), shareOAID)

	writeJSON(w, http.StatusOK, map[string]string{"message": "share revoked"})
}

func (h *DocumentHandler) ListShares(w http.ResponseWriter, r *http.Request) {
	docID := chi.URLParam(r, "id")
	doc, err := h.store.GetDocument(r.Context(), docID)
	if err != nil || doc == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "document not found"})
		return
	}

	shares := h.store.ListSharesForDocument(doc.NGACNodeID)
	for i := range shares {
		shares[i].DocumentID = docID
	}
	if shares == nil {
		shares = []models.ShareInfo{}
	}
	writeJSON(w, http.StatusOK, shares)
}

func (h *DocumentHandler) Publish(w http.ResponseWriter, r *http.Request) {
	docID := chi.URLParam(r, "id")
	userID := GetUserID(r)
	user, _ := h.store.GetUserByID(r.Context(), userID)
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "user not found"})
		return
	}

	doc, err := h.store.GetDocument(r.Context(), docID)
	if err != nil || doc == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "document not found"})
		return
	}

	if doc.Status != "approved" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Document must be approved before publishing"})
		return
	}

	publicDocsOA := h.store.FindNodeByName("PublicDocs", models.NodeTypeObjectAttr)
	if publicDocsOA == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "PublicDocs OA not found"})
		return
	}

	h.store.CreateAssignment(r.Context(), doc.NGACNodeID, publicDocsOA.ID)

	writeJSON(w, http.StatusOK, map[string]string{"message": "document published"})
}

func (h *DocumentHandler) Unpublish(w http.ResponseWriter, r *http.Request) {
	docID := chi.URLParam(r, "id")

	doc, err := h.store.GetDocument(r.Context(), docID)
	if err != nil || doc == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "document not found"})
		return
	}

	publicDocsOA := h.store.FindNodeByName("PublicDocs", models.NodeTypeObjectAttr)
	if publicDocsOA == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "PublicDocs OA not found"})
		return
	}

	h.store.RemoveAssignment(r.Context(), doc.NGACNodeID, publicDocsOA.ID)

	writeJSON(w, http.StatusOK, map[string]string{"message": "document unpublished"})
}

func (h *DocumentHandler) CheckAccess(w http.ResponseWriter, r *http.Request) {
	var req models.AccessCheckRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}

	// Look up user's NGAC node
	user, _ := h.store.GetUserByID(r.Context(), req.UserID)
	if user == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "user not found"})
		return
	}

	// Look up document's NGAC node
	doc, _ := h.store.GetDocument(r.Context(), req.ObjectID)
	if doc == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "document not found"})
		return
	}

	// Check access
	graph := h.store.GetGraph()
	decision := graph.CheckAccess(user.NGACNodeID, doc.NGACNodeID, req.Operation)

	// Apply constraints
	ctx := ngac.RequestContext{
		Time:      time.Now(),
		UserID:    req.UserID,
		ObjectID:  req.ObjectID,
		Operation: req.Operation,
	}
	denied, constraintName, msg, checked := h.constraints.Evaluate(ctx)
	decision.Explanation.ConstraintsChecked = checked

	if denied && decision.Decision == "ALLOW" {
		decision.Decision = "DENY"
		decision.Explanation.ConstraintDenied = &models.ConstraintDenial{
			Name:    constraintName,
			Message: msg,
		}
	}

	writeJSON(w, http.StatusOK, decision)
}
