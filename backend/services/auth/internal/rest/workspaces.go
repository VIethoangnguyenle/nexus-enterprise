// Package rest provides Echo REST handlers for the auth service.
// Handles client-facing HTTP/JSON for signup, signin, tenant switching, and user queries.
// Delegates all logic to the domain layer.
package rest

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"ngac-platform/pkg/httputil"
	"ngac-platform/services/auth/internal/domain"
)

// UpdateProfile handles PATCH /api/me/profile. It is a partial update: a field
// that is absent (or null) is left as it is, and "" clears it. The person
// edited is always the token's; nothing in the body can name another.
func (h *Handler) UpdateProfile(c echo.Context) error {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}

	var body struct {
		DisplayName *string `json:"display_name"`
		Title       *string `json:"title"`
		Location    *string `json:"location"`
		// The two fields below are read only to be refused by name.
		Department *string `json:"department"`
		AvatarURL  *string `json:"avatar_url"`
	}
	if err := c.Bind(&body); err != nil {
		return badBody()
	}
	// A department is assigned by an administrator, per workspace; letting
	// people write their own would let anyone present themselves as "Ban giám
	// đốc" in the directory. Refused, not ignored, so a client that sends it
	// learns it does not work.
	if body.Department != nil {
		return apiError(http.StatusBadRequest, "department_not_editable",
			"the department is assigned by a workspace administrator")
	}
	// There is nowhere to keep a picture yet, and a URL the person typed would
	// be loaded by every colleague's browser.
	if body.AvatarURL != nil {
		return apiError(http.StatusBadRequest, "avatar_not_supported", "profile pictures are not supported yet")
	}

	if err := h.svc.UpdateProfile(c.Request().Context(), claims.UserID, domain.ProfileUpdateInput{
		DisplayName: body.DisplayName,
		Title:       body.Title,
		Location:    body.Location,
	}); err != nil {
		return fail(c, err)
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "updated"})
}

// workspaceJSON is a workspace as the picker needs it. The identifier is for
// navigation; no screen prints it.
func workspaceJSON(w domain.WorkspaceInfo) map[string]any {
	return map[string]any{"id": w.ID, "name": w.Name, "role": w.Role, "member_count": w.MemberCount, "domain": w.Domain}
}

// ListMyWorkspaces handles GET /api/me/workspaces: the workspaces the caller
// actively belongs to, with their own role and the headcount.
func (h *Handler) ListMyWorkspaces(c echo.Context) error {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	list, err := h.svc.ListMyWorkspaces(c.Request().Context(), claims.UserID)
	if err != nil {
		return fail(c, err)
	}
	out := make([]map[string]any, len(list))
	for i, w := range list {
		out[i] = workspaceJSON(w)
	}
	return c.JSON(http.StatusOK, map[string]any{"workspaces": out})
}

// CreateMyWorkspace handles POST /api/me/workspaces. The name is the only
// input; the caller becomes the owner and the workspace is provisioned in full.
func (h *Handler) CreateMyWorkspace(c echo.Context) error {
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	var body struct {
		Name string `json:"name"`
	}
	if err := c.Bind(&body); err != nil {
		return badBody()
	}
	ws, err := h.svc.CreateMyWorkspace(c.Request().Context(), claims.UserID, claims.NGACNodeID, body.Name)
	if err != nil {
		return fail(c, err)
	}
	return c.JSON(http.StatusCreated, workspaceJSON(*ws))
}

// ListContacts handles GET /api/workspaces/:id/contacts: one page of the
// workspace directory. Query: department, location, search, limit (default 50,
// at most 200) and cursor (the next_cursor of the page before). The answer
// carries the true total, and next_cursor when more people follow.
func (h *Handler) ListContacts(c echo.Context) error {
	wsID := c.Param("id")
	if wsID == "" {
		return apiError(http.StatusBadRequest, "invalid_input", "workspace id required")
	}
	claims, err := httputil.RequireClaims(c)
	if err != nil {
		return err
	}
	q := domain.ContactQuery{
		Department: c.QueryParam("department"),
		Location:   c.QueryParam("location"),
		Search:     c.QueryParam("search"),
		Cursor:     c.QueryParam("cursor"),
	}
	if v := c.QueryParam("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			return apiError(http.StatusBadRequest, "invalid_input", "limit must be a positive number")
		}
		q.Limit = n
	}

	contacts, total, next, err := h.svc.ListContacts(c.Request().Context(), claims.UserID, wsID, q)
	if err != nil {
		return fail(c, err)
	}

	type contactJSON struct {
		UserID     string `json:"user_id"`
		NGACNodeID string `json:"ngac_node_id"`
		// Username is the login handle. It identifies a person to the chat's
		// own records; it is never a name, and display_name is empty (not the
		// handle) for someone who has not yet said what to call them.
		Username    string `json:"username"`
		DisplayName string `json:"display_name"`
		Email       string `json:"email"`
		Title       string `json:"title"`
		Department  string `json:"department"`
		Location    string `json:"location"`
		AvatarURL   string `json:"avatar_url"`
	}
	result := make([]contactJSON, len(contacts))
	for i, c := range contacts {
		result[i] = contactJSON{
			UserID: c.UserID, NGACNodeID: c.NGACNodeID, Username: c.Username,
			DisplayName: c.DisplayName, Email: c.Email,
			Title: c.Title, Department: c.Department,
			Location: c.Location, AvatarURL: c.AvatarURL,
		}
	}

	out := map[string]any{"contacts": result, "total": total}
	if next != "" {
		out["next_cursor"] = next
	}
	return c.JSON(http.StatusOK, out)
}
